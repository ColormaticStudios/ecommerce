package search

import (
	"context"
	"ecommerce/models"
	"gorm.io/gorm"
	"strings"
)

// Canonical fallback never depends on search projection tables. Aggregate only
// published, nondeleted variants so unpublished prices/stock cannot leak.
func (s *Service) canonicalSelection(ctx context.Context, f normalizedFilters, resolved canonicalFilterSelection, hidden []uint) *gorm.DB {
	summary := s.db.WithContext(ctx).Table("product_variants v").Select("v.product_id, COUNT(*) AS published_count, MIN(v.price) AS minimum, MAX(v.price) AS maximum, MAX(CASE WHEN v.stock > 0 THEN 1 ELSE 0 END) AS available").Where("v.is_published = ? AND v.deleted_at IS NULL", true).Group("v.product_id")
	q := s.db.WithContext(ctx).Table("products p").Joins("LEFT JOIN (?) v ON v.product_id = p.id", summary).Where("p.is_published = ? AND p.deleted_at IS NULL", true).Where("v.published_count > 0 OR NOT EXISTS (SELECT 1 FROM product_variants allv WHERE allv.product_id = p.id AND allv.deleted_at IS NULL)")
	if len(hidden) > 0 {
		q = q.Where("p.id NOT IN ?", hidden)
	}
	if len(f.brands) > 0 {
		q = q.Where("p.brand_id IN ?", resolved.brands)
	}
	if len(f.categories) > 0 {
		q = q.Where("EXISTS (SELECT 1 FROM product_categories pc WHERE pc.product_id = p.id AND pc.category_id IN ?)", resolved.categories)
	}
	if len(f.stock) > 0 {
		values := []int{}
		for available := range f.stock {
			if available {
				values = append(values, 1)
			} else {
				values = append(values, 0)
			}
		}
		q = q.Where("CASE WHEN v.published_count > 0 THEN v.available WHEN p.stock > 0 THEN 1 ELSE 0 END IN ?", values)
	}
	minimum := "CASE WHEN v.published_count > 0 THEN v.minimum ELSE p.price END"
	maximum := "CASE WHEN v.published_count > 0 THEN v.maximum ELSE p.price END"
	if len(f.priceRanges) > 0 {
		parts, args := []string{}, []any{}
		for _, p := range f.priceRanges {
			part := "1=1"
			if p.Min != nil {
				part += " AND " + maximum + " >= ?"
				args = append(args, models.MoneyFromFloat(*p.Min).Float64())
			}
			if p.Max != nil {
				part += " AND " + minimum + " <= ?"
				args = append(args, models.MoneyFromFloat(*p.Max).Float64())
			}
			parts = append(parts, "("+part+")")
		}
		q = q.Where("("+strings.Join(parts, " OR ")+")", args...)
	} else {
		if f.minimum != nil {
			q = q.Where(maximum+" >= ?", f.minimum.Float64())
		}
		if f.maximum != nil {
			q = q.Where(minimum+" <= ?", f.maximum.Float64())
		}
	}
	for slug := range f.attributes {
		a := resolved.attributes[slug]
		parts, args := []string{}, []any{a.definitions}
		if len(a.texts) > 0 {
			parts = append(parts, "av.text_value IN ?")
			args = append(args, a.texts)
		}
		if len(a.numbers) > 0 {
			parts = append(parts, "(av.text_value IS NULL AND av.number_value IN ?)")
			args = append(args, a.numbers)
		}
		if len(a.booleans) > 0 {
			parts = append(parts, "(av.text_value IS NULL AND av.number_value IS NULL AND av.boolean_value IN ?)")
			args = append(args, a.booleans)
		}
		if len(a.enums) > 0 {
			parts = append(parts, "(av.text_value IS NULL AND av.number_value IS NULL AND av.boolean_value IS NULL AND av.enum_value IN ?)")
			args = append(args, a.enums)
		}
		if len(parts) == 0 {
			q = q.Where("1=0")
			continue
		}
		q = q.Where("EXISTS (SELECT 1 FROM product_attribute_values av WHERE av.product_id = p.id AND av.deleted_at IS NULL AND av.product_attribute_id IN ? AND ("+strings.Join(parts, " OR ")+"))", args...)
	}
	return q
}

func (s *Service) canonicalSQLPage(ctx context.Context, f Filters, n normalizedFilters, hidden []uint) (Result, error) {
	resolved, err := s.resolveCanonicalFilters(ctx, n)
	if err != nil {
		return Result{}, err
	}
	var total int64
	if err := s.canonicalSelection(ctx, n, resolved, hidden).Count(&total).Error; err != nil {
		return Result{}, err
	}
	page, limit := max(1, f.Page), f.Limit
	if limit < 1 {
		limit = 20
	}
	limit = min(100, limit)
	direction := "DESC"
	if strings.EqualFold(f.SortOrder, "asc") {
		direction = "ASC"
	}
	column := "p.created_at"
	if f.SortField == "name" {
		column = "LOWER(p.name)"
	} else if f.SortField == "price" {
		column = "CASE WHEN v.published_count > 0 THEN v.minimum ELSE p.price END"
	}
	var ids []uint
	start, _ := sqlPageWindow(total, page, limit)
	if start < total {
		if err := s.canonicalSelection(ctx, n, resolved, hidden).Select("p.id").Order(column+" "+direction+", p.id ASC").Offset(int(start)).Limit(limit).Pluck("p.id", &ids).Error; err != nil {
			return Result{}, err
		}
	}
	products := []models.Product{}
	if len(ids) > 0 {
		var rows []models.Product
		if err := s.db.WithContext(ctx).Preload("Brand").Preload("Categories").Preload("Variants").Preload("AttributeValues.ProductAttribute").Where("id IN ?", ids).Find(&rows).Error; err != nil {
			return Result{}, err
		}
		byID := map[uint]models.Product{}
		for _, p := range rows {
			doc, include, err := projectProduct(p, s.now())
			if err != nil {
				return Result{}, err
			}
			if !include {
				continue
			}
			decoded, err := decodeIndexedProducts(ctx, []models.SearchDocument{doc})
			if err != nil {
				return Result{}, err
			}
			byID[p.ID] = *decoded[0].product
		}
		for _, id := range ids {
			if p, ok := byID[id]; ok {
				products = append(products, p)
			}
		}
	}
	return Result{Products: products, Facets: buildFacets(nil, queryPlan{}, false, n), Total: total, TotalPages: int((total + int64(limit) - 1) / int64(limit)), NormalizedQuery: NormalizeQuery(f.Query), AppliedRewrites: []AppliedRewrite{}}, nil
}
