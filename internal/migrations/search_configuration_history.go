package migrations

import (
	"ecommerce/internal/migrations/ops"
	"encoding/json"
	"gorm.io/gorm"
	"time"
)

const searchConfigurationHistoryVersion = "2026101005_search_configuration_history"

type searchConfigurationAuditSchema struct {
	ID         uint   `gorm:"primaryKey"`
	Kind       string `gorm:"size:32;not null;index:idx_search_configuration_history,priority:1"`
	EntityID   uint   `gorm:"not null;index:idx_search_configuration_history,priority:2"`
	Operation  string `gorm:"size:16;not null"`
	ActorID    *uint
	BeforeJSON *string   `gorm:"type:text"`
	AfterJSON  *string   `gorm:"type:text"`
	CreatedAt  time.Time `gorm:"not null;index"`
}

func (searchConfigurationAuditSchema) TableName() string { return "search_configuration_audits" }
func migrateSearchConfigurationHistory(tx *gorm.DB) error {
	if err := ops.CreateTableIfNotExists(tx.Session(&gorm.Session{NewDB: true}), &searchConfigurationAuditSchema{}); err != nil {
		return err
	}
	// Freeze the baseline query columns; future model fields do not affect replay.
	for _, source := range []struct{ kind, table, columns string }{
		{"synonym", "search_synonym_sets", "id, name, direction, terms_json, is_active, updated_by, created_at, updated_at"},
		{"typo_profile", "search_typo_tolerance_profiles", "id, name, minimum_token_length, one_edit_minimum_length, two_edit_minimum_length, strict_mode, is_active, updated_by, created_at, updated_at"},
		{"ranking_profile", "search_ranking_profiles", "id, name, weights_json, is_default, version, updated_by, created_at, updated_at"},
	} {
		var rows []map[string]any
		if err := tx.Session(&gorm.Session{NewDB: true}).Table(source.table).Select(source.columns).Order("id").Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			var count int64
			if err := tx.Session(&gorm.Session{NewDB: true}).Table("search_configuration_audits").Where("kind = ? AND entity_id = ? AND operation = ?", source.kind, row["id"], "baseline").Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				continue
			}
			raw, err := json.Marshal(row)
			if err != nil {
				return err
			}
			if err := tx.Session(&gorm.Session{NewDB: true}).Table("search_configuration_audits").Create(map[string]any{"kind": source.kind, "entity_id": row["id"], "operation": "baseline", "actor_id": nil, "before_json": nil, "after_json": string(raw), "created_at": time.Now().UTC()}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
