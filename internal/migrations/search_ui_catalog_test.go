package migrations

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Only migration-era scalar columns are used for replay setup and assertions.
type searchUICatalogValueRow struct {
	ID               uint
	TranslationKeyID uint
	LocaleID         uint
	Value            string
	State            string
	Version          uint
}

type searchUICatalogReleaseRow struct {
	ID           uint
	SnapshotHash string
}

func TestSearchUICatalogUpgradePreservesMerchantValuesAndDrafts(t *testing.T) {
	db := newTestDB(t)
	index := slices.IndexFunc(orderedMigrations, func(step Migration) bool { return step.Version == searchUICatalogVersion })
	require.Greater(t, index, 0)
	require.NoError(t, runWithMigrations(db, orderedMigrations[:index]))
	var before searchUICatalogValueRow
	require.NoError(t, db.Table("translation_values").Select("id,translation_key_id,locale_id,value,state,version").Order("id").First(&before).Error)
	var release searchUICatalogReleaseRow
	require.NoError(t, db.Table("translation_releases").Select("id,snapshot_hash").Where("status = ?", "active").First(&release).Error)
	var locale struct{ ID uint }
	require.NoError(t, db.Table("locales").Select("id").Where("is_default = ?", true).First(&locale).Error)

	// A merchant may already have authored one of the newly introduced keys.
	customKey := searchUICatalogKeys[0]
	draftKey := searchUICatalogKeys[1]
	createKey := func(key localizationP1BaselineKey, source string) uint {
		now := time.Now().UTC()
		require.NoError(t, db.Table("translation_keys").Create(map[string]any{
			"namespace": key.Namespace, "key": key.Key, "source_text": source,
			"description": "Merchant-authored", "owner_domain": key.OwnerDomain,
			"is_deprecated": false, "created_at": now, "updated_at": now,
		}).Error)
		var row struct{ ID uint }
		require.NoError(t, db.Table("translation_keys").Select("id").Where("namespace = ? AND key = ?", key.Namespace, key.Key).First(&row).Error)
		return row.ID
	}
	createValue := func(keyID uint, state, value string) searchUICatalogValueRow {
		now := time.Now().UTC()
		require.NoError(t, db.Table("translation_values").Create(map[string]any{
			"translation_key_id": keyID, "locale_id": locale.ID,
			"value": value, "state": state, "version": 1, "created_at": now, "updated_at": now,
		}).Error)
		var row searchUICatalogValueRow
		require.NoError(t, db.Table("translation_values").Select("id,translation_key_id,locale_id,value,state,version").Where("translation_key_id = ? AND locale_id = ?", keyID, locale.ID).First(&row).Error)
		return row
	}
	customID := createKey(customKey, "Merchant source")
	custom := createValue(customID, "published", "Merchant published translation")
	draftID := createKey(draftKey, draftKey.SourceText)
	draft := createValue(draftID, "draft", "Keep this unfinished edit")

	require.NoError(t, runWithMigrations(db, orderedMigrations[:index+1]))
	for _, original := range []searchUICatalogValueRow{before, custom, draft} {
		var retained searchUICatalogValueRow
		require.NoError(t, db.Table("translation_values").Select("id,translation_key_id,locale_id,value,state,version").Where("id = ?", original.ID).First(&retained).Error)
		require.Equal(t, original, retained)
	}
	var source struct{ SourceText string }
	require.NoError(t, db.Table("translation_keys").Select("source_text").Where("id = ?", customID).First(&source).Error)
	require.Equal(t, "Merchant source", source.SourceText)
	var newPublished searchUICatalogValueRow
	require.NoError(t, db.Table("translation_values").Select("id,translation_key_id,locale_id,value,state,version").Where("translation_key_id = ? AND locale_id = ? AND state = ?", draftID, locale.ID, "published").First(&newPublished).Error)
	require.Equal(t, draftKey.SourceText, newPublished.Value)
	require.EqualValues(t, 2, newPublished.Version)

	for _, key := range searchUICatalogKeys {
		var row struct{ Value string }
		require.NoError(t, db.Table("translation_keys AS k").Select("v.value").Joins("JOIN translation_values v ON v.translation_key_id=k.id JOIN translation_release_entries e ON e.translation_value_id=v.id JOIN translation_releases r ON r.id=e.release_id").Where("k.namespace = ? AND k.key = ? AND v.locale_id = ? AND v.state = ? AND r.status = ?", key.Namespace, key.Key, locale.ID, "published", "active").Take(&row).Error)
		if key.Namespace == customKey.Namespace && key.Key == customKey.Key {
			require.Equal(t, custom.Value, row.Value)
		} else {
			require.Equal(t, key.SourceText, row.Value, key.Namespace+"."+key.Key)
		}
	}
	require.NoError(t, localizationDefaultSourceCatalogPublished(db))
	var after searchUICatalogReleaseRow
	require.NoError(t, db.Table("translation_releases").Select("id,snapshot_hash").Where("status = ?", "active").First(&after).Error)
	require.Equal(t, release.ID, after.ID)
	require.NotEqual(t, release.SnapshotHash, after.SnapshotHash)
	counts := func() []int64 {
		result := make([]int64, 3)
		for i, table := range []string{"translation_keys", "translation_values", "translation_release_entries"} {
			require.NoError(t, db.Table(table).Count(&result[i]).Error)
		}
		return result
	}
	beforeRepeat := counts()
	require.NoError(t, db.Transaction(migrateSearchUICatalog))
	require.Equal(t, beforeRepeat, counts())
	var repeated searchUICatalogReleaseRow
	require.NoError(t, db.Table("translation_releases").Select("id,snapshot_hash").Where("status = ?", "active").First(&repeated).Error)
	require.Equal(t, after, repeated)
}
