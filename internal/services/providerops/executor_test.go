package providerops

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"ecommerce/internal/dbcontext"
	"ecommerce/models"

	"github.com/stretchr/testify/require"
)

func newExecutorTestDB(t *testing.T) *OperationExecutor {
	t.Helper()
	db := newProviderOpsTestDB(t,
		&models.ProviderOperation{},
		&models.ProviderOperationAttempt{},
		&models.ProviderReconciliationCase{},
	)
	return NewOperationExecutor(db)
}

func TestOperationExecutorPersistsBeforeExternalCallAndCompletes(t *testing.T) {
	executor := newExecutorTestDB(t)
	var providerCalls atomic.Int32
	var finalized atomic.Bool

	tx := executor.store.db.Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()
	ctx := dbcontext.WithDB(context.Background(), tx)

	operation, err := executor.Execute(ctx, ExecuteOperationInput{
		Prepare: PrepareOperationInput{
			OperationKey: "op-capture-42", ProviderType: models.ProviderTypePayment,
			ProviderID: "dummy-card", Environment: models.ProviderEnvironmentSandbox,
			Operation: "capture", IdempotencyKey: "capture-42", Request: map[string]any{"amount": 4200},
		},
		Call: func(_ context.Context, operation models.ProviderOperation) (ProviderCallResult, error) {
			providerCalls.Add(1)
			var count int64
			require.NoError(t, executor.store.db.Model(&models.ProviderOperation{}).
				Where("operation_key = ? AND status = ?", operation.OperationKey, models.ProviderOperationStatusExecuting).
				Count(&count).Error)
			require.EqualValues(t, 1, count, "intent must be committed before the provider call")
			return ProviderCallResult{
				Outcome: models.ProviderOutcomeSucceeded, ProviderReference: "provider-capture-42",
				Result: map[string]any{"captured": true},
			}, nil
		},
		Lookup: func(context.Context, string) (ProviderCallResult, error) {
			t.Fatal("lookup should not run for a definitive response")
			return ProviderCallResult{}, nil
		},
		Finalize: func(_ context.Context, _ models.ProviderOperation, result json.RawMessage) error {
			finalized.Store(true)
			require.JSONEq(t, `{"captured":true}`, string(result))
			return nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, models.ProviderOperationStatusCompleted, operation.Status)
	require.Equal(t, models.ProviderOutcomeSucceeded, operation.ProviderOutcome)
	require.Equal(t, "provider-capture-42", operation.ProviderReference)
	require.EqualValues(t, 1, providerCalls.Load())
	require.True(t, finalized.Load())
}

func TestOperationExecutorQueriesAmbiguousOutcomeByOperationKey(t *testing.T) {
	executor := newExecutorTestDB(t)
	var lookupKey string

	operation, err := executor.Execute(context.Background(), ExecuteOperationInput{
		Prepare: PrepareOperationInput{
			OperationKey: "op-label-7", ProviderType: models.ProviderTypeShipping,
			ProviderID: "dummy-ground", Environment: models.ProviderEnvironmentSandbox,
			Operation: "buy_label", IdempotencyKey: "label-7", Request: map[string]any{"rate_id": 7},
		},
		Call: func(context.Context, models.ProviderOperation) (ProviderCallResult, error) {
			return ProviderCallResult{}, ErrOutcomeUnknown
		},
		Lookup: func(_ context.Context, operationKey string) (ProviderCallResult, error) {
			lookupKey = operationKey
			return ProviderCallResult{
				Outcome: models.ProviderOutcomeSucceeded, ProviderReference: "shipment-7",
				Result: map[string]string{"provider_shipment_id": "shipment-7"},
			}, nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, "op-label-7", lookupKey)
	require.Equal(t, models.ProviderOperationStatusCompleted, operation.Status)
	require.Len(t, operation.Attempts, 2)
	require.Equal(t, models.ProviderOperationAttemptOutcomeUnknown, operation.Attempts[0].Outcome)
	require.Equal(t, models.ProviderOperationAttemptOutcomeSucceeded, operation.Attempts[1].Outcome)
}

func TestOperationExecutorCreatesReconciliationCaseWhenLookupIsAmbiguous(t *testing.T) {
	executor := newExecutorTestDB(t)

	operation, err := executor.Execute(context.Background(), ExecuteOperationInput{
		Prepare: PrepareOperationInput{
			OperationKey: "op-tax-9", ProviderType: models.ProviderTypeTax,
			ProviderID: "dummy-us-tax", Environment: models.ProviderEnvironmentSandbox,
			Operation: "finalize", IdempotencyKey: "tax-9", Request: map[string]any{"snapshot_id": 9},
		},
		Call: func(context.Context, models.ProviderOperation) (ProviderCallResult, error) {
			return ProviderCallResult{}, ErrOutcomeUnknown
		},
		Lookup: func(context.Context, string) (ProviderCallResult, error) {
			return ProviderCallResult{Outcome: models.ProviderOutcomeUnknown}, ErrOutcomeUnknown
		},
	})
	require.NoError(t, err)
	require.Equal(t, models.ProviderOperationStatusReconciliationRequired, operation.Status)
	require.Len(t, operation.ReconciliationCases, 1)
	require.Equal(t, operation.OperationKey, operation.ReconciliationCases[0].OperationKey)
}

func TestOperationExecutorReconcilesOutsideTransactionAndResolvesCase(t *testing.T) {
	executor := newExecutorTestDB(t)
	operation, err := executor.Execute(context.Background(), ExecuteOperationInput{
		Prepare: PrepareOperationInput{
			OperationKey: "op-reconcile-1", ProviderType: models.ProviderTypePayment,
			ProviderID: "dummy-card", Environment: models.ProviderEnvironmentSandbox,
			Operation: "capture", IdempotencyKey: "reconcile-1", Request: map[string]any{"amount": 100},
		},
		Call: func(context.Context, models.ProviderOperation) (ProviderCallResult, error) {
			return ProviderCallResult{}, ErrOutcomeUnknown
		},
		Lookup: func(context.Context, string) (ProviderCallResult, error) {
			return ProviderCallResult{Outcome: models.ProviderOutcomeUnknown}, ErrOutcomeUnknown
		},
	})
	require.NoError(t, err)
	require.Equal(t, models.ProviderOperationStatusReconciliationRequired, operation.Status)

	operation, err = executor.ReconcileOperation(context.Background(), ReconcileOperationInput{
		OperationKey: operation.OperationKey,
		Lookup: func(_ context.Context, operationKey string) (ProviderCallResult, error) {
			require.Equal(t, "op-reconcile-1", operationKey)
			return ProviderCallResult{
				Outcome: models.ProviderOutcomeSucceeded, ProviderReference: "capture-1",
				Result: map[string]bool{"captured": true},
			}, nil
		},
	})
	require.NoError(t, err)
	require.Equal(t, models.ProviderOperationStatusProviderSucceeded, operation.Status)

	loaded, err := executor.store.GetOperationByKey(context.Background(), operation.OperationKey)
	require.NoError(t, err)
	require.Len(t, loaded.ReconciliationCases, 1)
	require.Equal(t, models.ProviderReconciliationCaseStatusResolved, loaded.ReconciliationCases[0].Status)
	require.Equal(t, models.ProviderReconciliationCaseOutcomeConfirmedSucceeded, loaded.ReconciliationCases[0].Outcome)
}

func TestOperationExecutorRetriesFinalizationWithoutRepeatingProviderCall(t *testing.T) {
	executor := newExecutorTestDB(t)
	var providerCalls atomic.Int32
	var finalizeCalls atomic.Int32
	input := ExecuteOperationInput{
		Prepare: PrepareOperationInput{
			OperationKey: "op-refund-5", ProviderType: models.ProviderTypePayment,
			ProviderID: "dummy-card", Environment: models.ProviderEnvironmentSandbox,
			Operation: "refund", IdempotencyKey: "refund-5", Request: map[string]any{"amount": 500},
		},
		Call: func(context.Context, models.ProviderOperation) (ProviderCallResult, error) {
			providerCalls.Add(1)
			return ProviderCallResult{Outcome: models.ProviderOutcomeSucceeded, Result: map[string]bool{"refunded": true}}, nil
		},
		Lookup: func(context.Context, string) (ProviderCallResult, error) {
			return ProviderCallResult{}, errors.New("unexpected lookup")
		},
		Finalize: func(context.Context, models.ProviderOperation, json.RawMessage) error {
			if finalizeCalls.Add(1) == 1 {
				return errors.New("database temporarily unavailable")
			}
			return nil
		},
	}

	operation, err := executor.Execute(context.Background(), input)
	require.Error(t, err)
	require.Equal(t, models.ProviderOperationStatusFinalizeRetry, operation.Status)
	require.EqualValues(t, 1, providerCalls.Load())

	past := time.Now().UTC().Add(-time.Second)
	require.NoError(t, executor.store.db.Model(&models.ProviderOperation{}).Where("id = ?", operation.ID).Update("next_attempt_at", &past).Error)
	operation, err = executor.Execute(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, models.ProviderOperationStatusCompleted, operation.Status)
	require.EqualValues(t, 1, providerCalls.Load(), "finalize retry must not repeat the provider mutation")
	require.EqualValues(t, 2, finalizeCalls.Load())
}

func TestPrepareCompensationCreatesDurableChild(t *testing.T) {
	executor := newExecutorTestDB(t)
	parent, err := executor.store.PrepareOperation(context.Background(), PrepareOperationInput{
		OperationKey: "op-parent", ProviderType: models.ProviderTypePayment,
		ProviderID: "dummy-card", Environment: models.ProviderEnvironmentSandbox,
		Operation: "capture", IdempotencyKey: "parent", Request: map[string]any{"amount": 100},
	})
	require.NoError(t, err)

	child, err := executor.store.PrepareCompensation(context.Background(), parent.OperationKey, PrepareOperationInput{
		OperationKey: "op-parent-refund", ProviderType: models.ProviderTypePayment,
		ProviderID: "dummy-card", Environment: models.ProviderEnvironmentSandbox,
		Operation: "refund", IdempotencyKey: "parent-refund", Request: map[string]any{"amount": 100},
	})
	require.NoError(t, err)
	require.Equal(t, models.ProviderOperationStatusCompensationPrepared, child.Status)
	require.NotNil(t, child.ParentOperationID)
	require.Equal(t, parent.ID, *child.ParentOperationID)
}
