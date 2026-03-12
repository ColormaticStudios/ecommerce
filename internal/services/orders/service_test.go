package orders

import (
	"testing"
	"time"

	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newOrdersTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.CheckoutSession{}, &models.Product{}, &models.ProductVariant{}, &models.Order{}, &models.OrderItem{}))
	return db
}

func seedOrderSession(t *testing.T, db *gorm.DB, userID *uint) models.CheckoutSession {
	t.Helper()
	session := models.CheckoutSession{
		PublicToken: "orders-test-session",
		Status:      models.CheckoutSessionStatusActive,
		ExpiresAt:   time.Now().Add(24 * time.Hour).UTC(),
		LastSeenAt:  time.Now().UTC(),
	}
	if userID != nil {
		session.UserID = userID
	}
	require.NoError(t, db.Create(&session).Error)
	return session
}

func seedVariant(t *testing.T, db *gorm.DB, sku string, stock int) models.ProductVariant {
	t.Helper()
	product := models.Product{SKU: sku, Name: sku, Price: models.MoneyFromFloat(10), Stock: stock, IsPublished: true}
	require.NoError(t, db.Create(&product).Error)
	variant := models.ProductVariant{
		ProductID:   product.ID,
		SKU:         sku + "-default",
		Title:       sku,
		Price:       models.MoneyFromFloat(10),
		Stock:       stock,
		Position:    1,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&variant).Error)
	return variant
}

func TestApplyStatusTransition_CommitsStock(t *testing.T) {
	db := newOrdersTestDB(t)

	variant := seedVariant(t, db, "SKU-1", 8)
	userID := uint(1)
	session := seedOrderSession(t, db, &userID)
	order := models.Order{UserID: &userID, CheckoutSessionID: session.ID, Total: models.MoneyFromFloat(20), Status: models.StatusPending}
	require.NoError(t, db.Create(&order).Error)
	require.NoError(t, db.Create(&models.OrderItem{
		OrderID:          order.ID,
		ProductVariantID: variant.ID,
		VariantSKU:       variant.SKU,
		VariantTitle:     variant.Title,
		Quantity:         3,
		Price:            models.MoneyFromFloat(10),
	}).Error)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return ApplyStatusTransition(tx, &order, models.StatusPaid)
	}))

	var updatedVariant models.ProductVariant
	require.NoError(t, db.First(&updatedVariant, variant.ID).Error)
	assert.Equal(t, 5, updatedVariant.Stock)

	var updatedOrder models.Order
	require.NoError(t, db.First(&updatedOrder, order.ID).Error)
	assert.Equal(t, models.StatusPaid, updatedOrder.Status)
}

func TestApplyStatusTransition_RestoresStock(t *testing.T) {
	db := newOrdersTestDB(t)

	variant := seedVariant(t, db, "SKU-2", 4)
	userID := uint(1)
	session := seedOrderSession(t, db, &userID)
	order := models.Order{UserID: &userID, CheckoutSessionID: session.ID, Total: models.MoneyFromFloat(20), Status: models.StatusPaid}
	require.NoError(t, db.Create(&order).Error)
	require.NoError(t, db.Create(&models.OrderItem{
		OrderID:          order.ID,
		ProductVariantID: variant.ID,
		VariantSKU:       variant.SKU,
		VariantTitle:     variant.Title,
		Quantity:         2,
		Price:            models.MoneyFromFloat(10),
	}).Error)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return ApplyStatusTransition(tx, &order, models.StatusCancelled)
	}))

	var updatedVariant models.ProductVariant
	require.NoError(t, db.First(&updatedVariant, variant.ID).Error)
	assert.Equal(t, 6, updatedVariant.Stock)
}

func TestApplyStatusTransition_InsufficientStock(t *testing.T) {
	db := newOrdersTestDB(t)

	variant := seedVariant(t, db, "SKU-3", 1)
	userID := uint(1)
	session := seedOrderSession(t, db, &userID)
	order := models.Order{UserID: &userID, CheckoutSessionID: session.ID, Total: models.MoneyFromFloat(20), Status: models.StatusPending}
	require.NoError(t, db.Create(&order).Error)
	require.NoError(t, db.Create(&models.OrderItem{
		OrderID:          order.ID,
		ProductVariantID: variant.ID,
		VariantSKU:       variant.SKU,
		VariantTitle:     variant.Title,
		Quantity:         3,
		Price:            models.MoneyFromFloat(10),
	}).Error)

	err := db.Transaction(func(tx *gorm.DB) error {
		return ApplyStatusTransition(tx, &order, models.StatusPaid)
	})
	require.Error(t, err)

	var stockErr *InsufficientStockError
	require.ErrorAs(t, err, &stockErr)
	assert.Equal(t, variant.ID, stockErr.ProductVariantID)
	assert.Equal(t, 3, stockErr.Requested)
	assert.Equal(t, 1, stockErr.Available)
}
