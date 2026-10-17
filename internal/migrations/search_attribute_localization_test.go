package migrations

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSearchAttributeLocalizationFrozenBackfill(t *testing.T) {
	db := newTestDB(t)
	// Frozen minimal historical schemas avoid coupling replay to current models.
	for _, sql := range []string{`CREATE TABLE product_attributes(id INTEGER PRIMARY KEY,key TEXT,deleted_at DATETIME)`, `CREATE TABLE locales(id INTEGER PRIMARY KEY,is_default BOOLEAN,is_enabled BOOLEAN,deleted_at DATETIME)`, `CREATE TABLE localized_entity_values(id INTEGER PRIMARY KEY,created_at DATETIME,updated_at DATETIME,entity_type TEXT,entity_id INTEGER,locale_id INTEGER,field TEXT,value TEXT,UNIQUE(entity_type,entity_id,locale_id,field))`, `INSERT INTO locales(id,is_default,is_enabled) VALUES(1,true,true)`, `INSERT INTO product_attributes(id,key) VALUES(3,'Color'),(4,'Size')`, `INSERT INTO localized_entity_values(entity_type,entity_id,locale_id,field,value) VALUES('product_attribute',4,1,'key','Existing translation')`} {
		require.NoError(t, db.Exec(sql).Error)
	}
	require.NoError(t, db.Transaction(migrateSearchAttributeLocalization))
	require.NoError(t, db.Transaction(migrateSearchAttributeLocalization))
	var rows []struct {
		EntityID     uint
		Field, Value string
	}
	require.NoError(t, db.Table("localized_entity_values").Order("entity_id").Scan(&rows).Error)
	require.Len(t, rows, 2)
	require.Equal(t, "key", rows[0].Field)
	require.Equal(t, "Color", rows[0].Value)
	require.Equal(t, "Existing translation", rows[1].Value)
}
