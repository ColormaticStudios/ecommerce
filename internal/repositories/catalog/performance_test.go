//go:build performance

package catalog

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"ecommerce/models"

	"github.com/stretchr/testify/require"
)

const (
	catalogPerformanceProductCount = 10_000
	catalogQueryP95Budget          = 100 * time.Millisecond
)

func TestCatalogFilteredQueryPerformanceBudget(t *testing.T) {
	db := newRepositoryTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.ProductAttribute{}, &models.ProductAttributeValue{}))
	attribute := models.ProductAttribute{Key: "Material", Slug: "material", Type: "enum", Filterable: true}
	require.NoError(t, db.Select("*").Create(&attribute).Error)

	products := make([]models.Product, catalogPerformanceProductCount)
	for index := range products {
		products[index] = models.Product{
			SKU: fmt.Sprintf("PERF-%05d", index), Name: fmt.Sprintf("Performance product %05d", index),
			Description: "Seeded catalog performance product", Price: models.MoneyFromFloat(float64(10 + index%200)),
			Stock: 10, IsPublished: true,
		}
	}
	require.NoError(t, db.CreateInBatches(&products, 500).Error)
	variants := make([]models.ProductVariant, 0, len(products))
	values := make([]models.ProductAttributeValue, 0, len(products))
	for index, product := range products {
		variants = append(variants, models.ProductVariant{
			ProductID: product.ID, SKU: fmt.Sprintf("PERF-V-%05d", index), Title: "Default",
			Price: models.MoneyFromFloat(float64(10 + index%200)), Stock: 10, Position: 1, IsPublished: true,
		})
		material := "cotton"
		if index%2 == 0 {
			material = "wool"
		}
		values = append(values, models.ProductAttributeValue{ProductID: product.ID, ProductAttributeID: attribute.ID, EnumValue: &material, Position: 1})
	}
	require.NoError(t, db.CreateInBatches(&variants, 500).Error)
	require.NoError(t, db.CreateInBatches(&values, 500).Error)

	repo := NewRepository(db)
	filters := ProductListFilters{Attribute: map[string]string{"material": "wool"}, SortField: "price", SortOrder: "asc", Page: 1, Limit: 24}
	result, err := repo.ListProducts(filters)
	require.NoError(t, err)
	require.Equal(t, int64(catalogPerformanceProductCount/2), result.Total)

	durations := make([]time.Duration, 20)
	for index := range durations {
		started := time.Now()
		_, err := repo.ListProducts(filters)
		require.NoError(t, err)
		durations[index] = time.Since(started)
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	p95 := durations[18]
	if p95 >= catalogQueryP95Budget {
		t.Fatalf("catalog filtered-query p95 %s exceeds %s budget with %d products", p95, catalogQueryP95Budget, catalogPerformanceProductCount)
	}
}
