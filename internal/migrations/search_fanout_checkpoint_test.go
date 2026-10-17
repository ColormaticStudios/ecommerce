package migrations

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Frozen new schema for replay assertions; never use the application's model.
type searchFanoutCheckpointSchema struct {
	JobID         string    `gorm:"primaryKey;size:36"`
	PendingSince  time.Time `gorm:"not null"`
	CoveredAt     *time.Time
	LastProductID uint `gorm:"not null;default:0"`
}

func (searchFanoutCheckpointSchema) TableName() string { return "search_catalog_fanout_debts" }

func TestSearchFanoutCheckpointUpgradePreservesDebtAndProgress(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.AutoMigrate(&searchCatalogFanoutDebtSchema{}))
	now := time.Now().UTC()
	require.NoError(t, db.Create(&searchCatalogFanoutDebtSchema{JobID: "old-pending", PendingSince: now}).Error)
	require.NoError(t, db.Create(&searchCatalogFanoutDebtSchema{JobID: "old-covered", PendingSince: now, CoveredAt: &now}).Error)
	require.NoError(t, db.Transaction(migrateSearchFanoutCheckpoint))
	var rows []searchFanoutCheckpointSchema
	require.NoError(t, db.Order("job_id").Find(&rows).Error)
	require.Len(t, rows, 2)
	require.Zero(t, rows[0].LastProductID)
	require.NotNil(t, rows[0].CoveredAt)
	require.True(t, rows[0].PendingSince.Equal(now))
	require.Zero(t, rows[1].LastProductID)
	require.Nil(t, rows[1].CoveredAt)
	require.NoError(t, db.Model(&searchFanoutCheckpointSchema{}).Where("job_id = ?", "old-pending").Update("last_product_id", 250).Error)
	require.NoError(t, db.Transaction(migrateSearchFanoutCheckpoint))
	var pending searchFanoutCheckpointSchema
	require.NoError(t, db.First(&pending, "job_id = ?", "old-pending").Error)
	require.EqualValues(t, 250, pending.LastProductID, "migration replay never resets committed progress")
}

func TestSearchFanoutCheckpointMigrationRollsBackWithCaller(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.AutoMigrate(&searchCatalogFanoutDebtSchema{}))
	require.ErrorIs(t, db.Transaction(func(tx *gorm.DB) error {
		require.NoError(t, migrateSearchFanoutCheckpoint(tx))
		return gorm.ErrInvalidTransaction
	}), gorm.ErrInvalidTransaction)
	require.False(t, db.Migrator().HasColumn("search_catalog_fanout_debts", "last_product_id"))
}
