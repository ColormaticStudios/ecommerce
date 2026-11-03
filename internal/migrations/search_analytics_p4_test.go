package migrations

import (
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestSearchAnalyticsP4PreservesReservedLegacyRows(t *testing.T) {
	db := newTestDB(t)
	for _, m := range []any{&searchP0QueryEventSchema{}, &searchP0ClickEventSchema{}, &searchP0IndexStateSchema{}} {
		require.NoError(t, db.AutoMigrate(m))
	}
	now := time.Now().UTC()
	require.NoError(t, db.Table("search_query_events").Create(map[string]any{"id": 1, "query": "legacy", "normalized_query": "legacy", "filters_json": "{}", "result_count": 1, "latency_ms": 2, "session_id": "legacy", "created_at": now}).Error)
	require.NoError(t, db.Transaction(migrateSearchAnalyticsP4Schema))
	var row searchP0QueryEventSchema
	require.NoError(t, db.First(&row, 1).Error)
	require.Equal(t, "legacy", row.Query)
	require.True(t, db.Migrator().HasColumn("search_query_events", "result_json"))
	require.True(t, db.Migrator().HasTable("search_revoked_sessions"))
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error { return migrateSearchAnalyticsP4Schema(tx) }))
}
