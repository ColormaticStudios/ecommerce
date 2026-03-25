package shipping

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"ecommerce/internal/dbcontext"
	orderservice "ecommerce/internal/services/orders"
	paymentservice "ecommerce/internal/services/payments"
	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrShipmentRateNotFound     = errors.New("shipment rate not found")
	ErrShipmentNotFound         = errors.New("shipment not found")
	ErrShipmentServiceImmutable = errors.New("shipment service is immutable once finalized")
)

const pendingShipmentProviderIDPrefix = "pending:"

func QuoteAndPersistRates(
	ctx context.Context,
	tx *gorm.DB,
	registry ProviderRegistry,
	order models.Order,
	snapshot models.OrderCheckoutSnapshot,
	now time.Time,
) ([]models.ShipmentRate, error) {
	ctx = dbcontext.WithDB(ctx, tx)

	var existingShipment models.Shipment
	err := tx.Preload("Rates").
		Where("order_id = ? AND snapshot_id = ? AND provider = ?", order.ID, snapshot.ID, snapshot.ShippingProviderID).
		First(&existingShipment).Error
	if err == nil && existingShipment.FinalizedAt != nil {
		if len(existingShipment.Rates) > 0 {
			return existingShipment.Rates, nil
		}
		var selectedRate models.ShipmentRate
		if rateErr := tx.Where("id = ?", existingShipment.ShipmentRateID).First(&selectedRate).Error; rateErr == nil {
			return []models.ShipmentRate{selectedRate}, nil
		}
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	provider, err := registry.Provider(snapshot.ShippingProviderID)
	if err != nil {
		return nil, err
	}

	shippingData, err := unmarshalStringMap(snapshot.ShippingDataJSON)
	if err != nil {
		return nil, err
	}
	rates, err := provider.QuoteRates(ctx, QuoteRatesRequest{
		OrderID:               order.ID,
		SnapshotID:            snapshot.ID,
		Currency:              snapshot.Currency,
		ShippingAddressPretty: snapshot.ShippingAddressPretty,
		ShippingAmount:        snapshot.ShippingAmount,
		ShippingData:          shippingData,
	})
	if err != nil {
		return nil, err
	}

	if err := tx.Where("order_id = ? AND snapshot_id = ? AND provider = ? AND shipment_id IS NULL", order.ID, snapshot.ID, snapshot.ShippingProviderID).
		Delete(&models.ShipmentRate{}).Error; err != nil {
		return nil, err
	}

	selectedServiceCode := selectedServiceCode(snapshot.ShippingProviderID, shippingData)
	modelRates := make([]models.ShipmentRate, 0, len(rates))
	selectedIndex := -1
	for i, rate := range rates {
		selected := rate.ServiceCode == selectedServiceCode
		if !selected && selectedServiceCode == "" && rate.Amount == snapshot.ShippingAmount {
			selected = true
		}
		if selected && selectedIndex == -1 {
			selectedIndex = i
		}
		modelRates = append(modelRates, models.ShipmentRate{
			OrderID:        order.ID,
			SnapshotID:     snapshot.ID,
			Provider:       snapshot.ShippingProviderID,
			ProviderRateID: rate.ProviderRateID,
			ServiceCode:    rate.ServiceCode,
			ServiceName:    rate.ServiceName,
			Amount:         rate.Amount,
			Currency:       rate.Currency,
			Selected:       selected,
			ExpiresAt:      rate.ExpiresAt,
		})
	}
	if len(modelRates) > 0 && selectedIndex == -1 {
		modelRates[0].Selected = true
	}
	if len(modelRates) > 0 {
		if err := tx.Create(&modelRates).Error; err != nil {
			return nil, err
		}
	}
	return modelRates, nil
}

func PurchaseLabel(
	ctx context.Context,
	db *gorm.DB,
	registry ProviderRegistry,
	orderID uint,
	rateID uint,
	pkg PackageInput,
	idempotencyKey string,
	correlationID string,
	now time.Time,
) (models.Shipment, error) {
	var (
		preparedShipment models.Shipment
		rate             models.ShipmentRate
		snapshot         models.OrderCheckoutSnapshot
		effectiveKey     string
		alreadyDone      bool
	)
	if err := db.Transaction(func(tx *gorm.DB) error {
		var prepErr error
		preparedShipment, rate, snapshot, effectiveKey, alreadyDone, prepErr = prepareLabelPurchase(
			tx,
			orderID,
			rateID,
			pkg,
			idempotencyKey,
		)
		return prepErr
	}); err != nil {
		return models.Shipment{}, err
	}
	if alreadyDone {
		return preparedShipment, nil
	}

	provider, err := registry.Provider(rate.Provider)
	if err != nil {
		return models.Shipment{}, err
	}
	providerShipment, err := provider.BuyLabel(ctx, BuyLabelRequest{
		OrderID:               orderID,
		SnapshotID:            snapshot.ID,
		Provider:              rate.Provider,
		Rate:                  rate,
		ShippingAddressPretty: snapshot.ShippingAddressPretty,
		Package:               pkg,
		IdempotencyKey:        effectiveKey,
		CorrelationID:         correlationID,
	})
	if err != nil {
		return models.Shipment{}, err
	}

	var finalized models.Shipment
	if err := db.Transaction(func(tx *gorm.DB) error {
		var finalizeErr error
		finalized, finalizeErr = finalizeLabelPurchase(tx, preparedShipment.ID, providerShipment, now)
		return finalizeErr
	}); err != nil {
		return models.Shipment{}, err
	}
	return finalized, nil
}

func GetShipment(db *gorm.DB, shipmentID uint) (models.Shipment, error) {
	var shipment models.Shipment
	err := db.Preload("Rates", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at ASC, id ASC")
	}).
		Preload("Packages", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC, id ASC")
		}).
		Preload("TrackingEvents", func(db *gorm.DB) *gorm.DB {
			return db.Order("occurred_at ASC, id ASC")
		}).
		First(&shipment, shipmentID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Shipment{}, ErrShipmentNotFound
	}
	if err != nil {
		return models.Shipment{}, err
	}
	return shipment, nil
}

func GetOrderShipments(db *gorm.DB, orderID uint) ([]models.Shipment, error) {
	var shipments []models.Shipment
	if err := db.Preload("Rates", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at ASC, id ASC")
	}).
		Preload("Packages", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC, id ASC")
		}).
		Preload("TrackingEvents", func(db *gorm.DB) *gorm.DB {
			return db.Order("occurred_at ASC, id ASC")
		}).
		Where("order_id = ?", orderID).
		Order("created_at ASC, id ASC").
		Find(&shipments).Error; err != nil {
		return nil, err
	}
	return shipments, nil
}

func ApplyTrackingEvent(
	tx *gorm.DB,
	event TrackingWebhookEvent,
	correlationID string,
) (models.Shipment, bool, error) {
	var shipment models.Shipment
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("provider = ? AND provider_shipment_id = ?", event.Provider, event.ProviderShipmentID).
		First(&shipment).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Shipment{}, false, ErrShipmentNotFound
		}
		return models.Shipment{}, false, err
	}

	var existing models.TrackingEvent
	err := tx.Where("shipment_id = ? AND provider = ? AND provider_event_id = ?", shipment.ID, event.Provider, event.ProviderEventID).
		First(&existing).Error
	if err == nil {
		loaded, loadErr := GetShipment(tx, shipment.ID)
		return loaded, true, loadErr
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Shipment{}, false, err
	}

	record := models.TrackingEvent{
		ShipmentID:      shipment.ID,
		Provider:        event.Provider,
		ProviderEventID: event.ProviderEventID,
		Status:          event.Status,
		TrackingNumber:  event.TrackingNumber,
		Location:        event.Location,
		Description:     event.Description,
		OccurredAt:      event.OccurredAt.UTC(),
		RawPayload:      event.RawPayload,
	}
	if err := tx.Create(&record).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			loaded, loadErr := GetShipment(tx, shipment.ID)
			return loaded, true, loadErr
		}
		return models.Shipment{}, false, err
	}

	updates := map[string]any{
		"status":          strings.ToUpper(strings.TrimSpace(event.Status)),
		"tracking_number": event.TrackingNumber,
		"tracking_url":    shipment.TrackingURL,
	}
	if strings.EqualFold(event.Status, models.ShipmentStatusDelivered) {
		updates["delivered_at"] = event.OccurredAt.UTC()
	}
	if err := tx.Model(&models.Shipment{}).Where("id = ?", shipment.ID).Updates(updates).Error; err != nil {
		return models.Shipment{}, false, err
	}

	var order models.Order
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, shipment.OrderID).Error; err != nil {
		return models.Shipment{}, false, err
	}

	targetStatus := ""
	reason := ""
	switch strings.ToUpper(strings.TrimSpace(event.Status)) {
	case models.ShipmentStatusInTransit:
		if order.Status != models.StatusShipped && order.Status != models.StatusDelivered {
			targetStatus = models.StatusShipped
			reason = "tracking_in_transit"
		}
	case models.ShipmentStatusDelivered:
		if order.Status != models.StatusDelivered {
			targetStatus = models.StatusDelivered
			reason = "tracking_delivered"
		}
	}
	if targetStatus != "" {
		fromStatus := order.Status
		if err := orderservice.ApplyStatusTransition(tx, &order, targetStatus); err != nil {
			return models.Shipment{}, false, err
		}
		if err := paymentservice.AppendOrderStatusHistory(
			tx,
			order.ID,
			fromStatus,
			order.Status,
			reason,
			"webhook",
			"provider:"+event.Provider,
			correlationID,
		); err != nil {
			return models.Shipment{}, false, err
		}
	}

	loaded, err := GetShipment(tx, shipment.ID)
	return loaded, false, err
}

func selectedServiceCode(providerID string, shippingData map[string]string) string {
	switch strings.TrimSpace(providerID) {
	case "dummy-ground":
		return strings.ToLower(strings.TrimSpace(shippingData["service_level"]))
	case "dummy-pickup":
		return "pickup"
	default:
		return ""
	}
}

func prepareLabelPurchase(
	tx *gorm.DB,
	orderID uint,
	rateID uint,
	pkg PackageInput,
	idempotencyKey string,
) (models.Shipment, models.ShipmentRate, models.OrderCheckoutSnapshot, string, bool, error) {
	var rate models.ShipmentRate
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND order_id = ?", rateID, orderID).
		First(&rate).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Shipment{}, models.ShipmentRate{}, models.OrderCheckoutSnapshot{}, "", false, ErrShipmentRateNotFound
		}
		return models.Shipment{}, models.ShipmentRate{}, models.OrderCheckoutSnapshot{}, "", false, err
	}

	var existing models.Shipment
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("order_id = ? AND snapshot_id = ? AND provider = ?", orderID, rate.SnapshotID, rate.Provider).
		First(&existing).Error
	if err == nil {
		if existing.ShipmentRateID != rate.ID {
			return models.Shipment{}, models.ShipmentRate{}, models.OrderCheckoutSnapshot{}, "", false, ErrShipmentServiceImmutable
		}
		if existing.FinalizedAt != nil {
			loaded, loadErr := GetShipment(tx, existing.ID)
			return loaded, rate, models.OrderCheckoutSnapshot{}, "", true, loadErr
		}
		var snapshot models.OrderCheckoutSnapshot
		if err := tx.First(&snapshot, rate.SnapshotID).Error; err != nil {
			return models.Shipment{}, models.ShipmentRate{}, models.OrderCheckoutSnapshot{}, "", false, err
		}
		return existing, rate, snapshot, pendingShipmentIdempotencyKey(existing.ProviderShipmentID, idempotencyKey), false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Shipment{}, models.ShipmentRate{}, models.OrderCheckoutSnapshot{}, "", false, err
	}

	var snapshot models.OrderCheckoutSnapshot
	if err := tx.First(&snapshot, rate.SnapshotID).Error; err != nil {
		return models.Shipment{}, models.ShipmentRate{}, models.OrderCheckoutSnapshot{}, "", false, err
	}

	draft := models.Shipment{
		OrderID:               orderID,
		SnapshotID:            snapshot.ID,
		Provider:              rate.Provider,
		ShipmentRateID:        rate.ID,
		ProviderShipmentID:    pendingShipmentProviderID(idempotencyKey),
		Status:                models.ShipmentStatusQuoted,
		Currency:              rate.Currency,
		ServiceCode:           rate.ServiceCode,
		ServiceName:           rate.ServiceName,
		Amount:                rate.Amount,
		ShippingAddressPretty: snapshot.ShippingAddressPretty,
	}
	if err := tx.Create(&draft).Error; err != nil {
		return models.Shipment{}, models.ShipmentRate{}, models.OrderCheckoutSnapshot{}, "", false, err
	}

	if err := tx.Model(&models.ShipmentRate{}).
		Where("order_id = ? AND snapshot_id = ? AND provider = ?", orderID, rate.SnapshotID, rate.Provider).
		Update("selected", false).Error; err != nil {
		return models.Shipment{}, models.ShipmentRate{}, models.OrderCheckoutSnapshot{}, "", false, err
	}
	if err := tx.Model(&models.ShipmentRate{}).
		Where("id = ?", rate.ID).
		Updates(map[string]any{
			"selected":    true,
			"shipment_id": draft.ID,
		}).Error; err != nil {
		return models.Shipment{}, models.ShipmentRate{}, models.OrderCheckoutSnapshot{}, "", false, err
	}

	if pkg.Reference != "" || pkg.WeightGrams > 0 || pkg.LengthCM > 0 || pkg.WidthCM > 0 || pkg.HeightCM > 0 {
		record := models.ShipmentPackage{
			ShipmentID:  draft.ID,
			Reference:   strings.TrimSpace(pkg.Reference),
			WeightGrams: pkg.WeightGrams,
			LengthCM:    pkg.LengthCM,
			WidthCM:     pkg.WidthCM,
			HeightCM:    pkg.HeightCM,
		}
		if err := tx.Create(&record).Error; err != nil {
			return models.Shipment{}, models.ShipmentRate{}, models.OrderCheckoutSnapshot{}, "", false, err
		}
	}

	return draft, rate, snapshot, pendingShipmentIdempotencyKey(draft.ProviderShipmentID, idempotencyKey), false, nil
}

func finalizeLabelPurchase(
	tx *gorm.DB,
	shipmentID uint,
	providerShipment ProviderShipment,
	now time.Time,
) (models.Shipment, error) {
	var shipment models.Shipment
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&shipment, shipmentID).Error; err != nil {
		return models.Shipment{}, err
	}
	if shipment.FinalizedAt != nil {
		return GetShipment(tx, shipment.ID)
	}

	timestamp := now.UTC()
	updates := map[string]any{
		"provider_shipment_id": providerShipment.ProviderShipmentID,
		"status":               models.ShipmentStatusLabelPurchased,
		"service_code":         providerShipment.ServiceCode,
		"service_name":         providerShipment.ServiceName,
		"tracking_number":      providerShipment.TrackingNumber,
		"tracking_url":         providerShipment.TrackingURL,
		"label_url":            providerShipment.LabelURL,
		"purchased_at":         &timestamp,
		"finalized_at":         &timestamp,
	}
	if err := tx.Model(&models.Shipment{}).
		Where("id = ?", shipment.ID).
		Updates(updates).Error; err != nil {
		return models.Shipment{}, err
	}

	return GetShipment(tx, shipment.ID)
}

func pendingShipmentProviderID(idempotencyKey string) string {
	key := sanitizeKey(idempotencyKey)
	if key == "" {
		key = "nolabelkey"
	}
	return pendingShipmentProviderIDPrefix + key
}

func pendingShipmentIdempotencyKey(providerShipmentID string, fallback string) string {
	trimmed := strings.TrimSpace(providerShipmentID)
	if strings.HasPrefix(trimmed, pendingShipmentProviderIDPrefix) {
		if key := strings.TrimPrefix(trimmed, pendingShipmentProviderIDPrefix); key != "" {
			return key
		}
	}
	key := sanitizeKey(fallback)
	if key == "" {
		return "nolabelkey"
	}
	return key
}

func unmarshalStringMap(value string) (map[string]string, error) {
	if value == "" {
		return map[string]string{}, nil
	}
	var data map[string]string
	if err := json.Unmarshal([]byte(value), &data); err != nil {
		return nil, err
	}
	if data == nil {
		data = map[string]string{}
	}
	return data, nil
}
