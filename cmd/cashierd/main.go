package main

import (
	"bytes"
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

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/nsheridan/cashier/lib"
	"github.com/nsheridan/cashier/server/auth"
	"github.com/nsheridan/cashier/server/auth/github"
	"github.com/nsheridan/cashier/server/auth/google"
	"github.com/nsheridan/cashier/server/certutil"
	"github.com/nsheridan/cashier/server/config"
	"github.com/nsheridan/cashier/server/fs"
	"github.com/nsheridan/cashier/server/signer"
	"github.com/nsheridan/cashier/server/store"
	"github.com/nsheridan/cashier/templates"
)

var (
	cfg = flag.String("config_file", "cashierd.conf", "Path to configuration file.")
)

// appContext contains local context - cookiestore, authprovider, authsession etc.
type appContext struct {
	cookiestore  *sessions.CookieStore
	authprovider auth.Provider
	authsession  *auth.Session
	sshKeySigner *signer.KeySigner
	certstore    store.CertStorer
}

// getAuthTokenCookie retrieves a cookie from the request.
func (a *appContext) getAuthTokenCookie(r *http.Request) *oauth2.Token {
	session, _ := a.cookiestore.Get(r, "session")
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

// setAuthTokenCookie marshals the auth token and stores it as a cookie.
func (a *appContext) setAuthTokenCookie(w http.ResponseWriter, r *http.Request, t *oauth2.Token) {
	session, _ := a.cookiestore.Get(r, "session")
	val, _ := json.Marshal(t)
	session.Values["token"] = val
	session.Save(r, w)
}

// getAuthStateCookie retrieves the oauth csrf state value from the client request.
func (a *appContext) getAuthStateCookie(r *http.Request) string {
	session, _ := a.cookiestore.Get(r, "session")
	state, ok := session.Values["state"]
	if !ok {
		return ""
	}
	return state.(string)
}

// setAuthStateCookie saves the oauth csrf state value.
func (a *appContext) setAuthStateCookie(w http.ResponseWriter, r *http.Request, state string) {
	session, _ := a.cookiestore.Get(r, "session")
	session.Values["state"] = state
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
	cert, err := a.sshKeySigner.SignUserKey(req)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if err := a.certstore.SetCert(cert); err != nil {
		log.Printf("Error recording cert: %v", err)
	}
	json.NewEncoder(w).Encode(&lib.SignResponse{
		Status:   "ok",
		Response: certutil.GetPublicKey(cert),
	})
	return http.StatusOK, nil
}

// loginHandler starts the authentication process with the provider.
func loginHandler(a *appContext, w http.ResponseWriter, r *http.Request) (int, error) {
	state := newState()
	a.setAuthStateCookie(w, r, state)
	a.authsession = a.authprovider.StartSession(state)
	http.Redirect(w, r, a.authsession.AuthURL, http.StatusFound)
	return http.StatusFound, nil
}

// callbackHandler handles retrieving the access token from the auth provider and saves it for later use.
func callbackHandler(a *appContext, w http.ResponseWriter, r *http.Request) (int, error) {
	if r.FormValue("state") != a.getAuthStateCookie(r) {
		return http.StatusUnauthorized, errors.New(http.StatusText(http.StatusUnauthorized))
	}
	code := r.FormValue("code")
	if err := a.authsession.Authorize(a.authprovider, code); err != nil {
		return http.StatusInternalServerError, err
	}
	a.setAuthTokenCookie(w, r, a.authsession.Token)
	http.Redirect(w, r, "/", http.StatusFound)
	return http.StatusFound, nil
}

// rootHandler starts the auth process. If the client is authenticated it renders the token to the user.
func rootHandler(a *appContext, w http.ResponseWriter, r *http.Request) (int, error) {
	tok := a.getAuthTokenCookie(r)
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

func revokedCertsHandler(a *appContext, w http.ResponseWriter, r *http.Request) (int, error) {
	var out bytes.Buffer
	revoked, err := a.certstore.List()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	for _, c := range revoked {
		if c.Revoked {
			out.WriteString(c.Raw)
			out.WriteString("\n")
		}
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(out.Bytes())
	return http.StatusOK, nil
}

func revokeCertHandler(a *appContext, w http.ResponseWriter, r *http.Request) (int, error) {
	if r.Method == "GET" {
		return http.StatusMethodNotAllowed, errors.New(http.StatusText(http.StatusMethodNotAllowed))
	}
	r.ParseForm()
	id := r.FormValue("cert_id")
	if id == "" {
		return http.StatusBadRequest, errors.New(http.StatusText(http.StatusBadRequest))
	}
	if err := a.certstore.Revoke(r.FormValue("cert_id")); err != nil {
		return http.StatusInternalServerError, err
	}
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
		http.Error(w, err.Error(), status)
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

func certStore(config string) (store.CertStorer, error) {
	var cstore store.CertStorer
	var err error
	engine := strings.Split(config, ":")[0]
	switch engine {
	case "mysql":
		cstore, err = store.NewMySQLStore(config)
	case "mem":
		cstore = store.NewMemoryStore()
	default:
		cstore = store.NewMemoryStore()
	}
	return cstore, err
}

func main() {
	flag.Parse()
	config, err := readConfig(*cfg)
	if err != nil {
		log.Fatal(err)
	}
	fs.Register(&config.AWS)
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

	certstore, err := certStore(config.Server.Datastore)
	if err != nil {
		log.Fatal(err)
	}
	ctx := &appContext{
		cookiestore:  sessions.NewCookieStore([]byte(config.Server.CookieSecret)),
		authprovider: authprovider,
		sshKeySigner: signer,
		certstore:    certstore,
	}
	ctx.cookiestore.Options = &sessions.Options{
		MaxAge:   900,
		Path:     "/",
		Secure:   config.Server.UseTLS,
		HttpOnly: true,
	}

	r := mux.NewRouter()
	r.Handle("/", appHandler{ctx, rootHandler})
	r.Handle("/auth/login", appHandler{ctx, loginHandler})
	r.Handle("/auth/callback", appHandler{ctx, callbackHandler})
	r.Handle("/sign", appHandler{ctx, signHandler})
	r.Handle("/revoked", appHandler{ctx, revokedCertsHandler})
	r.Handle("/revoke", appHandler{ctx, revokeCertHandler})
	logfile := os.Stderr
	if config.Server.HTTPLogFile != "" {
		logfile, err = os.OpenFile(config.Server.HTTPLogFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0660)
		if err != nil {
			log.Fatal(err)
		}
	}
	h := handlers.LoggingHandler(logfile, r)

	fmt.Println("Starting server...")
	l := fmt.Sprintf(":%d", config.Server.Port)
	if config.Server.UseTLS {
		log.Fatal(http.ListenAndServeTLS(l, config.Server.TLSCert, config.Server.TLSKey, h))
	}
	log.Fatal(http.ListenAndServe(l, h))
}
