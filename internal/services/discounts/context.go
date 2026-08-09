package discounts

import (
	"context"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
)

// Service provides context-first discount and promotion APIs.
type Service struct{ db *gorm.DB }

func NewService(db *gorm.DB) *Service { return &Service{db: db} }

func (s *Service) ListCampaigns(ctx context.Context, status string) ([]models.DiscountCampaign, error) {
	return ListDiscountCampaigns(s.db.WithContext(ctx), status)
}
func (s *Service) CreateProductDiscount(ctx context.Context, input ProductDiscountInput) (models.DiscountCampaign, error) {
	return CreateProductDiscount(s.db.WithContext(ctx), input)
}
func (s *Service) UpdateProductDiscount(ctx context.Context, id uint, input ProductDiscountInput) (models.DiscountCampaign, error) {
	return UpdateProductDiscount(s.db.WithContext(ctx), id, input)
}
func (s *Service) DisableProductDiscount(ctx context.Context, id uint, actorID *uint) (models.DiscountCampaign, error) {
	return DisableProductDiscount(s.db.WithContext(ctx), id, actorID)
}
func (s *Service) ArchiveCampaign(ctx context.Context, id uint, actor string, now time.Time) (models.DiscountCampaign, error) {
	return ArchiveCampaign(s.db.WithContext(ctx), id, actor, now)
}
func (s *Service) UpsertSchedule(ctx context.Context, id uint, input ScheduleInput, now time.Time) (models.DiscountSchedule, error) {
	return UpsertSchedule(s.db.WithContext(ctx), id, input, now)
}
func (s *Service) RunLifecycle(ctx context.Context, now time.Time) (LifecycleResult, error) {
	return RunLifecycle(s.db.WithContext(ctx), now)
}
func (s *Service) ListHistory(ctx context.Context, campaignID *uint) ([]models.DiscountStateHistory, error) {
	return ListHistory(s.db.WithContext(ctx), campaignID)
}
func (s *Service) ListAudits(ctx context.Context, campaignID *uint) ([]models.DiscountCampaignAudit, error) {
	return ListCampaignAudits(s.db.WithContext(ctx), campaignID)
}
func (s *Service) CreatePromotion(ctx context.Context, input CreatePromotionInput) (models.DiscountCampaign, error) {
	return CreatePromotion(s.db.WithContext(ctx), input)
}
func (s *Service) Preview(ctx context.Context, lines []CartLine, now time.Time, options EvaluationOptions) (EvaluationResult, error) {
	return EvaluateCartWithOptions(s.db.WithContext(ctx), lines, now, options)
}
func (s *Service) Reconcile(ctx context.Context, now time.Time) (ReconciliationReport, error) {
	return RunReconciliation(s.db.WithContext(ctx), now)
}
func (s *Service) ListTemplates(ctx context.Context, activeOnly bool) ([]models.PromotionTemplate, error) {
	return ListTemplates(s.db.WithContext(ctx), activeOnly)
}
func (s *Service) CreateTemplate(ctx context.Context, input TemplateInput) (models.PromotionTemplate, error) {
	return CreateTemplate(s.db.WithContext(ctx), input)
}
func (s *Service) InstantiateTemplate(ctx context.Context, id uint, input InstantiateTemplateInput) (models.DiscountCampaign, error) {
	return InstantiateTemplate(s.db.WithContext(ctx), id, input)
}
