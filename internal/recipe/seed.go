package recipe

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed fixtures/recipes.json
var recipesFixture []byte

//go:embed fixtures/pantry.json
var pantryFixture []byte

// seedRecipe is the JSON shape of a fixture recipe.
type seedRecipe struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Cuisine      string   `json:"cuisine"`
	Difficulty   string   `json:"difficulty"`
	Servings     int      `json:"servings"`
	TotalMinutes int      `json:"totalMinutes"`
	Steps        []string `json:"steps"`
	Ingredients  []struct {
		ID       string  `json:"id"`
		Name     string  `json:"name"`
		Quantity float64 `json:"quantity"`
		Unit     string  `json:"unit"`
	} `json:"ingredients"`
}

// SeedIfEmpty loads fixture recipes (with metadata, steps, ingredients) and the
// pantry only when the recipe table is empty. Idempotent; the emptiness check
// and all inserts run in one transaction. Shared ingredients de-duplicate via
// ON CONFLICT DO NOTHING.
func SeedIfEmpty(database *sql.DB) error {
	var seeds []seedRecipe
	if err := json.Unmarshal(recipesFixture, &seeds); err != nil {
		return fmt.Errorf("parse recipe fixtures: %w", err)
	}
	var pantry []string
	if err := json.Unmarshal(pantryFixture, &pantry); err != nil {
		return fmt.Errorf("parse pantry fixtures: %w", err)
	}

	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("begin seed tx: %w", err)
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM recipe`).Scan(&count); err != nil {
		return fmt.Errorf("count recipes: %w", err)
	}
	if count > 0 {
		return nil
	}

	for _, s := range seeds {
		if _, err := tx.Exec(
			`INSERT INTO recipe (id, name, cuisine, difficulty, servings, total_minutes)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			s.ID, s.Name, s.Cuisine, s.Difficulty, s.Servings, s.TotalMinutes,
		); err != nil {
			return fmt.Errorf("insert recipe %q: %w", s.ID, err)
		}
		for i, text := range s.Steps {
			if _, err := tx.Exec(
				`INSERT INTO recipe_step (recipe_id, position, text) VALUES (?, ?, ?)`,
				s.ID, i+1, text,
			); err != nil {
				return fmt.Errorf("insert step %d for %q: %w", i+1, s.ID, err)
			}
		}
		for _, ing := range s.Ingredients {
			if _, err := tx.Exec(
				`INSERT INTO ingredient (id, name) VALUES (?, ?) ON CONFLICT DO NOTHING`,
				ing.ID, ing.Name,
			); err != nil {
				return fmt.Errorf("insert ingredient %q: %w", ing.ID, err)
			}
			if _, err := tx.Exec(
				`INSERT INTO recipe_ingredient (recipe_id, ingredient_id, quantity, unit)
				 VALUES (?, ?, ?, ?)`,
				s.ID, ing.ID, ing.Quantity, ing.Unit,
			); err != nil {
				return fmt.Errorf("insert recipe_ingredient %q/%q: %w", s.ID, ing.ID, err)
			}
		}
	}

	// Pantry items reference ingredient(id); insert after ingredients exist.
	for _, id := range pantry {
		if _, err := tx.Exec(
			`INSERT INTO pantry_item (ingredient_id) VALUES (?) ON CONFLICT DO NOTHING`, id,
		); err != nil {
			return fmt.Errorf("insert pantry item %q: %w", id, err)
		}
	}
	return tx.Commit()
}
