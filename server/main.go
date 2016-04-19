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
	"time"

	"golang.org/x/oauth2"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/nsheridan/cashier/lib"
	"github.com/nsheridan/cashier/server/auth"
	"github.com/nsheridan/cashier/server/auth/google"
	"github.com/nsheridan/cashier/server/config"
	"github.com/nsheridan/cashier/server/signer"
)

var (
	cfg = flag.String("config_file", "config.json", "Path to configuration file.")
)

type appContext struct {
	cookiestore   *sessions.CookieStore
	authprovider  auth.Provider
	authsession   *auth.Session
	views         *template.Template
	sshKeySigner  *signer.KeySigner
	jwtSigningKey []byte
}

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
	if !a.authprovider.Valid(&tok) {
		return nil
	}
	return &tok
}

func (a *appContext) setAuthCookie(w http.ResponseWriter, r *http.Request, t *oauth2.Token) {
	session, _ := a.cookiestore.Get(r, "tok")
	val, _ := json.Marshal(t)
	session.Values["token"] = val
	session.Save(r, w)
}

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
func signHandler(a *appContext, w http.ResponseWriter, r *http.Request) (int, error) {
	jwtoken, err := jwt.ParseFromRequest(r, func(t *jwt.Token) (interface{}, error) {
		return a.jwtSigningKey, nil
	})
	if err != nil {
		return http.StatusUnauthorized, errors.New(http.StatusText(http.StatusUnauthorized))
	}
	if !jwtoken.Valid {
		log.Printf("Token %v not valid", jwtoken)
		return http.StatusUnauthorized, errors.New(http.StatusText(http.StatusUnauthorized))
	}
	expiry := int64(jwtoken.Claims["exp"].(float64))
	token := &oauth2.Token{
		AccessToken: jwtoken.Claims["token"].(string),
		Expiry:      time.Unix(expiry, 0),
	}
	ok := a.authprovider.Valid(token)
	if !ok {
		return http.StatusUnauthorized, errors.New(http.StatusText(http.StatusUnauthorized))
	}
	// finally sign the pubkey and issue the cert.
	req, err := parseKey(r)
	req.Principal = a.authprovider.Username(token)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	signed, err := a.sshKeySigner.Sign(req)
	a.authprovider.Revoke(token)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	json.NewEncoder(w).Encode(&lib.SignResponse{
		Status:   "ok",
		Response: signed,
	})
	return http.StatusOK, nil
}

func loginHandler(a *appContext, w http.ResponseWriter, r *http.Request) (int, error) {
	a.authsession = a.authprovider.StartSession(newState(32))
	http.Redirect(w, r, a.authsession.AuthURL, http.StatusFound)
	return http.StatusFound, nil
}

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

func rootHandler(a *appContext, w http.ResponseWriter, r *http.Request) (int, error) {
	tok := a.getAuthCookie(r)
	if !tok.Valid() {
		http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
		return http.StatusSeeOther, nil
	}
	j := jwt.New(jwt.SigningMethodHS256)
	j.Claims["token"] = tok.AccessToken
	j.Claims["exp"] = tok.Expiry.Unix()
	t, err := j.SignedString(a.jwtSigningKey)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	page := struct {
		Token string
	}{t}
	a.views.ExecuteTemplate(w, "token.html", page)
	return http.StatusOK, nil
}

type appHandler struct {
	*appContext
	h func(*appContext, http.ResponseWriter, *http.Request) (int, error)
}

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

func newState() string {
	k := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		return "unexpectedstring"
	}
	return hex.EncodeToString(k)
}

func main() {
	flag.Parse()
	config, err := config.ReadConfig(*cfg)
	if err != nil {
		log.Fatal(err)
	}
	signer, err := signer.NewSigner(config.SSH)
	if err != nil {
		log.Fatal(err)
	}
	authprovider := google.New(config.Auth)
	ctx := &appContext{
		cookiestore:   sessions.NewCookieStore([]byte(config.Server.CookieSecret)),
		authprovider:  authprovider,
		views:         template.Must(template.ParseGlob("templates/*")),
		sshKeySigner:  signer,
		jwtSigningKey: []byte(config.Auth.JWTSigningKey),
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
