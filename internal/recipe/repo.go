package recipe

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotFound is returned by GetByID when no recipe matches the id.
var ErrNotFound = errors.New("recipe not found")

// Repo is a thin hand-written SQL repository over the recipe tables.
type Repo struct {
	db *sql.DB
}

// NewRepo wraps a database handle.
func NewRepo(database *sql.DB) *Repo {
	return &Repo{db: database}
}

// List returns all recipes ordered by name, each with its ingredients. It uses
// two queries (recipes, then all ingredient lines) to avoid N+1.
func (r *Repo) List(ctx context.Context) ([]Recipe, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, instructions, servings, total_minutes
		 FROM recipe ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query recipes: %w", err)
	}
	defer rows.Close()

	recipes := []Recipe{}
	indexByID := map[string]int{}
	for rows.Next() {
		var rec Recipe
		if err := rows.Scan(
			&rec.ID, &rec.Name, &rec.Instructions, &rec.Servings, &rec.TotalMinutes,
		); err != nil {
			return nil, fmt.Errorf("scan recipe: %w", err)
		}
		rec.Ingredients = []RecipeIngredient{}
		indexByID[rec.ID] = len(recipes)
		recipes = append(recipes, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recipes: %w", err)
	}

	lines, err := r.db.QueryContext(ctx,
		`SELECT ri.recipe_id, ri.ingredient_id, i.name, ri.quantity, ri.unit
		 FROM recipe_ingredient ri
		 JOIN ingredient i ON i.id = ri.ingredient_id
		 ORDER BY i.name`)
	if err != nil {
		return nil, fmt.Errorf("query ingredient lines: %w", err)
	}
	defer lines.Close()
	for lines.Next() {
		var recipeID string
		var li RecipeIngredient
		if err := lines.Scan(
			&recipeID, &li.IngredientID, &li.Name, &li.Quantity, &li.Unit,
		); err != nil {
			return nil, fmt.Errorf("scan ingredient line: %w", err)
		}
		// recipeID is always present: foreign_keys(ON) guarantees the join row
		// references a real recipe, and List fetches every recipe. The guard is
		// defensive against an inconsistent DB.
		if idx, ok := indexByID[recipeID]; ok {
			recipes[idx].Ingredients = append(recipes[idx].Ingredients, li)
		}
	}
	if err := lines.Err(); err != nil {
		return nil, fmt.Errorf("iterate ingredient lines: %w", err)
	}
	return recipes, nil
}

// GetByID returns one recipe with its ingredients, or ErrNotFound.
func (r *Repo) GetByID(ctx context.Context, id string) (Recipe, error) {
	var rec Recipe
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, instructions, servings, total_minutes
		 FROM recipe WHERE id = ?`, id).
		Scan(&rec.ID, &rec.Name, &rec.Instructions, &rec.Servings, &rec.TotalMinutes)
	if errors.Is(err, sql.ErrNoRows) {
		return Recipe{}, ErrNotFound
	}
	if err != nil {
		return Recipe{}, fmt.Errorf("query recipe %q: %w", id, err)
	}

	lines, err := r.db.QueryContext(ctx,
		`SELECT ri.ingredient_id, i.name, ri.quantity, ri.unit
		 FROM recipe_ingredient ri
		 JOIN ingredient i ON i.id = ri.ingredient_id
		 WHERE ri.recipe_id = ?
		 ORDER BY i.name`, id)
	if err != nil {
		return Recipe{}, fmt.Errorf("query ingredients for %q: %w", id, err)
	}
	defer lines.Close()
	rec.Ingredients = []RecipeIngredient{}
	for lines.Next() {
		var li RecipeIngredient
		if err := lines.Scan(&li.IngredientID, &li.Name, &li.Quantity, &li.Unit); err != nil {
			return Recipe{}, fmt.Errorf("scan ingredient line: %w", err)
		}
		rec.Ingredients = append(rec.Ingredients, li)
	}
	if err := lines.Err(); err != nil {
		return Recipe{}, fmt.Errorf("iterate ingredient lines: %w", err)
	}
	return rec, nil
}
