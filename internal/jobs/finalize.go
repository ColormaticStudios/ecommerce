package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"ecommerce/internal/reliability"
	"ecommerce/models"
	"gorm.io/gorm"
)

type executionResult struct {
	job            models.JobQueue
	attempt        models.JobAttempt
	retryScheduled bool
	deadLettered   bool
}

func (r *Runtime) finalize(ctx context.Context, claimed models.JobQueue, workerID string, startedAt time.Time, handlerErr error, registration Registration) (executionResult, error) {
	finishedAt := r.config.Now().UTC()
	result := executionResult{job: claimed}
	class, classified := reliability.ErrorClassOf(handlerErr)
	if handlerErr != nil && !classified {
		class = reliability.ClassTerminal
		handlerErr = reliability.Classify(handlerErr, class)
	}
	outcome := models.JobAttemptOutcomeSucceeded
	status := models.JobStatusSucceeded
	completedAt := &finishedAt
	runAt := claimed.RunAt
	if handlerErr != nil {
		switch class {
		case reliability.ClassDegraded:
			outcome = models.JobAttemptOutcomeDegraded
		case reliability.ClassRetryable:
			outcome = models.JobAttemptOutcomeRetryable
			if claimed.AttemptCount < claimed.MaxAttempts {
				status = models.JobStatusRetryScheduled
				completedAt = nil
				runAt = finishedAt.Add(r.retryDelay(claimed.AttemptCount))
				result.retryScheduled = true
			} else {
				status = models.JobStatusDeadLetter
				result.deadLettered = true
			}
		default:
			outcome = models.JobAttemptOutcomeTerminal
			status = models.JobStatusDeadLetter
			result.deadLettered = true
		}
	}

	attempt := models.JobAttempt{
		JobID: claimed.ID, JobType: claimed.JobType, AttemptNumber: claimed.AttemptCount,
		WorkerID: workerID, Outcome: outcome, StartedAt: startedAt, FinishedAt: finishedAt,
		LatencyMs: finishedAt.Sub(startedAt).Milliseconds(),
	}
	if handlerErr != nil {
		attempt.ErrorClass = string(class)
		attempt.ErrorMessage = boundedError(handlerErr)
	}
	result.attempt = attempt

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&attempt).Error; err != nil {
			return err
		}
		updates := map[string]any{
			"status": status, "run_at": runAt, "lease_owner": "", "lease_expires_at": nil,
			"attempt_started_at": nil, "completed_at": completedAt, "last_error": attempt.ErrorMessage,
		}
		update := tx.Model(&models.JobQueue{}).
			Where("id = ? AND status = ? AND lease_owner = ? AND attempt_count = ?", claimed.ID, models.JobStatusRunning, workerID, claimed.AttemptCount).
			Updates(updates)
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return ErrLeaseLost
		}
		if result.deadLettered {
			deadLetter := models.JobDeadLetter{
				JobID: claimed.ID, JobType: claimed.JobType, PayloadJSON: claimed.PayloadJSON,
				CorrelationID: claimed.CorrelationID, AttemptCount: claimed.AttemptCount,
				ErrorClass: string(class), FailureReason: attempt.ErrorMessage, FailedAt: finishedAt,
			}
			if err := tx.Create(&deadLetter).Error; err != nil {
				return err
			}
			if registration.OnDeadLetter != nil {
				if err := registration.OnDeadLetter(ctx, tx, json.RawMessage(claimed.PayloadJSON), handlerErr); err != nil {
					return fmt.Errorf("dead-letter hook: %w", err)
				}
			}
		}
		return tx.First(&result.job, "id = ?", claimed.ID).Error
	})
	return result, err
}

func (r *Runtime) retryDelay(attempt int) time.Duration {
	maxDelay := r.config.RetryBaseDelay
	for current := 1; current < attempt && maxDelay < r.config.RetryMaxDelay; current++ {
		if maxDelay > r.config.RetryMaxDelay/2 {
			maxDelay = r.config.RetryMaxDelay
			break
		}
		maxDelay *= 2
	}
	if maxDelay > r.config.RetryMaxDelay {
		maxDelay = r.config.RetryMaxDelay
	}
	return r.config.Jitter(maxDelay)
}

func boundedError(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	const limit = 2048
	if len(text) > limit {
		return text[:limit]
	}
	return text
}

func (r *Runtime) logCompletion(ctx context.Context, result executionResult) {
	if r.config.Observer != nil {
		r.config.Observer.AttemptCompleted(
			result.job.JobType,
			result.attempt.Outcome,
			result.attempt.ErrorClass,
			result.attempt.FinishedAt.Sub(result.attempt.StartedAt),
		)
	}
	attributes := []any{
		"event", "job_attempt_completed", "job_id", result.job.ID, "job_type", result.job.JobType,
		"attempt", result.attempt.AttemptNumber, "correlation_id", result.job.CorrelationID,
		"latency_ms", result.attempt.LatencyMs, "outcome", result.attempt.Outcome,
	}
	if result.attempt.ErrorClass != "" {
		attributes = append(attributes, "error_class", result.attempt.ErrorClass)
	}
	if result.retryScheduled {
		attributes = append(attributes, "next_attempt_at", result.job.RunAt)
	}
	if result.deadLettered {
		r.config.Logger.ErrorContext(ctx, "Job attempt completed", attributes...)
		return
	}
	if result.attempt.Outcome == models.JobAttemptOutcomeRetryable || result.attempt.Outcome == models.JobAttemptOutcomeDegraded {
		r.config.Logger.WarnContext(ctx, "Job attempt completed", attributes...)
		return
	}
	r.config.Logger.InfoContext(ctx, "Job attempt completed", attributes...)
}
