package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/csrf"
	"golang.org/x/oauth2"

	"github.com/nsheridan/cashier/lib"
	"github.com/nsheridan/cashier/server/store"
	"github.com/nsheridan/cashier/server/templates"
)

func (a *application) sign(w http.ResponseWriter, r *http.Request) {
	var t string
	if ah := r.Header.Get("Authorization"); ah != "" {
		if len(ah) > 6 && strings.ToUpper(ah[0:7]) == "BEARER " {
			t = ah[7:]
		}
	}

	token := &oauth2.Token{
		AccessToken: t,
	}
	if !a.authprovider.Valid(token) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, http.StatusText(http.StatusUnauthorized))
		return
	}

	// Sign the pubkey and issue the cert.
	req := &lib.SignRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, http.StatusText(http.StatusBadRequest))
		return
	}

	if a.requireReason && req.Message == "" {
		w.Header().Add("X-Need-Reason", "required")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, http.StatusText(http.StatusForbidden))
		return
	}

	username := a.authprovider.Username(token)
	a.authprovider.Revoke(token) // We don't need this anymore.
	cert, err := a.keysigner.SignUserKey(req, username)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error signing key: %v", err)
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
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error signing key: %v", err)
		return
	}
}

func (a *application) auth(w http.ResponseWriter, r *http.Request) {
	switch r.URL.EscapedPath() {
	case "/auth/login":
		buf := make([]byte, 32)
		io.ReadFull(rand.Reader, buf)
		state := hex.EncodeToString(buf)
		a.setSessionVariable(w, r, "state", state)
		http.Redirect(w, r, a.authprovider.StartSession(state), http.StatusFound)
	case "/auth/callback":
		state := a.getSessionVariable(r, "state")
		if r.FormValue("state") != state {
			log.Printf("Not authorized on /auth/callback")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprintf(w, http.StatusText(http.StatusUnauthorized))
			return
		}
		originURL := a.getSessionVariable(r, "origin_url")
		if originURL == "" {
			originURL = "/"
		}
		code := r.FormValue("code")
		token, err := a.authprovider.Exchange(r.Context(), code)
		if err != nil {
			log.Printf("Error on /auth/callback: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "%s\n%v", http.StatusText(http.StatusInternalServerError), err)
			return
		}
		log.Printf("Token found on /auth/callback, redirecting to %s", originURL)
		a.setAuthToken(w, r, token)

		// if we don't check the token here, it gets into an auth loop
		if !a.authprovider.Valid(token) {
			log.Printf("Not authorized")
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, http.StatusText(http.StatusUnauthorized))
			return
		}
		http.Redirect(w, r, originURL, http.StatusFound)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (a *application) index(w http.ResponseWriter, r *http.Request) {
	localserver := r.FormValue("localserver")
	tok := a.getAuthToken(r)
	if tok == nil {
		w.WriteHeader(400)
		fmt.Fprint(w, http.StatusText(400))
		return
	}
	page := struct {
		Token       string
		Localserver string
	}{
		Token:       encodeToken(tok.AccessToken),
		Localserver: localserver,
	}
	tmpl := template.Must(template.New("token.html").Parse(templates.Token))
	tmpl.Execute(w, page)
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
	if err := a.certstore.Revoke(r.Form["cert_id"]); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "Unable to revoke certs")
	} else {
		http.Redirect(w, r, "/admin/certs", http.StatusSeeOther)
	}
}
