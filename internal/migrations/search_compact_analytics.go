package migrations

import (
	"ecommerce/internal/migrations/ops"
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
)

const searchCompactAnalyticsVersion = "2026101002_search_compact_analytics"
const searchCompactAnalyticsContractVersion = "2026101004_search_compact_analytics_contract"

// Frozen legacy inputs deliberately do not depend on current search/models.
type compactAnalyticsLegacyRow struct {
	ID                        uint
	ImpressionID              string
	FiltersJSON, ProductsJSON string
}
type compactAnalyticsLegacyFilters struct {
	Page, Limit               int
	BrandSlug                 string
	BrandSlugs, CategorySlugs []string
	HasVariantStock           *bool
	StockAvailability         []bool
	PriceRanges               []json.RawMessage
	Attributes                map[string]string
	AttributeValues           map[string][]string
	MinPrice, MaxPrice        *float64
}

func migrateSearchCompactAnalytics(tx *gorm.DB) error {
	for _, c := range []struct{ name, definition string }{{"snapshot_id", "TEXT NOT NULL DEFAULT ''"}, {"first_position", "BIGINT NOT NULL DEFAULT 1"}, {"product_count", "BIGINT NOT NULL DEFAULT 0"}, {"facet_mask", "BIGINT NOT NULL DEFAULT 0"}} {
		if err := ops.AddColumnIfNotExists(tx, "search_query_events", c.name, c.definition); err != nil {
			return err
		}
	}
	var cursor uint
	for {
		var rows []compactAnalyticsLegacyRow
		if err := tx.Table("search_query_events").Select("id,impression_id,filters_json,products_json").Where("id>? AND snapshot_id = ''", cursor).Order("id").Limit(500).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			cursor = row.ID
			var f compactAnalyticsLegacyFilters
			var products []uint
			// Reserved pre-consent rows may have empty data. Preserve them without
			// fabricating exposure; corrupt consented rows fail rather than lose attribution.
			if row.ImpressionID != "" {
				if err := json.Unmarshal([]byte(row.FiltersJSON), &f); err != nil {
					return fmt.Errorf("invalid legacy search filters for event %d: %w", row.ID, err)
				}
				if err := json.Unmarshal([]byte(row.ProductsJSON), &products); err != nil {
					return fmt.Errorf("invalid legacy search exposure for event %d: %w", row.ID, err)
				}
			}
			if f.Page < 1 {
				f.Page = 1
			}
			if f.Limit < 1 {
				f.Limit = 10
			}
			mask := 0
			for i, selected := range []bool{f.BrandSlug != "" || len(f.BrandSlugs) > 0, len(f.CategorySlugs) > 0, f.HasVariantStock != nil || len(f.StockAvailability) > 0, len(f.PriceRanges) > 0 || f.MinPrice != nil || f.MaxPrice != nil, len(f.Attributes) > 0 || len(f.AttributeValues) > 0} {
				if selected {
					mask |= 1 << i
				}
			}
			if err := tx.Table("search_query_events").Where("id=?", row.ID).Updates(map[string]any{"snapshot_id": fmt.Sprintf("legacy:%d", row.ID), "first_position": (f.Page-1)*f.Limit + 1, "product_count": len(products), "facet_mask": mask}).Error; err != nil {
				return err
			}
		}
	}
	return tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_search_session_snapshot ON search_query_events (session_hash,snapshot_id)`).Error
}

func migrateSearchCompactAnalyticsContract(tx *gorm.DB) error {
	if tx.Migrator().HasColumn("search_query_events", "result_json") {
		return tx.Exec(`ALTER TABLE search_query_events DROP COLUMN result_json`).Error
	}
	return nil
}
