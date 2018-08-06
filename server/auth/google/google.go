package google

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/nsheridan/cashier/server/auth"
	"github.com/nsheridan/cashier/server/config"
	"github.com/nsheridan/cashier/server/metrics"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googleapi "google.golang.org/api/oauth2/v2"
)

const (
	revokeURL = "https://accounts.google.com/o/oauth2/revoke?token=%s"
	name      = "google"
)

// Config is an implementation of `auth.Provider` for authenticating using a
// Google account.
type Config struct {
	config    *oauth2.Config
	domain    string
	whitelist map[string]bool
}

var _ auth.Provider = (*Config)(nil)

// New creates a new Google provider from a configuration.
func New(c *config.Auth) (*Config, error) {
	uw := make(map[string]bool)
	for _, u := range c.UsersWhitelist {
		uw[u] = true
	}
	if c.ProviderOpts["domain"] == "" && len(uw) == 0 {
		return nil, errors.New("either Google Apps domain or users whitelist must be specified")
	}

	return &Config{
		config: &oauth2.Config{
			ClientID:     c.OauthClientID,
			ClientSecret: c.OauthClientSecret,
			RedirectURL:  c.OauthCallbackURL,
			Endpoint:     google.Endpoint,
			Scopes:       []string{googleapi.UserinfoEmailScope, googleapi.UserinfoProfileScope},
		},
		domain:    c.ProviderOpts["domain"],
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
	svc, err := googleapi.New(c.newClient(token))
	if err != nil {
		return false
	}
	t := svc.Tokeninfo()
	t.AccessToken(token.AccessToken)
	ti, err := t.Do()
	if err != nil {
		return false
	}
	if ti.Audience != c.config.ClientID {
		return false
	}
	ui, err := svc.Userinfo.Get().Do()
	if err != nil {
		return false
	}
	if c.domain != "" && ui.Hd != c.domain {
		return false
	}
	metrics.M.AuthValid.WithLabelValues("google").Inc()
	return true
}

// Revoke disables the access token.
func (c *Config) Revoke(token *oauth2.Token) error {
	h := c.newClient(token)
	_, err := h.Get(fmt.Sprintf(revokeURL, token.AccessToken))
	return err
}

// TODO: Implement me
func (c *Config) Principals(token *oauth2.Token) []string {
	return []string{}
}

// StartSession retrieves an authentication endpoint from Google.
func (c *Config) StartSession(state string) *auth.Session {
	return &auth.Session{
		AuthURL: c.config.AuthCodeURL(state, oauth2.SetAuthURLParam("hd", c.domain)),
	}
}

// Exchange authorizes the session and returns an access token.
func (c *Config) Exchange(code string) (*oauth2.Token, error) {
	t, err := c.config.Exchange(oauth2.NoContext, code)
	if err == nil {
		metrics.M.AuthExchange.WithLabelValues("google").Inc()
	}
	return t, err
}

// Email retrieves the email address of the user.
func (c *Config) Email(token *oauth2.Token) string {
	svc, err := googleapi.New(c.newClient(token))
	if err != nil {
		return ""
	}
	ui, err := svc.Userinfo.Get().Do()
	if err != nil {
		return ""
	}
	return ui.Email
}

// Username retrieves the username portion of the user's email address.
func (c *Config) Username(token *oauth2.Token) string {
	return strings.Split(c.Email(token), "@")[0]
}
