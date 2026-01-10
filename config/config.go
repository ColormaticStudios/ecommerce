package config

import "github.com/spf13/viper"

type Config struct {
	DBURL              string `mapstructure:"DATABASE_URL"`
	Port               string `mapstructure:"PORT"`
	JWTSecret          string `mapstructure:"JWT_SECRET"`
	DisableLocalSignIn string `mapstructure:"DISABLE_LOCAL_SIGN_IN"`
	OIDCProvider       string `mapstructure:"OIDC_PROVIDER"`
	OIDCClientID       string `mapstructure:"OIDC_CLIENT_ID"`
	OIDCClientSecret   string `mapstructure:"OIDC_CLIENT_SECRET"`
	OIDCRedirectURI    string `mapstructure:"OIDC_REDIRECT_URI"`
	DevMode            bool   `mapstructure:"DEV_MODE"`
	PublicURL          string `mapstructure:"PUBLIC_URL"`
	MediaRoot          string `mapstructure:"MEDIA_ROOT"`
	MediaPublicURL     string `mapstructure:"MEDIA_PUBLIC_URL"`
	ServeMedia         bool   `mapstructure:"SERVE_MEDIA"`
}

func LoadConfig() (config Config, err error) {
	viper.AddConfigPath(".")
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	err = viper.Unmarshal(&config)
	return
}
