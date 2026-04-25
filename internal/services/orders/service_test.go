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
	require.NoError(t, db.AutoMigrate(
		&models.CheckoutSession{},
		&models.Product{},
		&models.ProductVariant{},
		&models.InventoryItem{},
		&models.InventoryLevel{},
		&models.InventoryMovement{},
		&models.InventoryReservation{},
		&models.InventoryThreshold{},
		&models.InventoryAlert{},
		&models.Order{},
		&models.OrderItem{},
	))
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

	var movement models.InventoryMovement
	require.NoError(t, db.Where("reference_type = ? AND reference_id = ?", "ORDER", order.ID).First(&movement).Error)
	assert.Equal(t, "ORDER_COMMIT", movement.MovementType)
	assert.Equal(t, -3, movement.QuantityDelta)
}

func TestApplyStatusTransition_ConsumesReservation(t *testing.T) {
	db := newOrdersTestDB(t)

	variant := seedVariant(t, db, "SKU-R", 8)
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
	require.NoError(t, db.Create(&models.InventoryItem{ProductVariantID: variant.ID}).Error)
	var item models.InventoryItem
	require.NoError(t, db.Where("product_variant_id = ?", variant.ID).First(&item).Error)
	require.NoError(t, db.Create(&models.InventoryLevel{InventoryItemID: item.ID, OnHand: 8, Reserved: 3, Available: 5}).Error)
	require.NoError(t, db.Create(&models.InventoryReservation{
		InventoryItemID:   item.ID,
		ProductVariantID:  variant.ID,
		Quantity:          3,
		Status:            models.InventoryReservationStatusActive,
		ExpiresAt:         time.Now().Add(time.Hour),
		OwnerType:         "ORDER",
		CheckoutSessionID: &session.ID,
		OrderID:           &order.ID,
		IdempotencyKey:    "order-reservation",
	}).Error)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return ApplyStatusTransition(tx, &order, models.StatusPaid)
	}))

	var updatedVariant models.ProductVariant
	require.NoError(t, db.First(&updatedVariant, variant.ID).Error)
	assert.Equal(t, 5, updatedVariant.Stock)

	var reservation models.InventoryReservation
	require.NoError(t, db.Where("order_id = ?", order.ID).First(&reservation).Error)
	assert.Equal(t, models.InventoryReservationStatusConsumed, reservation.Status)
}

func TestApplyStatusTransition_PreservesReservationOnNoopPendingTransition(t *testing.T) {
	db := newOrdersTestDB(t)

	variant := seedVariant(t, db, "SKU-PENDING-HOLD", 8)
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
	require.NoError(t, db.Create(&models.InventoryItem{ProductVariantID: variant.ID}).Error)
	var item models.InventoryItem
	require.NoError(t, db.Where("product_variant_id = ?", variant.ID).First(&item).Error)
	require.NoError(t, db.Create(&models.InventoryLevel{InventoryItemID: item.ID, OnHand: 8, Reserved: 3, Available: 5}).Error)
	require.NoError(t, db.Create(&models.InventoryReservation{
		InventoryItemID:   item.ID,
		ProductVariantID:  variant.ID,
		Quantity:          3,
		Status:            models.InventoryReservationStatusActive,
		ExpiresAt:         time.Now().Add(time.Hour),
		OwnerType:         "ORDER",
		CheckoutSessionID: &session.ID,
		OrderID:           &order.ID,
		IdempotencyKey:    "pending-order-reservation",
	}).Error)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return ApplyStatusTransition(tx, &order, models.StatusPending)
	}))

	var reservation models.InventoryReservation
	require.NoError(t, db.Where("order_id = ?", order.ID).First(&reservation).Error)
	assert.Equal(t, models.InventoryReservationStatusActive, reservation.Status)

	var level models.InventoryLevel
	require.NoError(t, db.Where("inventory_item_id = ?", item.ID).First(&level).Error)
	assert.Equal(t, 3, level.Reserved)
	assert.Equal(t, 5, level.Available)
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

	var movement models.InventoryMovement
	require.NoError(t, db.Where("reference_type = ? AND reference_id = ?", "ORDER", order.ID).First(&movement).Error)
	assert.Equal(t, "ORDER_RELEASE", movement.MovementType)
	assert.Equal(t, 2, movement.QuantityDelta)
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
