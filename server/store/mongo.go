package store

import (
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	certsDB     = "certs"
	issuedTable = "issued_certs"
)

type mongoDB struct {
	collection *mgo.Collection
	session    *mgo.Session
}

func parseMongoConfig(config string) *mgo.DialInfo {
	s := strings.SplitN(config, ":", 4)
	_, user, passwd, hosts := s[0], s[1], s[2], s[3]
	d := &mgo.DialInfo{
		Addrs:    strings.Split(hosts, ","),
		Username: user,
		Password: passwd,
		Database: certsDB,
		Timeout:  time.Second * 5,
	}
	return d
}

// NewMongoStore returns a MongoDB CertStorer.
func NewMongoStore(config string) (CertStorer, error) {
	session, err := mgo.DialWithInfo(parseMongoConfig(config))
	if err != nil {
		return nil, err
	}
	c := session.DB(certsDB).C(issuedTable)
	return &mongoDB{
		collection: c,
		session:    session,
	}, nil
}

func (m *mongoDB) Get(id string) (*CertRecord, error) {
	if err := m.session.Ping(); err != nil {
		return nil, err
	}
	c := &CertRecord{}
	err := m.collection.Find(bson.M{"keyid": id}).One(c)
	return c, err
}

func (m *mongoDB) SetCert(cert *ssh.Certificate) error {
	r := parseCertificate(cert)
	return m.SetRecord(r)
}

func (m *mongoDB) SetRecord(record *CertRecord) error {
	if err := m.session.Ping(); err != nil {
		return err
	}
	return m.collection.Insert(record)
}

func (m *mongoDB) List() ([]*CertRecord, error) {
	if err := m.session.Ping(); err != nil {
		return nil, err
	}
	var result []*CertRecord
	err := m.collection.Find(bson.M{"expires": bson.M{"$gte": time.Now().UTC()}}).All(&result)
	return result, err
}

func (m *mongoDB) Revoke(id string) error {
	if err := m.session.Ping(); err != nil {
		return err
	}
	return m.collection.Update(bson.M{"keyid": id}, bson.M{"$set": bson.M{"revoked": true}})
}

func (m *mongoDB) GetRevoked() ([]*CertRecord, error) {
	if err := m.session.Ping(); err != nil {
		return nil, err
	}
	var result []*CertRecord
	err := m.collection.Find(bson.M{"expires": bson.M{"$gte": time.Now().UTC()}, "revoked": true}).All(&result)
	return result, err
}

func (m *mongoDB) Close() error {
	m.session.Close()
	return nil
}
