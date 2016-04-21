package config

import "github.com/spf13/viper"

// Config holds the values from the json config file.
type Config struct {
	Server Server `mapstructure:"server"`
	Auth   Auth   `mapstructure:"auth"`
	SSH    SSH    `mapstructure:"ssh"`
}

// Server holds the configuration specific to the web server and sessions.
type Server struct {
	UseTLS       bool   `mapstructure:"use_tls"`
	TLSKey       string `mapstructure:"tls_key"`
	TLSCert      string `mapstructure:"tls_cert"`
	Port         int    `mapstructure:"port"`
	CookieSecret string `mapstructure:"cookie_secret"`
}

// Auth holds the configuration specific to the OAuth provider.
type Auth struct {
	OauthClientID     string            `mapstructure:"oauth_client_id"`
	OauthClientSecret string            `mapstructure:"oauth_client_secret"`
	OauthCallbackURL  string            `mapstructure:"oauth_callback_url"`
	Provider          string            `mapstructure:"provider"`
	ProviderOpts      map[string]string `mapstructure:"provider_opts"`
	JWTSigningKey     string            `mapstructure:"jwt_signing_key"`
}

// SSH holds the configuration specific to signing ssh keys.
type SSH struct {
	SigningKey           string   `mapstructure:"signing_key"`
	AdditionalPrincipals []string `mapstructure:"additional_principals"`
	MaxAge               string   `mapstructure:"max_age"`
	Permissions          []string `mapstructure:"permissions"`
}

// ReadConfig parses a JSON configuration file into a Config struct.
func ReadConfig(filename string) (*Config, error) {
	config := &Config{}
	v := viper.New()
	v.SetConfigFile(filename)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	if err := v.Unmarshal(config); err != nil {
		return nil, err
	}
	return config, nil
}
