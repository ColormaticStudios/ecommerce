package search

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ecommerce/internal/migrations"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPostgresSearchPlanningTransactionIsolation(t *testing.T) {
	dsn := os.Getenv("SEARCH_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set SEARCH_TEST_POSTGRES_DSN for PostgreSQL search planning integration")
	}
	bootstrap, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	bootstrapSQL, err := bootstrap.DB()
	require.NoError(t, err)
	defer bootstrapSQL.Close()
	schema := fmt.Sprintf("search_planning_test_%d", time.Now().UnixNano())
	require.NoError(t, bootstrap.Exec(fmt.Sprintf(`CREATE SCHEMA "%s"`, schema)).Error)
	defer func() { require.NoError(t, bootstrap.Exec(fmt.Sprintf(`DROP SCHEMA "%s" CASCADE`, schema)).Error) }()
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	parameters := parsed.Query()
	parameters.Set("search_path", schema)
	parsed.RawQuery = parameters.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	defer pool.Close()
	require.NoError(t, migrations.Run(db))
	indexedFixture(t, db, 1, "Red Trail Shoe", "north", "trail", "red", 10, true)
	indexedFixture(t, db, 2, "Blue Trail Shoe", "south", "trail", "blue", 20, false)
	// Migration locking reserves its own connection. Restrict the pool only
	// after setup so these assertions reuse one session without blocking setup.
	pool.SetMaxOpenConns(1)
	pool.SetMaxIdleConns(1)
	b := NewDatabaseBackend(db).(*databaseBackend)
	sharedDB := b.db
	assertDefaults := func() {
		var mode, readOnly string
		require.NoError(t, db.Raw("SHOW plan_cache_mode").Scan(&mode).Error)
		require.NoError(t, db.Raw("SHOW transaction_read_only").Scan(&readOnly).Error)
		require.Equal(t, "auto", mode, "setting must not leak to the pooled session")
		require.Equal(t, "off", readOnly)
		require.Same(t, sharedDB, b.db, "shared backend storage remains immutable")
	}
	assertDefaults()
	var cancelQuery atomic.Bool
	var cancelOnce atomic.Bool
	var checked atomic.Int64
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("assert_search_planning_transaction", func(tx *gorm.DB) {
		// Subquery construction invokes callbacks without executing a query.
		// Probing there would overwrite its dry-run SQL instead of testing a session.
		if tx.DryRun {
			return
		}
		// NewDB retains the caller's transaction without mutating the active
		// statement. Raw probes use Row callbacks, avoiding query recursion.
		probe := tx.Session(&gorm.Session{NewDB: true})
		var mode, readOnly string
		if err := probe.Raw("SHOW plan_cache_mode").Scan(&mode).Error; err != nil {
			tx.AddError(err)
			return
		}
		if err := probe.Raw("SHOW transaction_read_only").Scan(&readOnly).Error; err != nil {
			tx.AddError(err)
			return
		}
		if mode != "force_custom_plan" || readOnly != "on" {
			tx.AddError(fmt.Errorf("search transaction settings: mode=%s read_only=%s", mode, readOnly))
			return
		}
		checked.Add(1)
		if cancelQuery.Load() && cancelOnce.CompareAndSwap(false, true) {
			tx.AddError(probe.Exec("SELECT pg_sleep(5)").Error)
		}
	}))
	filters := Filters{Query: "trail", Limit: 20}
	result, err := b.Search(context.Background(), filters)
	require.NoError(t, err)
	require.Equal(t, int64(2), result.Total)
	require.Len(t, result.Products, 2)
	require.Positive(t, checked.Load())
	assertDefaults() // Successful commit.
	result, err = b.Search(context.Background(), Filters{PriceRanges: []PriceRange{{}}})
	require.ErrorContains(t, err, "requires at least one bound")
	require.False(t, errors.Is(err, ErrIndexUnavailable), "validation is not an outage")
	require.Empty(t, result.Products)
	assertDefaults() // Query error rollback.
	cancelQuery.Store(true)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	result, err = b.Search(ctx, filters)
	cancel()
	require.Error(t, err)
	require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
	require.Empty(t, result.Products)
	require.True(t, cancelOnce.Load(), "cancel an in-flight query after transaction setup")
	cancelQuery.Store(false)
	assertDefaults() // Deadline rollback/discarded connection.
	pool.SetMaxOpenConns(4)
	pool.SetMaxIdleConns(4)
	var workers sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			found, err := b.Search(context.Background(), filters)
			if err == nil && (found.Total != 2 || len(found.Products) != 2) {
				err = fmt.Errorf("concurrent search differs: total=%d products=%d", found.Total, len(found.Products))
			}
			results <- err
		}()
	}
	workers.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	assertDefaults()
}
