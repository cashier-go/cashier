package microsoft

import (
	"encoding/json"
	"errors"
	"net/http"
	"path"
	"strings"

	"github.com/nsheridan/cashier/server/auth"
	"github.com/nsheridan/cashier/server/config"
	"github.com/nsheridan/cashier/server/metrics"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

const (
	name = "microsoft"
)

// Config is an implementation of `auth.Provider` for authenticating using a
// Office 365 account.
type Config struct {
	config    *oauth2.Config
	tenant    string
	groups    map[string]bool
	whitelist map[string]bool
}

var _ auth.Provider = (*Config)(nil)

// New creates a new Microsoft provider from a configuration.
func New(c *config.Auth) (*Config, error) {
	whitelist := make(map[string]bool)
	for _, u := range c.UsersWhitelist {
		whitelist[u] = true
	}
	if c.ProviderOpts["tenant"] == "" && len(whitelist) == 0 {
		return nil, errors.New("either Office 365 tenant or users whitelist must be specified")
	}
	groupMap := make(map[string]bool)
	if groups, ok := c.ProviderOpts["groups"]; ok {
		for _, group := range strings.Split(groups, ",") {
			groupMap[strings.Trim(group, " ")] = true
		}
	}

	return &Config{
		config: &oauth2.Config{
			ClientID:     c.OauthClientID,
			ClientSecret: c.OauthClientSecret,
			RedirectURL:  c.OauthCallbackURL,
			Endpoint:     microsoft.AzureADEndpoint(c.ProviderOpts["tenant"]),
			Scopes:       []string{"user.Read.All", "Directory.Read.All"},
		},
		tenant:    c.ProviderOpts["tenant"],
		whitelist: whitelist,
		groups:    groupMap,
	}, nil
}

// A new oauth2 http client.
func (c *Config) newClient(token *oauth2.Token) *http.Client {
	return c.config.Client(oauth2.NoContext, token)
}

// Gets a response for an graph api call.
func (c *Config) getDocument(token *oauth2.Token, pathElements ...string) map[string]interface{} {
	client := c.newClient(token)
	url := "https://" + path.Join("graph.microsoft.com/v1.0", path.Join(pathElements...))
	resp, err := client.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var document map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&document); err != nil {
		return nil
	}
	return document
}

// Get info from the "/me" endpoint of the Microsoft Graph API (MSG-API).
// https://developer.microsoft.com/en-us/graph/docs/concepts/v1-overview
func (c *Config) getMe(token *oauth2.Token, item string) string {
	document := c.getDocument(token, "/me")
	if value, ok := document[item].(string); ok {
		return value
	}
	return ""
}

// Check against verified domains from "/organization" endpoint of MSG-API.
func (c *Config) verifyTenant(token *oauth2.Token) bool {
	document := c.getDocument(token, "/organization")
	// The domains for an organisation are in an array of structs under
	// verifiedDomains, which is in a struct which is in turn an array
	// of such structs under value in the document.  Which in json looks
	// like this:
	// { "@odata.context": "https://graph.microsoft.com/v1.0/$metadata#organization",
	//   "value": [ {
	//      ...
	//      "verifiedDomains": [ {
	//                    ...
	//                    "name": "M365x214355.onmicrosoft.com",
	//            } ]
	//   } ]
	//}
	var value []interface{}
	var ok bool
	if value, ok = document["value"].([]interface{}); !ok {
		return false
	}
	for _, valueEntry := range value {
		if value, ok = valueEntry.(map[string]interface{})["verifiedDomains"].([]interface{}); !ok {
			continue
		}
		for _, val := range value {
			domain := val.(map[string]interface{})["name"].(string)
			if domain == c.tenant {
				return true
			}
		}
	}
	return false
}

// Check against groups from /users/{id}/memberOf endpoint of MSG-API.
func (c *Config) verifyGroups(token *oauth2.Token) bool {
	document := c.getDocument(token, "/users/me/memberOf")
	var value []interface{}
	var ok bool
	if value, ok = document["value"].([]interface{}); !ok {
		return false
	}
	for _, valueEntry := range value {
		if group, ok := valueEntry.(map[string]interface{})["displayName"].(string); ok {
			if c.groups[group] {
				return true
			}
		}
	}
	return false
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
	metrics.M.AuthValid.WithLabelValues("microsoft").Inc()
	if c.tenant != "" {
		if c.verifyTenant(token) {
			if len(c.groups) > 0 {
				return c.verifyGroups(token)
			}
			return true
		}
	}
	return false
}

// Revoke disables the access token.
func (c *Config) Revoke(token *oauth2.Token) error {
	return nil
}

// StartSession retrieves an authentication endpoint from Microsoft.
func (c *Config) StartSession(state string) string {
	return c.config.AuthCodeURL(state,
		oauth2.SetAuthURLParam("hd", c.tenant),
		oauth2.SetAuthURLParam("prompt", "login"))
}

// Exchange authorizes the session and returns an access token.
func (c *Config) Exchange(code string) (*oauth2.Token, error) {
	t, err := c.config.Exchange(oauth2.NoContext, code)
	if err == nil {
		metrics.M.AuthExchange.WithLabelValues("microsoft").Inc()
	}
	return t, err
}

// Email retrieves the email address of the user.
func (c *Config) Email(token *oauth2.Token) string {
	return c.getMe(token, "mail")
}

// Username retrieves the username portion of the user's email address.
func (c *Config) Username(token *oauth2.Token) string {
	return strings.Split(c.Email(token), "@")[0]
}
