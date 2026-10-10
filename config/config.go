package config

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	SearchMaxConcurrent           int `mapstructure:"SEARCH_MAX_CONCURRENT"`
	SearchTimeoutMS               int `mapstructure:"SEARCH_TIMEOUT_MS"`
	SearchCircuitFailureThreshold int `mapstructure:"SEARCH_CIRCUIT_FAILURE_THRESHOLD"`
	SearchCircuitOpenMS           int `mapstructure:"SEARCH_CIRCUIT_OPEN_MS"`
	SearchReindexQueueLimit       int `mapstructure:"SEARCH_REINDEX_QUEUE_LIMIT"`

	DBURL                          string        `mapstructure:"DATABASE_URL"`
	AutoApplyMigrations            bool          `mapstructure:"AUTO_APPLY_MIGRATIONS"`
	Port                           string        `mapstructure:"PORT"`
	JWTSecret                      string        `mapstructure:"JWT_SECRET"`
	DisableLocalSignIn             bool          `mapstructure:"DISABLE_LOCAL_SIGN_IN"`
	DevMode                        bool          `mapstructure:"DEV_MODE"`
	PublicURL                      string        `mapstructure:"PUBLIC_URL"`
	MediaRoot                      string        `mapstructure:"MEDIA_ROOT"`
	MediaPublicURL                 string        `mapstructure:"MEDIA_PUBLIC_URL"`
	ServeMedia                     bool          `mapstructure:"SERVE_MEDIA"`
	CheckoutPluginManifestsDir     string        `mapstructure:"CHECKOUT_PLUGIN_MANIFESTS_DIR"`
	ProviderPluginManifestsDir     string        `mapstructure:"PROVIDER_PLUGIN_MANIFESTS_DIR"`
	ProviderRuntimeEnvironment     string        `mapstructure:"PROVIDER_RUNTIME_ENVIRONMENT"`
	ProviderCredentialsKeys        string        `mapstructure:"PROVIDER_CREDENTIALS_KEYS"`
	ProviderCredentialsKeyVersion  string        `mapstructure:"PROVIDER_CREDENTIALS_ACTIVE_KEY_VERSION"`
	ProviderReconciliationInterval string        `mapstructure:"PROVIDER_RECONCILIATION_INTERVAL"`
	CMSInvalidationWebhookURL      string        `mapstructure:"CMS_INVALIDATION_WEBHOOK_URL"`
	HTTPReadHeaderTimeout          time.Duration `mapstructure:"HTTP_READ_HEADER_TIMEOUT"`
	HTTPReadTimeout                time.Duration `mapstructure:"HTTP_READ_TIMEOUT"`
	HTTPWriteTimeout               time.Duration `mapstructure:"HTTP_WRITE_TIMEOUT"`
	HTTPIdleTimeout                time.Duration `mapstructure:"HTTP_IDLE_TIMEOUT"`
	HTTPShutdownTimeout            time.Duration `mapstructure:"HTTP_SHUTDOWN_TIMEOUT"`
	ProviderExecutionTimeout       time.Duration `mapstructure:"PROVIDER_EXECUTION_TIMEOUT"`
	ProviderQueryTimeout           time.Duration `mapstructure:"PROVIDER_QUERY_TIMEOUT"`
	ProviderCompensationTimeout    time.Duration `mapstructure:"PROVIDER_COMPENSATION_TIMEOUT"`
	ProviderLeaseDuration          time.Duration `mapstructure:"PROVIDER_LEASE_DURATION"`
	TelemetryEnabled               bool          `mapstructure:"TELEMETRY_ENABLED"`
	TelemetryBindAddress           string        `mapstructure:"TELEMETRY_BIND_ADDRESS"`
	MetricsPath                    string        `mapstructure:"METRICS_PATH"`
	TracingEnabled                 bool          `mapstructure:"TRACING_ENABLED"`
	OTLPTraceEndpoint              string        `mapstructure:"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"`
	TraceSampleRatio               float64       `mapstructure:"TRACE_SAMPLE_RATIO"`
	TrustRequestIDs                bool          `mapstructure:"TRUST_REQUEST_IDS"`
	AlertOwner                     string        `mapstructure:"ALERT_OWNER"`
	AlertService                   string        `mapstructure:"ALERT_SERVICE"`
	DeploymentEnvironment          string        `mapstructure:"DEPLOYMENT_ENVIRONMENT"`
	ReleaseID                      string        `mapstructure:"RELEASE_ID"`
	JobWorkerConcurrency           int           `mapstructure:"JOB_WORKER_CONCURRENCY"`
	JobPollInterval                time.Duration `mapstructure:"JOB_POLL_INTERVAL"`
	JobLeaseDuration               time.Duration `mapstructure:"JOB_LEASE_DURATION"`
	JobMaxAttempts                 int           `mapstructure:"JOB_MAX_ATTEMPTS"`
	JobRetryBaseDelay              time.Duration `mapstructure:"JOB_RETRY_BASE_DELAY"`
	JobRetryMaxDelay               time.Duration `mapstructure:"JOB_RETRY_MAX_DELAY"`
}

var configKeys = []string{
	"SEARCH_MAX_CONCURRENT",
	"SEARCH_TIMEOUT_MS",
	"SEARCH_CIRCUIT_FAILURE_THRESHOLD",
	"SEARCH_CIRCUIT_OPEN_MS",
	"SEARCH_REINDEX_QUEUE_LIMIT",

	"DATABASE_URL",
	"AUTO_APPLY_MIGRATIONS",
	"PORT",
	"JWT_SECRET",
	"DISABLE_LOCAL_SIGN_IN",
	"DEV_MODE",
	"PUBLIC_URL",
	"MEDIA_ROOT",
	"MEDIA_PUBLIC_URL",
	"SERVE_MEDIA",
	"CHECKOUT_PLUGIN_MANIFESTS_DIR",
	"PROVIDER_PLUGIN_MANIFESTS_DIR",
	"PROVIDER_RUNTIME_ENVIRONMENT",
	"PROVIDER_CREDENTIALS_KEYS",
	"PROVIDER_CREDENTIALS_ACTIVE_KEY_VERSION",
	"PROVIDER_RECONCILIATION_INTERVAL",
	"CMS_INVALIDATION_WEBHOOK_URL",
	"HTTP_READ_HEADER_TIMEOUT",
	"HTTP_READ_TIMEOUT",
	"HTTP_WRITE_TIMEOUT",
	"HTTP_IDLE_TIMEOUT",
	"HTTP_SHUTDOWN_TIMEOUT",
	"PROVIDER_EXECUTION_TIMEOUT",
	"PROVIDER_QUERY_TIMEOUT",
	"PROVIDER_COMPENSATION_TIMEOUT",
	"PROVIDER_LEASE_DURATION",
	"TELEMETRY_ENABLED",
	"TELEMETRY_BIND_ADDRESS",
	"METRICS_PATH",
	"TRACING_ENABLED",
	"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
	"TRACE_SAMPLE_RATIO",
	"TRUST_REQUEST_IDS",
	"ALERT_OWNER",
	"ALERT_SERVICE",
	"DEPLOYMENT_ENVIRONMENT",
	"RELEASE_ID",
	"JOB_WORKER_CONCURRENCY",
	"JOB_POLL_INTERVAL",
	"JOB_LEASE_DURATION",
	"JOB_MAX_ATTEMPTS",
	"JOB_RETRY_BASE_DELAY",
	"JOB_RETRY_MAX_DELAY",
}

func LoadConfig() (config Config, err error) {
	v := viper.New()

	// Lowest precedence: optional config.toml for non-secret defaults.
	v.SetConfigName("config")
	v.SetConfigType("toml")
	v.AddConfigPath(".")
	if readErr := v.ReadInConfig(); readErr != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(readErr, &notFound) {
			return config, fmt.Errorf("read config.toml: %w", readErr)
		}
	}

	// Next precedence: optional .env key/value config.
	// Parsing via Viper (instead of mutating process env) keeps runtime env
	// variables highest-priority when AutomaticEnv is enabled below.
	if envBytes, readErr := os.ReadFile(".env"); readErr == nil {
		envV := viper.New()
		envV.SetConfigType("env")
		if parseErr := envV.ReadConfig(bytes.NewBuffer(envBytes)); parseErr != nil {
			return config, fmt.Errorf("parse .env: %w", parseErr)
		}
		if mergeErr := v.MergeConfigMap(envV.AllSettings()); mergeErr != nil {
			return config, fmt.Errorf("merge .env: %w", mergeErr)
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return config, fmt.Errorf("read .env: %w", readErr)
	}

	// Highest precedence: runtime environment variables.
	v.SetDefault("AUTO_APPLY_MIGRATIONS", false)
	v.SetDefault("SEARCH_MAX_CONCURRENT", 50)
	v.SetDefault("SEARCH_TIMEOUT_MS", 2000)
	v.SetDefault("SEARCH_CIRCUIT_FAILURE_THRESHOLD", 5)
	v.SetDefault("SEARCH_CIRCUIT_OPEN_MS", 30000)
	v.SetDefault("SEARCH_REINDEX_QUEUE_LIMIT", 1)

	v.SetDefault("HTTP_READ_HEADER_TIMEOUT", "10s")
	v.SetDefault("HTTP_READ_TIMEOUT", "5m")
	v.SetDefault("HTTP_WRITE_TIMEOUT", "5m")
	v.SetDefault("HTTP_IDLE_TIMEOUT", "2m")
	v.SetDefault("HTTP_SHUTDOWN_TIMEOUT", "30s")
	v.SetDefault("PROVIDER_EXECUTION_TIMEOUT", "30s")
	v.SetDefault("PROVIDER_QUERY_TIMEOUT", "15s")
	v.SetDefault("PROVIDER_COMPENSATION_TIMEOUT", "1m")
	v.SetDefault("PROVIDER_LEASE_DURATION", "2m")
	v.SetDefault("TELEMETRY_ENABLED", true)
	v.SetDefault("TELEMETRY_BIND_ADDRESS", "127.0.0.1:9090")
	v.SetDefault("METRICS_PATH", "/metrics")
	v.SetDefault("TRACING_ENABLED", false)
	v.SetDefault("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://127.0.0.1:4318/v1/traces")
	v.SetDefault("TRACE_SAMPLE_RATIO", 0.1)
	v.SetDefault("TRUST_REQUEST_IDS", false)
	v.SetDefault("ALERT_SERVICE", "ecommerce-api")
	v.SetDefault("DEPLOYMENT_ENVIRONMENT", "development")
	v.SetDefault("RELEASE_ID", "development")
	v.SetDefault("JOB_WORKER_CONCURRENCY", 4)
	v.SetDefault("JOB_POLL_INTERVAL", "1s")
	v.SetDefault("JOB_LEASE_DURATION", "2m")
	v.SetDefault("JOB_MAX_ATTEMPTS", 5)
	v.SetDefault("JOB_RETRY_BASE_DELAY", "5s")
	v.SetDefault("JOB_RETRY_MAX_DELAY", "15m")
	v.AutomaticEnv()
	for _, key := range configKeys {
		if bindErr := v.BindEnv(key); bindErr != nil {
			return config, fmt.Errorf("bind env %s: %w", key, bindErr)
		}
	}

	if err := v.Unmarshal(&config); err != nil {
		return config, fmt.Errorf("decode config: %w", err)
	}
	if err := config.validate(); err != nil {
		return config, err
	}
	return config, nil
}

func (c Config) validate() error {
	if c.SearchMaxConcurrent < 1 || c.SearchMaxConcurrent > 512 {
		return errors.New("SEARCH_MAX_CONCURRENT must be between 1 and 512")
	}
	if c.SearchTimeoutMS < 10 || c.SearchTimeoutMS > 60000 {
		return errors.New("SEARCH_TIMEOUT_MS must be between 10 and 60000")
	}
	if c.SearchCircuitFailureThreshold < 1 || c.SearchCircuitFailureThreshold > 100 {
		return errors.New("SEARCH_CIRCUIT_FAILURE_THRESHOLD must be between 1 and 100")
	}
	if c.SearchCircuitOpenMS < 10 || c.SearchCircuitOpenMS > 600000 {
		return errors.New("SEARCH_CIRCUIT_OPEN_MS must be between 10 and 600000")
	}
	if c.SearchReindexQueueLimit < 1 || c.SearchReindexQueueLimit > 20 {
		return errors.New("SEARCH_REINDEX_QUEUE_LIMIT must be between 1 and 20")
	}

	if c.MetricsPath == "" || c.MetricsPath[0] != '/' || len(c.MetricsPath) > 255 || strings.TrimSpace(c.MetricsPath) != c.MetricsPath {
		return errors.New("METRICS_PATH must be an absolute path no longer than 255 characters")
	}
	metricsURL, metricsURLErr := url.ParseRequestURI(c.MetricsPath)
	if metricsURLErr != nil || metricsURL.Path != c.MetricsPath || metricsURL.RawQuery != "" || strings.ContainsAny(c.MetricsPath, "{}") {
		return errors.New("METRICS_PATH must be a literal URL path without a query or wildcard")
	}
	if c.MetricsPath == "/healthz" || c.MetricsPath == "/readyz" {
		return errors.New("METRICS_PATH must not conflict with /healthz or /readyz")
	}
	if c.TelemetryEnabled {
		if strings.TrimSpace(c.TelemetryBindAddress) != c.TelemetryBindAddress || c.TelemetryBindAddress == "" {
			return errors.New("TELEMETRY_BIND_ADDRESS must be a non-empty host:port")
		}
		if _, _, err := net.SplitHostPort(c.TelemetryBindAddress); err != nil {
			return fmt.Errorf("TELEMETRY_BIND_ADDRESS must be a valid host:port: %w", err)
		}
	}
	if c.TracingEnabled && !c.TelemetryEnabled {
		return errors.New("TRACING_ENABLED requires TELEMETRY_ENABLED=true")
	}
	if c.TraceSampleRatio < 0 || c.TraceSampleRatio > 1 {
		return errors.New("TRACE_SAMPLE_RATIO must be between 0 and 1")
	}
	if c.TracingEnabled {
		endpoint, err := url.Parse(c.OTLPTraceEndpoint)
		if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
			return errors.New("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT must be an absolute HTTP(S) URL")
		}
	}
	if strings.TrimSpace(c.AlertService) == "" {
		return errors.New("ALERT_SERVICE must not be empty")
	}
	if strings.TrimSpace(c.DeploymentEnvironment) == "" {
		return errors.New("DEPLOYMENT_ENVIRONMENT must not be empty")
	}
	if strings.TrimSpace(c.ReleaseID) == "" || len(c.ReleaseID) > 128 || strings.ContainsAny(c.ReleaseID, "\r\n") {
		return errors.New("RELEASE_ID must be non-empty, no longer than 128 characters, and contain no line breaks")
	}
	if c.JobWorkerConcurrency < 1 || c.JobWorkerConcurrency > 128 {
		return errors.New("JOB_WORKER_CONCURRENCY must be between 1 and 128")
	}
	if c.JobMaxAttempts < 1 || c.JobMaxAttempts > 100 {
		return errors.New("JOB_MAX_ATTEMPTS must be between 1 and 100")
	}
	if c.JobRetryMaxDelay < c.JobRetryBaseDelay {
		return errors.New("JOB_RETRY_MAX_DELAY must be greater than or equal to JOB_RETRY_BASE_DELAY")
	}
	durations := []struct {
		name  string
		value time.Duration
	}{
		{"HTTP_READ_HEADER_TIMEOUT", c.HTTPReadHeaderTimeout},
		{"HTTP_READ_TIMEOUT", c.HTTPReadTimeout},
		{"HTTP_WRITE_TIMEOUT", c.HTTPWriteTimeout},
		{"HTTP_IDLE_TIMEOUT", c.HTTPIdleTimeout},
		{"HTTP_SHUTDOWN_TIMEOUT", c.HTTPShutdownTimeout},
		{"PROVIDER_EXECUTION_TIMEOUT", c.ProviderExecutionTimeout},
		{"PROVIDER_QUERY_TIMEOUT", c.ProviderQueryTimeout},
		{"PROVIDER_COMPENSATION_TIMEOUT", c.ProviderCompensationTimeout},
		{"PROVIDER_LEASE_DURATION", c.ProviderLeaseDuration},
		{"JOB_POLL_INTERVAL", c.JobPollInterval},
		{"JOB_LEASE_DURATION", c.JobLeaseDuration},
		{"JOB_RETRY_BASE_DELAY", c.JobRetryBaseDelay},
		{"JOB_RETRY_MAX_DELAY", c.JobRetryMaxDelay},
	}
	for _, duration := range durations {
		if duration.value <= 0 {
			return fmt.Errorf("%s must be a positive duration", duration.name)
		}
	}
	return nil
}
