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
	endpoint string
	path     string
	// origin is the cashierd url the browser is allowed to xhr from.
	origin     string
	httpserver *http.Server
}

func newLocalServer(ca, path string) *localserver {
	ls := &localserver{
		token:    make(chan string, 1),
		response: make(chan string, 1),
		path:     path,
		origin:   strings.TrimSuffix(ca, "/"),
	}
	return ls
}

func startServer(ca string) *localserver {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		log.Println("error starting local server:", err)
		return &localserver{}
	}
	port := listener.Addr().(*net.TCPAddr).Port
	ls := newLocalServer(ca, "/"+uuid.NewString())
	ls.endpoint = fmt.Sprintf("%d%s", port, ls.path)
	ls.httpserver = &http.Server{
		Handler:      ls,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	go ls.httpserver.Serve(listener)
	return ls
}

func (l *localserver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != l.path {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	// Allow browser to xhr the token from the oauth callback page.
	w.Header().Set("Access-Control-Allow-Origin", l.origin)
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
	l.token <- token
	resp := <-l.response
	if resp != srvOK {
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.Write([]byte(resp))
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
