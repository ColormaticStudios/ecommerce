package migrations

import (
	"ecommerce/models"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	require.Len(t, row.Checksum, 64)
}

func TestAppliedVersionsBackfillsLegacyChecksum(t *testing.T) {
	db := newTestDB(t)

	definitions := []Migration{
		noopMigration("2026030409_legacy_checksum_backfill", "legacy checksum backfill"),
	}
	require.NoError(t, runWithMigrations(db, definitions))

	require.NoError(t, db.Model(&SchemaMigration{}).
		Where("version = ?", "2026030409_legacy_checksum_backfill").
		Update("checksum", strings.Repeat("b", 64)).Error)

	applied, err := appliedVersionsWithMigrations(db, definitions)
	require.NoError(t, err)
	row, ok := applied["2026030409_legacy_checksum_backfill"]
	require.True(t, ok)
	require.Equal(t, migrationChecksum(definitions[0]), row.Checksum)
}

func TestAppliedVersionsFailsOnFirstSourceChecksumMismatch(t *testing.T) {
	db := newTestDB(t)

	definitions := []Migration{
		noopMigration(productCatalogDepthP0Version, "source checksum mismatch"),
	}
	require.NoError(t, runWithMigrations(db, definitions))

	require.NoError(t, db.Model(&SchemaMigration{}).
		Where("version = ?", productCatalogDepthP0Version).
		Update("checksum", strings.Repeat("c", 64)).Error)

	_, err := appliedVersionsWithMigrations(db, definitions)
	require.ErrorContains(t, err, "checksum mismatch")
}

func TestAppliedVersionsFailsOnChecksumMismatch(t *testing.T) {
	db := newTestDB(t)

	definitions := []Migration{
		noopMigration("2026030602_checksum_mismatch", "checksum mismatch"),
	}
	require.NoError(t, runWithMigrations(db, definitions))

	require.NoError(t, db.Model(&SchemaMigration{}).
		Where("version = ?", "2026030602_checksum_mismatch").
		Update("checksum", strings.Repeat("a", 64)).Error)

	_, err := appliedVersionsWithMigrations(db, definitions)
	require.ErrorContains(t, err, "checksum mismatch")
}

func TestAppliedVersionsBackfillsKnownCompatibleChecksum(t *testing.T) {
	db := newTestDB(t)

	definitions := []Migration{
		noopMigration(productCatalogDepthP2Version, "compatible checksum backfill"),
	}
	require.NoError(t, runWithMigrations(db, definitions))

	require.NoError(t, db.Model(&SchemaMigration{}).
		Where("version = ?", productCatalogDepthP2Version).
		Update("checksum", "5a483908a1331a23cfcfa2ab5b4992a5f63fd50e7cef4f732013328f23ca4329").Error)

	applied, err := appliedVersionsWithMigrations(db, definitions)
	require.NoError(t, err)
	row, ok := applied[productCatalogDepthP2Version]
	require.True(t, ok)
	require.Equal(t, migrationChecksum(definitions[0]), row.Checksum)
}

func TestMigrationChecksumStableAcrossLaterFileEdits(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "migrations.go")
	baseContent := `package migrations
const firstVersion = "2026030610_first"
var orderedMigrations = []Migration{
  {
    Version: firstVersion,
    Name: "first",
    Up: func(tx *gorm.DB) error {
      return nil
    },
  },
}
`
	expandedContent := `package migrations
const firstVersion = "2026030610_first"
const secondVersion = "2026030611_second"
var orderedMigrations = []Migration{
  {
    Version: firstVersion,
    Name: "first",
    Up: func(tx *gorm.DB) error {
      return nil
    },
  },
  {
    Version: secondVersion,
    Name: "second",
    Up: func(tx *gorm.DB) error {
      return nil
    },
  },
}
`
	require.NoError(t, os.WriteFile(path, []byte(baseContent), 0o644))

	originalPath := migrationSourcePath
	migrationSourcePath = path
	t.Cleanup(func() {
		migrationSourcePath = originalPath
	})

	migration := Migration{Version: "2026030610_first", Name: "first"}
	before := migrationChecksum(migration)
	require.NoError(t, os.WriteFile(path, []byte(expandedContent), 0o644))
	after := migrationChecksum(migration)
	require.Equal(t, before, after)
	require.Len(t, after, 64)
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

func TestMigrationChecksumSourceUsesEmbeddedSourceAtDefaultPath(t *testing.T) {
	originalPath := migrationSourcePath
	migrationSourcePath = "internal/migrations/migrations.go"
	t.Cleanup(func() {
		migrationSourcePath = originalPath
	})

	before := migrationChecksumSource(productCatalogDepthP0Version)
	require.NotEmpty(t, before)

	cwd, err := os.Getwd()
	require.NoError(t, err)

	tempDir := t.TempDir()
	require.NoError(t, os.Chdir(tempDir))
	t.Cleanup(func() {
		require.NoError(t, os.Chdir(cwd))
	})

	after := migrationChecksumSource(productCatalogDepthP0Version)
	require.Equal(t, before, after)
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
	}, true)
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
	}, true)
	require.NoError(t, err)
}

func TestRunWithMigrationsDoesNotBlockContractMigrationsWithoutAcknowledgement(t *testing.T) {
	db := newTestDB(t)
	t.Setenv(contractGuardEnvVar, "false")

	err := runWithMigrations(db, []Migration{
		{
			Version:          "2026030702_contract_run_allowed",
			Name:             "contract migration still runs",
			Tags:             []string{"contract"},
			ContractBlockers: []string{"allow_contract_migrations"},
			Up: func(tx *gorm.DB) error {
				return nil
			},
		},
	})
	require.NoError(t, err)
}

func TestRunWithoutContractSkipsContractMigrations(t *testing.T) {
	db := newTestDB(t)

	require.NoError(t, RunWithoutContract(db))

	status, err := statusForMigrations(db, orderedMigrations)
	require.NoError(t, err)
	require.Equal(t, guestCheckoutP3Version, status.LatestAppliedVersion)
	require.Equal(t, 1, status.PendingCount)
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

	inputs := []legacyProduct{
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
	require.NoError(t, db.Model(&legacyProduct{}).Where("sku IN ?", []string{
		"backfill-empty-draft",
		"backfill-has-draft",
	}).Update("is_published", false).Error)

	require.NoError(t, runWithMigrations(db, orderedMigrations[:2]))

	var emptyDraft legacyProduct
	require.NoError(t, db.Where("sku = ?", "backfill-empty-draft").First(&emptyDraft).Error)
	require.True(t, emptyDraft.IsPublished)

	var hasDraft legacyProduct
	require.NoError(t, db.Where("sku = ?", "backfill-has-draft").First(&hasDraft).Error)
	require.False(t, hasDraft.IsPublished)

	var alreadyPublished legacyProduct
	require.NoError(t, db.Where("sku = ?", "backfill-already-published").First(&alreadyPublished).Error)
	require.True(t, alreadyPublished.IsPublished)

	var applied SchemaMigration
	require.NoError(t, db.Where("version = ?", productPublishBackfillVersion).First(&applied).Error)
	require.Equal(t, "backfill publish state for products with empty draft payload", applied.Name)
}

func TestCatalogDepthP0MigrationCreatesCatalogTables(t *testing.T) {
	db := newTestDB(t)
	t.Setenv(contractGuardEnvVar, "true")
	require.NoError(t, Run(db))

	required := []any{
		&models.Brand{},
		&models.ProductOption{},
		&models.ProductOptionValue{},
		&models.ProductVariant{},
		&models.ProductVariantOptionValue{},
		&models.ProductAttribute{},
		&models.ProductAttributeValue{},
		&models.SEOMetadata{},
		&models.ProductDraft{},
		&models.ProductOptionDraft{},
		&models.ProductOptionValueDraft{},
		&models.ProductVariantDraft{},
		&models.ProductVariantOptionValueDraft{},
		&models.ProductAttributeValueDraft{},
		&models.ProductRelatedDraft{},
	}

	for _, model := range required {
		assert.True(t, db.Migrator().HasTable(model), "expected migrated table for %T", model)
	}
}

func TestCatalogDepthP2BackfillsDefaultVariantForLegacyProducts(t *testing.T) {
	db := newTestDB(t)
	t.Setenv(contractGuardEnvVar, "true")
	require.NoError(t, runWithMigrations(db, orderedMigrations[:2]))

	legacy := legacyProduct{
		SKU:         "legacy-no-variant",
		Name:        "Legacy Product",
		Description: "Flat product only",
		Price:       models.MoneyFromFloat(19.99),
		Stock:       7,
		IsPublished: true,
	}
	require.NoError(t, db.Create(&legacy).Error)

	require.NoError(t, runWithMigrations(db, orderedMigrations))

	var product models.Product
	require.NoError(t, db.Where("id = ?", legacy.ID).First(&product).Error)
	require.NotNil(t, product.DefaultVariantID)

	var variant models.ProductVariant
	require.NoError(t, db.First(&variant, *product.DefaultVariantID).Error)
	assert.Equal(t, product.ID, variant.ProductID)
	assert.Equal(t, product.SKU, variant.SKU)
	assert.Equal(t, product.Name, variant.Title)
	assert.Equal(t, product.Price, variant.Price)
	assert.Equal(t, product.Stock, variant.Stock)
}

func TestCatalogDepthP4BackfillsLegacyDraftBlobAndDropsColumn(t *testing.T) {
	db := newTestDB(t)
	t.Setenv(contractGuardEnvVar, "true")
	require.NoError(t, runWithMigrations(db, orderedMigrations[:1]))

	legacy := legacyProduct{
		SKU:         "legacy-draft-product",
		Name:        "Legacy Product",
		Description: "legacy live description",
		Price:       models.MoneyFromFloat(19.99),
		Stock:       4,
		IsPublished: false,
		DraftData:   `{"sku":"legacy-draft-product","name":"Legacy Draft","description":"legacy draft description","price":24.5,"stock":6,"images":["legacy-a","legacy-b","legacy-a"],"related_ids":[7,7]}`,
	}
	require.NoError(t, db.Create(&legacy).Error)

	require.NoError(t, runWithMigrations(db, orderedMigrations))

	assert.False(t, db.Migrator().HasColumn("products", "draft_data"))

	var draft models.ProductDraft
	require.NoError(t, db.Where("product_id = ?", legacy.ID).First(&draft).Error)
	assert.Equal(t, "Legacy Draft", draft.Name)
	assert.Equal(t, "legacy draft description", draft.Description)
	assert.Equal(t, models.MoneyFromFloat(24.5), draft.Price)
	assert.Equal(t, 6, draft.Stock)
	assert.Equal(t, "legacy-draft-product", draft.DefaultVariantSKU)

	var variantDrafts []models.ProductVariantDraft
	require.NoError(t, db.Where("product_draft_id = ?", draft.ID).Order("position asc").Find(&variantDrafts).Error)
	require.Len(t, variantDrafts, 1)
	assert.Equal(t, "legacy-draft-product", variantDrafts[0].SKU)
	assert.Equal(t, "Legacy Draft", variantDrafts[0].Title)
	assert.Equal(t, models.MoneyFromFloat(24.5), variantDrafts[0].Price)
	assert.Equal(t, 6, variantDrafts[0].Stock)

	var relatedDrafts []models.ProductRelatedDraft
	require.NoError(t, db.Where("product_draft_id = ?", draft.ID).Order("position asc").Find(&relatedDrafts).Error)
	require.Len(t, relatedDrafts, 1)
	assert.Equal(t, uint(7), relatedDrafts[0].RelatedProductID)
}

func TestGuestCheckoutP0BackfillsCheckoutSessionsForLegacyCarts(t *testing.T) {
	db := newTestDB(t)
	t.Setenv(contractGuardEnvVar, "true")

	guestCheckoutIndex := slices.IndexFunc(orderedMigrations, func(m Migration) bool {
		return m.Version == guestCheckoutP0Version
	})
	require.NotEqual(t, -1, guestCheckoutIndex)
	require.NoError(t, runWithMigrations(db, orderedMigrations[:guestCheckoutIndex]))

	user := legacyUser{
		Subject:  "legacy-cart-user",
		Username: "legacy-cart-user",
		Email:    "legacy-cart-user@example.com",
		Role:     "customer",
		Currency: "USD",
	}
	require.NoError(t, db.Create(&user).Error)
	cart := legacyCart{UserID: user.ID}
	require.NoError(t, db.Create(&cart).Error)

	require.NoError(t, runWithMigrations(db, orderedMigrations))

	assert.True(t, db.Migrator().HasTable(&models.CheckoutSession{}))
	assert.False(t, db.Migrator().HasColumn("carts", "user_id"))
	assert.True(t, db.Migrator().HasColumn("carts", "checkout_session_id"))

	var checkoutSession models.CheckoutSession
	require.NoError(t, db.Where("user_id = ?", user.ID).First(&checkoutSession).Error)
	assert.Equal(t, models.CheckoutSessionStatusActive, checkoutSession.Status)
	assert.NotEmpty(t, checkoutSession.PublicToken)

	var reloadedCart models.Cart
	require.NoError(t, db.First(&reloadedCart, cart.ID).Error)
	assert.Equal(t, checkoutSession.ID, reloadedCart.CheckoutSessionID)
}

func TestGuestCheckoutP1BackfillsCheckoutSessionsForLegacyOrders(t *testing.T) {
	db := newTestDB(t)
	t.Setenv(contractGuardEnvVar, "true")

	guestCheckoutIndex := slices.IndexFunc(orderedMigrations, func(m Migration) bool {
		return m.Version == guestCheckoutP1Version
	})
	require.NotEqual(t, -1, guestCheckoutIndex)
	require.NoError(t, runWithMigrations(db, orderedMigrations[:guestCheckoutIndex]))

	user := legacyUser{
		Subject:  "legacy-order-user",
		Username: "legacy-order-user",
		Email:    "legacy-order-user@example.com",
		Role:     "customer",
		Currency: "USD",
	}
	require.NoError(t, db.Create(&user).Error)
	order := legacyOrder{
		UserID: user.ID,
		Total:  models.MoneyFromFloat(42),
		Status: models.StatusPending,
	}
	require.NoError(t, db.Create(&order).Error)

	require.NoError(t, runWithMigrations(db, orderedMigrations))

	assert.True(t, db.Migrator().HasColumn("orders", "checkout_session_id"))
	assert.True(t, db.Migrator().HasColumn("orders", "guest_email"))
	assert.True(t, db.Migrator().HasColumn("orders", "confirmation_token"))

	var reloaded models.Order
	require.NoError(t, db.First(&reloaded, order.ID).Error)
	require.NotZero(t, reloaded.CheckoutSessionID)
	require.NotNil(t, reloaded.UserID)
	assert.Equal(t, user.ID, *reloaded.UserID)

	var checkoutSession models.CheckoutSession
	require.NoError(t, db.First(&checkoutSession, reloaded.CheckoutSessionID).Error)
	assert.Equal(t, models.CheckoutSessionStatusConverted, checkoutSession.Status)
	require.NotNil(t, checkoutSession.UserID)
	assert.Equal(t, user.ID, *checkoutSession.UserID)
}

func TestGuestCheckoutP3AddsClaimAndIdempotencyStructures(t *testing.T) {
	db := newTestDB(t)
	t.Setenv(contractGuardEnvVar, "true")

	guestCheckoutIndex := slices.IndexFunc(orderedMigrations, func(m Migration) bool {
		return m.Version == guestCheckoutP3Version
	})
	require.NotEqual(t, -1, guestCheckoutIndex)
	require.NoError(t, runWithMigrations(db, orderedMigrations[:guestCheckoutIndex+1]))

	assert.True(t, db.Migrator().HasColumn("orders", "claimed_at"))
	assert.True(t, db.Migrator().HasTable(&models.IdempotencyKey{}))
}
