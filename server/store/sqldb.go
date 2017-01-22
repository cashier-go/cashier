package store

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/nsheridan/cashier/server/config"
)

var _ CertStorer = (*SQLStore)(nil)

// SQLStore is an sql-based CertStorer
type SQLStore struct {
	conn *sqlx.DB

	get         *sqlx.Stmt
	set         *sqlx.Stmt
	listAll     *sqlx.Stmt
	listCurrent *sqlx.Stmt
	revoke      *sqlx.Stmt
	revoked     *sqlx.Stmt
}

// NewSQLStore returns a *sql.DB CertStorer.
func NewSQLStore(c config.Database) (*SQLStore, error) {
	var driver string
	var dsn string
	switch c["type"] {
	case "mysql":
		driver = "mysql"
		address := c["address"]
		_, _, err := net.SplitHostPort(address)
		if err != nil {
			address = address + ":3306"
		}
		m := &mysql.Config{
			User:      c["username"],
			Passwd:    c["password"],
			Net:       "tcp",
			Addr:      address,
			DBName:    "certs",
			ParseTime: true,
		}
		dsn = m.FormatDSN()
	case "sqlite":
		driver = "sqlite3"
		dsn = c["filename"]
	}
	conn, err := sqlx.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("SQLStore: could not get a connection: %v", err)
	}
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("SQLStore: could not establish a good connection: %v", err)
	}

	db := &SQLStore{
		conn: conn,
	}

	if db.set, err = conn.Preparex("INSERT INTO issued_certs (key_id, principals, created_at, expires_at, raw_key) VALUES (?, ?, ?, ?, ?)"); err != nil {
		return nil, fmt.Errorf("SQLStore: prepare set: %v", err)
	}
	if db.get, err = conn.Preparex("SELECT * FROM issued_certs WHERE key_id = ?"); err != nil {
		return nil, fmt.Errorf("SQLStore: prepare get: %v", err)
	}
	if db.listAll, err = conn.Preparex("SELECT * FROM issued_certs"); err != nil {
		return nil, fmt.Errorf("SQLStore: prepare listAll: %v", err)
	}
	if db.listCurrent, err = conn.Preparex("SELECT * FROM issued_certs WHERE ? <= expires_at"); err != nil {
		return nil, fmt.Errorf("SQLStore: prepare listCurrent: %v", err)
	}
	if db.revoke, err = conn.Preparex("UPDATE issued_certs SET revoked = 1 WHERE key_id = ?"); err != nil {
		return nil, fmt.Errorf("SQLStore: prepare revoke: %v", err)
	}
	if db.revoked, err = conn.Preparex("SELECT * FROM issued_certs WHERE revoked = 1 AND ? <= expires_at"); err != nil {
		return nil, fmt.Errorf("SQLStore: prepare revoked: %v", err)
	}
	return db, nil
}

// rowScanner is implemented by sql.Row and sql.Rows
type rowScanner interface {
	Scan(dest ...interface{}) error
}

// Get a single *CertRecord
func (db *SQLStore) Get(id string) (*CertRecord, error) {
	if err := db.conn.Ping(); err != nil {
		return nil, err
	}
	r := &CertRecord{}
	return r, db.get.Get(r, id)
}

// SetCert parses a *ssh.Certificate and records it
func (db *SQLStore) SetCert(cert *ssh.Certificate) error {
	return db.SetRecord(parseCertificate(cert))
}

// SetRecord records a *CertRecord
func (db *SQLStore) SetRecord(rec *CertRecord) error {
	if err := db.conn.Ping(); err != nil {
		return err
	}
	_, err := db.set.Exec(rec.KeyID, rec.Principals, rec.CreatedAt, rec.Expires, rec.Raw)
	return err
}

// List returns all recorded certs.
// By default only active certs are returned.
func (db *SQLStore) List(includeExpired bool) ([]*CertRecord, error) {
	if err := db.conn.Ping(); err != nil {
		return nil, err
	}
	recs := []*CertRecord{}
	if err := db.listAll.Select(&recs); err != nil {
		return nil, err
	}
	return recs, nil
}

// Revoke an issued cert by id.
func (db *SQLStore) Revoke(id string) error {
	if err := db.conn.Ping(); err != nil {
		return err
	}
	if _, err := db.revoke.Exec(id); err != nil {
		return err
	}
	return nil
}

// GetRevoked returns all revoked certs
func (db *SQLStore) GetRevoked() ([]*CertRecord, error) {
	if err := db.conn.Ping(); err != nil {
		return nil, err
	}
	var recs []*CertRecord
	if err := db.revoked.Select(&recs, time.Now().UTC()); err != nil {
		return nil, err
	}
	return recs, nil
}

// Close the connection to the database
func (db *SQLStore) Close() error {
	return db.conn.Close()
}
