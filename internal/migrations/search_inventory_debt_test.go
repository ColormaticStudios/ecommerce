package migrations

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInventoryDebtMigrationIsAdditiveAndIdempotent(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.AutoMigrate(&platformJobsP1QueueSchema{}, &searchP0IndexStateSchema{}, &searchProductRevisionSchema{}))
	now := time.Now().UTC()
	require.NoError(t, db.Create(&searchP0IndexStateSchema{Name: "products", DocumentCount: 7, LastFullReindexAt: &now}).Error)
	require.NoError(t, db.Create(&searchProductRevisionSchema{ProductID: 9, Revision: 42, ChangedAt: now, PendingSince: &now}).Error)
	require.NoError(t, db.Transaction(migrateSearchInventoryDebt))
	require.True(t, db.Migrator().HasColumn("search_index_states", "last_full_reindex_source_at"))
	require.NoError(t, db.Create(&searchInventoryRevisionSchema{VariantID: 4, ProductID: 9, Revision: 12, ChangedAt: now, PendingSince: &now}).Error)
	require.NoError(t, db.Transaction(migrateSearchInventoryDebt))
	var old searchProductRevisionSchema
	require.NoError(t, db.First(&old, "product_id = ?", 9).Error)
	require.EqualValues(t, 42, old.Revision)
	require.NotNil(t, old.PendingSince)
	var stock searchInventoryRevisionSchema
	require.NoError(t, db.First(&stock, "variant_id = ?", 4).Error)
	require.EqualValues(t, 12, stock.Revision)
	var source struct{ LastFullReindexSourceAt *time.Time }
	require.NoError(t, db.Table("search_index_states").Where("name = ?", "products").Select("last_full_reindex_source_at").Scan(&source).Error)
	require.Nil(t, source.LastFullReindexSourceAt, "upgrade must not invent a successful source capture")
}

func TestInventoryDebtMigrationRollsBackWithCaller(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.AutoMigrate(&platformJobsP1QueueSchema{}, &searchP0IndexStateSchema{}))
	require.ErrorIs(t, db.Transaction(func(tx *gorm.DB) error {
		require.NoError(t, migrateSearchInventoryDebt(tx))
		return gorm.ErrInvalidTransaction
	}), gorm.ErrInvalidTransaction)
	require.False(t, db.Migrator().HasTable("search_inventory_revisions"))
	require.False(t, db.Migrator().HasColumn("search_index_states", "last_full_reindex_source_at"))
}
