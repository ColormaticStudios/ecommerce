package commands

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ecommerce/config"
	"ecommerce/internal/apicontract"
	"ecommerce/internal/jobs"
	"ecommerce/internal/search"
	"ecommerce/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSearchReviewCLIPendingReindexReusesAPIJob(t *testing.T) {
	db, _, cfg := searchWorkflowFixture(t)
	service := search.NewService(db, nil, newJobRuntime(db, cfg))
	job, err := service.EnqueueFullReindex(context.Background())
	require.NoError(t, err)
	output, err := runSearchWorkflowCommand(context.Background(), "", "reindex")
	require.NoError(t, err)
	require.JSONEq(t, `{"job_id":"`+job.ID+`","status":"succeeded","disposition":"executed"}`, output)
	var count int64
	require.NoError(t, db.Model(&models.JobQueue{}).Where("job_type = ?", search.JobTypeFullReindex).Count(&count).Error)
	require.EqualValues(t, 1, count)
	var attempt models.JobAttempt
	require.NoError(t, db.First(&attempt, "job_id = ?", job.ID).Error)
	require.Equal(t, "cli-search-reindex", attempt.WorkerID)
	require.Equal(t, models.JobAttemptOutcomeSucceeded, attempt.Outcome)
}

// Hold a real runtime lease so the command must join the API's work, rather
// than succeeding by overwriting queue state or executing a second handler.
func startSearchReviewLeasedReindex(t *testing.T, db *gorm.DB, cfg config.Config) (models.JobQueue, func(), <-chan error, *atomic.Int32) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	runtime := newJobRuntime(db, cfg)
	service := search.NewService(db, nil, runtime)
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	require.NoError(t, runtime.Register(search.JobTypeFullReindex, jobs.Registration{Handle: func(ctx context.Context, _ json.RawMessage) error {
		calls.Add(1)
		close(started)
		select {
		case <-release:
			return service.Reindex(ctx)
		case <-ctx.Done():
			return ctx.Err()
		}
	}}))
	job, err := service.EnqueueFullReindex(context.Background())
	require.NoError(t, err)
	done := make(chan error, 1)
	workerStopped := make(chan struct{})
	go func() {
		done <- runtime.Execute(context.Background(), job.ID, "api-owned-reindex")
		close(workerStopped)
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("API runtime did not acquire the reindex lease")
	}
	var once sync.Once
	finish := func() { once.Do(func() { close(release) }) }
	t.Cleanup(func() {
		finish()
		select {
		case <-workerStopped:
		case <-time.After(3 * time.Second):
			t.Error("leased test worker did not stop")
		}
	})
	return job, finish, done, &calls
}

func TestSearchReviewCLIWaitsForLiveReindexLease(t *testing.T) {
	db, _, cfg := searchWorkflowFixture(t)
	job, finish, apiDone, calls := startSearchReviewLeasedReindex(t, db, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	blocked := make(chan struct{}, 1)
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("search_review_observe_blocked_claim", func(tx *gorm.DB) {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) && strings.Contains(tx.Statement.SQL.String(), "run_at <=") {
			select {
			case blocked <- struct{}{}:
			default:
			}
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove("search_review_observe_blocked_claim") })
	type result struct {
		output string
		err    error
	}
	cliDone := make(chan result, 1)
	go func() {
		output, err := runSearchWorkflowCommand(ctx, "", "reindex")
		cliDone <- result{output, err}
	}()
	select {
	case <-blocked:
	case early := <-cliDone:
		t.Fatalf("CLI returned before the API completed its leased job: %v (%s)", early.err, early.output)
	case <-ctx.Done():
		t.Fatal("CLI did not attempt to join the leased job")
	}
	var held models.JobQueue
	require.NoError(t, db.First(&held, "id = ?", job.ID).Error)
	require.Equal(t, "api-owned-reindex", held.LeaseOwner)
	require.Equal(t, 1, held.AttemptCount)
	finish()
	require.NoError(t, <-apiDone)
	completed := <-cliDone
	require.NoError(t, completed.err)
	require.JSONEq(t, `{"job_id":"`+job.ID+`","status":"succeeded","disposition":"awaited_existing"}`, completed.output)
	require.EqualValues(t, 1, calls.Load())
	var attempts int64
	require.NoError(t, db.Model(&models.JobAttempt{}).Where("job_id = ?", job.ID).Count(&attempts).Error)
	require.EqualValues(t, 1, attempts)
}

func TestSearchReviewCLICancelDoesNotStealOrCancelAPILease(t *testing.T) {
	db, _, cfg := searchWorkflowFixture(t)
	job, finish, apiDone, calls := startSearchReviewLeasedReindex(t, db, cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	output, err := runSearchWorkflowCommand(ctx, "", "reindex")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Empty(t, output)
	var held models.JobQueue
	require.NoError(t, db.First(&held, "id = ?", job.ID).Error)
	require.Equal(t, models.JobStatusRunning, held.Status)
	require.Equal(t, "api-owned-reindex", held.LeaseOwner)
	require.Equal(t, 1, held.AttemptCount)
	finish()
	require.NoError(t, <-apiDone)
	require.EqualValues(t, 1, calls.Load())
}

func TestSearchReviewCLIConcurrentCompletionReportsAwaitedExisting(t *testing.T) {
	db, _, cfg := searchWorkflowFixture(t)
	job, finish, apiDone, _ := startSearchReviewLeasedReindex(t, db, cfg)
	type callerKey struct{}
	ctx, cancel := context.WithTimeout(context.WithValue(context.Background(), callerKey{}, true), 3*time.Second)
	defer cancel()
	var observed atomic.Bool
	// Complete the API job between the CLI's rejected claim and its next
	// nontransactional job read. The CLI must not claim credit for execution.
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("search_review_complete_before_reload", func(tx *gorm.DB) {
		_, jobRead := tx.Statement.Dest.(*models.JobQueue)
		_, outsideTransaction := tx.Statement.ConnPool.(*sql.DB)
		if tx.Statement.Context.Value(callerKey{}) == true && jobRead && outsideTransaction && observed.CompareAndSwap(false, true) {
			finish()
			require.NoError(t, <-apiDone)
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Query().Remove("search_review_complete_before_reload") })
	output, err := runSearchWorkflowCommand(ctx, "", "reindex")
	require.NoError(t, err)
	require.True(t, observed.Load(), "completion interleaving was not exercised")
	require.JSONEq(t, `{"job_id":"`+job.ID+`","status":"succeeded","disposition":"awaited_existing"}`, output)
}

func TestSearchReviewCLICompletedAndTerminalRefreshKeepQueueHistory(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		t.Run(strconv.FormatBool(terminal), func(t *testing.T) {
			db, _, cfg := searchWorkflowFixture(t)
			runtime := newJobRuntime(db, cfg)
			service := search.NewService(db, nil, runtime)
			require.NoError(t, runtime.Register(search.JobTypeSalesRefresh, jobs.Registration{Handle: func(context.Context, json.RawMessage) error {
				if terminal {
					return errors.New("terminal test failure")
				}
				return nil
			}}))
			job, err := service.EnqueueSalesRefresh(context.Background())
			require.NoError(t, err)
			executeErr := runtime.Execute(context.Background(), job.ID, "api-refresh")
			if terminal {
				require.ErrorContains(t, executeErr, "terminal test failure")
			} else {
				require.NoError(t, executeErr)
			}
			output, err := runSearchWorkflowCommand(context.Background(), "", "refresh-sales")
			if terminal {
				require.ErrorContains(t, err, "failed permanently")
				require.Empty(t, output)
			} else {
				require.NoError(t, err)
				require.JSONEq(t, `{"job_id":"`+job.ID+`","status":"succeeded","disposition":"reused_completed"}`, output)
			}
			var persisted models.JobQueue
			require.NoError(t, db.First(&persisted, "id = ?", job.ID).Error)
			require.Equal(t, 1, persisted.AttemptCount)
			var count int64
			require.NoError(t, db.Model(&models.JobAttempt{}).Where("job_id = ?", job.ID).Count(&count).Error)
			require.EqualValues(t, 1, count)
			require.NoError(t, db.Model(&models.JobDeadLetter{}).Where("job_id = ?", job.ID).Count(&count).Error)
			if terminal {
				require.EqualValues(t, 1, count)
			} else {
				require.Zero(t, count)
			}
		})
	}
}

func TestSearchReviewCLIConfigurationHistoryLocalPagination(t *testing.T) {
	db, _, _ := searchWorkflowFixture(t)
	service := search.NewService(db, nil, nil)
	input := search.SynonymSetInput{Name: "Bags", Direction: search.SynonymDirectionBidirectional, Terms: []string{"bag", "pack"}, IsActive: true}
	created, err := service.CreateSynonymSet(context.Background(), input)
	require.NoError(t, err)
	input.Name = "Travel bags"
	_, err = service.UpdateSynonymSet(context.Background(), created.ID, input)
	require.NoError(t, err)
	require.NoError(t, service.DeleteSynonymSet(context.Background(), created.ID, nil))
	var ids []int
	for page, operation := range []string{"delete", "update", "create"} {
		output, err := runSearchWorkflowCommand(context.Background(), "", "configuration-history", "--kind", "synonym", "--entity-id", strconv.Itoa(int(created.ID)), "--page", strconv.Itoa(page+1), "--limit", "1")
		require.NoError(t, err)
		var response apicontract.SearchConfigurationAuditListResponse
		require.NoError(t, json.Unmarshal([]byte(output), &response))
		require.Len(t, response.Items, 1)
		require.Equal(t, 3, response.Pagination.Total)
		require.Equal(t, page+1, response.Pagination.Page)
		require.Equal(t, 1, response.Pagination.Limit)
		require.Equal(t, operation, string(response.Items[0].Operation))
		require.Nil(t, response.Items[0].ActorId)
		ids = append(ids, response.Items[0].Id)
	}
	require.Greater(t, ids[0], ids[1])
	require.Greater(t, ids[1], ids[2])
}

func TestSearchReviewCLIRemoteHistoryAnalyticsAndRefreshContracts(t *testing.T) {
	searchWorkflowRemote(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer workflow-token", r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/api/v1/admin/search/configuration-history":
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "ranking_profile", r.URL.Query().Get("kind"))
			require.Equal(t, "7", r.URL.Query().Get("entity_id"))
			require.Equal(t, "2", r.URL.Query().Get("page"))
			require.Equal(t, "3", r.URL.Query().Get("limit"))
			_, _ = w.Write([]byte(`{"items":[],"pagination":{"total":2,"page":2,"limit":3,"total_pages":1}}`))
		case "/api/v1/admin/search/analytics":
			require.Equal(t, "30", r.URL.Query().Get("days"))
			require.Equal(t, "2", r.URL.Query().Get("page"))
			require.Equal(t, "3", r.URL.Query().Get("limit"))
			_, _ = w.Write([]byte(`{"searches":12,"queries":[],"pagination":{"total":2,"page":2,"limit":3,"total_pages":1}}`))
		case "/api/v1/admin/search/refresh-sales":
			require.Equal(t, http.MethodPost, r.Method)
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":"daily","status":"succeeded","disposition":"reused_completed"}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	for _, args := range [][]string{
		{"configuration-history", "--kind", "ranking_profile", "--entity-id", "7", "--page", "2", "--limit", "3"},
		{"analytics", "--days", "30", "--page", "2", "--limit", "3"},
		{"refresh-sales"},
	} {
		output, err := runSearchWorkflowCommand(context.Background(), "", args...)
		require.NoError(t, err)
		if args[0] == "refresh-sales" {
			require.JSONEq(t, `{"job_id":"daily","status":"succeeded","disposition":"reused_completed"}`, output)
		} else {
			require.Contains(t, output, `"page": 2`)
		}
	}
}

func TestSearchReviewCLIAnalyticsLocalPaginationKeepsGlobalTotals(t *testing.T) {
	db, _, _ := searchWorkflowFixture(t)
	for i, query := range []string{"boots", "bags", "coats"} {
		require.NoError(t, db.Create(&models.SearchQueryEvent{
			ImpressionID: "impression-" + strconv.Itoa(i), SnapshotID: "snapshot-" + strconv.Itoa(i), SessionHash: "session-" + strconv.Itoa(i),
			Query: query, NormalizedQuery: query, FiltersJSON: "{}", ProductsJSON: "[]", CreatedAt: time.Now().UTC(), FirstPosition: 1, LatencyMs: int64(i + 1),
		}).Error)
	}
	for page, expectedQueries := range map[int]int{2: 1, 4: 0} {
		output, err := runSearchWorkflowCommand(context.Background(), "", "analytics", "--days", "30", "--page", strconv.Itoa(page), "--limit", "1")
		require.NoError(t, err)
		var response apicontract.SearchAnalytics
		require.NoError(t, json.Unmarshal([]byte(output), &response))
		require.Equal(t, 3, response.Searches)
		require.Equal(t, 3, response.ZeroResults)
		require.Equal(t, 3, response.UniqueSessions)
		require.Equal(t, 3, response.Pagination.Total)
		require.Equal(t, page, response.Pagination.Page)
		require.Equal(t, 1, response.Pagination.Limit)
		require.Len(t, response.Queries, expectedQueries)
	}
}

func TestSearchReviewCLIHistoryAnalyticsInvalidPaginationBeforeTransport(t *testing.T) {
	searchWorkflowRemote(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("invalid input reached transport: %s", r.URL)
		w.WriteHeader(http.StatusInternalServerError)
	})
	for _, args := range [][]string{
		{"configuration-history", "--kind", "unknown"},
		{"configuration-history", "--entity-id", "7"},
		{"configuration-history", "--entity-id", "-1"},
		{"configuration-history", "--page", "0"},
		{"configuration-history", "--page", "1000001"},
		{"configuration-history", "--limit", "0"},
		{"configuration-history", "--limit", "101"},
		{"analytics", "--page", "0"},
		{"analytics", "--page", "1000001"},
		{"analytics", "--limit", "0"},
		{"analytics", "--limit", "101"},
	} {
		output, err := runSearchWorkflowCommand(context.Background(), "", args...)
		require.Error(t, err, args)
		require.Empty(t, output)
	}
}
