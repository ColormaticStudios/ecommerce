package search_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"ecommerce/internal/jobs"
	"ecommerce/internal/search"
	"ecommerce/internal/searchprojection"
	"ecommerce/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func clockFixture(t *testing.T, dialect string) projectionPostgresFixture {
	t.Helper()
	if dialect == "postgres" {
		return newProjectionPostgresFixture(t)
	}
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Product{}, &models.ProductVariant{}, &models.Brand{}, &models.Category{}, &models.ProductCategory{}, &models.ProductAttribute{}, &models.ProductAttributeValue{},
		&models.SearchDocument{}, &models.SearchIndexState{}, &models.SearchCatalogFanoutDebt{}, &models.SearchInventoryRevision{}, &models.SearchProductRevision{}, &searchprojection.Rank{}, &searchprojection.Category{}, &searchprojection.Attribute{}, &searchprojection.Term{},
		&models.SearchSynonymSet{}, &models.SearchTypoToleranceProfile{}, &models.SearchRankingProfile{}, &models.SearchConfigurationAudit{}, &models.SearchMerchandisingRule{}, &models.SearchMerchandisingAudit{}, &models.SearchSalesSignal{}, &models.SearchConversionSignal{},
		&models.JobQueue{}, &models.JobAttempt{}, &models.JobDeadLetter{},
	))
	f := projectionPostgresFixture{db: db, ctx: context.Background(), runtime: jobs.NewRuntime(db, jobs.Config{MaxAttempts: 1})}
	f.search = search.NewService(db, nil, f.runtime)
	require.NoError(t, f.search.RegisterJobHandlers())
	f.product = models.Product{SKU: "CLOCK", Name: "Clock product", IsPublished: true}
	require.NoError(t, db.Create(&f.product).Error)
	require.NoError(t, f.search.Reindex(f.ctx))
	return f
}

func clockChange(t *testing.T, f projectionPostgresFixture, id uint, name string, changed time.Time) models.JobQueue {
	t.Helper()
	var job models.JobQueue
	require.NoError(t, f.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Product{}).Where("id = ?", id).UpdateColumn("name", name).Error; err != nil {
			return err
		}
		var err error
		job, err = search.RecordProductChangeTx(f.ctx, f.runtime, tx, id, changed)
		return err
	}))
	return job
}

func clockDocument(t *testing.T, f projectionPostgresFixture, id uint) models.SearchDocument {
	t.Helper()
	var doc models.SearchDocument
	require.NoError(t, f.db.First(&doc, "entity_type = ? AND entity_id = ?", search.ProductEntityType, id).Error)
	return doc
}

func TestProjectionFutureSourceClockDoesNotHideFailedLaterChange(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			f := clockFixture(t, dialect)
			future := time.Now().UTC().Add(24 * time.Hour)
			require.NoError(t, f.db.Model(&models.Product{}).Where("id = ?", f.product.ID).UpdateColumn("updated_at", future).Error)
			require.NoError(t, f.search.Reindex(f.ctx))
			require.Zero(t, clockDocument(t, f, f.product.ID).Version, "source timestamps cannot become convergence boundaries")
			changed := time.Now().UTC().Add(-10 * time.Minute)
			first := clockChange(t, f, f.product.ID, "First future-clock change", changed)
			require.NoError(t, f.runtime.Execute(f.ctx, first.ID, "clock-first"))
			firstDocument := clockDocument(t, f, f.product.ID)
			firstState := projectionPostgresState(t, f)
			require.Equal(t, firstState.Revision, firstDocument.Version)
			require.Equal(t, firstState.Revision, firstState.IndexedRevision)
			second := clockChange(t, f, f.product.ID, "Second future-clock change", changed)
			require.NoError(t, f.db.Exec("ALTER TABLE search_product_ranks RENAME TO search_product_ranks_paused").Error)
			require.Error(t, f.runtime.Execute(f.ctx, second.ID, "clock-second"))
			require.NoError(t, f.db.Exec("ALTER TABLE search_product_ranks_paused RENAME TO search_product_ranks").Error)
			health, err := f.search.Freshness(f.ctx)
			require.NoError(t, err)
			require.Equal(t, "stale", health.Status)
			require.Zero(t, health.PendingJobs)
			require.GreaterOrEqual(t, health.Lag, 10*time.Minute)
			require.NoError(t, search.NewDatabaseBackend(f.db).Upsert(f.ctx, firstDocument))
			require.NotNil(t, projectionPostgresState(t, f).PendingSince, "replayed older boundary cannot discharge newer debt")
			require.NoError(t, f.search.SyncProduct(f.ctx, f.product.ID))
			state := projectionPostgresState(t, f)
			require.Nil(t, state.PendingSince)
			require.Equal(t, state.Revision, state.IndexedRevision)
			current := clockDocument(t, f, f.product.ID)
			require.Greater(t, current.Version, firstDocument.Version)
			require.NoError(t, search.NewDatabaseBackend(f.db).Upsert(f.ctx, firstDocument))
			require.Equal(t, current.PayloadJSON, clockDocument(t, f, f.product.ID).PayloadJSON, "delayed source snapshot cannot overwrite newer projection")
		})
	}
}

func TestProjectionLegacyClockFloorAndAcknowledgmentRepair(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			f := clockFixture(t, dialect)
			changed := time.Now().UTC().Add(-10 * time.Minute)
			clockChange(t, f, f.product.ID, "Legacy source", changed)
			future := time.Now().Add(time.Hour).UnixNano()
			require.NoError(t, f.db.Model(&models.SearchDocument{}).Where("entity_id = ?", f.product.ID).UpdateColumn("version", future).Error)
			require.NoError(t, f.db.Model(&models.SearchProductRevision{}).Where("product_id = ?", f.product.ID).UpdateColumn("indexed_revision", future+5).Error)
			health, err := f.search.Freshness(f.ctx)
			require.NoError(t, err)
			require.Equal(t, "stale", health.Status, "pending debt remains visible even after a timestamp-era acknowledgment advanced too far")
			require.NoError(t, f.search.SyncProduct(f.ctx, f.product.ID))
			repaired := projectionPostgresState(t, f)
			require.Equal(t, future+6, repaired.Revision)
			require.Equal(t, repaired.Revision, repaired.IndexedRevision)
			require.Nil(t, repaired.PendingSince)
			job := clockChange(t, f, f.product.ID, "After repair", changed.Add(-time.Hour))
			var payload search.ProductSyncPayload
			require.NoError(t, json.Unmarshal([]byte(job.PayloadJSON), &payload))
			require.Equal(t, repaired.Revision+1, projectionPostgresState(t, f).Revision)
			require.NoError(t, f.search.SyncProduct(f.ctx, f.product.ID))
			require.Equal(t, projectionPostgresState(t, f).Revision, clockDocument(t, f, f.product.ID).Version)
			// An unrelated direct backend write must not acknowledge beyond the source revision.
			doc := clockDocument(t, f, f.product.ID)
			doc.Version++
			require.NoError(t, search.NewDatabaseBackend(f.db).Upsert(f.ctx, doc))
			require.Equal(t, repaired.Revision+1, projectionPostgresState(t, f).IndexedRevision)
			// Timestamp-era corruption can also leave the acknowledgment equal
			// to the source boundary while retaining explicit outstanding debt.
			clockChange(t, f, f.product.ID, "Equal acknowledgment debt", changed)
			equal := projectionPostgresState(t, f)
			require.NoError(t, f.db.Model(&models.SearchProductRevision{}).Where("product_id = ?", f.product.ID).UpdateColumn("indexed_revision", equal.Revision).Error)
			require.NoError(t, f.search.SyncProduct(f.ctx, f.product.ID))
			recovered := projectionPostgresState(t, f)
			require.Equal(t, equal.Revision+1, recovered.Revision)
			require.Equal(t, recovered.Revision, recovered.IndexedRevision)
			require.Nil(t, recovered.PendingSince)
		})
	}
}

type clockReindexBackend struct {
	search.SearchBackend
	before func()
}

func (b clockReindexBackend) ReplaceAll(ctx context.Context, documents []models.SearchDocument, at time.Time) error {
	b.before()
	return b.SearchBackend.ReplaceAll(ctx, documents, at)
}

func TestProjectionReindexCannotAcknowledgeOrDeactivatePostSnapshotChanges(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			f := clockFixture(t, dialect)
			changed := time.Now().UTC().Add(-10 * time.Minute)
			clockChange(t, f, f.product.ID, "Captured source", changed)
			require.NoError(t, f.search.SyncProduct(f.ctx, f.product.ID))
			var latePublished, lateAbsent models.Product
			var newer models.SearchDocument
			backend := clockReindexBackend{SearchBackend: search.NewDatabaseBackend(f.db), before: func() {
				clockChange(t, f, f.product.ID, "Newer source", changed.Add(-time.Hour))
				require.NoError(t, f.search.SyncProduct(f.ctx, f.product.ID))
				newer = clockDocument(t, f, f.product.ID)
				latePublished = models.Product{SKU: "LATE-PUBLIC", Name: "Late public", IsPublished: true}
				lateAbsent = models.Product{SKU: "LATE-ABSENT", Name: "Late absent", IsPublished: false}
				require.NoError(t, f.db.Create(&latePublished).Error)
				require.NoError(t, f.db.Table("products").Create(map[string]any{"sku": lateAbsent.SKU, "name": lateAbsent.Name, "is_published": false, "price": 0, "created_at": changed, "updated_at": changed}).Error)
				require.NoError(t, f.db.First(&lateAbsent, "sku = ?", lateAbsent.SKU).Error)
				clockChange(t, f, latePublished.ID, "Late public", changed)
				clockChange(t, f, lateAbsent.ID, "Late absent", changed)
				require.NoError(t, f.search.SyncProduct(f.ctx, latePublished.ID))
			}}
			require.NoError(t, search.NewService(f.db, backend, f.runtime).Reindex(f.ctx))
			require.Equal(t, newer.PayloadJSON, clockDocument(t, f, f.product.ID).PayloadJSON)
			require.True(t, clockDocument(t, f, latePublished.ID).Active, "uncaptured fresh document survives replacement")
			var absent models.SearchProductRevision
			require.NoError(t, f.db.First(&absent, "product_id = ?", lateAbsent.ID).Error)
			require.NotNil(t, absent.PendingSince, "absence from the captured snapshot cannot acknowledge a later mutation")
			require.Zero(t, absent.IndexedRevision)
		})
	}
}

func TestProjectionFutureClockDeletionAndOrphanReindex(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			f := clockFixture(t, dialect)
			changed := time.Now().UTC().Add(-10 * time.Minute)
			require.NoError(t, f.db.Model(&models.Product{}).Where("id = ?", f.product.ID).UpdateColumn("updated_at", time.Now().Add(time.Hour)).Error)
			clockChange(t, f, f.product.ID, "Before deletion", changed)
			require.NoError(t, f.search.SyncProduct(f.ctx, f.product.ID))
			old := clockDocument(t, f, f.product.ID)
			require.NoError(t, f.db.Model(&models.Product{}).Where("id = ?", f.product.ID).UpdateColumn("is_published", false).Error)
			clockChange(t, f, f.product.ID, "Unpublished", changed)
			require.NoError(t, f.search.SyncProduct(f.ctx, f.product.ID))
			tombstone := clockDocument(t, f, f.product.ID)
			require.False(t, tombstone.Active)
			require.Equal(t, projectionPostgresState(t, f).Revision, tombstone.Version)
			require.Greater(t, tombstone.Version, old.Version)
			require.NoError(t, search.NewDatabaseBackend(f.db).Upsert(f.ctx, old))
			require.False(t, clockDocument(t, f, f.product.ID).Active)
			orphanID := f.product.ID + 1000
			future := time.Now().Add(2 * time.Hour).UnixNano()
			orphan := models.SearchDocument{EntityType: search.ProductEntityType, EntityID: orphanID, PayloadJSON: fmt.Sprintf(`{"ID":%d,"name":"Legacy orphan"}`, orphanID), Version: future, Active: true}
			require.NoError(t, search.NewDatabaseBackend(f.db).Upsert(f.ctx, orphan))
			require.NoError(t, f.search.Reindex(f.ctx))
			require.False(t, clockDocument(t, f, orphanID).Active, "future legacy projection with no revision row must still be reconciled")
			require.Nil(t, projectionPostgresState(t, f).PendingSince)
			// The first source mutation for a legacy document with no revision
			// row must allocate above that document's future boundary too.
			job, err := search.RecordProductChangeTx(f.ctx, f.runtime, f.db, orphanID, changed)
			require.NoError(t, err)
			var payload search.ProductSyncPayload
			require.NoError(t, json.Unmarshal([]byte(job.PayloadJSON), &payload))
			var orphanRevision models.SearchProductRevision
			require.NoError(t, f.db.First(&orphanRevision, "product_id = ?", orphanID).Error)
			require.Equal(t, future+1, orphanRevision.Revision)
			require.NoError(t, f.search.SyncProduct(f.ctx, orphanID))
			require.Equal(t, orphanRevision.Revision, clockDocument(t, f, orphanID).Version)
		})
	}
}
