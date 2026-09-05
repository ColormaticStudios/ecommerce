package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ecommerce/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"gopkg.in/yaml.v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTelemetryTestDB(t *testing.T) (*gorm.DB, *Metrics) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "-")+"?mode=memory&cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.JobQueue{}, &models.JobAttempt{}, &models.JobDeadLetter{}))
	pool, err := db.DB()
	require.NoError(t, err)
	metrics := NewMetrics("test-api", "test", "test-on-call")
	require.NoError(t, metrics.RegisterDatabase(db, pool))
	return db, metrics
}

func TestHTTPAndDatabaseMetricsExposeBoundedLabels(t *testing.T) {
	db, metrics := newTelemetryTestDB(t)
	due := time.Now().UTC().Add(-2 * time.Minute)
	expiredLease := time.Now().UTC().Add(-time.Minute)
	activeLease := time.Now().UTC().Add(time.Minute)
	key := "media-1"
	require.NoError(t, db.Create(&models.JobQueue{
		ID: "job-1", JobType: "media.process", PayloadJSON: `{}`, PayloadFingerprint: "fingerprint",
		Status: models.JobStatusPending, RunAt: due, IdempotencyKey: &key, MaxAttempts: 5,
	}).Error)
	for _, job := range []models.JobQueue{
		{ID: "job-expired", JobType: "media.process", PayloadJSON: `{}`, PayloadFingerprint: "expired", Status: models.JobStatusRunning, RunAt: due, MaxAttempts: 5, LeaseOwner: "gone", LeaseExpiresAt: &expiredLease},
		{ID: "job-active", JobType: "media.process", PayloadJSON: `{}`, PayloadFingerprint: "active", Status: models.JobStatusRunning, RunAt: due, MaxAttempts: 5, LeaseOwner: "worker", LeaseExpiresAt: &activeLease},
	} {
		require.NoError(t, db.Create(&job).Error)
	}
	require.NoError(t, db.Create(&models.JobDeadLetter{
		JobID: "dead-1", JobType: "media.process", PayloadJSON: `{}`, AttemptCount: 5,
		ErrorClass: "terminal", FailureReason: "failed", FailedAt: time.Now(),
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(HTTPMiddleware(metrics, "test-api"))
	router.GET("/items/:id", func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) })
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/items/secret-customer-id", nil))
	require.Equal(t, http.StatusNoContent, response.Code)

	metrics.WorkerStarted()
	metrics.JobClaimed("media.process")
	metrics.AttemptCompleted("media.process", models.JobAttemptOutcomeSucceeded, "", 25*time.Millisecond)
	metrics.ProviderCallCompleted("payment", "authorize", "failed", 750*time.Millisecond)
	metrics.ProviderCallCompleted("attacker-controlled", "unbounded-operation", "strange", time.Millisecond)
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	output := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(output, request)
	body := output.Body.String()
	assert.Contains(t, body, `ecommerce_http_requests_total{deployment_environment="test",method="GET",owner="test-on-call",route="/items/:id",service="test-api",status_class="2xx"} 1`)
	assert.NotContains(t, body, "secret-customer-id")
	assert.Contains(t, body, `ecommerce_jobs_queue_depth{deployment_environment="test",job_type="media.process",owner="test-on-call",service="test-api",status="pending"} 1`)
	assert.Contains(t, body, `ecommerce_jobs_queue_depth{deployment_environment="test",job_type="media.process",owner="test-on-call",service="test-api",status="expired_running"} 1`)
	assert.Contains(t, body, `ecommerce_jobs_queue_depth{deployment_environment="test",job_type="media.process",owner="test-on-call",service="test-api",status="running"} 1`)
	assert.Contains(t, body, `ecommerce_jobs_dead_letters{deployment_environment="test",job_type="media.process",owner="test-on-call",service="test-api"} 1`)
	assert.Contains(t, body, "ecommerce_jobs_oldest_runnable_age_seconds")
	assert.Contains(t, body, `ecommerce_jobs_workers{deployment_environment="test",owner="test-on-call",service="test-api"} 1`)
	assert.Contains(t, body, `ecommerce_provider_calls_total{deployment_environment="test",operation="authorize",outcome="failed",owner="test-on-call",provider_type="payment",service="test-api"} 1`)
	assert.Contains(t, body, `ecommerce_provider_calls_total{deployment_environment="test",operation="unknown",outcome="unknown",owner="test-on-call",provider_type="unknown",service="test-api"} 1`)
	assert.NotContains(t, body, "attacker-controlled")
}

func TestGORMLoggerRecordsSuccessAndFailure(t *testing.T) {
	_, metrics := newTelemetryTestDB(t)
	delegate := logger.New(log.New(io.Discard, "", 0), logger.Config{LogLevel: logger.Silent})
	observer := NewGORMLogger(delegate, metrics)
	observer.Trace(context.Background(), time.Now().Add(-time.Millisecond), func() (string, int64) { return "SELECT 1", 1 }, nil)
	observer.Trace(context.Background(), time.Now().Add(-time.Millisecond), func() (string, int64) { return "SELECT 1", 0 }, errors.New("database unavailable"))

	output := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(output, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	assert.Contains(t, output.Body.String(), `ecommerce_db_queries_total{deployment_environment="test",outcome="success",owner="test-on-call",service="test-api"} 1`)
	assert.Contains(t, output.Body.String(), `ecommerce_db_queries_total{deployment_environment="test",outcome="error",owner="test-on-call",service="test-api"} 1`)
}

func TestTelemetryServerHealthAndReadiness(t *testing.T) {
	db, metrics := newTelemetryTestDB(t)
	pool, err := db.DB()
	require.NoError(t, err)
	server := NewServer("127.0.0.1:0", "/metrics", metrics.Handler(), ReadinessChecks{
		Pool: pool, Database: db,
		MigrationCheck: func(*gorm.DB) error { return nil },
		WorkersRunning: func() bool { return true },
	})
	server.SetReleaseID("release-test")
	errorsChannel, err := server.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
		select {
		case <-errorsChannel:
		case <-time.After(time.Second):
		}
	})

	for path, status := range map[string]int{"/healthz": http.StatusOK, "/readyz": http.StatusOK, "/metrics": http.StatusOK} {
		response, requestErr := http.Get("http://" + server.Address() + path)
		require.NoError(t, requestErr)
		assert.Equal(t, status, response.StatusCode)
		if path == "/healthz" || path == "/readyz" {
			assert.Equal(t, "release-test", response.Header.Get("X-Ecommerce-Release-ID"))
		}
		_ = response.Body.Close()
	}
	server.SetReady(false)
	response, err := http.Get("http://" + server.Address() + "/readyz")
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, response.StatusCode)
	_ = response.Body.Close()
}

func TestReadinessReportsDependencyFailures(t *testing.T) {
	db, metrics := newTelemetryTestDB(t)
	pool, err := db.DB()
	require.NoError(t, err)
	server := NewServer("127.0.0.1:0", "/metrics", metrics.Handler(), ReadinessChecks{
		Pool: pool, Database: db,
		MigrationCheck: func(*gorm.DB) error { return errors.New("schema behind") },
		WorkersRunning: func() bool { return false },
	})
	server.SetReady(true)
	response := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	assert.Equal(t, "not_ready", body.Status)
	assert.Equal(t, "ok", body.Checks["database"])
	assert.Equal(t, "failed", body.Checks["migrations"])
	assert.Equal(t, "failed", body.Checks["job_workers"])
}

func TestConfigureTracingDisabledDoesNotContactExporter(t *testing.T) {
	shutdown, err := ConfigureTracing(context.Background(), TracingConfig{Enabled: false})
	require.NoError(t, err)
	require.NoError(t, shutdown(context.Background()))
}

func TestConfigureTracingExportsCompletedSpan(t *testing.T) {
	received := make(chan struct{}, 1)
	collector := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "/v1/traces", request.URL.Path)
		select {
		case received <- struct{}{}:
		default:
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer collector.Close()

	shutdown, err := ConfigureTracing(context.Background(), TracingConfig{
		Enabled: true, Endpoint: collector.URL + "/v1/traces", SampleRatio: 1,
		Service: "test-api", Environment: "test",
	})
	require.NoError(t, err)
	_, span := otel.Tracer("test").Start(context.Background(), "completed")
	span.End()
	shutdownContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, shutdown(shutdownContext))
	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("OTLP collector did not receive the completed span")
	}
}

func TestObservabilityArtifactsAreParseableAndCoverFailureSimulations(t *testing.T) {
	for _, name := range []string{"api-health.json", "database-health.json", "background-jobs.json", "provider-health.json", "backup-restore.json"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "observability", "grafana", name))
		require.NoError(t, err)
		var dashboard map[string]any
		require.NoError(t, json.Unmarshal(raw, &dashboard), name)
		assert.NotEmpty(t, dashboard["uid"], name)
		assert.NotEmpty(t, dashboard["panels"], name)
	}

	rules, err := os.ReadFile(filepath.Join("..", "..", "observability", "prometheus", "alerts.yaml"))
	require.NoError(t, err)
	var ruleDocument map[string]any
	require.NoError(t, yaml.Unmarshal(rules, &ruleDocument))
	for _, alert := range []string{"EcommerceAPITargetDown", "EcommerceJobWorkersDown", "EcommerceJobBacklogCritical", "EcommerceProviderFailureStorm"} {
		assert.Contains(t, string(rules), "alert: "+alert)
	}
	backupRules, err := os.ReadFile(filepath.Join("..", "..", "observability", "prometheus", "backup-alerts-p3.yaml"))
	require.NoError(t, err)
	for _, alert := range []string{"EcommerceBackupFailed", "EcommerceBackupMissed", "EcommerceRestoreDrillFailed"} {
		assert.Contains(t, string(backupRules), "alert: "+alert)
	}

	simulations, err := os.ReadFile(filepath.Join("..", "..", "observability", "prometheus", "alerts.test.yaml"))
	require.NoError(t, err)
	var simulationDocument map[string]any
	require.NoError(t, yaml.Unmarshal(simulations, &simulationDocument))
	assert.Contains(t, string(simulations), "exp_alerts: []")
	assert.Contains(t, string(simulations), "EcommerceAPITargetDown")
	assert.Contains(t, string(simulations), "EcommerceJobWorkersDown")
	assert.Contains(t, string(simulations), "EcommerceJobBacklogCritical")
	assert.Contains(t, string(simulations), "EcommerceProviderFailureStorm")
}
