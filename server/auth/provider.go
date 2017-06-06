package auth

import "golang.org/x/oauth2"

// Provider is an abstraction of different auth methods.
type Provider interface {
	Name() string
	StartSession(string) *Session
	Exchange(string) (*oauth2.Token, error)
	Username(*oauth2.Token) string
	Principals(*oauth2.Token) []string
	Valid(*oauth2.Token) bool
	Revoke(*oauth2.Token) error
}

// Session stores authentication state.
type Session struct {
	AuthURL string
	Token   *oauth2.Token
}

// Authorize obtains data from the provider and retains an access token that
// can be stored for later access.
func (s *Session) Authorize(provider Provider, code string) error {
	t, err := provider.Exchange(code)
	if err != nil {
		return err
	}
	s.Token = t
	return nil
}
