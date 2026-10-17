package migrations

import (
	"time"

	"ecommerce/internal/migrations/ops"
	"gorm.io/gorm"
)

const searchInventoryDebtVersion = "2026101201_search_inventory_debt"

// Frozen additive schema; stock debt starts with new inventory mutations.
// Existing product projection debt and upgrade reindex intent remain intact.
type searchInventoryRevisionSchema struct {
	VariantID    uint       `gorm:"primaryKey;autoIncrement:false"`
	ProductID    uint       `gorm:"not null;index:idx_search_inventory_revisions_product_id"`
	Revision     int64      `gorm:"not null"`
	ChangedAt    time.Time  `gorm:"not null"`
	PendingSince *time.Time `gorm:"index:idx_search_inventory_revisions_pending_since"`
}

func (searchInventoryRevisionSchema) TableName() string { return "search_inventory_revisions" }

type searchCatalogFanoutDebtSchema struct {
	JobID        string     `gorm:"primaryKey;size:36"`
	PendingSince time.Time  `gorm:"not null;index:idx_search_catalog_fanout_debts_pending_since"`
	CoveredAt    *time.Time `gorm:"index:idx_search_catalog_fanout_debts_covered_at"`
}

func (searchCatalogFanoutDebtSchema) TableName() string { return "search_catalog_fanout_debts" }

func migrateSearchInventoryDebt(tx *gorm.DB) error {
	if err := ops.CreateTableIfNotExists(tx.Session(&gorm.Session{NewDB: true}), &searchInventoryRevisionSchema{}); err != nil {
		return err
	}
	if err := ops.CreateTableIfNotExists(tx.Session(&gorm.Session{NewDB: true}), &searchCatalogFanoutDebtSchema{}); err != nil {
		return err
	}
	if err := ops.AddColumnIfNotExists(tx.Session(&gorm.Session{NewDB: true}), "search_index_states", "last_full_reindex_source_at", "TIMESTAMP"); err != nil {
		return err
	}
	return tx.Exec(`INSERT INTO search_catalog_fanout_debts (job_id, pending_since)
 SELECT id, created_at FROM job_queue WHERE job_type = 'search.catalog_fanout' AND status <> 'succeeded'
 ON CONFLICT (job_id) DO NOTHING`).Error
}
