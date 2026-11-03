package search

import (
	"context"
	"ecommerce/models"
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
)

func TestAnalyticsProofAndReportingPostgres(t *testing.T) {
	dsn := os.Getenv("SEARCH_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set SEARCH_TEST_POSTGRES_DSN for PostgreSQL analytics integration")
	}
	bootstrap, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	schema := fmt.Sprintf("search_analytics_test_%d", time.Now().UnixNano())
	require.NoError(t, bootstrap.Exec(fmt.Sprintf(`CREATE SCHEMA "%s"`, schema)).Error)
	t.Cleanup(func() {
		require.NoError(t, bootstrap.Exec(fmt.Sprintf(`DROP SCHEMA "%s" CASCADE`, schema)).Error)
		sqlDB, _ := bootstrap.DB()
		_ = sqlDB.Close()
	})
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
	require.NoError(t, db.AutoMigrate(&models.SearchQueryEvent{}, &models.SearchClickEvent{}, &models.SearchCartAttribution{}, &models.SearchOrderAttribution{}, &models.SearchRevokedSession{}))
	s := NewService(db, nil, nil)
	require.NoError(t, s.ConfigureAnalyticsSigningKey([]byte("test-key")))
	ctx := context.Background()
	token, err := s.SignResultSnapshot(Result{Products: []models.Product{{BaseModel: models.BaseModel{ID: 1}}}, Total: 1}, Filters{Query: "boots", Page: 1, Limit: 10}, 12*time.Millisecond)
	require.NoError(t, err)
	first, err := s.RecordImpression(ctx, true, "", uuid.NewString(), token)
	require.NoError(t, err)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			im, err := s.RecordImpression(ctx, true, first.SessionToken, uuid.NewString(), token)
			if err == nil && im.ID != first.ID {
				err = fmt.Errorf("snapshot duplicate returned another impression")
			}
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	for i := 0; i < 20; i++ {
		proof, err := s.SignResultSnapshot(Result{Products: []models.Product{{BaseModel: models.BaseModel{ID: 1}}}, Total: 1}, Filters{Query: fmt.Sprintf("query-%02d", i), Page: 1, Limit: 10}, time.Duration(i+1)*time.Millisecond)
		require.NoError(t, err)
		_, err = s.RecordImpression(ctx, true, "", uuid.NewString(), proof)
		require.NoError(t, err)
	}
	queries := 0
	require.NoError(t, db.Callback().Row().Before("gorm:row").Register("analytics_pg_count", func(*gorm.DB) { queries++ }))
	m, err := s.AnalyticsPage(ctx, 7, 2, 5)
	require.NoError(t, err)
	require.Equal(t, int64(21), m.Searches)
	require.Equal(t, 21, m.TotalQueries)
	require.Len(t, m.Queries, 5)
	require.Equal(t, float64(19), m.P95LatencyMs)
	require.Equal(t, float64(20), m.P99LatencyMs)
	require.Equal(t, 6, queries)
}
