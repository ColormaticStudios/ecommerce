package inventory

import (
	"context"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
)

// Service provides context-first inventory APIs for HTTP and CLI entry points.
type Service struct{ db *gorm.DB }

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) ListReservations(ctx context.Context, statuses []string, limit int) ([]models.InventoryReservation, error) {
	return ListReservations(s.db.WithContext(ctx), statuses, limit)
}
func (s *Service) ListAlerts(ctx context.Context, statuses []string, limit int) ([]models.InventoryAlert, error) {
	return ListAlerts(s.db.WithContext(ctx), statuses, limit)
}
func (s *Service) AckAlert(ctx context.Context, id uint, input AlertActionInput) (models.InventoryAlert, error) {
	return AckAlert(s.db.WithContext(ctx), id, input)
}
func (s *Service) ResolveAlert(ctx context.Context, id uint, input AlertActionInput) (models.InventoryAlert, error) {
	return ResolveAlert(s.db.WithContext(ctx), id, input)
}
func (s *Service) GetThresholds(ctx context.Context, variantID *uint) ([]models.InventoryThreshold, error) {
	return GetThresholds(s.db.WithContext(ctx), variantID)
}
func (s *Service) SetThreshold(ctx context.Context, input ThresholdInput) (models.InventoryThreshold, error) {
	return SetThreshold(s.db.WithContext(ctx), input)
}
func (s *Service) DeleteThreshold(ctx context.Context, id uint) error {
	return DeleteThreshold(s.db.WithContext(ctx), id)
}
func (s *Service) CreateAdjustment(ctx context.Context, input AdjustmentInput, policy AdjustmentPolicy) (models.InventoryAdjustment, Availability, error) {
	return CreateAdjustment(s.db.WithContext(ctx), input, policy)
}
func (s *Service) Reconcile(ctx context.Context, now time.Time) (ReconciliationReport, error) {
	return Reconcile(s.db.WithContext(ctx), now)
}
func (s *Service) GetTimeline(ctx context.Context, variantID uint, limit int) (InventoryTimeline, error) {
	return GetTimeline(s.db.WithContext(ctx), variantID, limit)
}
func (s *Service) ListPurchaseOrders(ctx context.Context, limit int) ([]models.PurchaseOrder, error) {
	return ListPurchaseOrders(s.db.WithContext(ctx), limit)
}
func (s *Service) CreatePurchaseOrder(ctx context.Context, input PurchaseOrderInput) (models.PurchaseOrder, error) {
	return CreatePurchaseOrder(s.db.WithContext(ctx), input)
}
func (s *Service) IssuePurchaseOrder(ctx context.Context, id uint) (models.PurchaseOrder, error) {
	return IssuePurchaseOrder(s.db.WithContext(ctx), id)
}
func (s *Service) CancelPurchaseOrder(ctx context.Context, id uint) (models.PurchaseOrder, error) {
	return CancelPurchaseOrder(s.db.WithContext(ctx), id)
}
func (s *Service) ReceivePurchaseOrder(ctx context.Context, id uint, input ReceivePurchaseOrderInput) (models.InventoryReceipt, models.PurchaseOrder, error) {
	return ReceivePurchaseOrder(s.db.WithContext(ctx), id, input)
}
