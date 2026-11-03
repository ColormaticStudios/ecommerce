package search

import (
	"context"
	"strconv"
	"strings"

	"ecommerce/models"
)

type canonicalAttributeSelection struct {
	definitions  []uint
	texts, enums []string
	numbers      []float64
	booleans     []bool
}
type canonicalFilterSelection struct {
	brands, categories []uint
	attributes         map[string]canonicalAttributeSelection
}

// Fallback may fail unavailable, but must never silently approximate filters.
// One sentinel row distinguishes a complete metadata vocabulary from overflow.
const canonicalMetadataLimit = 5000

// Resolve only canonical metadata and distinct scalar attribute vocabulary.
// Database LOWER/REPLACE cannot implement NormalizeQuery's Unicode semantics
// portably. Product filtering, totals, sorting and pagination still run in SQL;
// no catalog product or JSON payload is hydrated by this resolution step.
func (s *Service) resolveCanonicalFilters(ctx context.Context, n normalizedFilters) (canonicalFilterSelection, error) {
	out := canonicalFilterSelection{attributes: map[string]canonicalAttributeSelection{}}
	if len(n.brands) > 0 {
		var rows []struct {
			ID   uint
			Slug string
		}
		if err := s.db.WithContext(ctx).Model(&models.Brand{}).Select("id, slug").Where("is_active = ?", true).Limit(canonicalMetadataLimit + 1).Scan(&rows).Error; err != nil {
			return out, err
		}
		if len(rows) > canonicalMetadataLimit {
			return out, ErrIndexUnavailable
		}
		for _, r := range rows {
			if _, ok := n.brands[strings.ToLower(r.Slug)]; ok {
				out.brands = append(out.brands, r.ID)
			}
		}
	}
	if len(n.categories) > 0 {
		var rows []struct {
			ID   uint
			Slug string
		}
		if err := s.db.WithContext(ctx).Model(&models.Category{}).Select("id, slug").Where("is_active = ?", true).Limit(canonicalMetadataLimit + 1).Scan(&rows).Error; err != nil {
			return out, err
		}
		if len(rows) > canonicalMetadataLimit {
			return out, ErrIndexUnavailable
		}
		for _, r := range rows {
			if _, ok := n.categories[NormalizeQuery(r.Slug)]; ok {
				out.categories = append(out.categories, r.ID)
			}
		}
	}
	if len(n.attributes) == 0 {
		return out, nil
	}
	var definitions []struct {
		ID   uint
		Slug string
	}
	if err := s.db.WithContext(ctx).Model(&models.ProductAttribute{}).Select("id, slug").Where("filterable = ?", true).Limit(canonicalMetadataLimit + 1).Scan(&definitions).Error; err != nil {
		return out, err
	}
	if len(definitions) > canonicalMetadataLimit {
		return out, ErrIndexUnavailable
	}
	for _, d := range definitions {
		slug := NormalizeQuery(d.Slug)
		if _, ok := n.attributes[slug]; ok {
			a := out.attributes[slug]
			a.definitions = append(a.definitions, d.ID)
			out.attributes[slug] = a
		}
	}
	for slug, selected := range n.attributes {
		a := out.attributes[slug]
		if len(a.definitions) == 0 {
			continue
		}
		var values []struct {
			TextValue    *string
			NumberValue  *float64
			BooleanValue *bool
			EnumValue    *string
		}
		if err := s.db.WithContext(ctx).Model(&models.ProductAttributeValue{}).Distinct("text_value", "number_value", "boolean_value", "enum_value").Where("product_attribute_id IN ?", a.definitions).Limit(canonicalMetadataLimit + 1).Scan(&values).Error; err != nil {
			return out, err
		}
		if len(values) > canonicalMetadataLimit {
			return out, ErrIndexUnavailable
		}
		for _, v := range values {
			raw := ""
			switch {
			case v.TextValue != nil:
				raw = *v.TextValue
			case v.NumberValue != nil:
				raw = strconv.FormatFloat(*v.NumberValue, 'f', -1, 64)
			case v.BooleanValue != nil:
				raw = strconv.FormatBool(*v.BooleanValue)
			case v.EnumValue != nil:
				raw = *v.EnumValue
			}
			if _, ok := selected[NormalizeQuery(raw)]; !ok {
				continue
			}
			switch {
			case v.TextValue != nil:
				a.texts = append(a.texts, *v.TextValue)
			case v.NumberValue != nil:
				a.numbers = append(a.numbers, *v.NumberValue)
			case v.BooleanValue != nil:
				a.booleans = append(a.booleans, *v.BooleanValue)
			case v.EnumValue != nil:
				a.enums = append(a.enums, *v.EnumValue)
			}
		}
		out.attributes[slug] = a
	}
	return out, nil
}
