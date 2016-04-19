package signer

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/nsheridan/cashier/lib"
	"github.com/nsheridan/cashier/testdata"

	"golang.org/x/crypto/ssh"
)

var (
	key, _ = ssh.ParsePrivateKey(testdata.Priv)
	signer = &KeySigner{
		ca:         key,
		validity:   12 * time.Hour,
		principals: []string{"ec2-user"},
	}
)

func TestSign(t *testing.T) {
	s := &lib.SignRequest{
		Key:        string(testdata.Pub),
		Principal:  "gopher1",
		ValidUntil: time.Now().Add(1 * time.Hour),
	}
	ret, err := signer.Sign(s)
	if err != nil {
		t.Fatal(err)
	}

	c, _, _, _, err := ssh.ParseAuthorizedKey([]byte(ret))
	cert, ok := c.(*ssh.Certificate)
	if !ok {
		t.Fatalf("Expected type *ssh.Certificate, got %v (%T)", cert, cert)
	}
}

func TestCert(t *testing.T) {
	r := &lib.SignRequest{
		Key:        string(testdata.Pub),
		Principal:  "gopher1",
		ValidUntil: time.Now().Add(1 * time.Hour),
	}
	ret, err := signer.Sign(r)
	if err != nil {
		t.Fatal(err)
	}
	c, _, _, _, err := ssh.ParseAuthorizedKey([]byte(ret))
	cert, ok := c.(*ssh.Certificate)
	if !ok {
		t.Fatalf("Expected type *ssh.Certificate, got %v (%T)", cert, cert)
	}
	if !bytes.Equal(cert.SignatureKey.Marshal(), signer.ca.PublicKey().Marshal()) {
		t.Fatal("Cert signer and server signer don't match")
	}
	var principals []string
	principals = append(principals, r.Principal)
	principals = append(principals, signer.principals...)
	if !reflect.DeepEqual(cert.ValidPrincipals, principals) {
		t.Fatalf("Expected %s, got %s", cert.ValidPrincipals, principals)
	}
	k1, _, _, _, err := ssh.ParseAuthorizedKey([]byte(r.Key))
	k2 := cert.Key
	if !bytes.Equal(k1.Marshal(), k2.Marshal()) {
		t.Fatal("Cert key doesn't match public key")
	}
	if cert.ValidBefore != uint64(r.ValidUntil.Unix()) {
		t.Fatalf("Invalid validity, expected %d, got %d", r.ValidUntil, cert.ValidBefore)
	}
}
