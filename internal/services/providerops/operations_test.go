package providerops

import (
	"context"
	"errors"
	"testing"
	"time"

	"ecommerce/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOperationServicePrepareIsDurableAndDetectsFingerprintConflict(t *testing.T) {
	db := newProviderOpsTestDB(
		t,
		&models.ProviderOperation{},
		&models.ProviderOperationAttempt{},
		&models.ProviderReconciliationCase{},
	)
	service := NewOperationService(db)
	ctx := context.Background()
	input := PrepareOperationInput{
		ProviderType:   models.ProviderTypePayment,
		ProviderID:     "dummy-card",
		Environment:    models.ProviderEnvironmentSandbox,
		Operation:      "capture",
		IdempotencyKey: "capture-order-42",
		Request: map[string]any{
			"order_id": 42,
			"amount":   "12.34",
		},
		CorrelationID: "correlation-42",
		EntityType:    "order",
		EntityID:      42,
	}

	prepared, err := service.PrepareOperation(ctx, input)
	require.NoError(t, err)
	assert.NotZero(t, prepared.ID)
	assert.Equal(t, models.ProviderOperationStatusPrepared, prepared.Status)
	assert.Len(t, prepared.RequestFingerprint, 64)

	replayed, err := service.PrepareOperation(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, prepared.ID, replayed.ID)

	input.Request = map[string]any{"order_id": 42, "amount": "99.99"}
	_, err = service.PrepareOperation(ctx, input)
	require.ErrorIs(t, err, ErrIdempotencyFingerprintConflict)

	operations, total, err := service.ListOperations(ctx, ListOperationsInput{
		ProviderType: models.ProviderTypePayment,
		EntityType:   "order",
		EntityID:     42,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, operations, 1)
	assert.Equal(t, prepared.ID, operations[0].ID)
}

func TestOperationServiceGuardsTransitionsAndAppendsAttempts(t *testing.T) {
	db := newProviderOpsTestDB(
		t,
		&models.ProviderOperation{},
		&models.ProviderOperationAttempt{},
		&models.ProviderReconciliationCase{},
	)
	service := NewOperationService(db)
	ctx := context.Background()
	operation, err := service.PrepareOperation(ctx, PrepareOperationInput{
		ProviderType:       models.ProviderTypeShipping,
		ProviderID:         "dummy-ground",
		Environment:        models.ProviderEnvironmentSandbox,
		Operation:          "buy_label",
		IdempotencyKey:     "label-order-7",
		RequestFingerprint: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	})
	require.NoError(t, err)

	operation, err = service.TransitionOperation(ctx, TransitionOperationInput{
		OperationID:    operation.ID,
		ExpectedStatus: models.ProviderOperationStatusPrepared,
		Status:         models.ProviderOperationStatusInProgress,
	})
	require.NoError(t, err)
	assert.Equal(t, models.ProviderOperationStatusInProgress, operation.Status)

	_, err = service.TransitionOperation(ctx, TransitionOperationInput{
		OperationID:    operation.ID,
		ExpectedStatus: models.ProviderOperationStatusPrepared,
		Status:         models.ProviderOperationStatusFailed,
	})
	require.ErrorIs(t, err, ErrProviderOperationTransitionConflict)

	startedAt := time.Now().UTC().Add(-time.Second)
	first, err := service.AppendAttempt(ctx, AppendAttemptInput{
		OperationID:       operation.ID,
		Outcome:           models.ProviderOperationAttemptOutcomeUnknown,
		ProviderReference: "provider-request-1",
		StartedAt:         startedAt,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, first.AttemptNumber)

	second, err := service.AppendAttempt(ctx, AppendAttemptInput{
		OperationID: operation.ID,
		Outcome:     models.ProviderOperationAttemptOutcomeFailed,
		StartedAt:   startedAt,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, second.AttemptNumber)

	operation, err = service.TransitionOperation(ctx, TransitionOperationInput{
		OperationID:       operation.ID,
		ExpectedStatus:    models.ProviderOperationStatusInProgress,
		Status:            models.ProviderOperationStatusUnknown,
		ProviderReference: "provider-request-1",
	})
	require.NoError(t, err)
	assert.Equal(t, models.ProviderOperationStatusUnknown, operation.Status)

	caseInput := CreateReconciliationCaseInput{
		OperationID: operation.ID,
		AttemptID:   &first.ID,
		Reason:      "provider response was ambiguous",
	}
	createdCase, err := service.CreateReconciliationCase(ctx, caseInput)
	require.NoError(t, err)
	assert.Equal(t, models.ProviderReconciliationCaseStatusOpen, createdCase.Status)

	replayedCase, err := service.CreateReconciliationCase(ctx, caseInput)
	require.NoError(t, err)
	assert.Equal(t, createdCase.ID, replayedCase.ID)

	loaded, err := service.GetOperation(ctx, operation.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ProviderOperationStatusReconciliationRequired, loaded.Status)
	assert.Len(t, loaded.Attempts, 2)
	assert.Len(t, loaded.ReconciliationCases, 1)

	var caseCount int64
	require.NoError(t, db.Model(&models.ProviderReconciliationCase{}).Count(&caseCount).Error)
	assert.Equal(t, int64(1), caseCount)
}

func TestOperationServiceRejectsInvalidDataAndCaseState(t *testing.T) {
	db := newProviderOpsTestDB(
		t,
		&models.ProviderOperation{},
		&models.ProviderOperationAttempt{},
		&models.ProviderReconciliationCase{},
	)
	service := NewOperationService(db)
	ctx := context.Background()

	_, err := service.PrepareOperation(ctx, PrepareOperationInput{
		ProviderType:       models.ProviderTypePayment,
		ProviderID:         "dummy-card",
		Environment:        models.ProviderEnvironmentSandbox,
		Operation:          "capture",
		IdempotencyKey:     "bad-fingerprint",
		RequestFingerprint: "not-a-digest",
	})
	require.ErrorContains(t, err, "SHA-256")

	operation, err := service.PrepareOperation(ctx, PrepareOperationInput{
		ProviderType:       models.ProviderTypeTax,
		ProviderID:         "dummy-us-tax",
		Environment:        models.ProviderEnvironmentSandbox,
		Operation:          "finalize",
		IdempotencyKey:     "tax-order-9",
		RequestFingerprint: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
	})
	require.NoError(t, err)

	_, err = service.TransitionOperation(ctx, TransitionOperationInput{
		OperationID:    operation.ID,
		ExpectedStatus: models.ProviderOperationStatusPrepared,
		Status:         models.ProviderOperationStatusSucceeded,
	})
	require.ErrorIs(t, err, ErrInvalidProviderOperationTransition)

	_, err = service.AppendAttempt(ctx, AppendAttemptInput{OperationID: operation.ID, Outcome: "MAYBE"})
	require.ErrorIs(t, err, ErrInvalidProviderAttemptOutcome)

	_, err = service.CreateReconciliationCase(ctx, CreateReconciliationCaseInput{OperationID: operation.ID})
	require.True(t, errors.Is(err, ErrInvalidReconciliationCaseState))
}
