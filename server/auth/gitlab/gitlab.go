package gitlab

import (
	"errors"
	"log"
	"strconv"

	"github.com/nsheridan/cashier/server/config"
	"github.com/nsheridan/cashier/server/metrics"

	gitlabapi "github.com/xanzy/go-gitlab"
	"golang.org/x/oauth2"
)

const (
	name = "gitlab"
)

// Config is an implementation of `auth.Provider` for authenticating using a
// Gitlab account.
type Config struct {
	config    *oauth2.Config
	baseurl   string
	group     string
	whitelist map[string]bool
	allusers  bool
}

// New creates a new Gitlab provider from a configuration.
func New(c *config.Auth) (*Config, error) {
	uw := make(map[string]bool)
	for _, u := range c.UsersWhitelist {
		uw[u] = true
	}
	allUsers, _ := strconv.ParseBool(c.ProviderOpts["allusers"])
	if !allUsers && c.ProviderOpts["group"] == "" && len(uw) == 0 {
		return nil, errors.New("gitlab_opts group and the users whitelist must not be both empty if allusers isn't true")
	}
	siteURL := "https://gitlab.com/"
	if c.ProviderOpts["siteurl"] != "" {
		siteURL = c.ProviderOpts["siteurl"]
		if siteURL[len(siteURL)-1] != '/' {
			return nil, errors.New("gitlab_opts siteurl must end in /")
		}
	} else {
		if allUsers {
			return nil, errors.New("gitlab_opts if allusers is set, siteurl must be set")
		}
	}
	oauth2.RegisterBrokenAuthHeaderProvider(siteURL)

	return &Config{
		config: &oauth2.Config{
			ClientID:     c.OauthClientID,
			ClientSecret: c.OauthClientSecret,
			RedirectURL:  c.OauthCallbackURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:  siteURL + "oauth/authorize",
				TokenURL: siteURL + "oauth/token",
			},
			Scopes: []string{
				"api",
			},
		},
		group:     c.ProviderOpts["group"],
		whitelist: uw,
		allusers:  allUsers,
		baseurl:   siteURL + "api/v3/",
	}, nil
}

// Name returns the name of the provider.
func (c *Config) Name() string {
	return name
}

// Valid validates the oauth token.
func (c *Config) Valid(token *oauth2.Token) bool {
	if !token.Valid() {
		log.Printf("Auth fail (oauth2 Valid failure)")
		return false
	}
	if c.allusers {
		log.Printf("Auth success (allusers)")
		metrics.M.AuthValid.WithLabelValues("gitlab").Inc()
		return true
	}
	if len(c.whitelist) > 0 && !c.whitelist[c.Username(token)] {
		log.Printf("Auth fail (not in whitelist)")
		return false
	}
	if c.group == "" {
		// There's no group and token is valid.  Can only reach
		// here if user whitelist is set and user is in whitelist.
		log.Printf("Auth success (no groups specified in server config)")
		metrics.M.AuthValid.WithLabelValues("gitlab").Inc()
		return true
	}
	client := gitlabapi.NewOAuthClient(nil, token.AccessToken)
	client.SetBaseURL(c.baseurl)
	groups, _, err := client.Groups.SearchGroup(c.group)
	if err != nil {
		log.Printf("Auth failure (error fetching groups: %s)", err)
		return false
	}
	for _, g := range groups {
		if g.Path == c.group {
			metrics.M.AuthValid.WithLabelValues("gitlab").Inc()
			log.Printf("Auth success (in allowed group)")
			return true
		}
	}
	log.Printf("Auth failure (not in allowed groups)")
	return false
}

// Revoke is a no-op revoke method. Gitlab doesn't allow token
// revocation - tokens live for an hour.
// Returns nil to satisfy the Provider interface.
func (c *Config) Revoke(token *oauth2.Token) error {
	return nil
}

// StartSession retrieves an authentication endpoint from Gitlab.
func (c *Config) StartSession(state string) string {
	return c.config.AuthCodeURL(state)
}

// Exchange authorizes the session and returns an access token.
func (c *Config) Exchange(code string) (*oauth2.Token, error) {
	t, err := c.config.Exchange(oauth2.NoContext, code)
	if err == nil {
		metrics.M.AuthExchange.WithLabelValues("gitlab").Inc()
	}
	return t, err
}

// Username retrieves the username of the Gitlab user.
func (c *Config) Username(token *oauth2.Token) string {
	client := gitlabapi.NewOAuthClient(nil, token.AccessToken)
	client.SetBaseURL(c.baseurl)
	u, _, err := client.Users.CurrentUser()
	if err != nil {
		return ""
	}
	return u.Username
}
