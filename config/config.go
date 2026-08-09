package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
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
}

var configKeys = []string{
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
	v.SetDefault("HTTP_READ_HEADER_TIMEOUT", "10s")
	v.SetDefault("HTTP_READ_TIMEOUT", "5m")
	v.SetDefault("HTTP_WRITE_TIMEOUT", "5m")
	v.SetDefault("HTTP_IDLE_TIMEOUT", "2m")
	v.SetDefault("HTTP_SHUTDOWN_TIMEOUT", "30s")
	v.SetDefault("PROVIDER_EXECUTION_TIMEOUT", "30s")
	v.SetDefault("PROVIDER_QUERY_TIMEOUT", "15s")
	v.SetDefault("PROVIDER_COMPENSATION_TIMEOUT", "1m")
	v.SetDefault("PROVIDER_LEASE_DURATION", "2m")
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
	}
	for _, duration := range durations {
		if duration.value <= 0 {
			return fmt.Errorf("%s must be a positive duration", duration.name)
		}
	}
	return nil
}
