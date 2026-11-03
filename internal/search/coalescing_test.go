package search

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ecommerce/internal/jobs"
	"ecommerce/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type completionBarrierBackend struct {
	SearchBackend
	written chan struct{}
	release chan struct{}
	held    atomic.Bool
}

func (b *completionBarrierBackend) Upsert(ctx context.Context, doc models.SearchDocument) error {
	if err := b.SearchBackend.Upsert(ctx, doc); err != nil {
		return err
	}
	if b.held.CompareAndSwap(false, true) {
		close(b.written)
		select {
		case <-b.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func TestProductSyncCoalescesAndHandsOffMutationDuringRunningCompletion(t *testing.T) {
	db := searchTestDB(t)
	runtime := jobs.NewRuntime(db, jobs.Config{MaxAttempts: 13})
	barrier := &completionBarrierBackend{SearchBackend: NewDatabaseBackend(db), written: make(chan struct{}), release: make(chan struct{})}
	service := NewService(db, barrier, runtime)
	require.NoError(t, service.RegisterJobHandlers())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	product := models.Product{SKU: "COALESCED", Name: "First name", IsPublished: true, Stock: 1}
	require.NoError(t, db.Create(&product).Error)
	var first models.JobQueue
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		var err error
		first, err = RecordProductChangeTx(ctx, runtime, tx, product.ID, time.Now())
		return err
	}))
	require.Equal(t, 13, first.MaxAttempts)
	for i := 0; i < 25; i++ {
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			job, err := RecordProductChangeTx(ctx, runtime, tx, product.ID, time.Now())
			require.Equal(t, first.ID, job.ID)
			return err
		}))
	}
	var count int64
	require.NoError(t, db.Model(&models.JobQueue{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	done := make(chan error, 1)
	go func() { done <- runtime.Execute(ctx, first.ID, "coalescing-worker") }()
	select {
	case <-barrier.written:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&product).Update("name", "Newest name").Error; err != nil {
			return err
		}
		joined, err := RecordProductChangeTx(ctx, runtime, tx, product.ID, time.Now())
		require.Equal(t, first.ID, joined.ID)
		return err
	}))
	close(barrier.release)
	require.NoError(t, <-done)
	var successor models.JobQueue
	require.NoError(t, db.Where("job_type = ? AND status = ?", JobTypeProductSync, models.JobStatusPending).First(&successor).Error)
	require.NotEqual(t, first.ID, successor.ID)
	require.NoError(t, runtime.Execute(ctx, successor.ID, "coalescing-worker"))
	var state models.SearchProductRevision
	require.NoError(t, db.First(&state, "product_id = ?", product.ID).Error)
	require.Nil(t, state.PendingSince)
	require.Equal(t, state.Revision, state.IndexedRevision)
	var document models.SearchDocument
	require.NoError(t, db.First(&document, "entity_id = ?", product.ID).Error)
	require.Contains(t, document.PayloadJSON, "Newest name")
	var original models.JobQueue
	require.NoError(t, db.First(&original, "id = ?", first.ID).Error)
	require.Equal(t, models.JobStatusSucceeded, original.Status)
	require.Nil(t, original.IdempotencyKey)
}

func TestIncrementalDocumentCountUsesAtomicActivityDeltas(t *testing.T) {
	db := searchTestDB(t)
	backend := NewDatabaseBackend(db)
	ctx := context.Background()
	counts := atomic.Int64{}
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("no_incremental_catalog_count", func(tx *gorm.DB) {
		if tx.Statement.Table == "search_documents" && strings.Contains(strings.ToLower(tx.Statement.SQL.String()), "count(") {
			counts.Add(1)
		}
	}))
	document := models.SearchDocument{EntityType: ProductEntityType, EntityID: 1, PayloadJSON: `{"name":"Counted"}`, Active: true, Version: 10}
	require.NoError(t, backend.Upsert(ctx, document))
	assertCount := func(expected int64) {
		var state models.SearchIndexState
		require.NoError(t, db.First(&state, "name = ?", ProductIndexName).Error)
		require.Equal(t, expected, state.DocumentCount)
	}
	assertCount(1)
	require.NoError(t, backend.Upsert(ctx, document))
	assertCount(1)
	require.NoError(t, backend.Delete(ctx, ProductEntityType, 1, 11))
	assertCount(0)
	require.NoError(t, backend.Upsert(ctx, document))
	assertCount(0)
	document.Version = 12
	require.NoError(t, backend.Upsert(ctx, document))
	assertCount(1)
	document.EntityID = 2
	require.NoError(t, backend.Upsert(ctx, document))
	assertCount(2)
	require.NoError(t, db.Callback().Query().Remove("no_incremental_catalog_count"))
	require.Zero(t, counts.Load())
}

func TestInventoryMissingRuntimeRollsBackStockAndUsesConfiguredPolicy(t *testing.T) {
	db := searchTestDB(t)
	product := models.Product{SKU: "POLICY", Name: "Policy", IsPublished: true}
	require.NoError(t, db.Create(&product).Error)
	variant := models.ProductVariant{ProductID: product.ID, SKU: "POLICY-V", Title: "Default", IsPublished: true}
	require.NoError(t, db.Create(&variant).Error)
	require.ErrorContains(t, RecordInventoryChangeTx(context.Background(), db, variant.ID), "configured job runtime")
	runtime := jobs.NewRuntime(db, jobs.Config{MaxAttempts: 19})
	bound := jobs.WithRuntime(db, runtime)
	require.NoError(t, bound.Transaction(func(tx *gorm.DB) error { return RecordInventoryChangeTx(context.Background(), tx, variant.ID) }))
	var job models.JobQueue
	require.NoError(t, db.First(&job).Error)
	require.Equal(t, 19, job.MaxAttempts)
}

func TestVariantDebtCoalescesWithoutProductLockAndPreservesRunningChanges(t *testing.T) {
	db := searchTestDB(t)
	runtime := jobs.NewRuntime(db, jobs.Config{})
	bound := jobs.WithRuntime(db, runtime)
	product := models.Product{SKU: "STOCK-DEBT", Name: "Stock debt", IsPublished: true}
	require.NoError(t, db.Create(&product).Error)
	variants := []models.ProductVariant{{ProductID: product.ID, SKU: "STOCK-A", Title: "A", Stock: 3, IsPublished: true}, {ProductID: product.ID, SKU: "STOCK-B", Title: "B", Stock: 4, IsPublished: true}}
	require.NoError(t, db.Create(&variants).Error)
	for i := 0; i < 20; i++ {
		require.NoError(t, bound.Transaction(func(tx *gorm.DB) error { return RecordInventoryChangeTx(context.Background(), tx, variants[i%2].ID) }))
	}
	var revisionCount int64
	require.NoError(t, db.Model(&models.SearchProductRevision{}).Count(&revisionCount).Error)
	require.Zero(t, revisionCount, "stock producers do not touch shared product revision")
	var active []models.JobQueue
	require.NoError(t, db.Where("job_type = ?", JobTypeProductSync).Find(&active).Error)
	require.Len(t, active, 2)
	barrier := &completionBarrierBackend{SearchBackend: NewDatabaseBackend(db), written: make(chan struct{}), release: make(chan struct{})}
	service := NewService(db, barrier, runtime)
	require.NoError(t, service.RegisterJobHandlers())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- runtime.Execute(ctx, active[0].ID, "stock-worker") }()
	select {
	case <-barrier.written:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var payload ProductSyncPayload
	require.NoError(t, json.Unmarshal([]byte(active[0].PayloadJSON), &payload))
	require.NoError(t, bound.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.ProductVariant{}).Where("id = ?", payload.VariantID).Update("stock", 0).Error; err != nil {
			return err
		}
		return RecordInventoryChangeTx(ctx, tx, payload.VariantID)
	}))
	close(barrier.release)
	require.NoError(t, <-done)
	var successor models.JobQueue
	require.NoError(t, db.Where("status = ? AND idempotency_key = ?", models.JobStatusPending, fmt.Sprintf("variant:%d:pending", payload.VariantID)).First(&successor).Error)
	require.NoError(t, runtime.Execute(ctx, successor.ID, "stock-worker"))
	var dirty int64
	require.NoError(t, db.Model(&models.SearchInventoryRevision{}).Where("pending_since IS NOT NULL").Count(&dirty).Error)
	require.Zero(t, dirty)
	var document models.SearchDocument
	require.NoError(t, db.First(&document, "entity_id = ?", product.ID).Error)
	var projected models.Product
	require.NoError(t, json.Unmarshal([]byte(document.PayloadJSON), &projected))
	for _, variant := range projected.Variants {
		if variant.ID == payload.VariantID {
			require.Zero(t, variant.Stock)
		}
	}
}

func TestFanoutDebtRemainsStaleAcrossLaterCompletionOfEarlierRebuild(t *testing.T) {
	db := searchTestDB(t)
	service := NewService(db, nil, jobs.NewRuntime(db, jobs.Config{}))
	now := time.Now().UTC()
	source := now.Add(-20 * time.Minute)
	complete := now.Add(-time.Minute)
	require.NoError(t, db.Create(&models.SearchIndexState{Name: ProductIndexName, LastFullReindexAt: &complete, LastFullReindexSourceAt: &source}).Error)
	failed := models.JobQueue{ID: "fanout-debt", JobType: JobTypeCatalogFanout, PayloadJSON: `{"version":1}`, PayloadFingerprint: "test", Status: models.JobStatusDeadLetter, RunAt: now, MaxAttempts: 1, CreatedAt: now.Add(-10 * time.Minute)}
	require.NoError(t, db.Create(&failed).Error)
	require.NoError(t, db.Create(&models.SearchCatalogFanoutDebt{JobID: failed.ID, PendingSince: failed.CreatedAt}).Error)
	health, err := service.Freshness(context.Background())
	require.NoError(t, err)
	require.Equal(t, "stale", health.Status)
	require.NoError(t, service.Reindex(context.Background()))
	health, err = service.Freshness(context.Background())
	require.NoError(t, err)
	require.Equal(t, "healthy", health.Status)
	var retained models.JobQueue
	require.NoError(t, db.First(&retained, "id = ?", failed.ID).Error)
	require.Equal(t, models.JobStatusDeadLetter, retained.Status)
}

func TestCatalogFanoutBatchesAndRetriesWithoutDuplicateActiveDeliveries(t *testing.T) {
	db := searchTestDB(t)
	ctx := context.Background()
	runtime := jobs.NewRuntime(db, jobs.Config{MaxAttempts: 2, Jitter: func(time.Duration) time.Duration { return 0 }})
	service := NewService(db, nil, runtime)
	require.NoError(t, service.RegisterJobHandlers())
	brand := models.Brand{Name: "Fanout brand", Slug: "fanout-brand", IsActive: true}
	require.NoError(t, db.Create(&brand).Error)
	products := make([]models.Product, 501)
	for i := range products {
		products[i] = models.Product{SKU: fmt.Sprintf("FANOUT-%d", i), Name: "Fanout product", BrandID: &brand.ID, IsPublished: true}
	}
	require.NoError(t, db.CreateInBatches(products, 100).Error)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error { return EnqueueCatalogFanoutTx(ctx, runtime, tx, "brand", []uint{brand.ID}) }))
	var job models.JobQueue
	require.NoError(t, db.First(&job).Error)
	var before int64
	require.NoError(t, db.Model(&models.SearchProductRevision{}).Count(&before).Error)
	require.Zero(t, before)
	creates := 0
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("fanout_batch_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "search_product_revisions" {
			creates++
			if creates == 251 {
				tx.AddError(errors.New("injected next batch failure"))
			}
		}
	}))
	require.Error(t, runtime.Execute(ctx, job.ID, "fanout-worker"))
	var partial int64
	require.NoError(t, db.Model(&models.SearchProductRevision{}).Count(&partial).Error)
	require.EqualValues(t, 250, partial, "committed batches survive later batch failure")
	var checkpoint models.SearchCatalogFanoutDebt
	require.NoError(t, db.First(&checkpoint, "job_id = ?", job.ID).Error)
	require.Equal(t, products[249].ID, checkpoint.LastProductID)
	require.Nil(t, checkpoint.CoveredAt)
	var firstRevision models.SearchProductRevision
	require.NoError(t, db.First(&firstRevision, "product_id = ?", products[0].ID).Error)
	require.NoError(t, db.Callback().Create().Remove("fanout_batch_failure"))
	// A new runtime/service simulates a process restart between attempts. Finish
	// an already expanded product first: retry must not recreate its delivery.
	var expanded models.JobQueue
	require.NoError(t, db.Where("job_type = ?", JobTypeProductSync).Order("created_at,id").First(&expanded).Error)
	require.NoError(t, runtime.Execute(ctx, expanded.ID, "product-worker"))
	restartedRuntime := jobs.NewRuntime(db, jobs.Config{MaxAttempts: 2, Jitter: func(time.Duration) time.Duration { return 0 }})
	restarted := NewService(db, nil, restartedRuntime)
	require.NoError(t, restarted.RegisterJobHandlers())
	require.NoError(t, restartedRuntime.Execute(ctx, job.ID, "fanout-restarted-worker"))
	var retainedRevision models.SearchProductRevision
	require.NoError(t, db.First(&retainedRevision, "product_id = ?", products[0].ID).Error)
	require.Equal(t, firstRevision.Revision, retainedRevision.Revision, "committed batches are never allocated again")
	require.NoError(t, db.Model(&models.SearchProductRevision{}).Count(&partial).Error)
	require.EqualValues(t, 501, partial)
	require.NoError(t, db.First(&checkpoint, "job_id = ?", job.ID).Error)
	require.Equal(t, products[500].ID, checkpoint.LastProductID)
	require.NotNil(t, checkpoint.CoveredAt)
	require.NoError(t, db.Model(&models.JobQueue{}).Where("job_type = ? AND status = ?", JobTypeProductSync, models.JobStatusPending).Count(&partial).Error)
	require.EqualValues(t, 500, partial)
	var finished models.JobQueue
	require.NoError(t, db.First(&finished, "id = ?", job.ID).Error)
	require.Equal(t, models.JobStatusSucceeded, finished.Status)
}

func TestRebuildCapturedFanoutFencesPreEditEqualVersionSync(t *testing.T) {
	db := searchTestDB(t)
	ctx := context.Background()
	runtime := jobs.NewRuntime(db, jobs.Config{})
	service := NewService(db, nil, runtime)
	brand := models.Brand{Name: "Old shared name", Slug: "shared", IsActive: true}
	require.NoError(t, db.Create(&brand).Error)
	product := models.Product{SKU: "SHARED-FENCE", Name: "Shared fence", BrandID: &brand.ID, IsPublished: true}
	require.NoError(t, db.Create(&product).Error)
	require.NoError(t, service.Reindex(ctx))
	var stale models.SearchDocument
	require.NoError(t, db.First(&stale, "entity_id = ?", product.ID).Error)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&brand).Update("name", "New shared name").Error; err != nil {
			return err
		}
		return EnqueueCatalogFanoutTx(ctx, runtime, tx, "brand", []uint{brand.ID})
	}))
	require.NoError(t, service.Reindex(ctx))
	var current models.SearchDocument
	require.NoError(t, db.First(&current, "entity_id = ?", product.ID).Error)
	require.Greater(t, current.Version, stale.Version)
	require.Contains(t, current.PayloadJSON, "New shared name")
	require.NoError(t, NewDatabaseBackend(db).Upsert(ctx, stale))
	require.NoError(t, db.First(&current, "entity_id = ?", product.ID).Error)
	require.Contains(t, current.PayloadJSON, "New shared name")
	require.NoError(t, db.Model(&models.JobQueue{}).Where("job_type = ?", JobTypeCatalogFanout).Update("status", models.JobStatusDeadLetter).Error)
	health, err := service.Freshness(ctx)
	require.NoError(t, err)
	require.Equal(t, "healthy", health.Status)
}
