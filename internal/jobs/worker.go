package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"ecommerce/internal/reliability"
	"ecommerce/models"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

func (r *Runtime) Run(ctx context.Context) {
	if r == nil || r.db == nil {
		return
	}
	instanceID := uuid.NewString()
	var workers sync.WaitGroup
	for index := 0; index < r.config.WorkerConcurrency; index++ {
		workers.Add(1)
		go func(workerID string) {
			defer workers.Done()
			r.running.Add(1)
			if r.config.Observer != nil {
				r.config.Observer.WorkerStarted()
			}
			defer func() {
				r.running.Add(-1)
				if r.config.Observer != nil {
					r.config.Observer.WorkerStopped()
				}
			}()
			r.runWorker(ctx, workerID)
		}(fmt.Sprintf("%s-%d", instanceID, index+1))
	}
	workers.Wait()
}

func (r *Runtime) runWorker(ctx context.Context, workerID string) {
	ticker := time.NewTicker(r.config.PollInterval)
	defer ticker.Stop()
	for {
		for {
			job, err := r.claim(ctx, "", workerID)
			if errors.Is(err, ErrNoRunnableJob) {
				break
			}
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				r.config.Logger.ErrorContext(ctx, "Job claim failed", "event", "job_claim_failed", "worker_id", workerID, "error_type", fmt.Sprintf("%T", err))
				break
			}
			_ = r.executeClaimed(ctx, job, workerID)
			if ctx.Err() != nil {
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-r.wake:
		}
	}
}

// Execute claims and executes one specific runnable job. It is used by
// synchronous callers such as CLI media imports while retaining durable job
// and attempt records.
func (r *Runtime) Execute(ctx context.Context, jobID, workerID string) error {
	if strings.TrimSpace(workerID) == "" {
		workerID = "inline-" + uuid.NewString()
	}
	job, err := r.claim(ctx, strings.TrimSpace(jobID), workerID)
	if err != nil {
		return err
	}
	return r.executeClaimed(ctx, job, workerID)
}

func (r *Runtime) claim(ctx context.Context, jobID, workerID string) (models.JobQueue, error) {
	if r == nil || r.db == nil {
		return models.JobQueue{}, errors.New("job runtime database is required")
	}
	// Serialize the short claim transaction within one process. Database CAS
	// predicates still coordinate separate instances, while this prevents
	// SQLite readers in sibling workers from deadlocking every contender.
	r.claimMu.Lock()
	defer r.claimMu.Unlock()
	now := r.config.Now().UTC()
	var claimed models.JobQueue
	var exhausted *executionResult
	var exhaustedContext context.Context
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&models.JobQueue{}).
			Where("((status IN ? AND run_at <= ?) OR (status = ? AND lease_expires_at IS NOT NULL AND lease_expires_at <= ?))",
				[]string{models.JobStatusPending, models.JobStatusRetryScheduled}, now, models.JobStatusRunning, now)
		if jobID != "" {
			query = query.Where("id = ?", jobID)
		}
		var candidate models.JobQueue
		if err := query.Order("run_at ASC, created_at ASC, id ASC").First(&candidate).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNoRunnableJob
			}
			return err
		}

		if candidate.Status == models.JobStatusRunning && candidate.AttemptCount > 0 {
			startedAt := candidate.CreatedAt
			if candidate.AttemptStartedAt != nil {
				startedAt = *candidate.AttemptStartedAt
			}
			attempt := models.JobAttempt{
				JobID: candidate.ID, JobType: candidate.JobType, AttemptNumber: candidate.AttemptCount,
				WorkerID: candidate.LeaseOwner, Outcome: models.JobAttemptOutcomeLeaseExpired,
				ErrorClass: string(reliability.ClassRetryable), ErrorMessage: "job lease expired",
				StartedAt: startedAt, FinishedAt: now, LatencyMs: now.Sub(startedAt).Milliseconds(),
			}
			if err := tx.Create(&attempt).Error; err != nil {
				return err
			}
			if candidate.AttemptCount >= candidate.MaxAttempts {
				failure := reliability.Classify(errors.New("job lease expired after final attempt"), reliability.ClassRetryable)
				correlation := reliability.Correlation{RequestID: uuid.NewString(), CorrelationID: candidate.CorrelationID}
				if correlation.CorrelationID == "" {
					correlation.CorrelationID = correlation.RequestID
				}
				exhaustedContext = reliability.WithCorrelation(ctx, correlation)
				update := tx.Model(&models.JobQueue{}).
					Where("id = ? AND status = ? AND lease_owner = ? AND attempt_count = ? AND lease_expires_at IS NOT NULL AND lease_expires_at <= ?",
						candidate.ID, models.JobStatusRunning, candidate.LeaseOwner, candidate.AttemptCount, now).
					Updates(map[string]any{
						"status": models.JobStatusDeadLetter, "lease_owner": "", "lease_expires_at": nil,
						"attempt_started_at": nil, "completed_at": &now, "last_error": failure.Error(),
					})
				if update.Error != nil {
					return update.Error
				}
				if update.RowsAffected != 1 {
					return ErrNoRunnableJob
				}
				deadLetter := models.JobDeadLetter{
					JobID: candidate.ID, JobType: candidate.JobType, PayloadJSON: candidate.PayloadJSON,
					CorrelationID: candidate.CorrelationID, AttemptCount: candidate.AttemptCount,
					ErrorClass: string(reliability.ClassRetryable), FailureReason: failure.Error(), FailedAt: now,
				}
				if err := tx.Create(&deadLetter).Error; err != nil {
					return err
				}
				registration, registered := r.registration(candidate.JobType)
				if registered && registration.OnDeadLetter != nil {
					if err := registration.OnDeadLetter(exhaustedContext, tx, json.RawMessage(candidate.PayloadJSON), failure); err != nil {
						return fmt.Errorf("dead-letter hook: %w", err)
					}
				}
				candidate.Status = models.JobStatusDeadLetter
				candidate.CompletedAt = &now
				candidate.LastError = failure.Error()
				exhausted = &executionResult{job: candidate, attempt: attempt, deadLettered: true}
				return nil
			}
		}

		expiresAt := now.Add(r.config.LeaseDuration)
		query = tx.Model(&models.JobQueue{}).
			Where("id = ? AND status = ? AND attempt_count = ?", candidate.ID, candidate.Status, candidate.AttemptCount)
		if candidate.Status == models.JobStatusRunning {
			query = query.Where("lease_expires_at IS NOT NULL AND lease_expires_at <= ?", now)
		} else {
			query = query.Where("run_at <= ?", now)
		}
		result := query.Updates(map[string]any{
			"status": models.JobStatusRunning, "attempt_count": gorm.Expr("attempt_count + 1"),
			"lease_owner": workerID, "lease_expires_at": &expiresAt, "attempt_started_at": &now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrNoRunnableJob
		}
		if err := tx.First(&claimed, "id = ?", candidate.ID).Error; err != nil {
			return err
		}
		return nil
	})
	if err == nil && exhausted != nil {
		r.logCompletion(exhaustedContext, *exhausted)
		return models.JobQueue{}, ErrNoRunnableJob
	}
	if err == nil && r.config.Observer != nil {
		r.config.Observer.JobClaimed(claimed.JobType)
	}
	return claimed, err
}

func (r *Runtime) executeClaimed(ctx context.Context, job models.JobQueue, workerID string) error {
	startedAt := r.config.Now().UTC()
	if job.AttemptStartedAt != nil {
		startedAt = *job.AttemptStartedAt
	}
	correlation := reliability.Correlation{RequestID: uuid.NewString(), CorrelationID: job.CorrelationID}
	if correlation.CorrelationID == "" {
		correlation.CorrelationID = correlation.RequestID
	}
	executionContext, cancel := context.WithCancel(reliability.WithCorrelation(ctx, correlation))
	executionContext, span := otel.Tracer("ecommerce/jobs").Start(
		executionContext,
		job.JobType,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("job.id", job.ID),
			attribute.String("job.type", job.JobType),
			attribute.Int("job.attempt", job.AttemptCount),
			attribute.String("correlation.id", correlation.CorrelationID),
		),
	)
	defer span.End()
	stopHeartbeat := make(chan struct{})
	heartbeatDone := make(chan bool, 1)
	go r.heartbeat(executionContext, job, workerID, stopHeartbeat, heartbeatDone, cancel)

	registration, registered := r.registration(job.JobType)
	var handlerErr error
	if !registered {
		handlerErr = reliability.Classify(fmt.Errorf("no handler registered for job type %s", job.JobType), reliability.ClassTerminal)
	} else {
		handlerErr = invokeHandler(executionContext, registration.Handle, json.RawMessage(job.PayloadJSON))
	}
	close(stopHeartbeat)
	leaseLost := <-heartbeatDone
	cancel()
	if leaseLost {
		span.SetStatus(codes.Error, "lease lost")
		span.SetAttributes(attribute.String("job.outcome", models.JobAttemptOutcomeLeaseExpired))
		return ErrLeaseLost
	}
	if errors.Is(handlerErr, context.Canceled) && ctx.Err() != nil {
		handlerErr = reliability.Classify(handlerErr, reliability.ClassRetryable)
	}

	finalizeContext, finalizeCancel := context.WithTimeout(context.WithoutCancel(executionContext), 10*time.Second)
	defer finalizeCancel()
	result, finalizeErr := r.finalize(finalizeContext, job, workerID, startedAt, handlerErr, registration)
	if finalizeErr != nil {
		span.SetStatus(codes.Error, "finalization failed")
		r.config.Logger.ErrorContext(executionContext, "Job finalization failed",
			"event", "job_finalization_failed", "job_id", job.ID, "job_type", job.JobType,
			"attempt", job.AttemptCount, "correlation_id", correlation.CorrelationID,
			"error_type", fmt.Sprintf("%T", finalizeErr),
		)
		return finalizeErr
	}
	r.logCompletion(executionContext, result)
	span.SetAttributes(attribute.String("job.outcome", result.attempt.Outcome))
	if result.attempt.ErrorClass != "" {
		span.SetAttributes(attribute.String("error.type", result.attempt.ErrorClass))
	}
	if result.deadLettered || result.retryScheduled {
		span.SetStatus(codes.Error, result.attempt.Outcome)
	}
	if result.retryScheduled {
		r.notify()
	}
	if class, ok := reliability.ErrorClassOf(handlerErr); ok && class == reliability.ClassDegraded {
		return nil
	}
	return handlerErr
}

func invokeHandler(ctx context.Context, handler Handler, payload json.RawMessage) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = reliability.Classify(fmt.Errorf("job handler panic (%T)", recovered), reliability.ClassTerminal)
		}
	}()
	return handler(ctx, payload)
}

func (r *Runtime) heartbeat(ctx context.Context, job models.JobQueue, workerID string, stop <-chan struct{}, done chan<- bool, cancel context.CancelFunc) {
	interval := r.config.LeaseDuration / 3
	if interval < 10*time.Millisecond {
		interval = 10 * time.Millisecond
	}
	confirmedExpiry := r.config.Now().UTC().Add(r.config.LeaseDuration)
	if job.LeaseExpiresAt != nil {
		confirmedExpiry = job.LeaseExpiresAt.UTC()
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			done <- false
			return
		case <-ctx.Done():
			done <- false
			return
		case <-ticker.C:
			now := r.config.Now().UTC()
			expiresAt := now.Add(r.config.LeaseDuration)
			heartbeatContext, heartbeatCancel := context.WithTimeout(context.Background(), interval)
			result := r.db.WithContext(heartbeatContext).Model(&models.JobQueue{}).
				Where("id = ? AND status = ? AND lease_owner = ? AND attempt_count = ?", job.ID, models.JobStatusRunning, workerID, job.AttemptCount).
				Updates(map[string]any{"lease_expires_at": &expiresAt})
			heartbeatCancel()
			if result.Error == nil && result.RowsAffected == 1 {
				confirmedExpiry = expiresAt
				continue
			}
			if result.Error == nil || !now.Before(confirmedExpiry) {
				cancel()
				done <- true
				return
			}
		}
	}
}
