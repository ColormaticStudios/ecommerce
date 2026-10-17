package search

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"ecommerce/internal/jobs"
	"ecommerce/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestProjectionDebtSurvivesDeadLetterUntilSuccessfulConvergence(t *testing.T) {
	db := searchTestDB(t)
	ctx := context.Background()
	product := models.Product{SKU: "DEBT", Name: "Debt product", IsPublished: true, Stock: 1}
	require.NoError(t, db.Create(&product).Error)
	service := NewService(db, nil, jobs.NewRuntime(db, jobs.Config{}))
	require.NoError(t, service.Reindex(ctx))
	changed := time.Now().UTC().Add(-10 * time.Minute)
	job, err := RecordProductChangeTx(ctx, service.jobs, db, product.ID, changed)
	require.NoError(t, err)
	require.NoError(t, db.Model(&job).Update("status", models.JobStatusDeadLetter).Error)
	health, err := service.Freshness(ctx)
	require.NoError(t, err)
	require.Equal(t, "stale", health.Status)
	require.Zero(t, health.PendingJobs)
	require.GreaterOrEqual(t, health.Lag, 10*time.Minute)
	require.NoError(t, service.SyncProduct(ctx, product.ID))
	health, err = service.Freshness(ctx)
	require.NoError(t, err)
	require.Equal(t, "healthy", health.Status)
	var stored models.JobQueue
	require.NoError(t, db.First(&stored, "id = ?", job.ID).Error)
	require.Equal(t, models.JobStatusDeadLetter, stored.Status, "convergence preserves job history")
}

func TestProjectionAcknowledgmentCannotClearNewerChanges(t *testing.T) {
	db := searchTestDB(t)
	runtime := jobs.NewRuntime(db, jobs.Config{})
	ctx := context.Background()
	old := time.Now().UTC().Add(-time.Minute)
	_, err := RecordProductChangeTx(ctx, runtime, db, 42, old)
	require.NoError(t, err)
	newer := old.Add(time.Second)
	_, err = RecordProductChangeTx(ctx, runtime, db, 42, newer)
	require.NoError(t, err)
	require.NoError(t, acknowledgeProjection(db, 42, old.UnixNano()))
	var state models.SearchProductRevision
	require.NoError(t, db.First(&state, "product_id = ?", 42).Error)
	require.Equal(t, newer.UnixNano(), state.Revision)
	require.Equal(t, old.UnixNano(), state.IndexedRevision)
	require.NotNil(t, state.PendingSince)
	require.Equal(t, old.UnixNano(), state.PendingSince.UnixNano())
	require.NoError(t, acknowledgeProjection(db, 42, newer.UnixNano()))
	state = models.SearchProductRevision{}
	require.NoError(t, db.First(&state, "product_id = ?", 42).Error)
	require.Nil(t, state.PendingSince)
}

func TestProjectionChangesWithEqualOrOlderTimestampsRemainDistinct(t *testing.T) {
	db := searchTestDB(t)
	runtime := jobs.NewRuntime(db, jobs.Config{})
	ctx := context.Background()
	changed := time.Now().UTC().Add(-time.Minute)
	first, err := RecordProductChangeTx(ctx, runtime, db, 42, changed)
	require.NoError(t, err)
	for i, timestamp := range []time.Time{changed, changed.Add(-time.Second)} {
		job, err := RecordProductChangeTx(ctx, runtime, db, 42, timestamp)
		require.NoError(t, err)
		require.Equal(t, first.ID, job.ID, "mutations join the one active delivery intent")
		var state models.SearchProductRevision
		require.NoError(t, db.First(&state, "product_id = ?", 42).Error)
		require.Equal(t, changed.UnixNano()+int64(i+1), state.Revision)
		require.NoError(t, acknowledgeProjection(db, 42, changed.UnixNano()))
		state = models.SearchProductRevision{}
		require.NoError(t, db.First(&state, "product_id = ?", 42).Error)
		require.NotNil(t, state.PendingSince, "an older projection cannot acknowledge a later mutation")
	}
}

func TestProjectionChangeRequiresRuntimeAndRollsBackWithCaller(t *testing.T) {
	db := searchTestDB(t)
	ctx := context.Background()
	_, err := RecordProductChangeTx(ctx, nil, db, 42, time.Now())
	require.ErrorContains(t, err, "runtime is required")
	require.ErrorIs(t, db.Transaction(func(tx *gorm.DB) error {
		_, err := RecordProductChangeTx(ctx, jobs.NewRuntime(db, jobs.Config{}), tx, 42, time.Now())
		require.NoError(t, err)
		return gorm.ErrInvalidTransaction
	}), gorm.ErrInvalidTransaction)
	var count int64
	require.NoError(t, db.Model(&models.SearchProductRevision{}).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Model(&models.JobQueue{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestProjectionAvailabilityAndStockUseOnlyPublishedVariants(t *testing.T) {
	product := models.Product{IsPublished: true, Stock: 999, Variants: []models.ProductVariant{
		{Stock: 0, IsPublished: true}, {Stock: 9, IsPublished: false},
	}}
	doc, ok, err := projectProduct(product, time.Now())
	require.NoError(t, err)
	require.True(t, ok)
	require.False(t, doc.Available)
	var copy models.Product
	require.NoError(t, json.Unmarshal([]byte(doc.PayloadJSON), &copy))
	require.Zero(t, copy.Stock)
	product.Variants[0].Stock = 3
	doc, ok, err = projectProduct(product, time.Now())
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, doc.Available)
	require.NoError(t, json.Unmarshal([]byte(doc.PayloadJSON), &copy))
	require.Equal(t, 3, copy.Stock)
}

func TestFullReindexReconcilesOrphanWithFutureCapturedRevision(t *testing.T) {
	db := searchTestDB(t)
	ctx := context.Background()
	future := time.Now().Add(time.Hour).UnixNano()
	require.NoError(t, NewDatabaseBackend(db).Upsert(ctx, models.SearchDocument{EntityType: ProductEntityType, EntityID: 42, PayloadJSON: `{"name":"Orphan"}`, Version: future, Active: true}))
	require.NoError(t, db.Create(&models.SearchProductRevision{ProductID: 42, Revision: future + 1, ChangedAt: time.Now(), PendingSince: timePointer(time.Now().Add(-10 * time.Minute))}).Error)
	require.NoError(t, NewService(db, nil, nil).Reindex(ctx))
	var document models.SearchDocument
	require.NoError(t, db.Where("entity_id = ?", 42).First(&document).Error)
	require.False(t, document.Active)
	require.Equal(t, future+1, document.Version)
	var state models.SearchProductRevision
	require.NoError(t, db.First(&state, "product_id = ?", 42).Error)
	require.Equal(t, future+1, state.IndexedRevision)
	require.Nil(t, state.PendingSince)
}
func timePointer(value time.Time) *time.Time { return &value }
