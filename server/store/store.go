package store

import (
	"encoding/json"
	"fmt"
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
		return newSQLStore(c)
	case "mem":
		return newMemoryStore(), nil
	}
	return nil, fmt.Errorf("unable to create store with driver %s", c["type"])
}

// CertStorer records issued certs in a persistent store for audit and
// revocation purposes.
type CertStorer interface {
	Get(id string) (*CertRecord, error)
	SetCert(cert *ssh.Certificate) error
	SetRecord(record *CertRecord) error
	List(includeExpired bool) ([]*CertRecord, error)
	Revoke(id []string) error
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

// MarshalJSON implements the json.Marshaler interface for the CreatedAt and
// Expires fields.
// The resulting string looks like "2017-04-11 10:00:00 +0000"
func (c *CertRecord) MarshalJSON() ([]byte, error) {
	type Alias CertRecord
	f := "2006-01-02 15:04:05 -0700"
	return json.Marshal(&struct {
		*Alias
		CreatedAt string `json:"created_at"`
		Expires   string `json:"expires"`
	}{
		Alias:     (*Alias)(c),
		CreatedAt: c.CreatedAt.Format(f),
		Expires:   c.Expires.Format(f),
	})
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
		Raw:        string(lib.GetPublicKey(cert)),
	}
}
