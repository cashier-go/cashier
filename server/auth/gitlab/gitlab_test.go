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
	groups            = "devops,cashier"
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
	groups = ""
	a := assert.New(t)

	_, err := newGitlab()
	a.EqualError(err, "gitlab_opts groups and the users whitelist must not be both empty if allusers isn't true")

	groups = "exampleorg"
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
			"groups":   groups,
			"siteurl":  siteurl,
			"allusers": allusers,
		},
	}
	return New(c)
}

func Test_providedAuthGroups(t *testing.T) {
	tests := []struct {
		name  string
		input *config.Auth
		want  []string
	}{
		{
			name:  "no-group-config",
			input: &config.Auth{ProviderOpts: map[string]string{}},
			want:  []string{},
		},
		{
			name:  "no-spaces",
			input: &config.Auth{ProviderOpts: map[string]string{"groups": "a,b,c"}},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "spaces-at-edges",
			input: &config.Auth{ProviderOpts: map[string]string{"groups": " a,b,c "}},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "random-spaces",
			input: &config.Auth{ProviderOpts: map[string]string{"groups": " a,  b  ,   c "}},
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "deprecated-config-only",
			input: &config.Auth{ProviderOpts: map[string]string{"group": "a"}},
			want:  []string{"a"},
		},
		{
			name:  "deprecated-config-alongside-groups",
			input: &config.Auth{ProviderOpts: map[string]string{"group": "a", "groups": "b, c"}},
			want:  []string{"b", "c", "a"},
		},
		{
			name:  "deprecated-config-alongside-groups-duplicated",
			input: &config.Auth{ProviderOpts: map[string]string{"group": "a", "groups": "a, c"}},
			want:  []string{"a", "c"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, providedAuthGroups(tt.input))
		})
	}
}
