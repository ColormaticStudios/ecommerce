package migrations

import (
	"ecommerce/models"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	return db
}

func noopMigration(version, name string) Migration {
	return Migration{
		Version: version,
		Name:    name,
		Up: func(tx *gorm.DB) error {
			return nil
		},
	}
}

func TestValidateMigrationsRejectsDuplicateVersion(t *testing.T) {
	err := validateMigrations([]Migration{
		noopMigration("2026030401_valid_name", "first"),
		noopMigration("2026030401_valid_name", "duplicate"),
	})
	require.ErrorContains(t, err, "duplicate migration version")
}

func TestValidateMigrationsRejectsMalformedVersion(t *testing.T) {
	err := validateMigrations([]Migration{
		noopMigration("not-a-version", "bad"),
	})
	require.ErrorContains(t, err, "invalid version format")
}

func TestValidateMigrationsRejectsInvalidTransactionMode(t *testing.T) {
	err := validateMigrations([]Migration{
		{
			Version:         "2026030501_bad_mode",
			Name:            "bad mode",
			TransactionMode: "invalid",
			Up: func(tx *gorm.DB) error {
				return nil
			},
		},
	})
	require.ErrorContains(t, err, "invalid transaction mode")
}

func TestValidateMigrationsRejectsContractWithoutBlockers(t *testing.T) {
	err := validateMigrations([]Migration{
		{
			Version: "2026030502_contract_without_blockers",
			Name:    "contract migration",
			Tags:    []string{"contract"},
			Up: func(tx *gorm.DB) error {
				return nil
			},
		},
	})
	require.ErrorContains(t, err, "must declare at least one contract blocker")
}

func TestRunWithMigrationsConcurrentRunnersApplyOnce(t *testing.T) {
	db := newTestDB(t)

	originalAcquire := acquireMigrationLock
	var lock sync.Mutex
	acquireMigrationLock = func(db *gorm.DB) (func() error, error) {
		lock.Lock()
		return func() error {
			lock.Unlock()
			return nil
		}, nil
	}
	t.Cleanup(func() {
		acquireMigrationLock = originalAcquire
	})

	var appliedCount int32
	definitions := []Migration{
		{
			Version: "2026030401_apply_once",
			Name:    "apply once under lock",
			Up: func(tx *gorm.DB) error {
				atomic.AddInt32(&appliedCount, 1)
				time.Sleep(40 * time.Millisecond)
				return nil
			},
		},
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- runWithMigrations(db, definitions)
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	require.Equal(t, int32(1), atomic.LoadInt32(&appliedCount))

	var rows []SchemaMigration
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, "2026030401_apply_once", rows[0].Version)
}

func TestRunWithMigrationsReturnsUnlockError(t *testing.T) {
	db := newTestDB(t)

	originalAcquire := acquireMigrationLock
	acquireMigrationLock = func(db *gorm.DB) (func() error, error) {
		return func() error {
			return errors.New("unlock failed")
		}, nil
	}
	t.Cleanup(func() {
		acquireMigrationLock = originalAcquire
	})

	err := runWithMigrations(db, []Migration{
		noopMigration("2026030405_unlock_error", "unlock error"),
	})
	require.ErrorContains(t, err, "unlock failed")
}

func TestRunWithMigrationsJoinsMigrationAndUnlockErrors(t *testing.T) {
	db := newTestDB(t)

	originalAcquire := acquireMigrationLock
	acquireMigrationLock = func(db *gorm.DB) (func() error, error) {
		return func() error {
			return errors.New("unlock failed")
		}, nil
	}
	t.Cleanup(func() {
		acquireMigrationLock = originalAcquire
	})

	err := runWithMigrations(db, []Migration{
		{
			Version: "2026030406_migration_error",
			Name:    "migration error",
			Up: func(tx *gorm.DB) error {
				return errors.New("migration up failed")
			},
		},
	})
	require.ErrorContains(t, err, "migration up failed")
	require.ErrorContains(t, err, "unlock failed")
}

func TestRunWithMigrationsNonTransactionalAndPostChecks(t *testing.T) {
	db := newTestDB(t)

	require.NoError(t, db.Exec(`CREATE TABLE test_table (id integer primary key, value integer not null default 0)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO test_table (id, value) VALUES (1, 0)`).Error)

	var postCheckRuns int32
	err := runWithMigrations(db, []Migration{
		{
			Version:         "2026030503_non_transactional_with_postcheck",
			Name:            "non transaction with postcheck",
			TransactionMode: TransactionModeNone,
			Tags:            []string{"backfill"},
			PostChecks: []PostCheck{
				{
					Name: "value_was_updated",
					Check: func(tx *gorm.DB) error {
						atomic.AddInt32(&postCheckRuns, 1)
						var count int64
						if err := tx.Raw(`SELECT COUNT(*) FROM test_table WHERE value = 1`).Scan(&count).Error; err != nil {
							return err
						}
						if count != 1 {
							return errors.New("expected one updated row")
						}
						return nil
					},
				},
			},
			Up: func(tx *gorm.DB) error {
				return tx.Exec(`UPDATE test_table SET value = 1 WHERE id = 1`).Error
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(&postCheckRuns))

	var row SchemaMigration
	require.NoError(t, db.Where("version = ?", "2026030503_non_transactional_with_postcheck").First(&row).Error)
	require.Equal(t, "non transaction with postcheck", row.Name)
	require.NotEmpty(t, row.ExecutionMeta)
	require.NotEmpty(t, row.Checksum)
}

func TestAppliedVersionsBackfillsMissingChecksum(t *testing.T) {
	db := newTestDB(t)

	definitions := []Migration{
		noopMigration("2026030601_checksum_backfill", "checksum backfill"),
	}
	require.NoError(t, runWithMigrations(db, definitions))

	require.NoError(t, db.Model(&SchemaMigration{}).
		Where("version = ?", "2026030601_checksum_backfill").
		Update("checksum", "").Error)

	applied, err := appliedVersionsWithMigrations(db, definitions)
	require.NoError(t, err)
	row, ok := applied["2026030601_checksum_backfill"]
	require.True(t, ok)
	require.NotEmpty(t, row.Checksum)
}

func TestAppliedVersionsFailsOnChecksumMismatch(t *testing.T) {
	db := newTestDB(t)

	definitions := []Migration{
		noopMigration("2026030602_checksum_mismatch", "checksum mismatch"),
	}
	require.NoError(t, runWithMigrations(db, definitions))

	require.NoError(t, db.Model(&SchemaMigration{}).
		Where("version = ?", "2026030602_checksum_mismatch").
		Update("checksum", "not-a-real-checksum").Error)

	_, err := appliedVersionsWithMigrations(db, definitions)
	require.ErrorContains(t, err, "checksum mismatch")
}

func TestStatusLinesOutput(t *testing.T) {
	db := newTestDB(t)
	definitions := []Migration{
		noopMigration("2026030401_create_users", "create users"),
		noopMigration("2026030402_add_indexes", "add indexes"),
	}

	require.NoError(t, ensureTable(db))
	require.NoError(t, db.Create(&SchemaMigration{
		Version:   "2026030401_create_users",
		AppliedAt: time.Now().UTC(),
	}).Error)

	status, err := statusForMigrations(db, definitions)
	require.NoError(t, err)
	require.Equal(t, "2026030402_add_indexes", status.LatestKnownVersion)
	require.Equal(t, "2026030401_create_users", status.LatestAppliedVersion)
	require.Equal(t, 1, status.PendingCount)

	lines := printStatusLines(status)
	require.Equal(t, []string{
		"latest_known_version=2026030402_add_indexes",
		"latest_applied_version=2026030401_create_users",
		"pending_count=1",
	}, lines)
}

func TestPlanLinesOutput(t *testing.T) {
	db := newTestDB(t)
	definitions := []Migration{
		noopMigration("2026030403_create_users", "create users"),
		noopMigration("2026030404_add_indexes", "add indexes"),
	}

	require.NoError(t, ensureTable(db))
	require.NoError(t, db.Create(&SchemaMigration{
		Version:   "2026030403_create_users",
		AppliedAt: time.Now().UTC(),
	}).Error)

	pending, err := pendingWithMigrations(db, definitions)
	require.NoError(t, err)
	lines := printPlanLines(pending)
	require.Equal(t, []string{
		"pending_count=1",
		"pending_01_version=2026030404_add_indexes",
		"pending_01_name=add indexes",
	}, lines)
}

func TestLintAutoMigrateUsageFailsForNewMigration(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "migrations.go")
	content := `package migrations
var orderedMigrations = []Migration{
  {
    Version: "2026022601_initial_schema",
    Name: "initial",
    Up: func(tx *gorm.DB) error { return tx.AutoMigrate(&models.User{}) },
  },
  {
    Version: "2026030504_new_schema",
    Name: "new schema",
    Up: func(tx *gorm.DB) error { return tx.AutoMigrate(&models.Order{}) },
  },
}
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	originalPath := migrationSourcePath
	migrationSourcePath = path
	t.Cleanup(func() {
		migrationSourcePath = originalPath
	})

	err := lintAutoMigrateUsage()
	require.ErrorContains(t, err, "must not call AutoMigrate directly")
}

func TestLintAutoMigrateUsageFailsForConstBackedNewMigration(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "migrations.go")
	content := `package migrations
const (
  initialSchemaVersion = "2026022601_initial_schema"
  addOrdersVersion = "2026030504_new_schema"
)
var orderedMigrations = []Migration{
  {
    Version: initialSchemaVersion,
    Name: "initial",
    Up: func(tx *gorm.DB) error { return tx.AutoMigrate(&models.User{}) },
  },
  {
    Version: addOrdersVersion,
    Name: "new schema",
    Up: func(tx *gorm.DB) error { return tx.AutoMigrate(&models.Order{}) },
  },
}
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	originalPath := migrationSourcePath
	migrationSourcePath = path
	t.Cleanup(func() {
		migrationSourcePath = originalPath
	})

	err := lintAutoMigrateUsage()
	require.ErrorContains(t, err, "must not call AutoMigrate directly")
}

func TestGuardPendingMigrationsContractBlocked(t *testing.T) {
	db := newTestDB(t)
	t.Setenv(contractGuardEnvVar, "false")

	err := guardPendingMigrations(db, []Migration{
		{
			Version:          "2026030505_contract_blocked",
			Name:             "contract blocked",
			Tags:             []string{"contract"},
			ContractBlockers: []string{"allow_contract_migrations"},
			Up: func(tx *gorm.DB) error {
				return nil
			},
		},
	})
	require.ErrorContains(t, err, contractGuardEnvVar)
}

func TestGuardPendingMigrationsContractAllowed(t *testing.T) {
	db := newTestDB(t)
	t.Setenv(contractGuardEnvVar, "true")

	err := guardPendingMigrations(db, []Migration{
		{
			Version:          "2026030506_contract_allowed",
			Name:             "contract allowed",
			Tags:             []string{"contract"},
			ContractBlockers: []string{"allow_contract_migrations"},
			Up: func(tx *gorm.DB) error {
				return nil
			},
		},
	})
	require.NoError(t, err)
}

func TestRunAppliesAllOrderedMigrationsAndReplayIsIdempotent(t *testing.T) {
	db := newTestDB(t)

	require.NoError(t, Run(db))
	require.NoError(t, Check(db))
	require.NoError(t, Run(db))

	var rows []SchemaMigration
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, len(orderedMigrations))

	status, err := StatusReport(db)
	require.NoError(t, err)
	require.Equal(t, LatestVersion(), status.LatestAppliedVersion)
	require.Equal(t, 0, status.PendingCount)
}

func TestProductPublishStateBackfillMigration(t *testing.T) {
	db := newTestDB(t)

	require.GreaterOrEqual(t, len(orderedMigrations), 2)
	require.NoError(t, runWithMigrations(db, orderedMigrations[:1]))

	inputs := []models.Product{
		{
			SKU:         "backfill-empty-draft",
			Name:        "Needs backfill",
			Description: "should be flipped to published",
			Price:       models.MoneyFromFloat(10),
			Stock:       1,
			DraftData:   "",
		},
		{
			SKU:         "backfill-has-draft",
			Name:        "Has draft payload",
			Description: "should remain unpublished",
			Price:       models.MoneyFromFloat(11),
			Stock:       1,
			DraftData:   `{"name":"draft"}`,
		},
		{
			SKU:         "backfill-already-published",
			Name:        "Already published",
			Description: "should remain published",
			Price:       models.MoneyFromFloat(12),
			Stock:       1,
			IsPublished: true,
			DraftData:   "",
		},
	}
	for _, product := range inputs {
		require.NoError(t, db.Create(&product).Error)
	}
	require.NoError(t, db.Model(&models.Product{}).Where("sku IN ?", []string{
		"backfill-empty-draft",
		"backfill-has-draft",
	}).Update("is_published", false).Error)

	require.NoError(t, runWithMigrations(db, orderedMigrations[:2]))

	var emptyDraft models.Product
	require.NoError(t, db.Where("sku = ?", "backfill-empty-draft").First(&emptyDraft).Error)
	require.True(t, emptyDraft.IsPublished)

	var hasDraft models.Product
	require.NoError(t, db.Where("sku = ?", "backfill-has-draft").First(&hasDraft).Error)
	require.False(t, hasDraft.IsPublished)

	var alreadyPublished models.Product
	require.NoError(t, db.Where("sku = ?", "backfill-already-published").First(&alreadyPublished).Error)
	require.True(t, alreadyPublished.IsPublished)

	var applied SchemaMigration
	require.NoError(t, db.Where("version = ?", productPublishBackfillVersion).First(&applied).Error)
	require.Equal(t, "backfill publish state for products with empty draft payload", applied.Name)
}
