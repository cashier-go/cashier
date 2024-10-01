package lib

import (
	"reflect"
	"testing"

	"github.com/cashier-go/cashier/testdata"
	"golang.org/x/crypto/ssh"
)

func TestGetPublicKey(t *testing.T) {
	c, _, _, _, _ := ssh.ParseAuthorizedKey(testdata.Cert)
	if !reflect.DeepEqual(GetPublicKey(c.(*ssh.Certificate)), testdata.Cert) {
		t.Fail()
	}
}
