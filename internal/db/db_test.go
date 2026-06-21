package db_test

import (
	"os"
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

func TestOpenHandlesPathWithSpecialChars(t *testing.T) {
	// A directory whose name contains a space and a "?" would corrupt a naive
	// DSN. Open must escape the path so the DB opens at the real location.
	dir := filepath.Join(t.TempDir(), "weird dir? name")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "test.db")

	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer database.Close()

	if err := db.ApplySchema(database, `CREATE TABLE t (id TEXT PRIMARY KEY);`); err != nil {
		t.Fatalf("ApplySchema: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO t (id) VALUES ('x')`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// The file must exist at the real (unescaped) path.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("db file not at expected path: %v", err)
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
