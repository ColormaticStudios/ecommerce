package inventory

import (
	"fmt"
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
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = bootstrap.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schema)).Error })
	require.NoError(t, db.AutoMigrate(
		&models.Product{}, &models.ProductVariant{}, &models.InventoryItem{}, &models.InventoryLevel{},
		&models.InventoryMovement{}, &models.InventoryReservation{}, &models.InventoryThreshold{}, &models.InventoryAlert{},
	))
	return db
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
