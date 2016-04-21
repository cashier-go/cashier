package google

import (
	"fmt"
	"testing"

	"github.com/nsheridan/cashier/server/auth"
	"github.com/nsheridan/cashier/server/config"
	"github.com/stretchr/testify/assert"
)

var (
	oauthClientID     = "id"
	oauthClientSecret = "secret"
	oauthCallbackURL  = "url"
	domain            = "example.com"
)

func TestNew(t *testing.T) {
	a := assert.New(t)

	p := newGoogle()
	g := p.(*Config)
	a.Equal(g.config.ClientID, oauthClientID)
	a.Equal(g.config.ClientSecret, oauthClientSecret)
	a.Equal(g.config.RedirectURL, oauthCallbackURL)
	a.Equal(g.domain, domain)
}

func TestStartSession(t *testing.T) {
	a := assert.New(t)

	p := newGoogle()
	s := p.StartSession("test_state")
	a.Equal(s.State, "test_state")
	a.Contains(s.AuthURL, "accounts.google.com/o/oauth2/auth")
	a.Contains(s.AuthURL, "state=test_state")
	a.Contains(s.AuthURL, fmt.Sprintf("hd=%s", domain))
	a.Contains(s.AuthURL, fmt.Sprintf("client_id=%s", oauthClientID))
}

func newGoogle() auth.Provider {
	c := &config.Auth{
		OauthClientID:     oauthClientID,
		OauthClientSecret: oauthClientSecret,
		OauthCallbackURL:  oauthCallbackURL,
		ProviderOpts:      make(map[string]interface{}),
	}
	c.ProviderOpts["domain"] = domain
	return New(c)
}
