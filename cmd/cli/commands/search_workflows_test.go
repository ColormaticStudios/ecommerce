package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"ecommerce/config"
	"ecommerce/internal/httpapi"
	"ecommerce/internal/search"
	"ecommerce/models"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func runSearchWorkflowCommand(ctx context.Context, input string, args ...string) (string, error) {
	cmd := NewSearchCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetIn(strings.NewReader(input))
	cmd.SetArgs(append(args, "--format", "json"))
	err := cmd.ExecuteContext(ctx)
	return output.String(), err
}
func searchWorkflowFixture(t *testing.T) (*gorm.DB, *httpapi.CatalogEndpoints, config.Config) {
	t.Helper()
	db := newTestDB(t, &models.Product{}, &models.ProductVariant{}, &models.ProductCategory{}, &models.Brand{}, &models.Category{}, &models.ProductAttribute{}, &models.ProductAttributeValue{}, &models.Locale{}, &models.LocaleMarketDefault{}, &models.LocalizedEntityValue{}, &models.SearchDocument{}, &models.SearchIndexState{}, &models.SearchSynonymSet{}, &models.SearchTypoToleranceProfile{}, &models.SearchRankingProfile{}, &models.SearchMerchandisingRule{}, &models.SearchMerchandisingAudit{}, &models.SearchSalesSignal{}, &models.SearchConversionSignal{}, &models.SearchQueryEvent{}, &models.SearchClickEvent{}, &models.SearchCartAttribution{}, &models.SearchOrderAttribution{}, &models.SearchRevokedSession{}, &models.SearchIndexIncident{}, &models.SearchFreshnessObservation{}, &models.Order{}, &models.OrderItem{}, &models.JobQueue{}, &models.JobAttempt{}, &models.JobDeadLetter{})
	svc := search.NewService(db, nil, nil)
	_, err := svc.CreateTypoToleranceProfile(context.Background(), search.TypoToleranceProfileInput{Name: "default", MinimumTokenLength: 4, OneEditMinimumLength: 4, TwoEditMinimumLength: 8, IsActive: true})
	require.NoError(t, err)
	_, err = svc.CreateRankingProfile(context.Background(), search.RankingProfileInput{Name: "default", IsDefault: true, Weights: search.DefaultRankingWeights()}, nil)
	require.NoError(t, err)
	cfg := config.Config{SearchMaxConcurrent: 7, SearchTimeoutMS: 1500, SearchCircuitFailureThreshold: 3, SearchCircuitOpenMS: 25000, SearchReindexQueueLimit: 1, JobWorkerConcurrency: 1, JobMaxAttempts: 3, JobLeaseDuration: time.Minute, JobPollInterval: time.Millisecond, JobRetryBaseDelay: time.Millisecond, JobRetryMaxDelay: time.Second}
	e, err := httpapi.NewCatalogEndpoints(db, nil, newJobRuntime(db, cfg))
	require.NoError(t, err)
	require.NoError(t, e.ConfigureSearchHardening(search.HardeningConfig{MaxConcurrent: cfg.SearchMaxConcurrent, SearchTimeoutMS: cfg.SearchTimeoutMS, CircuitFailureThreshold: cfg.SearchCircuitFailureThreshold, CircuitOpenMS: cfg.SearchCircuitOpenMS, ReindexQueueLimit: cfg.SearchReindexQueueLimit}))
	oldRuntime, oldOpen, oldDB := activeCLIRuntime, searchCLIOpenEndpoints, searchCLIOpenDatabase
	activeCLIRuntime = cliRuntime{}
	searchCLIOpenEndpoints = func(context.Context) (*httpapi.CatalogEndpoints, func(), error) { return e, func() {}, nil }
	searchCLIOpenDatabase = func(context.Context) (*gorm.DB, config.Config, func(), error) { return db, cfg, func() {}, nil }
	t.Cleanup(func() { activeCLIRuntime = oldRuntime; searchCLIOpenEndpoints = oldOpen; searchCLIOpenDatabase = oldDB })
	return db, e, cfg
}
func searchWorkflowRemote(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	old := activeCLIRuntime
	activeCLIRuntime = cliRuntime{Remote: &persistentCLIAuth{APIURL: server.URL, Token: "workflow-token"}}
	t.Cleanup(func() { activeCLIRuntime = old })
}
func TestSearchWorkflowsLocalJobsExecuteDurableHandlers(t *testing.T) {
	db, _, _ := searchWorkflowFixture(t)
	for _, kind := range []string{"reindex", "refresh-sales", "refresh-conversion"} {
		output, err := runSearchWorkflowCommand(context.Background(), "", kind)
		require.NoError(t, err)
		var result struct {
			JobID  string `json:"job_id"`
			Status string `json:"status"`
		}
		require.NoError(t, json.Unmarshal([]byte(output), &result))
		require.Equal(t, "succeeded", result.Status)
		var job models.JobQueue
		require.NoError(t, db.First(&job, "id = ?", result.JobID).Error)
		require.Equal(t, models.JobStatusSucceeded, job.Status)
		require.Equal(t, 1, job.AttemptCount)
		var attempt models.JobAttempt
		require.NoError(t, db.First(&attempt, "job_id = ?", job.ID).Error)
		require.Equal(t, models.JobAttemptOutcomeSucceeded, attempt.Outcome)
		if kind != "reindex" {
			repeated, err := runSearchWorkflowCommand(context.Background(), "", kind)
			require.NoError(t, err)
			require.JSONEq(t, output, repeated)
			require.NoError(t, db.First(&job, "id = ?", result.JobID).Error)
			require.Equal(t, 1, job.AttemptCount)
		}
	}
	var state models.SearchIndexState
	require.NoError(t, db.First(&state, "name = ?", "products").Error)
	require.NotNil(t, state.LastFullReindexAt)
	require.NotNil(t, state.LastConversionRefreshAt)
}
func TestSearchWorkflowsRemoteJobsRemainQueued(t *testing.T) {
	calls := 0
	searchWorkflowRemote(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "Bearer workflow-token", r.Header.Get("Authorization"))
		require.Contains(t, []string{"/api/v1/admin/search/reindex", "/api/v1/admin/search/refresh-sales", "/api/v1/admin/search/refresh-conversion"}, r.URL.Path)
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"job_id":"remote-job","status":"queued"}`))
	})
	for _, kind := range []string{"reindex", "refresh-sales", "refresh-conversion"} {
		output, err := runSearchWorkflowCommand(context.Background(), "", kind)
		require.NoError(t, err)
		require.JSONEq(t, `{"job_id":"remote-job","status":"queued"}`, output)
	}
	require.Equal(t, 3, calls)
}
func TestSearchWorkflowsProductFiltersAndExplanations(t *testing.T) {
	searchWorkflowRemote(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/admin/search/products", r.URL.Path)
		q := r.URL.Query()
		require.Equal(t, "canvas & bag", q.Get("q"))
		require.Equal(t, []string{"north", "south"}, q["brand_slug"])
		require.Equal(t, []string{"bags", "travel"}, q["category_slug"])
		require.Equal(t, []string{"true", "false"}, q["has_variant_stock"])
		require.Equal(t, "red & blue", q.Get("attribute[color][0]"))
		require.Equal(t, "green", q.Get("attribute[color][1]"))
		require.Equal(t, "margin-enabled", q.Get("ranking_profile"))
		require.Equal(t, "2", q.Get("page"))
		require.Equal(t, "5", q.Get("limit"))
		_, _ = w.Write([]byte(`{"items":[{"id":42,"name":"Canvas Bag"}],"explanations":[{"product_id":42,"score":17.5}],"facets":[],"pagination":{"total":1},"metadata":{}}`))
	})
	output, err := runSearchWorkflowCommand(context.Background(), `{"q":"ignored","brand_slug":["north","south"],"category_slug":["bags","travel"],"has_variant_stock":[true,false],"attribute":{"color":["red & blue","green"]},"ranking_profile":"margin-enabled","page":2,"limit":5}`, "products", "--file", "-", "--q", "canvas & bag")
	require.NoError(t, err)
	require.Contains(t, output, `"score": 17.5`)
	require.Contains(t, output, `"product_id": 42`)
}
func TestSearchWorkflowsProductLookup(t *testing.T) {
	tests := []struct {
		name, query, body string
		id                int
		errorText         string
	}{
		{"unique", "canvas", `{"items":[{"id":42,"name":"Canvas Bag"}],"pagination":{"total":1}}`, 42, ""},
		{"exact name", " canvas bag ", `{"items":[{"id":42,"name":"Canvas Bag"},{"id":43,"name":"Canvas Bag Deluxe"}],"pagination":{"total":2}}`, 42, ""},
		{"ambiguous", "canvas", `{"items":[{"id":42,"name":"Canvas Bag"},{"id":43,"name":"Canvas Bag Deluxe"}],"pagination":{"total":2}}`, 0, "ambiguous"},
		{"duplicate exact names", "canvas bag", `{"items":[{"id":42,"name":"Canvas Bag"},{"id":43,"name":"Canvas Bag"}],"pagination":{"total":2}}`, 0, "ambiguous"},
		{"incomplete result page", "canvas bag", `{"items":[{"id":42,"name":"Canvas Bag"}],"pagination":{"total":101}}`, 0, "ambiguous"},
		{"missing", "absent", `{"items":[],"pagination":{"total":0}}`, 0, "no indexed published product"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			searchWorkflowRemote(t, func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "100", r.URL.Query().Get("limit"))
				_, _ = w.Write([]byte(test.body))
			})
			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())
			cmd.Flags().String("format", "json", "")
			id, err := resolveSearchProduct(cmd, test.query)
			if test.errorText != "" {
				require.ErrorContains(t, err, test.errorText)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.id, id)
			}
		})
	}
}
func TestSearchWorkflowsLocalOperationsHideRuntimeCounters(t *testing.T) {
	searchWorkflowFixture(t)
	output, err := runSearchWorkflowCommand(context.Background(), "", "operations")
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(output), &result))
	require.Equal(t, false, result["runtime_counters_available"])
	for _, key := range []string{"active_searches", "circuit_state", "consecutive_failures", "circuit_open_until"} {
		require.Contains(t, result, key)
		require.Nil(t, result[key])
	}
	require.Equal(t, float64(7), result["max_concurrent"])
	require.Equal(t, float64(1500), result["search_timeout_ms"])
	require.Contains(t, result["runtime_note"], "remote-auth")
}
func TestSearchWorkflowsRemoteDiagnostics(t *testing.T) {
	searchWorkflowRemote(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/admin/search/operations":
			_, _ = w.Write([]byte(`{"active_searches":4,"circuit_state":"open","consecutive_failures":3,"max_concurrent":7}`))
		case "/api/v1/admin/search/analytics":
			require.Equal(t, "30", r.URL.Query().Get("days"))
			_, _ = w.Write([]byte(`{"impressions":12,"zero_result_rate":0.25,"popular_queries":[{"query":"boots"}]}`))
		case "/api/v1/admin/search/incidents":
			require.Equal(t, "resolved", r.URL.Query().Get("status"))
			require.Equal(t, "2", r.URL.Query().Get("page"))
			require.Equal(t, "10", r.URL.Query().Get("limit"))
			_, _ = w.Write([]byte(`{"data":[{"id":2,"status":"resolved","duration_seconds":3600}],"pagination":{"total":12}}`))
		case "/api/v1/admin/search/freshness":
			_, _ = w.Write([]byte(`{"status":"stale","lag_seconds":400,"document_count":12}`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	})
	for _, args := range [][]string{{"operations"}, {"analytics", "--days", "30"}, {"incidents", "--status", "resolved", "--page", "2", "--limit", "10"}, {"freshness"}} {
		output, err := runSearchWorkflowCommand(context.Background(), "", args...)
		require.NoError(t, err)
		if args[0] == "operations" {
			require.Contains(t, output, `"active_searches": 4`)
			require.Contains(t, output, `"circuit_state": "open"`)
			require.NotContains(t, output, "runtime_note")
		}
	}
}
func TestSearchWorkflowsEvaluateWithoutTarget(t *testing.T) {
	t.Setenv(cliDataDirEnv, t.TempDir())
	oldWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(t.TempDir()))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldWD)) })
	cmd := newRootCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"search", "evaluate", "--format", "json"})
	require.NoError(t, cmd.Execute())
	var report search.RelevanceReport
	require.NoError(t, json.Unmarshal(output.Bytes(), &report))
	require.NotEmpty(t, report.Queries)
	require.NoError(t, report.CheckRegression())
}
func TestSearchWorkflowsInvalidInputDoesNotOpenTarget(t *testing.T) {
	oldOpen, oldDB, oldRuntime := searchCLIOpenEndpoints, searchCLIOpenDatabase, activeCLIRuntime
	activeCLIRuntime = cliRuntime{}
	calls := 0
	searchCLIOpenEndpoints = func(context.Context) (*httpapi.CatalogEndpoints, func(), error) {
		calls++
		return nil, nil, errors.New("should not open")
	}
	searchCLIOpenDatabase = func(context.Context) (*gorm.DB, config.Config, func(), error) {
		calls++
		return nil, config.Config{}, nil, errors.New("should not open")
	}
	t.Cleanup(func() { searchCLIOpenEndpoints = oldOpen; searchCLIOpenDatabase = oldDB; activeCLIRuntime = oldRuntime })
	for _, input := range []string{`null`, `[]`, `{"q":null}`, `{"unknown":true}`, `{"q":"a"} {}`, `{"q":`, strings.Repeat(" ", 1024*1024+1)} {
		_, err := runSearchWorkflowCommand(context.Background(), input, "products", "--file", "-")
		require.Error(t, err)
	}
	for _, args := range [][]string{{"analytics", "--days", "91"}, {"incidents", "--status", "wrong"}, {"incidents", "--page", "0"}} {
		_, err := runSearchWorkflowCommand(context.Background(), "", args...)
		require.Error(t, err)
	}
	for _, kind := range []string{"reindex", "refresh-sales", "refresh-conversion"} {
		cmd := NewSearchCmd()
		cmd.SetArgs([]string{kind, "--format", "yaml"})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		require.ErrorContains(t, cmd.Execute(), "format must be text or json")
	}
	require.Zero(t, calls)
}
func TestSearchWorkflowsTransportCancellationAndProblems(t *testing.T) {
	searchWorkflowRemote(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/problem":
			w.WriteHeader(403)
			_, _ = w.Write([]byte(`{"detail":"Administrator role required","error_code":"forbidden"}`))
		case "/invalid":
			_, _ = w.Write([]byte(`not-json`))
		case "/redirect":
			http.Redirect(w, r, "/problem", http.StatusTemporaryRedirect)
		default:
			t.Errorf("canceled request reached server")
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := invokeCLIJSON[any](ctx, "GET", "/cancel", nil)
	require.ErrorIs(t, err, context.Canceled)
	_, err = invokeCLIJSON[any](context.Background(), "GET", "/problem", nil)
	require.ErrorContains(t, err, "API returned 403: Administrator role required")
	require.NotContains(t, err.Error(), "workflow-token")
	_, err = invokeCLIJSON[any](context.Background(), "GET", "/invalid", nil)
	require.ErrorContains(t, err, "decode API response")
	_, err = invokeCLIJSON[any](context.Background(), "GET", "/redirect", nil)
	require.ErrorContains(t, err, "API returned 307")
}

func TestSearchWorkflowsLocalDiagnosticsAndProductExplanations(t *testing.T) {
	db, _, _ := searchWorkflowFixture(t)
	now := time.Now().UTC()
	recovered := now.Add(-time.Hour)
	opened := recovered.Add(-time.Hour)
	require.NoError(t, db.Create(&models.SearchIndexIncident{IndexName: "products", Reason: "index_lag", OpenedAt: opened, DetectedAt: opened, LastObservedAt: recovered, RecoveredAt: &recovered, MaxLagSeconds: 600}).Error)
	output, err := runSearchWorkflowCommand(context.Background(), "", "incidents", "--status", "resolved")
	require.NoError(t, err)
	var incidents struct {
		Data []struct {
			Status   string `json:"status"`
			Duration int64  `json:"duration_seconds"`
		}
	}
	require.NoError(t, json.Unmarshal([]byte(output), &incidents))
	require.Len(t, incidents.Data, 1)
	require.Equal(t, "resolved", incidents.Data[0].Status)
	require.Equal(t, int64(3600), incidents.Data[0].Duration)
	output, err = runSearchWorkflowCommand(context.Background(), "", "analytics", "--days", "30")
	require.NoError(t, err)
	require.Contains(t, output, `"searches": 0`)
	require.Contains(t, output, `"days": 30`)
	output, err = runSearchWorkflowCommand(context.Background(), "", "freshness")
	require.NoError(t, err)
	require.Contains(t, output, `"status": "stale"`)
	require.NoError(t, db.Create(&models.Locale{Code: "en-US", Name: "English", IsDefault: true, IsEnabled: true}).Error)
	product := models.Product{BaseModel: models.BaseModel{ID: 42}, Name: "Canvas Bag", IsPublished: true, Related: []models.Product{}}
	require.NoError(t, db.Create(&product).Error)
	require.NoError(t, db.Create(&models.LocalizedEntityValue{EntityType: "product", EntityID: 42, LocaleID: 1, Field: "name", Value: product.Name}).Error)
	payload, err := json.Marshal(product)
	require.NoError(t, err)
	require.NoError(t, search.NewDatabaseBackend(db).Upsert(context.Background(), models.SearchDocument{EntityType: "product", EntityID: 42, PayloadJSON: string(payload), SearchableText: "canvas bag", NormalizedName: "canvas bag", Active: true, Version: 1, SourceCreatedAt: now, SourceUpdatedAt: now, IndexedAt: now}))
	output, err = runSearchWorkflowCommand(context.Background(), "", "products", "--q", "canvas bag")
	require.NoError(t, err)
	var result struct {
		Items []struct {
			ID int `json:"id"`
		}
		Explanations []map[string]any
	}
	require.NoError(t, json.Unmarshal([]byte(output), &result))
	require.Len(t, result.Items, 1)
	require.Equal(t, 42, result.Items[0].ID)
	require.Len(t, result.Explanations, 1)
	require.Equal(t, float64(42), result.Explanations[0]["product_id"])
	require.Contains(t, result.Explanations[0], "components")
}
