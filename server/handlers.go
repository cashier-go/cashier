package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gorilla/csrf"
	"golang.org/x/oauth2"

	"github.com/cashier-go/cashier/lib"
	"github.com/cashier-go/cashier/server/store"
	"github.com/cashier-go/cashier/server/templates"
)

func tokenFromRequest(r *http.Request) *oauth2.Token {
	token := &oauth2.Token{}
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(strings.ToUpper(header), "BEARER ") {
		t := strings.Split(header, " ")[1]
		token.AccessToken = t
	}
	return token
}

func (a *application) sign(w http.ResponseWriter, r *http.Request) {
	var (
		errNeedsReason  = errors.New("signing request needs a reason")
		errUnauthorized = errors.New("unauthorized")
		errSigningKey   = errors.New("error signing key")
	)

	fail := func(w http.ResponseWriter, code int, err error) {
		w.WriteHeader(code)
		json.NewEncoder(w).Encode(&lib.SignResponse{
			Status:   "error",
			Response: fmt.Sprintf("%s: %s", http.StatusText(code), err),
		})
	}

	ctx := r.Context()
	token := tokenFromRequest(r)
	if !a.authprovider.Valid(ctx, token) {
		fail(w, http.StatusUnauthorized, errUnauthorized)
		return
	}

	// Attempt to sign the pubkey and return a SignResponse.
	req := lib.SignRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}

	if a.config.RequireReason && req.Message == "" {
		w.Header().Add("X-Need-Reason", "required")
		fail(w, http.StatusForbidden, errNeedsReason)
		return
	}

	username := a.authprovider.Username(ctx, token)
	a.authprovider.Revoke(ctx, token) // We don't need this anymore.
	cert, err := a.keysigner.SignUserKey(&req, username)
	if err != nil {
		fail(w, http.StatusInternalServerError, fmt.Errorf("%w: %w", errSigningKey, err))
		return
	}

	rec := store.MakeRecord(cert)
	rec.Message = req.Message
	if err := a.certstore.SetRecord(rec); err != nil {
		log.Printf("Error recording cert: %v", err)
	}
	if err := json.NewEncoder(w).Encode(&lib.SignResponse{
		Status:   "ok",
		Response: string(lib.GetPublicKey(cert)),
	}); err != nil {
		fail(w, http.StatusInternalServerError, fmt.Errorf("%w: %w", errSigningKey, err))
		return
	}
}

func (a *application) auth(w http.ResponseWriter, r *http.Request) {
	switch r.URL.EscapedPath() {
	case "/auth/login":
		buf := make([]byte, 32)
		io.ReadFull(rand.Reader, buf)
		state := hex.EncodeToString(buf)
		// Without the state cookie the callback cannot succeed, so fail early.
		if err := a.setSessionVariable(w, r, "state", state); err != nil {
			log.Print(err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
			return
		}
		http.Redirect(w, r, a.authprovider.StartSession(state), http.StatusFound)
	case "/auth/callback":
		ctx := r.Context()
		state := a.getSessionVariable(r, "state")
		if state == "" || r.FormValue("state") != state {
			log.Printf("Not authorized on /auth/callback")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, http.StatusText(http.StatusUnauthorized))
			return
		}
		if err := a.deleteSessionVariable(w, r, "state"); err != nil {
			log.Print(err)
		}
		originURL := a.getSessionVariable(r, "origin_url")
		if originURL == "" {
			originURL = "/"
		}
		code := r.FormValue("code")
		token, err := a.authprovider.Exchange(r.Context(), code)
		if err != nil {
			log.Printf("Error on /auth/callback: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, templates.Retry)
			return
		}
		// if we don't check the token here, it gets into an auth loop
		if !a.authprovider.Valid(ctx, token) {
			log.Printf("Not authorized")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, http.StatusText(http.StatusUnauthorized))
			return
		}

		username := a.authprovider.Username(ctx, token)
		if username == "" {
			log.Print("No username returned by the auth provider")
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
			return
		}
		if err := a.setSessionVariable(w, r, "username", username); err != nil {
			log.Print(err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
			return
		}

		origin, err := url.Parse(originURL)
		if err != nil {
			log.Printf("Unparseable origin %q: %v", originURL, err)
			origin = &url.URL{Path: "/"}
		}
		if origin.Path == "/" {
			log.Print("Token found on /auth/callback, returning the token page")
			a.renderToken(w, token, origin.Query().Get("localserver"))
			return
		}
		log.Printf("Token found on /auth/callback, redirecting to %s", originURL)
		http.Redirect(w, r, originURL, http.StatusFound)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (a *application) index(w http.ResponseWriter, r *http.Request) {
	if err := a.setSessionVariable(w, r, "origin_url", r.URL.RequestURI()); err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
		return
	}
	http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
}

// renderToken displays the token for the user to copy, and for the client's local server to collect.
func (a *application) renderToken(w http.ResponseWriter, token *oauth2.Token, localserver string) {
	page := struct {
		Token       string
		Localserver string
	}{
		Token:       encodeToken(token.AccessToken),
		Localserver: localserver,
	}
	tmpl := template.Must(template.New("token.html").Parse(templates.Token))
	if err := tmpl.Execute(w, page); err != nil {
		log.Printf("Error rendering token page: %v", err)
	}
}

func (a *application) revoked(w http.ResponseWriter, r *http.Request) {
	revoked, err := a.certstore.GetRevoked()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "error retrieving revoked certs: %v", err)
		return
	}
	rl, err := a.keysigner.GenerateRevocationList(revoked)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "unable to generate KRL: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(rl)
}

func (a *application) getAllCerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-CSRF-Token", csrf.Token(r))
	tmpl := template.Must(template.New("certs.html").Parse(templates.Certs))
	tmpl.Execute(w, map[string]interface{}{
		csrf.TemplateTag: csrf.TemplateField(r),
	})
}

func (a *application) getCertsJSON(w http.ResponseWriter, r *http.Request) {
	includeExpired, _ := strconv.ParseBool(r.URL.Query().Get("all"))
	certs, err := a.certstore.List(includeExpired)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
		return
	}
	if err := json.NewEncoder(w).Encode(certs); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, http.StatusText(http.StatusInternalServerError))
		return
	}
}

func (a *application) revoke(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	certIDs := r.Form["cert_id"]
	log.Printf("Revoking certs %v on behalf of %s", certIDs, a.getSessionVariable(r, "username"))
	if err := a.certstore.Revoke(certIDs); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "Unable to revoke certs")
	} else {
		http.Redirect(w, r, "/admin/certs", http.StatusSeeOther)
	}
}
