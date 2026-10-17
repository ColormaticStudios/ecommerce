package jobs

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"ecommerce/internal/reliability"
	"ecommerce/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCoalescedContentionBackoffIsBoundedAndRollsBackCaller(t *testing.T) {
	runtime, db, _ := newTestRuntime(t, nil)
	input := EnqueueInput{JobType: "test.coalesced", Payload: map[string]int{"version": 1}, IdempotencyKey: "entity:1:pending"}
	original, err := runtime.Enqueue(context.Background(), input)
	require.NoError(t, err)
	calls := 0
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("coalesced_vanished_key", func(tx *gorm.DB) {
		if _, locked := tx.Statement.Clauses["FOR"]; locked && tx.Statement.Table == "job_queue" {
			calls++
			tx.AddError(gorm.ErrRecordNotFound)
		}
	}))
	started := time.Now()
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.JobQueue{}).Where("id = ?", original.ID).Update("correlation_id", "uncommitted-change").Error; err != nil {
			return err
		}
		_, err := runtime.EnqueueCoalescedTx(context.Background(), tx, input)
		return err
	})
	require.ErrorIs(t, err, ErrCoalescingContention)
	class, classified := reliability.ErrorClassOf(err)
	require.True(t, classified)
	require.Equal(t, reliability.ClassRetryable, class)
	require.Equal(t, 8, calls)
	require.GreaterOrEqual(t, time.Since(started), 81*time.Millisecond)
	require.NoError(t, db.Callback().Query().Remove("coalesced_vanished_key"))
	var stored models.JobQueue
	require.NoError(t, db.First(&stored, "id = ?", original.ID).Error)
	require.Empty(t, stored.CorrelationID)
	require.Equal(t, original.PayloadJSON, stored.PayloadJSON)
}

func TestCoalescedRetryHonorsCancellationAndDoesNotRetrySQLFailure(t *testing.T) {
	for _, canceled := range []bool{true, false} {
		t.Run(map[bool]string{true: "canceled", false: "database_failure"}[canceled], func(t *testing.T) {
			runtime, db, _ := newTestRuntime(t, nil)
			input := EnqueueInput{JobType: "test.coalesced", Payload: map[string]int{"version": 1}, IdempotencyKey: "entity:1:pending"}
			_, err := runtime.Enqueue(context.Background(), input)
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			failure := errors.New("database statement failed")
			calls := 0
			require.NoError(t, db.Callback().Query().Before("gorm:query").Register("coalesced_retry_failure", func(tx *gorm.DB) {
				if _, locked := tx.Statement.Clauses["FOR"]; locked && strings.Contains(tx.Statement.Table, "job_queue") {
					calls++
					if canceled {
						cancel()
						tx.AddError(gorm.ErrRecordNotFound)
					} else {
						tx.AddError(failure)
					}
				}
			}))
			err = db.Transaction(func(tx *gorm.DB) error { _, err := runtime.EnqueueCoalescedTx(ctx, tx, input); return err })
			if canceled {
				require.ErrorIs(t, err, context.Canceled)
			} else {
				require.ErrorIs(t, err, failure)
			}
			require.Equal(t, 1, calls)
		})
	}
}

func TestCoalescingBackoffWaitIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	waiting := make(chan error, 1)
	go func() { waiting <- waitCoalescingRetry(ctx, 7) }()
	cancel()
	select {
	case err := <-waiting:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("coalescing backoff ignored cancellation")
	}
}
