package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	DBURL                          string `mapstructure:"DATABASE_URL"`
	AutoApplyMigrations            bool   `mapstructure:"AUTO_APPLY_MIGRATIONS"`
	Port                           string `mapstructure:"PORT"`
	JWTSecret                      string `mapstructure:"JWT_SECRET"`
	DisableLocalSignIn             bool   `mapstructure:"DISABLE_LOCAL_SIGN_IN"`
	DevMode                        bool   `mapstructure:"DEV_MODE"`
	PublicURL                      string `mapstructure:"PUBLIC_URL"`
	MediaRoot                      string `mapstructure:"MEDIA_ROOT"`
	MediaPublicURL                 string `mapstructure:"MEDIA_PUBLIC_URL"`
	ServeMedia                     bool   `mapstructure:"SERVE_MEDIA"`
	CheckoutPluginManifestsDir     string `mapstructure:"CHECKOUT_PLUGIN_MANIFESTS_DIR"`
	ProviderPluginManifestsDir     string `mapstructure:"PROVIDER_PLUGIN_MANIFESTS_DIR"`
	ProviderRuntimeEnvironment     string `mapstructure:"PROVIDER_RUNTIME_ENVIRONMENT"`
	ProviderCredentialsKeys        string `mapstructure:"PROVIDER_CREDENTIALS_KEYS"`
	ProviderCredentialsKeyVersion  string `mapstructure:"PROVIDER_CREDENTIALS_ACTIVE_KEY_VERSION"`
	ProviderReconciliationInterval string `mapstructure:"PROVIDER_RECONCILIATION_INTERVAL"`
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
	v.AutomaticEnv()
	for _, key := range configKeys {
		if bindErr := v.BindEnv(key); bindErr != nil {
			return config, fmt.Errorf("bind env %s: %w", key, bindErr)
		}
	}

	err = v.Unmarshal(&config)
	return
}
