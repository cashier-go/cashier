package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	srvOK    = "ok"
	srvError = "error"
)

// localserver is a http server started by the cashier client.  It listens for a
// token on an unused localhost port so that the token can be sent by the
// browser to the client directly without needing to copy/paste.
type localserver struct {
	token, response chan string
	// endpoint is the local port and path cashierd sends the token to.
	// Rendered to "http://localhost:<port>/<path>" in
	// `server/templates/token.go`.
	endpoint   string
	httpserver *http.Server
}

func startServer(ca string) localserver {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		log.Println("error starting local server:", err)
		return localserver{}
	}
	port := l.Addr().(*net.TCPAddr).Port
	path := "/" + uuid.NewString()
	ls := localserver{
		token:    make(chan string, 1),
		response: make(chan string, 1),
		endpoint: fmt.Sprintf("%d%s", port, path),
		httpserver: &http.Server{
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
	}
	ls.httpserver.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// Allow browser to xhr the token from the oauth callback page.
		w.Header().Set("Access-Control-Allow-Origin", strings.TrimSuffix(ca, "/"))
		var token string
		switch r.Method {
		case http.MethodOptions:
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		case http.MethodGet, http.MethodPost:
			token = r.FormValue("token")
			if token == "" {
				// no token, no service
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		// Pass the token to the waiting client for validation, then wait until
		// it completes the signing request.
		ls.token <- token
		resp := <-ls.response
		if resp != srvOK {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write([]byte(resp))
	})
	go ls.httpserver.Serve(l)
	return ls
}

func (l *localserver) stop(ctx context.Context) {
	if l.httpserver != nil {
		l.httpserver.Shutdown(ctx)
	}
}

func (l *localserver) respond(val string) {
	if l.response != nil {
		l.response <- val
	}
}
