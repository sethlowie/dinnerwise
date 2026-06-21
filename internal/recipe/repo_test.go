package recipe

import (
	"context"
	"database/sql"
	"errors"
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

// insertRecipe is a test helper that writes a recipe, its ingredients, and the
// join rows directly (no seeding).
func insertRecipe(t *testing.T, database *sql.DB, r Recipe) {
	t.Helper()
	_, err := database.Exec(
		`INSERT INTO recipe (id, name, instructions, servings, total_minutes)
		 VALUES (?, ?, ?, ?, ?)`,
		r.ID, r.Name, r.Instructions, r.Servings, r.TotalMinutes,
	)
	if err != nil {
		t.Fatalf("insert recipe: %v", err)
	}
	for _, ing := range r.Ingredients {
		if _, err := database.Exec(
			`INSERT INTO ingredient (id, name) VALUES (?, ?) ON CONFLICT DO NOTHING`,
			ing.IngredientID, ing.Name,
		); err != nil {
			t.Fatalf("insert ingredient: %v", err)
		}
		if _, err := database.Exec(
			`INSERT INTO recipe_ingredient (recipe_id, ingredient_id, quantity, unit)
			 VALUES (?, ?, ?, ?)`,
			r.ID, ing.IngredientID, ing.Quantity, ing.Unit,
		); err != nil {
			t.Fatalf("insert join: %v", err)
		}
	}
}

func TestListAssemblesIngredients(t *testing.T) {
	database := newTestDB(t)
	insertRecipe(t, database, Recipe{
		ID: "a-soup", Name: "A Soup", Servings: 2, TotalMinutes: 20,
		Ingredients: []RecipeIngredient{
			{IngredientID: "water", Name: "Water", Quantity: 1, Unit: "L"},
			{IngredientID: "salt", Name: "Salt", Quantity: 1, Unit: "tsp"},
		},
	})
	insertRecipe(t, database, Recipe{
		ID: "b-toast", Name: "B Toast", Servings: 1, TotalMinutes: 5,
		Ingredients: []RecipeIngredient{
			{IngredientID: "bread", Name: "Bread", Quantity: 2, Unit: "slice"},
		},
	})

	got, err := NewRepo(database).List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(recipes) = %d, want 2", len(got))
	}
	// Ordered by name: "A Soup" then "B Toast".
	if got[0].ID != "a-soup" || got[1].ID != "b-toast" {
		t.Fatalf("order = %q,%q; want a-soup,b-toast", got[0].ID, got[1].ID)
	}
	if len(got[0].Ingredients) != 2 {
		t.Fatalf("a-soup ingredients = %d, want 2", len(got[0].Ingredients))
	}
	if len(got[1].Ingredients) != 1 {
		t.Fatalf("b-toast ingredients = %d, want 1", len(got[1].Ingredients))
	}
}

func TestGetByID(t *testing.T) {
	database := newTestDB(t)
	insertRecipe(t, database, Recipe{
		ID: "a-soup", Name: "A Soup", Servings: 2, TotalMinutes: 20,
		Ingredients: []RecipeIngredient{
			{IngredientID: "water", Name: "Water", Quantity: 1, Unit: "L"},
		},
	})

	got, err := NewRepo(database).GetByID(context.Background(), "a-soup")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "A Soup" || len(got.Ingredients) != 1 {
		t.Fatalf("got %+v", got)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	database := newTestDB(t)
	_, err := NewRepo(database).GetByID(context.Background(), "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestSeedIfEmptyInsertsOnceAndIsIdempotent(t *testing.T) {
	database := newTestDB(t)

	if err := SeedIfEmpty(database); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	first, err := NewRepo(database).List(context.Background())
	if err != nil {
		t.Fatalf("list after first seed: %v", err)
	}
	if len(first) != 3 {
		t.Fatalf("recipes after seed = %d, want 3", len(first))
	}

	// Second seed is a no-op (no duplicate-key error, count unchanged).
	if err := SeedIfEmpty(database); err != nil {
		t.Fatalf("second seed: %v", err)
	}
	second, err := NewRepo(database).List(context.Background())
	if err != nil {
		t.Fatalf("list after second seed: %v", err)
	}
	if len(second) != 3 {
		t.Fatalf("recipes after second seed = %d, want 3", len(second))
	}

	// Shared ingredient (olive-oil) collapses to one row.
	var ingredients int
	if err := database.QueryRow(`SELECT COUNT(*) FROM ingredient WHERE id='olive-oil'`).
		Scan(&ingredients); err != nil {
		t.Fatalf("count olive-oil: %v", err)
	}
	if ingredients != 1 {
		t.Fatalf("olive-oil rows = %d, want 1", ingredients)
	}
}

func TestForeignKeyCascadeOnRecipeDelete(t *testing.T) {
	database := newTestDB(t)
	if err := SeedIfEmpty(database); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := database.Exec(`DELETE FROM recipe WHERE id='tomato-pasta'`); err != nil {
		t.Fatalf("delete recipe: %v", err)
	}
	var joins int
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM recipe_ingredient WHERE recipe_id='tomato-pasta'`,
	).Scan(&joins); err != nil {
		t.Fatalf("count joins: %v", err)
	}
	if joins != 0 {
		t.Fatalf("join rows after delete = %d, want 0 (cascade)", joins)
	}
}
