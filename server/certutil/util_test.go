package certutil

import (
	"testing"

	"github.com/nsheridan/cashier/testdata"
	"golang.org/x/crypto/ssh"
)

func TestGetPublicKey(t *testing.T) {
	t.Parallel()
	c, _, _, _, _ := ssh.ParseAuthorizedKey(testdata.Cert)
	if GetPublicKey(c.(*ssh.Certificate)) != string(testdata.Cert) {
		t.Fail()
	}
}
