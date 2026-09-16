package providerops

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"ecommerce/internal/checkoutplugins"
	"ecommerce/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrInvalidCheckoutProviderUpdate = errors.New("invalid checkout provider update")

type CatalogService struct {
	db      *gorm.DB
	manager *checkoutplugins.Manager
}

func NewCatalogService(db *gorm.DB, manager *checkoutplugins.Manager) *CatalogService {
	return &CatalogService{db: db, manager: manager}
}
func (s *CatalogService) SyncSettings(ctx context.Context) error {
	if s == nil || s.db == nil || s.manager == nil || !s.db.Migrator().HasTable(&models.CheckoutProviderSetting{}) {
		return nil
	}
	var records []models.CheckoutProviderSetting
	if err := s.db.WithContext(ctx).Find(&records).Error; err != nil {
		return err
	}
	settings := make([]checkoutplugins.ProviderSetting, 0, len(records))
	for _, record := range records {
		var providerType checkoutplugins.ProviderType
		switch strings.TrimSpace(record.ProviderType) {
		case string(checkoutplugins.ProviderTypePayment):
			providerType = checkoutplugins.ProviderTypePayment
		case string(checkoutplugins.ProviderTypeShipping):
			providerType = checkoutplugins.ProviderTypeShipping
		case string(checkoutplugins.ProviderTypeTax):
			providerType = checkoutplugins.ProviderTypeTax
		default:
			continue
		}
		settings = append(settings, checkoutplugins.ProviderSetting{Type: providerType, ID: record.ProviderID, Enabled: record.Enabled})
	}
	s.manager.ReplaceSettings(settings)
	for _, setting := range s.manager.ListSettings() {
		if err := upsertCheckoutProviderSetting(ctx, s.db, setting); err != nil {
			return err
		}
	}
	return nil
}

// upsertCheckoutProviderSetting uses a map so explicit false values bypass
// GORM's struct defaults and both insert and conflict paths use one statement.
func upsertCheckoutProviderSetting(ctx context.Context, db *gorm.DB, setting checkoutplugins.ProviderSetting) error {
	now := time.Now().UTC()
	return db.WithContext(ctx).Model(&models.CheckoutProviderSetting{}).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "provider_type"}, {Name: "provider_id"}},
			DoUpdates: clause.Assignments(map[string]any{"enabled": setting.Enabled}),
		}).Create(map[string]any{
		"provider_type": string(setting.Type), "provider_id": setting.ID,
		"enabled": setting.Enabled, "created_at": now, "updated_at": now,
	}).Error
}

func (s *CatalogService) List(_ context.Context, admin bool) (payment, shipping, tax []checkoutplugins.Definition, err error) {
	if s == nil || s.manager == nil {
		return nil, nil, nil, errors.New("checkout plugin catalog is not configured")
	}
	if admin {
		payment, shipping, tax = s.manager.ListForAdmin()
	} else {
		payment, shipping, tax = s.manager.List()
	}
	return payment, shipping, tax, nil
}
func (s *CatalogService) SetEnabled(ctx context.Context, providerType checkoutplugins.ProviderType, providerID string, enabled bool) error {
	if s == nil || s.manager == nil {
		return errors.New("checkout plugin catalog is not configured")
	}
	if err := s.manager.SetProviderEnabled(providerType, providerID, enabled); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidCheckoutProviderUpdate, err)
	}
	if s.db == nil || !s.db.Migrator().HasTable(&models.CheckoutProviderSetting{}) {
		return nil
	}
	for _, setting := range s.manager.ListSettings() {
		if err := upsertCheckoutProviderSetting(ctx, s.db, setting); err != nil {
			return err
		}
	}
	return nil
}
