package gitlab

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/nsheridan/cashier/server/config"
	"github.com/nsheridan/cashier/server/metrics"

	"golang.org/x/oauth2"
)

const (
	name = "gitlab"
)

// Config is an implementation of `auth.Provider` for authenticating using a
// Gitlab account.
type Config struct {
	config    *oauth2.Config
	group     string
	whitelist map[string]bool
	allusers  bool
	apiurl    string
	log       bool
}

// Note on Gitlab REST API calls.  We don't parse errors because it's
// kind of a pain:
// https://gitlab.com/help/api/README.md#data-validation-and-error-reporting
// The two v4 api calls used are /user and /groups/:group/members/:uid
// https://gitlab.com/help/api/users.md#for-normal-users-1
// https://gitlab.com/help/api/members.md#get-a-member-of-a-group-or-project
type serviceUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type serviceGroupMember struct {
	ID          int    `json:"id"`
	State       string `json:"state"`
	AccessLevel int    `json:"access_level"`
}

func (c *Config) logMsg(message string) {
	if c.log {
		log.Print(message)
	}
}

// A new oauth2 http client.
func (c *Config) newClient(token *oauth2.Token) *http.Client {
	return c.config.Client(oauth2.NoContext, token)
}

// Gets info on the current user.
func (c *Config) getUser(token *oauth2.Token) *serviceUser {
	client := c.newClient(token)
	url := c.apiurl + "user"
	resp, err := client.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		if c.log {
			var body bytes.Buffer
			io.Copy(&body, resp.Body)
			log.Printf("Gitlab error(http: %d) getting user: '%s'",
				resp.StatusCode, body.String())
			return nil
		}
	}
	var user serviceUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil
	}
	return &user
}

// Gets current user group membership info.
func (c *Config) checkGroupMembership(token *oauth2.Token, uid int, group string) bool {
	client := c.newClient(token)
	log.Printf("Checking group membership...")
	url := fmt.Sprintf("%sgroups/%s/members/%d", c.apiurl, group, uid)
	resp, err := client.Get(url)
	if err != nil {
		c.logMsg(fmt.Sprintf("Failed to get groups: %s", err))
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		if c.log {
			var body bytes.Buffer
			io.Copy(&body, resp.Body)
			log.Printf("Gitlab error(http: %d) getting user membership: '%s'",
				resp.StatusCode, body.String())
			return false
		}
	}
	var m serviceGroupMember
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		c.logMsg(fmt.Sprintf("Failed to parse groups: %s", err))
		return false
	}
	return m.ID == uid
}

// New creates a new Gitlab provider from a configuration.
func New(c *config.Auth) (*Config, error) {
	logOpt, _ := strconv.ParseBool(c.ProviderOpts["log"])
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
	// TODO: Should make sure siteURL is just the host bit.
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
		apiurl:    siteURL + "api/v4/",
		log:       logOpt,
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
	u := c.getUser(token)
	if u == nil {
		return false
	}
	if len(c.whitelist) > 0 && !c.whitelist[c.Username(token)] {
		c.logMsg("Auth fail (not in whitelist)")
		return false
	}
	if c.group == "" {
		// There's no group and token is valid.  Can only reach
		// here if user whitelist is set and user is in whitelist.
		c.logMsg("Auth success (no groups specified in server config)")
		metrics.M.AuthValid.WithLabelValues("gitlab").Inc()
		return true
	}
	if !c.checkGroupMembership(token, u.ID, c.group) {
		c.logMsg("Auth failure (not in allowed group)")
		return false
	}
	metrics.M.AuthValid.WithLabelValues("gitlab").Inc()
	c.logMsg("Auth success (in allowed group)")
	return true
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
	u := c.getUser(token)
	if u == nil {
		return ""
	}
	return u.Username
}
