package config

import (
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/mapstructure"
	"github.com/nsheridan/cashier/server/helpers/vault"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

// Config holds the final server configuration.
type Config struct {
	Server *Server `mapstructure:"server"`
	Auth   *Auth   `mapstructure:"auth"`
	SSH    *SSH    `mapstructure:"ssh"`
	AWS    *AWS    `mapstructure:"aws"`
	Vault  *Vault  `mapstructure:"vault"`
}

// Database holds database configuration.
type Database map[string]string

// Server holds the configuration specific to the web server and sessions.
type Server struct {
	UseTLS                bool     `mapstructure:"use_tls"`
	TLSKey                string   `mapstructure:"tls_key"`
	TLSCert               string   `mapstructure:"tls_cert"`
	LetsEncryptServername string   `mapstructure:"letsencrypt_servername"`
	LetsEncryptCache      string   `mapstructure:"letsencrypt_cachedir"`
	Addr                  string   `mapstructure:"address"`
	Port                  int      `mapstructure:"port"`
	User                  string   `mapstructure:"user"`
	CookieSecret          string   `mapstructure:"cookie_secret"`
	CSRFSecret            string   `mapstructure:"csrf_secret"`
	HTTPLogFile           string   `mapstructure:"http_logfile"`
	Database              Database `mapstructure:"database"`
	Datastore             string   `mapstructure:"datastore"` // Deprecated. TODO: remove.
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

func verifyConfig(c *Config) error {
	var err error
	if c.SSH == nil {
		err = multierror.Append(err, errors.New("missing ssh config section"))
	}
	if c.Auth == nil {
		err = multierror.Append(err, errors.New("missing auth config section"))
	}
	if c.Server == nil {
		err = multierror.Append(err, errors.New("missing server config section"))
	}
	return err
}

func convertDatastoreConfig(c *Config) {
	// Convert the deprecated 'datastore' config to the new 'database' config.
	if c.Server != nil && c.Server.Datastore != "" {
		conf := c.Server.Datastore
		engine := strings.Split(conf, ":")[0]
		switch engine {
		case "mysql", "mongo":
			s := strings.SplitN(conf, ":", 4)
			engine, user, passwd, addrs := s[0], s[1], s[2], s[3]
			c.Server.Database = map[string]string{
				"type":     engine,
				"username": user,
				"password": passwd,
				"address":  addrs,
			}
		case "sqlite":
			s := strings.Split(conf, ":")
			c.Server.Database = map[string]string{"type": s[0], "filename": s[1]}
		case "mem":
			c.Server.Database = map[string]string{"type": "mem"}
		}
		log.Println("The `datastore` option has been deprecated in favour of the `database` option. You should update your config.")
		log.Println("The new config (passwords have been redacted) should look something like:")
		fmt.Printf("server {\n  database {\n")
		for k, v := range c.Server.Database {
			if v == "" {
				continue
			}
			if k == "password" {
				fmt.Printf("    password = \"[ REDACTED ]\"\n")
				continue
			}
			fmt.Printf("    %s = \"%s\"\n", k, v)
		}
		fmt.Printf("  }\n}\n")
	}
}

func setFromEnvironment(c *Config) {
	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err == nil {
		c.Server.Port = port
	}
	if os.Getenv("DATASTORE") != "" {
		c.Server.Datastore = os.Getenv("DATASTORE")
	}
	if os.Getenv("OAUTH_CLIENT_ID") != "" {
		c.Auth.OauthClientID = os.Getenv("OAUTH_CLIENT_ID")
	}
	if os.Getenv("OAUTH_CLIENT_SECRET") != "" {
		c.Auth.OauthClientSecret = os.Getenv("OAUTH_CLIENT_SECRET")
	}
	if os.Getenv("CSRF_SECRET") != "" {
		c.Server.CSRFSecret = os.Getenv("CSRF_SECRET")
	}
	if os.Getenv("COOKIE_SECRET") != "" {
		c.Server.CookieSecret = os.Getenv("COOKIE_SECRET")
	}
}

func setFromVault(c *Config) error {
	if c.Vault == nil || c.Vault.Token == "" || c.Vault.Address == "" {
		return nil
	}
	v, err := vault.NewClient(c.Vault.Address, c.Vault.Token)
	if err != nil {
		return errors.Wrap(err, "vault error")
	}
	var errs error
	get := func(value string) string {
		if strings.HasPrefix(value, "/vault/") {
			s, err := v.Read(value)
			if err != nil {
				errs = multierror.Append(errs, err)
			}
			return s
		}
		return value
	}
	c.Auth.OauthClientID = get(c.Auth.OauthClientID)
	c.Auth.OauthClientSecret = get(c.Auth.OauthClientSecret)
	c.Server.CSRFSecret = get(c.Server.CSRFSecret)
	c.Server.CookieSecret = get(c.Server.CookieSecret)
	if len(c.Server.Database) != 0 {
		c.Server.Database["password"] = get(c.Server.Database["password"])
	}
	if c.AWS != nil {
		c.AWS.AccessKey = get(c.AWS.AccessKey)
		c.AWS.SecretKey = get(c.AWS.SecretKey)
	}
	return errors.Wrap(errs, "errors reading from vault")
}

// Unmarshal the config into a *Config
func decode() (*Config, error) {
	var errs error
	config := &Config{}
	configPieces := map[string]interface{}{
		"auth":   &config.Auth,
		"aws":    &config.AWS,
		"server": &config.Server,
		"ssh":    &config.SSH,
		"vault":  &config.Vault,
	}
	for key, val := range configPieces {
		conf, ok := viper.Get(key).([]map[string]interface{})
		if !ok {
			continue
		}
		if err := mapstructure.WeakDecode(conf[0], val); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return config, errs
}

// ReadConfig parses a hcl configuration file into a Config struct.
func ReadConfig(r io.Reader) (*Config, error) {
	viper.SetConfigType("hcl")
	if err := viper.ReadConfig(r); err != nil {
		return nil, errors.Wrap(err, "unable to read config")
	}
	config, err := decode()
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse config")
	}
	if err := setFromVault(config); err != nil {
		return nil, err
	}
	setFromEnvironment(config)
	convertDatastoreConfig(config)
	if err := verifyConfig(config); err != nil {
		return nil, errors.Wrap(err, "unable to verify config")
	}
	return config, nil
}
