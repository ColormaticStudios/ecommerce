package migrations

import (
	"gorm.io/gorm"
	"time"
)

const searchAttributeLocalizationVersion = "2026101006_search_attribute_localization"

// This frozen SQL backfill preserves the existing merchant attribute heading
// in the global-default locale without introducing a runtime column fallback.
func migrateSearchAttributeLocalization(tx *gorm.DB) error {
	return tx.Exec(`INSERT INTO localized_entity_values (created_at,updated_at,entity_type,entity_id,locale_id,field,value)
 SELECT ?,?,'product_attribute',a.id,l.id,'key',a.key FROM product_attributes a
 JOIN locales l ON l.is_default = ? AND l.is_enabled = ? AND l.deleted_at IS NULL
 WHERE a.deleted_at IS NULL AND a.key<>''
 ON CONFLICT (entity_type,entity_id,locale_id,field) DO NOTHING`, time.Now().UTC(), time.Now().UTC(), true, true).Error
}
