package handlers

import (
	"strconv"
	"strings"

	catalogservice "ecommerce/internal/services/catalog"

	"github.com/gin-gonic/gin"
)

func parseOptionalBoolParam(c *gin.Context, key string) *bool {
	if _, exists := c.GetQuery(key); !exists {
		return nil
	}

	value := strings.TrimSpace(c.Query(key))
	switch strings.ToLower(value) {
	case "true", "1", "yes":
		result := true
		return &result
	case "false", "0", "no":
		result := false
		return &result
	default:
		return nil
	}
}

func parseStringArrayParam(c *gin.Context, key string) []string {
	raw := c.QueryArray(key)
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		values = append(values, trimmed)
	}
	return values
}

func parseUintArrayParam(c *gin.Context, key string) []uint {
	raw := c.QueryArray(key)
	values := make([]uint, 0, len(raw))
	for _, item := range raw {
		parsed, err := strconv.ParseUint(strings.TrimSpace(item), 10, 64)
		if err != nil || parsed == 0 {
			continue
		}
		values = append(values, uint(parsed))
	}
	return values
}

func parseCatalogAttributeFilters(c *gin.Context) map[string]string {
	raw := c.QueryMap("attribute")
	if len(raw) == 0 {
		return map[string]string{}
	}

	filters := make(map[string]string, len(raw))
	for key, value := range raw {
		slug := strings.TrimSpace(key)
		if slug == "" {
			continue
		}
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" {
			continue
		}
		filters[slug] = trimmedValue
	}
	return filters
}

func buildCatalogListInput(c *gin.Context, preview bool, defaultLimit int) catalogservice.ListProductsInput {
	page, limit, _ := parsePagination(c, defaultLimit)
	minPrice, _ := catalogservice.ParsePrice(c.Query("min_price"))
	maxPrice, _ := catalogservice.ParsePrice(c.Query("max_price"))

	return catalogservice.ListProductsInput{
		SearchTerm:                strings.TrimSpace(c.Query("q")),
		MinPrice:                  minPrice,
		MaxPrice:                  maxPrice,
		BrandSlug:                 strings.TrimSpace(c.Query("brand_slug")),
		CategorySlugs:             parseStringArrayParam(c, "category_slug"),
		CategoryIDs:               parseUintArrayParam(c, "category_id"),
		IncludeInactiveCategories: strings.EqualFold(c.Query("include_inactive_categories"), "true"),
		HasVariantStock:           parseOptionalBoolParam(c, "has_variant_stock"),
		Attribute:                 parseCatalogAttributeFilters(c),
		SortField:                 c.DefaultQuery("sort", "created_at"),
		SortOrder:                 c.DefaultQuery("order", "desc"),
		Page:                      page,
		Limit:                     limit,
		Preview:                   preview,
	}
}
