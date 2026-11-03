package search_test

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"ecommerce/internal/search"
	"ecommerce/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func receiveHandoff[T any](t *testing.T, ctx context.Context, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-ctx.Done():
		t.Fatal(ctx.Err())
		var zero T
		return zero
	}
}

// The producer and finalizer use distinct PostgreSQL connections. Every race
// waits for pg_blocking_pids evidence rather than assuming goroutine ordering
// or treating a sleep as proof that a row lock was acquired.
func TestSearchCoalescedProductAndVariantCompletionHandoffPostgres(t *testing.T) {
	for _, variantChange := range []bool{false, true} {
		for _, producerFirst := range []bool{true, false} {
			t.Run(fmt.Sprintf("variant_%v/producer_first_%v", variantChange, producerFirst), func(t *testing.T) {
				f := newProjectionPostgresFixture(t)
				ctx := f.ctx
				key := fmt.Sprintf("product:%d:pending", f.product.ID)
				if variantChange {
					key = fmt.Sprintf("variant:%d:pending", f.variant.ID)
				}
				change := func(tx *gorm.DB, initial bool) (models.JobQueue, error) {
					if variantChange {
						stock := 2
						if !initial {
							stock = 0
						}
						if err := tx.Model(&models.ProductVariant{}).Where("id = ?", f.variant.ID).Update("stock", stock).Error; err != nil {
							return models.JobQueue{}, err
						}
						if err := search.RecordInventoryChangeTx(ctx, tx, f.variant.ID); err != nil {
							return models.JobQueue{}, err
						}
						var job models.JobQueue
						err := tx.Where("idempotency_key = ?", key).First(&job).Error
						return job, err
					}
					name := "First handoff name"
					if !initial {
						name = "Final handoff name"
					}
					if err := tx.Model(&models.Product{}).Where("id = ?", f.product.ID).Update("name", name).Error; err != nil {
						return models.JobQueue{}, err
					}
					return search.RecordProductChangeTx(ctx, f.runtime, tx, f.product.ID, time.Now())
				}
				var first models.JobQueue
				require.NoError(t, f.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { var err error; first, err = change(tx, true); return err }))
				ready := make(chan struct{})
				releaseHandler := make(chan struct{})
				releaseCompletion := make(chan struct{})
				var releaseHandlerOnce, releaseCompletionOnce sync.Once
				defer releaseHandlerOnce.Do(func() { close(releaseHandler) })
				defer releaseCompletionOnce.Do(func() { close(releaseCompletion) })
				completionPID := make(chan int, 1)
				producerPID := make(chan int, 1)
				pid := func(tx *gorm.DB) (int, error) {
					var value int
					err := tx.Session(&gorm.Session{NewDB: true}).Raw("SELECT pg_backend_pid()").Scan(&value).Error
					return value, err
				}
				require.NoError(t, f.db.Callback().Create().Before("gorm:create").Register("handoff_handler_finished", func(tx *gorm.DB) {
					if tx.Statement.Table != "job_attempts" {
						return
					}
					close(ready)
					select {
					case <-releaseHandler:
					case <-ctx.Done():
						tx.AddError(ctx.Err())
					}
				}))
				if producerFirst {
					require.NoError(t, f.db.Callback().Update().Before("gorm:update").Register("handoff_finalization_pid", func(tx *gorm.DB) {
						if tx.Statement.Table != "job_queue" {
							return
						}
						updates, ok := tx.Statement.Dest.(map[string]any)
						if !ok || updates["status"] != models.JobStatusSucceeded {
							return
						}
						value, err := pid(tx)
						if err != nil {
							tx.AddError(err)
							return
						}
						completionPID <- value
					}))
				} else {
					require.NoError(t, f.db.Callback().Update().After("gorm:update").Register("handoff_finalization_hold", func(tx *gorm.DB) {
						if tx.Statement.Table != "job_queue" {
							return
						}
						updates, ok := tx.Statement.Dest.(map[string]any)
						if !ok || updates["status"] != models.JobStatusSucceeded {
							return
						}
						value, err := pid(tx)
						if err != nil {
							tx.AddError(err)
							return
						}
						completionPID <- value
						select {
						case <-releaseCompletion:
						case <-ctx.Done():
							tx.AddError(ctx.Err())
						}
					}))
					require.NoError(t, f.db.Callback().Query().Before("gorm:query").Register("handoff_producer_pid", func(tx *gorm.DB) {
						if _, locked := tx.Statement.Clauses["FOR"]; !locked || tx.Statement.Table != "job_queue" {
							return
						}
						value, err := pid(tx)
						if err != nil {
							tx.AddError(err)
							return
						}
						select {
						case producerPID <- value:
						default:
						}
					}))
				}
				workerDone := make(chan error, 1)
				go func() { workerDone <- f.runtime.Execute(ctx, first.ID, "handoff-worker") }()
				select {
				case <-ready:
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				producerDone := make(chan error, 1)
				var joined models.JobQueue
				var blocker, waiter int
				if producerFirst {
					producerLocked := make(chan struct{})
					go func() {
						producerDone <- f.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
							value, err := pid(tx)
							if err != nil {
								return err
							}
							producerPID <- value
							joined, err = change(tx, false)
							if err != nil {
								return err
							}
							close(producerLocked)
							select {
							case <-releaseCompletion:
								return nil
							case <-ctx.Done():
								return ctx.Err()
							}
						})
					}()
					select {
					case <-producerLocked:
					case <-ctx.Done():
						t.Fatal(ctx.Err())
					}
					releaseHandlerOnce.Do(func() { close(releaseHandler) })
					waiter, blocker = receiveHandoff(t, ctx, completionPID), receiveHandoff(t, ctx, producerPID)
				} else {
					releaseHandlerOnce.Do(func() { close(releaseHandler) })
					blocker = receiveHandoff(t, ctx, completionPID)
					go func() {
						producerDone <- f.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { var err error; joined, err = change(tx, false); return err })
					}()
					waiter = receiveHandoff(t, ctx, producerPID)
				}
				require.NotEqual(t, waiter, blocker)
				require.Eventually(t, func() bool {
					var blocked bool
					err := f.db.WithContext(ctx).Raw("SELECT ? = ANY(pg_blocking_pids(?))", blocker, waiter).Scan(&blocked).Error
					return err == nil && blocked
				}, 5*time.Second, 5*time.Millisecond, "actual producer/finalizer PostgreSQL row-lock contention")
				releaseCompletionOnce.Do(func() { close(releaseCompletion) })
				require.NoError(t, receiveHandoff(t, ctx, producerDone))
				require.NoError(t, receiveHandoff(t, ctx, workerDone))
				require.NoError(t, f.db.Callback().Create().Remove("handoff_handler_finished"))
				require.NoError(t, f.db.Callback().Update().Remove("handoff_finalization_pid"))
				require.NoError(t, f.db.Callback().Update().Remove("handoff_finalization_hold"))
				require.NoError(t, f.db.Callback().Query().Remove("handoff_producer_pid"))
				if producerFirst {
					require.Equal(t, first.ID, joined.ID)
				} else {
					require.NotEqual(t, first.ID, joined.ID)
				}
				var successor models.JobQueue
				require.NoError(t, f.db.WithContext(ctx).Where("idempotency_key = ?", key).First(&successor).Error)
				require.NotEqual(t, first.ID, successor.ID)
				require.Equal(t, models.JobStatusPending, successor.Status)
				require.Zero(t, successor.AttemptCount)
				require.Equal(t, first.PayloadJSON, successor.PayloadJSON)
				var original models.JobQueue
				require.NoError(t, f.db.First(&original, "id = ?", first.ID).Error)
				require.Equal(t, models.JobStatusSucceeded, original.Status)
				require.Nil(t, original.IdempotencyKey)
				var history []models.JobAttempt
				require.NoError(t, f.db.Where("job_id = ?", first.ID).Find(&history).Error)
				require.Len(t, history, 1)
				require.NoError(t, f.runtime.Execute(ctx, successor.ID, "handoff-successor"))
				var document models.SearchDocument
				require.NoError(t, f.db.Where("entity_type = ? AND entity_id = ?", search.ProductEntityType, f.product.ID).First(&document).Error)
				var indexed models.Product
				require.NoError(t, json.Unmarshal([]byte(document.PayloadJSON), &indexed))
				if variantChange {
					found := false
					for _, variant := range indexed.Variants {
						if variant.ID == f.variant.ID {
							found = true
							require.Zero(t, variant.Stock)
						}
					}
					require.True(t, found)
					var debt models.SearchInventoryRevision
					require.NoError(t, f.db.First(&debt, "variant_id = ?", f.variant.ID).Error)
					require.Nil(t, debt.PendingSince)
				} else {
					require.Equal(t, "Final handoff name", indexed.Name)
				}
				state := projectionPostgresState(t, f)
				require.Equal(t, state.Revision, state.IndexedRevision)
				require.Nil(t, state.PendingSince)
				var retained models.JobQueue
				require.NoError(t, f.db.First(&retained, "id = ?", first.ID).Error)
				require.Equal(t, original, retained, "successor must not rewrite completed delivery history")
				var retainedHistory []models.JobAttempt
				require.NoError(t, f.db.Where("job_id = ?", first.ID).Find(&retainedHistory).Error)
				require.Equal(t, history, retainedHistory)
				var active int64
				require.NoError(t, f.db.Model(&models.JobQueue{}).Where("idempotency_key = ?", key).Count(&active).Error)
				require.Zero(t, active)
			})
		}
	}
}
