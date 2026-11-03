package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"ecommerce/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func coalescingPostgresDB(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	dsn := os.Getenv("SEARCH_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set SEARCH_TEST_POSTGRES_DSN for PostgreSQL coalescing handoff integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	rootSQL, err := root.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, rootSQL.Close()) })
	schema := "jobs_coalescing_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, root.WithContext(ctx).Exec(`CREATE SCHEMA "`+schema+`"`).Error)
	t.Cleanup(func() { require.NoError(t, root.Exec(`DROP SCHEMA "`+schema+`" CASCADE`).Error) })
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	params := parsed.Query()
	params.Set("search_path", schema)
	parsed.RawQuery = params.Encode()
	db, err := gorm.Open(postgres.Open(parsed.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.WithContext(ctx).AutoMigrate(&models.JobQueue{}, &models.JobAttempt{}, &models.JobDeadLetter{}))
	return db, ctx
}

func waitCoalescingLock(t *testing.T, db *gorm.DB, ctx context.Context, waiter, blocker int) {
	t.Helper()
	require.NotEqual(t, waiter, blocker, "race requires separate PostgreSQL connections")
	require.Eventually(t, func() bool {
		var blocked bool
		err := db.WithContext(ctx).Raw("SELECT ? = ANY(pg_blocking_pids(?))", blocker, waiter).Scan(&blocked).Error
		return err == nil && blocked
	}, 5*time.Second, 5*time.Millisecond, "expected actual PostgreSQL row-lock contention")
}

func receiveCoalescing[T any](t *testing.T, ctx context.Context, ch <-chan T) T {
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

func coalescingPID(tx *gorm.DB) (int, error) {
	var pid int
	err := tx.Session(&gorm.Session{NewDB: true}).Raw("SELECT pg_backend_pid()").Scan(&pid).Error
	return pid, err
}

func TestCoalescedCompletionProducerHandoffPostgres(t *testing.T) {
	for _, producerFirst := range []bool{true, false} {
		t.Run(map[bool]string{true: "producer_locks_running_job", false: "completion_releases_key_first"}[producerFirst], func(t *testing.T) {
			db, ctx := coalescingPostgresDB(t)
			runtime := NewRuntime(db, Config{})
			input := EnqueueInput{JobType: "test.coalesced", Payload: map[string]int{"version": 1, "entity_id": 7}, IdempotencyKey: "entity:7:pending"}
			first, err := runtime.Enqueue(ctx, input)
			require.NoError(t, err)
			claimed, err := runtime.claim(ctx, first.ID, "handoff-worker")
			require.NoError(t, err)
			registration := Registration{Handle: func(context.Context, json.RawMessage) error { return nil }, ReleaseKeyOnCompletion: true}
			producerPID, completionPID := make(chan int, 1), make(chan int, 1)
			producerReady, release := make(chan struct{}), make(chan struct{})
			defer func() {
				select {
				case <-release:
				default:
					close(release)
				}
			}()
			producerDone, completionDone := make(chan error, 1), make(chan error, 1)
			var joined models.JobQueue
			if producerFirst {
				require.NoError(t, db.Callback().Update().Before("gorm:update").Register("handoff_completion_pid", func(tx *gorm.DB) {
					if tx.Statement.Table == "job_queue" {
						pid, err := coalescingPID(tx)
						if err != nil {
							tx.AddError(err)
							return
						}
						completionPID <- pid
					}
				}))
				registration.OnSuccess = func(ctx context.Context, tx *gorm.DB, _ models.JobQueue) error {
					_, err := runtime.EnqueueCoalescedTx(ctx, tx, input)
					return err
				}
				go func() {
					producerDone <- db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
						pid, err := coalescingPID(tx)
						if err != nil {
							return err
						}
						producerPID <- pid
						joined, err = runtime.EnqueueCoalescedTx(ctx, tx, input)
						if err != nil {
							return err
						}
						close(producerReady)
						select {
						case <-release:
							return nil
						case <-ctx.Done():
							return ctx.Err()
						}
					})
				}()
				select {
				case <-producerReady:
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				go func() {
					_, err := runtime.finalize(ctx, claimed, "handoff-worker", time.Now(), nil, registration)
					completionDone <- err
				}()
				waitCoalescingLock(t, db, ctx, receiveCoalescing(t, ctx, completionPID), receiveCoalescing(t, ctx, producerPID))
				close(release)
			} else {
				registration.OnSuccess = func(ctx context.Context, tx *gorm.DB, _ models.JobQueue) error {
					pid, err := coalescingPID(tx)
					if err != nil {
						return err
					}
					completionPID <- pid
					select {
					case <-release:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				require.NoError(t, db.Callback().Query().Before("gorm:query").Register("handoff_producer_pid", func(tx *gorm.DB) {
					if _, locked := tx.Statement.Clauses["FOR"]; locked && tx.Statement.Table == "job_queue" {
						pid, err := coalescingPID(tx)
						if err != nil {
							tx.AddError(err)
							return
						}
						select {
						case producerPID <- pid:
						default:
						}
					}
				}))
				go func() {
					_, err := runtime.finalize(ctx, claimed, "handoff-worker", time.Now(), nil, registration)
					completionDone <- err
				}()
				blocker := receiveCoalescing(t, ctx, completionPID)
				go func() {
					producerDone <- db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
						var err error
						joined, err = runtime.EnqueueCoalescedTx(ctx, tx, input)
						return err
					})
				}()
				waitCoalescingLock(t, db, ctx, receiveCoalescing(t, ctx, producerPID), blocker)
				close(release)
			}
			require.NoError(t, receiveCoalescing(t, ctx, producerDone))
			require.NoError(t, receiveCoalescing(t, ctx, completionDone))
			require.NoError(t, db.Callback().Update().Remove("handoff_completion_pid"))
			require.NoError(t, db.Callback().Query().Remove("handoff_producer_pid"))
			if producerFirst {
				require.Equal(t, first.ID, joined.ID)
			} else {
				require.NotEqual(t, first.ID, joined.ID)
			}
			var successor models.JobQueue
			require.NoError(t, db.Where("idempotency_key = ?", input.IdempotencyKey).First(&successor).Error)
			require.NotEqual(t, first.ID, successor.ID)
			require.Equal(t, models.JobStatusPending, successor.Status)
			require.Zero(t, successor.AttemptCount)
			require.Equal(t, first.PayloadJSON, successor.PayloadJSON)
			var original models.JobQueue
			require.NoError(t, db.First(&original, "id = ?", first.ID).Error)
			require.Equal(t, models.JobStatusSucceeded, original.Status)
			require.Nil(t, original.IdempotencyKey)
			require.Equal(t, first.PayloadJSON, original.PayloadJSON)
			var attempts []models.JobAttempt
			require.NoError(t, db.Where("job_id = ?", first.ID).Find(&attempts).Error)
			require.Len(t, attempts, 1)
			require.Equal(t, models.JobAttemptOutcomeSucceeded, attempts[0].Outcome)
			history := attempts[0]
			require.NoError(t, runtime.Register(input.JobType, Registration{Handle: func(context.Context, json.RawMessage) error { return nil }, ReleaseKeyOnCompletion: true}))
			require.NoError(t, runtime.Execute(ctx, successor.ID, "successor-worker"))
			require.NoError(t, db.First(&original, "id = ?", first.ID).Error)
			require.Equal(t, models.JobStatusSucceeded, original.Status)
			require.Equal(t, 1, original.AttemptCount)
			require.NoError(t, db.Where("job_id = ?", first.ID).Find(&attempts).Error)
			require.Equal(t, []models.JobAttempt{history}, attempts)
			var count int64
			require.NoError(t, db.Model(&models.JobQueue{}).Count(&count).Error)
			require.EqualValues(t, 2, count, fmt.Sprintf("handoff produced only original and successor: %v", producerFirst))
		})
	}
}
