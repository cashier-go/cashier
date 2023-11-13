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

var (
	srvOK    = "ok"
	srvError = "error"
)

type localserver struct {
	token, response chan string
	port            int
	path            string
	ca              string
	httpserver      *http.Server
}

func startServer(ca string) localserver {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		log.Println("error starting local server:", err)
		return localserver{}
	}
	ls := localserver{
		token:    make(chan string, 1),
		response: make(chan string, 1),
		port:     l.Addr().(*net.TCPAddr).Port,
		path:     "/" + uuid.NewString(),
		httpserver: &http.Server{
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		ca: ca,
	}
	http.HandleFunc(ls.path, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", strings.TrimSuffix(ls.ca, "/"))
		token := r.FormValue("token")
		if token != "" {
			ls.token <- r.FormValue("token")
		} else {
			// no token, no service
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// block here until we get something to send back
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

func (l *localserver) url() string {
	return fmt.Sprintf("%d%s", l.port, l.path)
}

func (l *localserver) respond(val string) {
	if l.response != nil {
		l.response <- val
	}
}
