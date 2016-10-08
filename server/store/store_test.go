package store

import (
	"crypto/rand"
	"crypto/rsa"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/nsheridan/cashier/testdata"
	"github.com/stretchr/testify/assert"

	"golang.org/x/crypto/ssh"
)

var (
	dbConfig = map[string]string{"username": "user", "password": "passwd", "address": "localhost"}
)

func TestParseCertificate(t *testing.T) {
	t.Parallel()
	a := assert.New(t)
	now := uint64(time.Now().Unix())
	r, _ := rsa.GenerateKey(rand.Reader, 1024)
	pub, _ := ssh.NewPublicKey(r.Public())
	c := &ssh.Certificate{
		KeyId:           "id",
		ValidPrincipals: []string{"principal"},
		ValidBefore:     now,
		CertType:        ssh.UserCert,
		Key:             pub,
	}
	s, _ := ssh.NewSignerFromKey(r)
	c.SignCert(rand.Reader, s)
	rec := parseCertificate(c)

	a.Equal(c.KeyId, rec.KeyID)
	a.Equal(c.ValidPrincipals, rec.Principals)
	a.Equal(c.ValidBefore, uint64(rec.Expires.Unix()))
	a.Equal(c.ValidAfter, uint64(rec.CreatedAt.Unix()))
}

func testStore(t *testing.T, db CertStorer) {
	defer db.Close()

	r := &CertRecord{
		KeyID:   "a",
		Expires: time.Now().UTC().Add(1 * time.Minute),
	}
	if err := db.SetRecord(r); err != nil {
		t.Error(err)
	}
	if _, err := db.List(true); err != nil {
		t.Error(err)
	}

	c, _, _, _, _ := ssh.ParseAuthorizedKey(testdata.Cert)
	cert := c.(*ssh.Certificate)
	cert.ValidBefore = uint64(time.Now().Add(1 * time.Hour).UTC().Unix())
	cert.ValidAfter = uint64(time.Now().Add(-5 * time.Minute).UTC().Unix())
	if err := db.SetCert(cert); err != nil {
		t.Error(err)
	}

	if _, err := db.Get("key"); err != nil {
		t.Error(err)
	}
	if err := db.Revoke("key"); err != nil {
		t.Error(err)
	}

	revoked, err := db.GetRevoked()
	if err != nil {
		t.Error(err)
	}
	for _, k := range revoked {
		if k.KeyID != "key" {
			t.Errorf("Unexpected key: %s", k.KeyID)
		}
	}
}

func TestMemoryStore(t *testing.T) {
	t.Parallel()
	db := NewMemoryStore()
	testStore(t, db)
}

func TestMySQLStore(t *testing.T) {
	t.Parallel()
	if os.Getenv("MYSQL_TEST") == "" {
		t.Skip("No MYSQL_TEST environment variable")
	}
	dbConfig["type"] = "mysql"
	db, err := NewSQLStore(dbConfig)
	if err != nil {
		t.Error(err)
	}
	testStore(t, db)
}

func TestMongoStore(t *testing.T) {
	t.Parallel()
	if os.Getenv("MONGO_TEST") == "" {
		t.Skip("No MONGO_TEST environment variable")
	}
	dbConfig["type"] = "mongo"
	db, err := NewMongoStore(dbConfig)
	if err != nil {
		t.Error(err)
	}
	testStore(t, db)
}

func TestSQLiteStore(t *testing.T) {
	t.Parallel()
	f, err := ioutil.TempFile("", "sqlite_test_db")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(f.Name())
	// This is so jank.
	args := []string{"run", "../../cmd/dbinit/dbinit.go", "-db_type", "sqlite", "-db_path", f.Name()}
	if err := exec.Command("go", args...).Run(); err != nil {
		t.Error(err)
	}
	config := map[string]string{"type": "sqlite", "filename": f.Name()}
	db, err := NewSQLStore(config)
	if err != nil {
		t.Error(err)
	}
	testStore(t, db)
}
