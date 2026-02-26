package orders

import (
	"testing"

	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newOrdersTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Product{}, &models.Order{}, &models.OrderItem{}))
	return db
}

func TestApplyStatusTransition_CommitsStock(t *testing.T) {
	db := newOrdersTestDB(t)

	product := models.Product{SKU: "SKU-1", Name: "Item", Price: models.MoneyFromFloat(10), Stock: 8, IsPublished: true}
	require.NoError(t, db.Create(&product).Error)
	order := models.Order{UserID: 1, Total: models.MoneyFromFloat(20), Status: models.StatusPending}
	require.NoError(t, db.Create(&order).Error)
	require.NoError(t, db.Create(&models.OrderItem{OrderID: order.ID, ProductID: product.ID, Quantity: 3, Price: models.MoneyFromFloat(10)}).Error)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return ApplyStatusTransition(tx, &order, models.StatusPaid)
	}))

	var updatedProduct models.Product
	require.NoError(t, db.First(&updatedProduct, product.ID).Error)
	assert.Equal(t, 5, updatedProduct.Stock)

	var updatedOrder models.Order
	require.NoError(t, db.First(&updatedOrder, order.ID).Error)
	assert.Equal(t, models.StatusPaid, updatedOrder.Status)
}

func TestApplyStatusTransition_RestoresStock(t *testing.T) {
	db := newOrdersTestDB(t)

	product := models.Product{SKU: "SKU-2", Name: "Item", Price: models.MoneyFromFloat(10), Stock: 4, IsPublished: true}
	require.NoError(t, db.Create(&product).Error)
	order := models.Order{UserID: 1, Total: models.MoneyFromFloat(20), Status: models.StatusPaid}
	require.NoError(t, db.Create(&order).Error)
	require.NoError(t, db.Create(&models.OrderItem{OrderID: order.ID, ProductID: product.ID, Quantity: 2, Price: models.MoneyFromFloat(10)}).Error)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return ApplyStatusTransition(tx, &order, models.StatusCancelled)
	}))

	var updatedProduct models.Product
	require.NoError(t, db.First(&updatedProduct, product.ID).Error)
	assert.Equal(t, 6, updatedProduct.Stock)
}

func TestApplyStatusTransition_InsufficientStock(t *testing.T) {
	db := newOrdersTestDB(t)

	product := models.Product{SKU: "SKU-3", Name: "Item", Price: models.MoneyFromFloat(10), Stock: 1, IsPublished: true}
	require.NoError(t, db.Create(&product).Error)
	order := models.Order{UserID: 1, Total: models.MoneyFromFloat(20), Status: models.StatusPending}
	require.NoError(t, db.Create(&order).Error)
	require.NoError(t, db.Create(&models.OrderItem{OrderID: order.ID, ProductID: product.ID, Quantity: 3, Price: models.MoneyFromFloat(10)}).Error)

	err := db.Transaction(func(tx *gorm.DB) error {
		return ApplyStatusTransition(tx, &order, models.StatusPaid)
	})
	require.Error(t, err)

	var stockErr *InsufficientStockError
	require.ErrorAs(t, err, &stockErr)
	assert.Equal(t, product.ID, stockErr.ProductID)
	assert.Equal(t, 3, stockErr.Requested)
	assert.Equal(t, 1, stockErr.Available)
}
