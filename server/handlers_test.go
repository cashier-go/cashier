package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/nsheridan/cashier/lib"
	"github.com/nsheridan/cashier/server/auth/testprovider"
	"github.com/nsheridan/cashier/server/config"
	"github.com/nsheridan/cashier/server/signer"
	"github.com/nsheridan/cashier/server/store"
	"github.com/nsheridan/cashier/testdata"
	"github.com/stripe/krl"
)

var a *application

func init() {
	f, _ := os.CreateTemp(os.TempDir(), "signing_key_")
	defer os.Remove(f.Name())
	f.Write(testdata.Priv)
	f.Close()
	keysigner, _ := signer.New(&config.SSH{
		SigningKey: f.Name(),
		MaxAge:     "4h",
	})
	certstore, _ := store.New(config.Database{Type: "mem"})
	a = &application{
		cookiestore:  sessions.NewCookieStore([]byte("secret")),
		authprovider: testprovider.New(),
		keysigner:    keysigner,
		certstore:    certstore,
		router:       mux.NewRouter(),
		config:       &config.Server{CSRFSecret: "0123456789abcdef"},
	}
	a.setupRoutes()
}

func TestLoginHandler(t *testing.T) {
	req, _ := http.NewRequest("GET", "/auth/login", nil)
	resp := httptest.NewRecorder()
	a.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusFound && resp.Header().Get("Location") != "https://www.example.com/auth" {
		t.Error("Unexpected response")
	}
}

func TestCallbackHandler(t *testing.T) {
	req, _ := http.NewRequest("GET", "/auth/callback", nil)
	req.Form = url.Values{"state": []string{"state"}, "code": []string{"abcdef"}}
	resp := httptest.NewRecorder()
	a.setSessionVariable(resp, req, "state", "state")
	req.Header.Add("Cookie", resp.HeaderMap["Set-Cookie"][0])
	a.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusFound && resp.Header().Get("Location") != "/" {
		t.Errorf("Response: %d\nHeaders: %v", resp.Code, resp.Header())
	}
}

func TestRootHandler(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	resp := httptest.NewRecorder()
	tok := &oauth2.Token{
		AccessToken: "XXX_TEST_TOKEN_STRING_XXX",
		Expiry:      time.Now().Add(1 * time.Hour),
	}
	a.setAuthToken(resp, req, tok)
	req.Header.Add("Cookie", resp.HeaderMap["Set-Cookie"][0])
	a.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK && !strings.Contains(resp.Body.String(), "XXX_TEST_TOKEN_STRING_XXX") {
		t.Error("Unable to find token in response")
	}
}

func TestRootHandlerNoSession(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	resp := httptest.NewRecorder()
	a.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusSeeOther {
		t.Errorf("Unexpected status: %s, wanted %s", http.StatusText(resp.Code), http.StatusText(http.StatusSeeOther))
	}
}

func TestSignRevoke(t *testing.T) {
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
	tok := &oauth2.Token{
		AccessToken: "authenticated",
		Expiry:      time.Now().Add(1 * time.Hour),
	}
	a.setAuthToken(resp, req, tok)
	a.router.ServeHTTP(resp, req)
	csrfToken := resp.Result().Header.Get("X-CSRF-Token")

	// 3. Revoke the cert
	req, _ = http.NewRequest("POST", "/admin/revoke", nil)
	for _, cookie := range resp.Result().Cookies() {
		req.AddCookie(cookie)
	}
	req.Header.Set("X-CSRF-Token", csrfToken)
	resp = httptest.NewRecorder()
	a.setAuthToken(resp, req, tok)
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
