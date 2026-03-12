package checkout

import (
	"testing"
	"time"

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
	require.NoError(t, db.AutoMigrate(&models.CheckoutSession{}, &models.Cart{}, &models.CartItem{}, &models.Product{}, &models.ProductVariant{}, &models.OrderItem{}, &models.IdempotencyKey{}))
	return db
}

func newLinkedCart(t *testing.T, db *gorm.DB, userID uint) models.Cart {
	t.Helper()
	session := models.CheckoutSession{
		PublicToken: "checkout-service-session",
		UserID:      &userID,
		Status:      models.CheckoutSessionStatusActive,
		ExpiresAt:   time.Now().Add(24 * time.Hour).UTC(),
		LastSeenAt:  time.Now().UTC(),
	}
	require.NoError(t, db.Create(&session).Error)

	cart := models.Cart{CheckoutSessionID: session.ID}
	require.NoError(t, db.Create(&cart).Error)
	return cart
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

	cart := newLinkedCart(t, db, 42)
	require.NoError(t, db.Create(&models.CartItem{CartID: cart.ID, ProductVariantID: variantA.ID, Quantity: 3}).Error)
	require.NoError(t, db.Create(&models.CartItem{CartID: cart.ID, ProductVariantID: variantB.ID, Quantity: 1}).Error)

	err := db.Transaction(func(tx *gorm.DB) error {
		return ClearOrderedItemsFromCart(tx, cart.CheckoutSessionID, []models.OrderItem{
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

func TestCleanupExpiredState(t *testing.T) {
	db := newCheckoutTestDB(t)

	expiredSession := models.CheckoutSession{
		PublicToken: "expired-session",
		Status:      models.CheckoutSessionStatusActive,
		ExpiresAt:   time.Now().Add(-time.Hour).UTC(),
		LastSeenAt:  time.Now().Add(-2 * time.Hour).UTC(),
	}
	activeSession := models.CheckoutSession{
		PublicToken: "active-session",
		Status:      models.CheckoutSessionStatusActive,
		ExpiresAt:   time.Now().Add(time.Hour).UTC(),
		LastSeenAt:  time.Now().UTC(),
	}
	require.NoError(t, db.Create(&expiredSession).Error)
	require.NoError(t, db.Create(&activeSession).Error)

	expiredKey := models.IdempotencyKey{
		Scope:             "checkout_order_create",
		Key:               "expired-key",
		RequestHash:       "hash-1",
		CheckoutSessionID: expiredSession.ID,
		ExpiresAt:         time.Now().Add(-time.Minute).UTC(),
	}
	activeKey := models.IdempotencyKey{
		Scope:             "checkout_order_create",
		Key:               "active-key",
		RequestHash:       "hash-2",
		CheckoutSessionID: activeSession.ID,
		ExpiresAt:         time.Now().Add(time.Hour).UTC(),
	}
	require.NoError(t, db.Create(&expiredKey).Error)
	require.NoError(t, db.Create(&activeKey).Error)

	summary, err := CleanupExpiredState(db, time.Now().UTC())
	require.NoError(t, err)
	assert.EqualValues(t, 1, summary.ExpiredSessions)
	assert.EqualValues(t, 1, summary.DeletedIdempotencyKeys)

	var reloadedExpired models.CheckoutSession
	require.NoError(t, db.First(&reloadedExpired, expiredSession.ID).Error)
	assert.Equal(t, models.CheckoutSessionStatusExpired, reloadedExpired.Status)

	var reloadedActive models.CheckoutSession
	require.NoError(t, db.First(&reloadedActive, activeSession.ID).Error)
	assert.Equal(t, models.CheckoutSessionStatusActive, reloadedActive.Status)

	var keyCount int64
	require.NoError(t, db.Model(&models.IdempotencyKey{}).Count(&keyCount).Error)
	assert.EqualValues(t, 1, keyCount)
}
