package backups

import "time"

const ManifestSchemaVersion = 1

const (
	OperationBackup       = "backup"
	OperationRestoreDrill = "restore_drill"
	OutcomeSucceeded      = "succeeded"
	OutcomeFailed         = "failed"
)

type Manifest struct {
	SchemaVersion             int       `json:"schema_version"`
	RunID                     string    `json:"run_id"`
	Operation                 string    `json:"operation"`
	Outcome                   string    `json:"outcome"`
	FailureStage              string    `json:"failure_stage,omitempty"`
	StartedAt                 time.Time `json:"started_at"`
	CompletedAt               time.Time `json:"completed_at"`
	DurationSeconds           float64   `json:"duration_seconds"`
	ArtifactKey               string    `json:"artifact_key,omitempty"`
	ArtifactSHA256            string    `json:"artifact_sha256,omitempty"`
	ArtifactSizeBytes         int64     `json:"artifact_size_bytes,omitempty"`
	BackupCompletedAt         time.Time `json:"backup_completed_at,omitempty"`
	SourceDatabaseFingerprint string    `json:"source_database_fingerprint,omitempty"`
	Encryption                string    `json:"encryption,omitempty"`
	Format                    string    `json:"format,omitempty"`
	RPOSeconds                float64   `json:"rpo_seconds,omitempty"`
	RTOSeconds                float64   `json:"rto_seconds,omitempty"`
	SchemaVerified            bool      `json:"schema_verified,omitempty"`
	ReadWriteVerified         bool      `json:"read_write_verified,omitempty"`
}
