package migrations

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"ecommerce/internal/migrations/ops"
	"ecommerce/models"

	"gorm.io/gorm"
)

// SchemaMigration tracks applied migration versions.
type SchemaMigration struct {
	Version       string    `gorm:"primaryKey;size:64"`
	Name          string    `gorm:"not null;size:255;default:''"`
	Checksum      string    `gorm:"not null;size:64;default:''"`
	AppliedAt     time.Time `gorm:"not null"`
	DurationMs    int64     `gorm:"not null;default:0"`
	ExecutionMeta string    `gorm:"type:text;not null;default:''"`
}

type TransactionMode string

const (
	TransactionModeRequired TransactionMode = "required"
	TransactionModeNone     TransactionMode = "none"
)

type PostCheck struct {
	Name  string
	Check func(tx *gorm.DB) error
}

type Migration struct {
	Version          string
	Name             string
	Up               func(tx *gorm.DB) error
	TransactionMode  TransactionMode
	PostChecks       []PostCheck
	Tags             []string
	ContractBlockers []string
}

type Status struct {
	LatestKnownVersion   string
	LatestAppliedVersion string
	PendingCount         int
}

const advisoryLockKey int64 = 2172384190179656700
const migrationContractTag = "contract"
const contractGuardEnvVar = "MIGRATIONS_ALLOW_CONTRACT"
const defaultSchemaSnapshotPath = "internal/migrations/schema_snapshot.sql"
const initialSchemaVersion = "2026022601_initial_schema"
const productPublishBackfillVersion = "2026030501_backfill_product_publish_state"
const migrationStepAlertThresholdEnvVar = "MIGRATIONS_STEP_ALERT_THRESHOLD_MS"

var versionPattern = regexp.MustCompile(`^\d{10}_[a-z0-9_]+$`)
var tagPattern = regexp.MustCompile(`^[a-z0-9_]+$`)

var acquireMigrationLock = acquireMigrationLockForDB
var migrationSourcePath = "internal/migrations/migrations.go"

var orderedMigrations = []Migration{
	{
		Version:         initialSchemaVersion,
		Name:            "create core schema",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"expand", "baseline"},
		Up: func(tx *gorm.DB) error {
			return tx.AutoMigrate(
				&legacyUser{},
				&legacyProduct{},
				&legacyProductRelated{},
				&legacyOrder{},
				&legacyOrderItem{},
				&legacyCart{},
				&legacyCartItem{},
				&legacyMediaObject{},
				&legacyMediaVariant{},
				&legacyMediaReference{},
				&legacySavedPaymentMethod{},
				&legacySavedAddress{},
				&legacyStorefrontSettings{},
				&legacyCheckoutProviderSetting{},
			)
		},
	},
	{
		Version:         productPublishBackfillVersion,
		Name:            "backfill publish state for products with empty draft payload",
		TransactionMode: TransactionModeRequired,
		Tags:            []string{"backfill"},
		PostChecks: []PostCheck{
			{
				Name: "products_publish_state_backfill_applied",
				Check: func(tx *gorm.DB) error {
					var count int64
					if err := tx.Model(&models.Product{}).
						Where("is_published = ? AND (draft_data IS NULL OR draft_data = '')", false).
						Count(&count).Error; err != nil {
						return err
					}
					if count > 0 {
						return fmt.Errorf("post-check failed: found %d unpublished products with empty draft_data", count)
					}
					return nil
				},
			},
		},
		Up: func(tx *gorm.DB) error {
			_, err := ops.BatchedBackfillByID(
				tx,
				"products",
				"id",
				250,
				func(tx *gorm.DB, ids []int64) (int64, error) {
					result := tx.Model(&models.Product{}).
						Where("id IN ?", ids).
						Where("is_published = ? AND (draft_data IS NULL OR draft_data = '')", false).
						Update("is_published", true)
					return result.RowsAffected, result.Error
				},
				log.Printf,
			)
			return err
		},
	},
}

func ensureTable(db *gorm.DB) error {
	return db.AutoMigrate(&SchemaMigration{})
}

func AppliedVersions(db *gorm.DB) (map[string]SchemaMigration, error) {
	return appliedVersionsWithMigrations(db, orderedMigrations)
}

func appliedVersionsWithMigrations(db *gorm.DB, definitions []Migration) (map[string]SchemaMigration, error) {
	if err := validateMigrations(definitions); err != nil {
		return nil, err
	}

	if err := ensureTable(db); err != nil {
		return nil, err
	}

	var rows []SchemaMigration
	if err := db.Find(&rows).Error; err != nil {
		return nil, err
	}

	if err := validateAndBackfillAppliedChecksums(db, definitions, rows); err != nil {
		return nil, err
	}

	applied := make(map[string]SchemaMigration, len(rows))
	for _, row := range rows {
		applied[row.Version] = row
	}
	return applied, nil
}

func Pending(db *gorm.DB) ([]Migration, error) {
	return pendingWithMigrations(db, orderedMigrations)
}

func Run(db *gorm.DB) error {
	return runWithMigrations(db, orderedMigrations)
}

func StatusReport(db *gorm.DB) (Status, error) {
	return statusForMigrations(db, orderedMigrations)
}

func runWithMigrations(db *gorm.DB, definitions []Migration) (runErr error) {
	if err := validateMigrations(definitions); err != nil {
		return err
	}

	unlock, err := acquireMigrationLock(db)
	if err != nil {
		return err
	}
	defer func() {
		if err := unlock(); err != nil {
			if runErr != nil {
				runErr = errors.Join(runErr, err)
				return
			}
			runErr = err
		}
	}()

	pending, err := pendingWithMigrations(db, definitions)
	if err != nil {
		return err
	}

	if err := guardPendingMigrations(db, pending); err != nil {
		return err
	}

	for _, migration := range pending {
		start := time.Now().UTC()
		log.Printf("migration_step_start version=%s name=%q transaction_mode=%s tags=%s", migration.Version, migration.Name, normalizeTransactionMode(migration.TransactionMode), strings.Join(migration.Tags, ","))

		durationMs := int64(0)
		rowsTouched := int64(0)
		checkResult := "ok"

		runErr := executeMigration(db, migration, &rowsTouched, &checkResult, &durationMs)
		if runErr != nil {
			log.Printf("migration_step_failed version=%s name=%q duration_ms=%d rows_touched=%d check_result=%s error=%q", migration.Version, migration.Name, durationMs, rowsTouched, checkResult, runErr.Error())
			return fmt.Errorf("migration %s (%s): %w", migration.Version, migration.Name, runErr)
		}

		meta, metaErr := buildExecutionMeta(migration, rowsTouched, checkResult)
		if metaErr != nil {
			return fmt.Errorf("migration %s (%s): %w", migration.Version, migration.Name, metaErr)
		}
		checksum := migrationChecksum(migration)

		if err := db.Create(&SchemaMigration{
			Version:       migration.Version,
			Name:          migration.Name,
			Checksum:      checksum,
			AppliedAt:     start,
			DurationMs:    durationMs,
			ExecutionMeta: meta,
		}).Error; err != nil {
			return fmt.Errorf("migration %s (%s): %w", migration.Version, migration.Name, err)
		}

		log.Printf("migration_step_complete version=%s name=%q duration_ms=%d rows_touched=%d check_result=%s", migration.Version, migration.Name, durationMs, rowsTouched, checkResult)
		alertThreshold := migrationStepAlertThresholdMs()
		if durationMs > alertThreshold {
			log.Printf("migration_step_alert version=%s name=%q duration_ms=%d threshold_ms=%d", migration.Version, migration.Name, durationMs, alertThreshold)
		}
	}
	return runErr
}

func executeMigration(db *gorm.DB, migration Migration, rowsTouched *int64, checkResult *string, durationMs *int64) error {
	mode := normalizeTransactionMode(migration.TransactionMode)
	start := time.Now().UTC()

	if mode == TransactionModeNone {
		runDB, counter := ops.AttachRowsCounter(db)
		if err := migration.Up(runDB); err != nil {
			*durationMs = time.Since(start).Milliseconds()
			*rowsTouched = ops.ReadRowsCounter(counter)
			*checkResult = "failed"
			return err
		}
		if err := runPostChecks(runDB, migration.PostChecks); err != nil {
			*durationMs = time.Since(start).Milliseconds()
			*rowsTouched = ops.ReadRowsCounter(counter)
			*checkResult = "failed"
			return err
		}
		*durationMs = time.Since(start).Milliseconds()
		*rowsTouched = ops.ReadRowsCounter(counter)
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		runTx, counter := ops.AttachRowsCounter(tx)
		if err := migration.Up(runTx); err != nil {
			*durationMs = time.Since(start).Milliseconds()
			*rowsTouched = ops.ReadRowsCounter(counter)
			*checkResult = "failed"
			return err
		}
		if err := runPostChecks(runTx, migration.PostChecks); err != nil {
			*durationMs = time.Since(start).Milliseconds()
			*rowsTouched = ops.ReadRowsCounter(counter)
			*checkResult = "failed"
			return err
		}
		*durationMs = time.Since(start).Milliseconds()
		*rowsTouched = ops.ReadRowsCounter(counter)
		return nil
	})
}

func runPostChecks(tx *gorm.DB, checks []PostCheck) error {
	for _, check := range checks {
		if err := check.Check(tx); err != nil {
			return fmt.Errorf("post-check %q failed: %w", check.Name, err)
		}
	}
	return nil
}

func buildExecutionMeta(migration Migration, rowsTouched int64, checkResult string) (string, error) {
	meta := map[string]any{
		"transaction_mode": normalizeTransactionMode(migration.TransactionMode),
		"rows_touched":     rowsTouched,
		"check_result":     checkResult,
	}
	if len(migration.Tags) > 0 {
		meta["tags"] = migration.Tags
	}
	if len(migration.ContractBlockers) > 0 {
		meta["contract_blockers"] = migration.ContractBlockers
	}

	encoded, err := json.Marshal(meta)
	if err != nil {
		return "", fmt.Errorf("failed to encode execution metadata: %w", err)
	}
	return string(encoded), nil
}

func pendingWithMigrations(db *gorm.DB, definitions []Migration) ([]Migration, error) {
	if err := validateMigrations(definitions); err != nil {
		return nil, err
	}

	applied, err := appliedVersionsWithMigrations(db, definitions)
	if err != nil {
		return nil, err
	}

	pending := make([]Migration, 0)
	for _, migration := range definitions {
		if _, ok := applied[migration.Version]; ok {
			continue
		}
		pending = append(pending, migration)
	}
	return pending, nil
}

func statusForMigrations(db *gorm.DB, definitions []Migration) (Status, error) {
	if err := validateMigrations(definitions); err != nil {
		return Status{}, err
	}

	applied, err := appliedVersionsWithMigrations(db, definitions)
	if err != nil {
		return Status{}, err
	}

	latestAppliedVersion := ""
	for version := range applied {
		if version > latestAppliedVersion {
			latestAppliedVersion = version
		}
	}

	pendingCount := 0
	for _, migration := range definitions {
		if _, ok := applied[migration.Version]; !ok {
			pendingCount++
		}
	}

	return Status{
		LatestKnownVersion:   latestVersionFor(definitions),
		LatestAppliedVersion: latestAppliedVersion,
		PendingCount:         pendingCount,
	}, nil
}

func validateMigrations(definitions []Migration) error {
	seen := make(map[string]struct{}, len(definitions))
	prevVersion := ""
	for idx, migration := range definitions {
		if migration.Version == "" {
			return fmt.Errorf("migration at index %d has empty version", idx)
		}
		if !versionPattern.MatchString(migration.Version) {
			return fmt.Errorf("migration %q has invalid version format (expected YYYYMMDDNN_slug)", migration.Version)
		}
		if migration.Name == "" {
			return fmt.Errorf("migration %q has empty name", migration.Version)
		}
		if migration.Up == nil {
			return fmt.Errorf("migration %q has nil Up function", migration.Version)
		}
		if migration.TransactionMode != "" &&
			migration.TransactionMode != TransactionModeRequired &&
			migration.TransactionMode != TransactionModeNone {
			return fmt.Errorf("migration %q has invalid transaction mode %q", migration.Version, migration.TransactionMode)
		}
		for tagIdx, tag := range migration.Tags {
			if tag == "" {
				return fmt.Errorf("migration %q has empty tag at index %d", migration.Version, tagIdx)
			}
			if !tagPattern.MatchString(tag) {
				return fmt.Errorf("migration %q has invalid tag %q", migration.Version, tag)
			}
		}
		for checkIdx, check := range migration.PostChecks {
			if check.Name == "" {
				return fmt.Errorf("migration %q has empty post-check name at index %d", migration.Version, checkIdx)
			}
			if check.Check == nil {
				return fmt.Errorf("migration %q has nil post-check handler for %q", migration.Version, check.Name)
			}
		}
		for blockerIdx, blocker := range migration.ContractBlockers {
			if blocker == "" {
				return fmt.Errorf("migration %q has empty contract blocker at index %d", migration.Version, blockerIdx)
			}
		}
		if hasTag(migration.Tags, migrationContractTag) && len(migration.ContractBlockers) == 0 {
			return fmt.Errorf("migration %q is tagged contract and must declare at least one contract blocker", migration.Version)
		}
		if _, exists := seen[migration.Version]; exists {
			return fmt.Errorf("duplicate migration version %q", migration.Version)
		}
		if prevVersion != "" && migration.Version <= prevVersion {
			return fmt.Errorf("migration %q is out of order (must be strictly increasing)", migration.Version)
		}
		seen[migration.Version] = struct{}{}
		prevVersion = migration.Version
	}
	return nil
}

func acquireMigrationLockForDB(db *gorm.DB) (func() error, error) {
	if db.Dialector.Name() != "postgres" {
		return func() error { return nil }, nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql db for migration advisory lock: %w", err)
	}

	conn, err := sqlDB.Conn(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to open migration advisory lock session: %w", err)
	}

	if _, err := conn.ExecContext(context.Background(), "SELECT pg_advisory_lock($1)", advisoryLockKey); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to acquire migration advisory lock: %w", err)
	}

	return func() (releaseErr error) {
		defer func() {
			closeErr := conn.Close()
			if closeErr != nil {
				if releaseErr != nil {
					releaseErr = errors.Join(releaseErr, fmt.Errorf("failed to close migration advisory lock session: %w", closeErr))
				} else {
					releaseErr = fmt.Errorf("failed to close migration advisory lock session: %w", closeErr)
				}
			}
		}()

		var unlocked bool
		if err := conn.QueryRowContext(context.Background(), "SELECT pg_advisory_unlock($1)", advisoryLockKey).Scan(&unlocked); err != nil {
			releaseErr = fmt.Errorf("failed to release migration advisory lock: %w", err)
			return
		}
		if !unlocked {
			releaseErr = errors.New("failed to release migration advisory lock: lock was not held on migration session")
			return
		}
		return
	}, nil
}

func latestVersionFor(definitions []Migration) string {
	if len(definitions) == 0 {
		return ""
	}
	return definitions[len(definitions)-1].Version
}

func normalizeTransactionMode(mode TransactionMode) TransactionMode {
	if mode == "" {
		return TransactionModeRequired
	}
	return mode
}

func hasTag(tags []string, expected string) bool {
	for _, tag := range tags {
		if tag == expected {
			return true
		}
	}
	return false
}

func latestVersionOrUnknown(version string) string {
	if version == "" {
		return "none"
	}
	return version
}

func printStatusLines(status Status) []string {
	return []string{
		fmt.Sprintf("latest_known_version=%s", latestVersionOrUnknown(status.LatestKnownVersion)),
		fmt.Sprintf("latest_applied_version=%s", latestVersionOrUnknown(status.LatestAppliedVersion)),
		fmt.Sprintf("pending_count=%d", status.PendingCount),
	}
}

func StatusLines(db *gorm.DB) ([]string, error) {
	status, err := StatusReport(db)
	if err != nil {
		return nil, err
	}
	return printStatusLines(status), nil
}

func printPlanLines(pending []Migration) []string {
	lines := []string{
		fmt.Sprintf("pending_count=%d", len(pending)),
	}
	for idx, migration := range pending {
		lines = append(lines,
			fmt.Sprintf("pending_%02d_version=%s", idx+1, migration.Version),
			fmt.Sprintf("pending_%02d_name=%s", idx+1, migration.Name),
		)
	}
	return lines
}

func PlanLines(db *gorm.DB) ([]string, error) {
	pending, err := Pending(db)
	if err != nil {
		return nil, err
	}
	return printPlanLines(pending), nil
}

func Check(db *gorm.DB) error {
	status, err := StatusReport(db)
	if err != nil {
		return err
	}
	if status.PendingCount > 0 {
		return errors.New("database is not at latest migration")
	}
	return nil
}

func EnsureReady(db *gorm.DB, autoApply bool) error {
	if autoApply {
		return Run(db)
	}

	status, err := StatusReport(db)
	if err != nil {
		return err
	}
	if status.PendingCount > 0 {
		return fmt.Errorf(
			"database has %d pending migrations (latest_applied=%s latest_known=%s); run `make migrate` or set AUTO_APPLY_MIGRATIONS=true",
			status.PendingCount,
			latestVersionOrUnknown(status.LatestAppliedVersion),
			latestVersionOrUnknown(status.LatestKnownVersion),
		)
	}
	return nil
}

func LatestVersion() string {
	return latestVersionFor(orderedMigrations)
}

func Versions() []string {
	versions := make([]string, 0, len(orderedMigrations))
	for _, migration := range orderedMigrations {
		versions = append(versions, migration.Version)
	}
	sort.Strings(versions)
	return versions
}

func validateAndBackfillAppliedChecksums(db *gorm.DB, definitions []Migration, rows []SchemaMigration) error {
	definitionsByVersion := make(map[string]Migration, len(definitions))
	for _, definition := range definitions {
		definitionsByVersion[definition.Version] = definition
	}

	for idx := range rows {
		row := &rows[idx]
		definition, ok := definitionsByVersion[row.Version]
		if !ok {
			return fmt.Errorf("applied migration %s is unknown to current binary", row.Version)
		}

		expectedChecksum := migrationChecksum(definition)
		if strings.TrimSpace(row.Checksum) == "" {
			if err := db.Model(&SchemaMigration{}).
				Where("version = ?", row.Version).
				Update("checksum", expectedChecksum).Error; err != nil {
				return fmt.Errorf("failed to backfill checksum for applied migration %s: %w", row.Version, err)
			}
			row.Checksum = expectedChecksum
			continue
		}
		if row.Checksum != expectedChecksum {
			return fmt.Errorf(
				"applied migration %s checksum mismatch (stored=%s current=%s)",
				row.Version,
				row.Checksum,
				expectedChecksum,
			)
		}
	}
	return nil
}

func migrationChecksum(migration Migration) string {
	tags := append([]string(nil), migration.Tags...)
	sort.Strings(tags)
	contractBlockers := append([]string(nil), migration.ContractBlockers...)
	sort.Strings(contractBlockers)

	upSignature := "nil"
	if migration.Up != nil {
		pc := runtime.FuncForPC(reflect.ValueOf(migration.Up).Pointer())
		if pc != nil {
			file, line := pc.FileLine(pc.Entry())
			upSignature = fmt.Sprintf("%s|%s:%d", pc.Name(), file, line)
		}
	}

	fingerprint := map[string]any{
		"version":           migration.Version,
		"name":              migration.Name,
		"transaction_mode":  normalizeTransactionMode(migration.TransactionMode),
		"tags":              tags,
		"contract_blockers": contractBlockers,
		"up_signature":      upSignature,
		"post_checks_count": len(migration.PostChecks),
	}
	encoded, err := json.Marshal(fingerprint)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(encoded)
	return fmt.Sprintf("%x", sum)
}

func SourcePath() string {
	return migrationSourcePath
}

func DefaultSchemaSnapshotPath() string {
	return defaultSchemaSnapshotPath
}

func allowContractMigrations() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(contractGuardEnvVar)), "true")
}

func migrationStepAlertThresholdMs() int64 {
	raw := strings.TrimSpace(os.Getenv(migrationStepAlertThresholdEnvVar))
	if raw == "" {
		return 30_000
	}
	parsed, err := time.ParseDuration(raw + "ms")
	if err != nil {
		return 30_000
	}
	return parsed.Milliseconds()
}
