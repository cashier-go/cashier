package gitlab

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
	allusers          = ""
	siteurl           = "https://exampleorg/"
	group             = "exampleorg"
)

func TestNew(t *testing.T) {
	a := assert.New(t)

	p, _ := newGitlab()
	g := p.(*Config)
	a.Equal(g.config.ClientID, oauthClientID)
	a.Equal(g.config.ClientSecret, oauthClientSecret)
	a.Equal(g.config.RedirectURL, oauthCallbackURL)
}

func TestNewBrokenSiteURL(t *testing.T) {
	siteurl = "https://exampleorg"
	a := assert.New(t)

	_, err := newGitlab()
	a.EqualError(err, "gitlab_opts siteurl must end in /")

	siteurl = "https://exampleorg/"
}

func TestBadAllUsers(t *testing.T) {
	allusers = "true"
	siteurl = ""
	a := assert.New(t)

	_, err := newGitlab()
	a.EqualError(err, "gitlab_opts if allusers is set, siteurl must be set")

	allusers = ""
	siteurl = "https://exampleorg/"
}

func TestGoodAllUsers(t *testing.T) {
	allusers = "true"
	a := assert.New(t)

	p, _ := newGitlab()
	s := p.StartSession("test_state")
	a.Contains(s, "exampleorg/oauth/authorize")
	a.Contains(s, "state=test_state")
	a.Contains(s, fmt.Sprintf("client_id=%s", oauthClientID))

	allusers = ""
}

func TestNewEmptyGroupList(t *testing.T) {
	group = ""
	a := assert.New(t)

	_, err := newGitlab()
	a.EqualError(err, "gitlab_opts group and the users whitelist must not be both empty if allusers isn't true")

	group = "exampleorg"
}

func TestStartSession(t *testing.T) {
	a := assert.New(t)

	p, _ := newGitlab()
	s := p.StartSession("test_state")
	a.Contains(s, "exampleorg/oauth/authorize")
	a.Contains(s, "state=test_state")
	a.Contains(s, fmt.Sprintf("client_id=%s", oauthClientID))
}

func newGitlab() (auth.Provider, error) {
	c := &config.Auth{
		OauthClientID:     oauthClientID,
		OauthClientSecret: oauthClientSecret,
		OauthCallbackURL:  oauthCallbackURL,
		ProviderOpts: map[string]string{
			"group":    group,
			"siteurl":  siteurl,
			"allusers": allusers,
		},
	}
	return New(c)
}
