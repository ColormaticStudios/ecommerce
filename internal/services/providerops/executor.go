package providerops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"ecommerce/models"

	"gorm.io/gorm"
)

var (
	ErrOperationNotClaimable = errors.New("provider operation is not claimable")
	ErrOutcomeUnknown        = errors.New("provider outcome is unknown")
)

const (
	defaultOperationLease      = 30 * time.Second
	defaultExecutionTimeout    = 30 * time.Second
	defaultQueryTimeout        = 15 * time.Second
	defaultCompensationTimeout = time.Minute
)

// ProviderCallResult is the provider-neutral result persisted before local
// finalization. Outcome must be one of the ProviderOutcome constants.
type ProviderCallResult struct {
	Outcome           string
	ProviderReference string
	Result            any
}

// ExecuteOperationInput defines a complete durable provider mutation. Call and
// Lookup are always invoked after the preceding ledger transaction commits.
// Finalize must be idempotent because it can be retried after an ambiguous
// database acknowledgement.
type ExecuteOperationInput struct {
	Prepare       PrepareOperationInput
	WorkerID      string
	LeaseDuration time.Duration
	RetryDelay    time.Duration
	Call          func(context.Context, models.ProviderOperation) (ProviderCallResult, error)
	Lookup        func(context.Context, string) (ProviderCallResult, error)
	Finalize      func(context.Context, models.ProviderOperation, json.RawMessage) error
}

type OperationExecutorConfig struct {
	ExecutionTimeout    time.Duration
	QueryTimeout        time.Duration
	CompensationTimeout time.Duration
	LeaseDuration       time.Duration
}

type OperationExecutor struct {
	store      *OperationService
	config     OperationExecutorConfig
	lifecycles *LifecycleRegistry
}

type ReconcileOperationInput struct {
	OperationKey  string
	WorkerID      string
	LeaseDuration time.Duration
	Lookup        func(context.Context, string) (ProviderCallResult, error)
}

func NewOperationExecutor(db *gorm.DB) *OperationExecutor {
	return NewOperationExecutorWithConfig(db, OperationExecutorConfig{})
}

func NewOperationExecutorWithConfig(db *gorm.DB, cfg OperationExecutorConfig) *OperationExecutor {
	if cfg.ExecutionTimeout <= 0 {
		cfg.ExecutionTimeout = defaultExecutionTimeout
	}
	if cfg.QueryTimeout <= 0 {
		cfg.QueryTimeout = defaultQueryTimeout
	}
	if cfg.CompensationTimeout <= 0 {
		cfg.CompensationTimeout = defaultCompensationTimeout
	}
	if cfg.LeaseDuration <= 0 {
		cfg.LeaseDuration = defaultOperationLease
	}
	return &OperationExecutor{store: NewOperationService(db), config: cfg, lifecycles: NewLifecycleRegistry()}
}

func (e *OperationExecutor) Store() *OperationService { return e.store }

func (e *OperationExecutor) Lifecycles() *LifecycleRegistry { return e.lifecycles }

func (e *OperationExecutor) Execute(ctx context.Context, input ExecuteOperationInput) (models.ProviderOperation, error) {
	if e == nil || e.store == nil || e.store.db == nil {
		return models.ProviderOperation{}, errors.New("provider operation executor is not configured")
	}
	if input.Call == nil || input.Lookup == nil {
		return models.ProviderOperation{}, errors.New("provider call and operation-key lookup are required")
	}
	if strings.TrimSpace(input.Prepare.IdempotencyKey) == "" {
		return models.ProviderOperation{}, errors.New("provider operation idempotency key is required")
	}

	// Deliberately use a context without dbcontext transaction state. The intent
	// must commit independently before any provider call can begin while retaining
	// the caller's deadline and root cancellation.
	storeCtx, cancelStore := detachedContext(ctx)
	defer cancelStore()
	operation, err := e.store.PrepareOperation(storeCtx, input.Prepare)
	if err != nil {
		return models.ProviderOperation{}, err
	}
	if e.lifecycles != nil {
		e.lifecycles.Register(operation.OperationKey, OperationLifecycle{
			Call: input.Call, Lookup: input.Lookup, Finalize: input.Finalize,
		})
	}
	if operation.Status == models.ProviderOperationStatusCompleted {
		return operation, nil
	}

	workerID := strings.TrimSpace(input.WorkerID)
	if workerID == "" {
		workerID = "inline:" + operation.OperationKey
	}
	leaseDuration := input.LeaseDuration
	if leaseDuration <= 0 {
		leaseDuration = e.config.LeaseDuration
	}

	if operation.Status == models.ProviderOperationStatusPrepared || operation.Status == models.ProviderOperationStatusCompensationPrepared || operation.Status == models.ProviderOperationStatusFailed || operation.Status == models.ProviderOperationStatusCompensationRetry {
		operation, err = e.store.ClaimExecution(storeCtx, operation.ID, workerID, leaseDuration, time.Now().UTC())
		if err != nil {
			return models.ProviderOperation{}, err
		}
	}

	if operation.Status == models.ProviderOperationStatusExecuting || operation.Status == models.ProviderOperationStatusCompensating {
		startedAt := time.Now().UTC()
		callTimeout := e.config.ExecutionTimeout
		if operation.Status == models.ProviderOperationStatusCompensating {
			callTimeout = e.config.CompensationTimeout
		}
		callCtx, cancelCall := e.phaseContext(ctx, callTimeout)
		result, callErr := input.Call(callCtx, operation)
		cancelCall()
		finishedAt := time.Now().UTC()
		if callErr != nil {
			outcome := models.ProviderOperationAttemptOutcomeFailed
			providerOutcome := models.ProviderOutcomeFailed
			if errors.Is(callErr, ErrOutcomeUnknown) {
				outcome = models.ProviderOperationAttemptOutcomeUnknown
				providerOutcome = models.ProviderOutcomeUnknown
			}
			attempt, appendErr := e.store.AppendAttempt(storeCtx, AppendAttemptInput{
				OperationID: operation.ID, Outcome: outcome, ProviderOutcome: providerOutcome,
				OperationKey: operation.OperationKey, ErrorMessage: callErr.Error(), StartedAt: startedAt, FinishedAt: finishedAt,
			})
			if appendErr != nil {
				return operation, fmt.Errorf("provider call failed: %w; append attempt: %v", callErr, appendErr)
			}
			if !errors.Is(callErr, ErrOutcomeUnknown) {
				operation, err = e.store.RecordExecutionFailure(storeCtx, operation, callErr, input.RetryDelay)
				if err != nil {
					return operation, fmt.Errorf("provider call failed: %w; persist failure: %v", callErr, err)
				}
				return operation, callErr
			}
			operation, err = e.store.RecordOutcomeUnknown(storeCtx, operation, callErr)
			if err != nil {
				return operation, err
			}
			lookupCtx, cancelLookup := e.phaseContext(ctx, e.config.QueryTimeout)
			result, callErr = input.Lookup(lookupCtx, operation.OperationKey)
			cancelLookup()
			if callErr != nil || normalizeProviderOutcome(result.Outcome) == models.ProviderOutcomeUnknown {
				_, caseErr := e.store.CreateReconciliationCase(storeCtx, CreateReconciliationCaseInput{
					OperationID: operation.ID, AttemptID: &attempt.ID, Reason: "provider outcome lookup remained ambiguous",
					CaseType: "operation_key_lookup", ProviderOutcome: models.ProviderOutcomeUnknown, OperationKey: operation.OperationKey,
				})
				if caseErr != nil {
					return operation, caseErr
				}
				return e.store.GetOperationByKey(storeCtx, operation.OperationKey)
			}
		}

		providerOutcome := normalizeProviderOutcome(result.Outcome)
		if providerOutcome == "" {
			providerOutcome = models.ProviderOutcomeSucceeded
		}
		if providerOutcome != models.ProviderOutcomeSucceeded {
			providerErr := fmt.Errorf("provider operation outcome: %s", providerOutcome)
			operation, err = e.store.RecordExecutionFailure(storeCtx, operation, providerErr, input.RetryDelay)
			return operation, errors.Join(providerErr, err)
		}
		resultJSON, marshalErr := marshalResult(result.Result)
		if marshalErr != nil {
			return operation, marshalErr
		}
		_, err = e.store.AppendAttempt(storeCtx, AppendAttemptInput{
			OperationID: operation.ID, Outcome: models.ProviderOperationAttemptOutcomeSucceeded,
			ProviderOutcome: providerOutcome, ProviderReference: result.ProviderReference,
			OperationKey: operation.OperationKey, ResultJSON: resultJSON, StartedAt: startedAt, FinishedAt: finishedAt,
		})
		if err != nil {
			return operation, err
		}
		operation, err = e.store.RecordProviderSuccess(storeCtx, operation, result.ProviderReference, providerOutcome, resultJSON)
		if err != nil {
			return operation, err
		}
	}

	if operation.Status == models.ProviderOperationStatusProviderSucceeded || operation.Status == models.ProviderOperationStatusCompensationSucceeded || operation.Status == models.ProviderOperationStatusFinalizeRetry {
		operation, err = e.store.ClaimFinalization(storeCtx, operation, workerID, leaseDuration, time.Now().UTC())
		if err != nil {
			return operation, err
		}
	}
	if operation.Status == models.ProviderOperationStatusFinalizing {
		if input.Finalize != nil {
			finalizeCtx, cancelFinalize := detachedContext(ctx)
			finalizeErr := input.Finalize(finalizeCtx, operation, json.RawMessage(operation.ResultJSON))
			cancelFinalize()
			if finalizeErr != nil {
				operation, err = e.store.RecordFinalizeRetry(storeCtx, operation, finalizeErr, input.RetryDelay)
				if err != nil {
					return operation, fmt.Errorf("finalize operation: %w; persist retry: %v", finalizeErr, err)
				}
				return operation, finalizeErr
			}
		}
		return e.store.CompleteOperation(storeCtx, operation)
	}
	return operation, nil
}

// ReconcileOperation resolves an ambiguous provider outcome. The lookup runs
// without a database transaction; only the claim and resolution are transactional.
func (e *OperationExecutor) ReconcileOperation(ctx context.Context, input ReconcileOperationInput) (models.ProviderOperation, error) {
	if e == nil || e.store == nil || input.Lookup == nil {
		return models.ProviderOperation{}, errors.New("provider reconciliation lookup is required")
	}
	storeCtx, cancelStore := detachedContext(ctx)
	defer cancelStore()
	operation, err := e.store.GetOperationByKey(storeCtx, input.OperationKey)
	if err != nil {
		return models.ProviderOperation{}, err
	}
	leaseDuration := input.LeaseDuration
	if leaseDuration <= 0 {
		leaseDuration = e.config.LeaseDuration
	}
	operation, err = e.store.ClaimReconciliation(storeCtx, operation, input.WorkerID, leaseDuration, time.Now().UTC())
	if err != nil {
		return models.ProviderOperation{}, err
	}
	lookupCtx, cancelLookup := e.phaseContext(ctx, e.config.QueryTimeout)
	result, lookupErr := input.Lookup(lookupCtx, operation.OperationKey)
	cancelLookup()
	outcome := normalizeProviderOutcome(result.Outcome)
	if lookupErr != nil || outcome == "" || outcome == models.ProviderOutcomeUnknown || outcome == models.ProviderOutcomeNotFound {
		updated, updateErr := e.store.updateOperation(storeCtx, operation, models.ProviderOperationStatusReconciliationRequired, map[string]any{
			"provider_outcome": outcome, "last_error": errorText(lookupErr), "lease_owner": "", "lease_expires_at": nil,
		})
		if updateErr != nil {
			return operation, updateErr
		}
		if lookupErr != nil {
			return updated, lookupErr
		}
		return updated, ErrOutcomeUnknown
	}
	if outcome == models.ProviderOutcomeFailed {
		updated, updateErr := e.store.RecordExecutionFailure(storeCtx, operation, errors.New("provider confirmed operation failed"), 0)
		if updateErr != nil {
			return operation, updateErr
		}
		_ = e.store.ResolveOpenReconciliationCase(storeCtx, operation.ID, models.ProviderReconciliationCaseOutcomeConfirmedFailed, result)
		return updated, nil
	}
	resultJSON, err := marshalResult(result.Result)
	if err != nil {
		return operation, err
	}
	updated, err := e.store.RecordProviderSuccess(storeCtx, operation, result.ProviderReference, outcome, resultJSON)
	if err != nil {
		return operation, err
	}
	if err := e.store.ResolveOpenReconciliationCase(storeCtx, operation.ID, models.ProviderReconciliationCaseOutcomeConfirmedSucceeded, result); err != nil {
		return updated, err
	}
	return updated, nil
}

func (s *OperationService) ClaimReconciliation(ctx context.Context, operation models.ProviderOperation, workerID string, lease time.Duration, now time.Time) (models.ProviderOperation, error) {
	if operation.Status != models.ProviderOperationStatusOutcomeUnknown && operation.Status != models.ProviderOperationStatusReconciliationRequired {
		return models.ProviderOperation{}, ErrOperationNotClaimable
	}
	if strings.TrimSpace(workerID) == "" {
		workerID = "reconcile:" + operation.OperationKey
	}
	return s.claim(ctx, operation, []string{models.ProviderOperationStatusOutcomeUnknown, models.ProviderOperationStatusReconciliationRequired}, models.ProviderOperationStatusReconciling, workerID, lease, now)
}

func (s *OperationService) ResolveOpenReconciliationCase(ctx context.Context, operationID uint, outcome string, details any) error {
	raw, err := marshalResult(details)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return s.db.WithContext(nonNilContext(ctx)).Transaction(func(tx *gorm.DB) error {
		var reconciliationCase models.ProviderReconciliationCase
		if err := tx.Where("provider_operation_id = ? AND status = ?", operationID, models.ProviderReconciliationCaseStatusOpen).
			First(&reconciliationCase).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		return tx.Model(&reconciliationCase).Updates(map[string]any{
			"status": models.ProviderReconciliationCaseStatusResolved, "outcome": outcome,
			"open_key": nil, "resolved_at": &now, "resolution_json": raw, "next_attempt_at": nil,
		}).Error
	})
}

// PrepareCompensation creates an independently executable child operation.
func (s *OperationService) PrepareCompensation(ctx context.Context, parentOperationKey string, input PrepareOperationInput) (models.ProviderOperation, error) {
	storeCtx, cancelStore := detachedContext(ctx)
	defer cancelStore()
	parent, err := s.GetOperationByKey(storeCtx, parentOperationKey)
	if err != nil {
		return models.ProviderOperation{}, err
	}
	input.ParentOperationID = &parent.ID
	input.InitialStatus = models.ProviderOperationStatusCompensationPrepared
	if strings.TrimSpace(input.OperationKey) == "" {
		input.OperationKey = parent.OperationKey + ":compensate:" + strings.TrimSpace(input.Operation)
	}
	return s.PrepareOperation(storeCtx, input)
}

func (s *OperationService) GetOperationByKey(ctx context.Context, operationKey string) (models.ProviderOperation, error) {
	if s == nil || s.db == nil {
		return models.ProviderOperation{}, errors.New("provider operation service is not configured")
	}
	var operation models.ProviderOperation
	err := s.db.WithContext(nonNilContext(ctx)).Preload("Attempts", func(query *gorm.DB) *gorm.DB {
		return query.Order("attempt_number ASC")
	}).Preload("ReconciliationCases", func(query *gorm.DB) *gorm.DB {
		return query.Order("opened_at ASC, id ASC")
	}).Where("operation_key = ?", strings.TrimSpace(operationKey)).First(&operation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.ProviderOperation{}, ErrProviderOperationNotFound
	}
	return operation, err
}

func (s *OperationService) ClaimExecution(ctx context.Context, operationID uint, workerID string, lease time.Duration, now time.Time) (models.ProviderOperation, error) {
	operation, err := s.GetOperation(ctx, operationID)
	if err != nil {
		return models.ProviderOperation{}, err
	}
	to := models.ProviderOperationStatusExecuting
	allowed := []string{models.ProviderOperationStatusPrepared, models.ProviderOperationStatusFailed}
	if operation.Status == models.ProviderOperationStatusCompensationPrepared || operation.Status == models.ProviderOperationStatusCompensationRetry {
		to = models.ProviderOperationStatusCompensating
		allowed = []string{models.ProviderOperationStatusCompensationPrepared, models.ProviderOperationStatusCompensationRetry, models.ProviderOperationStatusFailed}
	}
	return s.claim(ctx, operation, allowed, to, workerID, lease, now)
}

func (s *OperationService) ClaimFinalization(ctx context.Context, operation models.ProviderOperation, workerID string, lease time.Duration, now time.Time) (models.ProviderOperation, error) {
	return s.claim(ctx, operation, []string{
		models.ProviderOperationStatusProviderSucceeded,
		models.ProviderOperationStatusCompensationSucceeded,
		models.ProviderOperationStatusFinalizeRetry,
	}, models.ProviderOperationStatusFinalizing, workerID, lease, now)
}

func (s *OperationService) claim(ctx context.Context, operation models.ProviderOperation, allowed []string, target, workerID string, lease time.Duration, now time.Time) (models.ProviderOperation, error) {
	if lease <= 0 {
		lease = defaultOperationLease
	}
	expires := now.UTC().Add(lease)
	result := s.db.WithContext(nonNilContext(ctx)).Model(&models.ProviderOperation{}).
		Where("id = ? AND version = ? AND status IN ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?) AND (lease_expires_at IS NULL OR lease_expires_at <= ? OR lease_owner = ?)", operation.ID, operation.Version, allowed, now.UTC(), now.UTC(), workerID).
		Updates(map[string]any{"status": target, "lease_owner": workerID, "lease_expires_at": &expires, "version": gorm.Expr("version + 1")})
	if result.Error != nil {
		return models.ProviderOperation{}, result.Error
	}
	if result.RowsAffected != 1 {
		return models.ProviderOperation{}, ErrOperationNotClaimable
	}
	return s.GetOperation(ctx, operation.ID)
}

func (s *OperationService) RecordProviderSuccess(ctx context.Context, operation models.ProviderOperation, reference, outcome, resultJSON string) (models.ProviderOperation, error) {
	target := models.ProviderOperationStatusProviderSucceeded
	if operation.Status == models.ProviderOperationStatusCompensating || operation.ParentOperationID != nil {
		target = models.ProviderOperationStatusCompensationSucceeded
	}
	return s.updateOperation(ctx, operation, target, map[string]any{
		"provider_reference": strings.TrimSpace(reference), "provider_outcome": normalizeProviderOutcome(outcome),
		"result_json": normalizedJSONObject(resultJSON), "last_error": "", "lease_owner": "", "lease_expires_at": nil, "next_attempt_at": nil,
	})
}

func (s *OperationService) RecordOutcomeUnknown(ctx context.Context, operation models.ProviderOperation, cause error) (models.ProviderOperation, error) {
	return s.updateOperation(ctx, operation, models.ProviderOperationStatusOutcomeUnknown, map[string]any{
		"provider_outcome": models.ProviderOutcomeUnknown, "last_error": errorText(cause), "lease_owner": "", "lease_expires_at": nil,
	})
}

func (s *OperationService) RecordExecutionFailure(ctx context.Context, operation models.ProviderOperation, cause error, retryDelay time.Duration) (models.ProviderOperation, error) {
	updates := map[string]any{"provider_outcome": models.ProviderOutcomeFailed, "last_error": errorText(cause), "lease_owner": "", "lease_expires_at": nil}
	target := models.ProviderOperationStatusFailed
	if operation.Status == models.ProviderOperationStatusCompensating || operation.ParentOperationID != nil {
		target = models.ProviderOperationStatusCompensationRetry
		if retryDelay <= 0 {
			retryDelay = time.Minute
		}
	}
	if retryDelay > 0 {
		next := time.Now().UTC().Add(retryDelay)
		updates["next_attempt_at"] = &next
	}
	return s.updateOperation(ctx, operation, target, updates)
}

func (s *OperationService) RecordFinalizeRetry(ctx context.Context, operation models.ProviderOperation, cause error, retryDelay time.Duration) (models.ProviderOperation, error) {
	if retryDelay <= 0 {
		retryDelay = time.Minute
	}
	next := time.Now().UTC().Add(retryDelay)
	return s.updateOperation(ctx, operation, models.ProviderOperationStatusFinalizeRetry, map[string]any{
		"last_error": errorText(cause), "next_attempt_at": &next, "lease_owner": "", "lease_expires_at": nil,
	})
}

func (s *OperationService) CompleteOperation(ctx context.Context, operation models.ProviderOperation) (models.ProviderOperation, error) {
	now := time.Now().UTC()
	return s.updateOperation(ctx, operation, models.ProviderOperationStatusCompleted, map[string]any{
		"completed_at": &now, "last_error": "", "next_attempt_at": nil, "lease_owner": "", "lease_expires_at": nil,
	})
}

func (s *OperationService) updateOperation(ctx context.Context, operation models.ProviderOperation, target string, updates map[string]any) (models.ProviderOperation, error) {
	compensationReconciliation := operation.ParentOperationID != nil &&
		(operation.Status == models.ProviderOperationStatusOutcomeUnknown || operation.Status == models.ProviderOperationStatusReconciling) &&
		(target == models.ProviderOperationStatusCompensationSucceeded || target == models.ProviderOperationStatusCompensationRetry)
	if !allowedOperationTransition(operation.Status, target) && !compensationReconciliation {
		return models.ProviderOperation{}, ErrInvalidProviderOperationTransition
	}
	updates["status"] = target
	updates["version"] = gorm.Expr("version + 1")
	result := s.db.WithContext(nonNilContext(ctx)).Model(&models.ProviderOperation{}).
		Where("id = ? AND status = ? AND version = ?", operation.ID, operation.Status, operation.Version).Updates(updates)
	if result.Error != nil {
		return models.ProviderOperation{}, result.Error
	}
	if result.RowsAffected != 1 {
		return models.ProviderOperation{}, ErrProviderOperationTransitionConflict
	}
	return s.GetOperation(ctx, operation.ID)
}

func marshalResult(result any) (string, error) {
	if result == nil {
		return "{}", nil
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal provider result: %w", err)
	}
	return string(raw), nil
}

func normalizeProviderOutcome(value string) string { return strings.ToUpper(strings.TrimSpace(value)) }

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func nonNilContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

type valueDetachedContext struct {
	context.Context
}

func (valueDetachedContext) Value(any) any { return nil }

// detachedContext preserves cancellation and deadlines while intentionally
// dropping values such as dbcontext's transaction handle.
func detachedContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithCancel(valueDetachedContext{Context: nonNilContext(parent)})
}

func (e *OperationExecutor) phaseContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancelDetached := detachedContext(parent)
	if timeout <= 0 {
		return ctx, cancelDetached
	}
	phaseCtx, cancelTimeout := context.WithTimeout(ctx, timeout)
	return phaseCtx, func() {
		cancelTimeout()
		cancelDetached()
	}
}
