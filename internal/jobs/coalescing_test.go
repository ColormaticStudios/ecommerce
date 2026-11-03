package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"ecommerce/internal/reliability"
	"ecommerce/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCoalescedKeyRetainedThroughRetriesAndReleasedAtDeadLetter(t *testing.T) {
	runtime, db, now := newTestRuntime(t, func(config *Config) {
		config.MaxAttempts = 2
		config.Jitter = func(time.Duration) time.Duration { return 0 }
	})
	require.NoError(t, runtime.Register("test.coalesced", Registration{ReleaseKeyOnCompletion: true, Handle: func(context.Context, json.RawMessage) error {
		return reliability.Classify(errors.New("temporary"), reliability.ClassRetryable)
	}}))
	ctx := context.Background()
	input := EnqueueInput{JobType: "test.coalesced", Payload: map[string]int{"version": 1}, IdempotencyKey: "entity:1:pending"}
	var first models.JobQueue
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		var err error
		first, err = runtime.EnqueueCoalescedTx(ctx, tx, input)
		return err
	}))
	require.Error(t, runtime.Execute(ctx, first.ID, "coalesced-test"))
	var retry models.JobQueue
	require.NoError(t, db.First(&retry, "id = ?", first.ID).Error)
	require.Equal(t, models.JobStatusRetryScheduled, retry.Status)
	require.NotNil(t, retry.IdempotencyKey)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		joined, err := runtime.EnqueueCoalescedTx(ctx, tx, input)
		require.Equal(t, first.ID, joined.ID)
		return err
	}))
	*now = now.Add(time.Second)
	require.Error(t, runtime.Execute(ctx, first.ID, "coalesced-test"))
	var failed models.JobQueue
	require.NoError(t, db.First(&failed, "id = ?", first.ID).Error)
	require.Equal(t, models.JobStatusDeadLetter, failed.Status)
	require.Nil(t, failed.IdempotencyKey)
	var successor models.JobQueue
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		var err error
		successor, err = runtime.EnqueueCoalescedTx(ctx, tx, input)
		return err
	}))
	require.NotEqual(t, first.ID, successor.ID)
	require.Zero(t, successor.AttemptCount)
	var attempts int64
	require.NoError(t, db.Model(&models.JobAttempt{}).Where("job_id = ?", first.ID).Count(&attempts).Error)
	require.EqualValues(t, 2, attempts)
	var dead models.JobDeadLetter
	require.NoError(t, db.First(&dead, "job_id = ?", first.ID).Error)
}

func TestCoalescedSuccessHookFailureAndLeaseFencingAreAtomic(t *testing.T) {
	runtime, db, now := newTestRuntime(t, nil)
	ctx := context.Background()
	job, err := runtime.Enqueue(ctx, EnqueueInput{JobType: "test.coalesced", Payload: map[string]int{"version": 1}, IdempotencyKey: "entity:1:pending"})
	require.NoError(t, err)
	claimed, err := runtime.claim(ctx, job.ID, "owner")
	require.NoError(t, err)
	called := false
	registration := Registration{ReleaseKeyOnCompletion: true, OnSuccess: func(context.Context, *gorm.DB, models.JobQueue) error {
		called = true
		return errors.New("hook failure")
	}}
	_, err = runtime.finalize(ctx, claimed, "owner", *now, nil, registration)
	require.ErrorContains(t, err, "hook failure")
	require.True(t, called)
	var stored models.JobQueue
	require.NoError(t, db.First(&stored, "id = ?", job.ID).Error)
	require.Equal(t, models.JobStatusRunning, stored.Status)
	require.NotNil(t, stored.IdempotencyKey)
	var attempts int64
	require.NoError(t, db.Model(&models.JobAttempt{}).Count(&attempts).Error)
	require.Zero(t, attempts)
	called = false
	_, err = runtime.finalize(ctx, claimed, "different-owner", *now, nil, registration)
	require.ErrorIs(t, err, ErrLeaseLost)
	require.False(t, called)
}

func TestCoalescedExpiredFinalLeaseReleasesKey(t *testing.T) {
	runtime, db, now := newTestRuntime(t, func(config *Config) { config.MaxAttempts = 1 })
	require.NoError(t, runtime.Register("test.expired", Registration{ReleaseKeyOnCompletion: true, Handle: func(context.Context, json.RawMessage) error { return nil }}))
	ctx := context.Background()
	input := EnqueueInput{JobType: "test.expired", Payload: map[string]int{"version": 1}, IdempotencyKey: "variant:1:pending"}
	var job models.JobQueue
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		var err error
		job, err = runtime.EnqueueCoalescedTx(ctx, tx, input)
		return err
	}))
	_, err := runtime.claim(ctx, job.ID, "expired-owner")
	require.NoError(t, err)
	*now = now.Add(2 * time.Minute)
	_, err = runtime.claim(ctx, job.ID, "replacement-owner")
	require.ErrorIs(t, err, ErrNoRunnableJob)
	var failed models.JobQueue
	require.NoError(t, db.First(&failed, "id = ?", job.ID).Error)
	require.Equal(t, models.JobStatusDeadLetter, failed.Status)
	require.Nil(t, failed.IdempotencyKey)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		replacement, err := runtime.EnqueueCoalescedTx(ctx, tx, input)
		require.NotEqual(t, job.ID, replacement.ID)
		return err
	}))
}

func TestCoalescedPendingKeyNormalization(t *testing.T) {
	runtime, db, _ := newTestRuntime(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	input := EnqueueInput{JobType: "test.coalesced", Payload: map[string]int{"version": 1}, IdempotencyKey: "  entity:1:pending\t"}
	var first models.JobQueue
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		var err error
		first, err = runtime.EnqueueCoalescedTx(ctx, tx, input)
		return err
	}))
	require.Equal(t, "entity:1:pending", *first.IdempotencyKey)
	input.IdempotencyKey = "entity:1:pending"
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		joined, err := runtime.EnqueueCoalescedTx(ctx, tx, input)
		require.Equal(t, first.ID, joined.ID)
		return err
	}))
	var count int64
	require.NoError(t, db.Model(&models.JobQueue{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}
