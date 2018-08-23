package store

import (
	"fmt"
	"log"
	"net"
	"path"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/gobuffalo/packr"
	multierror "github.com/hashicorp/go-multierror"
	"github.com/jmoiron/sqlx"
	"github.com/nsheridan/cashier/server/config"
	"github.com/pkg/errors"
	migrate "github.com/rubenv/sql-migrate"
)

var _ CertStorer = (*sqlStore)(nil)

// sqlStore is an sql-based CertStorer
type sqlStore struct {
	conn *sqlx.DB

	get         *sqlx.Stmt
	set         *sqlx.Stmt
	listAll     *sqlx.Stmt
	listCurrent *sqlx.Stmt
	revoked     *sqlx.Stmt
}

// newSQLStore returns a *sql.DB CertStorer.
func newSQLStore(c config.Database) (*sqlStore, error) {
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
		m := mysql.NewConfig()
		m.User = c["username"]
		m.Passwd = c["password"]
		m.Addr = address
		m.Net = "tcp"
		m.DBName = c["dbname"]
		if m.DBName == "" {
			m.DBName = "certs" // Legacy database name
		}
		m.ParseTime = true
		dsn = m.FormatDSN()
	case "sqlite":
		driver = "sqlite3"
		dsn = c["filename"]
	}

	conn, err := sqlx.Connect(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlStore: could not get a connection: %v", err)
	}
	if err := autoMigrate(driver, conn); err != nil {
		return nil, fmt.Errorf("sqlStore: could not update schema: %v", err)
	}

	db := &sqlStore{
		conn: conn,
	}

	if db.set, err = conn.Preparex("INSERT INTO issued_certs (key_id, principals, created_at, expires_at, raw_key, message) VALUES (?, ?, ?, ?, ?, ?)"); err != nil {
		return nil, fmt.Errorf("sqlStore: prepare set: %v", err)
	}
	if db.get, err = conn.Preparex("SELECT * FROM issued_certs WHERE key_id = ?"); err != nil {
		return nil, fmt.Errorf("sqlStore: prepare get: %v", err)
	}
	if db.listAll, err = conn.Preparex("SELECT * FROM issued_certs"); err != nil {
		return nil, fmt.Errorf("sqlStore: prepare listAll: %v", err)
	}
	if db.listCurrent, err = conn.Preparex("SELECT * FROM issued_certs WHERE expires_at >= ?"); err != nil {
		return nil, fmt.Errorf("sqlStore: prepare listCurrent: %v", err)
	}
	if db.revoked, err = conn.Preparex("SELECT * FROM issued_certs WHERE revoked = 1 AND ? <= expires_at"); err != nil {
		return nil, fmt.Errorf("sqlStore: prepare revoked: %v", err)
	}
	return db, nil
}

func autoMigrate(driver string, conn *sqlx.DB) error {
	log.Print("Executing any pending schema migrations")
	var err error
	migrate.SetTable("schema_migrations")
	srcs := &migrate.PackrMigrationSource{
		Box: packr.NewBox(path.Join("migrations", driver)),
	}
	n, err := migrate.Exec(conn.DB, driver, srcs, migrate.Up)
	if err != nil {
		err = multierror.Append(err)
		return err
	}
	log.Printf("Executed %d migrations", n)
	if err != nil {
		log.Fatalf("Errors were found running migrations: %v", err)
	}
	return nil
}

// Get a single *CertRecord
func (db *sqlStore) Get(id string) (*CertRecord, error) {
	if err := db.conn.Ping(); err != nil {
		return nil, errors.Wrap(err, "unable to connect to database")
	}
	r := &CertRecord{}
	return r, db.get.Get(r, id)
}

// SetRecord records a *CertRecord
func (db *sqlStore) SetRecord(rec *CertRecord) error {
	if err := db.conn.Ping(); err != nil {
		return errors.Wrap(err, "unable to connect to database")
	}
	_, err := db.set.Exec(rec.KeyID, rec.Principals, rec.CreatedAt, rec.Expires, rec.Raw, rec.Message)
	return err
}

// List returns all recorded certs.
// By default only active certs are returned.
func (db *sqlStore) List(includeExpired bool) ([]*CertRecord, error) {
	if err := db.conn.Ping(); err != nil {
		return nil, errors.Wrap(err, "unable to connect to database")
	}
	recs := []*CertRecord{}
	if includeExpired {
		if err := db.listAll.Select(&recs); err != nil {
			return nil, err
		}
	} else {
		if err := db.listCurrent.Select(&recs, time.Now()); err != nil {
			return nil, err
		}
	}
	return recs, nil
}

// Revoke an issued cert by id.
func (db *sqlStore) Revoke(ids []string) error {
	if err := db.conn.Ping(); err != nil {
		return errors.Wrap(err, "unable to connect to database")
	}
	q, args, err := sqlx.In("UPDATE issued_certs SET revoked = 1 WHERE key_id IN (?)", ids)
	if err != nil {
		return err
	}
	q = db.conn.Rebind(q)
	_, err = db.conn.Query(q, args...)
	return err
}

// GetRevoked returns all revoked certs
func (db *sqlStore) GetRevoked() ([]*CertRecord, error) {
	if err := db.conn.Ping(); err != nil {
		return nil, errors.Wrap(err, "unable to connect to database")
	}
	var recs []*CertRecord
	if err := db.revoked.Select(&recs, time.Now().UTC()); err != nil {
		return nil, err
	}
	return recs, nil
}

// Close the connection to the database
func (db *sqlStore) Close() error {
	return db.conn.Close()
}
