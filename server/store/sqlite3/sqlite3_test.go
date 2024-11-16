package sqlite3

import (
	"database/sql"
	"testing"
)

func TestSqlite3Compat(t *testing.T) {
	db, err := sql.Open(sqlite3Driver, ":memory:")
	if err != nil {
		t.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()
	_, err = db.Exec("CREATE TABLE testing (id INT)")
	if err != nil {
		t.Fatalf("Error creating table: %v", err)
	}
	testValue := 123
	_, err = db.Exec("INSERT INTO testing VALUES ($1)", testValue)
	if err != nil {
		t.Fatalf("Error inserting values: %v", err)
	}
	row := db.QueryRow("SELECT id FROM testing WHERE id = $1", testValue)
	if err != nil {
		t.Fatalf("Error execing query: %v", err)
	}
	var id int
	if err := row.Scan(&id); err != nil {
		t.Fatalf("Error scanning result: %v", err)
	}
	if id != 123 {
		t.Fatalf("Expected %d for fetched result, got %d", testValue, id)
	}
}
