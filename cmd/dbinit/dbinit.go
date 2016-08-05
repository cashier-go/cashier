package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"

	mgo "gopkg.in/mgo.v2"

	"github.com/go-sql-driver/mysql"
)

var (
	host        = flag.String("host", "localhost", "host[:port]")
	adminUser   = flag.String("admin_user", "root", "Admin user")
	adminPasswd = flag.String("admin_password", "", "Admin password")
	dbUser      = flag.String("db_user", "user", "Database user")
	dbPasswd    = flag.String("db_password", "passwd", "Admin password")
	dbType      = flag.String("db_type", "mysql", "Database engine (\"mysql\" or \"mongo\")")
	authDB      = flag.String("authdb", "admin", "Admin database (mongo)")

	certsDB     = "certs"
	issuedTable = "issued_certs"
)

func initMySQL() {
	var createTableStmt = []string{
		`CREATE DATABASE IF NOT EXISTS ` + certsDB + ` DEFAULT CHARACTER SET = 'utf8' DEFAULT COLLATE 'utf8_general_ci';`,
		`USE ` + certsDB + `;`,
		`CREATE TABLE IF NOT EXISTS ` + issuedTable + ` (
			key_id VARCHAR(255) NOT NULL,
			principals VARCHAR(255) NULL,
			created_at DATETIME NULL,
			expires_at DATETIME NULL,
			revoked BOOLEAN DEFAULT NULL,
			raw_key TEXT NULL,
			PRIMARY KEY (key_id)
		);`,
		`GRANT ALL PRIVILEGES ON certs.* TO '` + *dbUser + `'@'%' IDENTIFIED BY '` + *dbPasswd + `';`,
	}

	if len(strings.Split(*host, ":")) != 2 {
		*host = fmt.Sprintf("%s:3306", *host)
	}
	conn := &mysql.Config{
		User:   *adminUser,
		Passwd: *adminPasswd,
		Net:    "tcp",
		Addr:   *host,
	}
	db, err := sql.Open("mysql", conn.FormatDSN())
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("Unable to connect to database.")
	}
	for _, stmt := range createTableStmt {
		_, err := db.Exec(stmt)
		if err != nil {
			log.Fatalf("Error running setup: %v", err)
		}
	}
}

func initMongo() {
	di := &mgo.DialInfo{
		Addrs:    strings.Split(*host, ","),
		Username: *adminUser,
		Password: *adminPasswd,
		Database: *authDB,
	}
	session, err := mgo.DialWithInfo(di)
	if err != nil {
		log.Fatalln(err)
	}
	defer session.Close()
	d := session.DB(certsDB)
	if err := d.UpsertUser(&mgo.User{
		Username: *dbUser,
		Password: *dbPasswd,
		Roles:    []mgo.Role{mgo.RoleReadWrite},
	}); err != nil {
		log.Fatalln(err)
	}
	c := d.C(issuedTable)
	i := mgo.Index{
		Key:    []string{"keyid"},
		Unique: true,
	}
	if err != c.EnsureIndex(i) {
		log.Fatalln(err)
	}
}

func main() {
	flag.Parse()
	switch *dbType {
	case "mysql":
		initMySQL()
	case "mongo":
		initMongo()
	default:
		log.Fatalf("Invalid database type")
	}
}
