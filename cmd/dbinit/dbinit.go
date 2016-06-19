package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/go-sql-driver/mysql"
)

var (
	host        = flag.String("host", "localhost", "host[:port]")
	adminUser   = flag.String("admin_user", "root", "Admin user")
	adminPasswd = flag.String("admin_password", "", "Admin password")
	dbUser      = flag.String("db_user", "root", "Database user")
	dbPasswd    = flag.String("db_password", "", "Admin password")
)

func main() {
	flag.Parse()
	var createTableStmt = []string{
		`CREATE DATABASE IF NOT EXISTS certs DEFAULT CHARACTER SET = 'utf8' DEFAULT COLLATE 'utf8_general_ci';`,
		`USE certs;`,
		`CREATE TABLE IF NOT EXISTS issued_certs (
			key_id VARCHAR(255) NOT NULL,
			principals VARCHAR(255) NULL,
			created_at INT(11) NULL,
			expires_at INT(11) NULL,
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
