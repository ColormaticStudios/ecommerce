package providerops

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"ecommerce/internal/dbcontext"
	"ecommerce/models"

	"gorm.io/gorm"
)

type AuditService struct {
	db *gorm.DB
}

type AuditRecord struct {
	ProviderType    string
	ProviderID      string
	Environment     string
	Operation       string
	CorrelationID   string
	IdempotencyKey  string
	Status          string
	RequestPayload  any
	ResponsePayload any
	ErrorMessage    string
	Latency         time.Duration
}

func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
}

func (s *AuditService) Record(ctx context.Context, record AuditRecord) error {
	if s == nil {
		return nil
	}

	db := s.db
	if tx := dbcontext.GetDB(ctx); tx != nil {
		db = tx
	}
	if db == nil || !db.Migrator().HasTable(&models.ProviderCallAudit{}) {
		return nil
	}

	return db.WithContext(dbcontext.OrBackground(ctx)).Create(&models.ProviderCallAudit{
		ProviderType:            strings.TrimSpace(record.ProviderType),
		ProviderID:              strings.TrimSpace(record.ProviderID),
		Environment:             strings.TrimSpace(record.Environment),
		Operation:               strings.TrimSpace(record.Operation),
		CorrelationID:           strings.TrimSpace(record.CorrelationID),
		IdempotencyKey:          strings.TrimSpace(record.IdempotencyKey),
		Status:                  strings.TrimSpace(record.Status),
		RequestPayloadRedacted:  marshalRedactedPayload(record.RequestPayload),
		ResponsePayloadRedacted: marshalRedactedPayload(record.ResponsePayload),
		ErrorMessage:            strings.TrimSpace(record.ErrorMessage),
		LatencyMs:               record.Latency.Milliseconds(),
	}).Error
}

func marshalRedactedPayload(payload any) string {
	if payload == nil {
		return ""
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}

	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return string(raw)
	}
	redactPayload(decoded)

	redacted, err := json.Marshal(decoded)
	if err != nil {
		return string(raw)
	}
	return string(redacted)
}

func redactPayload(value any) {
	switch typed := value.(type) {
	case map[string]any:
		for key, inner := range typed {
			if shouldRedactField(key) {
				typed[key] = "[REDACTED]"
				continue
			}
			redactPayload(inner)
		}
	case []any:
		for _, inner := range typed {
			redactPayload(inner)
		}
	}
}

func shouldRedactField(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "secret",
		"secret_data",
		"client_secret",
		"api_key",
		"token",
		"signature",
		"card_number",
		"cvc",
		"cvv",
		"payment_data",
		"credential",
		"credentials":
		return true
	default:
		return false
	}
}
