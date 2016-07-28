package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/go-sql-driver/mysql"
)

type mysqlDB struct {
	conn *sql.DB

	get     *sql.Stmt
	set     *sql.Stmt
	list    *sql.Stmt
	revoke  *sql.Stmt
	revoked *sql.Stmt
}

func parseMySQLConfig(config string) string {
	s := strings.Split(config, ":")
	if len(s) == 4 {
		s = append(s, "3306")
	}
	_, user, passwd, host, port := s[0], s[1], s[2], s[3], s[4]
	c := &mysql.Config{
		User:      user,
		Passwd:    passwd,
		Net:       "tcp",
		Addr:      fmt.Sprintf("%s:%s", host, port),
		DBName:    "certs",
		ParseTime: true,
	}
	return c.FormatDSN()
}

// NewMySQLStore returns a MySQL CertStorer.
func NewMySQLStore(config string) (CertStorer, error) {
	conn, err := sql.Open("mysql", parseMySQLConfig(config))
	if err != nil {
		return nil, fmt.Errorf("mysql: could not get a connection: %v", err)
	}
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("mysql: could not establish a good connection: %v", err)
	}

	db := &mysqlDB{
		conn: conn,
	}

	if db.set, err = conn.Prepare("INSERT INTO issued_certs (key_id, principals, created_at, expires_at, raw_key) VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE key_id = VALUES(key_id), principals = VALUES(principals), created_at = VALUES(created_at), expires_at = VALUES(expires_at), raw_key = VALUES(raw_key)"); err != nil {
		return nil, fmt.Errorf("mysql: prepare set: %v", err)
	}
	if db.get, err = conn.Prepare("SELECT * FROM issued_certs WHERE key_id = ?"); err != nil {
		return nil, fmt.Errorf("mysql: prepare get: %v", err)
	}
	if db.list, err = conn.Prepare("SELECT * FROM issued_certs"); err != nil {
		return nil, fmt.Errorf("mysql: prepare list: %v", err)
	}
	if db.revoke, err = conn.Prepare("UPDATE issued_certs SET revoked = TRUE WHERE key_id = ?"); err != nil {
		return nil, fmt.Errorf("mysql: prepare revoke: %v", err)
	}
	if db.revoked, err = conn.Prepare("SELECT * FROM issued_certs WHERE revoked = TRUE AND ? <= expires_at"); err != nil {
		return nil, fmt.Errorf("mysql: prepare revoked: %v", err)
	}
	return db, nil
}

// rowScanner is implemented by sql.Row and sql.Rows
type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanCert(s rowScanner) (*CertRecord, error) {
	var (
		keyID      sql.NullString
		principals sql.NullString
		createdAt  mysql.NullTime
		expires    mysql.NullTime
		revoked    sql.NullBool
		raw        sql.NullString
	)
	if err := s.Scan(&keyID, &principals, &createdAt, &expires, &revoked, &raw); err != nil {
		return nil, err
	}
	var p []string
	if err := json.Unmarshal([]byte(principals.String), &p); err != nil {
		return nil, err
	}
	return &CertRecord{
		KeyID:      keyID.String,
		Principals: p,
		CreatedAt:  createdAt.Time,
		Expires:    expires.Time,
		Revoked:    revoked.Bool,
		Raw:        raw.String,
	}, nil
}

func (db *mysqlDB) Get(id string) (*CertRecord, error) {
	return scanCert(db.get.QueryRow(id))
}

func (db *mysqlDB) SetCert(cert *ssh.Certificate) error {
	return db.SetRecord(parseCertificate(cert))
}

func (db *mysqlDB) SetRecord(rec *CertRecord) error {
	principals, err := json.Marshal(rec.Principals)
	if err != nil {
		return err
	}
	_, err = db.set.Exec(rec.KeyID, string(principals), rec.CreatedAt, rec.Expires, rec.Raw)
	return err
}

func (db *mysqlDB) List() ([]*CertRecord, error) {
	var recs []*CertRecord
	rows, _ := db.list.Query()
	defer rows.Close()
	for rows.Next() {
		cert, err := scanCert(rows)
		if err != nil {
			return nil, err
		}
		recs = append(recs, cert)
	}
	return recs, nil
}

func (db *mysqlDB) Revoke(id string) error {
	_, err := db.revoke.Exec(id)
	if err != nil {
		return err
	}
	return nil
}

func (db *mysqlDB) GetRevoked() ([]*CertRecord, error) {
	var recs []*CertRecord
	rows, _ := db.revoked.Query(time.Now().UTC())
	defer rows.Close()
	for rows.Next() {
		cert, err := scanCert(rows)
		if err != nil {
			return nil, err
		}
		recs = append(recs, cert)
	}
	return recs, nil
}

func (db *mysqlDB) Close() error {
	return db.conn.Close()
}
