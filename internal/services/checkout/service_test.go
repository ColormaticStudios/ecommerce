package checkout

import (
	"testing"

	"ecommerce/internal/checkoutplugins"
	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newCheckoutTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Cart{}, &models.CartItem{}, &models.Product{}, &models.ProductVariant{}, &models.OrderItem{}))
	return db
}

func TestResolveProviderSelection(t *testing.T) {
	manager := checkoutplugins.NewDefaultManager()
	result, err := ResolveProviderSelection(manager, models.MoneyFromFloat(100), ProviderSelection{
		PaymentProviderID:  "dummy-card",
		ShippingProviderID: "dummy-ground",
		TaxProviderID:      "dummy-us-tax",
		PaymentData: map[string]string{
			"cardholder_name": "Alex Tester",
			"card_number":     "4111111111111111",
			"exp_month":       "12",
			"exp_year":        "2030",
		},
		ShippingData: map[string]string{
			"full_name":     "Alex Tester",
			"line1":         "123 Main St",
			"city":          "Austin",
			"state":         "TX",
			"postal_code":   "78701",
			"country":       "US",
			"service_level": "standard",
		},
		TaxData: map[string]string{"state": "TX"},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.PaymentDisplay)
	assert.NotEmpty(t, result.ShippingAddress)
	assert.True(t, result.Total > 0)
}

func TestClearOrderedItemsFromCart(t *testing.T) {
	db := newCheckoutTestDB(t)

	productA := models.Product{SKU: "A", Name: "A", Price: models.MoneyFromFloat(10), Stock: 10, IsPublished: true}
	productB := models.Product{SKU: "B", Name: "B", Price: models.MoneyFromFloat(5), Stock: 10, IsPublished: true}
	require.NoError(t, db.Create(&productA).Error)
	require.NoError(t, db.Create(&productB).Error)
	variantA := models.ProductVariant{ProductID: productA.ID, SKU: "A-default", Title: "A", Price: models.MoneyFromFloat(10), Stock: 10, Position: 1, IsPublished: true}
	variantB := models.ProductVariant{ProductID: productB.ID, SKU: "B-default", Title: "B", Price: models.MoneyFromFloat(5), Stock: 10, Position: 1, IsPublished: true}
	require.NoError(t, db.Create(&variantA).Error)
	require.NoError(t, db.Create(&variantB).Error)

	cart := models.Cart{UserID: 42}
	require.NoError(t, db.Create(&cart).Error)
	require.NoError(t, db.Create(&models.CartItem{CartID: cart.ID, ProductVariantID: variantA.ID, Quantity: 3}).Error)
	require.NoError(t, db.Create(&models.CartItem{CartID: cart.ID, ProductVariantID: variantB.ID, Quantity: 1}).Error)

	err := db.Transaction(func(tx *gorm.DB) error {
		return ClearOrderedItemsFromCart(tx, 42, []models.OrderItem{
			{ProductVariantID: variantA.ID, Quantity: 2},
			{ProductVariantID: variantB.ID, Quantity: 1},
		})
	})
	require.NoError(t, err)

	var cartItems []models.CartItem
	require.NoError(t, db.Where("cart_id = ?", cart.ID).Order("product_variant_id asc").Find(&cartItems).Error)
	require.Len(t, cartItems, 1)
	assert.Equal(t, variantA.ID, cartItems[0].ProductVariantID)
	assert.Equal(t, 1, cartItems[0].Quantity)
}
