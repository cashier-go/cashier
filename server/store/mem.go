package store

import (
	"fmt"
	"sync"
	"time"
)

var _ CertStorer = (*memoryStore)(nil)

// memoryStore is an in-memory CertStorer
type memoryStore struct {
	sync.Mutex
	certs map[string]*CertRecord
}

// Get a single *CertRecord
func (ms *memoryStore) Get(id string) (*CertRecord, error) {
	ms.Lock()
	defer ms.Unlock()
	r, ok := ms.certs[id]
	if !ok {
		return nil, fmt.Errorf("unknown cert %s", id)
	}
	return r, nil
}

// SetRecord records a *CertRecord
func (ms *memoryStore) SetRecord(record *CertRecord) error {
	ms.Lock()
	defer ms.Unlock()
	ms.certs[record.KeyID] = record
	return nil
}

// List returns all recorded certs.
// By default only active certs are returned.
func (ms *memoryStore) List(includeExpired bool) ([]*CertRecord, error) {
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
func (ms *memoryStore) Revoke(ids []string) error {
	ms.Lock()
	defer ms.Unlock()
	for _, id := range ids {
		ms.certs[id].Revoked = true
	}
	return nil
}

// GetRevoked returns all revoked certs
func (ms *memoryStore) GetRevoked() ([]*CertRecord, error) {
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
func (ms *memoryStore) Close() error {
	ms.Lock()
	defer ms.Unlock()
	ms.certs = nil
	return nil
}

func (ms *memoryStore) clear() {
	for k := range ms.certs {
		delete(ms.certs, k)
	}
}

// newMemoryStore returns an in-memory CertStorer.
func newMemoryStore() *memoryStore {
	return &memoryStore{
		certs: make(map[string]*CertRecord),
	}
}
