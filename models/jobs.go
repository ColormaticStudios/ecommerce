package models

import "time"

const (
	JobStatusPending        = "pending"
	JobStatusRunning        = "running"
	JobStatusRetryScheduled = "retry_scheduled"
	JobStatusSucceeded      = "succeeded"
	JobStatusDeadLetter     = "dead_letter"

	JobAttemptOutcomeSucceeded    = "succeeded"
	JobAttemptOutcomeRetryable    = "retryable_failure"
	JobAttemptOutcomeTerminal     = "terminal_failure"
	JobAttemptOutcomeDegraded     = "degraded"
	JobAttemptOutcomeLeaseExpired = "lease_expired"
)

// JobQueue is the durable lifecycle record for one background job.
type JobQueue struct {
	ID                 string     `gorm:"primaryKey;size:36"`
	CreatedAt          time.Time  `gorm:"not null"`
	UpdatedAt          time.Time  `gorm:"not null"`
	JobType            string     `gorm:"not null;size:128;index:idx_job_queue_type_status,priority:1;index:idx_job_queue_type_idempotency,unique,priority:1"`
	PayloadJSON        string     `gorm:"type:text;not null"`
	PayloadFingerprint string     `gorm:"not null;size:64"`
	Status             string     `gorm:"not null;size:32;index:idx_job_queue_status_run_at,priority:1;index:idx_job_queue_type_status,priority:2"`
	RunAt              time.Time  `gorm:"not null;index:idx_job_queue_status_run_at,priority:2"`
	IdempotencyKey     *string    `gorm:"size:255;index:idx_job_queue_type_idempotency,unique,priority:2"`
	CorrelationID      string     `gorm:"not null;size:128;default:'';index"`
	AttemptCount       int        `gorm:"not null;default:0"`
	MaxAttempts        int        `gorm:"not null"`
	LeaseOwner         string     `gorm:"not null;size:128;default:'';index"`
	LeaseExpiresAt     *time.Time `gorm:"index"`
	AttemptStartedAt   *time.Time
	LastError          string     `gorm:"type:text;not null;default:''"`
	CompletedAt        *time.Time `gorm:"index"`
}

func (JobQueue) TableName() string { return "job_queue" }

// JobAttempt is append-only after insertion and records one completed,
// canceled, or lease-expired execution attempt.
type JobAttempt struct {
	ID            uint      `gorm:"primaryKey"`
	CreatedAt     time.Time `gorm:"not null"`
	JobID         string    `gorm:"not null;size:36;index;index:idx_job_attempts_job_number,unique,priority:1"`
	JobType       string    `gorm:"not null;size:128;index"`
	AttemptNumber int       `gorm:"not null;index:idx_job_attempts_job_number,unique,priority:2"`
	WorkerID      string    `gorm:"not null;size:128"`
	Outcome       string    `gorm:"not null;size:32;index"`
	ErrorClass    string    `gorm:"not null;size:32;default:'';index"`
	ErrorMessage  string    `gorm:"type:text;not null;default:''"`
	StartedAt     time.Time `gorm:"not null"`
	FinishedAt    time.Time `gorm:"not null"`
	LatencyMs     int64     `gorm:"not null"`
}

func (JobAttempt) TableName() string { return "job_attempts" }

// JobDeadLetter records the terminal snapshot used by future operator replay
// tooling. A queue row can have at most one dead-letter record.
type JobDeadLetter struct {
	ID             uint       `gorm:"primaryKey"`
	CreatedAt      time.Time  `gorm:"not null"`
	JobID          string     `gorm:"not null;size:36;uniqueIndex"`
	JobType        string     `gorm:"not null;size:128;index"`
	PayloadJSON    string     `gorm:"type:text;not null"`
	CorrelationID  string     `gorm:"not null;size:128;default:'';index"`
	AttemptCount   int        `gorm:"not null"`
	ErrorClass     string     `gorm:"not null;size:32;index"`
	FailureReason  string     `gorm:"type:text;not null"`
	FailedAt       time.Time  `gorm:"not null;index"`
	ReplayCount    int        `gorm:"not null;default:0"`
	LastReplayedAt *time.Time `gorm:"index"`
	ReplayJobID    *string    `gorm:"size:36;index"`
}

func (JobDeadLetter) TableName() string { return "job_dead_letters" }
