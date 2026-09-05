package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ecommerce/internal/reliability"
	"ecommerce/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTestRuntime(t *testing.T, configure func(*Config)) (*Runtime, *gorm.DB, *time.Time) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_busy_timeout=5000", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.JobQueue{}, &models.JobAttempt{}, &models.JobDeadLetter{}))
	now := time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC)
	config := Config{
		WorkerConcurrency: 4, PollInterval: 5 * time.Millisecond, LeaseDuration: time.Minute,
		MaxAttempts: 5, RetryBaseDelay: time.Second, RetryMaxDelay: time.Minute,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: func() time.Time { return now },
		Jitter: func(max time.Duration) time.Duration { return max },
	}
	if configure != nil {
		configure(&config)
	}
	return NewRuntime(db, config), db, &now
}

func TestEnqueueIsIdempotentAndRejectsChangedPayload(t *testing.T) {
	runtime, db, _ := newTestRuntime(t, nil)
	ctx := reliability.WithCorrelation(context.Background(), reliability.Correlation{CorrelationID: "correlation-1"})
	first, err := runtime.Enqueue(ctx, EnqueueInput{JobType: "test.email", IdempotencyKey: "order:1", Payload: map[string]any{"order_id": 1}})
	require.NoError(t, err)
	second, err := runtime.Enqueue(ctx, EnqueueInput{JobType: "test.email", IdempotencyKey: "order:1", Payload: json.RawMessage(`{"order_id":1}`), MaxAttempts: 9, RunAt: time.Now().Add(time.Hour)})
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID)
	assert.Equal(t, "correlation-1", first.CorrelationID)

	_, err = runtime.Enqueue(ctx, EnqueueInput{JobType: "test.email", IdempotencyKey: "order:1", Payload: map[string]any{"order_id": 2}})
	assert.ErrorIs(t, err, ErrIdempotencyConflict)
	var count int64
	require.NoError(t, db.Model(&models.JobQueue{}).Count(&count).Error)
	assert.EqualValues(t, 1, count)
}

func TestScheduledJobIsNotClaimedEarly(t *testing.T) {
	runtime, _, now := newTestRuntime(t, nil)
	job, err := runtime.Enqueue(context.Background(), EnqueueInput{JobType: "test.scheduled", Payload: map[string]any{}, RunAt: now.Add(time.Hour)})
	require.NoError(t, err)
	_, err = runtime.claim(context.Background(), job.ID, "worker-1")
	assert.ErrorIs(t, err, ErrNoRunnableJob)

	*now = now.Add(2 * time.Hour)
	claimed, err := runtime.claim(context.Background(), job.ID, "worker-1")
	require.NoError(t, err)
	assert.Equal(t, models.JobStatusRunning, claimed.Status)
}

func TestRetryExhaustionCreatesAttemptsAndDeadLetter(t *testing.T) {
	runtime, db, now := newTestRuntime(t, func(config *Config) {
		config.MaxAttempts = 2
		config.Jitter = func(time.Duration) time.Duration { return 0 }
	})
	var calls atomic.Int32
	var hookCalls atomic.Int32
	require.NoError(t, runtime.Register("test.retry", Registration{
		Handle: func(context.Context, json.RawMessage) error {
			calls.Add(1)
			return reliability.Classify(errors.New("temporary failure"), reliability.ClassRetryable)
		},
		OnDeadLetter: func(_ context.Context, tx *gorm.DB, _ json.RawMessage, _ error) error {
			hookCalls.Add(1)
			return tx.Exec("SELECT 1").Error
		},
	}))
	job, err := runtime.Enqueue(context.Background(), EnqueueInput{JobType: "test.retry", Payload: map[string]any{}})
	require.NoError(t, err)
	require.Error(t, runtime.Execute(context.Background(), job.ID, "worker-1"))
	*now = now.Add(time.Second)
	require.Error(t, runtime.Execute(context.Background(), job.ID, "worker-1"))

	require.NoError(t, db.First(&job, "id = ?", job.ID).Error)
	assert.Equal(t, models.JobStatusDeadLetter, job.Status)
	assert.Equal(t, int32(2), calls.Load())
	assert.Equal(t, int32(1), hookCalls.Load())
	var attempts []models.JobAttempt
	require.NoError(t, db.Where("job_id = ?", job.ID).Order("attempt_number").Find(&attempts).Error)
	require.Len(t, attempts, 2)
	assert.Equal(t, models.JobAttemptOutcomeRetryable, attempts[0].Outcome)
	var deadLetter models.JobDeadLetter
	require.NoError(t, db.First(&deadLetter, "job_id = ?", job.ID).Error)
	assert.Equal(t, 2, deadLetter.AttemptCount)
	assert.Equal(t, string(reliability.ClassRetryable), deadLetter.ErrorClass)
}

func TestUnclassifiedFailureAndPanicAreTerminal(t *testing.T) {
	for _, test := range []struct {
		name    string
		handler Handler
	}{
		{name: "unclassified", handler: func(context.Context, json.RawMessage) error { return errors.New("invalid input") }},
		{name: "panic", handler: func(context.Context, json.RawMessage) error { panic("secret") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			runtime, db, _ := newTestRuntime(t, nil)
			require.NoError(t, runtime.Register("test.terminal", Registration{Handle: test.handler}))
			job, err := runtime.Enqueue(context.Background(), EnqueueInput{JobType: "test.terminal", Payload: map[string]any{}})
			require.NoError(t, err)
			require.Error(t, runtime.Execute(context.Background(), job.ID, "worker-1"))
			require.NoError(t, db.First(&job, "id = ?", job.ID).Error)
			assert.Equal(t, models.JobStatusDeadLetter, job.Status)
			var attempt models.JobAttempt
			require.NoError(t, db.First(&attempt, "job_id = ?", job.ID).Error)
			assert.Equal(t, string(reliability.ClassTerminal), attempt.ErrorClass)
			if test.name == "panic" {
				assert.NotContains(t, attempt.ErrorMessage, "secret")
			}
		})
	}
}

func TestMissingHandlerIsTerminal(t *testing.T) {
	runtime, db, _ := newTestRuntime(t, nil)
	job, err := runtime.Enqueue(context.Background(), EnqueueInput{JobType: "test.missing", Payload: map[string]any{"version": 1}})
	require.NoError(t, err)
	require.Error(t, runtime.Execute(context.Background(), job.ID, "worker-1"))
	require.NoError(t, db.First(&job, "id = ?", job.ID).Error)
	assert.Equal(t, models.JobStatusDeadLetter, job.Status)
	var attempt models.JobAttempt
	require.NoError(t, db.First(&attempt, "job_id = ?", job.ID).Error)
	assert.Equal(t, string(reliability.ClassTerminal), attempt.ErrorClass)
}

func TestExpiredLeaseIsRecordedAndReclaimed(t *testing.T) {
	runtime, db, now := newTestRuntime(t, nil)
	require.NoError(t, runtime.Register("test.restart", Registration{Handle: func(context.Context, json.RawMessage) error { return nil }}))
	job, err := runtime.Enqueue(context.Background(), EnqueueInput{JobType: "test.restart", Payload: map[string]any{}})
	require.NoError(t, err)
	first, err := runtime.claim(context.Background(), job.ID, "dead-worker")
	require.NoError(t, err)
	assert.Equal(t, 1, first.AttemptCount)

	*now = now.Add(2 * time.Minute)
	second, err := runtime.claim(context.Background(), job.ID, "new-worker")
	require.NoError(t, err)
	assert.Equal(t, 2, second.AttemptCount)
	require.NoError(t, runtime.executeClaimed(context.Background(), second, "new-worker"))

	var attempts []models.JobAttempt
	require.NoError(t, db.Where("job_id = ?", job.ID).Order("attempt_number").Find(&attempts).Error)
	require.Len(t, attempts, 2)
	assert.Equal(t, models.JobAttemptOutcomeLeaseExpired, attempts[0].Outcome)
	assert.Equal(t, models.JobAttemptOutcomeSucceeded, attempts[1].Outcome)
}

func TestExpiredFinalLeaseDeadLettersWithoutAnotherExecution(t *testing.T) {
	runtime, db, now := newTestRuntime(t, nil)
	var calls atomic.Int32
	var hookCalls atomic.Int32
	require.NoError(t, runtime.Register("test.expired-final", Registration{
		Handle: func(context.Context, json.RawMessage) error {
			calls.Add(1)
			return nil
		},
		OnDeadLetter: func(context.Context, *gorm.DB, json.RawMessage, error) error {
			hookCalls.Add(1)
			return nil
		},
	}))
	job, err := runtime.Enqueue(context.Background(), EnqueueInput{JobType: "test.expired-final", MaxAttempts: 1, Payload: map[string]any{}})
	require.NoError(t, err)
	_, err = runtime.claim(context.Background(), job.ID, "dead-worker")
	require.NoError(t, err)

	*now = now.Add(2 * time.Minute)
	err = runtime.Execute(context.Background(), job.ID, "new-worker")
	assert.ErrorIs(t, err, ErrNoRunnableJob)
	assert.Zero(t, calls.Load())
	assert.Equal(t, int32(1), hookCalls.Load())

	require.NoError(t, db.First(&job, "id = ?", job.ID).Error)
	assert.Equal(t, models.JobStatusDeadLetter, job.Status)
	var attempt models.JobAttempt
	require.NoError(t, db.First(&attempt, "job_id = ?", job.ID).Error)
	assert.Equal(t, models.JobAttemptOutcomeLeaseExpired, attempt.Outcome)
	var deadLetter models.JobDeadLetter
	require.NoError(t, db.First(&deadLetter, "job_id = ?", job.ID).Error)
	assert.Equal(t, 1, deadLetter.AttemptCount)
}

func TestConcurrentClaimsChooseOneWorker(t *testing.T) {
	runtime, _, _ := newTestRuntime(t, nil)
	job, err := runtime.Enqueue(context.Background(), EnqueueInput{JobType: "test.concurrent", Payload: map[string]any{}})
	require.NoError(t, err)

	const contenders = 12
	var successes atomic.Int32
	var unexpected atomic.Int32
	var wait sync.WaitGroup
	start := make(chan struct{})
	for index := 0; index < contenders; index++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			<-start
			_, claimErr := runtime.claim(context.Background(), job.ID, fmt.Sprintf("worker-%d", worker))
			if claimErr == nil {
				successes.Add(1)
				return
			}
			if !errors.Is(claimErr, ErrNoRunnableJob) && !stringsContainsLock(claimErr.Error()) {
				unexpected.Add(1)
			}
		}(index)
	}
	close(start)
	wait.Wait()
	assert.Equal(t, int32(1), successes.Load())
	assert.Zero(t, unexpected.Load())
}

func TestWorkerPoolExecutesClaimedJobOnce(t *testing.T) {
	runtime, db, _ := newTestRuntime(t, nil)
	var calls atomic.Int32
	completed := make(chan struct{}, 1)
	require.NoError(t, runtime.Register("test.pool", Registration{Handle: func(context.Context, json.RawMessage) error {
		calls.Add(1)
		completed <- struct{}{}
		return nil
	}}))
	job, err := runtime.Enqueue(context.Background(), EnqueueInput{JobType: "test.pool", Payload: map[string]any{}})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runtime.Run(ctx)
		close(done)
	}()
	select {
	case <-completed:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not execute job")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker pool did not stop")
	}

	require.Eventually(t, func() bool {
		return db.First(&job, "id = ?", job.ID).Error == nil && job.Status == models.JobStatusSucceeded
	}, time.Second, 5*time.Millisecond)
	assert.Equal(t, int32(1), calls.Load())
}

type recordingObserver struct {
	started  atomic.Int32
	stopped  atomic.Int32
	claimed  atomic.Int32
	attempts atomic.Int32
}

func (o *recordingObserver) WorkerStarted() { o.started.Add(1) }
func (o *recordingObserver) WorkerStopped() { o.stopped.Add(1) }
func (o *recordingObserver) JobClaimed(string) {
	o.claimed.Add(1)
}
func (o *recordingObserver) AttemptCompleted(string, string, string, time.Duration) {
	o.attempts.Add(1)
}

func TestWorkerLifecycleAndAttemptsNotifyObserver(t *testing.T) {
	observer := &recordingObserver{}
	runtime, _, _ := newTestRuntime(t, func(config *Config) {
		config.WorkerConcurrency = 1
		config.Observer = observer
	})
	completed := make(chan struct{}, 1)
	require.NoError(t, runtime.Register("test.observed", Registration{Handle: func(context.Context, json.RawMessage) error {
		completed <- struct{}{}
		return nil
	}}))
	_, err := runtime.Enqueue(context.Background(), EnqueueInput{JobType: "test.observed", Payload: map[string]any{}})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		runtime.Run(ctx)
		close(done)
	}()
	require.Eventually(t, runtime.Running, time.Second, time.Millisecond)
	select {
	case <-completed:
	case <-time.After(time.Second):
		t.Fatal("observed job did not complete")
	}
	require.Eventually(t, func() bool { return observer.attempts.Load() == 1 }, time.Second, time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("observed worker did not stop")
	}

	assert.False(t, runtime.Running())
	assert.Equal(t, int32(1), observer.started.Load())
	assert.Equal(t, int32(1), observer.stopped.Load())
	assert.Equal(t, int32(1), observer.claimed.Load())
	assert.Equal(t, int32(1), observer.attempts.Load())
}

func TestRunningJobRenewsItsLease(t *testing.T) {
	runtime, db, _ := newTestRuntime(t, func(config *Config) {
		config.LeaseDuration = 60 * time.Millisecond
		config.Now = time.Now
	})
	started := make(chan struct{})
	release := make(chan struct{})
	require.NoError(t, runtime.Register("test.heartbeat", Registration{Handle: func(context.Context, json.RawMessage) error {
		close(started)
		<-release
		return nil
	}}))
	job, err := runtime.Enqueue(context.Background(), EnqueueInput{JobType: "test.heartbeat", Payload: map[string]any{}})
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() { done <- runtime.Execute(context.Background(), job.ID, "worker-1") }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}

	var claimed models.JobQueue
	require.NoError(t, db.First(&claimed, "id = ?", job.ID).Error)
	require.NotNil(t, claimed.LeaseExpiresAt)
	initialExpiry := *claimed.LeaseExpiresAt
	require.Eventually(t, func() bool {
		var current models.JobQueue
		return db.First(&current, "id = ?", job.ID).Error == nil && current.LeaseExpiresAt != nil && current.LeaseExpiresAt.After(initialExpiry)
	}, time.Second, 5*time.Millisecond)

	close(release)
	require.NoError(t, <-done)
}

func stringsContainsLock(text string) bool {
	return strings.Contains(strings.ToLower(text), "locked")
}
