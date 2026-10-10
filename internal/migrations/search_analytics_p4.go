package migrations

import (
	"time"

	"ecommerce/internal/migrations/ops"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const searchAnalyticsP4Version = "2026100701_search_analytics_p4"

type searchP4CartAttributionSchema struct {
	CartItemID   uint      `gorm:"primaryKey;autoIncrement:false"`
	ClickID      string    `gorm:"not null;size:64;index"`
	ImpressionID string    `gorm:"not null;size:64;index"`
	ProductID    uint      `gorm:"not null"`
	CreatedAt    time.Time `gorm:"not null;index"`
}
type searchP4OrderAttributionSchema struct {
	ID           uint       `gorm:"primaryKey"`
	OrderID      uint       `gorm:"not null;uniqueIndex:idx_search_order_product,priority:1;index"`
	ProductID    uint       `gorm:"not null;uniqueIndex:idx_search_order_product,priority:2"`
	ImpressionID string     `gorm:"not null;size:64;index"`
	ClickID      string     `gorm:"not null;size:64"`
	CreatedAt    time.Time  `gorm:"not null;index"`
	PaidAt       *time.Time `gorm:"index"`
}

func (searchP4CartAttributionSchema) TableName() string  { return "search_cart_attributions" }
func (searchP4OrderAttributionSchema) TableName() string { return "search_order_attributions" }
func migrateSearchAnalyticsP4Schema(tx *gorm.DB) error {
	columns := []struct{ table, name, definition string }{
		{"search_index_states", "generation", "BIGINT NOT NULL DEFAULT 0"},
		{"search_query_events", "impression_id", "VARCHAR(64)"},
		{"search_query_events", "session_hash", "VARCHAR(64)"},
		{"search_query_events", "products_json", "TEXT"},
		{"search_query_events", "result_json", "TEXT"},
		{"search_click_events", "event_id", "VARCHAR(64)"},
		{"search_click_events", "impression_id", "VARCHAR(64)"},
		{"search_click_events", "session_hash", "VARCHAR(64)"},
		{"search_click_events", "product_id", "INTEGER"},
	}
	for _, c := range columns {
		if err := ops.AddColumnIfNotExists(tx.Session(&gorm.Session{NewDB: true}), c.table, c.name, c.definition); err != nil {
			return err
		}
	}
	for _, sql := range []string{"CREATE UNIQUE INDEX IF NOT EXISTS idx_search_query_events_impression_id ON search_query_events(impression_id)", "CREATE INDEX IF NOT EXISTS idx_search_query_events_session_hash ON search_query_events(session_hash)", "CREATE UNIQUE INDEX IF NOT EXISTS idx_search_click_events_event_id ON search_click_events(event_id)", "CREATE INDEX IF NOT EXISTS idx_search_click_events_impression_id ON search_click_events(impression_id)", "CREATE INDEX IF NOT EXISTS idx_search_click_events_session_hash ON search_click_events(session_hash)", "CREATE INDEX IF NOT EXISTS idx_search_click_events_product_id ON search_click_events(product_id)"} {
		if err := tx.Session(&gorm.Session{NewDB: true}).Exec(sql).Error; err != nil {
			return err
		}
	}
	for _, m := range []any{&searchP4RevokedSessionSchema{}, &searchP4CartAttributionSchema{}, &searchP4OrderAttributionSchema{}} {
		if err := ops.CreateTableIfNotExists(tx.Session(&gorm.Session{NewDB: true}), m); err != nil {
			return err
		}
	}
	return nil
}

func migrateSearchAnalyticsP4(tx *gorm.DB) error {
	if err := migrateSearchAnalyticsP4Schema(tx); err != nil {
		return err
	}

	now := time.Now().UTC()
	for _, key := range searchAnalyticsP4CatalogKeys {
		row := map[string]any{"namespace": key.Namespace, "key": key.Key, "source_text": key.SourceText, "description": "Search administration navigation", "owner_domain": key.OwnerDomain, "is_deprecated": false, "created_at": now, "updated_at": now}
		if err := tx.Session(&gorm.Session{NewDB: true}).Table("translation_keys").Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "namespace"}, {Name: "key"}}, DoNothing: true}).Create(row).Error; err != nil {
			return err
		}
	}
	return publishLocalizationDefaultSourceCatalog(tx)
}

type searchP4RevokedSessionSchema struct {
	SessionHash string    `gorm:"primaryKey;size:64"`
	CreatedAt   time.Time `gorm:"not null;index"`
}

func (searchP4RevokedSessionSchema) TableName() string { return "search_revoked_sessions" }
