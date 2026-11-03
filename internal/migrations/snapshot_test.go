package migrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSchemaSnapshotAndDriftCheck(t *testing.T) {
	db := newTestDB(t)
	t.Setenv(contractGuardEnvVar, "true")
	require.NoError(t, Run(db))

	snapshot, err := SchemaSnapshot(db)
	require.NoError(t, err)
	require.Contains(t, snapshot, "COLUMN id")
	require.Contains(t, snapshot, "INDEX")
	require.True(t, strings.Count(snapshot, "TABLE ") > 0)

	path := filepath.Join(t.TempDir(), "schema_snapshot.sql")
	require.NoError(t, WriteSchemaSnapshot(db, path))
	require.NoError(t, DriftCheck(db, path))

	require.NoError(t, db.Exec(`CREATE TABLE drift_test (id integer primary key)`).Error)
	require.ErrorContains(t, DriftCheck(db, path), "schema drift detected")
}

func TestCommittedSchemaSnapshotMatchesLatestMigrations(t *testing.T) {
	db := newTestDB(t)
	t.Setenv(contractGuardEnvVar, "true")
	require.NoError(t, Run(db))
	require.NoError(t, DriftCheck(db, "schema_snapshot.sql"))
}

func TestSnapshotNormalizesGeneratedIdentifierLimitsOnly(t *testing.T) {
	table := "product_variant_option_value_drafts"
	columns := []string{"product_variant_draft_id"}
	require.Equal(t, "idx_product_variant_option_value_drafts_product_varianta2ba91fe", canonicalSnapshotIndexName(table, "idx_product_variant_option_value_drafts_product_variant_draft_id", columns))
	require.Equal(t, "idx_explicit_variant_order", canonicalSnapshotIndexName(table, "idx_explicit_variant_order", columns))
}

func TestWriteCurrentSchemaSnapshotRequiresLatestMigration(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, RunWithoutContract(db))

	path := filepath.Join(t.TempDir(), "schema_snapshot.sql")
	err := WriteCurrentSchemaSnapshot(db, path)
	require.ErrorContains(t, err, "database must be at latest migration")
	require.NoFileExists(t, path)
}

func TestWriteCurrentSchemaSnapshotWritesWhenDatabaseIsCurrent(t *testing.T) {
	db := newTestDB(t)
	t.Setenv(contractGuardEnvVar, "true")
	require.NoError(t, Run(db))

	path := filepath.Join(t.TempDir(), "schema_snapshot.sql")
	require.NoError(t, WriteCurrentSchemaSnapshot(db, path))
	require.FileExists(t, path)
}

func TestGenerateStub(t *testing.T) {
	name, content, err := GenerateStub(time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC), "add_order_notes")
	require.NoError(t, err)
	require.Equal(t, "2026030501_add_order_notes.go", name)
	require.Contains(t, content, `Version: "2026030501_add_order_notes"`)
}

func TestWriteStub(t *testing.T) {
	originalStubDir := stubDir
	defer func() {
		stubDir = originalStubDir
	}()

	tempDir := t.TempDir()
	stubDir = filepath.Join(tempDir, "stubs")

	path, err := WriteStub(time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC), "add_customer_flags")
	require.NoError(t, err)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(content), "add_customer_flags")
}
