package migrations

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	migrate "github.com/rubenv/sql-migrate"
	"github.com/stretchr/testify/assert"
)

func TestSQLiteMigrations(t *testing.T) {
	subdir := "sqlite3"
	db, err := sql.Open(subdir, ":memory:")
	assert.NoError(t, err)
	defer db.Close()
	runMigrations(t, db, subdir)
}

func TestMySQLMigrations(t *testing.T) {
	if os.Getenv("MYSQL_TEST") == "" {
		t.Skip("No MYSQL_TEST environment variable")
	}
	subdir := "mysql"
	dsn := mysql.NewConfig()
	dsn.Net = "tcp"
	dsn.ParseTime = true
	dsn.Addr = os.Getenv("MYSQL_TEST_HOST")
	dsn.Passwd = os.Getenv("MYSQL_TEST_PASS")
	u, _ := user.Current()
	if testUser, ok := os.LookupEnv("MYSQL_TEST_USER"); ok {
		dsn.User = testUser
	} else {
		dsn.User = u.Username
	}
	db, err := sql.Open(subdir, dsn.FormatDSN())
	assert.NoError(t, err)

	rnd := make([]byte, 4)
	rand.Read(rnd)
	suffix := fmt.Sprintf("_%x", string(rnd))
	_, err = db.Exec("CREATE DATABASE migrations_test" + suffix)
	assert.NoError(t, err)
	_, err = db.Exec("USE migrations_test" + suffix)
	assert.NoError(t, err)
	runMigrations(t, db, subdir)
	db.Exec("DROP DATABASE IF EXISTS migrations_test" + suffix)
	db.Close()
}

func runMigrations(t *testing.T, db *sql.DB, directory string) {
	m := &migrate.FileMigrationSource{
		Dir: directory,
	}
	files, err := filepath.Glob(path.Join(directory, "*.sql"))
	// Verify that there is at least one migration to run
	assert.NotEmpty(t, files)
	assert.NoError(t, err)
	// Verify that migrating up works
	n, err := migrate.Exec(db, directory, m, migrate.Up)
	assert.Len(t, files, n)
	assert.NoError(t, err)
	// Verify that a subsequent run has no migrations
	n, err = migrate.Exec(db, directory, m, migrate.Up)
	assert.Equal(t, 0, n)
	assert.NoError(t, err)
	// Verify that reversing migrations works
	n, err = migrate.Exec(db, directory, m, migrate.Down)
	assert.Len(t, files, n)
}

// Test that all migration directories contain the same set of migrations files.
func TestMigationDirectoryContents(t *testing.T) {
	names := map[string][]string{}
	contents, err := ioutil.ReadDir(".")
	assert.NoError(t, err)
	for _, i := range contents {
		if i.IsDir() {
			dir := path.Join(i.Name(), "*.sql")
			files, _ := filepath.Glob(dir)
			trimmed := []string{}
			for _, f := range files {
				trimmed = append(trimmed, filepath.Base(f))
			}
			names[i.Name()] = trimmed
		}
	}
	// Use one entry from the `names` map as a reference for all the others.
	first := names[reflect.ValueOf(names).MapKeys()[0].String()]
	for _, v := range names {
		assert.EqualValues(t, first, v)
	}
}