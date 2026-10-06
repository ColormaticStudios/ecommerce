package migrations

import (
	"ecommerce/internal/migrations/ops"
	"fmt"
	"gorm.io/gorm"
	"time"
)

const searchRankingP2Version = "2026100201_search_ranking_p2"

// Frozen migration schema; runtime models must not alter historical replay.
type searchP2SalesSignalSchema struct {
	ProductID   uint      `gorm:"primaryKey;autoIncrement:false"`
	Units30Days int64     `gorm:"not null;default:0"`
	AsOf        time.Time `gorm:"not null;index"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (searchP2SalesSignalSchema) TableName() string { return "search_sales_signals" }

func migrateSearchRankingP2(tx *gorm.DB) error {
	if err := ops.AddColumnIfNotExists(tx.Session(&gorm.Session{NewDB: true}), "search_ranking_profiles", "version", "INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
	}
	if err := ops.CreateTableIfNotExists(tx.Session(&gorm.Session{NewDB: true}), &searchP2SalesSignalSchema{}); err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, profile := range []struct{ Name, Weights string }{
		{"default", `{"token_coverage":8,"exact_phrase":6,"name":5,"brand":2,"attributes":2,"recency":0.5,"availability":1,"sales":1}`},
		{"new_arrivals", `{"token_coverage":8,"exact_phrase":6,"name":5,"brand":2,"attributes":2,"recency":4,"availability":1,"sales":0.5}`},
	} {
		if err := tx.Session(&gorm.Session{NewDB: true}).Exec(`INSERT INTO search_ranking_profiles (name, weights_json, version, is_default, created_at, updated_at) VALUES (?, ?, 1, false, ?, ?) ON CONFLICT (name) DO UPDATE SET weights_json = excluded.weights_json, version = 1, is_default = false, deleted_at = NULL, updated_at = excluded.updated_at WHERE search_ranking_profiles.deleted_at IS NOT NULL`, profile.Name, profile.Weights, now, now).Error; err != nil {
			return err
		}
	}
	// P0 reserved this table without an activation invariant. Normalize ownership
	// deterministically before installing the new uniqueness constraint.
	var rows []struct {
		ID        uint
		Name      string
		IsDefault bool
	}
	if err := tx.Session(&gorm.Session{NewDB: true}).Table("search_ranking_profiles").Where("deleted_at IS NULL").Order("id ASC").Find(&rows).Error; err != nil {
		return err
	}
	var selected uint
	for _, row := range rows {
		if row.IsDefault {
			selected = row.ID
			break
		}
	}
	if selected == 0 {
		for _, row := range rows {
			if row.Name == "default" {
				selected = row.ID
				break
			}
		}
	}
	if selected == 0 {
		return fmt.Errorf("search default ranking profile missing")
	}
	if err := tx.Session(&gorm.Session{NewDB: true}).Table("search_ranking_profiles").Where("id <> ?", selected).Update("is_default", false).Error; err != nil {
		return err
	}
	if err := tx.Session(&gorm.Session{NewDB: true}).Table("search_ranking_profiles").Where("id = ?", selected).Update("is_default", true).Error; err != nil {
		return err
	}
	return tx.Session(&gorm.Session{NewDB: true}).Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_search_ranking_profiles_one_default ON search_ranking_profiles (is_default) WHERE is_default = true AND deleted_at IS NULL`).Error
}
