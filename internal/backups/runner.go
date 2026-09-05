package backups

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"filippo.io/age"
	"github.com/google/uuid"
)

type Clock func() time.Time

type Service struct {
	store              Store
	dumper             Dumper
	restorer           Restorer
	verifier           TargetVerifier
	temporaryDirectory string
	recipient          age.Recipient
	identities         []age.Identity
	sourceFingerprint  string
	targetFingerprint  string
	now                Clock
}

type ServiceOptions struct {
	Store              Store
	Dumper             Dumper
	Restorer           Restorer
	Verifier           TargetVerifier
	TemporaryDirectory string
	AgeRecipient       string
	AgeIdentityFile    string
	SourceDatabaseURL  string
	RestoreDatabaseURL string
	Now                Clock
}

func NewService(options ServiceOptions) (*Service, error) {
	if options.Store == nil {
		return nil, errors.New("backup store is required")
	}
	if options.TemporaryDirectory == "" {
		options.TemporaryDirectory = os.TempDir()
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	service := &Service{
		store: options.Store, dumper: options.Dumper, restorer: options.Restorer, verifier: options.Verifier,
		temporaryDirectory: options.TemporaryDirectory, now: options.Now,
		sourceFingerprint: databaseFingerprint(options.SourceDatabaseURL),
		targetFingerprint: databaseFingerprint(options.RestoreDatabaseURL),
	}
	if options.AgeRecipient != "" {
		recipient, err := age.ParseX25519Recipient(strings.TrimSpace(options.AgeRecipient))
		if err != nil {
			return nil, fmt.Errorf("parse BACKUP_AGE_RECIPIENT: %w", err)
		}
		service.recipient = recipient
	}
	if options.AgeIdentityFile != "" {
		identityInfo, err := os.Stat(options.AgeIdentityFile)
		if err != nil {
			return nil, fmt.Errorf("stat BACKUP_AGE_IDENTITY_FILE: %w", err)
		}
		if identityInfo.Mode().Perm()&0o077 != 0 {
			return nil, errors.New("BACKUP_AGE_IDENTITY_FILE must not be readable or writable by group or others")
		}
		identityFile, err := os.Open(options.AgeIdentityFile)
		if err != nil {
			return nil, fmt.Errorf("open BACKUP_AGE_IDENTITY_FILE: %w", err)
		}
		identities, parseErr := age.ParseIdentities(identityFile)
		closeErr := identityFile.Close()
		if parseErr != nil {
			return nil, fmt.Errorf("parse BACKUP_AGE_IDENTITY_FILE: %w", parseErr)
		}
		if closeErr != nil {
			return nil, closeErr
		}
		service.identities = identities
	}
	return service, nil
}

func (s *Service) Backup(ctx context.Context) (Manifest, error) {
	startedAt := s.now().UTC()
	manifest := newManifest(OperationBackup, startedAt)
	if s.dumper == nil || s.recipient == nil {
		return s.fail(ctx, manifest, "configuration", errors.New("backup dumper and age recipient are required"))
	}
	file, err := os.CreateTemp(s.temporaryDirectory, "ecommerce-backup-*.dump.age")
	if err != nil {
		return s.fail(ctx, manifest, "temporary_file", err)
	}
	filename := file.Name()
	defer os.Remove(filename)
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return s.fail(ctx, manifest, "temporary_file", err)
	}
	encrypted, err := age.Encrypt(file, s.recipient)
	if err != nil {
		_ = file.Close()
		return s.fail(ctx, manifest, "encryption", err)
	}
	if err := s.dumper.Dump(ctx, encrypted); err != nil {
		_ = encrypted.Close()
		_ = file.Close()
		return s.fail(ctx, manifest, "pg_dump", err)
	}
	if err := encrypted.Close(); err != nil {
		_ = file.Close()
		return s.fail(ctx, manifest, "encryption", err)
	}
	if err := file.Close(); err != nil {
		return s.fail(ctx, manifest, "temporary_file", err)
	}
	checksum, size, err := fileChecksum(filename)
	if err != nil {
		return s.fail(ctx, manifest, "checksum", err)
	}
	manifest.ArtifactKey = path.Join("artifacts", manifest.RunID+".dump.age")
	manifest.ArtifactSHA256 = checksum
	manifest.ArtifactSizeBytes = size
	manifest.SourceDatabaseFingerprint = s.sourceFingerprint
	manifest.Encryption = "age-x25519"
	manifest.Format = "postgresql-custom"
	if err := s.store.PutFile(ctx, manifest.ArtifactKey, filename, "application/octet-stream"); err != nil {
		return s.fail(ctx, manifest, "artifact_upload", err)
	}
	manifest.Outcome = OutcomeSucceeded
	manifest.CompletedAt = s.now().UTC()
	manifest.DurationSeconds = manifest.CompletedAt.Sub(manifest.StartedAt).Seconds()
	if err := putManifest(ctx, s.store, backupManifestKey(manifest.RunID), manifest); err != nil {
		return s.fail(ctx, manifest, "manifest_upload", err)
	}
	if err := putManifest(ctx, s.store, "status/latest-successful-backup.json", manifest); err != nil {
		return manifest, fmt.Errorf("update latest successful backup status: %w", err)
	}
	if err := putManifest(ctx, s.store, "status/latest-backup.json", manifest); err != nil {
		return manifest, fmt.Errorf("update latest backup status: %w", err)
	}
	return manifest, nil
}

func (s *Service) RestoreDrill(ctx context.Context) (Manifest, error) {
	startedAt := s.now().UTC()
	manifest := newManifest(OperationRestoreDrill, startedAt)
	if s.restorer == nil || s.verifier == nil || len(s.identities) == 0 {
		return s.failDrill(ctx, manifest, "configuration", errors.New("restore tool, verifier, and age identity are required"))
	}
	if err := s.verifier.EnsureEmpty(ctx); err != nil {
		return s.failDrill(ctx, manifest, "isolation_guard", err)
	}
	backup, err := latestSuccessfulManifest(ctx, s.store, OperationBackup)
	if err != nil {
		return s.failDrill(ctx, manifest, "backup_selection", err)
	}
	if err := validateBackupArtifact(backup); err != nil {
		return s.failDrill(ctx, manifest, "backup_selection", err)
	}
	if backup.SourceDatabaseFingerprint != "" && backup.SourceDatabaseFingerprint == s.targetFingerprint {
		return s.failDrill(ctx, manifest, "isolation_guard", errors.New("restore target resolves to the protected source database"))
	}
	file, err := os.CreateTemp(s.temporaryDirectory, "ecommerce-restore-*.dump.age")
	if err != nil {
		return s.failDrill(ctx, manifest, "temporary_file", err)
	}
	filename := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(filename)
		return s.failDrill(ctx, manifest, "temporary_file", err)
	}
	defer os.Remove(filename)
	if err := s.store.GetFile(ctx, backup.ArtifactKey, filename); err != nil {
		return s.failDrill(ctx, manifest, "artifact_download", err)
	}
	checksum, _, err := fileChecksum(filename)
	if err != nil || checksum != backup.ArtifactSHA256 {
		if err == nil {
			err = errors.New("encrypted artifact checksum mismatch")
		}
		return s.failDrill(ctx, manifest, "checksum", err)
	}
	encrypted, err := os.Open(filename)
	if err != nil {
		return s.failDrill(ctx, manifest, "temporary_file", err)
	}
	decrypted, err := age.Decrypt(encrypted, s.identities...)
	if err != nil {
		_ = encrypted.Close()
		return s.failDrill(ctx, manifest, "decryption", err)
	}
	restoreStartedAt := s.now().UTC()
	if err := s.restorer.Restore(ctx, decrypted); err != nil {
		_ = encrypted.Close()
		return s.failDrill(ctx, manifest, "pg_restore", err)
	}
	if err := encrypted.Close(); err != nil {
		return s.failDrill(ctx, manifest, "temporary_file", err)
	}
	if err := s.verifier.Verify(ctx); err != nil {
		return s.failDrill(ctx, manifest, "verification", err)
	}
	completedAt := s.now().UTC()
	manifest.Outcome = OutcomeSucceeded
	manifest.CompletedAt = completedAt
	manifest.DurationSeconds = completedAt.Sub(startedAt).Seconds()
	manifest.BackupCompletedAt = backup.CompletedAt
	manifest.ArtifactKey = backup.ArtifactKey
	manifest.ArtifactSHA256 = backup.ArtifactSHA256
	manifest.RPOSeconds = maxSeconds(0, startedAt.Sub(backup.CompletedAt).Seconds())
	manifest.RTOSeconds = completedAt.Sub(restoreStartedAt).Seconds()
	manifest.SchemaVerified = true
	manifest.ReadWriteVerified = true
	if err := putManifest(ctx, s.store, drillManifestKey(manifest.RunID), manifest); err != nil {
		return manifest, fmt.Errorf("record restore drill manifest: %w", err)
	}
	if err := putManifest(ctx, s.store, "status/latest-successful-restore-drill.json", manifest); err != nil {
		return manifest, fmt.Errorf("update latest successful restore drill status: %w", err)
	}
	if err := putManifest(ctx, s.store, "status/latest-restore-drill.json", manifest); err != nil {
		return manifest, fmt.Errorf("update latest restore drill status: %w", err)
	}
	return manifest, nil
}

func (s *Service) fail(ctx context.Context, manifest Manifest, stage string, cause error) (Manifest, error) {
	manifest.Outcome = OutcomeFailed
	manifest.FailureStage = stage
	manifest.CompletedAt = s.now().UTC()
	manifest.DurationSeconds = manifest.CompletedAt.Sub(manifest.StartedAt).Seconds()
	recordErr := putManifest(ctx, s.store, backupManifestKey(manifest.RunID), manifest)
	_ = putManifest(ctx, s.store, "status/latest-backup.json", manifest)
	return manifest, errors.Join(fmt.Errorf("backup %s: %w", stage, cause), recordErr)
}

func (s *Service) failDrill(ctx context.Context, manifest Manifest, stage string, cause error) (Manifest, error) {
	manifest.Outcome = OutcomeFailed
	manifest.FailureStage = stage
	manifest.CompletedAt = s.now().UTC()
	manifest.DurationSeconds = manifest.CompletedAt.Sub(manifest.StartedAt).Seconds()
	recordErr := putManifest(ctx, s.store, drillManifestKey(manifest.RunID), manifest)
	_ = putManifest(ctx, s.store, "status/latest-restore-drill.json", manifest)
	return manifest, errors.Join(fmt.Errorf("restore drill %s: %w", stage, cause), recordErr)
}

func newManifest(operation string, startedAt time.Time) Manifest {
	return Manifest{SchemaVersion: ManifestSchemaVersion, RunID: startedAt.Format("20060102T150405.000000000Z") + "-" + uuid.NewString(), Operation: operation, StartedAt: startedAt}
}

func backupManifestKey(runID string) string { return path.Join("runs/backups", runID+".json") }
func drillManifestKey(runID string) string  { return path.Join("runs/restore-drills", runID+".json") }

func fileChecksum(filename string) (string, int64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func databaseFingerprint(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" {
		return ""
	}
	port := parsed.Port()
	if port == "" {
		port = "5432"
	}
	canonical := strings.ToLower(parsed.Hostname()) + ":" + port + "/" + strings.TrimPrefix(parsed.Path, "/")
	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

func maxSeconds(left, right float64) float64 {
	if right > left {
		return right
	}
	return left
}

func validateBackupArtifact(manifest Manifest) error {
	if strings.ContainsAny(manifest.RunID, `/\`) || manifest.ArtifactKey != path.Join("artifacts", manifest.RunID+".dump.age") {
		return errors.New("successful backup manifest has an invalid artifact key")
	}
	checksum, err := hex.DecodeString(manifest.ArtifactSHA256)
	if err != nil || len(checksum) != sha256.Size {
		return errors.New("successful backup manifest has an invalid SHA-256 checksum")
	}
	if manifest.Encryption != "age-x25519" || manifest.Format != "postgresql-custom" {
		return errors.New("successful backup manifest uses an unsupported encryption or dump format")
	}
	return nil
}
