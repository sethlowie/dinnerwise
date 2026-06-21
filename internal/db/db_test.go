package db_test

import (
	"path/filepath"
	"testing"

	"github.com/sethlowie/dinnerwise/internal/db"
)

func TestOpenEnablesForeignKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()

	var fk int
	if err := database.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("read pragma: %v", err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys = %d, want 1", fk)
	}
}

func TestApplySchemaIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()

	const schema = `CREATE TABLE IF NOT EXISTS t (id TEXT PRIMARY KEY);`
	if err := db.ApplySchema(database, schema); err != nil {
		t.Fatalf("first ApplySchema: %v", err)
	}
	if err := db.ApplySchema(database, schema); err != nil {
		t.Fatalf("second ApplySchema: %v", err)
	}
}
