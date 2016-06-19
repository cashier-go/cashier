package store

import (
	"golang.org/x/crypto/ssh"

	"github.com/nsheridan/cashier/server/certutil"
)

// CertStorer records issued certs in a persistent store for audit and
// revocation purposes.
type CertStorer interface {
	Get(id string) (*CertRecord, error)
	SetCert(cert *ssh.Certificate) error
	SetRecord(record *CertRecord) error
	List() ([]*CertRecord, error)
	Revoke(id string) error
	GetRevoked() ([]*CertRecord, error)
	Close() error
}

// A CertRecord is a representation of a ssh certificate used by a CertStorer.
type CertRecord struct {
	KeyID      string
	Principals []string
	CreatedAt  uint64
	Expires    uint64
	Revoked    bool
	Raw        string
}

func parseCertificate(cert *ssh.Certificate) *CertRecord {
	return &CertRecord{
		KeyID:      cert.KeyId,
		Principals: cert.ValidPrincipals,
		CreatedAt:  cert.ValidAfter,
		Expires:    cert.ValidBefore,
		Raw:        certutil.GetPublicKey(cert),
	}
}
