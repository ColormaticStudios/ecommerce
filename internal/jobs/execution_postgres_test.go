package jobs

import (
	"sync/atomic"
	"testing"
	"time"

	"ecommerce/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestCheckpointExecutionRechecksOwnershipAfterPostgresLockWait(t *testing.T) {
	for _, scenario := range []string{"reclaimed", "expired_while_waiting"} {
		t.Run(scenario, func(t *testing.T) {
			db, ctx := coalescingPostgresDB(t)
			var clock atomic.Int64
			clock.Store(time.Now().UTC().UnixNano())
			runtime := NewRuntime(db, Config{Now: func() time.Time { return time.Unix(0, clock.Load()).UTC() }})
			job, err := runtime.Enqueue(ctx, EnqueueInput{JobType: "test.checkpoint"})
			require.NoError(t, err)
			claimed, err := runtime.claim(ctx, job.ID, "original-worker")
			require.NoError(t, err)
			execution := withExecution(ctx, claimed, claimed.LeaseOwner, runtime.config.Now)
			blocker := db.WithContext(ctx).Begin()
			require.NoError(t, blocker.Error)
			defer blocker.Rollback()
			var locked models.JobQueue
			require.NoError(t, blocker.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, "id = ?", job.ID).Error)
			blockerPID, err := coalescingPID(blocker)
			require.NoError(t, err)
			if scenario == "reclaimed" {
				require.NoError(t, blocker.Model(&job).Updates(map[string]any{"lease_owner": "new-worker", "attempt_count": claimed.AttemptCount + 1}).Error)
			}
			waiterPID := make(chan int, 1)
			done := make(chan error, 1)
			go func() {
				done <- db.WithContext(execution).Transaction(func(tx *gorm.DB) error {
					pid, err := coalescingPID(tx)
					if err != nil {
						return err
					}
					waiterPID <- pid
					_, err = LockExecutionTx(execution, tx)
					return err
				})
			}()
			var waiter int
			select {
			case waiter = <-waiterPID:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			waitCoalescingLock(t, db, ctx, waiter, blockerPID)
			if scenario == "expired_while_waiting" {
				clock.Add(int64(time.Hour))
			}
			require.NoError(t, blocker.Commit().Error)
			select {
			case err := <-done:
				require.ErrorIs(t, err, ErrLeaseLost)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		})
	}
}
