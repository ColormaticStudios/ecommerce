package models

import "time"

const (
	ProviderTypePayment  = "payment"
	ProviderTypeShipping = "shipping"
	ProviderTypeTax      = "tax"
)

const (
	ProviderEnvironmentSandbox    = "sandbox"
	ProviderEnvironmentProduction = "production"
)

const (
	ProviderFXModeSameCurrencyOnly = "same_currency_only"
	ProviderFXModeProviderManaged  = "provider_managed"
)

const (
	ProviderCallStatusSucceeded = "SUCCEEDED"
	ProviderCallStatusFailed    = "FAILED"
)

const (
	ProviderReconciliationStatusSucceeded  = "SUCCEEDED"
	ProviderReconciliationStatusFailed     = "FAILED"
	ProviderReconciliationTriggerManual    = "MANUAL"
	ProviderReconciliationTriggerScheduled = "SCHEDULED"
)

const (
	ProviderDriftSeverityError = "ERROR"
	ProviderDriftSeverityWarn  = "WARN"
)

const (
	ProviderOperationStatusPrepared               = "PREPARED"
	ProviderOperationStatusExecuting              = "EXECUTING"
	ProviderOperationStatusOutcomeUnknown         = "OUTCOME_UNKNOWN"
	ProviderOperationStatusProviderSucceeded      = "PROVIDER_SUCCEEDED"
	ProviderOperationStatusFinalizing             = "FINALIZING"
	ProviderOperationStatusFinalizeRetry          = "FINALIZE_RETRY"
	ProviderOperationStatusCompensationPrepared   = "COMPENSATION_PREPARED"
	ProviderOperationStatusCompensating           = "COMPENSATING"
	ProviderOperationStatusCompensationSucceeded  = "COMPENSATION_SUCCEEDED"
	ProviderOperationStatusCompensationRetry      = "COMPENSATION_RETRY"
	ProviderOperationStatusCompleted              = "COMPLETED"
	ProviderOperationStatusFailed                 = "FAILED"
	ProviderOperationStatusReconciling            = "RECONCILING"
	ProviderOperationStatusReconciliationRequired = "RECONCILIATION_REQUIRED"

	// Deprecated compatibility aliases. New code should use the explicit
	// execution and outcome states above.
	ProviderOperationStatusInProgress = ProviderOperationStatusExecuting
	ProviderOperationStatusSucceeded  = ProviderOperationStatusCompleted
	ProviderOperationStatusUnknown    = ProviderOperationStatusOutcomeUnknown
)

const (
	ProviderOperationAttemptOutcomeSucceeded = "SUCCEEDED"
	ProviderOperationAttemptOutcomeFailed    = "FAILED"
	ProviderOperationAttemptOutcomeUnknown   = "UNKNOWN"
)

const (
	ProviderOutcomeSucceeded = "SUCCEEDED"
	ProviderOutcomeFailed    = "FAILED"
	ProviderOutcomeUnknown   = "UNKNOWN"
	ProviderOutcomeNotFound  = "NOT_FOUND"
)

const (
	ProviderReconciliationCaseStatusOpen     = "OPEN"
	ProviderReconciliationCaseStatusResolved = "RESOLVED"
)

const (
	ProviderReconciliationCaseOutcomeConfirmedSucceeded = "CONFIRMED_SUCCEEDED"
	ProviderReconciliationCaseOutcomeConfirmedFailed    = "CONFIRMED_FAILED"
	ProviderReconciliationCaseOutcomeRetryRequired      = "RETRY_REQUIRED"
	ProviderReconciliationCaseOutcomeManualReview       = "MANUAL_REVIEW"
)

type ProviderCredential struct {
	ID                 uint `gorm:"primaryKey"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
	ProviderType       string    `gorm:"not null;index:idx_provider_credentials_scope,unique"`
	ProviderID         string    `gorm:"not null;index:idx_provider_credentials_scope,unique"`
	Environment        string    `gorm:"not null;index:idx_provider_credentials_scope,unique"`
	Label              string    `gorm:"not null;default:''"`
	SecretEnvelopeJSON string    `gorm:"type:text;not null;default:''"`
	KeyVersion         string    `gorm:"not null;default:'';index"`
	MetadataJSON       string    `gorm:"type:text;not null;default:''"`
	LastRotatedAt      time.Time `gorm:"not null;index"`
}

// ProviderOperation is the durable identity and lifecycle record for one
// idempotent provider-side mutation. Existing provider audit rows remain a
// separate observational log.
type ProviderOperation struct {
	ID                  uint `gorm:"primaryKey"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
	OperationKey        string                       `gorm:"not null;size:128;uniqueIndex"`
	ParentOperationID   *uint                        `gorm:"index"`
	ProviderType        string                       `gorm:"not null;index:idx_provider_operations_idempotency,unique;index:idx_provider_operations_scope_status"`
	ProviderID          string                       `gorm:"not null;index:idx_provider_operations_idempotency,unique;index:idx_provider_operations_scope_status"`
	Environment         string                       `gorm:"not null;index:idx_provider_operations_idempotency,unique;index:idx_provider_operations_scope_status"`
	Operation           string                       `gorm:"not null;index:idx_provider_operations_idempotency,unique;index:idx_provider_operations_scope_status"`
	IdempotencyKey      string                       `gorm:"not null;index:idx_provider_operations_idempotency,unique"`
	RequestFingerprint  string                       `gorm:"not null;size:64"`
	CorrelationID       string                       `gorm:"not null;default:'';index"`
	EntityType          string                       `gorm:"not null;default:'';index:idx_provider_operations_entity"`
	EntityID            uint                         `gorm:"not null;default:0;index:idx_provider_operations_entity"`
	Status              string                       `gorm:"not null;index;index:idx_provider_operations_scope_status"`
	ProviderOutcome     string                       `gorm:"not null;default:'';index"`
	ProviderReference   string                       `gorm:"not null;default:'';index"`
	RequestJSON         string                       `gorm:"type:text;not null;default:'{}'"`
	ResultJSON          string                       `gorm:"type:text;not null;default:'{}'"`
	MetadataJSON        string                       `gorm:"type:text;not null;default:'{}'"`
	LastError           string                       `gorm:"type:text;not null;default:''"`
	LeaseOwner          string                       `gorm:"not null;default:'';index"`
	LeaseExpiresAt      *time.Time                   `gorm:"index"`
	NextAttemptAt       *time.Time                   `gorm:"index"`
	Version             uint                         `gorm:"not null;default:1"`
	CompletedAt         *time.Time                   `gorm:"index"`
	Attempts            []ProviderOperationAttempt   `gorm:"foreignKey:ProviderOperationID"`
	ReconciliationCases []ProviderReconciliationCase `gorm:"foreignKey:ProviderOperationID"`
}

type ProviderOperationAttempt struct {
	ID                      uint `gorm:"primaryKey"`
	CreatedAt               time.Time
	UpdatedAt               time.Time
	ProviderOperationID     uint   `gorm:"not null;index:idx_provider_operation_attempt_number,unique;index"`
	AttemptNumber           int    `gorm:"not null;index:idx_provider_operation_attempt_number,unique"`
	Phase                   string `gorm:"not null;default:'provider';index"`
	Outcome                 string `gorm:"not null;index"`
	ProviderOutcome         string `gorm:"not null;default:'';index"`
	ProviderReference       string `gorm:"not null;default:'';index"`
	OperationKey            string `gorm:"not null;default:'';index"`
	ResultJSON              string `gorm:"type:text;not null;default:'{}'"`
	Retryable               bool   `gorm:"not null;default:false"`
	RequestPayloadRedacted  string `gorm:"type:text;not null;default:''"`
	ResponsePayloadRedacted string `gorm:"type:text;not null;default:''"`
	ErrorMessage            string `gorm:"type:text;not null;default:''"`
	StartedAt               time.Time
	FinishedAt              time.Time
}

type ProviderReconciliationCase struct {
	ID                  uint `gorm:"primaryKey"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
	ProviderOperationID uint       `gorm:"not null;index;index:idx_provider_reconciliation_cases_open,unique"`
	AttemptID           *uint      `gorm:"index"`
	OpenKey             *string    `gorm:"size:16;index:idx_provider_reconciliation_cases_open,unique"`
	Status              string     `gorm:"not null;index"`
	Outcome             string     `gorm:"not null;default:'';index"`
	Reason              string     `gorm:"type:text;not null;default:''"`
	CaseType            string     `gorm:"not null;default:'ambiguous_outcome';index"`
	ProviderOutcome     string     `gorm:"not null;default:'';index"`
	OperationKey        string     `gorm:"not null;default:'';index"`
	DetailsJSON         string     `gorm:"type:text;not null;default:'{}'"`
	ResolutionJSON      string     `gorm:"type:text;not null;default:'{}'"`
	NextAttemptAt       *time.Time `gorm:"index"`
	OpenedAt            time.Time
	ResolvedAt          *time.Time `gorm:"index"`
}

type ProviderCallAudit struct {
	ID                      uint `gorm:"primaryKey"`
	CreatedAt               time.Time
	UpdatedAt               time.Time
	ProviderType            string `gorm:"not null;index"`
	ProviderID              string `gorm:"not null;index"`
	Environment             string `gorm:"not null;index"`
	Operation               string `gorm:"not null;index"`
	CorrelationID           string `gorm:"not null;default:'';index"`
	IdempotencyKey          string `gorm:"not null;default:'';index"`
	Status                  string `gorm:"not null;index"`
	RequestPayloadRedacted  string `gorm:"type:text;not null;default:''"`
	ResponsePayloadRedacted string `gorm:"type:text;not null;default:''"`
	ErrorMessage            string `gorm:"type:text;not null;default:''"`
	LatencyMs               int64  `gorm:"not null;default:0"`
}

type ProviderReconciliationRun struct {
	ID           uint `gorm:"primaryKey"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ProviderType string `gorm:"not null;index"`
	ProviderID   string `gorm:"not null;index"`
	Environment  string `gorm:"not null;index"`
	Trigger      string `gorm:"not null;index"`
	Status       string `gorm:"not null;index"`
	CheckedCount int    `gorm:"not null;default:0"`
	DriftCount   int    `gorm:"not null;default:0"`
	ErrorCount   int    `gorm:"not null;default:0"`
	StartedAt    time.Time
	FinishedAt   *time.Time                    `gorm:"index"`
	SummaryJSON  string                        `gorm:"type:text;not null;default:''"`
	Drifts       []ProviderReconciliationDrift `gorm:"foreignKey:RunID"`
}

type ProviderReconciliationDrift struct {
	ID                uint `gorm:"primaryKey"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
	RunID             uint   `gorm:"not null;index"`
	EntityType        string `gorm:"not null;index"`
	EntityID          uint   `gorm:"not null;default:0;index"`
	ProviderReference string `gorm:"not null;default:'';index"`
	Severity          string `gorm:"not null;index"`
	FieldName         string `gorm:"not null;default:''"`
	ExpectedValue     string `gorm:"type:text;not null;default:''"`
	ActualValue       string `gorm:"type:text;not null;default:''"`
	Message           string `gorm:"type:text;not null;default:''"`
}
