package client

import (
	"reflect"
	"testing"
)

func TestGenerateKeys(t *testing.T) {
	var tests = []struct {
		key  string
		size int
		want string
	}{
		{"ecdsa", 256, "*ecdsa.PrivateKey"},
		{"rsa", 1024, "*rsa.PrivateKey"},
		{"ed25519", 256, "*ed25519.PrivateKey"},
	}

	for _, tst := range tests {
		k, _, err := GenerateKey(tst.key, tst.size)
		if err != nil {
			t.Error(err)
		}
		if reflect.TypeOf(k).String() != tst.want {
			t.Errorf("Wrong key type returned. Got %s, wanted %s", reflect.TypeOf(k).String(), tst.want)
		}
	}
}
