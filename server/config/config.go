package config

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/nsheridan/cashier/server/helpers/vault"
	"github.com/spf13/viper"
)

// Config holds the final server configuration.
type Config struct {
	Server *Server
	Auth   *Auth
	SSH    *SSH
	AWS    *AWS
	Vault  *Vault
}

// unmarshalled holds the raw config.
// The original hcl config is a series of slices. The config is unmarshalled from hcl into this structure and from there
// we perform some validation checks, other overrides and then produce a final Config struct.
type unmarshalled struct {
	Server []Server `mapstructure:"server"`
	Auth   []Auth   `mapstructure:"auth"`
	SSH    []SSH    `mapstructure:"ssh"`
	AWS    []AWS    `mapstructure:"aws"`
	Vault  []Vault  `mapstructure:"vault"`
}

// Database config
type Database map[string]string

// Server holds the configuration specific to the web server and sessions.
type Server struct {
	UseTLS       bool     `mapstructure:"use_tls"`
	TLSKey       string   `mapstructure:"tls_key"`
	TLSCert      string   `mapstructure:"tls_cert"`
	Addr         string   `mapstructure:"address"`
	Port         int      `mapstructure:"port"`
	User         string   `mapstructure:"user"`
	CookieSecret string   `mapstructure:"cookie_secret"`
	CSRFSecret   string   `mapstructure:"csrf_secret"`
	HTTPLogFile  string   `mapstructure:"http_logfile"`
	Database     Database `mapstructure:"database"`
	Datastore    string   `mapstructure:"datastore"` // Deprecated.
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
		err = multierror.Append(err, errors.New("missing ssh config section"))
	}
	if len(u.Auth) == 0 {
		err = multierror.Append(err, errors.New("missing auth config section"))
	}
	if len(u.Server) == 0 {
		err = multierror.Append(err, errors.New("missing server config section"))
	}
	if len(u.AWS) == 0 {
		// AWS config is optional
		u.AWS = append(u.AWS, AWS{})
	}
	if len(u.Vault) == 0 {
		// Vault config is optional
		u.Vault = append(u.Vault, Vault{})
	}
	if u.Server[0].Datastore != "" {
		log.Println("The `datastore` option has been deprecated in favour of the `database` option. You should update your config.")
		log.Println("The new config (passwords have been redacted) should look something like:")
		fmt.Printf("server {\n  database {\n")
		for k, v := range u.Server[0].Database {
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
	return err
}

func convertDatastoreConfig(u *unmarshalled) {
	// Convert the deprecated 'datastore' config to the new 'database' config.
	if len(u.Server[0].Database) == 0 && u.Server[0].Datastore != "" {
		c := u.Server[0].Datastore
		engine := strings.Split(c, ":")[0]
		switch engine {
		case "mysql", "mongo":
			s := strings.SplitN(c, ":", 4)
			engine, user, passwd, addrs := s[0], s[1], s[2], s[3]
			u.Server[0].Database = map[string]string{
				"type":     engine,
				"username": user,
				"password": passwd,
				"address":  addrs,
			}
		case "sqlite":
			s := strings.Split(c, ":")
			u.Server[0].Database = map[string]string{"type": s[0], "filename": s[1]}
		case "mem":
			u.Server[0].Database = map[string]string{"type": "mem"}
		}
	}
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
		if len(value) > 0 && value[:7] == "/vault/" {
			return v.Read(value)
		}
		return value, nil
	}
	var errors error
	if len(u.Auth) > 0 {
		u.Auth[0].OauthClientID, err = get(u.Auth[0].OauthClientID)
		if err != nil {
			errors = multierror.Append(errors, err)
		}
		u.Auth[0].OauthClientSecret, err = get(u.Auth[0].OauthClientSecret)
		if err != nil {
			errors = multierror.Append(errors, err)
		}
	}
	if len(u.Server) > 0 {
		u.Server[0].CSRFSecret, err = get(u.Server[0].CSRFSecret)
		if err != nil {
			errors = multierror.Append(errors, err)
		}
		u.Server[0].CookieSecret, err = get(u.Server[0].CookieSecret)
		if err != nil {
			errors = multierror.Append(errors, err)
		}
		if len(u.Server[0].Database) > 0 {
			u.Server[0].Database["password"], err = get(u.Server[0].Database["password"])
			if err != nil {
				errors = multierror.Append(errors, err)
			}
		}
	}
	if len(u.AWS) > 0 {
		u.AWS[0].AccessKey, err = get(u.AWS[0].AccessKey)
		if err != nil {
			errors = multierror.Append(errors, err)
		}
		u.AWS[0].SecretKey, err = get(u.AWS[0].SecretKey)
		if err != nil {
			errors = multierror.Append(errors, err)
		}
	}
	return errors
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
	convertDatastoreConfig(u)
	if err := verifyConfig(u); err != nil {
		return nil, err
	}
	c := &Config{
		Server: &u.Server[0],
		Auth:   &u.Auth[0],
		SSH:    &u.SSH[0],
		AWS:    &u.AWS[0],
		Vault:  &u.Vault[0],
	}
	return c, nil
}
