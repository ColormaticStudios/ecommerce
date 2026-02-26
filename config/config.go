package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	DBURL                      string `mapstructure:"DATABASE_URL"`
	Port                       string `mapstructure:"PORT"`
	JWTSecret                  string `mapstructure:"JWT_SECRET"`
	DisableLocalSignIn         bool   `mapstructure:"DISABLE_LOCAL_SIGN_IN"`
	OIDCProvider               string `mapstructure:"OIDC_PROVIDER"`
	OIDCClientID               string `mapstructure:"OIDC_CLIENT_ID"`
	OIDCClientSecret           string `mapstructure:"OIDC_CLIENT_SECRET"`
	OIDCRedirectURI            string `mapstructure:"OIDC_REDIRECT_URI"`
	DevMode                    bool   `mapstructure:"DEV_MODE"`
	PublicURL                  string `mapstructure:"PUBLIC_URL"`
	MediaRoot                  string `mapstructure:"MEDIA_ROOT"`
	MediaPublicURL             string `mapstructure:"MEDIA_PUBLIC_URL"`
	ServeMedia                 bool   `mapstructure:"SERVE_MEDIA"`
	CheckoutPluginManifestsDir string `mapstructure:"CHECKOUT_PLUGIN_MANIFESTS_DIR"`
}

var configKeys = []string{
	"DATABASE_URL",
	"PORT",
	"JWT_SECRET",
	"DISABLE_LOCAL_SIGN_IN",
	"OIDC_PROVIDER",
	"OIDC_CLIENT_ID",
	"OIDC_CLIENT_SECRET",
	"OIDC_REDIRECT_URI",
	"DEV_MODE",
	"PUBLIC_URL",
	"MEDIA_ROOT",
	"MEDIA_PUBLIC_URL",
	"SERVE_MEDIA",
	"CHECKOUT_PLUGIN_MANIFESTS_DIR",
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
	v.AutomaticEnv()
	for _, key := range configKeys {
		if bindErr := v.BindEnv(key); bindErr != nil {
			return config, fmt.Errorf("bind env %s: %w", key, bindErr)
		}
	}

	err = v.Unmarshal(&config)
	return
}
