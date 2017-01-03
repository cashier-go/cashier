package client

import (
	"crypto/rsa"
	"reflect"
	"testing"
)

func TestGenerateKeys(t *testing.T) {
	var tests = []struct {
		opts KeyOptions
		want string
	}{
		{KeyOptions{"ecdsa", 256}, "*ecdsa.PrivateKey"},
		{KeyOptions{Type: "ecdsa"}, "*ecdsa.PrivateKey"},
		{KeyOptions{"rsa", 1024}, "*rsa.PrivateKey"},
		{KeyOptions{Type: "ed25519"}, "*ed25519.PrivateKey"},
	}

	for _, tst := range tests {
		k, _, err := GenerateKey(tst.opts)
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
