package migrations

import (
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
	"time"
)

type searchRevisionLegacyProduct struct {
	ID uint `gorm:"primaryKey"`
}

func (searchRevisionLegacyProduct) TableName() string { return "products" }
func TestProjectionRevisionMigrationRepairsProductsAndOrphansIdempotently(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.AutoMigrate(&searchRevisionLegacyProduct{}, &searchP0DocumentSchema{}, &platformJobsP1QueueSchema{}))
	require.NoError(t, db.Create(&searchRevisionLegacyProduct{ID: 1}).Error)
	future := time.Now().Add(time.Hour).UnixNano()
	require.NoError(t, db.Create(&searchP0DocumentSchema{EntityType: "product", EntityID: 2, Version: future, PayloadJSON: "{}"}).Error)
	require.NoError(t, db.Transaction(migrateSearchProjectionRevisions))
	var rows []searchProductRevisionSchema
	require.NoError(t, db.Order("product_id").Find(&rows).Error)
	require.Len(t, rows, 2)
	require.Equal(t, uint(1), rows[0].ProductID)
	require.Equal(t, uint(2), rows[1].ProductID)
	require.Equal(t, future+1, rows[1].Revision)
	require.Zero(t, rows[1].IndexedRevision)
	require.NotNil(t, rows[1].PendingSince)
	before := rows[0].Revision
	require.NoError(t, db.Transaction(migrateSearchProjectionRevisions))
	require.NoError(t, db.Order("product_id").Find(&rows).Error)
	require.Equal(t, before, rows[0].Revision)
	var jobs int64
	require.NoError(t, db.Table("job_queue").Count(&jobs).Error)
	require.EqualValues(t, 1, jobs)
}
func TestProjectionRevisionMigrationRollsBackWithCaller(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.AutoMigrate(&searchRevisionLegacyProduct{}, &searchP0DocumentSchema{}, &platformJobsP1QueueSchema{}))
	require.NoError(t, db.Create(&searchRevisionLegacyProduct{ID: 1}).Error)
	require.ErrorIs(t, db.Transaction(func(tx *gorm.DB) error {
		require.NoError(t, migrateSearchProjectionRevisions(tx))
		return gorm.ErrInvalidTransaction
	}), gorm.ErrInvalidTransaction)
	require.False(t, db.Migrator().HasTable("search_product_revisions"))
	var jobs int64
	require.NoError(t, db.Table("job_queue").Count(&jobs).Error)
	require.Zero(t, jobs)
}
