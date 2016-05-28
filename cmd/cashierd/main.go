package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"golang.org/x/oauth2"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/nsheridan/cashier/lib"
	"github.com/nsheridan/cashier/server/auth"
	"github.com/nsheridan/cashier/server/auth/github"
	"github.com/nsheridan/cashier/server/auth/google"
	"github.com/nsheridan/cashier/server/config"
	"github.com/nsheridan/cashier/server/signer"
	"github.com/nsheridan/cashier/templates"
)

var (
	cfg = flag.String("config_file", "config.json", "Path to configuration file.")
)

// appContext contains local context - cookiestore, authprovider, authsession, templates etc.
type appContext struct {
	cookiestore  *sessions.CookieStore
	authprovider auth.Provider
	authsession  *auth.Session
	sshKeySigner *signer.KeySigner
}

// getAuthCookie retrieves a cookie from the request and validates it.
func (a *appContext) getAuthCookie(r *http.Request) *oauth2.Token {
	session, _ := a.cookiestore.Get(r, "tok")
	t, ok := session.Values["token"]
	if !ok {
		return nil
	}
	var tok oauth2.Token
	if err := json.Unmarshal(t.([]byte), &tok); err != nil {
		return nil
	}
	if !tok.Valid() {
		return nil
	}
	return &tok
}

// setAuthCookie marshals the auth token and stores it as a cookie.
func (a *appContext) setAuthCookie(w http.ResponseWriter, r *http.Request, t *oauth2.Token) {
	session, _ := a.cookiestore.Get(r, "tok")
	val, _ := json.Marshal(t)
	session.Values["token"] = val
	session.Save(r, w)
}

// parseKey retrieves and unmarshals the signing request.
func parseKey(r *http.Request) (*lib.SignRequest, error) {
	var s lib.SignRequest
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(body, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// signHandler handles the "/sign" path.
// It unmarshals the client token to an oauth token, validates it and signs the provided public ssh key.
func signHandler(a *appContext, w http.ResponseWriter, r *http.Request) (int, error) {
	var t string
	if ah := r.Header.Get("Authorization"); ah != "" {
		if len(ah) > 6 && strings.ToUpper(ah[0:7]) == "BEARER " {
			t = ah[7:]
		}
	}
	if t == "" {
		return http.StatusUnauthorized, errors.New(http.StatusText(http.StatusUnauthorized))
	}
	token := &oauth2.Token{
		AccessToken: t,
	}
	ok := a.authprovider.Valid(token)
	if !ok {
		return http.StatusUnauthorized, errors.New(http.StatusText(http.StatusUnauthorized))
	}

	// Sign the pubkey and issue the cert.
	req, err := parseKey(r)
	req.Principal = a.authprovider.Username(token)
	a.authprovider.Revoke(token) // We don't need this anymore.
	if err != nil {
		return http.StatusInternalServerError, err
	}
	signed, err := a.sshKeySigner.SignUserKey(req)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	json.NewEncoder(w).Encode(&lib.SignResponse{
		Status:   "ok",
		Response: signed,
	})
	return http.StatusOK, nil
}

// loginHandler starts the authentication process with the provider.
func loginHandler(a *appContext, w http.ResponseWriter, r *http.Request) (int, error) {
	a.authsession = a.authprovider.StartSession(newState())
	http.Redirect(w, r, a.authsession.AuthURL, http.StatusFound)
	return http.StatusFound, nil
}

// callbackHandler handles retrieving the access token from the auth provider and saves it for later use.
func callbackHandler(a *appContext, w http.ResponseWriter, r *http.Request) (int, error) {
	if r.FormValue("state") != a.authsession.State {
		return http.StatusUnauthorized, errors.New(http.StatusText(http.StatusUnauthorized))
	}
	code := r.FormValue("code")
	if err := a.authsession.Authorize(a.authprovider, code); err != nil {
		return http.StatusInternalServerError, err
	}
	a.setAuthCookie(w, r, a.authsession.Token)
	http.Redirect(w, r, "/", http.StatusFound)
	return http.StatusFound, nil
}

// rootHandler starts the auth process. If the client is authenticated it renders the token to the user.
func rootHandler(a *appContext, w http.ResponseWriter, r *http.Request) (int, error) {
	tok := a.getAuthCookie(r)
	if !tok.Valid() || !a.authprovider.Valid(tok) {
		http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
		return http.StatusSeeOther, nil
	}
	page := struct {
		Token string
	}{tok.AccessToken}

	tmpl := template.Must(template.New("token.html").Parse(templates.Token))
	tmpl.Execute(w, page)
	return http.StatusOK, nil
}

// appHandler is a handler which uses appContext to manage state.
type appHandler struct {
	*appContext
	h func(*appContext, http.ResponseWriter, *http.Request) (int, error)
}

// ServeHTTP handles the request and writes responses.
func (ah appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	status, err := ah.h(ah.appContext, w, r)
	if err != nil {
		log.Printf("HTTP %d: %q", status, err)
		switch status {
		case http.StatusNotFound:
			http.NotFound(w, r)
		case http.StatusInternalServerError:
			http.Error(w, http.StatusText(status), status)
		default:
			http.Error(w, http.StatusText(status), status)
		}
	}
}

// newState generates a state identifier for the oauth process.
func newState() string {
	k := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		return "unexpectedstring"
	}
	return hex.EncodeToString(k)
}

func readConfig(filename string) (*config.Config, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return config.ReadConfig(f)
}

func main() {
	flag.Parse()
	config, err := readConfig(*cfg)
	if err != nil {
		log.Fatal(err)
	}
	signer, err := signer.New(config.SSH)
	if err != nil {
		log.Fatal(err)
	}

	var authprovider auth.Provider
	switch config.Auth.Provider {
	case "google":
		authprovider, err = google.New(&config.Auth)
	case "github":
		authprovider, err = github.New(&config.Auth)
	default:
		log.Fatalln("Unknown provider %s", config.Auth.Provider)
	}

	if err != nil {
		log.Fatal(err)
	}

	ctx := &appContext{
		cookiestore:  sessions.NewCookieStore([]byte(config.Server.CookieSecret)),
		authprovider: authprovider,
		sshKeySigner: signer,
	}
	ctx.cookiestore.Options = &sessions.Options{
		MaxAge:   900,
		Path:     "/",
		Secure:   config.Server.UseTLS,
		HttpOnly: true,
	}

	m := mux.NewRouter()
	m.Handle("/", appHandler{ctx, rootHandler})
	m.Handle("/auth/login", appHandler{ctx, loginHandler})
	m.Handle("/auth/callback", appHandler{ctx, callbackHandler})
	m.Handle("/sign", appHandler{ctx, signHandler})

	fmt.Println("Starting server...")
	l := fmt.Sprintf(":%d", config.Server.Port)
	if config.Server.UseTLS {
		log.Fatal(http.ListenAndServeTLS(l, config.Server.TLSCert, config.Server.TLSKey, m))
	}
	log.Fatal(http.ListenAndServe(l, m))
}
