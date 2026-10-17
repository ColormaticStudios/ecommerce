package search_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"ecommerce/internal/apicontract"
	"ecommerce/internal/jobs"
	"ecommerce/internal/search"
	"ecommerce/internal/searchprojection"
	"ecommerce/internal/services/catalogadmin"
	"ecommerce/internal/services/inventory"
	"ecommerce/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type projectionPostgresFixture struct {
	db        *gorm.DB
	ctx       context.Context
	runtime   *jobs.Runtime
	search    *search.Service
	catalog   *catalogadmin.Service
	product   models.Product
	variant   models.ProductVariant
	brand     models.Brand
	category  models.Category
	attribute models.ProductAttribute
}

func newProjectionPostgresFixture(t *testing.T) projectionPostgresFixture {
	t.Helper()
	dsn := os.Getenv("SEARCH_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set SEARCH_TEST_POSTGRES_DSN for PostgreSQL projection convergence integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	bootstrap, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	bootstrapSQL, err := bootstrap.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, bootstrapSQL.Close()) })
	schema := "search_projection_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, bootstrap.WithContext(ctx).Exec(`CREATE SCHEMA "`+schema+`"`).Error)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		require.NoError(t, bootstrap.WithContext(cleanupCtx).Exec(`DROP SCHEMA "`+schema+`" CASCADE`).Error)
	})
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	params := parsed.Query()
	params.Set("search_path", schema)
	parsed.RawQuery = params.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), DisableForeignKeyConstraintWhenMigrating: true})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.WithContext(ctx).AutoMigrate(
		&models.Product{}, &models.ProductVariant{}, &models.Brand{}, &models.Category{}, &models.ProductCategory{}, &models.ProductAttribute{}, &models.ProductAttributeValue{},
		&models.Locale{}, &models.LocalizedEntityValue{}, &models.MediaReference{},
		&models.SearchDocument{}, &models.SearchIndexState{}, &models.SearchCatalogFanoutDebt{}, &models.SearchInventoryRevision{}, &models.SearchProductRevision{}, &searchprojection.Rank{}, &searchprojection.Category{}, &searchprojection.Attribute{}, &searchprojection.Term{},
		&models.SearchSynonymSet{}, &models.SearchTypoToleranceProfile{}, &models.SearchRankingProfile{}, &models.SearchConfigurationAudit{}, &models.SearchMerchandisingRule{}, &models.SearchMerchandisingAudit{}, &models.SearchSalesSignal{}, &models.SearchConversionSignal{},
		&models.JobQueue{}, &models.JobAttempt{}, &models.JobDeadLetter{},
		&models.InventoryItem{}, &models.InventoryLevel{}, &models.InventoryMovement{}, &models.InventoryThreshold{}, &models.InventoryAlert{},
	))
	runtime := jobs.NewRuntime(db, jobs.Config{MaxAttempts: 1})
	db = jobs.WithRuntime(db, runtime)
	f := projectionPostgresFixture{db: db, ctx: ctx, runtime: runtime}
	f.search = search.NewService(db, nil, f.runtime)
	f.catalog = catalogadmin.NewService(db, nil, f.runtime)
	require.NoError(t, f.search.RegisterJobHandlers())
	_, err = f.search.CreateTypoToleranceProfile(ctx, search.TypoToleranceProfileInput{Name: "default", MinimumTokenLength: 4, OneEditMinimumLength: 4, TwoEditMinimumLength: 8, IsActive: true})
	require.NoError(t, err)
	_, err = f.search.CreateRankingProfile(ctx, search.RankingProfileInput{Name: "default", Weights: search.DefaultRankingWeights(), IsDefault: true}, nil)
	require.NoError(t, err)
	require.NoError(t, db.WithContext(ctx).Create(&models.Locale{Code: "en-US", Name: "English", IsEnabled: true, IsDefault: true}).Error)
	f.brand, err = f.catalog.CreateBrand(ctx, apicontract.BrandInput{Name: "Original equipment"})
	require.NoError(t, err)
	f.category, err = f.catalog.CreateCategory(ctx, apicontract.CategoryInput{Name: "Original trails"})
	require.NoError(t, err)
	filterable := true
	f.attribute, err = f.catalog.CreateAttribute(ctx, apicontract.ProductAttributeDefinitionInput{Key: "Color", Type: "text", Filterable: &filterable}, nil)
	require.NoError(t, err)
	baseline := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	f.product = models.Product{BaseModel: models.BaseModel{CreatedAt: baseline, UpdatedAt: baseline}, SKU: "PG-BOOT", Name: "Trail boot", Description: "Original description", BrandID: &f.brand.ID, Price: models.MoneyFromFloat(80), Stock: 999, IsPublished: true}
	require.NoError(t, db.WithContext(ctx).Create(&f.product).Error)
	require.NoError(t, db.WithContext(ctx).Create(&models.ProductCategory{ProductID: f.product.ID, CategoryID: f.category.ID}).Error)
	red := "red"
	require.NoError(t, db.WithContext(ctx).Create(&models.ProductAttributeValue{ProductID: f.product.ID, ProductAttributeID: f.attribute.ID, TextValue: &red}).Error)
	f.variant = models.ProductVariant{ProductID: f.product.ID, SKU: "PG-BOOT-PUBLISHED", Title: "Red", Price: models.MoneyFromFloat(85), Stock: 4, IsPublished: true}
	require.NoError(t, db.WithContext(ctx).Create(&f.variant).Error)
	draft := models.ProductVariant{ProductID: f.product.ID, SKU: "PG-BOOT-DRAFT", Title: "Draft", Price: models.MoneyFromFloat(5), Stock: 9, IsPublished: false}
	// Capture false before Create because GORM's default:true also mutates it.
	wantPublished := draft.IsPublished
	require.NoError(t, db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&draft).Error; err != nil {
			return err
		}
		return tx.Model(&draft).Update("is_published", wantPublished).Error
	}))
	require.NoError(t, f.search.Reindex(ctx))
	return f
}

func projectionPostgresState(t *testing.T, f projectionPostgresFixture) models.SearchProductRevision {
	t.Helper()
	var state models.SearchProductRevision
	require.NoError(t, f.db.WithContext(f.ctx).First(&state, "product_id = ?", f.product.ID).Error)
	return state
}

func TestProjectionConcurrentChangesAndOlderAcknowledgmentPostgres(t *testing.T) {
	f := newProjectionPostgresFixture(t)
	old := time.Now().UTC().Add(-10 * time.Minute)
	require.NoError(t, f.db.WithContext(f.ctx).Transaction(func(tx *gorm.DB) error {
		_, err := search.RecordProductChangeTx(f.ctx, f.runtime, tx, f.product.ID, old)
		return err
	}))
	var oldDocument models.SearchDocument
	require.NoError(t, f.db.WithContext(f.ctx).First(&oldDocument, "entity_id = ?", f.product.ID).Error)
	oldDocument.Version = old.UnixNano()
	var wg sync.WaitGroup
	errors := make(chan error, 8)
	for i := 1; i <= 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errors <- f.db.WithContext(f.ctx).Transaction(func(tx *gorm.DB) error {
				if err := tx.Model(&models.Product{}).Where("id = ?", f.product.ID).UpdateColumn("description", fmt.Sprintf("Concurrent change %d", i)).Error; err != nil {
					return err
				}
				// Independent writes can share the clock's exact timestamp. Each
				// committed mutation still needs a newer convergence boundary.
				_, err := search.RecordProductChangeTx(f.ctx, f.runtime, tx, f.product.ID, old)
				return err
			})
		}(i)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	before := projectionPostgresState(t, f)
	require.Equal(t, old.UnixNano()+8, before.Revision)
	var queued []models.JobQueue
	require.NoError(t, f.db.WithContext(f.ctx).Where("job_type = ?", search.JobTypeProductSync).Find(&queued).Error)
	require.Len(t, queued, 1, "concurrent source revisions share one active job")
	require.NoError(t, search.NewDatabaseBackend(f.db).Upsert(f.ctx, oldDocument))
	after := projectionPostgresState(t, f)
	require.Equal(t, before.Revision, after.Revision)
	require.Equal(t, old.UnixNano(), after.IndexedRevision)
	require.NotNil(t, after.PendingSince)
	require.WithinDuration(t, old, *after.PendingSince, time.Microsecond)
	require.NoError(t, f.search.SyncProduct(f.ctx, f.product.ID))
	after = projectionPostgresState(t, f)
	require.Equal(t, after.Revision, after.IndexedRevision)
	require.Nil(t, after.PendingSince)
	var canonical models.Product
	require.NoError(t, f.db.WithContext(f.ctx).First(&canonical, f.product.ID).Error)
	result, err := f.search.Search(f.ctx, search.Filters{Query: "boot", Page: 1, Limit: 10})
	require.NoError(t, err)
	require.Len(t, result.Products, 1)
	require.Equal(t, canonical.Description, result.Products[0].Description)
	for _, job := range queued {
		require.NoError(t, f.runtime.Execute(f.ctx, job.ID, "pg-convergence"))
	}
	health, err := f.search.Freshness(f.ctx)
	require.NoError(t, err)
	require.Equal(t, "healthy", health.Status)
}

func TestProjectionRelatedCatalogAndInventoryChangesConvergePostgres(t *testing.T) {
	f := newProjectionPostgresFixture(t)
	active := true
	_, err := f.catalog.UpdateBrand(f.ctx, f.brand.ID, apicontract.BrandInput{Name: "Nimbus equipment", IsActive: &active})
	require.NoError(t, err)
	_, err = f.catalog.UpdateCategory(f.ctx, f.category.ID, apicontract.CategoryInput{Name: "Mountain trails", IsActive: &active})
	require.NoError(t, err)
	filterable, slug := true, "shade"
	_, err = f.catalog.UpdateAttribute(f.ctx, f.attribute.ID, apicontract.ProductAttributeDefinitionInput{Key: "Shade", Slug: &slug, Type: "text", Filterable: &filterable}, nil)
	require.NoError(t, err)
	_, err = inventory.ApplyMovement(f.db.WithContext(f.ctx), inventory.MovementInput{ProductVariantID: f.variant.ID, MovementType: inventory.MovementTypeOrderCommit, QuantityDelta: -4, ReferenceType: inventory.ReferenceTypeOrder})
	require.NoError(t, err)
	var canonical models.Product
	require.NoError(t, f.db.WithContext(f.ctx).First(&canonical, f.product.ID).Error)
	require.True(t, f.product.UpdatedAt.Equal(canonical.UpdatedAt), "related mutations must not manufacture a product edit")
	var fanouts []models.JobQueue
	require.NoError(t, f.db.Where("job_type = ?", search.JobTypeCatalogFanout).Find(&fanouts).Error)
	require.Len(t, fanouts, 3)
	for _, fanout := range fanouts {
		require.NoError(t, f.runtime.Execute(f.ctx, fanout.ID, "pg-catalog-fanout"))
	}
	var queued []models.JobQueue
	require.NoError(t, f.db.WithContext(f.ctx).Where("job_type = ?", search.JobTypeProductSync).Order("created_at,id").Find(&queued).Error)
	require.Len(t, queued, 2, "one active product and one active variant delivery")
	require.NotNil(t, projectionPostgresState(t, f).PendingSince)
	// Even the oldest delivery projects current canonical dependencies.
	require.NoError(t, f.runtime.Execute(f.ctx, queued[0].ID, "pg-related-convergence"))
	result, err := f.search.Search(f.ctx, search.Filters{Query: "nimbus", BrandSlugs: []string{"nimbus-equipment"}, CategorySlugs: []string{"mountain-trails"}, StockAvailability: []bool{false}, AttributeValues: map[string][]string{"shade": {"red"}}, Page: 1, Limit: 10})
	require.NoError(t, err)
	require.EqualValues(t, 1, result.Total)
	require.Len(t, result.Products, 1)
	product := result.Products[0]
	require.Zero(t, product.Stock, "legacy product stock and unpublished variant stock cannot supply availability")
	require.Len(t, product.Variants, 1)
	require.Zero(t, product.Variants[0].Stock)
	require.Equal(t, "Nimbus equipment", product.Brand.Name)
	require.Equal(t, "Mountain trails", product.Categories[0].Name)
	require.Equal(t, "Shade", product.AttributeValues[0].ProductAttribute.Key)
	result, err = f.search.Search(f.ctx, search.Filters{Query: "nimbus", StockAvailability: []bool{true}, Page: 1, Limit: 10})
	require.NoError(t, err)
	require.Zero(t, result.Total)
	for _, job := range queued[1:] {
		require.NoError(t, f.runtime.Execute(f.ctx, job.ID, "pg-related-convergence"))
	}
	state := projectionPostgresState(t, f)
	require.Equal(t, state.Revision, state.IndexedRevision)
	require.Nil(t, state.PendingSince)
	_, err = inventory.ApplyMovement(f.db.WithContext(f.ctx), inventory.MovementInput{ProductVariantID: f.variant.ID, MovementType: "RESTOCK", QuantityDelta: 2})
	require.NoError(t, err)
	var restock models.JobQueue
	require.NoError(t, f.db.WithContext(f.ctx).Where("job_type = ? AND status = ?", search.JobTypeProductSync, models.JobStatusPending).First(&restock).Error)
	require.NoError(t, f.runtime.Execute(f.ctx, restock.ID, "pg-restock-convergence"))
	result, err = f.search.Search(f.ctx, search.Filters{Query: "nimbus", StockAvailability: []bool{true}, Page: 1, Limit: 10})
	require.NoError(t, err)
	require.Len(t, result.Products, 1)
	require.Equal(t, 2, result.Products[0].Stock)
}

func TestProjectionDeadLetterDebtAndRecoveryPreserveHistoryPostgres(t *testing.T) {
	f := newProjectionPostgresFixture(t)
	var job models.JobQueue
	changed := time.Now().UTC().Add(-10 * time.Minute)
	require.NoError(t, f.db.WithContext(f.ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Product{}).Where("id = ?", f.product.ID).UpdateColumn("name", "Recovered trail boot").Error; err != nil {
			return err
		}
		var err error
		job, err = search.RecordProductChangeTx(f.ctx, f.runtime, tx, f.product.ID, changed)
		return err
	}))
	require.NoError(t, f.db.WithContext(f.ctx).Exec("ALTER TABLE search_product_ranks RENAME TO search_product_ranks_paused").Error)
	require.Error(t, f.runtime.Execute(f.ctx, job.ID, "pg-failing-projection"))
	require.NoError(t, f.db.WithContext(f.ctx).Exec("ALTER TABLE search_product_ranks_paused RENAME TO search_product_ranks").Error)
	health, err := f.search.Freshness(f.ctx)
	require.NoError(t, err)
	require.Equal(t, "stale", health.Status)
	require.Zero(t, health.PendingJobs)
	require.GreaterOrEqual(t, health.Lag, 10*time.Minute)
	var failed models.JobQueue
	var attempt models.JobAttempt
	var deadLetter models.JobDeadLetter
	require.NoError(t, f.db.WithContext(f.ctx).First(&failed, "id = ?", job.ID).Error)
	require.Equal(t, models.JobStatusDeadLetter, failed.Status)
	require.NoError(t, f.db.WithContext(f.ctx).First(&attempt, "job_id = ?", job.ID).Error)
	require.NoError(t, f.db.WithContext(f.ctx).First(&deadLetter, "job_id = ?", job.ID).Error)
	require.NotNil(t, projectionPostgresState(t, f).PendingSince)
	require.NoError(t, f.search.SyncProduct(f.ctx, f.product.ID))
	health, err = f.search.Freshness(f.ctx)
	require.NoError(t, err)
	require.Equal(t, "healthy", health.Status)
	var persistedJob models.JobQueue
	var persistedAttempt models.JobAttempt
	var persistedDeadLetter models.JobDeadLetter
	require.NoError(t, f.db.WithContext(f.ctx).First(&persistedJob, "id = ?", job.ID).Error)
	require.NoError(t, f.db.WithContext(f.ctx).First(&persistedAttempt, "id = ?", attempt.ID).Error)
	require.NoError(t, f.db.WithContext(f.ctx).First(&persistedDeadLetter, "id = ?", deadLetter.ID).Error)
	require.Equal(t, failed, persistedJob)
	require.Equal(t, attempt, persistedAttempt)
	require.Equal(t, deadLetter, persistedDeadLetter)
	result, err := f.search.Search(f.ctx, search.Filters{Query: "recovered", Page: 1, Limit: 10})
	require.NoError(t, err)
	require.Len(t, result.Products, 1)
	require.Equal(t, "Recovered trail boot", result.Products[0].Name)
}

func TestProjectionSourceTransactionsRollbackWithoutDeliveryPostgres(t *testing.T) {
	f := newProjectionPostgresFixture(t)
	active := true
	_, err := catalogadmin.NewService(f.db, nil, nil).UpdateBrand(f.ctx, f.brand.ID, apicontract.BrandInput{Name: "Must roll back", IsActive: &active})
	require.ErrorContains(t, err, "runtime is required")
	var brand models.Brand
	require.NoError(t, f.db.WithContext(f.ctx).First(&brand, f.brand.ID).Error)
	require.Equal(t, f.brand.Name, brand.Name)
	var localization models.LocalizedEntityValue
	require.NoError(t, f.db.WithContext(f.ctx).Where("entity_type = ? AND entity_id = ? AND field = ?", "brand", f.brand.ID, "name").First(&localization).Error)
	require.Equal(t, f.brand.Name, localization.Value)
	_, err = inventory.GetAvailability(f.db.WithContext(f.ctx), f.variant.ID)
	require.NoError(t, err)
	require.NoError(t, f.db.WithContext(f.ctx).Exec("ALTER TABLE job_queue RENAME TO job_queue_paused").Error)
	_, err = f.catalog.UpdateCategory(f.ctx, f.category.ID, apicontract.CategoryInput{Name: "Must roll back", IsActive: &active})
	require.Error(t, err)
	_, err = inventory.ApplyMovement(f.db.WithContext(f.ctx), inventory.MovementInput{ProductVariantID: f.variant.ID, MovementType: inventory.MovementTypeOrderCommit, QuantityDelta: -1})
	require.Error(t, err)
	require.NoError(t, f.db.WithContext(f.ctx).Exec("ALTER TABLE job_queue_paused RENAME TO job_queue").Error)
	var category models.Category
	require.NoError(t, f.db.WithContext(f.ctx).First(&category, f.category.ID).Error)
	require.Equal(t, f.category.Name, category.Name)
	localization = models.LocalizedEntityValue{}
	require.NoError(t, f.db.WithContext(f.ctx).Where("entity_type = ? AND entity_id = ? AND field = ?", "category", f.category.ID, "name").First(&localization).Error)
	require.Equal(t, f.category.Name, localization.Value)
	var variant models.ProductVariant
	require.NoError(t, f.db.WithContext(f.ctx).First(&variant, f.variant.ID).Error)
	require.Equal(t, 4, variant.Stock)
	availability, err := inventory.GetAvailability(f.db.WithContext(f.ctx), f.variant.ID)
	require.NoError(t, err)
	require.Equal(t, 4, availability.Available)
	for _, model := range []any{&models.JobQueue{}, &models.SearchCatalogFanoutDebt{}, &models.SearchInventoryRevision{}, &models.SearchProductRevision{}, &models.InventoryMovement{}} {
		var count int64
		require.NoError(t, f.db.WithContext(f.ctx).Model(model).Count(&count).Error)
		require.Zero(t, count, "failed transaction must not leave source mutations, debt or delivery intent")
	}
}

func TestIncrementalDocumentCountsAreExactUnderConcurrentWritesPostgres(t *testing.T) {
	f := newProjectionPostgresFixture(t)
	backend := search.NewDatabaseBackend(f.db)
	errs := make(chan error, 24)
	var wg sync.WaitGroup
	for i := uint(1); i <= 12; i++ {
		for range 2 {
			wg.Add(1)
			go func(id uint) {
				defer wg.Done()
				errs <- backend.Upsert(f.ctx, models.SearchDocument{EntityType: search.ProductEntityType, EntityID: 1000 + id, PayloadJSON: `{"name":"Concurrent count"}`, Active: true, Version: 1})
			}(i)
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var state models.SearchIndexState
	require.NoError(t, f.db.First(&state, "name = ?", search.ProductIndexName).Error)
	require.EqualValues(t, 13, state.DocumentCount)
	for i := uint(1); i <= 12; i++ {
		if i%2 == 0 {
			require.NoError(t, backend.Delete(f.ctx, search.ProductEntityType, 1000+i, 2))
		}
		require.NoError(t, backend.Upsert(f.ctx, models.SearchDocument{EntityType: search.ProductEntityType, EntityID: 1000 + i, PayloadJSON: `{"name":"Stale count"}`, Active: true, Version: 0}))
	}
	require.NoError(t, f.db.First(&state, "name = ?", search.ProductIndexName).Error)
	require.EqualValues(t, 7, state.DocumentCount)
	var exact int64
	require.NoError(t, f.db.Model(&models.SearchDocument{}).Where("entity_type = ? AND active = ?", search.ProductEntityType, true).Count(&exact).Error)
	require.Equal(t, exact, state.DocumentCount)
}

func TestUncommittedCatalogFanoutIsNotCoveredByRebuildPostgres(t *testing.T) {
	f := newProjectionPostgresFixture(t)
	source := f.db.WithContext(f.ctx).Begin()
	require.NoError(t, source.Error)
	defer source.Rollback()
	require.NoError(t, source.Model(&models.Brand{}).Where("id = ?", f.brand.ID).Update("name", "Late committed brand").Error)
	require.NoError(t, search.EnqueueCatalogFanoutTx(f.ctx, f.runtime, source, "brand", []uint{f.brand.ID}))
	var fanout models.JobQueue
	require.NoError(t, source.Where("job_type = ?", search.JobTypeCatalogFanout).First(&fanout).Error)
	old := time.Now().UTC().Add(-10 * time.Minute)
	require.NoError(t, source.Model(&models.JobQueue{}).Where("id = ?", fanout.ID).Updates(map[string]any{"created_at": old, "status": models.JobStatusDeadLetter}).Error)
	require.NoError(t, source.Model(&models.SearchCatalogFanoutDebt{}).Where("job_id = ?", fanout.ID).Update("pending_since", old).Error)
	require.NoError(t, f.search.Reindex(f.ctx))
	require.NoError(t, source.Commit().Error)
	var debt models.SearchCatalogFanoutDebt
	require.NoError(t, f.db.First(&debt, "job_id = ?", fanout.ID).Error)
	require.Nil(t, debt.CoveredAt, "the source transaction was not committed in the rebuild's captured fanout set")
	health, err := f.search.Freshness(f.ctx)
	require.NoError(t, err)
	require.Equal(t, "stale", health.Status)
	var document models.SearchDocument
	require.NoError(t, f.db.First(&document, "entity_id = ?", f.product.ID).Error)
	require.NotContains(t, document.PayloadJSON, "Late committed brand")
	require.NoError(t, f.search.Reindex(f.ctx))
	require.NoError(t, f.db.First(&debt, "job_id = ?", fanout.ID).Error)
	require.NotNil(t, debt.CoveredAt)
	health, err = f.search.Freshness(f.ctx)
	require.NoError(t, err)
	require.Equal(t, "healthy", health.Status)
}
