package recipe

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed fixtures/recipes.json
var recipesFixture []byte

// seedRecipe is the JSON shape of a fixture recipe.
type seedRecipe struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Instructions string `json:"instructions"`
	Servings     int    `json:"servings"`
	TotalMinutes int    `json:"totalMinutes"`
	Ingredients  []struct {
		ID       string  `json:"id"`
		Name     string  `json:"name"`
		Quantity float64 `json:"quantity"`
		Unit     string  `json:"unit"`
	} `json:"ingredients"`
}

// SeedIfEmpty loads fixture recipes only when the recipe table is empty. It is
// idempotent and safe to call on every startup. The emptiness check and all
// inserts run in one transaction, so the check and writes are atomic; shared
// ingredients are de-duplicated via ON CONFLICT DO NOTHING.
func SeedIfEmpty(database *sql.DB) error {
	var seeds []seedRecipe
	if err := json.Unmarshal(recipesFixture, &seeds); err != nil {
		return fmt.Errorf("parse fixtures: %w", err)
	}

	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("begin seed tx: %w", err)
	}
	defer tx.Rollback()

	// Check emptiness inside the transaction so the count and the inserts are
	// a single atomic unit (no check-then-act race between callers).
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM recipe`).Scan(&count); err != nil {
		return fmt.Errorf("count recipes: %w", err)
	}
	if count > 0 {
		return nil
	}

	for _, s := range seeds {
		if _, err := tx.Exec(
			`INSERT INTO recipe (id, name, instructions, servings, total_minutes)
			 VALUES (?, ?, ?, ?, ?)`,
			s.ID, s.Name, s.Instructions, s.Servings, s.TotalMinutes,
		); err != nil {
			return fmt.Errorf("insert recipe %q: %w", s.ID, err)
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
	return tx.Commit()
}
