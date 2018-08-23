package store

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/user"
	"testing"
	"time"

	"github.com/nsheridan/cashier/testdata"
	"github.com/stretchr/testify/assert"

	"golang.org/x/crypto/ssh"
)

func TestParseCertificate(t *testing.T) {
	a := assert.New(t)
	now := uint64(time.Now().Unix())
	r, _ := rsa.GenerateKey(rand.Reader, 1024)
	pub, _ := ssh.NewPublicKey(r.Public())
	c := &ssh.Certificate{
		KeyId:           "id",
		ValidPrincipals: StringSlice{"principal"},
		ValidBefore:     now,
		CertType:        ssh.UserCert,
		Key:             pub,
	}
	s, _ := ssh.NewSignerFromKey(r)
	c.SignCert(rand.Reader, s)
	rec := MakeRecord(c)

	a.Equal(c.KeyId, rec.KeyID)
	a.Equal(c.ValidPrincipals, []string(rec.Principals))
	a.Equal(c.ValidBefore, uint64(rec.Expires.Unix()))
	a.Equal(c.ValidAfter, uint64(rec.CreatedAt.Unix()))
}

func testStore(t *testing.T, db CertStorer) {
	defer db.Close()

	r := &CertRecord{
		KeyID:      "a",
		Principals: []string{"b"},
		CreatedAt:  time.Now().UTC(),
		Expires:    time.Now().UTC().Add(-1 * time.Second),
		Raw:        "AAAAAA",
	}
	if err := db.SetRecord(r); err != nil {
		t.Error(err)
	}

	// includeExpired = false should return 0 results
	recs, err := db.List(false)
	if err != nil {
		t.Error(err)
	}
	if len(recs) > 0 {
		t.Errorf("Expected 0 results, got %d", len(recs))
	}
	// includeExpired = false should return 1 result
	recs, err = db.List(true)
	if err != nil {
		t.Error(err)
	}
	if recs[0].KeyID != r.KeyID {
		t.Error("key mismatch")
	}

	c, _, _, _, _ := ssh.ParseAuthorizedKey(testdata.Cert)
	cert := c.(*ssh.Certificate)
	cert.ValidBefore = uint64(time.Now().Add(1 * time.Hour).UTC().Unix())
	cert.ValidAfter = uint64(time.Now().Add(-5 * time.Minute).UTC().Unix())
	rec := MakeRecord(cert)
	if err := db.SetRecord(rec); err != nil {
		t.Error(err)
	}

	ret, err := db.Get("key")
	if err != nil {
		t.Error(err)
	}
	if ret.KeyID != cert.KeyId {
		t.Error("key mismatch")
	}
	if err := db.Revoke([]string{"key"}); err != nil {
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
	db := newMemoryStore()
	testStore(t, db)
}

func TestMySQLStore(t *testing.T) {
	if os.Getenv("MYSQL_TEST") == "" {
		t.Skip("No MYSQL_TEST environment variable")
	}
	u, _ := user.Current()
	sqlConfig := map[string]string{
		"type":     "mysql",
		"password": os.Getenv("MYSQL_TEST_PASS"),
		"address":  os.Getenv("MYSQL_TEST_HOST"),
	}
	if testUser, ok := os.LookupEnv("MYSQL_TEST_USER"); ok {
		sqlConfig["username"] = testUser
	} else {
		sqlConfig["username"] = u.Username
	}
	db, err := newSQLStore(sqlConfig)
	if err != nil {
		t.Error(err)
	}
	testStore(t, db)
}

func TestSQLiteStore(t *testing.T) {
	f, err := ioutil.TempFile("", "sqlite_test_db")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(f.Name())
	config := map[string]string{"type": "sqlite", "filename": f.Name()}
	db, err := newSQLStore(config)
	if err != nil {
		t.Error(err)
	}
	testStore(t, db)
}

func TestMarshalCert(t *testing.T) {
	a := assert.New(t)
	c := &CertRecord{
		KeyID:      "id",
		Principals: []string{"user"},
		CreatedAt:  time.Date(2017, time.April, 10, 13, 0, 0, 0, time.UTC),
		Expires:    time.Date(2017, time.April, 11, 10, 0, 0, 0, time.UTC),
		Raw:        "ABCDEF",
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Error(err)
	}
	want := `{"key_id":"id","principals":["user"],"revoked":false,"created_at":"2017-04-10 13:00:00 +0000","expires":"2017-04-11 10:00:00 +0000","message":""}`
	a.JSONEq(want, string(b))
}
