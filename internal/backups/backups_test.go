package backups

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"filippo.io/age"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memoryStore struct {
	mu      sync.Mutex
	objects map[string][]byte
	putErr  error
}

func newMemoryStore() *memoryStore { return &memoryStore{objects: make(map[string][]byte)} }

func (s *memoryStore) PutFile(_ context.Context, key, filename, _ string) error {
	value, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return s.PutBytes(context.Background(), key, value, "")
}

func (s *memoryStore) PutBytes(_ context.Context, key string, value []byte, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.putErr != nil {
		return s.putErr
	}
	s.objects[key] = append([]byte(nil), value...)
	return nil
}

func (s *memoryStore) GetFile(_ context.Context, key, filename string) error {
	s.mu.Lock()
	value, exists := s.objects[key]
	s.mu.Unlock()
	if !exists {
		return os.ErrNotExist
	}
	return os.WriteFile(filename, value, 0o600)
}

func (s *memoryStore) GetBytes(_ context.Context, key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, exists := s.objects[key]
	if !exists {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), value...), nil
}

func (s *memoryStore) List(_ context.Context, prefix string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var keys []string
	for key := range s.objects {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

type dumpFunc func(context.Context, io.Writer) error

func (f dumpFunc) Dump(ctx context.Context, writer io.Writer) error { return f(ctx, writer) }

type restoreRecorder struct{ value []byte }

func (r *restoreRecorder) Restore(_ context.Context, reader io.Reader) error {
	value, err := io.ReadAll(reader)
	r.value = value
	return err
}

type verifierRecorder struct {
	emptyErr  error
	verifyErr error
	empty     int
	verified  int
}

func (v *verifierRecorder) EnsureEmpty(context.Context) error { v.empty++; return v.emptyErr }
func (v *verifierRecorder) Verify(context.Context) error      { v.verified++; return v.verifyErr }
func (v *verifierRecorder) Close() error                      { return nil }

func incrementingClock(start time.Time) Clock {
	var mu sync.Mutex
	current := start
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		value := current
		current = current.Add(time.Second)
		return value
	}
}

func TestBackupEncryptsArtifactAndPublishesManifests(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	store := newMemoryStore()
	service, err := NewService(ServiceOptions{
		Store: store, TemporaryDirectory: t.TempDir(), AgeRecipient: identity.Recipient().String(),
		SourceDatabaseURL: "postgresql://secret@db.example:5432/ecommerce?sslmode=require",
		Dumper: dumpFunc(func(_ context.Context, writer io.Writer) error {
			_, err := io.WriteString(writer, "custom-format-dump")
			return err
		}),
		Now: incrementingClock(time.Date(2026, 9, 5, 2, 0, 0, 0, time.UTC)),
	})
	require.NoError(t, err)
	manifest, err := service.Backup(context.Background())
	require.NoError(t, err)
	assert.Equal(t, OutcomeSucceeded, manifest.Outcome)
	assert.Equal(t, "age-x25519", manifest.Encryption)
	assert.NotEmpty(t, manifest.ArtifactSHA256)
	assert.NotContains(t, manifest.SourceDatabaseFingerprint, "secret")

	encrypted := store.objects[manifest.ArtifactKey]
	assert.NotContains(t, string(encrypted), "custom-format-dump")
	reader, err := age.Decrypt(bytes.NewReader(encrypted), identity)
	require.NoError(t, err)
	decrypted, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "custom-format-dump", string(decrypted))
	assert.Contains(t, store.objects, backupManifestKey(manifest.RunID))
	assert.Contains(t, store.objects, "status/latest-backup.json")
	assert.Contains(t, store.objects, "status/latest-successful-backup.json")
}

func TestBackupFailureRecordsBoundedFailureManifest(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	store := newMemoryStore()
	service, err := NewService(ServiceOptions{
		Store: store, TemporaryDirectory: t.TempDir(), AgeRecipient: identity.Recipient().String(),
		Dumper: dumpFunc(func(context.Context, io.Writer) error { return errors.New("database password=secret") }),
	})
	require.NoError(t, err)
	manifest, err := service.Backup(context.Background())
	require.Error(t, err)
	assert.Equal(t, OutcomeFailed, manifest.Outcome)
	assert.Equal(t, "pg_dump", manifest.FailureStage)
	encoded := store.objects[backupManifestKey(manifest.RunID)]
	assert.NotContains(t, string(encoded), "password")
	assert.NotContains(t, string(encoded), "secret")
}

func TestRestoreDrillDecryptsLatestBackupAndVerifiesTarget(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	identityFile := filepath.Join(t.TempDir(), "identity.txt")
	require.NoError(t, os.WriteFile(identityFile, []byte(identity.String()+"\n"), 0o600))
	store := newMemoryStore()
	clock := incrementingClock(time.Date(2026, 9, 5, 2, 0, 0, 0, time.UTC))
	backupService, err := NewService(ServiceOptions{
		Store: store, TemporaryDirectory: t.TempDir(), AgeRecipient: identity.Recipient().String(),
		SourceDatabaseURL: "postgresql://backup@source.example:5432/ecommerce",
		Dumper: dumpFunc(func(_ context.Context, writer io.Writer) error {
			_, err := io.WriteString(writer, "restorable")
			return err
		}),
		Now: clock,
	})
	require.NoError(t, err)
	_, err = backupService.Backup(context.Background())
	require.NoError(t, err)

	restorer := &restoreRecorder{}
	verifier := &verifierRecorder{}
	restoreService, err := NewService(ServiceOptions{
		Store: store, TemporaryDirectory: t.TempDir(), AgeIdentityFile: identityFile,
		RestoreDatabaseURL: "postgresql://restore@target.example:5432/ecommerce_restore",
		Restorer:           restorer, Verifier: verifier, Now: clock,
	})
	require.NoError(t, err)
	manifest, err := restoreService.RestoreDrill(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []byte("restorable"), restorer.value)
	assert.Equal(t, 1, verifier.empty)
	assert.Equal(t, 1, verifier.verified)
	assert.True(t, manifest.SchemaVerified)
	assert.True(t, manifest.ReadWriteVerified)
	assert.GreaterOrEqual(t, manifest.RPOSeconds, float64(0))
	assert.Contains(t, store.objects, "status/latest-successful-restore-drill.json")
}

func TestRestoreDrillRefusesSourceDatabaseAndNonEmptyTarget(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	identityFile := filepath.Join(t.TempDir(), "identity.txt")
	require.NoError(t, os.WriteFile(identityFile, []byte(identity.String()+"\n"), 0o600))
	store := newMemoryStore()
	completed := time.Now().UTC()
	backup := Manifest{
		SchemaVersion: ManifestSchemaVersion, RunID: "backup", Operation: OperationBackup, Outcome: OutcomeSucceeded,
		CompletedAt: completed, ArtifactKey: "artifacts/backup.dump.age",
		ArtifactSHA256: strings.Repeat("a", 64), Encryption: "age-x25519", Format: "postgresql-custom",
		SourceDatabaseFingerprint: databaseFingerprint("postgresql://source:secret@db.example:5432/ecommerce"),
	}
	require.NoError(t, putManifest(context.Background(), store, "status/latest-successful-backup.json", backup))
	verifier := &verifierRecorder{}
	service, err := NewService(ServiceOptions{
		Store: store, TemporaryDirectory: t.TempDir(), AgeIdentityFile: identityFile,
		RestoreDatabaseURL: "postgresql://restore:different@db.example:5432/ecommerce",
		Restorer:           &restoreRecorder{}, Verifier: verifier,
	})
	require.NoError(t, err)
	manifest, err := service.RestoreDrill(context.Background())
	require.Error(t, err)
	assert.Equal(t, "isolation_guard", manifest.FailureStage)
	assert.Zero(t, verifier.verified)

	verifier.emptyErr = errors.New("target has tables")
	service.targetFingerprint = databaseFingerprint("postgresql://restore@other.example:5432/ecommerce_restore")
	manifest, err = service.RestoreDrill(context.Background())
	require.Error(t, err)
	assert.Equal(t, "isolation_guard", manifest.FailureStage)
}

func TestManifestMetricsExposeFreshnessFailureAndDrillMeasurements(t *testing.T) {
	store := newMemoryStore()
	now := time.Now().UTC().Truncate(time.Second)
	backup := Manifest{SchemaVersion: 1, RunID: "failed-backup", Operation: OperationBackup, Outcome: OutcomeFailed, FailureStage: "pg_dump", CompletedAt: now}
	success := Manifest{SchemaVersion: 1, RunID: "successful-backup", Operation: OperationBackup, Outcome: OutcomeSucceeded, CompletedAt: now.Add(-time.Hour)}
	drill := Manifest{SchemaVersion: 1, RunID: "successful-drill", Operation: OperationRestoreDrill, Outcome: OutcomeSucceeded, CompletedAt: now, RPOSeconds: 90, RTOSeconds: 30}
	for key, value := range map[string]Manifest{
		"status/latest-backup.json": backup, "status/latest-successful-backup.json": success,
		"status/latest-restore-drill.json": drill, "status/latest-successful-restore-drill.json": drill,
	} {
		require.NoError(t, putManifest(context.Background(), store, key, value))
	}
	response := httptest.NewRecorder()
	NewMetricsHandler(store, "backup", "test", "on-call").ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := response.Body.String()
	assert.Contains(t, body, `ecommerce_backup_last_run_timestamp_seconds{deployment_environment="test",failure_stage="pg_dump",outcome="failed",owner="on-call",service="backup"}`)
	assert.Contains(t, body, `ecommerce_backup_manifest_collection_success{deployment_environment="test",owner="on-call",service="backup"} 1`)
	assert.Contains(t, body, `ecommerce_restore_drill_rpo_seconds{deployment_environment="test",owner="on-call",service="backup"} 90`)
}

func TestConfigValidationAndSchedule(t *testing.T) {
	config := Config{Bucket: "bucket", Prefix: "backups", Region: "region", TemporaryDirectory: t.TempDir(), MetricsBindAddress: "127.0.0.1:9091", ScheduleUTC: "02:00", Service: "backup", Environment: "test", Owner: "owner"}
	require.NoError(t, config.ValidateBase())
	config.Prefix = "../escape"
	require.Error(t, config.ValidateBase())
	schedule, err := ParseScheduleUTC("23:45")
	require.NoError(t, err)
	assert.Equal(t, 23*time.Hour+45*time.Minute, schedule)
	_, err = ParseScheduleUTC("24:00")
	require.Error(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = RunDaily(ctx, 2*time.Hour, func() time.Time { return time.Date(2026, 9, 5, 1, 0, 0, 0, time.UTC) }, func(context.Context) error {
		t.Fatal("cancelled scheduler must not start a backup")
		return nil
	})
	require.ErrorIs(t, err, context.Canceled)
}

func TestManifestsNeverSerializeUnknownFields(t *testing.T) {
	encoded, err := json.Marshal(Manifest{SchemaVersion: 1, Outcome: OutcomeFailed, FailureStage: "pg_dump"})
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "error")
}

func TestPGToolsStreamsDumpWithoutExposingDatabaseURLInArguments(t *testing.T) {
	directory := t.TempDir()
	dumpPath := filepath.Join(directory, "pg_dump")
	require.NoError(t, os.WriteFile(dumpPath, []byte(`#!/bin/sh
test "$PGHOST" = "source.example" || exit 10
test "$PGPORT" = "5432" || exit 11
test "$PGDATABASE" = "ecommerce" || exit 12
test "$PGUSER" = "backup" || exit 13
test "$PGPASSWORD" = "secret" || exit 14
test "$PGSSLMODE" = "require" || exit 15
test "$#" -eq 3 || exit 11
test "$1" = "--format=custom" || exit 12
test "$2" = "--no-owner" || exit 13
test "$3" = "--no-acl" || exit 14
printf 'custom-dump'
`), 0o700))

	var destination bytes.Buffer
	err := (PGTools{
		DatabaseURL: "postgresql://backup:secret@source.example/ecommerce?sslmode=require",
		DumpPath:    dumpPath,
	}).Dump(context.Background(), &destination)
	require.NoError(t, err)
	assert.Equal(t, "custom-dump", destination.String())
}

func TestPGToolsStreamsRestoreIntoConfiguredDatabase(t *testing.T) {
	directory := t.TempDir()
	restorePath := filepath.Join(directory, "pg_restore")
	restoredPath := filepath.Join(directory, "restored.dump")
	require.NoError(t, os.WriteFile(restorePath, []byte(`#!/bin/sh
test "$PGHOST" = "target.example" || exit 10
test "$PGPORT" = "5432" || exit 11
test "$PGDATABASE" = "ecommerce" || exit 12
test "$PGUSER" = "restore" || exit 13
test "$PGPASSWORD" = "secret" || exit 14
test "$#" -eq 4 || exit 11
test "$1" = "--exit-on-error" || exit 12
test "$2" = "--no-owner" || exit 13
test "$3" = "--no-acl" || exit 14
test "$4" = "--dbname=" || exit 15
cat > "$RESTORED_PATH"
`), 0o700))

	t.Setenv("RESTORED_PATH", restoredPath)
	err := (PGTools{
		RestoreDatabaseURL: "postgresql://restore:secret@target.example/ecommerce",
		RestorePath:        restorePath,
	}).Restore(context.Background(), strings.NewReader("custom-dump"))
	require.NoError(t, err)
	restored, err := os.ReadFile(restoredPath)
	require.NoError(t, err)
	assert.Equal(t, "custom-dump", string(restored))
}

func TestDatabaseEnvironmentRejectsUnsupportedParameters(t *testing.T) {
	_, err := databaseEnvironment("postgresql://backup@source.example/ecommerce?unknown=value")
	require.ErrorContains(t, err, `unsupported or repeated PostgreSQL URL parameter "unknown"`)
}

func TestServiceRejectsOverexposedAgeIdentity(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	identityFile := filepath.Join(t.TempDir(), "identity.txt")
	require.NoError(t, os.WriteFile(identityFile, []byte(identity.String()+"\n"), 0o644))

	_, err = NewService(ServiceOptions{Store: newMemoryStore(), AgeIdentityFile: identityFile})
	require.ErrorContains(t, err, "must not be readable or writable by group or others")
}

func TestStatusManifestRejectsUnboundedLabelsAndUnsafeArtifactKeys(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: 1, RunID: "run", Operation: OperationBackup, Outcome: OutcomeFailed,
		FailureStage: "attacker-controlled", CompletedAt: time.Now().UTC(),
	}
	require.ErrorContains(t, validateStatusManifest(manifest), "unsupported failure stage")

	manifest.Outcome = OutcomeSucceeded
	manifest.FailureStage = ""
	manifest.ArtifactKey = "../unrelated-object"
	manifest.ArtifactSHA256 = strings.Repeat("a", 64)
	manifest.Encryption = "age-x25519"
	manifest.Format = "postgresql-custom"
	require.ErrorContains(t, validateBackupArtifact(manifest), "invalid artifact key")
}
