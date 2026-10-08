package migrations

import (
	"ecommerce/internal/migrations/ops"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const searchMerchandisingP3Version = "2026100601_search_merchandising_p3"

// This frozen schema deliberately has no rule or actor associations. Audit
// snapshots must survive hard deletion of either referenced entity.
type searchP3MerchandisingAuditSchema struct {
	ID         uint      `gorm:"primaryKey"`
	RuleID     uint      `gorm:"not null;index"`
	Operation  string    `gorm:"not null;size:16"`
	ActorID    *uint     `gorm:"index"`
	BeforeJSON *string   `gorm:"type:text"`
	AfterJSON  *string   `gorm:"type:text"`
	CreatedAt  time.Time `gorm:"not null;index"`
}

func (searchP3MerchandisingAuditSchema) TableName() string { return "search_merchandising_audits" }

func migrateSearchMerchandisingP3Schema(tx *gorm.DB) error {
	if err := ops.AddColumnIfNotExists(tx.Session(&gorm.Session{NewDB: true}), "search_merchandising_rules", "version", "INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
	}
	return ops.CreateTableIfNotExists(tx.Session(&gorm.Session{NewDB: true}), &searchP3MerchandisingAuditSchema{})
}

func migrateSearchMerchandisingP3(tx *gorm.DB) error {
	if err := migrateSearchMerchandisingP3Schema(tx); err != nil {
		return err
	}
	now := time.Now().UTC()

	for _, catalogKey := range searchMerchandisingP3CatalogKeys {
		key := map[string]any{
			"namespace": catalogKey.Namespace, "key": catalogKey.Key, "source_text": catalogKey.SourceText,
			"description": "Search merchandising administration navigation", "owner_domain": catalogKey.OwnerDomain,
			"is_deprecated": false, "created_at": now, "updated_at": now,
		}
		if err := tx.Session(&gorm.Session{NewDB: true}).Table("translation_keys").Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "namespace"}, {Name: "key"}}, DoNothing: true,
		}).Create(&key).Error; err != nil {
			return err
		}
	}

	return publishLocalizationDefaultSourceCatalog(tx)
}

var searchMerchandisingP3CatalogKeys = []localizationP1BaselineKey{
	{Namespace: "admin", Key: "navigation.search_merchandising", SourceText: "Search merchandising", OwnerDomain: "admin"},
}
