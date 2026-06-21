package recipe

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/sethlowie/dinnerwise/internal/db"
)

// newTestDB returns a migrated, empty database in a temp dir.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return database
}

func TestMigrateIsIdempotentAndCreatesTables(t *testing.T) {
	database := newTestDB(t)

	// Running Migrate again must not error.
	if err := Migrate(database); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	for _, table := range []string{"recipe", "ingredient", "recipe_ingredient"} {
		var name string
		err := database.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name)
		if err != nil {
			t.Fatalf("table %q missing: %v", table, err)
		}
	}
}
