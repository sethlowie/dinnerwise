package meal

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/sethlowie/dinnerwise/internal/db"
	"github.com/sethlowie/dinnerwise/internal/recipe"
)

// newTestDB returns a migrated DB with recipes seeded (so meal.recipe_id FKs
// resolve) and the meal tables created.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := recipe.Migrate(database); err != nil {
		t.Fatalf("recipe migrate: %v", err)
	}
	if err := recipe.SeedIfEmpty(database); err != nil {
		t.Fatalf("recipe seed: %v", err)
	}
	if err := Migrate(database); err != nil {
		t.Fatalf("meal migrate: %v", err)
	}
	return database
}

func TestMigrateIsIdempotentAndCreatesTables(t *testing.T) {
	database := newTestDB(t)
	if err := Migrate(database); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	for _, table := range []string{"meal", "meal_cook"} {
		var name string
		if err := database.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name); err != nil {
			t.Fatalf("table %q missing: %v", table, err)
		}
	}
}
