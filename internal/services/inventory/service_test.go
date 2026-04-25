package inventory

import (
	"testing"
	"time"

	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newInventoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Product{},
		&models.ProductVariant{},
		&models.InventoryItem{},
		&models.InventoryLevel{},
		&models.InventoryMovement{},
		&models.InventoryReservation{},
		&models.CheckoutSession{},
		&models.Order{},
		&models.PaymentIntent{},
		&models.InventoryThreshold{},
		&models.InventoryAlert{},
		&models.InventoryAdjustment{},
		&models.Supplier{},
		&models.PurchaseOrder{},
		&models.PurchaseOrderItem{},
		&models.InventoryReceipt{},
		&models.InventoryReceiptItem{},
	))
	return db
}

func TestApplyMovementCreatesAndDeduplicatesLowStockAlert(t *testing.T) {
	db := newInventoryTestDB(t)
	variant := seedInventoryVariant(t, db, 8)
	_, err := SetThreshold(db, ThresholdInput{LowStockQuantity: 5})
	require.NoError(t, err)

	_, err = ApplyMovement(db, MovementInput{
		ProductVariantID: variant.ID,
		MovementType:     MovementTypeOrderCommit,
		QuantityDelta:    -3,
		ReferenceType:    ReferenceTypeOrder,
	})
	require.NoError(t, err)
	_, err = ApplyMovement(db, MovementInput{
		ProductVariantID: variant.ID,
		MovementType:     MovementTypeOrderCommit,
		QuantityDelta:    -1,
		ReferenceType:    ReferenceTypeOrder,
	})
	require.NoError(t, err)

	var alerts []models.InventoryAlert
	require.NoError(t, db.Find(&alerts).Error)
	require.Len(t, alerts, 1)
	assert.Equal(t, models.InventoryAlertTypeLowStock, alerts[0].AlertType)
	assert.Equal(t, models.InventoryAlertStatusOpen, alerts[0].Status)
}

func TestApplyMovementResolvesStockAlertAndRecordsRecovery(t *testing.T) {
	db := newInventoryTestDB(t)
	variant := seedInventoryVariant(t, db, 3)
	_, err := SetThreshold(db, ThresholdInput{LowStockQuantity: 5})
	require.NoError(t, err)

	_, err = ApplyMovement(db, MovementInput{
		ProductVariantID: variant.ID,
		MovementType:     MovementTypeOrderCommit,
		QuantityDelta:    -1,
		ReferenceType:    ReferenceTypeOrder,
	})
	require.NoError(t, err)
	_, err = ApplyMovement(db, MovementInput{
		ProductVariantID: variant.ID,
		MovementType:     "RESTOCK",
		QuantityDelta:    5,
		ReasonCode:       "test_restock",
	})
	require.NoError(t, err)

	var stockAlert models.InventoryAlert
	require.NoError(t, db.Where("alert_type = ?", models.InventoryAlertTypeLowStock).First(&stockAlert).Error)
	assert.Equal(t, models.InventoryAlertStatusResolved, stockAlert.Status)
	require.NotNil(t, stockAlert.ResolvedAt)

	var recovery models.InventoryAlert
	require.NoError(t, db.Where("alert_type = ?", models.InventoryAlertTypeRecovery).First(&recovery).Error)
	assert.Equal(t, models.InventoryAlertStatusResolved, recovery.Status)
}

func TestAckAndResolveAlertAreIdempotent(t *testing.T) {
	db := newInventoryTestDB(t)
	variant := seedInventoryVariant(t, db, 2)
	_, err := SetThreshold(db, ThresholdInput{LowStockQuantity: 5})
	require.NoError(t, err)
	_, err = ApplyMovement(db, MovementInput{
		ProductVariantID: variant.ID,
		MovementType:     MovementTypeOrderCommit,
		QuantityDelta:    -1,
		ReferenceType:    ReferenceTypeOrder,
	})
	require.NoError(t, err)

	alerts, err := ListAlerts(db, []string{models.InventoryAlertStatusOpen}, 10)
	require.NoError(t, err)
	require.Len(t, alerts, 1)

	acked, err := AckAlert(db, alerts[0].ID, AlertActionInput{ActorType: "admin"})
	require.NoError(t, err)
	assert.Equal(t, models.InventoryAlertStatusAcked, acked.Status)
	acked, err = AckAlert(db, alerts[0].ID, AlertActionInput{ActorType: "admin"})
	require.NoError(t, err)
	assert.Equal(t, models.InventoryAlertStatusAcked, acked.Status)

	resolved, err := ResolveAlert(db, alerts[0].ID, AlertActionInput{ActorType: "admin"})
	require.NoError(t, err)
	assert.Equal(t, models.InventoryAlertStatusResolved, resolved.Status)
}

func TestSetOnHandThenReserveCrossingThresholdCreatesAlert(t *testing.T) {
	db := newInventoryTestDB(t)
	variant := seedInventoryVariant(t, db, 10)
	_, err := SetThreshold(db, ThresholdInput{LowStockQuantity: 5})
	require.NoError(t, err)
	_, err = GetAvailability(db, variant.ID)
	require.NoError(t, err)

	availability, err := SetOnHand(db, variant.ID, 6, MovementInput{
		MovementType:  MovementTypeAdminSync,
		ReferenceType: "PRODUCT_PUBLISH",
		ReasonCode:    "test_stock_publish",
	})
	require.NoError(t, err)
	assert.Equal(t, 6, availability.Available)

	_, availability, err = Reserve(db, ReservationInput{
		ProductVariantID: variant.ID,
		Quantity:         1,
		OwnerType:        ReferenceTypeOrder,
		IdempotencyKey:   "threshold-crossing-reserve",
	})
	require.NoError(t, err)
	assert.Equal(t, 5, availability.Available)

	var alert models.InventoryAlert
	require.NoError(t, db.Where("product_variant_id = ?", variant.ID).First(&alert).Error)
	assert.Equal(t, models.InventoryAlertTypeLowStock, alert.AlertType)
	assert.Equal(t, models.InventoryAlertStatusOpen, alert.Status)
	assert.Equal(t, 5, alert.Available)
	assert.Equal(t, 5, alert.Threshold)
}

func TestPurchaseOrderIssuePartialAndFullReceipt(t *testing.T) {
	db := newInventoryTestDB(t)
	variant := seedInventoryVariant(t, db, 2)

	po, err := CreatePurchaseOrder(db, PurchaseOrderInput{
		Supplier: &SupplierInput{Name: "Test Supplier"},
		Items: []PurchaseOrderItemInput{{
			ProductVariantID: variant.ID,
			QuantityOrdered:  5,
			UnitCost:         3.25,
		}},
	})
	require.NoError(t, err)
	assert.Equal(t, models.PurchaseOrderStatusDraft, po.Status)
	require.Len(t, po.Items, 1)

	po, err = IssuePurchaseOrder(db, po.ID)
	require.NoError(t, err)
	assert.Equal(t, models.PurchaseOrderStatusIssued, po.Status)

	receipt, po, err := ReceivePurchaseOrder(db, po.ID, ReceivePurchaseOrderInput{
		Items: []ReceiveItemInput{{PurchaseOrderItemID: po.Items[0].ID, QuantityReceived: 2}},
	})
	require.NoError(t, err)
	assert.Equal(t, models.PurchaseOrderStatusPartiallyReceived, po.Status)
	require.Len(t, receipt.Items, 1)

	availability, err := GetAvailability(db, variant.ID)
	require.NoError(t, err)
	assert.Equal(t, 4, availability.Available)

	_, po, err = ReceivePurchaseOrder(db, po.ID, ReceivePurchaseOrderInput{
		Items: []ReceiveItemInput{{PurchaseOrderItemID: po.Items[0].ID, QuantityReceived: 3}},
	})
	require.NoError(t, err)
	assert.Equal(t, models.PurchaseOrderStatusReceived, po.Status)

	availability, err = GetAvailability(db, variant.ID)
	require.NoError(t, err)
	assert.Equal(t, 7, availability.Available)

	var movements int64
	require.NoError(t, db.Model(&models.InventoryMovement{}).Where("movement_type = ?", MovementTypeRestockReceipt).Count(&movements).Error)
	assert.Equal(t, int64(2), movements)
}

func TestCreateAdjustmentRejectsInvalidReason(t *testing.T) {
	db := newInventoryTestDB(t)
	variant := seedInventoryVariant(t, db, 4)

	_, _, err := CreateAdjustment(db, AdjustmentInput{
		ProductVariantID: variant.ID,
		QuantityDelta:    1,
		ReasonCode:       "BAD_REASON",
		ActorType:        "admin",
	}, AdjustmentPolicy{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid adjustment reason")
}

func TestCreateAdjustmentEnforcesApprovalPolicy(t *testing.T) {
	db := newInventoryTestDB(t)
	variant := seedInventoryVariant(t, db, 4)

	_, _, err := CreateAdjustment(db, AdjustmentInput{
		ProductVariantID: variant.ID,
		QuantityDelta:    1,
		ReasonCode:       models.InventoryAdjustmentReasonCorrection,
		ActorType:        "admin",
	}, AdjustmentPolicy{RequireApproval: true})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "approval")
}

func TestCreateAdjustmentWritesAuditMovementAndUpdatesAvailability(t *testing.T) {
	db := newInventoryTestDB(t)
	variant := seedInventoryVariant(t, db, 4)

	adjustment, availability, err := CreateAdjustment(db, AdjustmentInput{
		ProductVariantID: variant.ID,
		QuantityDelta:    3,
		ReasonCode:       models.InventoryAdjustmentReasonCycleCountGain,
		Notes:            "counted shelf",
		ActorType:        "admin",
		ApprovedByType:   "admin",
	}, AdjustmentPolicy{RequireApproval: true})

	require.NoError(t, err)
	assert.Equal(t, 7, availability.OnHand)
	assert.Equal(t, 7, availability.Available)
	assert.Equal(t, models.InventoryAdjustmentReasonCycleCountGain, adjustment.ReasonCode)
	assert.Equal(t, "counted shelf", adjustment.Notes)
	require.NotNil(t, adjustment.ApprovedAt)

	var movement models.InventoryMovement
	require.NoError(t, db.Where("reference_type = ? AND reference_id = ?", ReferenceTypeAdjustment, adjustment.ID).First(&movement).Error)
	assert.Equal(t, MovementTypeAdjustment, movement.MovementType)
	assert.Equal(t, 3, movement.QuantityDelta)
	assert.Equal(t, models.InventoryAdjustmentReasonCycleCountGain, movement.ReasonCode)
}

func TestReconcileDetectsLevelAndReservationDrift(t *testing.T) {
	db := newInventoryTestDB(t)
	variant := seedInventoryVariant(t, db, 5)
	reservation, _, err := Reserve(db, ReservationInput{
		ProductVariantID: variant.ID,
		Quantity:         2,
		OwnerType:        ReferenceTypeOrder,
		IdempotencyKey:   "reconcile-reservation",
		ExpiresAt:        time.Now().UTC().Add(-time.Minute),
	})
	require.NoError(t, err)

	var level models.InventoryLevel
	require.NoError(t, db.Where("inventory_item_id = ?", reservation.InventoryItemID).First(&level).Error)
	require.NoError(t, db.Model(&models.InventoryLevel{}).Where("id = ?", level.ID).Updates(map[string]any{
		"reserved":  1,
		"available": 99,
	}).Error)

	report, err := Reconcile(db, time.Now().UTC())
	require.NoError(t, err)

	var issueTypes []string
	for _, issue := range report.Issues {
		issueTypes = append(issueTypes, issue.IssueType)
	}
	assert.Contains(t, issueTypes, "LEVEL_AVAILABLE_MISMATCH")
	assert.Contains(t, issueTypes, "RESERVED_MISMATCH")
	assert.Contains(t, issueTypes, "VARIANT_STOCK_MISMATCH")
	assert.Contains(t, issueTypes, "STALE_ACTIVE_RESERVATION")
}

func seedInventoryVariant(t *testing.T, db *gorm.DB, stock int) models.ProductVariant {
	t.Helper()
	product := models.Product{SKU: t.Name(), Name: t.Name(), Price: models.MoneyFromFloat(10), Stock: stock, IsPublished: true}
	require.NoError(t, db.Create(&product).Error)
	variant := models.ProductVariant{
		ProductID:   product.ID,
		SKU:         t.Name() + "-default",
		Title:       t.Name(),
		Price:       models.MoneyFromFloat(10),
		Stock:       stock,
		Position:    1,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&variant).Error)
	return variant
}

func TestGetAvailabilityBootstrapsFromVariantStock(t *testing.T) {
	db := newInventoryTestDB(t)
	variant := seedInventoryVariant(t, db, 7)

	availability, err := GetAvailability(db, variant.ID)
	require.NoError(t, err)

	assert.Equal(t, 7, availability.OnHand)
	assert.Equal(t, 0, availability.Reserved)
	assert.Equal(t, 7, availability.Available)
}

func TestApplyMovementAppendsMovementAndUpdatesLevel(t *testing.T) {
	db := newInventoryTestDB(t)
	variant := seedInventoryVariant(t, db, 7)
	referenceID := uint(42)

	availability, err := ApplyMovement(db, MovementInput{
		ProductVariantID: variant.ID,
		MovementType:     MovementTypeOrderCommit,
		QuantityDelta:    -3,
		ReferenceType:    ReferenceTypeOrder,
		ReferenceID:      &referenceID,
		ReasonCode:       "test_commit",
	})
	require.NoError(t, err)

	assert.Equal(t, 4, availability.OnHand)
	assert.Equal(t, 4, availability.Available)

	var movement models.InventoryMovement
	require.NoError(t, db.First(&movement).Error)
	assert.Equal(t, -3, movement.QuantityDelta)
	assert.Equal(t, MovementTypeOrderCommit, movement.MovementType)

	var updatedVariant models.ProductVariant
	require.NoError(t, db.First(&updatedVariant, variant.ID).Error)
	assert.Equal(t, 4, updatedVariant.Stock)
}

func TestApplyMovementRejectsNegativeAvailability(t *testing.T) {
	db := newInventoryTestDB(t)
	variant := seedInventoryVariant(t, db, 2)

	_, err := ApplyMovement(db, MovementInput{
		ProductVariantID: variant.ID,
		MovementType:     MovementTypeOrderCommit,
		QuantityDelta:    -3,
		ReferenceType:    ReferenceTypeOrder,
	})
	require.Error(t, err)

	var availabilityErr *InsufficientAvailabilityError
	require.ErrorAs(t, err, &availabilityErr)
	assert.Equal(t, 2, availabilityErr.Available)

	var movementCount int64
	require.NoError(t, db.Model(&models.InventoryMovement{}).Count(&movementCount).Error)
	assert.Equal(t, int64(0), movementCount)
}

func TestReserveReducesAvailabilityAndIsIdempotent(t *testing.T) {
	db := newInventoryTestDB(t)
	variant := seedInventoryVariant(t, db, 5)

	reservation, availability, err := Reserve(db, ReservationInput{
		ProductVariantID: variant.ID,
		Quantity:         3,
		OwnerType:        ReferenceTypeOrder,
		IdempotencyKey:   "reserve-once",
	})
	require.NoError(t, err)
	assert.Equal(t, models.InventoryReservationStatusActive, reservation.Status)
	assert.Equal(t, 5, availability.OnHand)
	assert.Equal(t, 3, availability.Reserved)
	assert.Equal(t, 2, availability.Available)

	_, availability, err = Reserve(db, ReservationInput{
		ProductVariantID: variant.ID,
		Quantity:         3,
		OwnerType:        ReferenceTypeOrder,
		IdempotencyKey:   "reserve-once",
	})
	require.NoError(t, err)
	assert.Equal(t, 3, availability.Reserved)
	assert.Equal(t, 2, availability.Available)
}

func TestConsumeReservationsForOrderCommitsOnHandOnce(t *testing.T) {
	db := newInventoryTestDB(t)
	variant := seedInventoryVariant(t, db, 5)
	orderID := uint(77)
	sessionID := uint(12)

	_, _, err := Reserve(db, ReservationInput{
		ProductVariantID:  variant.ID,
		Quantity:          2,
		OwnerType:         ReferenceTypeOrder,
		CheckoutSessionID: &sessionID,
		OrderID:           &orderID,
		IdempotencyKey:    "consume-order",
	})
	require.NoError(t, err)

	consumed, err := ConsumeReservationsForOrder(db, orderID, "consume-once")
	require.NoError(t, err)
	assert.True(t, consumed)
	consumed, err = ConsumeReservationsForOrder(db, orderID, "consume-once")
	require.NoError(t, err)
	assert.True(t, consumed)

	availability, err := GetAvailability(db, variant.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, availability.OnHand)
	assert.Equal(t, 0, availability.Reserved)
	assert.Equal(t, 3, availability.Available)

	var movementCount int64
	require.NoError(t, db.Model(&models.InventoryMovement{}).Count(&movementCount).Error)
	assert.Equal(t, int64(1), movementCount)
}

func TestExpireReservationsReleasesAvailability(t *testing.T) {
	db := newInventoryTestDB(t)
	variant := seedInventoryVariant(t, db, 4)
	now := time.Now().UTC()

	_, _, err := Reserve(db, ReservationInput{
		ProductVariantID: variant.ID,
		Quantity:         2,
		OwnerType:        ReferenceTypeOrder,
		IdempotencyKey:   "expire-order",
		ExpiresAt:        now.Add(-time.Minute),
	})
	require.NoError(t, err)

	expired, err := ExpireReservations(db, now, 10)
	require.NoError(t, err)
	assert.Equal(t, 1, expired)

	availability, err := GetAvailability(db, variant.ID)
	require.NoError(t, err)
	assert.Equal(t, 4, availability.Available)
	assert.Equal(t, 0, availability.Reserved)
}

func TestExpireReservationsKeepsAuthorizedOrderHold(t *testing.T) {
	db := newInventoryTestDB(t)
	variant := seedInventoryVariant(t, db, 5)
	now := time.Now().UTC()
	session := models.CheckoutSession{
		PublicToken: "authorized-hold-session",
		Status:      models.CheckoutSessionStatusConverted,
		ExpiresAt:   now.Add(time.Hour),
		LastSeenAt:  now,
	}
	require.NoError(t, db.Create(&session).Error)
	order := models.Order{
		CheckoutSessionID: session.ID,
		Total:             models.MoneyFromFloat(30),
		Status:            models.StatusPending,
	}
	require.NoError(t, db.Create(&order).Error)
	_, _, err := Reserve(db, ReservationInput{
		ProductVariantID:  variant.ID,
		Quantity:          3,
		OwnerType:         ReferenceTypeOrder,
		CheckoutSessionID: &session.ID,
		OrderID:           &order.ID,
		IdempotencyKey:    "authorized-order-reservation",
		ExpiresAt:         now.Add(-time.Minute),
	})
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.PaymentIntent{
		OrderID:          order.ID,
		SnapshotID:       1,
		Provider:         "dummy",
		Status:           models.PaymentIntentStatusAuthorized,
		AuthorizedAmount: models.MoneyFromFloat(30),
		CapturedAmount:   0,
		Currency:         "USD",
		Version:          1,
	}).Error)

	expired, err := ExpireReservations(db, now, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, expired)

	var reservation models.InventoryReservation
	require.NoError(t, db.Where("order_id = ?", order.ID).First(&reservation).Error)
	assert.Equal(t, models.InventoryReservationStatusActive, reservation.Status)

	availability, err := GetAvailability(db, variant.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, availability.Reserved)
	assert.Equal(t, 2, availability.Available)
}
