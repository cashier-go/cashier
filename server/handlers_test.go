package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"

	"github.com/stripe/krl"

	"github.com/cashier-go/cashier/lib"
	"github.com/cashier-go/cashier/server/auth/testprovider"
	"github.com/cashier-go/cashier/testdata"
)

func TestLoginHandler(t *testing.T) {
	a := newTestApp(t, testprovider.New())
	req, _ := http.NewRequest("GET", "/auth/login", nil)
	resp := httptest.NewRecorder()
	a.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusFound {
		t.Errorf("got status %d, want %d", resp.Code, http.StatusFound)
	}
	if got := resp.Header().Get("Location"); got != "https://www.example.com/auth" {
		t.Errorf("got redirect to %q, want the provider's auth endpoint", got)
	}
}

func TestCallbackHandler(t *testing.T) {
	a := newTestApp(t, testprovider.New())
	tests := []struct {
		name     string
		state    string
		expected int
	}{
		{
			name:     "valid",
			state:    "state",
			expected: http.StatusOK,
		},
		{
			name:     "missing state",
			expected: http.StatusUnauthorized,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/auth/callback", nil)
			req.Form = url.Values{"state": []string{test.state}, "code": []string{"code"}}
			resp := httptest.NewRecorder()
			a.setSessionVariable(resp, req, "state", test.state)
			for _, cookie := range resp.Result().Cookies() {
				req.AddCookie(cookie)
			}
			a.router.ServeHTTP(resp, req)
			if resp.Code != test.expected {
				t.Errorf("got response %d, expected %d", resp.Code, test.expected)
			}
		})
	}
}

func TestSignRevoke(t *testing.T) {
	a := newTestApp(t, testprovider.New())
	var resp *httptest.ResponseRecorder
	var req *http.Request

	// 1. Get a signed cert from the server
	s, _ := json.Marshal(&lib.SignRequest{
		Key:        string(testdata.Pub),
		ValidUntil: time.Now().UTC().Add(4 * time.Hour),
	})
	req, _ = http.NewRequest("POST", "/sign", bytes.NewReader(s))
	resp = httptest.NewRecorder()
	req.Header.Set("Authorization", "Bearer abcdef")
	a.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatal("Unexpected response")
	}
	r := &lib.SignResponse{}
	if err := json.NewDecoder(resp.Body).Decode(r); err != nil {
		t.Fatal(err)
	}
	if r.Status != "ok" {
		t.Fatal("Unexpected response")
	}
	k, _, _, _, err := ssh.ParseAuthorizedKey([]byte(r.Response))
	if err != nil {
		t.Fatal(err)
	}
	cert, ok := k.(*ssh.Certificate)
	if !ok {
		t.Fatal("Did not receive a certificate")
	}

	// 2. Request the issued certs page, to obtain the necessary CSRF token
	req, _ = http.NewRequest("GET", "/admin/certs", nil)
	resp = httptest.NewRecorder()
	a.setSessionVariable(resp, req, "username", "test")
	a.router.ServeHTTP(resp, req)
	csrfToken := resp.Result().Header.Get("X-CSRF-Token")

	// 3. Revoke the cert
	req, _ = http.NewRequest("POST", "/admin/revoke", nil)
	for _, cookie := range resp.Result().Cookies() {
		req.AddCookie(cookie)
	}
	req.Header.Set("X-CSRF-Token", csrfToken)
	resp = httptest.NewRecorder()
	a.setSessionVariable(resp, req, "username", "test")
	req.PostForm = url.Values{
		"cert_id": []string{cert.KeyId},
	}
	a.router.ServeHTTP(resp, req)

	// 4. Retrieve the KRL and verify that the cert is revoked
	req, _ = http.NewRequest("GET", "/revoked", nil)
	resp = httptest.NewRecorder()
	a.router.ServeHTTP(resp, req)
	revoked, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	rl, err := krl.ParseKRL(revoked)
	if err != nil {
		t.Fatal(err)
	}
	if !rl.IsRevoked(cert) {
		t.Fatalf("cert %s was not revoked", cert.KeyId)
	}
}

func TestTokenFromRequest(t *testing.T) {
	tests := []struct {
		name string
		req  *http.Request
		want *oauth2.Token
	}{
		{
			name: "authorization present",
			req:  &http.Request{Header: http.Header{"Authorization": []string{"BEARER abcdef"}}},
			want: &oauth2.Token{AccessToken: "abcdef"},
		},
		{
			name: "authorization present, case-insensitive",
			req:  &http.Request{Header: http.Header{"Authorization": []string{"bearer abcdef"}}},
			want: &oauth2.Token{AccessToken: "abcdef"},
		},
		{
			name: "authorization missing",
			req:  &http.Request{},
			want: &oauth2.Token{},
		},
		{
			name: "authorization invalid",
			req:  &http.Request{Header: http.Header{"Authorization": []string{"BERARE abcdef"}}},
			want: &oauth2.Token{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			token := tokenFromRequest(test.req)
			if token.AccessToken != test.want.AccessToken {
				t.Fatalf("Expected token %q, got token %q", test.want.AccessToken, token.AccessToken)
			}
		})
	}
}
