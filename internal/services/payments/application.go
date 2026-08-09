package payments

import (
	"context"

	"ecommerce/models"

	"gorm.io/gorm"
)

// Service provides context-first payment application APIs. Transaction-level
// helpers remain package functions and accept the caller's *gorm.DB explicitly.
type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) CreateCheckoutSnapshot(ctx context.Context, input CreateCheckoutSnapshotInput) (models.OrderCheckoutSnapshot, error) {
	return CreateCheckoutSnapshot(s.db.WithContext(ctx), input)
}

func (s *Service) GetCheckoutSnapshotForSession(ctx context.Context, checkoutSessionID, snapshotID uint) (models.OrderCheckoutSnapshot, error) {
	return GetCheckoutSnapshotForSession(s.db.WithContext(ctx), checkoutSessionID, snapshotID)
}

func (s *Service) GetOrderPaymentLedger(ctx context.Context, orderID uint) ([]models.PaymentIntent, error) {
	return GetOrderPaymentLedger(s.db.WithContext(ctx), orderID)
}
