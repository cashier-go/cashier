package server

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
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

var a *app

func init() {
	f, _ := ioutil.TempFile(os.TempDir(), "signing_key_")
	defer os.Remove(f.Name())
	f.Write(testdata.Priv)
	f.Close()
	keysigner, _ := signer.New(&config.SSH{
		SigningKey: f.Name(),
		MaxAge:     "4h",
	})
	certstore, _ := store.New(map[string]string{"type": "mem"})
	a = &app{
		cookiestore:  sessions.NewCookieStore([]byte("secret")),
		authprovider: testprovider.New(),
		keysigner:    keysigner,
		certstore:    certstore,
		router:       mux.NewRouter(),
		config:       &config.Server{CSRFSecret: "0123456789abcdef"},
	}
	a.routes()
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
	s, _ := json.Marshal(&lib.SignRequest{
		Key:        string(testdata.Pub),
		ValidUntil: time.Now().UTC().Add(4 * time.Hour),
	})
	req, _ := http.NewRequest("POST", "/sign", bytes.NewReader(s))
	resp := httptest.NewRecorder()
	req.Header.Set("Authorization", "Bearer abcdef")
	a.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Error("Unexpected response")
	}
	r := &lib.SignResponse{}
	if err := json.NewDecoder(resp.Body).Decode(r); err != nil {
		t.Error(err)
	}
	if r.Status != "ok" {
		t.Error("Unexpected response")
	}
	k, _, _, _, err := ssh.ParseAuthorizedKey([]byte(r.Response))
	if err != nil {
		t.Error(err)
	}
	cert, ok := k.(*ssh.Certificate)
	if !ok {
		t.Error("Did not receive a certificate")
	}
	// Revoke the cert and verify
	req, _ = http.NewRequest("POST", "/admin/revoke", nil)
	req.Form = url.Values{"cert_id": []string{cert.KeyId}}
	tok := &oauth2.Token{
		AccessToken: "authenticated",
		Expiry:      time.Now().Add(1 * time.Hour),
	}
	a.certstore.Revoke([]string{cert.KeyId})
	a.setAuthToken(resp, req, tok)
	a.router.ServeHTTP(resp, req)
	req, _ = http.NewRequest("GET", "/revoked", nil)
	a.router.ServeHTTP(resp, req)
	revoked, _ := ioutil.ReadAll(resp.Body)
	rl, err := krl.ParseKRL(revoked)
	if err != nil {
		t.Fail()
	}
	if !rl.IsRevoked(cert) {
		t.Errorf("cert %s was not revoked", cert.KeyId)
	}
}
