package config

import (
	"errors"
	"io"

	"github.com/hashicorp/go-multierror"
	"github.com/spf13/viper"
)

// Config holds the server configuration.
type Config struct {
	Server `mapstructure:"server"`
	Auth   `mapstructure:"auth"`
	SSH    `mapstructure:"ssh"`
}

// unmarshalled holds the raw config.
type unmarshalled struct {
	Server []Server `mapstructure:"server"`
	Auth   []Auth   `mapstructure:"auth"`
	SSH    []SSH    `mapstructure:"ssh"`
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
}

// SSH holds the configuration specific to signing ssh keys.
type SSH struct {
	SigningKey           string   `mapstructure:"signing_key"`
	AdditionalPrincipals []string `mapstructure:"additional_principals"`
	MaxAge               string   `mapstructure:"max_age"`
	Permissions          []string `mapstructure:"permissions"`
}

func verifyConfig(u *unmarshalled) error {
	var err error
	if len(u.SSH) == 0 {
		err = multierror.Append(errors.New("missing ssh config block"))
	}
	if len(u.Auth) == 0 {
		err = multierror.Append(errors.New("missing auth config block"))
	}
	if len(u.Server) == 0 {
		err = multierror.Append(errors.New("missing server config block"))
	}
	return err
}

// ReadConfig parses a JSON configuration file into a Config struct.
func ReadConfig(r io.Reader) (*Config, error) {
	u := &unmarshalled{}
	v := viper.New()
	v.SetConfigType("hcl")
	if err := v.ReadConfig(r); err != nil {
		return nil, err
	}
	if err := v.Unmarshal(u); err != nil {
		return nil, err
	}
	if err := verifyConfig(u); err != nil {
		return nil, err
	}
	return &Config{
		Server: u.Server[0],
		Auth:   u.Auth[0],
		SSH:    u.SSH[0],
	}, nil
}
