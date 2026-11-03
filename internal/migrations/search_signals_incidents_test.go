package migrations

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type searchLegacyCostVariant struct {
	ID    uint `gorm:"primaryKey"`
	Price float64
}

func TestSearchSignalsSchemaPreservesExistingVariantsAndReplays(t *testing.T) {
	db := newTestDB(t)
	for _, table := range []string{"product_variants", "product_variant_drafts"} {
		require.NoError(t, db.Table(table).AutoMigrate(&searchLegacyCostVariant{}))
		require.NoError(t, db.Table(table).Create(map[string]any{"id": 1, "price": 25}).Error)
	}
	require.NoError(t, db.AutoMigrate(&searchP0DocumentSchema{}, &searchP0IndexStateSchema{}))
	require.NoError(t, db.Transaction(migrateSearchSignalsIncidentsSchema))
	require.NoError(t, db.Transaction(migrateSearchSignalsIncidentsSchema))
	for _, table := range []string{"product_variants", "product_variant_drafts"} {
		var row struct {
			Price    float64
			UnitCost *float64
		}
		require.NoError(t, db.Table(table).Where("id = 1").Take(&row).Error)
		require.Equal(t, 25.0, row.Price)
		require.Nil(t, row.UnitCost)
	}
	now := time.Now().UTC()
	require.NoError(t, db.Create(&searchIncidentSchema{IndexName: "products", Reason: "index_lag", OpenedAt: now, DetectedAt: now, LastObservedAt: now}).Error)
	require.Error(t, db.Create(&searchIncidentSchema{IndexName: "products", Reason: "index_lag", OpenedAt: now, DetectedAt: now, LastObservedAt: now}).Error)
}

func TestSearchSignalsMigrationKeepsWeightsAndQueuesOneBackfill(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, Run(db))
	var profile struct {
		ID          uint
		WeightsJSON string
		Version     int
	}
	require.NoError(t, db.Table("search_ranking_profiles").Where("is_default = ?", true).Take(&profile).Error)
	var before map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(profile.WeightsJSON), &before))
	delete(before, "margin")
	delete(before, "conversion")
	old, err := json.Marshal(before)
	require.NoError(t, err)
	require.NoError(t, db.Table("search_ranking_profiles").Where("id = ?", profile.ID).Update("weights_json", string(old)).Error)
	require.NoError(t, db.Transaction(migrateSearchSignalsIncidents))
	var after struct {
		WeightsJSON string
		Version     int
	}
	require.NoError(t, db.Table("search_ranking_profiles").Where("id = ?", profile.ID).Take(&after).Error)
	var weights map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(after.WeightsJSON), &weights))
	require.Equal(t, json.RawMessage("0"), weights["margin"])
	require.Equal(t, json.RawMessage("0"), weights["conversion"])
	for name, value := range before {
		require.Equal(t, value, weights[name])
	}
	require.Equal(t, profile.Version+1, after.Version)
	require.NoError(t, db.Transaction(migrateSearchSignalsIncidents))
	var count int64
	require.NoError(t, db.Table("job_queue").Where("idempotency_key = ?", "full:search-signals-2026100801").Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestSearchSignalsMigrationRollsBackWithCaller(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, Run(db))
	var profile struct {
		ID          uint
		WeightsJSON string
		Version     int
	}
	require.NoError(t, db.Table("search_ranking_profiles").Where("is_default = ?", true).Take(&profile).Error)
	var weights map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(profile.WeightsJSON), &weights))
	delete(weights, "margin")
	delete(weights, "conversion")
	legacy, err := json.Marshal(weights)
	require.NoError(t, err)
	require.NoError(t, db.Table("search_ranking_profiles").Where("id = ?", profile.ID).Update("weights_json", string(legacy)).Error)
	require.NoError(t, db.Exec("DELETE FROM job_queue WHERE idempotency_key = ?", "full:search-signals-2026100801").Error)
	rollback := errors.New("force rollback")
	require.ErrorIs(t, db.Transaction(func(tx *gorm.DB) error {
		if err := migrateSearchSignalsIncidents(tx); err != nil {
			return err
		}
		return rollback
	}), rollback)
	var after struct {
		WeightsJSON string
		Version     int
	}
	require.NoError(t, db.Table("search_ranking_profiles").Where("id = ?", profile.ID).Take(&after).Error)
	require.Equal(t, string(legacy), after.WeightsJSON)
	require.Equal(t, profile.Version, after.Version)
	var count int64
	require.NoError(t, db.Table("job_queue").Where("idempotency_key = ?", "full:search-signals-2026100801").Count(&count).Error)
	require.Zero(t, count)
}

func TestSearchSignalsMigrationRepairsMissingSearchSourceKeys(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, Run(db))
	require.NoError(t, db.Exec("DELETE FROM translation_release_entries WHERE translation_value_id IN (SELECT id FROM translation_values WHERE translation_key_id IN (SELECT id FROM translation_keys WHERE namespace = ? AND key = ?))", "admin", "navigation.search_operations").Error)
	require.NoError(t, db.Exec("DELETE FROM translation_values WHERE translation_key_id IN (SELECT id FROM translation_keys WHERE namespace = ? AND key = ?)", "admin", "navigation.search_operations").Error)
	require.NoError(t, db.Exec("DELETE FROM translation_keys WHERE namespace = ? AND key = ?", "admin", "navigation.search_operations").Error)
	require.NoError(t, db.Transaction(migrateSearchSignalsIncidents))
	var count int64
	require.NoError(t, db.Table("translation_keys").Where("namespace = ? AND key = ?", "admin", "navigation.search_operations").Count(&count).Error)
	require.Equal(t, int64(1), count)
	require.NoError(t, localizationDefaultSourceCatalogPublished(db))
}
