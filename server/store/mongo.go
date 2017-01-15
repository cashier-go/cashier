package store

import (
	"strings"
	"time"

	"github.com/nsheridan/cashier/server/config"

	"golang.org/x/crypto/ssh"

	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	certsDB     = "certs"
	issuedTable = "issued_certs"
)

func collection(session *mgo.Session) *mgo.Collection {
	return session.DB(certsDB).C(issuedTable)
}

// NewMongoStore returns a MongoDB CertStorer.
func NewMongoStore(c config.Database) (*MongoStore, error) {
	m := &mgo.DialInfo{
		Addrs:    strings.Split(c["address"], ","),
		Username: c["username"],
		Password: c["password"],
		Database: certsDB,
		Timeout:  time.Second * 5,
	}
	session, err := mgo.DialWithInfo(m)
	if err != nil {
		return nil, err
	}
	return &MongoStore{
		session: session,
	}, nil
}

var _ CertStorer = (*MongoStore)(nil)

// MongoStore is a MongoDB-based CertStorer
type MongoStore struct {
	session *mgo.Session
}

// Get a single *CertRecord
func (m *MongoStore) Get(id string) (*CertRecord, error) {
	s := m.session.Copy()
	defer s.Close()
	if err := s.Ping(); err != nil {
		return nil, err
	}
	c := &CertRecord{}
	err := collection(s).Find(bson.M{"keyid": id}).One(c)
	return c, err
}

// SetCert parses a *ssh.Certificate and records it
func (m *MongoStore) SetCert(cert *ssh.Certificate) error {
	r := parseCertificate(cert)
	return m.SetRecord(r)
}

// SetRecord records a *CertRecord
func (m *MongoStore) SetRecord(record *CertRecord) error {
	s := m.session.Copy()
	defer s.Close()
	if err := s.Ping(); err != nil {
		return err
	}
	return collection(s).Insert(record)
}

// List returns all recorded certs.
// By default only active certs are returned.
func (m *MongoStore) List(includeExpired bool) ([]*CertRecord, error) {
	s := m.session.Copy()
	defer s.Close()
	if err := s.Ping(); err != nil {
		return nil, err
	}
	var result []*CertRecord
	var err error
	c := collection(s)
	if includeExpired {
		err = c.Find(nil).All(&result)
	} else {
		err = c.Find(bson.M{"expires": bson.M{"$gte": time.Now().UTC()}}).All(&result)
	}
	return result, err
}

// Revoke an issued cert by id.
func (m *MongoStore) Revoke(id string) error {
	s := m.session.Copy()
	defer s.Close()
	if err := s.Ping(); err != nil {
		return err
	}
	c := collection(s)
	return c.Update(bson.M{"keyid": id}, bson.M{"$set": bson.M{"revoked": true}})
}

// GetRevoked returns all revoked certs
func (m *MongoStore) GetRevoked() ([]*CertRecord, error) {
	s := m.session.Copy()
	defer s.Close()
	if err := s.Ping(); err != nil {
		return nil, err
	}
	var result []*CertRecord
	err := collection(s).Find(bson.M{"expires": bson.M{"$gte": time.Now().UTC()}, "revoked": true}).All(&result)
	return result, err
}

// Close the connection to the database
func (m *MongoStore) Close() error {
	m.session.Close()
	return nil
}
