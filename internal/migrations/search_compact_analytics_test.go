package migrations

import (
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestSearchCompactAnalyticsReplayPreservesAttribution(t *testing.T) {
	db := newTestDB(t)
	for _, m := range []any{&searchP0QueryEventSchema{}, &searchP0ClickEventSchema{}, &searchP0IndexStateSchema{}} {
		require.NoError(t, db.AutoMigrate(m))
	}
	require.NoError(t, db.Transaction(migrateSearchAnalyticsP4Schema))
	now := time.Now().UTC()
	require.NoError(t, db.Table("search_query_events").Create(map[string]any{"id": 17, "query": "boots", "normalized_query": "boots", "filters_json": `{"Page":2,"Limit":10,"BrandSlugs":["brand"]}`, "products_json": "[8,9]", "result_json": `{"items":[{"id":8}]}`, "impression_id": "impression", "session_hash": "owner", "result_count": 25, "latency_ms": 19, "created_at": now}).Error)
	require.NoError(t, db.Table("search_order_attributions").Create(map[string]any{"id": 31, "order_id": 51, "product_id": 8, "impression_id": "impression", "click_id": "click", "created_at": now, "paid_at": now}).Error)
	require.NoError(t, db.Transaction(migrateSearchCompactAnalytics))
	var row struct {
		ID                                                   uint
		ImpressionID, SessionHash, ProductsJSON, FiltersJSON string
		FirstPosition, ProductCount, FacetMask, LatencyMs    int
		SnapshotID                                           string
	}
	require.NoError(t, db.Table("search_query_events").First(&row).Error)
	require.Equal(t, uint(17), row.ID)
	require.Equal(t, 11, row.FirstPosition)
	require.Equal(t, 2, row.ProductCount)
	require.Equal(t, 1, row.FacetMask)
	require.Equal(t, 19, row.LatencyMs)
	require.Equal(t, "[8,9]", row.ProductsJSON)
	require.NoError(t, db.Transaction(migrateSearchCompactAnalyticsContract))
	require.False(t, db.Migrator().HasColumn("search_query_events", "result_json"))
	var attribution searchP4OrderAttributionSchema
	require.NoError(t, db.First(&attribution).Error)
	require.Equal(t, uint(31), attribution.ID)
	require.Equal(t, "impression", attribution.ImpressionID)
	require.NotNil(t, attribution.PaidAt)
	require.NoError(t, db.Transaction(migrateSearchCompactAnalytics))
	require.NoError(t, db.Transaction(migrateSearchCompactAnalyticsContract))
}

func TestSearchCompactAnalyticsBackfillsLegacyAndCurrentFilterAliases(t *testing.T) {
	for _, filters := range []string{
		`{"BrandSlug":"brand","CategorySlugs":["shoes"],"HasVariantStock":false,"MinPrice":0,"Attributes":{"color":"red"}}`,
		`{"BrandSlugs":["brand"],"CategorySlugs":["shoes"],"StockAvailability":[false],"PriceRanges":[{"Max":10}],"AttributeValues":{"color":["red"]}}`,
	} {
		t.Run(filters, func(t *testing.T) {
			db := newTestDB(t)
			require.NoError(t, db.AutoMigrate(&searchP0QueryEventSchema{}))
			require.NoError(t, db.Exec(`ALTER TABLE search_query_events ADD COLUMN impression_id TEXT NOT NULL DEFAULT ''`).Error)
			require.NoError(t, db.Exec(`ALTER TABLE search_query_events ADD COLUMN session_hash TEXT NOT NULL DEFAULT ''`).Error)
			require.NoError(t, db.Exec(`ALTER TABLE search_query_events ADD COLUMN products_json TEXT NOT NULL DEFAULT '[]'`).Error)
			require.NoError(t, db.Table("search_query_events").Create(map[string]any{"query": "boots", "normalized_query": "boots", "filters_json": filters, "impression_id": "impression", "session_hash": "owner", "products_json": "[8]", "result_count": 1, "latency_ms": 12, "created_at": time.Now().UTC()}).Error)
			require.NoError(t, db.Transaction(migrateSearchCompactAnalytics))
			var mask int
			require.NoError(t, db.Table("search_query_events").Select("facet_mask").Scan(&mask).Error)
			require.Equal(t, 31, mask)
		})
	}
}
