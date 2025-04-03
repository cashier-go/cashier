package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/gorilla/csrf"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	wkfscache "github.com/nsheridan/autocert-wkfs-cache"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sid77/drop"
	"go4.org/wkfs"
	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/oauth2"

	"github.com/cashier-go/cashier/lib"
	"github.com/cashier-go/cashier/server/auth"
	"github.com/cashier-go/cashier/server/auth/github"
	"github.com/cashier-go/cashier/server/auth/gitlab"
	"github.com/cashier-go/cashier/server/auth/google"
	"github.com/cashier-go/cashier/server/auth/microsoft"
	"github.com/cashier-go/cashier/server/config"
	"github.com/cashier-go/cashier/server/metrics"
	"github.com/cashier-go/cashier/server/signer"
	"github.com/cashier-go/cashier/server/store"
)

// Server is a convenience wrapper around a *httpServer
type Server struct {
	httpServer *http.Server
	logfile    *os.File
}

// Shutdown the server and perform any cleanup
func (s *Server) Shutdown(ctx context.Context) error {
	s.logfile.Close()
	return s.httpServer.Shutdown(ctx)
}

func loadCerts(certFile, keyFile string) (tls.Certificate, error) {
	key, err := wkfs.ReadFile(keyFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("error reading TLS private key: %w", err)
	}
	cert, err := wkfs.ReadFile(certFile)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("error reading TLS certificate: %w", err)
	}
	return tls.X509KeyPair(cert, key)
}

func setupTLS(l net.Listener, conf *config.Server) (net.Listener, error) {
	var err error
	if conf.LetsEncryptServername != "" {
		m := autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(conf.LetsEncryptServername),
		}
		if conf.LetsEncryptCache != "" {
			m.Cache = wkfscache.Cache(conf.LetsEncryptCache)
		}
		return tls.NewListener(l, m.TLSConfig()), nil
	}
	if conf.TLSCert == "" || conf.TLSKey == "" {
		return nil, fmt.Errorf("TLS cert or key not specified in config")
	}
	tlsConfig := &tls.Config{}
	tlsConfig.Certificates = make([]tls.Certificate, 1)
	tlsConfig.Certificates[0], err = loadCerts(conf.TLSCert, conf.TLSKey)
	if err != nil {
		return nil, fmt.Errorf("unable to create TLS listener: %w", err)
	}
	return tls.NewListener(l, tlsConfig), nil
}

// Run the server.
func Run(conf *config.Config) (*Server, error) {
	var err error

	httpServerListenAddress := fmt.Sprintf("%s:%d", conf.Server.Addr, conf.Server.Port)
	httpServerListener, err := net.Listen("tcp", httpServerListenAddress)
	if err != nil {
		return nil, fmt.Errorf("unable to listen on %s:%d", conf.Server.Addr, conf.Server.Port)
	}

	if conf.Server.UseTLS {
		httpServerListener, err = setupTLS(httpServerListener, conf.Server)
		if err != nil {
			return nil, fmt.Errorf("unable to configure TLS: %w", err)
		}
	}

	// lock the current goroutine's thread to the current system thread before making UID/GID changes.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if conf.Server.User != "" {
		log.Print("Dropping privileges...")
		if err = drop.DropPrivileges(conf.Server.User); err != nil {
			return nil, fmt.Errorf("unable to drop privileges to user %q: %w", conf.Server.User, err)
		}
	}

	// Unprivileged section
	metrics.Register()

	var authprovider auth.Provider
	switch conf.Auth.Provider {
	case "github":
		authprovider, err = github.New(conf.Auth)
	case "gitlab":
		authprovider, err = gitlab.New(conf.Auth)
	case "google":
		authprovider, err = google.New(conf.Auth)
	case "microsoft":
		authprovider, err = microsoft.New(conf.Auth)
	default:
		return nil, fmt.Errorf("unknown provider %q", conf.Auth.Provider)
	}
	if err != nil {
		return nil, fmt.Errorf("unable to configure provider %q: %w", conf.Auth.Provider, err)
	}

	keysigner, err := signer.New(conf.SSH)
	if err != nil {
		return nil, fmt.Errorf("unable to configure signer: %w", err)
	}

	certstore, err := store.New(conf.Server.Database)
	if err != nil {
		return nil, fmt.Errorf("unable to configure datastore: %w", err)
	}

	app := &application{
		cookiestore:   sessions.NewCookieStore([]byte(conf.Server.CookieSecret)),
		requireReason: conf.Server.RequireReason,
		keysigner:     keysigner,
		certstore:     certstore,
		authprovider:  authprovider,
		config:        conf.Server,
		router:        mux.NewRouter(),
	}
	app.cookiestore.Options = &sessions.Options{
		MaxAge:   900,
		Path:     "/",
		Secure:   conf.Server.UseTLS,
		HttpOnly: true,
	}

	logfile := os.Stderr
	if conf.Server.HTTPLogFile != "" {
		logfile, err = os.OpenFile(conf.Server.HTTPLogFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o640)
		if err != nil {
			log.Printf("error opening log: %v. logging to stdout", err)
		}
	}

	app.setupRoutes()
	httpServerLoggingHandler := handlers.LoggingHandler(logfile, app.router)
	httpServer := &http.Server{
		Handler:      httpServerLoggingHandler,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("Starting HTTP server on %s", httpServerListenAddress)
	go httpServer.Serve(httpServerListener)
	return &Server{
		httpServer: httpServer,
		logfile:    logfile,
	}, nil
}

// mwVersion is middleware to add a X-Cashier-Version header to the response.
func mwVersion(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Cashier-Version", lib.Version)
		next.ServeHTTP(w, r)
	})
}

func encodeToken(s string) string {
	var buffer bytes.Buffer
	chunkSize := 70
	runes := []rune(base64.StdEncoding.EncodeToString([]byte(s)))

	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		buffer.WriteString(string(runes[i:end]))
		buffer.WriteString("\n")
	}
	return buffer.String()
}

//go:embed static
var static embed.FS

// application contains local context - cookiestore, authsession etc.
type application struct {
	cookiestore   *sessions.CookieStore
	authprovider  auth.Provider
	certstore     store.CertStorer
	keysigner     *signer.KeySigner
	router        *mux.Router
	config        *config.Server
	requireReason bool
}

func (a *application) setupRoutes() {
	// login required
	csrfHandler := csrf.Protect([]byte(a.config.CSRFSecret), csrf.Secure(a.config.UseTLS))
	a.router.Methods("GET").Path("/").Handler(a.authed(http.HandlerFunc(a.index)))
	a.router.Methods("POST").Path("/admin/revoke").Handler(a.authed(csrfHandler(http.HandlerFunc(a.revoke))))
	a.router.Methods("GET").Path("/admin/certs").Handler(a.authed(csrfHandler(http.HandlerFunc(a.getAllCerts))))
	a.router.Methods("GET").Path("/admin/certs.json").Handler(a.authed(http.HandlerFunc(a.getCertsJSON)))

	// no login required
	a.router.Methods("GET").Path("/auth/login").HandlerFunc(a.auth)
	a.router.Methods("GET").Path("/auth/callback").HandlerFunc(a.auth)
	a.router.Methods("GET").Path("/revoked").HandlerFunc(a.revoked)
	a.router.Methods("POST").Path("/sign").HandlerFunc(a.sign)

	a.router.Methods("GET").Path("/healthcheck").HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "ok")
	})
	a.router.Methods("GET").Path("/metrics").Handler(promhttp.Handler())

	// static files
	a.router.PathPrefix("/static/").Handler(http.FileServer(http.FS(static)))

	// middlewares
	a.router.Use(mwVersion)
	a.router.Use(handlers.CompressHandler)
	a.router.Use(handlers.RecoveryHandler())
}

func (a *application) getAuthToken(r *http.Request) *oauth2.Token {
	token := &oauth2.Token{}
	marshalled := a.getSessionVariable(r, "token")
	json.Unmarshal([]byte(marshalled), token)
	return token
}

func (a *application) setAuthToken(w http.ResponseWriter, r *http.Request, token *oauth2.Token) {
	v, _ := json.Marshal(token)
	a.setSessionVariable(w, r, "token", string(v))
}

func (a *application) getSessionVariable(r *http.Request, key string) string {
	session, _ := a.cookiestore.Get(r, "session")
	v, ok := session.Values[key].(string)
	if !ok {
		v = ""
	}
	return v
}

func (a *application) setSessionVariable(w http.ResponseWriter, r *http.Request, key, value string) {
	session, _ := a.cookiestore.Get(r, "session")
	session.Values[key] = value
	session.Save(r, w)
}

func (a *application) authed(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := a.getAuthToken(r)
		ctx := r.Context()
		if !t.Valid() || !a.authprovider.Valid(ctx, t) {
			a.setSessionVariable(w, r, "origin_url", r.URL.RequestURI())
			http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
