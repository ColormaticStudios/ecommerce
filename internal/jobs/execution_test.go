package jobs

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"ecommerce/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCheckpointExecutionContextComesFromRuntime(t *testing.T) {
	runtime, db, _ := newTestRuntime(t, nil)
	require.NoError(t, runtime.Register("test.checkpoint", Registration{Handle: func(ctx context.Context, _ json.RawMessage) error {
		return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			job, err := LockExecutionTx(ctx, tx)
			require.NoError(t, err)
			require.Equal(t, "test.checkpoint", job.JobType)
			require.Equal(t, "checkpoint-worker", job.LeaseOwner)
			require.Equal(t, 1, job.AttemptCount)
			return err
		})
	}}))
	job, err := runtime.Enqueue(context.Background(), EnqueueInput{JobType: "test.checkpoint"})
	require.NoError(t, err)
	require.NoError(t, runtime.Execute(context.Background(), job.ID, "checkpoint-worker"))
	_, err = LockExecutionTx(context.Background(), db)
	require.ErrorIs(t, err, ErrMissingExecution)
}

func TestCheckpointRejectsLostOrExpiredExecution(t *testing.T) {
	for _, change := range []string{"owner", "attempt", "status", "expiry", "missing_expiry", "missing_row"} {
		t.Run(change, func(t *testing.T) {
			runtime, db, _ := newTestRuntime(t, nil)
			job, err := runtime.Enqueue(context.Background(), EnqueueInput{JobType: "test.checkpoint"})
			require.NoError(t, err)
			claimed, err := runtime.claim(context.Background(), job.ID, "original-worker")
			require.NoError(t, err)
			ctx := withExecution(context.Background(), claimed, claimed.LeaseOwner, runtime.config.Now)
			switch change {
			case "owner":
				require.NoError(t, db.Model(&job).Update("lease_owner", "successor-worker").Error)
			case "attempt":
				require.NoError(t, db.Model(&job).Update("attempt_count", claimed.AttemptCount+1).Error)
			case "status":
				require.NoError(t, db.Model(&job).Update("status", models.JobStatusSucceeded).Error)
			case "expiry":
				require.NoError(t, db.Model(&job).Update("lease_expires_at", runtime.config.Now()).Error)
			case "missing_expiry":
				require.NoError(t, db.Model(&job).Update("lease_expires_at", nil).Error)
			case "missing_row":
				require.NoError(t, db.Delete(&job).Error)
			}
			err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				_, err := LockExecutionTx(ctx, tx)
				return err
			})
			require.ErrorIs(t, err, ErrLeaseLost)
		})
	}
}

func TestCheckpointRecheckRollsBackEffectsAfterLeaseExpiry(t *testing.T) {
	runtime, db, now := newTestRuntime(t, nil)
	job, err := runtime.Enqueue(context.Background(), EnqueueInput{JobType: "test.checkpoint"})
	require.NoError(t, err)
	claimed, err := runtime.claim(context.Background(), job.ID, "checkpoint-worker")
	require.NoError(t, err)
	ctx := withExecution(context.Background(), claimed, claimed.LeaseOwner, runtime.config.Now)
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		_, err := LockExecutionTx(ctx, tx)
		require.NoError(t, err)
		require.NoError(t, tx.Model(&job).Update("last_error", "effect to roll back").Error)
		*now = now.Add(time.Hour)
		_, err = LockExecutionTx(ctx, tx)
		return err
	})
	require.ErrorIs(t, err, ErrLeaseLost)
	require.NoError(t, db.First(&job, "id = ?", job.ID).Error)
	require.Empty(t, job.LastError)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = LockExecutionTx(canceled, db)
	require.ErrorIs(t, err, context.Canceled)
}
