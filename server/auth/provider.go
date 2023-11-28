package auth

import (
	"context"

	"golang.org/x/oauth2"
)

// Provider is an abstraction of different auth methods.
type Provider interface {
	Name() string
	StartSession(string) string
	Exchange(context.Context, string) (*oauth2.Token, error)
	Username(context.Context, *oauth2.Token) string
	Valid(context.Context, *oauth2.Token) bool
	Revoke(context.Context, *oauth2.Token) error
}
