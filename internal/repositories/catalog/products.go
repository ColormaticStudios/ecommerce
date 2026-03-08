package catalog

import (
	"fmt"
	"strconv"
	"strings"

	"ecommerce/models"

	"gorm.io/gorm"
)

type ProductListFilters struct {
	SearchTerm      string
	MinPrice        *float64
	MaxPrice        *float64
	BrandSlug       string
	HasVariantStock *bool
	Attribute       map[string]string
	SortField       string
	SortOrder       string
	Page            int
	Limit           int
	Preview         bool
}

type ProductListResult struct {
	Products []models.Product
	Total    int64
}

type Repository struct {
	db *gorm.DB
}

const publicCatalogVariantVisibilityClause = `
NOT EXISTS (
	SELECT 1
	FROM product_variants pv_all
	WHERE pv_all.product_id = products.id
) OR EXISTS (
	SELECT 1
	FROM product_variants pv_public
	WHERE pv_public.product_id = products.id
	  AND pv_public.is_published = TRUE
)`

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func normalizeSort(sortField, sortOrder string) (string, string) {
	validSortFields := map[string]bool{"price": true, "name": true, "created_at": true}
	if !validSortFields[sortField] {
		sortField = "created_at"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}
	return sortField, sortOrder
}

func parseAttributeFilterValue(definition models.ProductAttribute, raw string) (any, bool) {
	trimmed := strings.TrimSpace(raw)
	switch definition.Type {
	case "text", "enum":
		if trimmed == "" {
			return nil, false
		}
		return strings.ToLower(trimmed), true
	case "number":
		value, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return nil, false
		}
		return value, true
	case "boolean":
		switch strings.ToLower(trimmed) {
		case "true", "1", "yes":
			return true, true
		case "false", "0", "no":
			return false, true
		default:
			return nil, false
		}
	default:
		return nil, false
	}
}

func (r *Repository) ListProducts(filters ProductListFilters) (ProductListResult, error) {
	query := r.db.Model(&models.Product{})
	if filters.Preview {
		query = query.Where("products.is_published = ? OR products.draft_updated_at IS NOT NULL", true)
	} else {
		query = query.Where("products.is_published = ?", true).
			Where(publicCatalogVariantVisibilityClause)
	}

	variantPrices := r.db.Table("product_variants").
		Select("product_id, MIN(price) AS min_price, MAX(price) AS max_price")
	if !filters.Preview {
		variantPrices = variantPrices.Where("is_published = ?", true)
	}
	variantPrices = variantPrices.Group("product_id")
	query = query.Joins("LEFT JOIN (?) AS variant_prices ON variant_prices.product_id = products.id", variantPrices)

	if term := strings.TrimSpace(filters.SearchTerm); term != "" {
		query = query.Where("products.name ILIKE ?", "%"+term+"%")
	}
	if filters.MinPrice != nil {
		query = query.Where("COALESCE(variant_prices.max_price, products.price) >= ?", *filters.MinPrice)
	}
	if filters.MaxPrice != nil {
		query = query.Where("COALESCE(variant_prices.min_price, products.price) <= ?", *filters.MaxPrice)
	}
	if brandSlug := strings.TrimSpace(filters.BrandSlug); brandSlug != "" {
		query = query.Joins("JOIN brands ON brands.id = products.brand_id").Where("brands.slug = ?", strings.ToLower(brandSlug))
		if !filters.Preview {
			query = query.Where("brands.is_active = ?", true)
		}
	}
	if filters.HasVariantStock != nil {
		stockClause := "EXISTS (SELECT 1 FROM product_variants pv WHERE pv.product_id = products.id AND pv.stock > 0"
		if !filters.Preview {
			stockClause += " AND pv.is_published = TRUE"
		}
		stockClause += ")"
		if *filters.HasVariantStock {
			query = query.Where(stockClause)
		} else {
			query = query.Where("NOT " + stockClause)
		}
	}
	if len(filters.Attribute) > 0 {
		slugs := make([]string, 0, len(filters.Attribute))
		for slug := range filters.Attribute {
			trimmed := strings.TrimSpace(strings.ToLower(slug))
			if trimmed != "" {
				slugs = append(slugs, trimmed)
			}
		}
		var definitions []models.ProductAttribute
		if len(slugs) > 0 {
			definitionQuery := r.db.Model(&models.ProductAttribute{}).Where("slug IN ?", slugs)
			if !filters.Preview {
				definitionQuery = definitionQuery.Where("filterable = ?", true)
			}
			if err := definitionQuery.Find(&definitions).Error; err != nil {
				return ProductListResult{}, err
			}
		}

		definitionBySlug := make(map[string]models.ProductAttribute, len(definitions))
		for _, definition := range definitions {
			definitionBySlug[strings.ToLower(definition.Slug)] = definition
		}

		for rawSlug, rawValue := range filters.Attribute {
			slug := strings.TrimSpace(strings.ToLower(rawSlug))
			definition, ok := definitionBySlug[slug]
			if !ok || (!filters.Preview && !definition.Filterable) {
				query = query.Where("1 = 0")
				continue
			}

			parsedValue, ok := parseAttributeFilterValue(definition, rawValue)
			if !ok {
				query = query.Where("1 = 0")
				continue
			}

			switch definition.Type {
			case "text":
				query = query.Where(
					`EXISTS (
						SELECT 1
						FROM product_attribute_values pav
						WHERE pav.product_id = products.id
						  AND pav.product_attribute_id = ?
						  AND LOWER(COALESCE(pav.text_value, '')) = ?
					)`,
					definition.ID,
					parsedValue,
				)
			case "enum":
				query = query.Where(
					`EXISTS (
						SELECT 1
						FROM product_attribute_values pav
						WHERE pav.product_id = products.id
						  AND pav.product_attribute_id = ?
						  AND LOWER(COALESCE(pav.enum_value, '')) = ?
					)`,
					definition.ID,
					parsedValue,
				)
			case "number":
				query = query.Where(
					`EXISTS (
						SELECT 1
						FROM product_attribute_values pav
						WHERE pav.product_id = products.id
						  AND pav.product_attribute_id = ?
						  AND pav.number_value = ?
					)`,
					definition.ID,
					parsedValue,
				)
			case "boolean":
				query = query.Where(
					`EXISTS (
						SELECT 1
						FROM product_attribute_values pav
						WHERE pav.product_id = products.id
						  AND pav.product_attribute_id = ?
						  AND pav.boolean_value = ?
					)`,
					definition.ID,
					parsedValue,
				)
			default:
				return ProductListResult{}, fmt.Errorf("unsupported attribute type %q", definition.Type)
			}
		}
	}

	sortField, sortOrder := normalizeSort(filters.SortField, filters.SortOrder)
	if sortField == "price" {
		query = query.Order("COALESCE(variant_prices.min_price, products.price) " + sortOrder)
	} else {
		query = query.Order("products." + sortField + " " + sortOrder)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return ProductListResult{}, err
	}

	offset := (filters.Page - 1) * filters.Limit
	var products []models.Product
	if err := query.Offset(offset).Limit(filters.Limit).Find(&products).Error; err != nil {
		return ProductListResult{}, err
	}

	return ProductListResult{Products: products, Total: total}, nil
}

func (r *Repository) GetPublicProductByID(id string) (models.Product, error) {
	var product models.Product
	if err := r.db.Preload("Related", "is_published = ?", true).
		Where("products.is_published = ?", true).
		Where(publicCatalogVariantVisibilityClause).
		First(&product, id).Error; err != nil {
		return models.Product{}, err
	}
	return product, nil
}

func (r *Repository) GetPreviewProductByID(id string) (models.Product, error) {
	var product models.Product
	if err := r.db.Preload("Related").First(&product, id).Error; err != nil {
		return models.Product{}, err
	}
	return product, nil
}
