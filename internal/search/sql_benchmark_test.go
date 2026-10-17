package search

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"ecommerce/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// There are no process-local result/document caches. Cold constructs a backend
// for each request; warm reuses its bounded configuration cache. Neither caches result documents.
func BenchmarkSQLRetrieval(b *testing.B) {
	for _, size := range []int{10000, 25000} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			db := searchTestDB(b)
			db.Logger = logger.Default.LogMode(logger.Silent)
			now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
			docs := make([]models.SearchDocument, 0, size)
			for i := 1; i <= size; i++ {
				color := []string{"red", "blue", "green", "black"}[i%4]
				brand := fmt.Sprintf("brand%d", i%20)
				category := fmt.Sprintf("category%d", i%12)
				p := models.Product{BaseModel: models.BaseModel{ID: uint(i), CreatedAt: now.Add(-time.Duration(i%60) * 24 * time.Hour), UpdatedAt: now}, SKU: fmt.Sprintf("sku%d", i), Name: fmt.Sprintf("%s Trail Shoe %d", color, i), Description: "waterproof hiking footwear durable lightweight", Price: models.MoneyFromFloat(float64(10 + i%200)), Stock: i % 7, IsPublished: true, Brand: &models.Brand{Name: brand, Slug: brand, IsActive: true}, Categories: []models.Category{{Slug: category, Name: category, IsActive: true}}, AttributeValues: []models.ProductAttributeValue{{TextValue: &color, ProductAttribute: &models.ProductAttribute{Slug: "color", Key: "Color", Filterable: true}}}}
				doc, ok, err := projectProduct(p, now)
				require.NoError(b, err)
				require.True(b, ok)
				docs = append(docs, doc)
			}
			backend := NewDatabaseBackend(db).(*databaseBackend)
			backend.now = func() time.Time { return now }
			require.NoError(b, backend.ReplaceAll(context.Background(), docs, now))
			if os.Getenv("SEARCH_SQL_PROFILE") != "" {
				db.Logger = logger.Default.LogMode(logger.Info)
				for _, phrase := range []string{"trail", "trali"} {
					var count int64
					sql := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
						copy := *backend
						copy.db = tx
						return copy.selection(context.Background(), sqlSelection{plan: buildQueryPlan(phrase, nil)}, "").Count(&count)
					})
					var plan []struct{ Detail string }
					require.NoError(b, db.Raw("EXPLAIN QUERY PLAN "+sql).Scan(&plan).Error)
					fmt.Printf("%s plan: %+v\n", phrase, plan)
				}
			}
			cases := []struct {
				name    string
				filters Filters
			}{{"broad", Filters{Query: "trail", Limit: 20}}, {"mixed_facets", Filters{Query: "shoe", BrandSlugs: []string{"brand1", "brand5"}, CategorySlugs: []string{"category1", "category5"}, AttributeValues: map[string][]string{"color": {"blue"}}, Limit: 20}}, {"deep_page", Filters{Query: "trail", Page: size / 40, Limit: 20}}, {"typo", Filters{Query: "trali", Limit: 20}}}
			for _, c := range cases {
				for _, cold := range []bool{true, false} {
					mode := "warm"
					if cold {
						mode = "cold"
					}
					b.Run(c.name+"/"+mode, func(b *testing.B) {
						b.ReportAllocs()
						b.ResetTimer()
						for i := 0; i < b.N; i++ {
							current := backend
							if cold {
								current = NewDatabaseBackend(db).(*databaseBackend)
								current.now = backend.now
							}
							r, err := current.Search(context.Background(), c.filters)
							if err != nil {
								b.Fatal(err)
							}
							if len(r.Products) > 20 {
								b.Fatal("unbounded page")
							}
						}
					})
				}
			}
			b.Run("suggest", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					if _, err := backend.Suggest(context.Background(), "trali", 10); err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}
