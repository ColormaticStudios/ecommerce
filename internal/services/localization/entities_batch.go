package localization

import (
	"context"
	"ecommerce/models"
	"gorm.io/gorm"
)

type EntityReference struct {
	EntityType EntityType
	EntityID   uint
}

// ResolveEntities applies the same field-by-field locale fallback as
// EntityLocalizations, with bounded queries per entity type rather than entity.
// It returns no canonical-column fallback when a translation is missing.
func (s *Service) ResolveEntities(ctx context.Context, refs []EntityReference, input ResolutionInput) (map[EntityReference]EntityResolution, error) {
	result := make(map[EntityReference]EntityResolution, len(refs))
	if len(refs) == 0 {
		return result, nil
	}
	grouped := map[EntityType]map[uint]bool{}
	for _, ref := range refs {
		if _, ok := entityPolicies[ref.EntityType]; !ok || ref.EntityID == 0 {
			return nil, ErrInvalidTranslation
		}
		if grouped[ref.EntityType] == nil {
			grouped[ref.EntityType] = map[uint]bool{}
		}
		grouped[ref.EntityType][ref.EntityID] = true
	}
	resolution, err := s.ResolveLocale(ctx, input)
	if err != nil {
		return nil, err
	}
	db := s.db.WithContext(ctx)
	query := db.Model(&models.LocalizedEntityValue{})
	where := db.Session(&gorm.Session{NewDB: true})
	for kind, ids := range grouped {
		list := make([]uint, 0, len(ids))
		for id := range ids {
			list = append(list, id)
		}
		policy := entityPolicies[kind]
		exists := db.Table(policy.table).Where("id IN ?", list)
		if policy.table != "website_settings" {
			exists = exists.Where("deleted_at IS NULL")
		}
		var count int64
		if err := exists.Count(&count).Error; err != nil {
			return nil, err
		}
		if count != int64(len(list)) {
			return nil, ErrLocalizedEntityNotFound
		}
		where = where.Or("entity_type = ? AND entity_id IN ?", kind, list)
	}
	var rows []models.LocalizedEntityValue
	if err := query.Preload("Locale").Where(where).Find(&rows).Error; err != nil {
		return nil, err
	}
	byEntity := map[EntityReference]map[string]map[string]string{}
	for _, row := range rows {
		ref := EntityReference{EntityType(row.EntityType), row.EntityID}
		if byEntity[ref] == nil {
			byEntity[ref] = map[string]map[string]string{}
		}
		if byEntity[ref][row.Locale.Code] == nil {
			byEntity[ref][row.Locale.Code] = map[string]string{}
		}
		byEntity[ref][row.Locale.Code][row.Field] = row.Value
	}
	for _, ref := range refs {
		value := EntityResolution{RequestedLocale: resolution.RequestedLocale, ResolvedLocale: resolution.ResolvedLocale, FallbackChain: append([]string(nil), resolution.FallbackChain...), Fields: map[string]string{}, SourceLocales: map[string]string{}}
		for _, field := range entityPolicies[ref.EntityType].fields {
			for _, locale := range resolution.FallbackChain {
				if text := byEntity[ref][locale][field]; text != "" {
					value.Fields[field] = text
					value.SourceLocales[field] = locale
					value.UsedFallback = value.UsedFallback || locale != resolution.ResolvedLocale
					break
				}
			}
		}
		result[ref] = value
	}
	return result, nil
}
