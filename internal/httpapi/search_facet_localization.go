package httpapi

import (
	"context"
	"ecommerce/internal/requestctx"
	searchservice "ecommerce/internal/search"
	localizationservice "ecommerce/internal/services/localization"
	"ecommerce/models"
	"strings"
)

// Metadata and localized copy are fetched in batches, including selected values
// which may have zero counts or occur only in degraded results. Stable filter
// values remain canonical; only customer-visible labels change with locale.
func (e *CatalogEndpoints) localizeSearchFacets(ctx context.Context, facets []searchservice.Facet) ([]searchservice.Facet, error) {
	brandSlugs, categorySlugs, attributeSlugs := []string{}, []string{}, []string{}
	for _, f := range facets {
		switch f.Name {
		case "brand":
			for _, v := range f.Values {
				brandSlugs = append(brandSlugs, v.Value, canonicalFacetSlug(v.Value))
			}
		case "category":
			for _, v := range f.Values {
				categorySlugs = append(categorySlugs, v.Value, canonicalFacetSlug(v.Value))
			}
		default:
			if strings.HasPrefix(f.Name, "attribute:") {
				slug := strings.TrimPrefix(f.Name, "attribute:")
				attributeSlugs = append(attributeSlugs, slug, canonicalFacetSlug(slug))
			}
		}
	}
	refs := []localizationservice.EntityReference{}
	canonicalLabels := map[localizationservice.EntityReference]string{}
	brands, categories, attributes := map[string]uint{}, map[string]uint{}, map[string]uint{}
	if len(brandSlugs) > 0 {
		var rows []models.Brand
		if err := e.db.WithContext(ctx).Select("id,slug,name").Where("slug IN ?", brandSlugs).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			brands[searchservice.NormalizeQuery(row.Slug)] = row.ID
			refs = append(refs, localizationservice.EntityReference{EntityType: localizationservice.EntityTypeBrand, EntityID: row.ID})
			canonicalLabels[localizationservice.EntityReference{EntityType: localizationservice.EntityTypeBrand, EntityID: row.ID}] = row.Name
		}
	}
	if len(categorySlugs) > 0 {
		var rows []models.Category
		if err := e.db.WithContext(ctx).Select("id,slug,name").Where("slug IN ?", categorySlugs).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			categories[searchservice.NormalizeQuery(row.Slug)] = row.ID
			refs = append(refs, localizationservice.EntityReference{EntityType: localizationservice.EntityTypeCategory, EntityID: row.ID})
			canonicalLabels[localizationservice.EntityReference{EntityType: localizationservice.EntityTypeCategory, EntityID: row.ID}] = row.Name
		}
	}
	if len(attributeSlugs) > 0 {
		var rows []models.ProductAttribute
		if err := e.db.WithContext(ctx).Select("id,slug,key").Where("slug IN ?", attributeSlugs).Find(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			attributes[searchservice.NormalizeQuery(row.Slug)] = row.ID
			refs = append(refs, localizationservice.EntityReference{EntityType: localizationservice.EntityTypeProductAttribute, EntityID: row.ID})
			canonicalLabels[localizationservice.EntityReference{EntityType: localizationservice.EntityTypeProductAttribute, EntityID: row.ID}] = row.Key
		}
	}
	input := localizationservice.ResolutionInput{}
	if locale, ok := requestctx.LocaleResolutionFrom(ctx); ok {
		input.ExplicitLocale = locale.ResolvedLocale
	}
	localized, err := e.localization.ResolveEntities(ctx, refs, input)
	if err != nil {
		return nil, err
	}
	result := make([]searchservice.Facet, len(facets))
	for i, f := range facets {
		result[i] = f
		result[i].Values = append([]searchservice.FacetValue(nil), f.Values...)
		var kind localizationservice.EntityType
		var ids map[string]uint
		switch f.Name {
		case "brand":
			kind = localizationservice.EntityTypeBrand
			ids = brands
		case "category":
			kind = localizationservice.EntityTypeCategory
			ids = categories
		default:
			if strings.HasPrefix(f.Name, "attribute:") {
				slug := strings.TrimPrefix(f.Name, "attribute:")
				id := attributes[searchservice.NormalizeQuery(slug)]
				if label := localized[localizationservice.EntityReference{EntityType: localizationservice.EntityTypeProductAttribute, EntityID: id}].Fields["key"]; label != "" {
					result[i].Label = label
				} else if canonical := canonicalLabels[localizationservice.EntityReference{EntityType: localizationservice.EntityTypeProductAttribute, EntityID: id}]; canonical != "" {
					result[i].Label = canonical
				} else if result[i].Label == "" {
					result[i].Label = slug
				}
			}
			continue
		}
		for j, v := range f.Values {
			key := v.Value
			if kind == localizationservice.EntityTypeCategory || kind == localizationservice.EntityTypeBrand {
				key = searchservice.NormalizeQuery(key)
			}
			if label := localized[localizationservice.EntityReference{EntityType: kind, EntityID: ids[key]}].Fields["name"]; label != "" {
				result[i].Values[j].Label = label
			} else if canonical := canonicalLabels[localizationservice.EntityReference{EntityType: kind, EntityID: ids[key]}]; canonical != "" {
				result[i].Values[j].Label = canonical
			} else if result[i].Values[j].Label == "" {
				result[i].Values[j].Label = v.Value
			}
		}
	}
	return result, nil
}

// Catalog writes use lowercase hyphen-separated slugs. Search projections use
// space-separated tokens; restoring separators gives an indexed metadata lookup
// without scanning the catalog. Preserve Unicode for historical catalog slugs.
func canonicalFacetSlug(value string) string {
	return strings.ReplaceAll(searchservice.NormalizeQuery(value), " ", "-")
}
