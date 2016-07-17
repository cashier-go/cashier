package store

import (
	"time"

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
	CreatedAt  time.Time
	Expires    time.Time
	Revoked    bool
	Raw        string
}

func parseTime(t uint64) time.Time {
	return time.Unix(int64(t), 0)
}

func parseCertificate(cert *ssh.Certificate) *CertRecord {
	return &CertRecord{
		KeyID:      cert.KeyId,
		Principals: cert.ValidPrincipals,
		CreatedAt:  parseTime(cert.ValidAfter),
		Expires:    parseTime(cert.ValidBefore),
		Raw:        certutil.GetPublicKey(cert),
	}
}
