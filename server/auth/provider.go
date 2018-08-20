package auth

import "golang.org/x/oauth2"

// Provider is an abstraction of different auth methods.
type Provider interface {
	Name() string
	StartSession(string) string
	Exchange(string) (*oauth2.Token, error)
	Username(*oauth2.Token) string
	Valid(*oauth2.Token) bool
	Revoke(*oauth2.Token) error
}
