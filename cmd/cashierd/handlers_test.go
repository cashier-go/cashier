package main

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

	"github.com/gorilla/sessions"
	"github.com/nsheridan/cashier/lib"
	"github.com/nsheridan/cashier/server/auth"
	"github.com/nsheridan/cashier/server/auth/testprovider"
	"github.com/nsheridan/cashier/server/config"
	"github.com/nsheridan/cashier/server/signer"
	"github.com/nsheridan/cashier/server/store"
	"github.com/nsheridan/cashier/testdata"
)

func newContext(t *testing.T) *appContext {
	f, err := ioutil.TempFile(os.TempDir(), "signing_key_")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(f.Name())
	f.Write(testdata.Priv)
	f.Close()
	signer, err := signer.New(&config.SSH{
		SigningKey: f.Name(),
		MaxAge:     "1h",
	})
	if err != nil {
		t.Error(err)
	}
	return &appContext{
		cookiestore:  sessions.NewCookieStore([]byte("secret")),
		authprovider: testprovider.New(),
		certstore:    store.NewMemoryStore(),
		authsession:  &auth.Session{AuthURL: "https://www.example.com/auth"},
		sshKeySigner: signer,
	}
}

func TestLoginHandler(t *testing.T) {
	req, _ := http.NewRequest("GET", "/auth/login", nil)
	resp := httptest.NewRecorder()
	loginHandler(newContext(t), resp, req)
	if resp.Code != http.StatusFound && resp.Header().Get("Location") != "https://www.example.com/auth" {
		t.Error("Unexpected response")
	}
}

func TestCallbackHandler(t *testing.T) {
	req, _ := http.NewRequest("GET", "/auth/callback", nil)
	req.Form = url.Values{"state": []string{"state"}, "code": []string{"abcdef"}}
	resp := httptest.NewRecorder()
	ctx := newContext(t)
	ctx.setAuthStateCookie(resp, req, "state")
	callbackHandler(ctx, resp, req)
	if resp.Code != http.StatusFound && resp.Header().Get("Location") != "/" {
		t.Error("Unexpected response")
	}
}

func TestRootHandler(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	resp := httptest.NewRecorder()
	ctx := newContext(t)
	tok := &oauth2.Token{
		AccessToken: "XXX_TEST_TOKEN_STRING_XXX",
		Expiry:      time.Now().Add(1 * time.Hour),
	}
	ctx.setAuthTokenCookie(resp, req, tok)
	rootHandler(ctx, resp, req)
	if resp.Code != http.StatusOK && !strings.Contains(resp.Body.String(), "XXX_TEST_TOKEN_STRING_XXX") {
		t.Error("Unable to find token in response")
	}
}

func TestRootHandlerNoSession(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	resp := httptest.NewRecorder()
	ctx := newContext(t)
	rootHandler(ctx, resp, req)
	if resp.Code != http.StatusSeeOther {
		t.Errorf("Unexpected status: %s, wanted %s", http.StatusText(resp.Code), http.StatusText(http.StatusSeeOther))
	}
}

func TestSignRevoke(t *testing.T) {
	t.Skip()
	s, _ := json.Marshal(&lib.SignRequest{
		Key: string(testdata.Pub),
	})
	req, _ := http.NewRequest("POST", "/sign", bytes.NewReader(s))
	resp := httptest.NewRecorder()
	ctx := newContext(t)
	req.Header.Set("Authorization", "Bearer abcdef")
	signHandler(ctx, resp, req)
	if resp.Code != http.StatusOK {
		t.Error("Unexpected response")
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}
	r := &lib.SignResponse{}
	if err := json.Unmarshal(b, r); err != nil {
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
	req, _ = http.NewRequest("POST", "/revoke", nil)
	req.Form = url.Values{"cert_id": []string{cert.KeyId}}
	revokeCertHandler(ctx, resp, req)
	req, _ = http.NewRequest("GET", "/revoked", nil)
	listRevokedCertsHandler(ctx, resp, req)
	revoked, _ := ioutil.ReadAll(resp.Body)
	if string(revoked[:len(revoked)-1]) != r.Response {
		t.Error("omg")
	}
}
