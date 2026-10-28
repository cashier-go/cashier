package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestLocalServer() *localserver {
	return newLocalServer("https://cashier.example.com/", "/callback-path")
}

func TestUnknownPath(t *testing.T) {
	ls := newTestLocalServer()
	w := httptest.NewRecorder()
	ls.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/blammo", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAccessControlPreflight(t *testing.T) {
	ls := newTestLocalServer()
	w := httptest.NewRecorder()
	ls.ServeHTTP(w, httptest.NewRequest(http.MethodOptions, ls.path, nil))

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "https://cashier.example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type", w.Header().Get("Access-Control-Allow-Headers"))
}

func TestMissingToken(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			ls := newTestLocalServer()
			w := httptest.NewRecorder()
			ls.ServeHTTP(w, httptest.NewRequest(method, ls.path, nil))
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

func TestBadMethod(t *testing.T) {
	ls := newTestLocalServer()
	w := httptest.NewRecorder()
	ls.ServeHTTP(w, httptest.NewRequest(http.MethodHead, ls.path, nil))
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestToken(t *testing.T) {
	tests := []struct {
		name     string
		response string
		wantCode int
	}{
		{"signing succeeded", srvOK, http.StatusOK},
		{"signing failed", srvError, http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ls := newTestLocalServer()
			var got string
			go func() {
				got = <-ls.token
				ls.respond(test.response)
			}()
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, ls.path+"?token=abcdef", nil)
			ls.ServeHTTP(w, r)

			assert.Equal(t, test.wantCode, w.Code)
			assert.Equal(t, test.response, w.Body.String())
			assert.Equal(t, "abcdef", got)
		})
	}
}
