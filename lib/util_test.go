package lib

import (
	"reflect"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/cashier-go/cashier/testdata"
)

func TestGetPublicKey(t *testing.T) {
	c, _, _, _, _ := ssh.ParseAuthorizedKey(testdata.Cert)
	if !reflect.DeepEqual(GetPublicKey(c.(*ssh.Certificate)), testdata.Cert) {
		t.Fail()
	}
}
