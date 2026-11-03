package migrations

import (
	"ecommerce/internal/migrations/ops"
	"gorm.io/gorm"
)

const searchFanoutCheckpointVersion = "2026101701_search_fanout_checkpoint"

// Old fanouts have no durable progress and safely begin at the first product.
// Keep the prior debt schema frozen rather than changing its historical shape.
func migrateSearchFanoutCheckpoint(tx *gorm.DB) error {
	return ops.AddColumnIfNotExists(tx, "search_catalog_fanout_debts", "last_product_id", "BIGINT NOT NULL DEFAULT 0")
}
