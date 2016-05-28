package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nsheridan/cashier/lib"
	"github.com/nsheridan/cashier/testdata"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func TestLoadCert(t *testing.T) {
	priv, _ := ssh.ParseRawPrivateKey(testdata.Priv)
	key := priv.(*rsa.PrivateKey)
	pub, _ := ssh.NewPublicKey(&key.PublicKey)
	c := &ssh.Certificate{
		Key:         pub,
		CertType:    ssh.UserCert,
		ValidBefore: ssh.CertTimeInfinity,
		ValidAfter:  0,
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	c.SignCert(rand.Reader, signer)
	a := agent.NewKeyring()
	if err := installCert(a, c, key); err != nil {
		t.Fatal(err)
	}
	listedKeys, err := a.List()
	if err != nil {
		t.Fatalf("Error reading from agent: %v", err)
	}
	if len(listedKeys) != 1 {
		t.Fatalf("Expected 1 key, got %d", len(listedKeys))
	}
	if !bytes.Equal(listedKeys[0].Marshal(), c.Marshal()) {
		t.Fatal("Certs not equal")
	}
}

func TestSignGood(t *testing.T) {
	res := &lib.SignResponse{
		Status:   "ok",
		Response: string(testdata.Cert),
	}
	j, _ := json.Marshal(res)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, string(j))
	}))
	defer ts.Close()
	_, err := send([]byte(`{}`), "token", ts.URL, true)
	if err != nil {
		t.Fatal(err)
	}
	k, _, _, _, err := ssh.ParseAuthorizedKey(testdata.Pub)
	if err != nil {
		t.Fatal(err)
	}
	c := &config{
		CA:       ts.URL,
		Validity: "24h",
	}
	cert, err := sign(k, "token", c)
	if cert == nil && err != nil {
		t.Fatal(err)
	}
}

func TestSignBad(t *testing.T) {
	res := &lib.SignResponse{
		Status:   "error",
		Response: `{"response": "error"}`,
	}
	j, _ := json.Marshal(res)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, string(j))
	}))
	defer ts.Close()
	_, err := send([]byte(`{}`), "token", ts.URL, true)
	if err != nil {
		t.Fatal(err)
	}
	k, _, _, _, err := ssh.ParseAuthorizedKey(testdata.Pub)
	if err != nil {
		t.Fatal(err)
	}
	c := &config{
		CA:       ts.URL,
		Validity: "24h",
	}
	cert, err := sign(k, "token", c)
	if cert != nil && err == nil {
		t.Fatal(err)
	}
}
