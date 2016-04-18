package auth

import "golang.org/x/oauth2"

type Provider interface {
	Name() string
	StartSession(string) *Session
	Exchange(string) (*oauth2.Token, error)
	Username(*oauth2.Token) string
	Valid(*oauth2.Token) bool
	Revoke(*oauth2.Token) error
}

type Session struct {
	AuthURL string
	Token   *oauth2.Token
	State   string
}

func (s *Session) Authorize(provider Provider, code string) error {
	t, err := provider.Exchange(code)
	if err != nil {
		return err
	}
	s.Token = t
	return nil
}
