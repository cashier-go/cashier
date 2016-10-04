package config

import (
	"errors"
	"io"
	"os"
	"strconv"

	"github.com/hashicorp/go-multierror"
	"github.com/nsheridan/cashier/server/helpers/vault"
	"github.com/spf13/viper"
)

// Config holds the server configuration.
type Config struct {
	Server *Server `mapstructure:"server"`
	Auth   *Auth   `mapstructure:"auth"`
	SSH    *SSH    `mapstructure:"ssh"`
	AWS    *AWS    `mapstructure:"aws"`
	Vault  *Vault  `mapstructure:"vault"`
}

// unmarshalled holds the raw config.
type unmarshalled struct {
	Server []Server `mapstructure:"server"`
	Auth   []Auth   `mapstructure:"auth"`
	SSH    []SSH    `mapstructure:"ssh"`
	AWS    []AWS    `mapstructure:"aws"`
	Vault  []Vault  `mapstructure:"vault"`
}

// Server holds the configuration specific to the web server and sessions.
type Server struct {
	UseTLS       bool   `mapstructure:"use_tls"`
	TLSKey       string `mapstructure:"tls_key"`
	TLSCert      string `mapstructure:"tls_cert"`
	Addr         string `mapstructure:"address"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	CookieSecret string `mapstructure:"cookie_secret"`
	CSRFSecret   string `mapstructure:"csrf_secret"`
	HTTPLogFile  string `mapstructure:"http_logfile"`
	Datastore    string `mapstructure:"datastore"`
}

// Auth holds the configuration specific to the OAuth provider.
type Auth struct {
	OauthClientID     string            `mapstructure:"oauth_client_id"`
	OauthClientSecret string            `mapstructure:"oauth_client_secret"`
	OauthCallbackURL  string            `mapstructure:"oauth_callback_url"`
	Provider          string            `mapstructure:"provider"`
	ProviderOpts      map[string]string `mapstructure:"provider_opts"`
	UsersWhitelist    []string          `mapstructure:"users_whitelist"`
}

// SSH holds the configuration specific to signing ssh keys.
type SSH struct {
	SigningKey           string   `mapstructure:"signing_key"`
	AdditionalPrincipals []string `mapstructure:"additional_principals"`
	MaxAge               string   `mapstructure:"max_age"`
	Permissions          []string `mapstructure:"permissions"`
}

// AWS holds Amazon AWS configuration.
// AWS can also be configured using SDK methods.
type AWS struct {
	Region    string `mapstructure:"region"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
}

// Vault holds Hashicorp Vault configuration.
type Vault struct {
	Address string `mapstructure:"address"`
	Token   string `mapstructure:"token"`
}

func verifyConfig(u *unmarshalled) error {
	var err error
	if len(u.SSH) == 0 {
		err = multierror.Append(errors.New("missing ssh config section"))
	}
	if len(u.Auth) == 0 {
		err = multierror.Append(errors.New("missing auth config section"))
	}
	if len(u.Server) == 0 {
		err = multierror.Append(errors.New("missing server config section"))
	}
	if len(u.AWS) == 0 {
		// AWS config is optional
		u.AWS = append(u.AWS, AWS{})
	}
	if len(u.Vault) == 0 {
		// Vault config is optional
		u.Vault = append(u.Vault, Vault{})
	}
	return err
}

func setFromEnv(u *unmarshalled) {
	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err == nil {
		u.Server[0].Port = port
	}
	if os.Getenv("DATASTORE") != "" {
		u.Server[0].Datastore = os.Getenv("DATASTORE")
	}
	if os.Getenv("OAUTH_CLIENT_ID") != "" {
		u.Auth[0].OauthClientID = os.Getenv("OAUTH_CLIENT_ID")
	}
	if os.Getenv("OAUTH_CLIENT_SECRET") != "" {
		u.Auth[0].OauthClientSecret = os.Getenv("OAUTH_CLIENT_SECRET")
	}
	if os.Getenv("CSRF_SECRET") != "" {
		u.Server[0].CSRFSecret = os.Getenv("CSRF_SECRET")
	}
	if os.Getenv("COOKIE_SECRET") != "" {
		u.Server[0].CookieSecret = os.Getenv("COOKIE_SECRET")
	}
}

func setFromVault(u *unmarshalled) error {
	if len(u.Vault) == 0 || u.Vault[0].Token == "" || u.Vault[0].Address == "" {
		return nil
	}
	v, err := vault.NewClient(u.Vault[0].Address, u.Vault[0].Token)
	if err != nil {
		return err
	}
	get := func(value string) (string, error) {
		if value[:7] == "/vault/" {
			return v.Read(value)
		}
		return value, nil
	}
	if len(u.Auth) > 0 {
		u.Auth[0].OauthClientID, err = get(u.Auth[0].OauthClientID)
		if err != nil {
			err = multierror.Append(err)
		}
		u.Auth[0].OauthClientSecret, err = get(u.Auth[0].OauthClientSecret)
		if err != nil {
			err = multierror.Append(err)
		}
	}
	if len(u.Server) > 0 {
		u.Server[0].CSRFSecret, err = get(u.Server[0].CSRFSecret)
		if err != nil {
			err = multierror.Append(err)
		}
		u.Server[0].CookieSecret, err = get(u.Server[0].CookieSecret)
		if err != nil {
			err = multierror.Append(err)
		}
	}
	if len(u.AWS) > 0 {
		u.AWS[0].AccessKey, err = get(u.AWS[0].AccessKey)
		if err != nil {
			err = multierror.Append(err)
		}
		u.AWS[0].SecretKey, err = get(u.AWS[0].SecretKey)
		if err != nil {
			err = multierror.Append(err)
		}
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
	setFromEnv(u)
	if err := setFromVault(u); err != nil {
		return nil, err
	}
	if err := verifyConfig(u); err != nil {
		return nil, err
	}
	return &Config{
		Server: &u.Server[0],
		Auth:   &u.Auth[0],
		SSH:    &u.SSH[0],
		AWS:    &u.AWS[0],
		Vault:  &u.Vault[0],
	}, nil
}
