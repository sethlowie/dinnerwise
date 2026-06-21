package meal

import (
	"context"
	"database/sql"
	"errors"
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

// insertMeal writes a meal and its cook rows directly (no seeding).
func insertMeal(t *testing.T, database *sql.DB, m Meal, cooks []Cook) {
	t.Helper()
	var rid any
	if m.RecipeID != "" {
		rid = m.RecipeID
	}
	if _, err := database.Exec(
		`INSERT INTO meal (id, name, cuisine, rating, recipe_id) VALUES (?, ?, ?, ?, ?)`,
		m.ID, m.Name, m.Cuisine, m.Rating, rid,
	); err != nil {
		t.Fatalf("insert meal: %v", err)
	}
	for _, c := range cooks {
		if _, err := database.Exec(
			`INSERT INTO meal_cook (meal_id, cooked_on, note) VALUES (?, ?, ?)`,
			m.ID, c.CookedOn, c.Note,
		); err != nil {
			t.Fatalf("insert cook: %v", err)
		}
	}
}

func TestListDerivesCountsAndSorts(t *testing.T) {
	database := newTestDB(t)
	insertMeal(t, database, Meal{ID: "a", Name: "Alpha", Cuisine: "Thai", Rating: 5}, []Cook{
		{CookedOn: "2026-06-01"}, {CookedOn: "2026-06-08"}, {CookedOn: "2026-06-15"},
	})
	insertMeal(t, database, Meal{ID: "b", Name: "Bravo", Cuisine: "Italian", Rating: 3}, []Cook{
		{CookedOn: "2026-06-20"},
	})

	repo := NewRepo(database)

	// recent: Bravo (last 06-20) before Alpha (last 06-15)
	recent, err := repo.List(context.Background(), "recent", false)
	if err != nil {
		t.Fatalf("List recent: %v", err)
	}
	if len(recent) != 2 || recent[0].ID != "b" {
		t.Fatalf("recent order wrong: %+v", recent)
	}
	// derived counts
	if recent[1].ID != "a" || recent[1].TimesCooked != 3 || recent[1].LastCooked != "2026-06-15" {
		t.Fatalf("derived stats wrong: %+v", recent[1])
	}

	// rating: Alpha (5) before Bravo (3)
	byRating, err := repo.List(context.Background(), "rating", false)
	if err != nil {
		t.Fatalf("List rating: %v", err)
	}
	if byRating[0].ID != "a" {
		t.Fatalf("rating order wrong: %+v", byRating)
	}

	// favoritesOnly: only rating>=4 (Alpha)
	favs, err := repo.List(context.Background(), "recent", true)
	if err != nil {
		t.Fatalf("List favs: %v", err)
	}
	if len(favs) != 1 || favs[0].ID != "a" {
		t.Fatalf("favorites filter wrong: %+v", favs)
	}
}

func TestGetByIDReturnsMealHistoryAndLink(t *testing.T) {
	database := newTestDB(t)
	insertMeal(t, database, Meal{ID: "a", Name: "Alpha", Cuisine: "Thai", Rating: 5, RecipeID: "tomato-pasta"}, []Cook{
		{CookedOn: "2026-06-01", Note: "first"}, {CookedOn: "2026-06-15", Note: "latest"},
	})

	m, cooks, err := NewRepo(database).GetByID(context.Background(), "a")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if m.TimesCooked != 2 || m.LastCooked != "2026-06-15" || m.RecipeID != "tomato-pasta" {
		t.Fatalf("meal wrong: %+v", m)
	}
	if len(cooks) != 2 || cooks[0].CookedOn != "2026-06-15" {
		t.Fatalf("cooks newest-first wrong: %+v", cooks)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	database := newTestDB(t)
	_, _, err := NewRepo(database).GetByID(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestSeedIfEmptyGeneratesCookLog(t *testing.T) {
	database := newTestDB(t)

	if err := SeedIfEmpty(database); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	repo := NewRepo(database)

	all, err := repo.List(context.Background(), "recent", false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 11 {
		t.Fatalf("meals = %d, want 11", len(all))
	}

	// salmon: timesCooked 7, last 2026-06-18, no recipe link
	m, cooks, err := repo.GetByID(context.Background(), "salmon")
	if err != nil {
		t.Fatalf("get salmon: %v", err)
	}
	if m.TimesCooked != 7 || m.LastCooked != "2026-06-18" {
		t.Fatalf("salmon derived stats wrong: %+v", m)
	}
	if len(cooks) != 5 { // LIMIT 5 of the 7 logged
		t.Fatalf("salmon recent cooks = %d, want 5", len(cooks))
	}

	// pasta links to a real recipe
	pasta, _, err := repo.GetByID(context.Background(), "pasta")
	if err != nil {
		t.Fatalf("get pasta: %v", err)
	}
	if pasta.RecipeID != "tomato-pasta" {
		t.Fatalf("pasta recipe_id = %q, want tomato-pasta", pasta.RecipeID)
	}

	// idempotent
	if err := SeedIfEmpty(database); err != nil {
		t.Fatalf("second seed: %v", err)
	}
	again, _ := repo.List(context.Background(), "recent", false)
	if len(again) != 11 {
		t.Fatalf("meals after second seed = %d, want 11", len(again))
	}
}
