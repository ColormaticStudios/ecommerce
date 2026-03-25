package providerops

import (
	"context"
	"fmt"

	webhookservice "ecommerce/internal/services/webhooks"

	"gorm.io/gorm"
)

type WebhookStatusSummary struct {
	PendingCount    int64 `json:"pending_count"`
	ProcessedCount  int64 `json:"processed_count"`
	DeadLetterCount int64 `json:"dead_letter_count"`
	RejectedCount   int64 `json:"rejected_count"`
}

type OperationsOverview struct {
	RuntimeEnvironment          string               `json:"runtime_environment"`
	CredentialServiceConfigured bool                 `json:"credential_service_configured"`
	WebhookEvents               WebhookStatusSummary `json:"webhook_events"`
}

type OverviewService struct {
	db                 *gorm.DB
	environment        string
	credentials        *CredentialService
	maxWebhookAttempts int
}

func NewOverviewService(
	db *gorm.DB,
	environment string,
	credentials *CredentialService,
	maxWebhookAttempts int,
) *OverviewService {
	if maxWebhookAttempts <= 0 {
		maxWebhookAttempts = webhookservice.DefaultMaxAttempts
	}
	return &OverviewService{
		db:                 db,
		environment:        environment,
		credentials:        credentials,
		maxWebhookAttempts: maxWebhookAttempts,
	}
}

func (s *OverviewService) Get(ctx context.Context) (OperationsOverview, error) {
	if s == nil || s.db == nil {
		return OperationsOverview{}, fmt.Errorf("provider overview service is not configured")
	}

	count := func(query func(*gorm.DB) *gorm.DB) (int64, error) {
		db := query(s.db.WithContext(ctx).Table("webhook_events"))
		var total int64
		if err := db.Count(&total).Error; err != nil {
			return 0, err
		}
		return total, nil
	}

	rejectedCount, err := count(func(db *gorm.DB) *gorm.DB {
		return db.Where("signature_valid = ?", false)
	})
	if err != nil {
		return OperationsOverview{}, err
	}
	pendingCount, err := count(func(db *gorm.DB) *gorm.DB {
		return db.Where("signature_valid = ? AND processed_at IS NULL AND attempt_count < ?", true, s.maxWebhookAttempts)
	})
	if err != nil {
		return OperationsOverview{}, err
	}
	processedCount, err := count(func(db *gorm.DB) *gorm.DB {
		return db.Where("signature_valid = ? AND processed_at IS NOT NULL", true)
	})
	if err != nil {
		return OperationsOverview{}, err
	}
	deadLetterCount, err := count(func(db *gorm.DB) *gorm.DB {
		return db.Where("signature_valid = ? AND processed_at IS NULL AND attempt_count >= ?", true, s.maxWebhookAttempts)
	})
	if err != nil {
		return OperationsOverview{}, err
	}

	return OperationsOverview{
		RuntimeEnvironment:          s.environment,
		CredentialServiceConfigured: s.credentials != nil && s.credentials.Enabled(),
		WebhookEvents: WebhookStatusSummary{
			PendingCount:    pendingCount,
			ProcessedCount:  processedCount,
			DeadLetterCount: deadLetterCount,
			RejectedCount:   rejectedCount,
		},
	}, nil
}
