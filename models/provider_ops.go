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
