package microsoft

import (
	"fmt"
	"testing"

	"github.com/nsheridan/cashier/server/config"
	"github.com/stretchr/testify/assert"
)

var (
	oauthClientID     = "id"
	oauthClientSecret = "secret"
	oauthCallbackURL  = "url"
	tenant            = "example.com"
	users             = []string{"user"}
)

func TestNew(t *testing.T) {
	a := assert.New(t)
	p, err := newMicrosoft()
	a.NoError(err)
	a.Equal(p.config.ClientID, oauthClientID)
	a.Equal(p.config.ClientSecret, oauthClientSecret)
	a.Equal(p.config.RedirectURL, oauthCallbackURL)
	a.Equal(p.tenant, tenant)
	a.Equal(p.whitelist, map[string]bool{"user": true})
}

func TestWhitelist(t *testing.T) {
	c := &config.Auth{
		OauthClientID:     oauthClientID,
		OauthClientSecret: oauthClientSecret,
		OauthCallbackURL:  oauthCallbackURL,
		ProviderOpts:      map[string]string{"tenant": ""},
		UsersWhitelist:    []string{},
	}
	if _, err := New(c); err == nil {
		t.Error("creating a provider without a tenant set should return an error")
	}
	// Set a user whitelist but no tenant
	c.UsersWhitelist = users
	if _, err := New(c); err != nil {
		t.Error("creating a provider with users but no tenant should not return an error")
	}
	// Unset the user whitelist and set a tenant
	c.UsersWhitelist = []string{}
	c.ProviderOpts = map[string]string{"tenant": tenant}
	if _, err := New(c); err != nil {
		t.Error("creating a provider with a tenant set but without a user whitelist should not return an error")
	}
}

func TestStartSession(t *testing.T) {
	a := assert.New(t)

	p, err := newMicrosoft()
	a.NoError(err)
	s := p.StartSession("test_state")
	a.Contains(s, fmt.Sprintf("login.microsoftonline.com/%s/oauth2/v2.0/authorize", tenant))
}

func newMicrosoft() (*Config, error) {
	c := &config.Auth{
		OauthClientID:     oauthClientID,
		OauthClientSecret: oauthClientSecret,
		OauthCallbackURL:  oauthCallbackURL,
		ProviderOpts:      map[string]string{"tenant": tenant},
		UsersWhitelist:    users,
	}
	return New(c)
}
