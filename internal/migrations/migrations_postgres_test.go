package migrations

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openPostgresTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("MIGRATIONS_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set MIGRATIONS_TEST_POSTGRES_DSN to run Postgres migration integration tests")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func openIsolatedPostgresTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	baseDSN := os.Getenv("MIGRATIONS_TEST_POSTGRES_DSN")
	if baseDSN == "" {
		t.Skip("set MIGRATIONS_TEST_POSTGRES_DSN to run Postgres migration integration tests")
	}

	bootstrapDB, err := gorm.Open(postgres.Open(baseDSN), &gorm.Config{})
	require.NoError(t, err)

	schemaName := fmt.Sprintf("migration_test_%d", time.Now().UTC().UnixNano())
	require.NoError(t, bootstrapDB.Exec(fmt.Sprintf(`CREATE SCHEMA "%s"`, schemaName)).Error)

	dsnWithSchema, err := withPostgresSearchPath(baseDSN, schemaName)
	require.NoError(t, err)
	db, err := gorm.Open(postgres.Open(dsnWithSchema), &gorm.Config{})
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = bootstrapDB.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS "%s" CASCADE`, schemaName)).Error
	})

	return db
}

func withPostgresSearchPath(dsn, searchPath string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Scheme == "" {
		return "", fmt.Errorf("MIGRATIONS_TEST_POSTGRES_DSN must be a URL-formatted DSN")
	}
	query := parsed.Query()
	query.Set("search_path", searchPath)
	parsed.RawQuery = query.Encode()

	result := parsed.String()
	if strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("failed to build DSN with search_path")
	}
	return result, nil
}

func TestRunWithMigrationsConcurrentRunnersPostgresLockApplyOnce(t *testing.T) {
	db1 := openPostgresTestDB(t)
	db2 := openPostgresTestDB(t)

	require.NoError(t, ensureTable(db1))

	version := "2099010101_postgres_lock_apply_once"
	require.NoError(t, db1.Where("version = ?", version).Delete(&SchemaMigration{}).Error)

	var appliedCount int32
	definitions := []Migration{
		{
			Version: version,
			Name:    "postgres advisory lock apply once",
			Up: func(tx *gorm.DB) error {
				atomic.AddInt32(&appliedCount, 1)
				time.Sleep(100 * time.Millisecond)
				return nil
			},
		},
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs <- runWithMigrations(db1, definitions)
	}()
	go func() {
		defer wg.Done()
		errs <- runWithMigrations(db2, definitions)
	}()
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	require.Equal(t, int32(1), atomic.LoadInt32(&appliedCount))

	var rows []SchemaMigration
	require.NoError(t, db1.Where("version = ?", version).Find(&rows).Error)
	require.Len(t, rows, 1)
}

func TestRunReplayFromEmptyToLatestPostgres(t *testing.T) {
	db := openIsolatedPostgresTestDB(t)
	t.Setenv(contractGuardEnvVar, "true")

	require.NoError(t, Run(db))
	require.NoError(t, Check(db))
	require.NoError(t, Run(db))

	status, err := StatusReport(db)
	require.NoError(t, err)
	require.Equal(t, LatestVersion(), status.LatestAppliedVersion)
	require.Equal(t, 0, status.PendingCount)
}
