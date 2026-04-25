package inventory

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	MovementTypeOrderCommit  = "ORDER_COMMIT"
	MovementTypeOrderRelease = "ORDER_RELEASE"
	MovementTypeAdminSync    = "ADMIN_STOCK_SYNC"
	MovementTypeAdjustment   = "INVENTORY_ADJUSTMENT"

	ReferenceTypeOrder       = "ORDER"
	ReferenceTypeReservation = "INVENTORY_RESERVATION"
	ReferenceTypeAdjustment  = "INVENTORY_ADJUSTMENT"

	ReservationTTL = 15 * time.Minute
)

type Availability struct {
	ProductVariantID uint
	OnHand           int
	Reserved         int
	Available        int
}

type MovementInput struct {
	ProductVariantID uint
	MovementType     string
	QuantityDelta    int
	ReferenceType    string
	ReferenceID      *uint
	ReasonCode       string
	ActorType        string
	ActorID          *uint
}

type ReservationInput struct {
	ProductVariantID  uint
	Quantity          int
	OwnerType         string
	OwnerID           *uint
	CheckoutSessionID *uint
	OrderID           *uint
	IdempotencyKey    string
	ExpiresAt         time.Time
}

type ReservationSummary struct {
	ID                uint
	ProductVariantID  uint
	Quantity          int
	Status            string
	ExpiresAt         time.Time
	OwnerType         string
	OwnerID           *uint
	CheckoutSessionID *uint
	OrderID           *uint
}

type ThresholdInput struct {
	ProductVariantID *uint
	LowStockQuantity int
}

type AlertActionInput struct {
	ActorType string
	ActorID   *uint
}

type AdjustmentInput struct {
	ProductVariantID uint
	QuantityDelta    int
	ReasonCode       string
	Notes            string
	ActorType        string
	ActorID          *uint
	ApprovedByType   string
	ApprovedByID     *uint
}

type AdjustmentPolicy struct {
	RequireApproval bool
}

type ReconciliationIssue struct {
	IssueType        string
	InventoryItemID  uint
	ProductVariantID uint
	Expected         int
	Actual           int
	Message          string
	EntityType       string
	EntityID         *uint
}

type ReconciliationReport struct {
	CheckedAt time.Time
	Issues    []ReconciliationIssue
}

type InventoryTimeline struct {
	ProductVariantID uint
	Movements        []models.InventoryMovement
	Reservations     []models.InventoryReservation
	Adjustments      []models.InventoryAdjustment
}

func GetAvailability(db *gorm.DB, productVariantID uint) (Availability, error) {
	item, level, err := ensureLevel(db, productVariantID)
	if err != nil {
		return Availability{}, err
	}

	return Availability{
		ProductVariantID: item.ProductVariantID,
		OnHand:           level.OnHand,
		Reserved:         level.Reserved,
		Available:        level.Available,
	}, nil
}

func Reserve(db *gorm.DB, input ReservationInput) (models.InventoryReservation, Availability, error) {
	if input.ProductVariantID == 0 {
		return models.InventoryReservation{}, Availability{}, fmt.Errorf("product variant id is required")
	}
	if input.Quantity < 1 {
		return models.InventoryReservation{}, Availability{}, fmt.Errorf("reservation quantity must be positive")
	}
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if idempotencyKey == "" {
		return models.InventoryReservation{}, Availability{}, fmt.Errorf("idempotency key is required")
	}

	var reservation models.InventoryReservation
	var availability Availability
	err := db.Transaction(func(tx *gorm.DB) error {
		var existing models.InventoryReservation
		err := tx.Where("idempotency_key = ?", idempotencyKey).First(&existing).Error
		if err == nil {
			reservation = existing
			availability, err = GetAvailability(tx, existing.ProductVariantID)
			return err
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		item, level, err := lockedLevel(tx, input.ProductVariantID)
		if err != nil {
			return err
		}
		if level.Available < input.Quantity {
			return &InsufficientAvailabilityError{
				ProductVariantID: input.ProductVariantID,
				Requested:        input.Quantity,
				Available:        level.Available,
			}
		}

		nextReserved := level.Reserved + input.Quantity
		nextAvailable := level.OnHand - nextReserved
		if nextReserved < 0 || nextAvailable < 0 {
			return &InsufficientAvailabilityError{
				ProductVariantID: input.ProductVariantID,
				Requested:        input.Quantity,
				Available:        level.Available,
			}
		}

		expiresAt := input.ExpiresAt.UTC()
		if expiresAt.IsZero() {
			expiresAt = time.Now().UTC().Add(ReservationTTL)
		}
		reservation = models.InventoryReservation{
			InventoryItemID:   item.ID,
			ProductVariantID:  input.ProductVariantID,
			Quantity:          input.Quantity,
			Status:            models.InventoryReservationStatusActive,
			ExpiresAt:         expiresAt,
			OwnerType:         strings.TrimSpace(input.OwnerType),
			OwnerID:           input.OwnerID,
			CheckoutSessionID: input.CheckoutSessionID,
			OrderID:           input.OrderID,
			IdempotencyKey:    idempotencyKey,
		}
		if err := tx.Create(&reservation).Error; err != nil {
			return err
		}
		availability = Availability{
			ProductVariantID: input.ProductVariantID,
			OnHand:           level.OnHand,
			Reserved:         nextReserved,
			Available:        nextAvailable,
		}
		return updateLevelAndVariant(tx, level.ID, input.ProductVariantID, availability)
	})
	return reservation, availability, err
}

func ReserveOrderItems(tx *gorm.DB, order models.Order, idempotencyKeyPrefix string, expiresAt time.Time) error {
	for _, item := range order.Items {
		key := fmt.Sprintf("%s:order:%d:variant:%d", strings.TrimSpace(idempotencyKeyPrefix), order.ID, item.ProductVariantID)
		sessionID := order.CheckoutSessionID
		orderID := order.ID
		if _, _, err := Reserve(tx, ReservationInput{
			ProductVariantID:  item.ProductVariantID,
			Quantity:          item.Quantity,
			OwnerType:         ReferenceTypeOrder,
			CheckoutSessionID: &sessionID,
			OrderID:           &orderID,
			IdempotencyKey:    key,
			ExpiresAt:         expiresAt,
		}); err != nil {
			return err
		}
	}
	return nil
}

func ConsumeReservationsForOrder(tx *gorm.DB, orderID uint, idempotencyKey string) (bool, error) {
	return closeReservationsForOrder(tx, orderID, idempotencyKey, models.InventoryReservationStatusConsumed)
}

func ReleaseReservationsForOrder(tx *gorm.DB, orderID uint, idempotencyKey string) error {
	_, err := closeReservationsForOrder(tx, orderID, idempotencyKey, models.InventoryReservationStatusReleased)
	return err
}

func ExpireReservations(db *gorm.DB, now time.Time, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	expired := 0
	err := db.Transaction(func(tx *gorm.DB) error {
		var reservations []models.InventoryReservation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("status = ? AND expires_at <= ?", models.InventoryReservationStatusActive, now.UTC()).
			Order("expires_at ASC, id ASC").
			Limit(limit).
			Find(&reservations).Error; err != nil {
			return err
		}
		for _, reservation := range reservations {
			protected, err := reservationHasActivePaymentIntent(tx, reservation)
			if err != nil {
				return err
			}
			if protected {
				continue
			}
			if err := releaseReservation(tx, &reservation, models.InventoryReservationStatusExpired, now.UTC()); err != nil {
				return err
			}
			expired++
		}
		return nil
	})
	return expired, err
}

func reservationHasActivePaymentIntent(tx *gorm.DB, reservation models.InventoryReservation) (bool, error) {
	if reservation.OrderID == nil {
		return false, nil
	}
	var count int64
	if err := tx.Model(&models.PaymentIntent{}).
		Where("order_id = ? AND status IN ?", *reservation.OrderID, []string{
			models.PaymentIntentStatusRequiresAction,
			models.PaymentIntentStatusAuthorized,
			models.PaymentIntentStatusPartiallyCaptured,
		}).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func ListReservations(db *gorm.DB, statuses []string, limit int) ([]models.InventoryReservation, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := db.Order("created_at DESC, id DESC").Limit(limit)
	if len(statuses) > 0 {
		query = query.Where("status IN ?", statuses)
	}
	var reservations []models.InventoryReservation
	if err := query.Find(&reservations).Error; err != nil {
		return nil, err
	}
	return reservations, nil
}

func ListAlerts(db *gorm.DB, statuses []string, limit int) ([]models.InventoryAlert, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := db.Order("opened_at DESC, id DESC").Limit(limit)
	if len(statuses) > 0 {
		query = query.Where("status IN ?", statuses)
	}
	var alerts []models.InventoryAlert
	if err := query.Find(&alerts).Error; err != nil {
		return nil, err
	}
	return alerts, nil
}

func AckAlert(db *gorm.DB, alertID uint, input AlertActionInput) (models.InventoryAlert, error) {
	return updateAlertStatus(db, alertID, input, models.InventoryAlertStatusAcked)
}

func ResolveAlert(db *gorm.DB, alertID uint, input AlertActionInput) (models.InventoryAlert, error) {
	return updateAlertStatus(db, alertID, input, models.InventoryAlertStatusResolved)
}

func CreateAdjustment(db *gorm.DB, input AdjustmentInput, policy AdjustmentPolicy) (models.InventoryAdjustment, Availability, error) {
	if input.ProductVariantID == 0 {
		return models.InventoryAdjustment{}, Availability{}, fmt.Errorf("product variant id is required")
	}
	if input.QuantityDelta == 0 {
		return models.InventoryAdjustment{}, Availability{}, fmt.Errorf("quantity delta is required")
	}
	reasonCode := strings.ToUpper(strings.TrimSpace(input.ReasonCode))
	if !slices.Contains(validAdjustmentReasons(), reasonCode) {
		return models.InventoryAdjustment{}, Availability{}, fmt.Errorf("invalid adjustment reason")
	}
	actorType := strings.TrimSpace(input.ActorType)
	if actorType == "" {
		return models.InventoryAdjustment{}, Availability{}, fmt.Errorf("actor type is required")
	}
	approvedByType := strings.TrimSpace(input.ApprovedByType)
	if policy.RequireApproval && approvedByType == "" {
		return models.InventoryAdjustment{}, Availability{}, fmt.Errorf("adjustment approval is required")
	}

	var adjustment models.InventoryAdjustment
	var availability Availability
	err := db.Transaction(func(tx *gorm.DB) error {
		item, level, err := lockedLevel(tx, input.ProductVariantID)
		if err != nil {
			return err
		}
		nextOnHand := level.OnHand + input.QuantityDelta
		nextAvailable := nextOnHand - level.Reserved
		if nextOnHand < 0 || nextAvailable < 0 {
			return &InsufficientAvailabilityError{
				ProductVariantID: input.ProductVariantID,
				Requested:        -input.QuantityDelta,
				Available:        level.Available,
			}
		}

		var approvedAt *time.Time
		if approvedByType != "" {
			now := time.Now().UTC()
			approvedAt = &now
		}
		adjustment = models.InventoryAdjustment{
			InventoryItemID:  item.ID,
			ProductVariantID: input.ProductVariantID,
			QuantityDelta:    input.QuantityDelta,
			ReasonCode:       reasonCode,
			Notes:            strings.TrimSpace(input.Notes),
			ActorType:        actorType,
			ActorID:          input.ActorID,
			ApprovedByType:   approvedByType,
			ApprovedByID:     input.ApprovedByID,
			ApprovedAt:       approvedAt,
		}
		if err := tx.Create(&adjustment).Error; err != nil {
			return err
		}
		referenceID := adjustment.ID
		movement := models.InventoryMovement{
			InventoryItemID: item.ID,
			MovementType:    MovementTypeAdjustment,
			QuantityDelta:   input.QuantityDelta,
			ReferenceType:   ReferenceTypeAdjustment,
			ReferenceID:     &referenceID,
			ReasonCode:      reasonCode,
			ActorType:       actorType,
			ActorID:         input.ActorID,
		}
		if err := tx.Create(&movement).Error; err != nil {
			return err
		}
		availability = Availability{
			ProductVariantID: input.ProductVariantID,
			OnHand:           nextOnHand,
			Reserved:         level.Reserved,
			Available:        nextAvailable,
		}
		return updateLevelAndVariant(tx, level.ID, input.ProductVariantID, availability)
	})
	return adjustment, availability, err
}

func Reconcile(db *gorm.DB, now time.Time) (ReconciliationReport, error) {
	checkedAt := now.UTC()
	if checkedAt.IsZero() {
		checkedAt = time.Now().UTC()
	}
	report := ReconciliationReport{CheckedAt: checkedAt}

	var levels []models.InventoryLevel
	if err := db.Preload("InventoryItem").Find(&levels).Error; err != nil {
		return report, err
	}
	for _, level := range levels {
		variantID := level.InventoryItem.ProductVariantID
		expectedAvailable := level.OnHand - level.Reserved
		if level.Available != expectedAvailable {
			report.Issues = append(report.Issues, ReconciliationIssue{
				IssueType:        "LEVEL_AVAILABLE_MISMATCH",
				InventoryItemID:  level.InventoryItemID,
				ProductVariantID: variantID,
				Expected:         expectedAvailable,
				Actual:           level.Available,
				Message:          "inventory level available does not equal on_hand minus reserved",
				EntityType:       "inventory_level",
				EntityID:         &level.ID,
			})
		}

		var activeReservationTotal int64
		if err := db.Model(&models.InventoryReservation{}).
			Where("inventory_item_id = ? AND status = ?", level.InventoryItemID, models.InventoryReservationStatusActive).
			Select("COALESCE(SUM(quantity), 0)").
			Scan(&activeReservationTotal).Error; err != nil {
			return report, err
		}
		if level.Reserved != int(activeReservationTotal) {
			report.Issues = append(report.Issues, ReconciliationIssue{
				IssueType:        "RESERVED_MISMATCH",
				InventoryItemID:  level.InventoryItemID,
				ProductVariantID: variantID,
				Expected:         int(activeReservationTotal),
				Actual:           level.Reserved,
				Message:          "inventory level reserved does not match active reservations",
				EntityType:       "inventory_level",
				EntityID:         &level.ID,
			})
		}

		var variant models.ProductVariant
		if err := db.First(&variant, variantID).Error; err == nil && variant.Stock != level.Available {
			report.Issues = append(report.Issues, ReconciliationIssue{
				IssueType:        "VARIANT_STOCK_MISMATCH",
				InventoryItemID:  level.InventoryItemID,
				ProductVariantID: variantID,
				Expected:         level.Available,
				Actual:           variant.Stock,
				Message:          "product variant stock does not match inventory available",
				EntityType:       "product_variant",
				EntityID:         &variant.ID,
			})
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return report, err
		}
	}

	var staleReservations []models.InventoryReservation
	if err := db.Where("status = ? AND expires_at < ?", models.InventoryReservationStatusActive, checkedAt).
		Order("expires_at ASC, id ASC").
		Limit(200).
		Find(&staleReservations).Error; err != nil {
		return report, err
	}
	for _, reservation := range staleReservations {
		report.Issues = append(report.Issues, ReconciliationIssue{
			IssueType:        "STALE_ACTIVE_RESERVATION",
			InventoryItemID:  reservation.InventoryItemID,
			ProductVariantID: reservation.ProductVariantID,
			Expected:         0,
			Actual:           reservation.Quantity,
			Message:          "active reservation is past its expiry time",
			EntityType:       "inventory_reservation",
			EntityID:         &reservation.ID,
		})
	}

	return report, nil
}

func GetTimeline(db *gorm.DB, productVariantID uint, limit int) (InventoryTimeline, error) {
	if productVariantID == 0 {
		return InventoryTimeline{}, fmt.Errorf("product variant id is required")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	item, _, err := ensureLevel(db, productVariantID)
	if err != nil {
		return InventoryTimeline{}, err
	}
	timeline := InventoryTimeline{ProductVariantID: productVariantID}
	if err := db.Where("inventory_item_id = ?", item.ID).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&timeline.Movements).Error; err != nil {
		return timeline, err
	}
	if err := db.Where("inventory_item_id = ?", item.ID).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&timeline.Reservations).Error; err != nil {
		return timeline, err
	}
	if err := db.Where("inventory_item_id = ?", item.ID).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&timeline.Adjustments).Error; err != nil {
		return timeline, err
	}
	return timeline, nil
}

func GetThresholds(db *gorm.DB, productVariantID *uint) ([]models.InventoryThreshold, error) {
	query := db.Order("product_variant_id IS NOT NULL, product_variant_id ASC, id ASC")
	if productVariantID != nil {
		query = query.Where("product_variant_id = ? OR product_variant_id IS NULL", *productVariantID)
	}
	var thresholds []models.InventoryThreshold
	if err := query.Find(&thresholds).Error; err != nil {
		return nil, err
	}
	return thresholds, nil
}

func SetThreshold(db *gorm.DB, input ThresholdInput) (models.InventoryThreshold, error) {
	if input.LowStockQuantity < 0 {
		return models.InventoryThreshold{}, fmt.Errorf("low stock quantity cannot be negative")
	}
	var threshold models.InventoryThreshold
	err := db.Transaction(func(tx *gorm.DB) error {
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"})
		if input.ProductVariantID == nil {
			query = query.Where("product_variant_id IS NULL")
		} else {
			query = query.Where("product_variant_id = ?", *input.ProductVariantID)
		}
		err := query.First(&threshold).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			threshold = models.InventoryThreshold{
				ProductVariantID: input.ProductVariantID,
			}
		}
		threshold.LowStockQuantity = input.LowStockQuantity
		if threshold.ID == 0 {
			return tx.Create(&threshold).Error
		}
		return tx.Model(&threshold).Update("low_stock_quantity", input.LowStockQuantity).Error
	})
	return threshold, err
}

func DeleteThreshold(db *gorm.DB, thresholdID uint) error {
	if thresholdID == 0 {
		return fmt.Errorf("threshold id is required")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var threshold models.InventoryThreshold
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&threshold, thresholdID).Error; err != nil {
			return err
		}
		return tx.Delete(&threshold).Error
	})
}

func StartReservationExpiryWorker(ctx context.Context, db *gorm.DB, interval time.Duration, logger *log.Logger) {
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				expired, err := ExpireReservations(db, time.Now().UTC(), 100)
				if err != nil {
					if logger != nil {
						logger.Printf("[ERROR] Inventory reservation expiry failed: %v", err)
					}
					continue
				}
				if expired > 0 && logger != nil {
					logger.Printf("[INFO] Inventory reservation expiry released=%d", expired)
				}
			}
		}
	}()
}

func ApplyMovement(db *gorm.DB, input MovementInput) (Availability, error) {
	if input.ProductVariantID == 0 {
		return Availability{}, fmt.Errorf("product variant id is required")
	}
	if input.MovementType == "" {
		return Availability{}, fmt.Errorf("movement type is required")
	}
	if input.QuantityDelta == 0 {
		return Availability{}, fmt.Errorf("quantity delta is required")
	}

	var availability Availability
	err := db.Transaction(func(tx *gorm.DB) error {
		item, level, err := lockedLevel(tx, input.ProductVariantID)
		if err != nil {
			return err
		}

		nextOnHand := level.OnHand + input.QuantityDelta
		nextReserved := level.Reserved
		nextAvailable := nextOnHand - nextReserved
		if nextOnHand < 0 {
			return &InsufficientAvailabilityError{
				ProductVariantID: input.ProductVariantID,
				Requested:        -input.QuantityDelta,
				Available:        level.Available,
			}
		}
		if nextReserved < 0 {
			return fmt.Errorf("inventory reserved cannot be negative")
		}
		if nextAvailable != nextOnHand-nextReserved {
			return fmt.Errorf("inventory availability invariant failed")
		}
		if nextAvailable < 0 {
			return &InsufficientAvailabilityError{
				ProductVariantID: input.ProductVariantID,
				Requested:        -input.QuantityDelta,
				Available:        level.Available,
			}
		}

		movement := models.InventoryMovement{
			InventoryItemID: item.ID,
			MovementType:    input.MovementType,
			QuantityDelta:   input.QuantityDelta,
			ReferenceType:   input.ReferenceType,
			ReferenceID:     input.ReferenceID,
			ReasonCode:      input.ReasonCode,
			ActorType:       input.ActorType,
			ActorID:         input.ActorID,
		}
		if err := tx.Create(&movement).Error; err != nil {
			return err
		}

		availability = Availability{
			ProductVariantID: input.ProductVariantID,
			OnHand:           nextOnHand,
			Reserved:         nextReserved,
			Available:        nextAvailable,
		}
		return updateLevelAndVariant(tx, level.ID, input.ProductVariantID, availability)
	})
	return availability, err
}

func SetOnHand(db *gorm.DB, productVariantID uint, onHand int, input MovementInput) (Availability, error) {
	if productVariantID == 0 {
		return Availability{}, fmt.Errorf("product variant id is required")
	}
	if onHand < 0 {
		return Availability{}, fmt.Errorf("on hand cannot be negative")
	}
	movementType := strings.TrimSpace(input.MovementType)
	if movementType == "" {
		movementType = MovementTypeAdminSync
	}

	var availability Availability
	err := db.Transaction(func(tx *gorm.DB) error {
		item, level, err := lockedLevel(tx, productVariantID)
		if err != nil {
			return err
		}

		nextAvailable := onHand - level.Reserved
		if nextAvailable < 0 {
			return &InsufficientAvailabilityError{
				ProductVariantID: productVariantID,
				Requested:        level.Reserved,
				Available:        level.Available,
			}
		}
		delta := onHand - level.OnHand
		if delta != 0 {
			movement := models.InventoryMovement{
				InventoryItemID: item.ID,
				MovementType:    movementType,
				QuantityDelta:   delta,
				ReferenceType:   input.ReferenceType,
				ReferenceID:     input.ReferenceID,
				ReasonCode:      input.ReasonCode,
				ActorType:       input.ActorType,
				ActorID:         input.ActorID,
			}
			if err := tx.Create(&movement).Error; err != nil {
				return err
			}
		}

		availability = Availability{
			ProductVariantID: productVariantID,
			OnHand:           onHand,
			Reserved:         level.Reserved,
			Available:        nextAvailable,
		}
		return updateLevelAndVariant(tx, level.ID, productVariantID, availability)
	})
	return availability, err
}

type InsufficientAvailabilityError struct {
	ProductVariantID uint
	Requested        int
	Available        int
}

func (e *InsufficientAvailabilityError) Error() string {
	return "insufficient inventory availability"
}

func ensureLevel(db *gorm.DB, productVariantID uint) (models.InventoryItem, models.InventoryLevel, error) {
	if productVariantID == 0 {
		return models.InventoryItem{}, models.InventoryLevel{}, fmt.Errorf("product variant id is required")
	}

	var item models.InventoryItem
	err := db.Where("product_variant_id = ?", productVariantID).First(&item).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return models.InventoryItem{}, models.InventoryLevel{}, err
		}
		var variant models.ProductVariant
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&variant, productVariantID).Error; err != nil {
			return models.InventoryItem{}, models.InventoryLevel{}, err
		}
		item = models.InventoryItem{ProductVariantID: productVariantID}
		if err := db.Create(&item).Error; err != nil {
			return models.InventoryItem{}, models.InventoryLevel{}, err
		}
		level := models.InventoryLevel{
			InventoryItemID: item.ID,
			OnHand:          variant.Stock,
			Reserved:        0,
			Available:       variant.Stock,
		}
		if err := db.Create(&level).Error; err != nil {
			return models.InventoryItem{}, models.InventoryLevel{}, err
		}
		return item, level, nil
	}

	var level models.InventoryLevel
	if err := db.Where("inventory_item_id = ?", item.ID).First(&level).Error; err != nil {
		return models.InventoryItem{}, models.InventoryLevel{}, err
	}
	return item, level, nil
}

func lockedLevel(tx *gorm.DB, productVariantID uint) (models.InventoryItem, models.InventoryLevel, error) {
	item, level, err := ensureLevel(tx, productVariantID)
	if err != nil {
		return models.InventoryItem{}, models.InventoryLevel{}, err
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&level, level.ID).Error; err != nil {
		return models.InventoryItem{}, models.InventoryLevel{}, err
	}
	return item, level, nil
}

func updateLevelAndVariant(tx *gorm.DB, levelID uint, productVariantID uint, availability Availability) error {
	if availability.OnHand < 0 || availability.Reserved < 0 || availability.Available != availability.OnHand-availability.Reserved {
		return fmt.Errorf("inventory availability invariant failed")
	}
	var level models.InventoryLevel
	if err := tx.First(&level, levelID).Error; err != nil {
		return err
	}
	updates := map[string]any{
		"on_hand":   availability.OnHand,
		"reserved":  availability.Reserved,
		"available": availability.Available,
	}
	if err := tx.Model(&models.InventoryLevel{}).Where("id = ?", levelID).Updates(updates).Error; err != nil {
		return err
	}
	if err := tx.Model(&models.ProductVariant{}).Where("id = ?", productVariantID).Update("stock", availability.Available).Error; err != nil {
		return err
	}
	level.OnHand = availability.OnHand
	level.Reserved = availability.Reserved
	level.Available = availability.Available
	return reconcileAlerts(tx, level.InventoryItemID, productVariantID, availability.Available)
}

func updateAlertStatus(db *gorm.DB, alertID uint, input AlertActionInput, status string) (models.InventoryAlert, error) {
	if alertID == 0 {
		return models.InventoryAlert{}, fmt.Errorf("alert id is required")
	}
	var alert models.InventoryAlert
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&alert, alertID).Error; err != nil {
			return err
		}
		if alert.Status == status {
			return nil
		}
		if alert.Status == models.InventoryAlertStatusResolved {
			return nil
		}
		now := time.Now().UTC()
		updates := map[string]any{"status": status, "updated_at": now}
		switch status {
		case models.InventoryAlertStatusAcked:
			updates["acked_at"] = now
			updates["acked_by_type"] = strings.TrimSpace(input.ActorType)
			updates["acked_by_id"] = input.ActorID
		case models.InventoryAlertStatusResolved:
			updates["resolved_at"] = now
			updates["resolved_by_type"] = strings.TrimSpace(input.ActorType)
			updates["resolved_by_id"] = input.ActorID
		default:
			return fmt.Errorf("unsupported alert status")
		}
		if err := tx.Model(&models.InventoryAlert{}).Where("id = ?", alert.ID).Updates(updates).Error; err != nil {
			return err
		}
		return tx.First(&alert, alert.ID).Error
	})
	return alert, err
}

func reconcileAlerts(tx *gorm.DB, inventoryItemID uint, productVariantID uint, available int) error {
	threshold, err := effectiveThreshold(tx, productVariantID)
	if err != nil {
		return err
	}
	nextType := ""
	if available <= 0 {
		nextType = models.InventoryAlertTypeOutOfStock
	} else if available <= threshold {
		nextType = models.InventoryAlertTypeLowStock
	}

	now := time.Now().UTC()
	if nextType == "" {
		resolved, err := resolveOpenStockAlerts(tx, productVariantID, now, "inventory_recovery", nil)
		if err != nil {
			return err
		}
		if resolved {
			return createRecoveryAlert(tx, inventoryItemID, productVariantID, available, threshold, now)
		}
		return nil
	}

	if err := resolveOtherStockAlerts(tx, productVariantID, nextType, now); err != nil {
		return err
	}
	var existing int64
	if err := tx.Model(&models.InventoryAlert{}).
		Where("product_variant_id = ? AND alert_type = ? AND status IN ?", productVariantID, nextType, []string{models.InventoryAlertStatusOpen, models.InventoryAlertStatusAcked}).
		Count(&existing).Error; err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	return tx.Create(&models.InventoryAlert{
		InventoryItemID:  inventoryItemID,
		ProductVariantID: productVariantID,
		AlertType:        nextType,
		Status:           models.InventoryAlertStatusOpen,
		Available:        available,
		Threshold:        threshold,
		OpenedAt:         now,
	}).Error
}

func effectiveThreshold(tx *gorm.DB, productVariantID uint) (int, error) {
	var threshold models.InventoryThreshold
	err := tx.Where("product_variant_id = ?", productVariantID).First(&threshold).Error
	if err == nil {
		return threshold.LowStockQuantity, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	err = tx.Where("product_variant_id IS NULL").First(&threshold).Error
	if err == nil {
		return threshold.LowStockQuantity, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	return 5, nil
}

func resolveOtherStockAlerts(tx *gorm.DB, productVariantID uint, keepType string, now time.Time) error {
	return tx.Model(&models.InventoryAlert{}).
		Where("product_variant_id = ? AND alert_type IN ? AND alert_type <> ? AND status IN ?", productVariantID, []string{models.InventoryAlertTypeLowStock, models.InventoryAlertTypeOutOfStock}, keepType, []string{models.InventoryAlertStatusOpen, models.InventoryAlertStatusAcked}).
		Updates(map[string]any{
			"status":           models.InventoryAlertStatusResolved,
			"resolved_at":      now,
			"resolved_by_type": "inventory_state_change",
			"updated_at":       now,
		}).Error
}

func resolveOpenStockAlerts(tx *gorm.DB, productVariantID uint, now time.Time, actorType string, actorID *uint) (bool, error) {
	result := tx.Model(&models.InventoryAlert{}).
		Where("product_variant_id = ? AND alert_type IN ? AND status IN ?", productVariantID, []string{models.InventoryAlertTypeLowStock, models.InventoryAlertTypeOutOfStock}, []string{models.InventoryAlertStatusOpen, models.InventoryAlertStatusAcked}).
		Updates(map[string]any{
			"status":           models.InventoryAlertStatusResolved,
			"resolved_at":      now,
			"resolved_by_type": actorType,
			"resolved_by_id":   actorID,
			"updated_at":       now,
		})
	return result.RowsAffected > 0, result.Error
}

func createRecoveryAlert(tx *gorm.DB, inventoryItemID uint, productVariantID uint, available int, threshold int, now time.Time) error {
	var recent int64
	if err := tx.Model(&models.InventoryAlert{}).
		Where("product_variant_id = ? AND alert_type = ? AND opened_at >= ?", productVariantID, models.InventoryAlertTypeRecovery, now.Add(-time.Hour)).
		Count(&recent).Error; err != nil {
		return err
	}
	if recent > 0 {
		return nil
	}
	return tx.Create(&models.InventoryAlert{
		InventoryItemID:  inventoryItemID,
		ProductVariantID: productVariantID,
		AlertType:        models.InventoryAlertTypeRecovery,
		Status:           models.InventoryAlertStatusResolved,
		Available:        available,
		Threshold:        threshold,
		OpenedAt:         now,
		ResolvedAt:       &now,
		ResolvedByType:   "inventory_recovery",
	}).Error
}

func validAdjustmentReasons() []string {
	return []string{
		models.InventoryAdjustmentReasonCycleCountGain,
		models.InventoryAdjustmentReasonCycleCountLoss,
		models.InventoryAdjustmentReasonDamage,
		models.InventoryAdjustmentReasonShrinkage,
		models.InventoryAdjustmentReasonReturnRestock,
		models.InventoryAdjustmentReasonCorrection,
	}
}

func closeReservationsForOrder(tx *gorm.DB, orderID uint, idempotencyKey string, status string) (bool, error) {
	if orderID == 0 {
		return false, fmt.Errorf("order id is required")
	}
	if status != models.InventoryReservationStatusConsumed && status != models.InventoryReservationStatusReleased {
		return false, fmt.Errorf("unsupported reservation status transition")
	}

	var reservations []models.InventoryReservation
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("order_id = ? AND status = ?", orderID, models.InventoryReservationStatusActive).
		Order("id ASC").
		Find(&reservations).Error; err != nil {
		return false, err
	}
	if len(reservations) == 0 {
		var closedCount int64
		if err := tx.Model(&models.InventoryReservation{}).
			Where("order_id = ? AND status = ?", orderID, status).
			Count(&closedCount).Error; err != nil {
			return false, err
		}
		return closedCount > 0, nil
	}

	now := time.Now().UTC()
	for _, reservation := range reservations {
		switch status {
		case models.InventoryReservationStatusConsumed:
			if err := consumeReservation(tx, &reservation, strings.TrimSpace(idempotencyKey), now); err != nil {
				return false, err
			}
		case models.InventoryReservationStatusReleased:
			if err := releaseReservation(tx, &reservation, status, now); err != nil {
				return false, err
			}
		}
	}
	return true, nil
}

func consumeReservation(tx *gorm.DB, reservation *models.InventoryReservation, idempotencyKey string, now time.Time) error {
	_, level, err := lockedLevel(tx, reservation.ProductVariantID)
	if err != nil {
		return err
	}
	nextOnHand := level.OnHand - reservation.Quantity
	nextReserved := level.Reserved - reservation.Quantity
	nextAvailable := nextOnHand - nextReserved
	if nextOnHand < 0 || nextReserved < 0 || nextAvailable < 0 {
		return &InsufficientAvailabilityError{
			ProductVariantID: reservation.ProductVariantID,
			Requested:        reservation.Quantity,
			Available:        level.Available,
		}
	}

	referenceID := reservation.ID
	movement := models.InventoryMovement{
		InventoryItemID: reservation.InventoryItemID,
		MovementType:    MovementTypeOrderCommit,
		QuantityDelta:   -reservation.Quantity,
		ReferenceType:   ReferenceTypeReservation,
		ReferenceID:     &referenceID,
		ReasonCode:      "reservation_consumed",
		ActorType:       ReferenceTypeOrder,
	}
	if strings.TrimSpace(idempotencyKey) != "" {
		movement.ReasonCode = "reservation_consumed:" + strings.TrimSpace(idempotencyKey)
	}
	if err := tx.Create(&movement).Error; err != nil {
		return err
	}
	timestamp := now.UTC()
	if err := tx.Model(&models.InventoryReservation{}).
		Where("id = ? AND status = ?", reservation.ID, models.InventoryReservationStatusActive).
		Updates(map[string]any{
			"status":      models.InventoryReservationStatusConsumed,
			"consumed_at": timestamp,
			"updated_at":  timestamp,
		}).Error; err != nil {
		return err
	}
	return updateLevelAndVariant(tx, level.ID, reservation.ProductVariantID, Availability{
		ProductVariantID: reservation.ProductVariantID,
		OnHand:           nextOnHand,
		Reserved:         nextReserved,
		Available:        nextAvailable,
	})
}

func releaseReservation(tx *gorm.DB, reservation *models.InventoryReservation, status string, now time.Time) error {
	if !slices.Contains([]string{models.InventoryReservationStatusReleased, models.InventoryReservationStatusExpired}, status) {
		return fmt.Errorf("unsupported reservation release status")
	}
	_, level, err := lockedLevel(tx, reservation.ProductVariantID)
	if err != nil {
		return err
	}
	nextReserved := level.Reserved - reservation.Quantity
	nextAvailable := level.OnHand - nextReserved
	if nextReserved < 0 || nextAvailable < 0 {
		return fmt.Errorf("inventory reserved cannot be negative")
	}
	timestamp := now.UTC()
	updates := map[string]any{
		"status":     status,
		"updated_at": timestamp,
	}
	if status == models.InventoryReservationStatusExpired {
		updates["expired_at"] = timestamp
	} else {
		updates["released_at"] = timestamp
	}
	if err := tx.Model(&models.InventoryReservation{}).
		Where("id = ? AND status = ?", reservation.ID, models.InventoryReservationStatusActive).
		Updates(updates).Error; err != nil {
		return err
	}
	return updateLevelAndVariant(tx, level.ID, reservation.ProductVariantID, Availability{
		ProductVariantID: reservation.ProductVariantID,
		OnHand:           level.OnHand,
		Reserved:         nextReserved,
		Available:        nextAvailable,
	})
}
