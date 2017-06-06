package testprovider

import (
	"time"

	"github.com/nsheridan/cashier/server/auth"

	"golang.org/x/oauth2"
)

const (
	name = "testprovider"
)

// Config is an implementation of `auth.Provider` for testing.
type Config struct{}

var _ auth.Provider = (*Config)(nil)

// New creates a new provider.
func New() *Config {
	return &Config{}
}

// Name returns the name of the provider.
func (c *Config) Name() string {
	return name
}

// Valid validates the oauth token.
func (c *Config) Valid(token *oauth2.Token) bool {
	return true
}

// Revoke disables the access token.
func (c *Config) Revoke(token *oauth2.Token) error {
	return nil
}

// TODO: Implement me
func (c *Config) Principals(token *oauth2.Token) []string {
	return []string{}
}

// StartSession retrieves an authentication endpoint.
func (c *Config) StartSession(state string) *auth.Session {
	return &auth.Session{
		AuthURL: "https://www.example.com/auth",
	}
}

// Exchange authorizes the session and returns an access token.
func (c *Config) Exchange(code string) (*oauth2.Token, error) {
	return &oauth2.Token{
		AccessToken: "token",
		Expiry:      time.Now().Add(1 * time.Hour),
	}, nil
}

// Username retrieves the username portion of the user's email address.
func (c *Config) Username(token *oauth2.Token) string {
	return "test"
}
