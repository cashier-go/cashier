package auth

import (
	"crypto/tls"
	"net/http"
	"testing"
)

func TestHTTP(t *testing.T) {
	want := "http://example.com/auth/callback"
	r := &http.Request{
		Host: "example.com",
	}
	ret := Oauth2RedirectURL(r)
	if want != ret {
		t.Errorf("Wanted %s, got %s", want, ret)
	}
}

func TestHTTPS(t *testing.T) {
	want := "https://example.com/auth/callback"
	r := &http.Request{
		Host: "example.com",
		TLS:  &tls.ConnectionState{},
	}
	ret := Oauth2RedirectURL(r)
	if want != ret {
		t.Errorf("Wanted %s, got %s", want, ret)
	}
}
