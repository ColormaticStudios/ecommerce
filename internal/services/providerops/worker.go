package providerops

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
)

const (
	defaultRecoveryPollInterval = 5 * time.Second
	defaultRecoveryBatchSize    = 50
)

type RecoveryWorkerConfig struct {
	WorkerID     string
	PollInterval time.Duration
	BatchSize    int
	Logger       *log.Logger
}

type RecoverySummary struct {
	Examined  int
	Recovered int
	Skipped   int
	Failed    int
}

type RecoveryWorker struct {
	executor *OperationExecutor
	resolver LifecycleResolver
	config   RecoveryWorkerConfig
}

func NewRecoveryWorker(executor *OperationExecutor, resolver LifecycleResolver, cfg RecoveryWorkerConfig) *RecoveryWorker {
	if strings.TrimSpace(cfg.WorkerID) == "" {
		cfg.WorkerID = fmt.Sprintf("provider-recovery:%d", time.Now().UnixNano())
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultRecoveryPollInterval
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultRecoveryBatchSize
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Default()
	}
	return &RecoveryWorker{executor: executor, resolver: resolver, config: cfg}
}

func (w *RecoveryWorker) Run(ctx context.Context) {
	if w == nil || w.executor == nil || w.resolver == nil {
		return
	}
	w.runAndLog(ctx)
	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runAndLog(ctx)
		}
	}
}

func (w *RecoveryWorker) runAndLog(ctx context.Context) {
	summary, err := w.RunOnce(ctx)
	if err != nil {
		if !errors.Is(err, context.Canceled) {
			w.config.Logger.Printf("[ERROR] Provider operation recovery failed: %v", err)
		}
		return
	}
	if summary.Recovered > 0 || summary.Failed > 0 {
		w.config.Logger.Printf(
			"[INFO] Provider operation recovery examined=%d recovered=%d skipped=%d failed=%d",
			summary.Examined, summary.Recovered, summary.Skipped, summary.Failed,
		)
	}
}

func (w *RecoveryWorker) RunOnce(ctx context.Context) (RecoverySummary, error) {
	var summary RecoverySummary
	if w == nil || w.executor == nil || w.executor.store == nil || w.resolver == nil {
		return summary, errors.New("provider recovery worker is not configured")
	}
	storeCtx, cancelStore := detachedContext(ctx)
	defer cancelStore()
	now := time.Now().UTC()
	operations, err := w.recoveryCandidates(storeCtx, now)
	if err != nil {
		return summary, err
	}
	for _, operation := range operations {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		summary.Examined++
		recovered, recoverErr := w.recoverOne(ctx, operation, now)
		if recoverErr != nil {
			if errors.Is(recoverErr, ErrOperationNotClaimable) || errors.Is(recoverErr, ErrProviderOperationTransitionConflict) {
				summary.Skipped++
				continue
			}
			summary.Failed++
			w.config.Logger.Printf("[ERROR] Recover provider operation key=%s: %v", operation.OperationKey, recoverErr)
			continue
		}
		if recovered {
			summary.Recovered++
		} else {
			summary.Skipped++
		}
	}
	return summary, nil
}

func (w *RecoveryWorker) recoveryCandidates(ctx context.Context, now time.Time) ([]models.ProviderOperation, error) {
	var operations []models.ProviderOperation
	immediateStatuses := []string{
		models.ProviderOperationStatusPrepared,
		models.ProviderOperationStatusOutcomeUnknown,
		models.ProviderOperationStatusReconciliationRequired,
		models.ProviderOperationStatusProviderSucceeded,
		models.ProviderOperationStatusCompensationPrepared,
		models.ProviderOperationStatusCompensationSucceeded,
	}
	retryStatuses := []string{
		models.ProviderOperationStatusFailed,
		models.ProviderOperationStatusFinalizeRetry,
		models.ProviderOperationStatusCompensationRetry,
	}
	leasedStatuses := []string{
		models.ProviderOperationStatusExecuting,
		models.ProviderOperationStatusCompensating,
		models.ProviderOperationStatusReconciling,
		models.ProviderOperationStatusFinalizing,
	}
	err := w.executor.store.db.WithContext(nonNilContext(ctx)).
		Where("(status IN ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?) AND (lease_expires_at IS NULL OR lease_expires_at <= ?)) OR (status IN ? AND next_attempt_at IS NOT NULL AND next_attempt_at <= ? AND (lease_expires_at IS NULL OR lease_expires_at <= ?)) OR (status IN ? AND lease_expires_at <= ?)", immediateStatuses, now, now, retryStatuses, now, now, leasedStatuses, now).
		Order("COALESCE(next_attempt_at, created_at) ASC, id ASC").
		Limit(w.config.BatchSize).
		Find(&operations).Error
	return operations, err
}

func (w *RecoveryWorker) recoverOne(ctx context.Context, operation models.ProviderOperation, now time.Time) (bool, error) {
	lifecycle, resolved := w.resolver.Resolve(operation)
	if !resolved {
		return false, nil
	}

	switch operation.Status {
	case models.ProviderOperationStatusExecuting, models.ProviderOperationStatusCompensating:
		if lifecycle.Lookup == nil {
			return false, nil
		}
		updated, err := w.releaseExpiredLease(ctx, operation, models.ProviderOperationStatusOutcomeUnknown, now)
		if err != nil {
			return false, err
		}
		return w.reconcile(ctx, updated, lifecycle)
	case models.ProviderOperationStatusReconciling:
		if lifecycle.Lookup == nil {
			return false, nil
		}
		updated, err := w.releaseExpiredLease(ctx, operation, models.ProviderOperationStatusReconciliationRequired, now)
		if err != nil {
			return false, err
		}
		return w.reconcile(ctx, updated, lifecycle)
	case models.ProviderOperationStatusOutcomeUnknown, models.ProviderOperationStatusReconciliationRequired:
		if lifecycle.Lookup == nil {
			return false, nil
		}
		return w.reconcile(ctx, operation, lifecycle)
	case models.ProviderOperationStatusFinalizing:
		if lifecycle.Call == nil {
			return false, nil
		}
		updated, err := w.releaseExpiredLease(ctx, operation, models.ProviderOperationStatusFinalizeRetry, now)
		if err != nil {
			return false, err
		}
		operation = updated
	}

	if lifecycle.Call == nil || lifecycle.Lookup == nil {
		return false, nil
	}
	_, err := w.executor.Execute(ctx, executeInputForRecovery(operation, lifecycle, w.config.WorkerID))
	return err == nil, err
}

func (w *RecoveryWorker) reconcile(ctx context.Context, operation models.ProviderOperation, lifecycle OperationLifecycle) (bool, error) {
	updated, err := w.executor.ReconcileOperation(ctx, ReconcileOperationInput{
		OperationKey: operation.OperationKey,
		WorkerID:     w.config.WorkerID,
		Lookup:       lifecycle.Lookup,
	})
	if err != nil {
		return false, err
	}
	if (updated.Status == models.ProviderOperationStatusProviderSucceeded || updated.Status == models.ProviderOperationStatusCompensationSucceeded) && lifecycle.Call != nil {
		_, err = w.executor.Execute(ctx, executeInputForRecovery(updated, lifecycle, w.config.WorkerID))
		if err != nil {
			return false, err
		}
	}
	return true, nil
}

func (w *RecoveryWorker) releaseExpiredLease(ctx context.Context, operation models.ProviderOperation, target string, now time.Time) (models.ProviderOperation, error) {
	if operation.LeaseExpiresAt == nil || operation.LeaseExpiresAt.After(now) {
		return models.ProviderOperation{}, ErrOperationNotClaimable
	}
	storeCtx, cancelStore := detachedContext(ctx)
	defer cancelStore()
	updates := map[string]any{
		"status": target, "lease_owner": "", "lease_expires_at": nil,
		"last_error": "operation lease expired during " + strings.ToLower(operation.Status),
		"version":    gorm.Expr("version + 1"),
	}
	if target == models.ProviderOperationStatusFinalizeRetry {
		updates["next_attempt_at"] = now
	}
	result := w.executor.store.db.WithContext(storeCtx).Model(&models.ProviderOperation{}).
		Where("id = ? AND status = ? AND version = ? AND lease_expires_at <= ?", operation.ID, operation.Status, operation.Version, now).
		Updates(updates)
	if result.Error != nil {
		return models.ProviderOperation{}, result.Error
	}
	if result.RowsAffected != 1 {
		return models.ProviderOperation{}, ErrOperationNotClaimable
	}
	return w.executor.store.GetOperation(storeCtx, operation.ID)
}

func executeInputForRecovery(operation models.ProviderOperation, lifecycle OperationLifecycle, workerID string) ExecuteOperationInput {
	return ExecuteOperationInput{
		Prepare: PrepareOperationInput{
			OperationKey:       operation.OperationKey,
			ParentOperationID:  operation.ParentOperationID,
			InitialStatus:      initialStatusFor(operation),
			ProviderType:       operation.ProviderType,
			ProviderID:         operation.ProviderID,
			Environment:        operation.Environment,
			Operation:          operation.Operation,
			IdempotencyKey:     operation.IdempotencyKey,
			RequestFingerprint: operation.RequestFingerprint,
			RequestJSON:        operation.RequestJSON,
			CorrelationID:      operation.CorrelationID,
			EntityType:         operation.EntityType,
			EntityID:           operation.EntityID,
			MetadataJSON:       operation.MetadataJSON,
		},
		WorkerID: workerID,
		Call:     lifecycle.Call,
		Lookup:   lifecycle.Lookup,
		Finalize: lifecycle.Finalize,
	}
}

func initialStatusFor(operation models.ProviderOperation) string {
	if operation.ParentOperationID != nil || operation.Status == models.ProviderOperationStatusCompensationPrepared || operation.Status == models.ProviderOperationStatusCompensationRetry || operation.Status == models.ProviderOperationStatusCompensationSucceeded {
		return models.ProviderOperationStatusCompensationPrepared
	}
	return models.ProviderOperationStatusPrepared
}
