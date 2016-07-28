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
	conn *mgo.Collection
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

func NewMongoStore(config string) (CertStorer, error) {
	session, err := mgo.DialWithInfo(parseMongoConfig(config))
	if err != nil {
		return nil, err
	}
	c := session.DB(certsDB).C(issuedTable)
	return &mongoDB{
		conn: c,
	}, nil
}

func (m *mongoDB) Get(id string) (*CertRecord, error) {
	c := &CertRecord{}
	err := m.conn.Find(bson.M{"keyid": id}).One(c)
	return c, err
}

func (m *mongoDB) SetCert(cert *ssh.Certificate) error {
	r := parseCertificate(cert)
	return m.SetRecord(r)
}

func (m *mongoDB) SetRecord(record *CertRecord) error {
	return m.conn.Insert(record)
}

func (m *mongoDB) List() ([]*CertRecord, error) {
	var result []*CertRecord
	m.conn.Find(nil).All(&result)
	return result, nil
}

func (m *mongoDB) Revoke(id string) error {
	return m.conn.Update(bson.M{"keyid": id}, bson.M{"$set": bson.M{"revoked": true}})
}

func (m *mongoDB) GetRevoked() ([]*CertRecord, error) {
	var result []*CertRecord
	err := m.conn.Find(bson.M{"expires": bson.M{"$gte": time.Now().UTC()}, "revoked": true}).All(&result)
	return result, err
}

func (m *mongoDB) Close() error {
	m.conn.Database.Session.Close()
	return nil
}
