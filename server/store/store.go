package store

import (
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/nsheridan/cashier/lib"
	"github.com/nsheridan/cashier/server/config"
	"github.com/nsheridan/cashier/server/store/types"
)

// New returns a new configured database.
func New(c config.Database) (CertStorer, error) {
	switch c["type"] {
	case "mysql", "sqlite":
		return NewSQLStore(c)
	case "mem":
		return NewMemoryStore(), nil
	}
	return NewMemoryStore(), nil
}

// CertStorer records issued certs in a persistent store for audit and
// revocation purposes.
type CertStorer interface {
	Get(id string) (*CertRecord, error)
	SetCert(cert *ssh.Certificate) error
	SetRecord(record *CertRecord) error
	List(includeExpired bool) ([]*CertRecord, error)
	Revoke(id string) error
	GetRevoked() ([]*CertRecord, error)
	Close() error
}

// A CertRecord is a representation of a ssh certificate used by a CertStorer.
type CertRecord struct {
	KeyID      string            `json:"key_id" db:"key_id"`
	Principals types.StringSlice `json:"principals" db:"principals"`
	CreatedAt  time.Time         `json:"created_at" db:"created_at"`
	Expires    time.Time         `json:"expires" db:"expires_at"`
	Revoked    bool              `json:"revoked" db:"revoked"`
	Raw        string            `json:"-" db:"raw_key"`
}

func parseTime(t uint64) time.Time {
	return time.Unix(int64(t), 0)
}

func parseCertificate(cert *ssh.Certificate) *CertRecord {
	return &CertRecord{
		KeyID:      cert.KeyId,
		Principals: types.StringSlice(cert.ValidPrincipals),
		CreatedAt:  parseTime(cert.ValidAfter),
		Expires:    parseTime(cert.ValidBefore),
		Raw:        lib.GetPublicKey(cert),
	}
}
