package jobs

import (
	"context"
	"errors"
	"time"

	"ecommerce/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrMissingExecution = errors.New("job execution context is required")

type executionContextKey struct{}

type executionClaim struct {
	jobID    string
	workerID string
	attempt  int
	now      func() time.Time
}

func withExecution(ctx context.Context, job models.JobQueue, workerID string, now func() time.Time) context.Context {
	return context.WithValue(ctx, executionContextKey{}, executionClaim{jobID: job.ID, workerID: workerID, attempt: job.AttemptCount, now: now})
}

// LockExecutionTx fences a handler's checkpoint and database effects against
// lease reclaim. The caller must own a transaction and commit all effects and
// progress in that transaction. Recheck before returning after long batches so
// an expired lease rolls back rather than committing unowned progress.
func LockExecutionTx(ctx context.Context, tx *gorm.DB) (models.JobQueue, error) {
	claim, ok := ctx.Value(executionContextKey{}).(executionClaim)
	if !ok || claim.jobID == "" || claim.workerID == "" || claim.attempt < 1 || claim.now == nil {
		return models.JobQueue{}, ErrMissingExecution
	}
	if tx == nil {
		return models.JobQueue{}, errors.New("job checkpoint transaction is required")
	}
	if err := ctx.Err(); err != nil {
		return models.JobQueue{}, err
	}
	var current models.JobQueue
	err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", claim.jobID).First(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.JobQueue{}, ErrLeaseLost
	}
	if err != nil {
		return models.JobQueue{}, err
	}
	if current.Status != models.JobStatusRunning || current.LeaseOwner != claim.workerID || current.AttemptCount != claim.attempt || current.LeaseExpiresAt == nil || !claim.now().Before(*current.LeaseExpiresAt) {
		return models.JobQueue{}, ErrLeaseLost
	}
	return current, nil
}
