package tax

import (
	"context"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
)

// Service provides context-first tax application APIs. Provider execution
// remains outside these database preparation/finalization methods.
type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) PrepareFinalization(ctx context.Context, input FinalizeInput) (TaxFinalizationPlan, error) {
	return PrepareTaxFinalization(s.db.WithContext(ctx), input)
}

func (s *Service) PersistFinalization(ctx context.Context, input FinalizeInput, result TaxFinalized, now time.Time) (TaxFinalized, error) {
	return PersistTaxFinalization(s.db.WithContext(ctx), input, result, now)
}

func (s *Service) LoadOrderLines(ctx context.Context, orderID uint) ([]models.OrderTaxLine, error) {
	return LoadOrderTaxLines(s.db.WithContext(ctx), orderID)
}
