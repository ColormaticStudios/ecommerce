package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigPrecedence(t *testing.T) {
	tempDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	toml := `PORT = "1111"
DEV_MODE = true
PUBLIC_URL = "toml.example.com"
`
	if err := os.WriteFile(filepath.Join(tempDir, "config.toml"), []byte(toml), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	dotenv := `PORT="2222"
PUBLIC_URL="envfile.example.com"
DISABLE_LOCAL_SIGN_IN="true"
AUTO_APPLY_MIGRATIONS="true"
`
	if err := os.WriteFile(filepath.Join(tempDir, ".env"), []byte(dotenv), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	t.Setenv("PORT", "3333")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if cfg.Port != "3333" {
		t.Fatalf("expected PORT from environment variable, got %q", cfg.Port)
	}
	if cfg.PublicURL != "envfile.example.com" {
		t.Fatalf("expected PUBLIC_URL from .env, got %q", cfg.PublicURL)
	}
	if !cfg.DevMode {
		t.Fatalf("expected DEV_MODE from config.toml fallback to be true")
	}
	if !cfg.DisableLocalSignIn {
		t.Fatalf("expected DISABLE_LOCAL_SIGN_IN from .env to parse as true")
	}
	if !cfg.AutoApplyMigrations {
		t.Fatalf("expected AUTO_APPLY_MIGRATIONS from .env to parse as true")
	}
}

func TestLoadConfigDurationDefaults(t *testing.T) {
	useEmptyConfigDir(t)
	unsetEnvironment(t, configKeys...)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	expected := map[string]struct {
		got  time.Duration
		want time.Duration
	}{
		"HTTP_READ_HEADER_TIMEOUT":      {cfg.HTTPReadHeaderTimeout, 10 * time.Second},
		"HTTP_READ_TIMEOUT":             {cfg.HTTPReadTimeout, 5 * time.Minute},
		"HTTP_WRITE_TIMEOUT":            {cfg.HTTPWriteTimeout, 5 * time.Minute},
		"HTTP_IDLE_TIMEOUT":             {cfg.HTTPIdleTimeout, 2 * time.Minute},
		"HTTP_SHUTDOWN_TIMEOUT":         {cfg.HTTPShutdownTimeout, 30 * time.Second},
		"PROVIDER_EXECUTION_TIMEOUT":    {cfg.ProviderExecutionTimeout, 30 * time.Second},
		"PROVIDER_QUERY_TIMEOUT":        {cfg.ProviderQueryTimeout, 15 * time.Second},
		"PROVIDER_COMPENSATION_TIMEOUT": {cfg.ProviderCompensationTimeout, time.Minute},
		"PROVIDER_LEASE_DURATION":       {cfg.ProviderLeaseDuration, 2 * time.Minute},
		"JOB_POLL_INTERVAL":             {cfg.JobPollInterval, time.Second},
		"JOB_LEASE_DURATION":            {cfg.JobLeaseDuration, 2 * time.Minute},
		"JOB_RETRY_BASE_DELAY":          {cfg.JobRetryBaseDelay, 5 * time.Second},
		"JOB_RETRY_MAX_DELAY":           {cfg.JobRetryMaxDelay, 15 * time.Minute},
	}
	for name, duration := range expected {
		if duration.got != duration.want {
			t.Errorf("%s = %s, want %s", name, duration.got, duration.want)
		}
	}
}

func TestLoadConfigReliabilityDefaults(t *testing.T) {
	useEmptyConfigDir(t)
	unsetEnvironment(t, configKeys...)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if !cfg.TelemetryEnabled {
		t.Error("TelemetryEnabled = false, want true")
	}
	if cfg.MetricsPath != "/metrics" {
		t.Errorf("MetricsPath = %q, want /metrics", cfg.MetricsPath)
	}
	if cfg.TelemetryBindAddress != "127.0.0.1:9090" {
		t.Errorf("TelemetryBindAddress = %q, want 127.0.0.1:9090", cfg.TelemetryBindAddress)
	}
	if cfg.TracingEnabled {
		t.Error("TracingEnabled = true, want false")
	}
	if cfg.OTLPTraceEndpoint != "http://127.0.0.1:4318/v1/traces" || cfg.TraceSampleRatio != 0.1 {
		t.Errorf("unexpected tracing defaults: endpoint=%q ratio=%v", cfg.OTLPTraceEndpoint, cfg.TraceSampleRatio)
	}
	if cfg.TrustRequestIDs {
		t.Error("TrustRequestIDs = true, want safe default false")
	}
	if cfg.AlertService != "ecommerce-api" {
		t.Errorf("AlertService = %q, want ecommerce-api", cfg.AlertService)
	}
	if cfg.DeploymentEnvironment != "development" {
		t.Errorf("DeploymentEnvironment = %q, want development", cfg.DeploymentEnvironment)
	}
	if cfg.ReleaseID != "development" {
		t.Errorf("ReleaseID = %q, want development", cfg.ReleaseID)
	}
	if cfg.JobWorkerConcurrency != 4 || cfg.JobMaxAttempts != 5 {
		t.Errorf("job integer defaults = concurrency %d, max attempts %d", cfg.JobWorkerConcurrency, cfg.JobMaxAttempts)
	}
}

func TestLoadConfigReliabilityOverrides(t *testing.T) {
	useEmptyConfigDir(t)
	unsetEnvironment(t, configKeys...)
	t.Setenv("TELEMETRY_ENABLED", "false")
	t.Setenv("METRICS_PATH", "/internal/metrics")
	t.Setenv("TELEMETRY_BIND_ADDRESS", "0.0.0.0:9191")
	t.Setenv("TRACING_ENABLED", "false")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "https://collector.example.com/v1/traces")
	t.Setenv("TRACE_SAMPLE_RATIO", "0.25")
	t.Setenv("TRUST_REQUEST_IDS", "true")
	t.Setenv("ALERT_OWNER", "commerce-on-call")
	t.Setenv("ALERT_SERVICE", "store-api")
	t.Setenv("DEPLOYMENT_ENVIRONMENT", "staging")
	t.Setenv("RELEASE_ID", "git-abc123")
	t.Setenv("JOB_WORKER_CONCURRENCY", "8")
	t.Setenv("JOB_POLL_INTERVAL", "250ms")
	t.Setenv("JOB_LEASE_DURATION", "3m")
	t.Setenv("JOB_MAX_ATTEMPTS", "7")
	t.Setenv("JOB_RETRY_BASE_DELAY", "10s")
	t.Setenv("JOB_RETRY_MAX_DELAY", "20m")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.TelemetryEnabled {
		t.Error("TelemetryEnabled = true, want false")
	}
	if cfg.MetricsPath != "/internal/metrics" || !cfg.TrustRequestIDs || cfg.AlertOwner != "commerce-on-call" || cfg.AlertService != "store-api" || cfg.DeploymentEnvironment != "staging" || cfg.ReleaseID != "git-abc123" {
		t.Fatalf("unexpected reliability config: %+v", cfg)
	}
	if cfg.TelemetryBindAddress != "0.0.0.0:9191" || cfg.TracingEnabled || cfg.OTLPTraceEndpoint != "https://collector.example.com/v1/traces" || cfg.TraceSampleRatio != 0.25 {
		t.Fatalf("unexpected telemetry config: %+v", cfg)
	}
	if cfg.JobWorkerConcurrency != 8 || cfg.JobPollInterval != 250*time.Millisecond || cfg.JobLeaseDuration != 3*time.Minute || cfg.JobMaxAttempts != 7 || cfg.JobRetryBaseDelay != 10*time.Second || cfg.JobRetryMaxDelay != 20*time.Minute {
		t.Fatalf("unexpected job runtime config: %+v", cfg)
	}
}

func TestLoadConfigRejectsInvalidReliabilityMetadata(t *testing.T) {
	for _, test := range []struct {
		name  string
		key   string
		value string
	}{
		{name: "relative metrics path", key: "METRICS_PATH", value: "metrics"},
		{name: "metrics path conflict", key: "METRICS_PATH", value: "/readyz"},
		{name: "metrics path query", key: "METRICS_PATH", value: "/metrics?format=openmetrics"},
		{name: "metrics path wildcard", key: "METRICS_PATH", value: "/metrics/{tenant}"},
		{name: "invalid telemetry bind", key: "TELEMETRY_BIND_ADDRESS", value: "127.0.0.1"},
		{name: "invalid trace ratio", key: "TRACE_SAMPLE_RATIO", value: "1.1"},
		{name: "empty alert service", key: "ALERT_SERVICE", value: " "},
		{name: "empty deployment environment", key: "DEPLOYMENT_ENVIRONMENT", value: " "},
		{name: "invalid release ID", key: "RELEASE_ID", value: " "},
		{name: "zero worker concurrency", key: "JOB_WORKER_CONCURRENCY", value: "0"},
		{name: "too many attempts", key: "JOB_MAX_ATTEMPTS", value: "101"},
		{name: "retry range reversed", key: "JOB_RETRY_BASE_DELAY", value: "20m"},
	} {
		t.Run(test.name, func(t *testing.T) {
			useEmptyConfigDir(t)
			unsetEnvironment(t, configKeys...)
			t.Setenv(test.key, test.value)
			_, err := LoadConfig()
			if err == nil || !strings.Contains(err.Error(), test.key) {
				t.Fatalf("LoadConfig error = %v, want error identifying %s", err, test.key)
			}
		})
	}
}

func TestLoadConfigValidatesEnabledTracing(t *testing.T) {
	useEmptyConfigDir(t)
	unsetEnvironment(t, configKeys...)
	t.Setenv("TRACING_ENABLED", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "collector-without-scheme")
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") {
		t.Fatalf("LoadConfig error = %v, want invalid tracing endpoint", err)
	}
}

func TestLoadConfigRejectsTracingWhenTelemetryDisabled(t *testing.T) {
	useEmptyConfigDir(t)
	unsetEnvironment(t, configKeys...)
	t.Setenv("TELEMETRY_ENABLED", "false")
	t.Setenv("TRACING_ENABLED", "true")
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "TRACING_ENABLED") {
		t.Fatalf("LoadConfig error = %v, want tracing dependency error", err)
	}
}

func TestLoadConfigDurationOverrides(t *testing.T) {
	useEmptyConfigDir(t)
	unsetEnvironment(t, configKeys...)
	t.Setenv("HTTP_READ_HEADER_TIMEOUT", "7s")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "45s")
	t.Setenv("PROVIDER_EXECUTION_TIMEOUT", "90s")
	t.Setenv("PROVIDER_QUERY_TIMEOUT", "20s")
	t.Setenv("PROVIDER_COMPENSATION_TIMEOUT", "2m")
	t.Setenv("PROVIDER_LEASE_DURATION", "3m")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.HTTPReadHeaderTimeout != 7*time.Second {
		t.Errorf("HTTPReadHeaderTimeout = %s, want 7s", cfg.HTTPReadHeaderTimeout)
	}
	if cfg.HTTPShutdownTimeout != 45*time.Second {
		t.Errorf("HTTPShutdownTimeout = %s, want 45s", cfg.HTTPShutdownTimeout)
	}
	if cfg.ProviderExecutionTimeout != 90*time.Second {
		t.Errorf("ProviderExecutionTimeout = %s, want 90s", cfg.ProviderExecutionTimeout)
	}
	if cfg.ProviderQueryTimeout != 20*time.Second {
		t.Errorf("ProviderQueryTimeout = %s, want 20s", cfg.ProviderQueryTimeout)
	}
	if cfg.ProviderCompensationTimeout != 2*time.Minute {
		t.Errorf("ProviderCompensationTimeout = %s, want 2m", cfg.ProviderCompensationTimeout)
	}
	if cfg.ProviderLeaseDuration != 3*time.Minute {
		t.Errorf("ProviderLeaseDuration = %s, want 3m", cfg.ProviderLeaseDuration)
	}
}

func TestLoadConfigRejectsInvalidDurations(t *testing.T) {
	for _, test := range []struct {
		name  string
		key   string
		value string
	}{
		{name: "malformed", key: "HTTP_READ_TIMEOUT", value: "eventually"},
		{name: "zero", key: "HTTP_SHUTDOWN_TIMEOUT", value: "0s"},
		{name: "negative", key: "PROVIDER_QUERY_TIMEOUT", value: "-1s"},
	} {
		t.Run(test.name, func(t *testing.T) {
			useEmptyConfigDir(t)
			unsetEnvironment(t, configKeys...)
			t.Setenv(test.key, test.value)

			_, err := LoadConfig()
			if err == nil {
				t.Fatalf("LoadConfig accepted %s=%q", test.key, test.value)
			}
			if !strings.Contains(err.Error(), test.key) && test.value != "eventually" {
				t.Fatalf("LoadConfig error %q does not identify %s", err, test.key)
			}
		})
	}
}

func TestLoadConfigAllowsMissingFiles(t *testing.T) {
	tempDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	t.Setenv("PORT", "8080")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig with env-only setup: %v", err)
	}
	if cfg.Port != "8080" {
		t.Fatalf("expected PORT from env-only setup, got %q", cfg.Port)
	}
	if cfg.AutoApplyMigrations {
		t.Fatalf("expected AUTO_APPLY_MIGRATIONS default to be false")
	}
}

func useEmptyConfigDir(t *testing.T) {
	t.Helper()
	tempDir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
}

func unsetEnvironment(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		value, exists := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
		cleanupKey := key
		cleanupValue := value
		cleanupExists := exists
		t.Cleanup(func() {
			if cleanupExists {
				_ = os.Setenv(cleanupKey, cleanupValue)
			} else {
				_ = os.Unsetenv(cleanupKey)
			}
		})
	}
}
