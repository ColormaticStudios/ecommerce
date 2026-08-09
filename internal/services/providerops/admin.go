package providerops

import (
	"context"
	"errors"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
)

var (
	ErrProviderOperationActionConflict    = errors.New("provider operation action is not valid for the current state")
	ErrProviderOperationCapabilityBlocked = errors.New("provider outcome lookup capability is unavailable")
)

type AdminService struct {
	db       *gorm.DB
	executor *OperationExecutor
	resolver LifecycleResolver
}

func NewAdminService(db *gorm.DB, executor *OperationExecutor, resolver LifecycleResolver) *AdminService {
	return &AdminService{db: db, executor: executor, resolver: resolver}
}

func (s *AdminService) AvailableActions(operation models.ProviderOperation) []string {
	actions := []string{}
	switch operation.Status {
	case models.ProviderOperationStatusOutcomeUnknown, models.ProviderOperationStatusReconciliationRequired:
		if s != nil && s.resolver != nil {
			lifecycle, ok := s.resolver.Resolve(operation)
			if ok && lifecycle.Lookup != nil {
				actions = append(actions, "query_outcome")
			}
		}
	case models.ProviderOperationStatusFinalizeRetry:
		actions = append(actions, "retry_finalize")
	case models.ProviderOperationStatusCompensationRetry:
		actions = append(actions, "retry_compensation")
	}
	return actions
}

func (s *AdminService) QueryOutcome(ctx context.Context, operationID uint) (models.ProviderOperation, error) {
	if s == nil || s.executor == nil || s.executor.store == nil || s.resolver == nil {
		return models.ProviderOperation{}, ErrProviderOperationCapabilityBlocked
	}
	operation, err := s.executor.store.GetOperation(ctx, operationID)
	if err != nil {
		return models.ProviderOperation{}, err
	}
	if operation.Status != models.ProviderOperationStatusOutcomeUnknown && operation.Status != models.ProviderOperationStatusReconciliationRequired {
		return operation, ErrProviderOperationActionConflict
	}
	lifecycle, ok := s.resolver.Resolve(operation)
	if !ok || lifecycle.Lookup == nil {
		return operation, ErrProviderOperationCapabilityBlocked
	}
	startedAt := time.Now().UTC()
	var observed ProviderCallResult
	var observedErr error
	lookupCalled := false
	lookup := func(lookupCtx context.Context, operationKey string) (ProviderCallResult, error) {
		lookupCalled = true
		observed, observedErr = lifecycle.Lookup(lookupCtx, operationKey)
		return observed, observedErr
	}
	updated, reconcileErr := s.executor.ReconcileOperation(ctx, ReconcileOperationInput{
		OperationKey: operation.OperationKey,
		WorkerID:     "admin-query:" + operation.OperationKey,
		Lookup:       lookup,
	})
	if !lookupCalled {
		return updated, reconcileErr
	}
	outcome := normalizeProviderOutcome(observed.Outcome)
	attemptOutcome := models.ProviderOperationAttemptOutcomeUnknown
	if outcome == models.ProviderOutcomeSucceeded {
		attemptOutcome = models.ProviderOperationAttemptOutcomeSucceeded
	} else if outcome == models.ProviderOutcomeFailed {
		attemptOutcome = models.ProviderOperationAttemptOutcomeFailed
	}
	resultJSON, marshalErr := marshalResult(observed.Result)
	if marshalErr != nil {
		return updated, errors.Join(reconcileErr, marshalErr)
	}
	_, appendErr := s.executor.store.AppendAttempt(ctx, AppendAttemptInput{
		OperationID:       operation.ID,
		Phase:             "query",
		Outcome:           attemptOutcome,
		ProviderOutcome:   defaultIfEmpty(outcome, models.ProviderOutcomeUnknown),
		ProviderReference: observed.ProviderReference,
		OperationKey:      operation.OperationKey,
		ResultJSON:        resultJSON,
		ErrorMessage:      errorText(observedErr),
		StartedAt:         startedAt,
		FinishedAt:        time.Now().UTC(),
	})
	if appendErr != nil {
		return updated, errors.Join(reconcileErr, appendErr)
	}
	reloaded, reloadErr := s.executor.store.GetOperation(ctx, operation.ID)
	return reloaded, errors.Join(reconcileErr, reloadErr)
}

func (s *AdminService) RetryFinalize(ctx context.Context, operationID uint) (models.ProviderOperation, error) {
	return s.markRetryReady(ctx, operationID, models.ProviderOperationStatusFinalizeRetry)
}

func (s *AdminService) RetryCompensation(ctx context.Context, operationID uint) (models.ProviderOperation, error) {
	return s.markRetryReady(ctx, operationID, models.ProviderOperationStatusCompensationRetry)
}

func (s *AdminService) markRetryReady(ctx context.Context, operationID uint, requiredStatus string) (models.ProviderOperation, error) {
	if s == nil || s.db == nil || s.executor == nil || s.executor.store == nil {
		return models.ProviderOperation{}, errors.New("provider operation admin service is not configured")
	}
	operation, err := s.executor.store.GetOperation(ctx, operationID)
	if err != nil {
		return models.ProviderOperation{}, err
	}
	if operation.Status != requiredStatus {
		return operation, ErrProviderOperationActionConflict
	}

	now := time.Now().UTC()
	result := s.db.WithContext(nonNilContext(ctx)).Model(&models.ProviderOperation{}).
		Where("id = ? AND status = ? AND version = ? AND (lease_expires_at IS NULL OR lease_expires_at <= ?)", operation.ID, requiredStatus, operation.Version, now).
		Updates(map[string]any{
			"next_attempt_at":  &now,
			"lease_owner":      "",
			"lease_expires_at": nil,
			"version":          gorm.Expr("version + 1"),
		})
	if result.Error != nil {
		return operation, result.Error
	}
	if result.RowsAffected != 1 {
		return operation, ErrProviderOperationActionConflict
	}
	return s.executor.store.GetOperation(ctx, operation.ID)
}
