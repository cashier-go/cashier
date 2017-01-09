package client

import (
	"crypto/rsa"
	"reflect"
	"testing"

	"golang.org/x/crypto/ed25519"
)

func TestGenerateKeys(t *testing.T) {
	var tests = []struct {
		keytype string
		keysize int
		want    string
	}{
		{"rsa", 1024, "*rsa.PrivateKey"},
		{"rsa", 0, "*rsa.PrivateKey"},
		{"ecdsa", 0, "*ecdsa.PrivateKey"},
		{"ecdsa", 384, "*ecdsa.PrivateKey"},
		{"ed25519", 0, "*ed25519.PrivateKey"},
	}

	for _, tst := range tests {
		var k Key
		var err error
		k, _, err = GenerateKey(KeyType(tst.keytype), KeySize(tst.keysize))
		if err != nil {
			t.Error(err)
		}
		if reflect.TypeOf(k).String() != tst.want {
			t.Errorf("Wrong key type returned. Got %T, wanted %s", k, tst.want)
		}
	}
}

func TestDefaultOptions(t *testing.T) {
	k, _, err := GenerateKey()
	if err != nil {
		t.Error(err)
	}
	_, ok := k.(*rsa.PrivateKey)
	if !ok {
		t.Errorf("Unexpected key type %T, wanted *rsa.PrivateKey", k)
	}
}

func TestGenerateKeyType(t *testing.T) {
	k, _, err := GenerateKey(KeyType("ed25519"))
	if err != nil {
		t.Error(err)
	}
	_, ok := k.(*ed25519.PrivateKey)
	if !ok {
		t.Errorf("Unexpected key type %T, wanted *ed25519.PrivateKey", k)
	}
}

func TestGenerateKeySize(t *testing.T) {
	k, _, err := GenerateKey(KeySize(1024))
	if err != nil {
		t.Error(err)
	}
	_, ok := k.(*rsa.PrivateKey)
	if !ok {
		t.Errorf("Unexpected key type %T, wanted *rsa.PrivateKey", k)
	}
}
