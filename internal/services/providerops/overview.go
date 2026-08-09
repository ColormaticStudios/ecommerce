package providerops

import (
	"context"
	"encoding/json"
	"fmt"

	webhookservice "ecommerce/internal/services/webhooks"
	"ecommerce/models"

	"gorm.io/gorm"
)

type WebhookStatusSummary struct {
	PendingCount    int64 `json:"pending_count"`
	ProcessedCount  int64 `json:"processed_count"`
	DeadLetterCount int64 `json:"dead_letter_count"`
	RejectedCount   int64 `json:"rejected_count"`
}

type OperationStatusSummary struct {
	TotalCount             int64 `json:"total_count"`
	ActiveCount            int64 `json:"active_count"`
	UnknownCount           int64 `json:"unknown_count"`
	FinalizeRetryCount     int64 `json:"finalize_retry_count"`
	CompensationRetryCount int64 `json:"compensation_retry_count"`
	FailedCount            int64 `json:"failed_count"`
	CompletedCount         int64 `json:"completed_count"`
}

type ReconciliationCaseSummary struct {
	OpenCount       int64 `json:"open_count"`
	UnassignedCount int64 `json:"unassigned_count"`
}

type OperationsOverview struct {
	RuntimeEnvironment          string                    `json:"runtime_environment"`
	CredentialServiceConfigured bool                      `json:"credential_service_configured"`
	WebhookEvents               WebhookStatusSummary      `json:"webhook_events"`
	Operations                  OperationStatusSummary    `json:"operations"`
	ReconciliationCases         ReconciliationCaseSummary `json:"reconciliation_cases"`
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

	operationSummary, err := s.operationSummary(ctx)
	if err != nil {
		return OperationsOverview{}, err
	}
	caseSummary, err := s.caseSummary(ctx)
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
		Operations:          operationSummary,
		ReconciliationCases: caseSummary,
	}, nil
}

func (s *OverviewService) operationSummary(ctx context.Context) (OperationStatusSummary, error) {
	type statusCount struct {
		Status string
		Count  int64
	}
	var rows []statusCount
	if err := s.db.WithContext(ctx).Model(&models.ProviderOperation{}).
		Select("status, COUNT(*) AS count").Group("status").Scan(&rows).Error; err != nil {
		return OperationStatusSummary{}, err
	}
	summary := OperationStatusSummary{}
	for _, row := range rows {
		summary.TotalCount += row.Count
		switch row.Status {
		case models.ProviderOperationStatusPrepared,
			models.ProviderOperationStatusExecuting,
			models.ProviderOperationStatusProviderSucceeded,
			models.ProviderOperationStatusFinalizing,
			models.ProviderOperationStatusCompensationPrepared,
			models.ProviderOperationStatusCompensating,
			models.ProviderOperationStatusCompensationSucceeded,
			models.ProviderOperationStatusReconciling:
			summary.ActiveCount += row.Count
		case models.ProviderOperationStatusOutcomeUnknown, models.ProviderOperationStatusReconciliationRequired:
			summary.UnknownCount += row.Count
		case models.ProviderOperationStatusFinalizeRetry:
			summary.FinalizeRetryCount += row.Count
		case models.ProviderOperationStatusCompensationRetry:
			summary.CompensationRetryCount += row.Count
		case models.ProviderOperationStatusFailed:
			summary.FailedCount += row.Count
		case models.ProviderOperationStatusCompleted:
			summary.CompletedCount += row.Count
		}
	}
	return summary, nil
}

func (s *OverviewService) caseSummary(ctx context.Context) (ReconciliationCaseSummary, error) {
	var rows []models.ProviderReconciliationCase
	if err := s.db.WithContext(ctx).Select("id", "details_json").
		Where("status = ?", models.ProviderReconciliationCaseStatusOpen).Find(&rows).Error; err != nil {
		return ReconciliationCaseSummary{}, err
	}
	summary := ReconciliationCaseSummary{OpenCount: int64(len(rows))}
	for _, row := range rows {
		var details map[string]any
		if err := json.Unmarshal([]byte(row.DetailsJSON), &details); err != nil {
			summary.UnassignedCount++
			continue
		}
		assignedTo, _ := details["assigned_to"].(string)
		if assignedTo == "" {
			summary.UnassignedCount++
		}
	}
	return summary, nil
}
