package main

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"go4.org/wkfs"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/oauth2"

	"github.com/gorilla/csrf"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	wkfscache "github.com/nsheridan/autocert-wkfs-cache"
	"github.com/nsheridan/cashier/lib"
	"github.com/nsheridan/cashier/server/auth"
	"github.com/nsheridan/cashier/server/auth/github"
	"github.com/nsheridan/cashier/server/auth/gitlab"
	"github.com/nsheridan/cashier/server/auth/google"
	"github.com/nsheridan/cashier/server/config"
	"github.com/nsheridan/cashier/server/signer"
	"github.com/nsheridan/cashier/server/static"
	"github.com/nsheridan/cashier/server/store"
	"github.com/nsheridan/cashier/server/templates"
	"github.com/nsheridan/cashier/server/wkfs/vaultfs"
	"github.com/nsheridan/wkfs/s3"
	"github.com/sid77/drop"
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
	if !tok.Valid() || !a.authprovider.Valid(tok) {
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
	ok := a.authprovider.Valid(token)
	if !ok {
		return http.StatusUnauthorized, errors.New(http.StatusText(http.StatusUnauthorized))
	}

	// Sign the pubkey and issue the cert.
	req, err := extractKey(r)
	if err != nil {
		return http.StatusBadRequest, errors.Wrap(err, "unable to extract key from request")
	}
	username := a.authprovider.Username(token)
	a.authprovider.Revoke(token) // We don't need this anymore.
	cert, err := a.sshKeySigner.SignUserKey(req, username)
	if err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "error signing key")
	}
	if err := a.certstore.SetCert(cert); err != nil {
		log.Printf("Error recording cert: %v", err)
	}
	if err := json.NewEncoder(w).Encode(&lib.SignResponse{
		Status:   "ok",
		Response: lib.GetPublicKey(cert),
	}); err != nil {
		return http.StatusInternalServerError, errors.Wrap(err, "error encoding response")
	}
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
	revoked, err := a.certstore.GetRevoked()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	rl, err := a.sshKeySigner.GenerateRevocationList(revoked)
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
	certs, err := a.certstore.List(includeExpired)
	j, err := json.Marshal(certs)
	if err != nil {
		return http.StatusInternalServerError, errors.New(http.StatusText(http.StatusInternalServerError))
	}
	w.Write(j)
	return http.StatusOK, nil
}

func revokeCertHandler(a *appContext, w http.ResponseWriter, r *http.Request) (int, error) {
	if !a.isLoggedIn(w, r) {
		return a.login(w, r)
	}
	r.ParseForm()
	for _, id := range r.Form["cert_id"] {
		if err := a.certstore.Revoke(id); err != nil {
			return http.StatusInternalServerError, errors.Wrap(err, "unable to revoke")
		}
	}
	http.Redirect(w, r, "/admin/certs", http.StatusSeeOther)
	return http.StatusSeeOther, nil
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
		return nil, errors.Wrap(err, "failed to parse config file")
	}
	defer f.Close()
	return config.ReadConfig(f)
}

func loadCerts(certFile, keyFile string) (tls.Certificate, error) {
	key, err := wkfs.ReadFile(keyFile)
	if err != nil {
		return tls.Certificate{}, errors.Wrap(err, "error reading TLS private key")
	}
	cert, err := wkfs.ReadFile(certFile)
	if err != nil {
		return tls.Certificate{}, errors.Wrap(err, "error reading TLS certificate")
	}
	return tls.X509KeyPair(cert, key)
}

func main() {
	// Privileged section
	flag.Parse()
	conf, err := readConfig(*cfg)
	if err != nil {
		log.Fatal(err)
	}

	// Register well-known filesystems.
	if conf.AWS == nil {
		conf.AWS = &config.AWS{}
	}
	s3.Register(&s3.Options{
		Region:    conf.AWS.Region,
		AccessKey: conf.AWS.AccessKey,
		SecretKey: conf.AWS.SecretKey,
	})
	vaultfs.Register(conf.Vault)

	signer, err := signer.New(conf.SSH)
	if err != nil {
		log.Fatal(err)
	}

	logfile := os.Stderr
	if conf.Server.HTTPLogFile != "" {
		logfile, err = os.OpenFile(conf.Server.HTTPLogFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0640)
		if err != nil {
			log.Printf("unable to open %s for writing. logging to stdout", conf.Server.HTTPLogFile)
			logfile = os.Stderr
		}
	}

	laddr := fmt.Sprintf("%s:%d", conf.Server.Addr, conf.Server.Port)
	l, err := net.Listen("tcp", laddr)
	if err != nil {
		log.Fatal(errors.Wrapf(err, "unable to listen on %s:%d", conf.Server.Addr, conf.Server.Port))
	}

	tlsConfig := &tls.Config{}
	if conf.Server.UseTLS {
		if conf.Server.LetsEncryptServername != "" {
			m := autocert.Manager{
				Prompt:     autocert.AcceptTOS,
				Cache:      wkfscache.Cache(conf.Server.LetsEncryptCache),
				HostPolicy: autocert.HostWhitelist(conf.Server.LetsEncryptServername),
			}
			tlsConfig.GetCertificate = m.GetCertificate
		} else {
			if conf.Server.TLSCert == "" || conf.Server.TLSKey == "" {
				log.Fatal("TLS cert or key not specified in config")
			}
			tlsConfig.Certificates = make([]tls.Certificate, 1)
			tlsConfig.Certificates[0], err = loadCerts(conf.Server.TLSCert, conf.Server.TLSKey)
			if err != nil {
				log.Fatal(errors.Wrap(err, "unable to create TLS listener"))
			}
		}
		l = tls.NewListener(l, tlsConfig)
	}

	if conf.Server.User != "" {
		log.Print("Dropping privileges...")
		if err := drop.DropPrivileges(conf.Server.User); err != nil {
			log.Fatal(errors.Wrap(err, "unable to drop privileges"))
		}
	}

	// Unprivileged section
	var authprovider auth.Provider
	switch conf.Auth.Provider {
	case "google":
		authprovider, err = google.New(conf.Auth)
	case "github":
		authprovider, err = github.New(conf.Auth)
	case "gitlab":
		authprovider, err = gitlab.New(conf.Auth)
	default:
		log.Fatalf("Unknown provider %s\n", conf.Auth.Provider)
	}
	if err != nil {
		log.Fatal(errors.Wrapf(err, "unable to use provider '%s'", conf.Auth.Provider))
	}

	certstore, err := store.New(conf.Server.Database)
	if err != nil {
		log.Fatal(err)
	}
	ctx := &appContext{
		cookiestore:  sessions.NewCookieStore([]byte(conf.Server.CookieSecret)),
		authprovider: authprovider,
		sshKeySigner: signer,
		certstore:    certstore,
	}
	ctx.cookiestore.Options = &sessions.Options{
		MaxAge:   900,
		Path:     "/",
		Secure:   conf.Server.UseTLS,
		HttpOnly: true,
	}

	CSRF := csrf.Protect([]byte(conf.Server.CSRFSecret), csrf.Secure(conf.Server.UseTLS))
	r := mux.NewRouter()
	r.Methods("GET").Path("/").Handler(appHandler{ctx, rootHandler})
	r.Methods("GET").Path("/auth/login").Handler(appHandler{ctx, loginHandler})
	r.Methods("GET").Path("/auth/callback").Handler(appHandler{ctx, callbackHandler})
	r.Methods("POST").Path("/sign").Handler(appHandler{ctx, signHandler})
	r.Methods("GET").Path("/revoked").Handler(appHandler{ctx, listRevokedCertsHandler})
	r.Methods("POST").Path("/admin/revoke").Handler(CSRF(appHandler{ctx, revokeCertHandler}))
	r.Methods("GET").Path("/admin/certs").Handler(CSRF(appHandler{ctx, listAllCertsHandler}))
	r.Methods("GET").Path("/admin/certs.json").Handler(appHandler{ctx, listCertsJSONHandler})
	r.PathPrefix("/").Handler(http.FileServer(static.FS(false)))
	h := handlers.LoggingHandler(logfile, r)

	log.Printf("Starting server on %s", laddr)
	s := &http.Server{
		Handler: h,
	}
	log.Fatal(s.Serve(l))
}
