package migrations

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSearchConfigurationHistoryFrozenBaselineRetainsDeletedSources(t *testing.T) {
	db := newTestDB(t)
	for _, sql := range []string{
		`CREATE TABLE search_synonym_sets(id INTEGER PRIMARY KEY,name TEXT,direction TEXT,terms_json TEXT,is_active BOOLEAN,updated_by INTEGER,created_at DATETIME,updated_at DATETIME)`,
		`CREATE TABLE search_typo_tolerance_profiles(id INTEGER PRIMARY KEY,name TEXT,minimum_token_length INTEGER,one_edit_minimum_length INTEGER,two_edit_minimum_length INTEGER,strict_mode BOOLEAN,is_active BOOLEAN,updated_by INTEGER,created_at DATETIME,updated_at DATETIME)`,
		`CREATE TABLE search_ranking_profiles(id INTEGER PRIMARY KEY,name TEXT,weights_json TEXT,is_default BOOLEAN,version INTEGER,updated_by INTEGER,created_at DATETIME,updated_at DATETIME)`,
		`INSERT INTO search_synonym_sets(id,name,direction,terms_json,is_active) VALUES(1,'Trail vocabulary','both','["hike","trail"]',true)`,
		`INSERT INTO search_ranking_profiles(id,name,weights_json,is_default,version) VALUES(2,'Default','{"name":3}',true,7)`,
	} {
		require.NoError(t, db.Exec(sql).Error)
	}
	require.NoError(t, db.Transaction(migrateSearchConfigurationHistory))
	require.NoError(t, db.Transaction(migrateSearchConfigurationHistory))
	require.NoError(t, db.Exec("DELETE FROM search_synonym_sets").Error)
	var rows []searchConfigurationAuditSchema
	require.NoError(t, db.Order("id").Find(&rows).Error)
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.Equal(t, "baseline", row.Operation)
		require.Nil(t, row.ActorID)
		require.Nil(t, row.BeforeJSON)
		require.NotNil(t, row.AfterJSON)
		var snapshot map[string]any
		require.NoError(t, json.Unmarshal([]byte(*row.AfterJSON), &snapshot))
		if row.Kind == "synonym" {
			require.Equal(t, "Trail vocabulary", snapshot["name"])
		} else {
			require.EqualValues(t, 7, snapshot["version"])
		}
	}
}
