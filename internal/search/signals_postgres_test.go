package search

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"ecommerce/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestBusinessSignalsAndIncidentSerializationPostgres(t *testing.T) {
	dsn := os.Getenv("SEARCH_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set SEARCH_TEST_POSTGRES_DSN for PostgreSQL signal integration")
	}
	bootstrap, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	schema := fmt.Sprintf("search_test_%d", time.Now().UnixNano())
	require.NoError(t, bootstrap.Exec(fmt.Sprintf(`CREATE SCHEMA "%s"`, schema)).Error)
	t.Cleanup(func() { require.NoError(t, bootstrap.Exec(fmt.Sprintf(`DROP SCHEMA "%s" CASCADE`, schema)).Error) })
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	params := parsed.Query()
	params.Set("search_path", schema)
	parsed.RawQuery = params.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
	require.NoError(t, db.AutoMigrate(&models.SearchIndexState{}, &models.SearchQueryEvent{}, &models.SearchClickEvent{}, &models.SearchRevokedSession{}, &models.SearchConversionSignal{}, &models.SearchOrderAttribution{}, &models.SearchCartAttribution{}, &models.SearchIndexIncident{}, &models.SearchFreshnessObservation{}, &models.JobQueue{}))
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX idx_search_open ON search_index_incidents(index_name) WHERE recovered_at IS NULL").Error)
	require.NoError(t, db.Exec("CREATE TABLE orders (id INTEGER PRIMARY KEY,status TEXT,created_at TIMESTAMPTZ,deleted_at TIMESTAMPTZ)").Error)
	now := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	service := NewService(db, nil, nil)
	service.now = func() time.Time { return now }
	token := strings.Repeat("a", 64)
	owner, err := sessionHash(token)
	require.NoError(t, err)
	impression := addExposure(t, db, "mature", owner, now.Add(-10*24*time.Hour), []uint{1, 1, 2})
	addConversion(t, db, impression, 1, 1, models.StatusPaid, impression.CreatedAt.Add(time.Hour), impression.CreatedAt)
	addExposure(t, db, "immature", owner, now.Add(-7*24*time.Hour), []uint{1})
	parallel := func(action func() error) {
		var wg sync.WaitGroup
		results := make(chan error, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); results <- action() }()
		}
		wg.Wait()
		close(results)
		for err := range results {
			require.NoError(t, err)
		}
	}
	parallel(func() error { return service.RefreshConversionSignals(context.Background(), now) })
	var signal models.SearchConversionSignal
	require.NoError(t, db.First(&signal, "product_id = 1").Error)
	require.Equal(t, int64(1), signal.Impressions30Days)
	require.Equal(t, int64(1), signal.Conversions30Days)
	require.InDelta(t, 1.0/21, conversionRate(signal), 1e-12)
	// Corrupt unrelated analytics must not prevent consent withdrawal. PostgreSQL
	// requires rolling back the failed derived attempt to its savepoint.
	addExposure(t, db, "corrupt", "other", now.Add(-10*24*time.Hour), []uint{2})
	require.NoError(t, db.Model(&models.SearchQueryEvent{}).Where("impression_id = ?", "corrupt").Update("products_json", "invalid").Error)
	require.NoError(t, service.RevokeConsent(context.Background(), token))
	require.NoError(t, db.First(&signal, "product_id = 1").Error)
	require.Zero(t, signal.Impressions30Days)
	require.Zero(t, signal.Conversions30Days)
	var revoked int64
	require.NoError(t, db.Model(&models.SearchRevokedSession{}).Where("session_hash = ?", owner).Count(&revoked).Error)
	require.Equal(t, int64(1), revoked)

	require.NoError(t, db.Model(&models.SearchIndexState{}).Where("name = ?", ProductIndexName).Update("last_full_reindex_at", now).Error)
	job := models.JobQueue{ID: "f7300520-a809-4138-b67a-bc9552b46637", CreatedAt: now.Add(-10 * time.Minute), UpdatedAt: now, JobType: JobTypeProductSync, PayloadJSON: "{}", Status: models.JobStatusPending, RunAt: now.Add(time.Hour)}
	require.NoError(t, db.Create(&job).Error)
	parallel(func() error { return service.ObserveFreshness(context.Background()) })
	var count int64
	require.NoError(t, db.Model(&models.SearchIndexIncident{}).Count(&count).Error)
	require.Equal(t, int64(1), count)
	require.NoError(t, db.Delete(&job).Error)
	require.NoError(t, service.ObserveFreshness(context.Background()))
	var incident models.SearchIndexIncident
	require.NoError(t, db.First(&incident).Error)
	require.NotNil(t, incident.RecoveredAt)
	require.True(t, incident.RecoveredAt.Equal(now))
}
