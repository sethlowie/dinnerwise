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

// pantrySet loads the global pantry as a set of ingredient ids.
func (r *Repo) pantrySet(ctx context.Context) (map[string]struct{}, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT ingredient_id FROM pantry_item`)
	if err != nil {
		return nil, fmt.Errorf("query pantry: %w", err)
	}
	defer rows.Close()
	set := map[string]struct{}{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan pantry item: %w", err)
		}
		set[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pantry: %w", err)
	}
	return set, nil
}

// inPantry reports whether every ingredient of rec is in the pantry. A recipe
// with no ingredients is never "in pantry".
func inPantry(rec Recipe, pantry map[string]struct{}) bool {
	if len(rec.Ingredients) == 0 {
		return false
	}
	for _, ing := range rec.Ingredients {
		if _, ok := pantry[ing.IngredientID]; !ok {
			return false
		}
	}
	return true
}

// List returns all recipes ordered by name, each with its ingredients, steps,
// and in_pantry flag. Ingredients and steps are assembled with one query each
// (no N+1); in_pantry is computed against the pantry set.
func (r *Repo) List(ctx context.Context) ([]Recipe, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, cuisine, difficulty, servings, total_minutes
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
			&rec.ID, &rec.Name, &rec.Cuisine, &rec.Difficulty, &rec.Servings, &rec.TotalMinutes,
		); err != nil {
			return nil, fmt.Errorf("scan recipe: %w", err)
		}
		rec.Ingredients = []RecipeIngredient{}
		rec.Steps = []string{}
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
		if idx, ok := indexByID[recipeID]; ok {
			recipes[idx].Ingredients = append(recipes[idx].Ingredients, li)
		}
	}
	if err := lines.Err(); err != nil {
		return nil, fmt.Errorf("iterate ingredient lines: %w", err)
	}

	steps, err := r.db.QueryContext(ctx,
		`SELECT recipe_id, text FROM recipe_step ORDER BY recipe_id, position`)
	if err != nil {
		return nil, fmt.Errorf("query steps: %w", err)
	}
	defer steps.Close()
	for steps.Next() {
		var recipeID, text string
		if err := steps.Scan(&recipeID, &text); err != nil {
			return nil, fmt.Errorf("scan step: %w", err)
		}
		if idx, ok := indexByID[recipeID]; ok {
			recipes[idx].Steps = append(recipes[idx].Steps, text)
		}
	}
	if err := steps.Err(); err != nil {
		return nil, fmt.Errorf("iterate steps: %w", err)
	}

	pantry, err := r.pantrySet(ctx)
	if err != nil {
		return nil, err
	}
	for i := range recipes {
		recipes[i].InPantry = inPantry(recipes[i], pantry)
	}
	return recipes, nil
}

// GetByID returns one recipe with its ingredients, steps, and in_pantry flag,
// or ErrNotFound.
func (r *Repo) GetByID(ctx context.Context, id string) (Recipe, error) {
	var rec Recipe
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, cuisine, difficulty, servings, total_minutes
		 FROM recipe WHERE id = ?`, id).
		Scan(&rec.ID, &rec.Name, &rec.Cuisine, &rec.Difficulty, &rec.Servings, &rec.TotalMinutes)
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

	steps, err := r.db.QueryContext(ctx,
		`SELECT text FROM recipe_step WHERE recipe_id = ? ORDER BY position`, id)
	if err != nil {
		return Recipe{}, fmt.Errorf("query steps for %q: %w", id, err)
	}
	defer steps.Close()
	rec.Steps = []string{}
	for steps.Next() {
		var text string
		if err := steps.Scan(&text); err != nil {
			return Recipe{}, fmt.Errorf("scan step: %w", err)
		}
		rec.Steps = append(rec.Steps, text)
	}
	if err := steps.Err(); err != nil {
		return Recipe{}, fmt.Errorf("iterate steps: %w", err)
	}

	pantry, err := r.pantrySet(ctx)
	if err != nil {
		return Recipe{}, err
	}
	rec.InPantry = inPantry(rec, pantry)
	return rec, nil
}
