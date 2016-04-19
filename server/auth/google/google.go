package google

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/nsheridan/cashier/server/auth"
	"github.com/nsheridan/cashier/server/config"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googleapi "google.golang.org/api/oauth2/v2"
)

const (
	revokeURL = "https://accounts.google.com/o/oauth2/revoke?token=%s"
	name      = "google"
)

type Config struct {
	config *oauth2.Config
	domain string
}

func New(c *config.Auth) auth.Provider {
	return &Config{
		config: &oauth2.Config{
			ClientID:     c.OauthClientID,
			ClientSecret: c.OauthClientSecret,
			RedirectURL:  c.OauthCallbackURL,
			Endpoint:     google.Endpoint,
			Scopes:       []string{googleapi.UserinfoEmailScope, googleapi.UserinfoProfileScope},
		},
		domain: c.GoogleOpts["domain"].(string),
	}
}

func (c *Config) newClient(token *oauth2.Token) *http.Client {
	return c.config.Client(oauth2.NoContext, token)
}

func (c *Config) Name() string {
	return name
}

func (c *Config) Valid(token *oauth2.Token) bool {
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
	ui, err := svc.Userinfo.Get().Do()
	if err != nil {
		return false
	}
	switch {
	case ti.Audience != c.config.ClientID:
	case ui.Hd != c.domain:
		return false
	}
	return true
}

func (c *Config) Revoke(token *oauth2.Token) error {
	h := c.newClient(token)
	_, err := h.Get(fmt.Sprintf(revokeURL, token.AccessToken))
	return err
}

func (c *Config) StartSession(state string) *auth.Session {
	return &auth.Session{
		AuthURL: c.config.AuthCodeURL(state, oauth2.SetAuthURLParam("hd", c.domain)),
		State:   state,
	}
}

func (c *Config) Exchange(code string) (*oauth2.Token, error) {
	return c.config.Exchange(oauth2.NoContext, code)
}

func (c *Config) Username(token *oauth2.Token) string {
	svc, err := googleapi.New(c.newClient(token))
	if err != nil {
		return ""
	}
	ui, err := svc.Userinfo.Get().Do()
	if err != nil {
		return ""
	}
	return strings.Split(ui.Email, "@")[0]
}
