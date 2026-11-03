package migrations

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestSearchSQLRetrievalBackfillsFrozenDocuments(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`CREATE TABLE search_documents (id INTEGER PRIMARY KEY, entity_id INTEGER, entity_type TEXT, payload_json TEXT, searchable_text TEXT, brand_slug TEXT, category_tokens TEXT, source_created_at DATETIME, active BOOLEAN)`).Error)
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	for i := 1; i <= 251; i++ {
		payload := fmt.Sprintf(`{"name":"Été Shoe %d","brand":{"name":"North"},"categories":[{"slug":"trail","name":"Trail"}],"attribute_values":[{"text_value":"RÉD!","product_attribute":{"slug":"color","key":"Color","filterable":true}}]}`, i)
		require.NoError(t, db.Table("search_documents").Create(map[string]any{"id": i, "entity_id": i, "entity_type": "product", "payload_json": payload, "searchable_text": "été shoe red trail", "brand_slug": "Été-North!", "category_tokens": "trail", "source_created_at": now, "active": true}).Error)
	}
	require.NoError(t, db.Table("search_documents").Create(map[string]any{"id": 252, "entity_id": 252, "entity_type": "product", "payload_json": "invalid tombstone", "active": false}).Error)
	require.NoError(t, db.Transaction(migrateSearchSQLRetrieval))
	var count int64
	require.NoError(t, db.Table("search_product_ranks").Count(&count).Error)
	require.Equal(t, int64(251), count)
	var value struct{ Value, Label, Name string }
	require.NoError(t, db.Table("search_product_attributes").Where("product_id = ?", 251).Take(&value).Error)
	require.Equal(t, "réd", value.Value)
	require.Equal(t, "RÉD!", value.Label)
	require.Equal(t, "Color", value.Name)
	var brandSlug string
	require.NoError(t, db.Table("search_documents").Where("id = ?", 251).Pluck("brand_slug", &brandSlug).Error)
	require.Equal(t, "été north", brandSlug)
	require.NoError(t, db.Transaction(migrateSearchSQLRetrieval))
	require.NoError(t, db.Table("search_product_terms").Where("term = ?", "été").Count(&count).Error)
	require.Equal(t, int64(251), count)
}
