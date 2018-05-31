package microsoft

import (
	"errors"
	"net/http"
	"strings"

	"github.com/nsheridan/cashier/server/auth"
	"github.com/nsheridan/cashier/server/config"
	"github.com/nsheridan/cashier/server/metrics"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

const (
	name = "microsoft"
)

// Config is an implementation of `auth.Provider` for authenticating using a
// Office 365 account.
type Config struct {
	config    *oauth2.Config
	tenant    string
	whitelist map[string]bool
}

var _ auth.Provider = (*Config)(nil)

// New creates a new Microsoft provider from a configuration.
func New(c *config.Auth) (*Config, error) {
	uw := make(map[string]bool)
	for _, u := range c.UsersWhitelist {
		uw[u] = true
	}
	if c.ProviderOpts["tenant"] == "" && len(uw) == 0 {
		return nil, errors.New("either Office 365 tenant or users whitelist must be specified")
	}

	return &Config{
		config: &oauth2.Config{
			ClientID:     c.OauthClientID,
			ClientSecret: c.OauthClientSecret,
			RedirectURL:  c.OauthCallbackURL,
			Endpoint:     microsoft.AzureADEndpoint(c.ProviderOpts["tenant"]),
		},
		tenant:    c.ProviderOpts["tenant"],
		whitelist: uw,
	}, nil
}

// A new oauth2 http client.
func (c *Config) newClient(token *oauth2.Token) *http.Client {
	return c.config.Client(oauth2.NoContext, token)
}

// Name returns the name of the provider.
func (c *Config) Name() string {
	return name
}

// Valid validates the oauth token.
func (c *Config) Valid(token *oauth2.Token) bool {
	if len(c.whitelist) > 0 && !c.whitelist[c.Email(token)] {
		return false
	}
	if !token.Valid() {
		return false
	}
	metrics.M.AuthValid.WithLabelValues("microsoft").Inc()
	return true
}

// Revoke disables the access token.
func (c *Config) Revoke(token *oauth2.Token) error {
	return nil
}

// StartSession retrieves an authentication endpoint from Microsoft.
func (c *Config) StartSession(state string) *auth.Session {
	return &auth.Session{
		AuthURL: c.config.AuthCodeURL(state, oauth2.SetAuthURLParam("hd", c.tenant)),
	}
}

// Exchange authorizes the session and returns an access token.
func (c *Config) Exchange(code string) (*oauth2.Token, error) {
	t, err := c.config.Exchange(oauth2.NoContext, code)
	if err == nil {
		metrics.M.AuthExchange.WithLabelValues("microsoft").Inc()
	}
	return t, err
}

// Email retrieves the email address of the user.
func (c *Config) Email(token *oauth2.Token) string {
	return "nobody@nowhere"
}

// Username retrieves the username portion of the user's email address.
func (c *Config) Username(token *oauth2.Token) string {
	return strings.Split(c.Email(token), "@")[0]
}
