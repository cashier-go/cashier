package store

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

var _ CertStorer = (*MemoryStore)(nil)

// MemoryStore is an in-memory CertStorer
type MemoryStore struct {
	sync.Mutex
	certs map[string]*CertRecord
}

// Get a single *CertRecord
func (ms *MemoryStore) Get(id string) (*CertRecord, error) {
	ms.Lock()
	defer ms.Unlock()
	r, ok := ms.certs[id]
	if !ok {
		return nil, fmt.Errorf("unknown cert %s", id)
	}
	return r, nil
}

// SetCert parses a *ssh.Certificate and records it
func (ms *MemoryStore) SetCert(cert *ssh.Certificate) error {
	return ms.SetRecord(parseCertificate(cert))
}

// SetRecord records a *CertRecord
func (ms *MemoryStore) SetRecord(record *CertRecord) error {
	ms.Lock()
	defer ms.Unlock()
	ms.certs[record.KeyID] = record
	return nil
}

// List returns all recorded certs.
// By default only active certs are returned.
func (ms *MemoryStore) List(includeExpired bool) ([]*CertRecord, error) {
	var records []*CertRecord
	ms.Lock()
	defer ms.Unlock()

	for _, value := range ms.certs {
		if !includeExpired && value.Expires.Before(time.Now().UTC()) {
			continue
		}
		records = append(records, value)
	}
	return records, nil
}

// Revoke an issued cert by id.
func (ms *MemoryStore) Revoke(id string) error {
	r, err := ms.Get(id)
	if err != nil {
		return err
	}
	r.Revoked = true
	ms.SetRecord(r)
	return nil
}

// GetRevoked returns all revoked certs
func (ms *MemoryStore) GetRevoked() ([]*CertRecord, error) {
	var revoked []*CertRecord
	all, _ := ms.List(false)
	for _, r := range all {
		if r.Revoked {
			revoked = append(revoked, r)
		}
	}
	return revoked, nil
}

// Close the store. This will clear the contents.
func (ms *MemoryStore) Close() error {
	ms.Lock()
	defer ms.Unlock()
	ms.certs = nil
	return nil
}

func (ms *MemoryStore) clear() {
	for k := range ms.certs {
		delete(ms.certs, k)
	}
}

// NewMemoryStore returns an in-memory CertStorer.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		certs: make(map[string]*CertRecord),
	}
}
