package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/cashier-go/cashier/server/config"
	"github.com/cashier-go/cashier/server/metrics"

	"golang.org/x/oauth2"
)

const (
	name = "gitlab"
)

// Config is an implementation of `auth.Provider` for authenticating using a
// Gitlab account.
type Config struct {
	config    *oauth2.Config
	groups    []string
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

func (c *Config) logMsg(message error) {
	if c.log {
		log.Print(message)
	}
}

// A new oauth2 http client.
func (c *Config) newClient(ctx context.Context, token *oauth2.Token) *http.Client {
	return c.config.Client(ctx, token)
}

func (c *Config) getURL(ctx context.Context, token *oauth2.Token, url string) (*bytes.Buffer, error) {
	client := c.newClient(ctx, token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var body bytes.Buffer
	io.Copy(&body, resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gitlab error(http: %d) getting %s: '%s'",
			resp.StatusCode, url, body.String())
	}
	return &body, nil
}

// Gets info on the current user.
func (c *Config) getUser(ctx context.Context, token *oauth2.Token) *serviceUser {
	url := c.apiurl + "user"
	body, err := c.getURL(ctx, token, url)
	if err != nil {
		c.logMsg(fmt.Errorf("failed to get user: %w", err))
		return nil
	}
	var user serviceUser
	if err := json.NewDecoder(body).Decode(&user); err != nil {
		c.logMsg(fmt.Errorf("failed to decode user (%s): %s", url, err))
		return nil
	}
	return &user
}

// Gets current user group membership info.
func (c *Config) checkGroupMembership(ctx context.Context, token *oauth2.Token, uid int, group string) bool {
	url := fmt.Sprintf("%sgroups/%s/members/%d", c.apiurl, group, uid)
	body, err := c.getURL(ctx, token, url)
	if err != nil {
		c.logMsg(fmt.Errorf("failed to fetch group memberships: %w", err))
		return false
	}
	var m serviceGroupMember
	if err := json.NewDecoder(body).Decode(&m); err != nil {
		c.logMsg(fmt.Errorf("failed to parse groups (%s): %s", url, err))
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
	if !allUsers && c.ProviderOpts["groups"] == "" && len(uw) == 0 {
		return nil, errors.New("gitlab_opts groups and the users whitelist must not be both empty if allusers isn't true")
	}
	siteURL := "https://gitlab.com/"
	if c.ProviderOpts["siteurl"] != "" {
		siteURL = c.ProviderOpts["siteurl"]
		if siteURL[len(siteURL)-1] != '/' {
			return nil, errors.New("gitlab_opts siteurl must end in /")
		}
	} else if allUsers {
		return nil, errors.New("gitlab_opts if allusers is set, siteurl must be set")
	}
	// TODO: Should make sure siteURL is just the host bit.

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
				"read_api",
			},
		},
		groups:    providedAuthGroups(c),
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
func (c *Config) Valid(ctx context.Context, token *oauth2.Token) bool {
	if !token.Valid() {
		log.Printf("Auth fail (oauth2 Valid failure)")
		return false
	}
	if c.allusers {
		log.Printf("Auth success (allusers)")
		metrics.M.AuthValid.WithLabelValues("gitlab").Inc()
		return true
	}
	u := c.getUser(ctx, token)
	if u == nil {
		c.logMsg(errors.New("auth fail (unable to fetch user information)"))
		return false
	}
	if len(c.whitelist) > 0 && !c.whitelist[c.Username(ctx, token)] {
		c.logMsg(errors.New("auth fail (not in whitelist)"))
		return false
	}
	if len(c.groups) == 0 {
		// There's no group and token is valid.  Can only reach
		// here if user whitelist is set and user is in whitelist.
		c.logMsg(errors.New("auth success (no groups specified in server config)"))
		metrics.M.AuthValid.WithLabelValues("gitlab").Inc()
		return true
	}

	for idx, group := range c.groups {
		// url.QueryEscape is necessary when we aren't using the group IDs and we are checking a subgroup:
		// https://gitlab.com/gitlab-org/gitlab-foss/-/issues/29296#:~:text=You%27ll%20need%20to%20encode%20the%20full%20path%20to%20the%20group
		isMember := c.checkGroupMembership(ctx, token, u.ID, url.QueryEscape(group))
		if !isMember && idx == len(c.groups)-1 {
			c.logMsg(fmt.Errorf("auth failure (user '%s' is not member of group '%s')", u.Username, group))
			return false
		}

		if isMember {
			c.logMsg(fmt.Errorf("auth Success (user '%s' is a member of group '%s')", u.Username, group))
			break
		}

		c.logMsg(fmt.Errorf("auth failure (user '%s' is not a member of group '%s')", u.Username, group))
	}

	metrics.M.AuthValid.WithLabelValues("gitlab").Inc()
	c.logMsg(errors.New("auth success (in allowed group)"))
	return true
}

// Revoke is a no-op revoke method. Gitlab doesn't allow token
// revocation - tokens live for an hour.
// Returns nil to satisfy the Provider interface.
func (c *Config) Revoke(ctx context.Context, token *oauth2.Token) error {
	return nil
}

// StartSession retrieves an authentication endpoint from Gitlab.
func (c *Config) StartSession(state string) string {
	return c.config.AuthCodeURL(state)
}

// Exchange authorizes the session and returns an access token.
func (c *Config) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	t, err := c.config.Exchange(ctx, code)
	if err == nil {
		metrics.M.AuthExchange.WithLabelValues("gitlab").Inc()
	}
	return t, err
}

// Username retrieves the username of the Gitlab user.
func (c *Config) Username(ctx context.Context, token *oauth2.Token) string {
	u := c.getUser(ctx, token)
	if u == nil {
		return ""
	}
	return u.Username
}

// providedAuthGroups returns a list of groups from `groups` config in Auth provider options
func providedAuthGroups(c *config.Auth) []string {
	sliced := make([]string, 0)

	groups, ok := c.ProviderOpts["groups"]
	if ok {
		sliced = strings.Split(groups, ",")
		for i := range sliced {
			sliced[i] = strings.TrimSpace(sliced[i])
		}
	}

	// check if deprecated `group` config is also specified, add that group to the list as well
	if group, ok := c.ProviderOpts["group"]; ok && !strings.Contains(groups, group) {
		sliced = append(sliced, group)
	}

	return sliced
}
