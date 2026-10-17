package migrations

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSearchReviewCatalogUpgradePreservesPublishedMessagesAndIsIdempotent(t *testing.T) {
	db := newTestDB(t)
	index := slices.IndexFunc(orderedMigrations, func(step Migration) bool { return step.Version == searchReviewCatalogVersion })
	require.Greater(t, index, 0)
	require.NoError(t, runWithMigrations(db, orderedMigrations[:index]))
	// Historical SQL columns keep upgrade assertions independent of live models.
	var before struct {
		ID    uint
		Value string
	}
	require.NoError(t, db.Table("translation_values").Select("id,value").Order("id").First(&before).Error)
	require.NoError(t, runWithMigrations(db, orderedMigrations[:index+1]))
	require.NoError(t, db.Transaction(migrateSearchReviewCatalog))
	var after struct {
		ID    uint
		Value string
	}
	require.NoError(t, db.Table("translation_values").Select("id,value").Where("id = ?", before.ID).First(&after).Error)
	require.Equal(t, before, after)
	for _, key := range searchReviewCatalogKeys {
		var count int64
		require.NoError(t, db.Table("translation_keys AS k").Joins("JOIN translation_values v ON v.translation_key_id=k.id JOIN translation_release_entries e ON e.translation_value_id=v.id JOIN translation_releases r ON r.id=e.release_id").Where("k.namespace=? AND k.key=? AND v.state='published' AND r.status='active'", key.Namespace, key.Key).Count(&count).Error)
		require.EqualValues(t, 1, count, key.Namespace+"."+key.Key)
	}
}
