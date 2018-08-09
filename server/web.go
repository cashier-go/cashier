package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gobuffalo/packr"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"golang.org/x/oauth2"

	"github.com/gorilla/csrf"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/nsheridan/cashier/lib"
	"github.com/nsheridan/cashier/server/auth"
	"github.com/nsheridan/cashier/server/config"
	"github.com/nsheridan/cashier/server/templates"
)

// appContext contains local context - cookiestore, authsession etc.
type appContext struct {
	cookiestore   *sessions.CookieStore
	authsession   *auth.Session
	requireReason bool
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

func (a *appContext) getCurrentURL(r *http.Request) string {
	session, _ := a.cookiestore.Get(r, "session")
	path, ok := session.Values["auth_url"]
	if !ok {
		return ""
	}
	return path.(string)
}

func (a *appContext) setCurrentURL(w http.ResponseWriter, r *http.Request) {
	session, _ := a.cookiestore.Get(r, "session")
	session.Values["auth_url"] = r.URL.Path
	session.Save(r, w)
}

func (a *appContext) isLoggedIn(w http.ResponseWriter, r *http.Request) bool {
	tok := a.getAuthTokenCookie(r)
	if !tok.Valid() || !authprovider.Valid(tok) {
		return false
	}
	return true
}

func (a *appContext) login(w http.ResponseWriter, r *http.Request) (int, error) {
	a.setCurrentURL(w, r)
	http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
	return http.StatusSeeOther, nil
}

// parseKey retrieves and unmarshals the signing request.
func extractKey(r *http.Request) (*lib.SignRequest, error) {
	var s lib.SignRequest
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
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
	if !authprovider.Valid(token) {
		return http.StatusUnauthorized, errors.New(http.StatusText(http.StatusUnauthorized))
	}

	// Sign the pubkey and issue the cert.
	req, err := extractKey(r)
	if err != nil {
		return http.StatusBadRequest, errors.Wrap(err, "unable to extract key from request")
	}

	if a.requireReason && req.Message == "" {
		w.Header().Add("X-Need-Reason", "required")
		return http.StatusForbidden, errors.New(http.StatusText(http.StatusForbidden))
	}

	username := authprovider.Username(token)
	authprovider.Revoke(token) // We don't need this anymore.
	cert, err := keysigner.SignUserKey(req, username)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "error signing key")
	}
	if err := certstore.SetCert(cert); err != nil {
		log.Printf("Error recording cert: %v", err)
	}
	if err := json.NewEncoder(w).Encode(&lib.SignResponse{
		Status:   "ok",
		Response: string(lib.GetPublicKey(cert)),
	}); err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "error encoding response")
	}
	return http.StatusOK, nil
}

// loginHandler starts the authentication process with the provider.
func loginHandler(a *appContext, w http.ResponseWriter, r *http.Request) (int, error) {
	state := newState()
	a.setAuthStateCookie(w, r, state)
	a.authsession = authprovider.StartSession(state)
	http.Redirect(w, r, a.authsession.AuthURL, http.StatusFound)
	return http.StatusFound, nil
}

// callbackHandler handles retrieving the access token from the auth provider and saves it for later use.
func callbackHandler(a *appContext, w http.ResponseWriter, r *http.Request) (int, error) {
	if r.FormValue("state") != a.getAuthStateCookie(r) {
		return http.StatusUnauthorized, errors.New(http.StatusText(http.StatusUnauthorized))
	}
	code := r.FormValue("code")
	if err := a.authsession.Authorize(authprovider, code); err != nil {
		return http.StatusInternalServerError, err
	}
	a.setAuthTokenCookie(w, r, a.authsession.Token)
	http.Redirect(w, r, a.getCurrentURL(r), http.StatusFound)
	return http.StatusFound, nil
}

// rootHandler starts the auth process. If the client is authenticated it renders the token to the user.
func rootHandler(a *appContext, w http.ResponseWriter, r *http.Request) (int, error) {
	if !a.isLoggedIn(w, r) {
		return a.login(w, r)
	}
	tok := a.getAuthTokenCookie(r)
	page := struct {
		Token string
	}{tok.AccessToken}

	tmpl := template.Must(template.New("token.html").Parse(templates.Token))
	tmpl.Execute(w, page)
	return http.StatusOK, nil
}

func listRevokedCertsHandler(a *appContext, w http.ResponseWriter, r *http.Request) (int, error) {
	revoked, err := certstore.GetRevoked()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	rl, err := keysigner.GenerateRevocationList(revoked)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "unable to generate KRL")
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(rl)
	return http.StatusOK, nil
}

func listAllCertsHandler(a *appContext, w http.ResponseWriter, r *http.Request) (int, error) {
	if !a.isLoggedIn(w, r) {
		return a.login(w, r)
	}
	tmpl := template.Must(template.New("certs.html").Parse(templates.Certs))
	tmpl.Execute(w, map[string]interface{}{
		csrf.TemplateTag: csrf.TemplateField(r),
	})
	return http.StatusOK, nil
}

func listCertsJSONHandler(a *appContext, w http.ResponseWriter, r *http.Request) (int, error) {
	if !a.isLoggedIn(w, r) {
		return http.StatusUnauthorized, errors.New(http.StatusText(http.StatusUnauthorized))
	}
	includeExpired, _ := strconv.ParseBool(r.URL.Query().Get("all"))
	certs, err := certstore.List(includeExpired)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	j, err := json.Marshal(certs)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	w.Write(j)
	return http.StatusOK, nil
}

func revokeCertHandler(a *appContext, w http.ResponseWriter, r *http.Request) (int, error) {
	if !a.isLoggedIn(w, r) {
		return a.login(w, r)
	}
	r.ParseForm()
	if err := certstore.Revoke(r.Form["cert_id"]); err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "unable to revoke certs")
	}
	http.Redirect(w, r, "/admin/certs", http.StatusSeeOther)
	return http.StatusSeeOther, nil
}

func healthcheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
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

func runHTTPServer(conf *config.Server, l net.Listener) {
	var err error
	ctx := &appContext{
		cookiestore:   sessions.NewCookieStore([]byte(conf.CookieSecret)),
		requireReason: conf.RequireReason,
	}
	ctx.cookiestore.Options = &sessions.Options{
		MaxAge:   900,
		Path:     "/",
		Secure:   conf.UseTLS,
		HttpOnly: true,
	}

	logfile := os.Stderr
	if conf.HTTPLogFile != "" {
		logfile, err = os.OpenFile(conf.HTTPLogFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0640)
		if err != nil {
			log.Printf("unable to open %s for writing. logging to stdout", conf.HTTPLogFile)
			logfile = os.Stderr
		}
	}

	CSRF := csrf.Protect([]byte(conf.CSRFSecret), csrf.Secure(conf.UseTLS))
	r := mux.NewRouter()
	r.Methods("GET").Path("/").Handler(appHandler{ctx, rootHandler})
	r.Methods("GET").Path("/auth/login").Handler(appHandler{ctx, loginHandler})
	r.Methods("GET").Path("/auth/callback").Handler(appHandler{ctx, callbackHandler})
	r.Methods("POST").Path("/sign").Handler(appHandler{ctx, signHandler})
	r.Methods("GET").Path("/revoked").Handler(appHandler{ctx, listRevokedCertsHandler})
	r.Methods("POST").Path("/admin/revoke").Handler(CSRF(appHandler{ctx, revokeCertHandler}))
	r.Methods("GET").Path("/admin/certs").Handler(CSRF(appHandler{ctx, listAllCertsHandler}))
	r.Methods("GET").Path("/admin/certs.json").Handler(appHandler{ctx, listCertsJSONHandler})
	r.Methods("GET").Path("/metrics").Handler(promhttp.Handler())
	r.Methods("GET").Path("/healthcheck").HandlerFunc(healthcheck)

	box := packr.NewBox("static")
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(box)))
	h := handlers.LoggingHandler(logfile, r)
	s := &http.Server{
		Handler: h,
	}
	s.Serve(l)
}
