package providerops

import (
	"context"
	"errors"
	"io"
	"log"
	"sync/atomic"
	"testing"
	"time"

	"ecommerce/internal/dbcontext"
	"ecommerce/models"

	"github.com/stretchr/testify/require"
)

func TestRuntimeWiresOperationLifecycleConfiguration(t *testing.T) {
	db := newProviderOpsTestDB(t)
	runtime := NewRuntime(db, RuntimeConfig{
		ExecutionTimeout: 11 * time.Second, QueryTimeout: 12 * time.Second,
		CompensationTimeout: 13 * time.Second, LeaseDuration: 14 * time.Second,
	})

	require.NotNil(t, runtime.Executor)
	require.NotNil(t, runtime.Recovery)
	require.Same(t, runtime.Operations, runtime.Executor.Store())
	require.Equal(t, 11*time.Second, runtime.Executor.config.ExecutionTimeout)
	require.Equal(t, 12*time.Second, runtime.Executor.config.QueryTimeout)
	require.Equal(t, 13*time.Second, runtime.Executor.config.CompensationTimeout)
	require.Equal(t, 14*time.Second, runtime.Executor.config.LeaseDuration)
}

func TestRuntimeBindDatabasePreservesLifecycleConfiguration(t *testing.T) {
	runtime := NewRuntime(nil, RuntimeConfig{ExecutionTimeout: 11 * time.Second, QueryTimeout: 12 * time.Second})
	db := newProviderOpsTestDB(t)

	runtime.BindDatabase(db)

	require.Same(t, db, runtime.Executor.store.db)
	require.Same(t, runtime.Operations, runtime.Executor.Store())
	require.Equal(t, 11*time.Second, runtime.Executor.config.ExecutionTimeout)
	require.Equal(t, 12*time.Second, runtime.Executor.config.QueryTimeout)
	require.NotNil(t, runtime.Reconciliation)
	require.NotNil(t, runtime.Recovery)
}

func TestOperationExecutorUsesConfiguredExecutionTimeoutAndLease(t *testing.T) {
	executor := newExecutorTestDBWithConfig(t, OperationExecutorConfig{
		ExecutionTimeout: 20 * time.Millisecond,
		LeaseDuration:    3 * time.Minute,
	})
	var leaseDuration time.Duration

	operation, err := executor.Execute(context.Background(), ExecuteOperationInput{
		Prepare: testPrepare("timeout-execution"),
		Call: func(ctx context.Context, operation models.ProviderOperation) (ProviderCallResult, error) {
			require.NotNil(t, operation.LeaseExpiresAt)
			leaseDuration = operation.LeaseExpiresAt.Sub(time.Now().UTC())
			<-ctx.Done()
			return ProviderCallResult{}, ctx.Err()
		},
		Lookup: func(context.Context, string) (ProviderCallResult, error) {
			return ProviderCallResult{}, errors.New("unexpected lookup")
		},
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, models.ProviderOperationStatusFailed, operation.Status)
	require.InDelta(t, (3 * time.Minute).Seconds(), leaseDuration.Seconds(), 2)
}

func TestOperationExecutorUsesConfiguredQueryTimeout(t *testing.T) {
	executor := newExecutorTestDBWithConfig(t, OperationExecutorConfig{QueryTimeout: 20 * time.Millisecond})

	operation, err := executor.Execute(context.Background(), ExecuteOperationInput{
		Prepare: testPrepare("timeout-query"),
		Call: func(context.Context, models.ProviderOperation) (ProviderCallResult, error) {
			return ProviderCallResult{}, ErrOutcomeUnknown
		},
		Lookup: func(ctx context.Context, _ string) (ProviderCallResult, error) {
			<-ctx.Done()
			return ProviderCallResult{Outcome: models.ProviderOutcomeUnknown}, ctx.Err()
		},
	})
	require.NoError(t, err)
	require.Equal(t, models.ProviderOperationStatusReconciliationRequired, operation.Status)
}

func TestOperationExecutorUsesConfiguredCompensationTimeout(t *testing.T) {
	executor := newExecutorTestDBWithConfig(t, OperationExecutorConfig{CompensationTimeout: 20 * time.Millisecond})
	parent, err := executor.Store().PrepareOperation(context.Background(), testPrepare("compensation-parent"))
	require.NoError(t, err)
	prepare := testPrepare("compensation-timeout")
	prepare.ParentOperationID = &parent.ID
	prepare.InitialStatus = models.ProviderOperationStatusCompensationPrepared

	operation, err := executor.Execute(context.Background(), ExecuteOperationInput{
		Prepare: prepare,
		Call: func(ctx context.Context, _ models.ProviderOperation) (ProviderCallResult, error) {
			<-ctx.Done()
			return ProviderCallResult{}, ctx.Err()
		},
		Lookup: func(context.Context, string) (ProviderCallResult, error) {
			return ProviderCallResult{}, errors.New("unexpected lookup")
		},
	})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, models.ProviderOperationStatusCompensationRetry, operation.Status)
	require.NotNil(t, operation.NextAttemptAt)
}

func TestDetachedContextDropsValuesAndPreservesDeadlineAndCancellation(t *testing.T) {
	deadline := time.Now().Add(time.Minute)
	parent, cancelParent := context.WithDeadline(context.Background(), deadline)
	txMarker := newProviderOpsTestDB(t)
	parent = dbcontext.WithDB(parent, txMarker)
	detached, cancelDetached := detachedContext(parent)
	defer cancelDetached()

	_, ok := detached.Deadline()
	require.True(t, ok)
	require.Nil(t, dbcontext.GetDB(detached))
	cancelParent()
	select {
	case <-detached.Done():
		require.ErrorIs(t, detached.Err(), context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("detached context did not preserve parent cancellation")
	}
}

func TestRecoveryWorkerQueriesExpiredExecutionWithoutRepeatingMutation(t *testing.T) {
	executor := newExecutorTestDBWithConfig(t, OperationExecutorConfig{QueryTimeout: time.Second})
	operation, err := executor.Store().PrepareOperation(context.Background(), testPrepare("expired-execution"))
	require.NoError(t, err)
	operation, err = executor.Store().ClaimExecution(context.Background(), operation.ID, "dead-worker", time.Second, time.Now().Add(-2*time.Second))
	require.NoError(t, err)

	var lookupCalls atomic.Int32
	resolver := staticLifecycleResolver{resolve: func(models.ProviderOperation) (OperationLifecycle, bool) {
		return OperationLifecycle{Lookup: func(context.Context, string) (ProviderCallResult, error) {
			lookupCalls.Add(1)
			return ProviderCallResult{
				Outcome: models.ProviderOutcomeSucceeded, ProviderReference: "provider-recovered",
				Result: map[string]bool{"recovered": true},
			}, nil
		}}, true
	}}
	worker := newTestRecoveryWorker(executor, resolver)

	summary, err := worker.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, summary.Recovered)
	require.EqualValues(t, 1, lookupCalls.Load())
	operation, err = executor.Store().GetOperation(context.Background(), operation.ID)
	require.NoError(t, err)
	require.Equal(t, models.ProviderOperationStatusProviderSucceeded, operation.Status)
	require.Equal(t, "provider-recovered", operation.ProviderReference)
}

func TestRecoveryWorkerPreservesCompensationStateAfterExpiredLease(t *testing.T) {
	executor := newExecutorTestDBWithConfig(t, OperationExecutorConfig{QueryTimeout: time.Second})
	parent, err := executor.Store().PrepareOperation(context.Background(), testPrepare("recovery-parent"))
	require.NoError(t, err)
	prepare := testPrepare("expired-compensation")
	prepare.ParentOperationID = &parent.ID
	prepare.InitialStatus = models.ProviderOperationStatusCompensationPrepared
	operation, err := executor.Store().PrepareOperation(context.Background(), prepare)
	require.NoError(t, err)
	operation, err = executor.Store().ClaimExecution(context.Background(), operation.ID, "dead-worker", time.Second, time.Now().Add(-2*time.Second))
	require.NoError(t, err)

	resolver := staticLifecycleResolver{resolve: func(models.ProviderOperation) (OperationLifecycle, bool) {
		return OperationLifecycle{Lookup: func(context.Context, string) (ProviderCallResult, error) {
			return ProviderCallResult{Outcome: models.ProviderOutcomeSucceeded, ProviderReference: "reversed"}, nil
		}}, true
	}}
	summary, err := newTestRecoveryWorker(executor, resolver).RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, summary.Recovered)
	operation, err = executor.Store().GetOperation(context.Background(), operation.ID)
	require.NoError(t, err)
	require.Equal(t, models.ProviderOperationStatusCompensationSucceeded, operation.Status)
	require.Equal(t, "reversed", operation.ProviderReference)
}

func TestRecoveryWorkerResumesOnlyResolvedCallbacks(t *testing.T) {
	executor := newExecutorTestDBWithConfig(t, OperationExecutorConfig{})
	resolved, err := executor.Store().PrepareOperation(context.Background(), testPrepare("resolved-prepared"))
	require.NoError(t, err)
	unresolved, err := executor.Store().PrepareOperation(context.Background(), testPrepare("unresolved-prepared"))
	require.NoError(t, err)

	var calls atomic.Int32
	resolver := staticLifecycleResolver{resolve: func(operation models.ProviderOperation) (OperationLifecycle, bool) {
		if operation.ID != resolved.ID {
			return OperationLifecycle{}, false
		}
		return OperationLifecycle{
			Call: func(context.Context, models.ProviderOperation) (ProviderCallResult, error) {
				calls.Add(1)
				return ProviderCallResult{Outcome: models.ProviderOutcomeSucceeded}, nil
			},
			Lookup: func(context.Context, string) (ProviderCallResult, error) {
				return ProviderCallResult{Outcome: models.ProviderOutcomeSucceeded}, nil
			},
		}, true
	}}
	worker := newTestRecoveryWorker(executor, resolver)

	summary, err := worker.RunOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, summary.Examined)
	require.Equal(t, 1, summary.Recovered)
	require.Equal(t, 1, summary.Skipped)
	require.EqualValues(t, 1, calls.Load())

	resolved, err = executor.Store().GetOperation(context.Background(), resolved.ID)
	require.NoError(t, err)
	require.Equal(t, models.ProviderOperationStatusCompleted, resolved.Status)
	unresolved, err = executor.Store().GetOperation(context.Background(), unresolved.ID)
	require.NoError(t, err)
	require.Equal(t, models.ProviderOperationStatusPrepared, unresolved.Status)
}

func TestRecoveryWorkerStopsOnCancellation(t *testing.T) {
	executor := newExecutorTestDBWithConfig(t, OperationExecutorConfig{})
	worker := NewRecoveryWorker(executor, staticLifecycleResolver{}, RecoveryWorkerConfig{
		PollInterval: time.Hour,
		Logger:       log.New(io.Discard, "", 0),
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("recovery worker did not stop after cancellation")
	}
}

type staticLifecycleResolver struct {
	resolve func(models.ProviderOperation) (OperationLifecycle, bool)
}

func (r staticLifecycleResolver) Resolve(operation models.ProviderOperation) (OperationLifecycle, bool) {
	if r.resolve == nil {
		return OperationLifecycle{}, false
	}
	return r.resolve(operation)
}

func newExecutorTestDBWithConfig(t *testing.T, cfg OperationExecutorConfig) *OperationExecutor {
	t.Helper()
	db := newProviderOpsTestDB(t,
		&models.ProviderOperation{},
		&models.ProviderOperationAttempt{},
		&models.ProviderReconciliationCase{},
	)
	return NewOperationExecutorWithConfig(db, cfg)
}

func newTestRecoveryWorker(executor *OperationExecutor, resolver LifecycleResolver) *RecoveryWorker {
	return NewRecoveryWorker(executor, resolver, RecoveryWorkerConfig{
		WorkerID: "recovery-test", Logger: log.New(io.Discard, "", 0),
	})
}

func testPrepare(key string) PrepareOperationInput {
	return PrepareOperationInput{
		OperationKey: key, ProviderType: models.ProviderTypePayment,
		ProviderID: "dummy-card", Environment: models.ProviderEnvironmentSandbox,
		Operation: "capture", IdempotencyKey: key, Request: map[string]any{"key": key},
	}
}
