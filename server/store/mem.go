package store

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type memoryStore struct {
	sync.Mutex
	certs map[string]*CertRecord
}

func (ms *memoryStore) Get(id string) (*CertRecord, error) {
	ms.Lock()
	defer ms.Unlock()
	r, ok := ms.certs[id]
	if !ok {
		return nil, fmt.Errorf("unknown cert %s", id)
	}
	return r, nil
}

func (ms *memoryStore) SetCert(cert *ssh.Certificate) error {
	return ms.SetRecord(parseCertificate(cert))
}

func (ms *memoryStore) SetRecord(record *CertRecord) error {
	ms.Lock()
	defer ms.Unlock()
	ms.certs[record.KeyID] = record
	return nil
}

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

func (ms *memoryStore) Revoke(id string) error {
	r, err := ms.Get(id)
	if err != nil {
		return err
	}
	r.Revoked = true
	ms.SetRecord(r)
	return nil
}

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

// NewMemoryStore returns an in-memory CertStorer.
func NewMemoryStore() CertStorer {
	return &memoryStore{
		certs: make(map[string]*CertRecord),
	}
}
