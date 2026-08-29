package localization

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"ecommerce/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrLocalizedEntityNotFound = errors.New("localized entity not found")

type EntityType string

const (
	EntityTypeProduct            EntityType = "product"
	EntityTypeProductVariant     EntityType = "product_variant"
	EntityTypeProductOption      EntityType = "product_option"
	EntityTypeProductOptionValue EntityType = "product_option_value"
	EntityTypeBrand              EntityType = "brand"
	EntityTypeCategory           EntityType = "category"
	EntityTypeWebsiteSettings    EntityType = "website_settings"
	EntityTypeSEOMetadata        EntityType = "seo_metadata"
)

type EntityLocalization struct {
	Locale string
	Fields map[string]string
}

type EntityResolution struct {
	RequestedLocale string
	ResolvedLocale  string
	FallbackChain   []string
	Fields          map[string]string
	SourceLocales   map[string]string
	UsedFallback    bool
}

type EntityLocalizationRecord struct {
	EntityType    EntityType
	EntityID      uint
	Localizations []EntityLocalization
	Resolved      EntityResolution
}

type entityPolicy struct {
	table  string
	fields []string
}

var entityPolicies = map[EntityType]entityPolicy{
	EntityTypeProduct:            {table: "products", fields: []string{"name", "subtitle", "description"}},
	EntityTypeProductVariant:     {table: "product_variants", fields: []string{"title"}},
	EntityTypeProductOption:      {table: "product_options", fields: []string{"name"}},
	EntityTypeProductOptionValue: {table: "product_option_values", fields: []string{"value"}},
	EntityTypeBrand:              {table: "brands", fields: []string{"name", "description"}},
	EntityTypeCategory:           {table: "categories", fields: []string{"name", "description"}},
	EntityTypeWebsiteSettings:    {table: "website_settings", fields: []string{"site_title"}},
	EntityTypeSEOMetadata:        {table: "seo_metadata", fields: []string{"title", "description", "og_title", "og_description", "twitter_title", "twitter_description"}},
}

func ParseEntityType(value string) (EntityType, error) {
	entityType := EntityType(strings.TrimSpace(value))
	if _, ok := entityPolicies[entityType]; !ok {
		return "", ErrInvalidTranslation
	}
	return entityType, nil
}

func (s *Service) PutEntityLocalization(ctx context.Context, entityType EntityType, entityID uint, localeCode string, fields map[string]string, actorID *uint) (EntityLocalizationRecord, error) {
	policy, ok := entityPolicies[entityType]
	if !ok || entityID == 0 || len(fields) == 0 {
		return EntityLocalizationRecord{}, ErrInvalidTranslation
	}
	locale, err := s.RequireEnabledLocale(ctx, localeCode)
	if err != nil {
		return EntityLocalizationRecord{}, err
	}
	allowed := make(map[string]bool, len(policy.fields))
	for _, field := range policy.fields {
		allowed[field] = true
	}
	cleaned := make(map[string]string, len(fields))
	for field, value := range fields {
		field, value = strings.TrimSpace(field), strings.TrimSpace(value)
		if !allowed[field] {
			return EntityLocalizationRecord{}, fmt.Errorf("%w: field %s is not localizable for %s", ErrInvalidTranslation, field, entityType)
		}
		cleaned[field] = value
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireEntity(tx, policy, entityID); err != nil {
			return err
		}
		if err := tx.Where("entity_type = ? AND entity_id = ? AND locale_id = ?", entityType, entityID, locale.ID).Delete(&models.LocalizedEntityValue{}).Error; err != nil {
			return err
		}
		for _, field := range policy.fields {
			value, exists := cleaned[field]
			if !exists || value == "" {
				continue
			}
			row := models.LocalizedEntityValue{EntityType: string(entityType), EntityID: entityID, LocaleID: locale.ID, Field: field, Value: value, UpdatedBy: actorID}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "entity_type"}, {Name: "entity_id"}, {Name: "locale_id"}, {Name: "field"}},
				DoUpdates: clause.Assignments(map[string]any{
					"value": value, "updated_by": actorID, "deleted_at": nil,
				}),
			}).Select("*").Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return EntityLocalizationRecord{}, err
	}
	return s.EntityLocalizations(ctx, entityType, entityID, ResolutionInput{ExplicitLocale: locale.Code})
}

// SyncDefaultEntityLocalization keeps the default-locale localization rows in
// the same transaction as the owning entity mutation. Callers retain ownership
// of the transaction so a catalog write cannot commit without its public copy.
func SyncDefaultEntityLocalization(tx *gorm.DB, entityType EntityType, entityID uint, fields map[string]string, actorID *uint) error {
	policy, ok := entityPolicies[entityType]
	if !ok || entityID == 0 {
		return ErrInvalidTranslation
	}
	var locale models.Locale
	if err := tx.Where("is_default = ? AND is_enabled = ?", true, true).First(&locale).Error; err != nil {
		return err
	}
	allowed := make(map[string]bool, len(policy.fields))
	for _, field := range policy.fields {
		allowed[field] = true
	}
	for field := range fields {
		if !allowed[field] {
			return fmt.Errorf("%w: field %s is not localizable for %s", ErrInvalidTranslation, field, entityType)
		}
	}
	for _, field := range policy.fields {
		value := strings.TrimSpace(fields[field])
		query := tx.Where("entity_type = ? AND entity_id = ? AND locale_id = ? AND field = ?", entityType, entityID, locale.ID, field)
		if value == "" {
			if err := query.Delete(&models.LocalizedEntityValue{}).Error; err != nil {
				return err
			}
			continue
		}
		row := models.LocalizedEntityValue{EntityType: string(entityType), EntityID: entityID, LocaleID: locale.ID, Field: field, Value: value, UpdatedBy: actorID}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "entity_type"}, {Name: "entity_id"}, {Name: "locale_id"}, {Name: "field"}},
			DoUpdates: clause.Assignments(map[string]any{
				"value": value, "updated_by": actorID, "deleted_at": nil,
			}),
		}).Select("*").Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) EntityLocalizations(ctx context.Context, entityType EntityType, entityID uint, input ResolutionInput) (EntityLocalizationRecord, error) {
	policy, ok := entityPolicies[entityType]
	if !ok || entityID == 0 {
		return EntityLocalizationRecord{}, ErrInvalidTranslation
	}
	if err := requireEntity(s.db.WithContext(ctx), policy, entityID); err != nil {
		return EntityLocalizationRecord{}, err
	}
	resolution, err := s.ResolveLocale(ctx, input)
	if err != nil {
		return EntityLocalizationRecord{}, err
	}
	var rows []models.LocalizedEntityValue
	if err := s.db.WithContext(ctx).Preload("Locale").Where("entity_type = ? AND entity_id = ?", entityType, entityID).Order("locale_id ASC, field ASC").Find(&rows).Error; err != nil {
		return EntityLocalizationRecord{}, err
	}
	byLocale := map[string]map[string]string{}
	for _, row := range rows {
		if byLocale[row.Locale.Code] == nil {
			byLocale[row.Locale.Code] = map[string]string{}
		}
		byLocale[row.Locale.Code][row.Field] = row.Value
	}
	localizations := make([]EntityLocalization, 0, len(byLocale))
	locales := make([]string, 0, len(byLocale))
	for locale := range byLocale {
		locales = append(locales, locale)
	}
	sort.Strings(locales)
	for _, locale := range locales {
		localizations = append(localizations, EntityLocalization{Locale: locale, Fields: byLocale[locale]})
	}
	resolved := EntityResolution{RequestedLocale: resolution.RequestedLocale, ResolvedLocale: resolution.ResolvedLocale, FallbackChain: append([]string(nil), resolution.FallbackChain...), Fields: map[string]string{}, SourceLocales: map[string]string{}}
	for _, field := range policy.fields {
		for _, locale := range resolution.FallbackChain {
			if value := byLocale[locale][field]; value != "" {
				resolved.Fields[field] = value
				resolved.SourceLocales[field] = locale
				if locale != resolution.ResolvedLocale {
					resolved.UsedFallback = true
				}
				break
			}
		}
	}
	return EntityLocalizationRecord{EntityType: entityType, EntityID: entityID, Localizations: localizations, Resolved: resolved}, nil
}

func requireEntity(db *gorm.DB, policy entityPolicy, entityID uint) error {
	var count int64
	query := db.Table(policy.table).Where("id = ?", entityID)
	if policy.table != "website_settings" {
		query = query.Where("deleted_at IS NULL")
	}
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return ErrLocalizedEntityNotFound
	}
	return nil
}
