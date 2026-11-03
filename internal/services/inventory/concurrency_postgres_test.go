package inventory

import (
	"context"
	"ecommerce/internal/jobs"
	"fmt"
	"gorm.io/gorm/clause"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newInventoryPostgresTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	baseDSN := os.Getenv("MIGRATIONS_TEST_POSTGRES_DSN")
	if baseDSN == "" {
		t.Skip("set MIGRATIONS_TEST_POSTGRES_DSN to run Postgres inventory concurrency tests")
	}
	bootstrap, err := gorm.Open(postgres.Open(baseDSN), &gorm.Config{})
	require.NoError(t, err)
	schema := fmt.Sprintf("inventory_concurrency_%d", time.Now().UTC().UnixNano())
	require.NoError(t, bootstrap.Exec(fmt.Sprintf(`CREATE SCHEMA "%s"`, schema)).Error)
	parsed, err := url.Parse(baseDSN)
	require.NoError(t, err)
	require.NotEmpty(t, parsed.Scheme, "MIGRATIONS_TEST_POSTGRES_DSN must be URL-formatted")
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bootstrap.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema)).Error })
	require.NoError(t, db.AutoMigrate(
		&models.Product{}, &models.ProductVariant{}, &models.InventoryItem{}, &models.JobQueue{}, &models.SearchCatalogFanoutDebt{}, &models.SearchInventoryRevision{}, &models.SearchProductRevision{}, &models.SearchDocument{}, &models.InventoryLevel{},
		&models.InventoryMovement{}, &models.InventoryReservation{}, &models.InventoryThreshold{}, &models.InventoryAlert{},
	))
	return jobs.WithRuntime(db, jobs.NewRuntime(db, jobs.Config{}))
}

func TestConcurrentReservationsCannotOversellPostgres(t *testing.T) {
	db := newInventoryPostgresTestDB(t)
	product := models.Product{SKU: "concurrent-product", Name: "Concurrent product", Price: models.MoneyFromFloat(10), Stock: 1, IsPublished: true}
	require.NoError(t, db.Create(&product).Error)
	variant := models.ProductVariant{ProductID: product.ID, SKU: "concurrent-variant", Title: "Default", Price: models.MoneyFromFloat(10), Stock: 1, Position: 1, IsPublished: true}
	require.NoError(t, db.Create(&variant).Error)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, key := range []string{"checkout-a", "checkout-b"} {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			<-start
			_, _, err := Reserve(db, ReservationInput{ProductVariantID: variant.ID, Quantity: 1, OwnerType: ReferenceTypeOrder, IdempotencyKey: key})
			errs <- err
		}(key)
	}
	close(start)
	wg.Wait()
	close(errs)

	var successes, unavailable int
	for err := range errs {
		if err == nil {
			successes++
			continue
		}
		var availabilityErr *InsufficientAvailabilityError
		if assert.ErrorAs(t, err, &availabilityErr) {
			unavailable++
		}
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, 1, unavailable)
	availability, err := GetAvailability(db, variant.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, availability.Available)
	assert.Equal(t, 1, availability.Reserved)
	var count int64
	require.NoError(t, db.Model(&models.InventoryReservation{}).Where("status = ?", models.InventoryReservationStatusActive).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestDifferentVariantStockChangesDoNotLockProductRevisionPostgres(t *testing.T) {
	db := newInventoryPostgresTestDB(t)
	product := models.Product{SKU: "independent-stock", Name: "Independent stock", IsPublished: true}
	require.NoError(t, db.Create(&product).Error)
	variants := []models.ProductVariant{{ProductID: product.ID, SKU: "independent-a", Title: "A", Stock: 5, IsPublished: true}, {ProductID: product.ID, SKU: "independent-b", Title: "B", Stock: 5, IsPublished: true}}
	require.NoError(t, db.Create(&variants).Error)
	require.NoError(t, db.Create(&models.SearchProductRevision{ProductID: product.ID, Revision: 1, ChangedAt: time.Now()}).Error)
	for _, variant := range variants {
		_, err := GetAvailability(db, variant.ID)
		require.NoError(t, err)
	}
	locked := db.Begin()
	require.NoError(t, locked.Error)
	defer locked.Rollback()
	var revision models.SearchProductRevision
	require.NoError(t, locked.Clauses(clause.Locking{Strength: "UPDATE"}).First(&revision, "product_id = ?", product.ID).Error)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 2)
	for _, variant := range variants {
		go func(id uint) {
			_, err := ApplyMovement(db.WithContext(ctx), MovementInput{ProductVariantID: id, MovementType: MovementTypeOrderCommit, QuantityDelta: -1})
			done <- err
		}(variant.ID)
	}
	for range variants {
		select {
		case err := <-done:
			require.NoError(t, err, "a held product revision must not block unrelated variant checkout")
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	require.NoError(t, locked.Rollback().Error)
	var queue []models.JobQueue
	require.NoError(t, db.Where("job_type = ? AND status = ?", "search.product_sync", models.JobStatusPending).Find(&queue).Error)
	require.Len(t, queue, 2, "outstanding inventory deliveries are bounded by variants")
	var debt []models.SearchInventoryRevision
	require.NoError(t, db.Find(&debt).Error)
	require.Len(t, debt, 2)
	require.NoError(t, db.First(&revision, "product_id = ?", product.ID).Error)
	require.EqualValues(t, 1, revision.Revision)
}
