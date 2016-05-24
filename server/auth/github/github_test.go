package github

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
	organization      = "exampleorg"
)

func TestNew(t *testing.T) {
	a := assert.New(t)

	p, _ := newGithub()
	g := p.(*Config)
	a.Equal(g.config.ClientID, oauthClientID)
	a.Equal(g.config.ClientSecret, oauthClientSecret)
	a.Equal(g.config.RedirectURL, oauthCallbackURL)
	a.Equal(g.organization, organization)
}

func TestNewEmptyOrganization(t *testing.T) {
	organization = ""
	a := assert.New(t)

	_, err := newGithub()
	a.EqualError(err, "github_opts organization must not be empty")

	organization = "exampleorg"
}

func TestStartSession(t *testing.T) {
	a := assert.New(t)

	p, _ := newGithub()
	s := p.StartSession("test_state")
	a.Equal(s.State, "test_state")
	a.Contains(s.AuthURL, "github.com/login/oauth/authorize")
	a.Contains(s.AuthURL, "state=test_state")
	a.Contains(s.AuthURL, fmt.Sprintf("client_id=%s", oauthClientID))
}

func newGithub() (auth.Provider, error) {
	c := &config.Auth{
		OauthClientID:     oauthClientID,
		OauthClientSecret: oauthClientSecret,
		OauthCallbackURL:  oauthCallbackURL,
		ProviderOpts:      map[string]string{"organization": organization},
	}
	return New(c)
}
