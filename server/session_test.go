package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"golang.org/x/oauth2"

	"github.com/cashier-go/cashier/server/auth"
	"github.com/cashier-go/cashier/server/auth/testprovider"
	"github.com/cashier-go/cashier/server/config"
	"github.com/cashier-go/cashier/server/signer"
	"github.com/cashier-go/cashier/server/store"
	"github.com/cashier-go/cashier/testdata"
)

func newTestApp(t *testing.T, provider auth.Provider) *application {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "signing_key_")
	if err != nil {
		t.Fatal(err)
	}
	f.Write(testdata.Priv)
	f.Close()
	keysigner, err := signer.New(&config.SSH{SigningKey: f.Name(), MaxAge: "4h"})
	if err != nil {
		t.Fatal(err)
	}
	certstore, err := store.New(config.Database{Type: "mem"})
	if err != nil {
		t.Fatal(err)
	}
	app := newApplication(keysigner, provider, certstore, &config.Server{
		CookieSecret: "secret",
		CSRFSecret:   "0123456789abcdef",
	})
	return app
}

func testCallback(t *testing.T, app *application, originURL string) *httptest.ResponseRecorder {
	t.Helper()
	req, _ := http.NewRequest("GET", "/auth/callback", nil)
	req.Form = url.Values{"state": []string{"state"}, "code": []string{"code"}}
	setup := httptest.NewRecorder()
	if err := app.setSessionVariable(setup, req, "state", "state"); err != nil {
		t.Fatal(err)
	}
	if originURL != "" {
		if err := app.setSessionVariable(setup, req, "origin_url", originURL); err != nil {
			t.Fatal(err)
		}
	}
	for _, cookie := range setup.Result().Cookies() {
		req.AddCookie(cookie)
	}
	resp := httptest.NewRecorder()
	app.router.ServeHTTP(resp, req)
	return resp
}

func TestCallbackRedirectsToNonRootOrigin(t *testing.T) {
	app := newTestApp(t, testprovider.New())
	resp := testCallback(t, app, "/admin/certs")

	if resp.Code != http.StatusFound {
		t.Fatalf("got status %d, want %d", resp.Code, http.StatusFound)
	}
	if got := resp.Header().Get("Location"); got != "/admin/certs" {
		t.Errorf("got redirect to %q, want %q", got, "/admin/certs")
	}
}

func TestAuthedRequiresUsername(t *testing.T) {
	app := newTestApp(t, testprovider.New())

	req, _ := http.NewRequest("GET", "/admin/certs", nil)
	resp := httptest.NewRecorder()
	app.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusSeeOther {
		t.Errorf("unauthenticated request got status %d, want %d", resp.Code, http.StatusSeeOther)
	}

	req, _ = http.NewRequest("GET", "/admin/certs", nil)
	setup := httptest.NewRecorder()
	if err := app.setSessionVariable(setup, req, "username", "test"); err != nil {
		t.Fatal(err)
	}
	for _, c := range setup.Result().Cookies() {
		req.AddCookie(c)
	}
	resp = httptest.NewRecorder()
	app.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Errorf("authenticated request got status %d, want %d", resp.Code, http.StatusOK)
	}
}

type stateRecorder struct {
	*testprovider.Config
	state string
}

func (p *stateRecorder) StartSession(state string) string {
	p.state = state
	return p.Config.StartSession(state)
}

func TestClientLoginFlow(t *testing.T) {
	provider := stateRecorder{Config: testprovider.New()}
	app := newTestApp(t, &provider)
	var jar []*http.Cookie

	do := func(target string) *httptest.ResponseRecorder {
		req, _ := http.NewRequest("GET", target, nil)
		for _, c := range jar {
			req.AddCookie(c)
		}
		resp := httptest.NewRecorder()
		app.router.ServeHTTP(resp, req)
		jar = resp.Result().Cookies()
		return resp
	}

	resp := do("/?localserver=45678")
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("/ returned %d, want %d", resp.Code, http.StatusSeeOther)
	}

	resp = do(resp.Header().Get("Location"))
	if resp.Code != http.StatusFound {
		t.Fatalf("/auth/login returned %d, want %d", resp.Code, http.StatusFound)
	}

	state := provider.state
	if state == "" {
		t.Fatal("/auth/login did not record a state value")
	}

	resp = do("/auth/callback?state=" + state + "&code=code")
	if resp.Code != http.StatusOK {
		t.Fatalf("/auth/callback returned %d, want %d", resp.Code, http.StatusOK)
	}
	if !strings.Contains(resp.Body.String(), "http://localhost:45678") {
		t.Error("token page is not addressed to the client's local server")
	}
}

func TestTokenPageIgnoresRemoteLocalserver(t *testing.T) {
	app := newTestApp(t, testprovider.New())
	w := httptest.NewRecorder()
	app.renderToken(w, &oauth2.Token{AccessToken: "SECRET"}, "1@evil.example.com/")

	body := w.Body.String()
	if strings.Contains(body, "evil.example.com") {
		t.Error("token page points the browser at an attacker-supplied host")
	}
	if strings.Contains(body, "xhr.open") {
		t.Error("token page still submits the token automatically")
	}
}

func TestTokenPagePostsToClientLocalserver(t *testing.T) {
	app := newTestApp(t, testprovider.New())
	w := httptest.NewRecorder()
	app.renderToken(w, &oauth2.Token{AccessToken: "SECRET"}, "45678/3f2a1b4c-dead-beef")

	if !strings.Contains(w.Body.String(), "http://localhost:45678/3f2a1b4c-dead-beef") {
		t.Error("token page does not submit to the client's local server")
	}
}

func TestCallbackStateIsClearedAfterUse(t *testing.T) {
	app := newTestApp(t, testprovider.New())

	first := testCallback(t, app, "/")
	if first.Code != http.StatusOK {
		t.Fatalf("first callback returned %d, want %d", first.Code, http.StatusOK)
	}

	// Replay the same state with the session the callback handed back.
	req, _ := http.NewRequest("GET", "/auth/callback", nil)
	req.Form = url.Values{"state": []string{"state"}, "code": []string{"code"}}
	for _, c := range first.Result().Cookies() {
		req.AddCookie(c)
	}
	replay := httptest.NewRecorder()
	app.router.ServeHTTP(replay, req)
	if replay.Code != http.StatusUnauthorized {
		t.Errorf("replayed callback returned %d, want %d", replay.Code, http.StatusUnauthorized)
	}
}
