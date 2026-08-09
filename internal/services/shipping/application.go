package shipping

import (
	"context"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
)

// Service provides context-first shipping application APIs. Provider calls are
// deliberately separate from these database preparation/finalization methods.
type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) PrepareRateQuote(ctx context.Context, order models.Order, snapshot models.OrderCheckoutSnapshot) (RateQuotePlan, error) {
	return PrepareRateQuote(s.db.WithContext(ctx), order, snapshot)
}

func (s *Service) PersistQuotedRates(ctx context.Context, plan RateQuotePlan, rates []QuotedRate) ([]models.ShipmentRate, error) {
	var stored []models.ShipmentRate
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		stored, err = PersistQuotedRates(tx, plan, rates)
		return err
	})
	return stored, err
}

func (s *Service) PrepareLabelPurchase(ctx context.Context, orderID, rateID uint, pkg PackageInput, key string) (PreparedLabelPurchase, error) {
	return PrepareLabelPurchase(s.db.WithContext(ctx), orderID, rateID, pkg, key)
}

func (s *Service) FinalizeLabelPurchase(ctx context.Context, shipmentID uint, result ProviderShipment, now time.Time) (models.Shipment, error) {
	return FinalizeLabelPurchase(s.db.WithContext(ctx), shipmentID, result, now)
}

func (s *Service) GetShipment(ctx context.Context, shipmentID uint) (models.Shipment, error) {
	return GetShipment(s.db.WithContext(ctx), shipmentID)
}

func (s *Service) GetOrderShipments(ctx context.Context, orderID uint) ([]models.Shipment, error) {
	return GetOrderShipments(s.db.WithContext(ctx), orderID)
}
