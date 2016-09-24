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

func collection(session *mgo.Session) *mgo.Collection {
	return session.DB(certsDB).C(issuedTable)
}

// NewMongoStore returns a MongoDB CertStorer.
func NewMongoStore(config string) (CertStorer, error) {
	session, err := mgo.DialWithInfo(parseMongoConfig(config))
	if err != nil {
		return nil, err
	}
	return &mongoDB{
		session: session,
	}, nil
}

type mongoDB struct {
	session *mgo.Session
}

func (m *mongoDB) Get(id string) (*CertRecord, error) {
	s := m.session.Copy()
	defer s.Close()
	if err := s.Ping(); err != nil {
		return nil, err
	}
	c := &CertRecord{}
	err := collection(s).Find(bson.M{"keyid": id}).One(c)
	return c, err
}

func (m *mongoDB) SetCert(cert *ssh.Certificate) error {
	r := parseCertificate(cert)
	return m.SetRecord(r)
}

func (m *mongoDB) SetRecord(record *CertRecord) error {
	s := m.session.Copy()
	defer s.Close()
	if err := s.Ping(); err != nil {
		return err
	}
	return collection(s).Insert(record)
}

func (m *mongoDB) List(includeExpired bool) ([]*CertRecord, error) {
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

func (m *mongoDB) Revoke(id string) error {
	s := m.session.Copy()
	defer s.Close()
	if err := s.Ping(); err != nil {
		return err
	}
	c := collection(s)
	return c.Update(bson.M{"keyid": id}, bson.M{"$set": bson.M{"revoked": true}})
}

func (m *mongoDB) GetRevoked() ([]*CertRecord, error) {
	s := m.session.Copy()
	defer s.Close()
	if err := s.Ping(); err != nil {
		return nil, err
	}
	var result []*CertRecord
	err := collection(s).Find(bson.M{"expires": bson.M{"$gte": time.Now().UTC()}, "revoked": true}).All(&result)
	return result, err
}

func (m *mongoDB) Close() error {
	m.session.Close()
	return nil
}
