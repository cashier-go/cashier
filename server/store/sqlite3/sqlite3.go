package sqlite3

import (
	"database/sql"

	"modernc.org/sqlite"
)

const sqlite3Driver = "sqlite3"

func init() {
	sql.Register(sqlite3Driver, &sqlite.Driver{})
}
