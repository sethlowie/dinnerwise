# Recipe Metadata + Pantry (Slice 3) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enrich the recipe domain with cuisine, difficulty, structured numbered steps (replacing `instructions`), and an `in_pantry` flag derived from a real global pantry set — plus the Sous list badge and detail numbered steps.

**Architecture:** Modify the existing `internal/recipe` package in place. Dropping the `instructions` struct/proto field ripples through schema/repo/seed/service as one compile unit, so the backend is one task; the frontend (regenerated TS types) is the second. Pantry is a global `pantry_item` set; `in_pantry` = recipe has ingredients and all are in the pantry, computed in Go.

**Tech Stack:** Go 1.25, `database/sql`/SQLite, ConnectRPC, buf v2, React 19 + Vite, TanStack Router, Connect-Query, Tailwind v4.

## Global Constraints

- Modify `internal/recipe` in place; no new package. stdlib `database/sql` + hand-written SQL.
- Schema stays idempotent `CREATE TABLE IF NOT EXISTS` (no migration tooling); the local `dinnerwise.db` is disposable and must be deleted to pick up new columns. Tests use `t.TempDir`.
- `instructions` is removed (struct, proto, repo, seed, service, fixtures). Reserve proto field number 3 (`reserved 3;`) — never reuse it.
- `in_pantry` = `len(ingredients) > 0 && every ingredient ∈ pantry`. Pantry is a single global set.
- Generated code is gitignored (`make gen`). Proto pkg `recipev1`, connect pkg `recipev1connect`.
- Module path `github.com/sethlowie/dinnerwise`. Commit messages end with: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

---

## File Structure

- Modify: `internal/recipe/schema.sql`, `internal/recipe/recipe.go`, `internal/recipe/v1/recipe.proto`, `internal/recipe/repo.go`, `internal/recipe/seed.go`, `internal/recipe/service.go`, `internal/recipe/repo_test.go`, `internal/recipe/service_test.go`, `internal/recipe/fixtures/recipes.json`.
- Create: `internal/recipe/fixtures/pantry.json`.
- Modify: `web/app/src/routes/recipes.tsx`, `web/app/src/routes/recipe.tsx`.

---

## Task 1: Recipe metadata, steps, and pantry (backend)

**Files:**
- Modify: `internal/recipe/schema.sql`, `recipe.go`, `v1/recipe.proto`, `repo.go`, `seed.go`, `service.go`, `repo_test.go`, `service_test.go`, `fixtures/recipes.json`
- Create: `internal/recipe/fixtures/pantry.json`

**Interfaces:**
- Consumes: `db.ApplySchema`; existing recipe repo/seed/service.
- Produces: `Recipe` with `Cuisine, Difficulty string`, `Steps []string`, `InPantry bool` (no `Instructions`); repo `List`/`GetByID` populate them; proto `Recipe` gains `cuisine`/`difficulty`/`steps`/`in_pantry` (field 3 reserved).

- [ ] **Step 1: Update the schema**

Replace the entire contents of `internal/recipe/schema.sql` with:
```sql
CREATE TABLE IF NOT EXISTS recipe (
  id            TEXT PRIMARY KEY,
  name          TEXT NOT NULL,
  cuisine       TEXT NOT NULL DEFAULT '',
  difficulty    TEXT NOT NULL DEFAULT '',
  servings      INTEGER NOT NULL DEFAULT 0,
  total_minutes INTEGER NOT NULL DEFAULT 0,
  created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS ingredient (
  id   TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS recipe_ingredient (
  recipe_id     TEXT NOT NULL REFERENCES recipe(id) ON DELETE CASCADE,
  ingredient_id TEXT NOT NULL REFERENCES ingredient(id) ON DELETE CASCADE,
  quantity      REAL NOT NULL DEFAULT 0,
  unit          TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (recipe_id, ingredient_id)
);

CREATE TABLE IF NOT EXISTS recipe_step (
  recipe_id TEXT NOT NULL REFERENCES recipe(id) ON DELETE CASCADE,
  position  INTEGER NOT NULL,
  text      TEXT NOT NULL,
  PRIMARY KEY (recipe_id, position)
);

CREATE TABLE IF NOT EXISTS pantry_item (
  ingredient_id TEXT PRIMARY KEY REFERENCES ingredient(id) ON DELETE CASCADE
);
```

- [ ] **Step 2: Update the structs**

Replace the `Recipe` struct in `internal/recipe/recipe.go` (keep the package comment and `RecipeIngredient` unchanged):
```go
// Recipe is a recipe with metadata, ordered method steps, and assembled
// ingredient lines. InPantry is true when every ingredient is in the pantry.
type Recipe struct {
	ID           string
	Name         string
	Cuisine      string
	Difficulty   string
	Servings     int
	TotalMinutes int
	Steps        []string
	Ingredients  []RecipeIngredient
	InPantry     bool
}
```

- [ ] **Step 3: Update the proto and regenerate**

Replace the `Recipe` message in `internal/recipe/v1/recipe.proto` with:
```proto
message Recipe {
  string id = 1;
  string name = 2;
  reserved 3;  // was: string instructions
  int32 servings = 4;
  int32 total_minutes = 5;
  repeated RecipeIngredient ingredients = 6;
  string cuisine = 7;
  string difficulty = 8;
  repeated string steps = 9;
  bool in_pantry = 10;
}
```
Then run: `make gen`
Expected: succeeds; regenerated Go/TS reflect the new fields (no `Instructions`).

- [ ] **Step 4: Update the recipe fixtures**

Replace the entire contents of `internal/recipe/fixtures/recipes.json` with:
```json
[
  {
    "id": "sheet-pan-chicken",
    "name": "Sheet-Pan Chicken & Veg",
    "cuisine": "American",
    "difficulty": "Easy",
    "servings": 4,
    "totalMinutes": 40,
    "steps": [
      "Heat the oven to 220°C (425°F).",
      "Toss the chicken and broccoli with olive oil, salt, and pepper.",
      "Spread on a sheet pan and roast 30 minutes until golden."
    ],
    "ingredients": [
      { "id": "chicken-thigh", "name": "Chicken thighs", "quantity": 800, "unit": "g" },
      { "id": "broccoli", "name": "Broccoli", "quantity": 1, "unit": "head" },
      { "id": "olive-oil", "name": "Olive oil", "quantity": 2, "unit": "tbsp" }
    ]
  },
  {
    "id": "tomato-pasta",
    "name": "Weeknight Tomato Pasta",
    "cuisine": "Italian",
    "difficulty": "Easy",
    "servings": 4,
    "totalMinutes": 25,
    "steps": [
      "Boil the spaghetti until al dente; reserve a little pasta water.",
      "Sizzle the garlic in olive oil, add the canned tomatoes, and simmer 10 minutes.",
      "Toss the pasta through the sauce, loosening with pasta water, and serve."
    ],
    "ingredients": [
      { "id": "spaghetti", "name": "Spaghetti", "quantity": 400, "unit": "g" },
      { "id": "canned-tomato", "name": "Canned tomatoes", "quantity": 800, "unit": "g" },
      { "id": "garlic", "name": "Garlic", "quantity": 3, "unit": "clove" },
      { "id": "olive-oil", "name": "Olive oil", "quantity": 1, "unit": "tbsp" }
    ]
  },
  {
    "id": "veggie-stir-fry",
    "name": "Tofu Veggie Stir-Fry",
    "cuisine": "Chinese",
    "difficulty": "Easy",
    "servings": 3,
    "totalMinutes": 30,
    "steps": [
      "Press and cube the tofu, then fry until golden.",
      "Add the bell pepper and stir-fry 3 minutes.",
      "Add the soy sauce, toss, and serve over rice."
    ],
    "ingredients": [
      { "id": "tofu", "name": "Firm tofu", "quantity": 400, "unit": "g" },
      { "id": "bell-pepper", "name": "Bell pepper", "quantity": 2, "unit": "whole" },
      { "id": "soy-sauce", "name": "Soy sauce", "quantity": 3, "unit": "tbsp" },
      { "id": "rice", "name": "Rice", "quantity": 300, "unit": "g" }
    ]
  }
]
```

- [ ] **Step 5: Create the pantry fixture**

Create `internal/recipe/fixtures/pantry.json`:
```json
[
  "olive-oil",
  "garlic",
  "spaghetti",
  "canned-tomato",
  "chicken-thigh",
  "broccoli"
]
```

- [ ] **Step 6: Rewrite the repo**

Replace the entire contents of `internal/recipe/repo.go` with:
```go
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
```

- [ ] **Step 7: Rewrite the seed**

Replace the entire contents of `internal/recipe/seed.go` with:
```go
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
```

- [ ] **Step 8: Update the service mapping**

In `internal/recipe/service.go`, replace the `toProtoRecipe` function with:
```go
// toProtoRecipe maps a domain Recipe to its proto representation.
func toProtoRecipe(r Recipe) *recipev1.Recipe {
	ingredients := make([]*recipev1.RecipeIngredient, len(r.Ingredients))
	for i, ing := range r.Ingredients {
		ingredients[i] = &recipev1.RecipeIngredient{
			IngredientId: ing.IngredientID,
			Name:         ing.Name,
			Quantity:     ing.Quantity,
			Unit:         ing.Unit,
		}
	}
	return &recipev1.Recipe{
		Id:           r.ID,
		Name:         r.Name,
		Cuisine:      r.Cuisine,
		Difficulty:   r.Difficulty,
		Servings:     int32(r.Servings),
		TotalMinutes: int32(r.TotalMinutes),
		Steps:        r.Steps,
		Ingredients:  ingredients,
		InPantry:     r.InPantry,
	}
}
```

- [ ] **Step 9: Update the repo tests (helper + new behavior)**

In `internal/recipe/repo_test.go`, replace the `insertRecipe` helper with the version below (drops `instructions`, adds cuisine/difficulty + steps), and add an `addPantry` helper:
```go
// insertRecipe writes a recipe, its steps, ingredients, and join rows directly.
func insertRecipe(t *testing.T, database *sql.DB, r Recipe) {
	t.Helper()
	if _, err := database.Exec(
		`INSERT INTO recipe (id, name, cuisine, difficulty, servings, total_minutes)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		r.ID, r.Name, r.Cuisine, r.Difficulty, r.Servings, r.TotalMinutes,
	); err != nil {
		t.Fatalf("insert recipe: %v", err)
	}
	for i, s := range r.Steps {
		if _, err := database.Exec(
			`INSERT INTO recipe_step (recipe_id, position, text) VALUES (?, ?, ?)`,
			r.ID, i+1, s,
		); err != nil {
			t.Fatalf("insert step: %v", err)
		}
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

// addPantry marks ingredient ids as on-hand (they must already exist).
func addPantry(t *testing.T, database *sql.DB, ids ...string) {
	t.Helper()
	for _, id := range ids {
		if _, err := database.Exec(
			`INSERT INTO pantry_item (ingredient_id) VALUES (?) ON CONFLICT DO NOTHING`, id,
		); err != nil {
			t.Fatalf("add pantry %q: %v", id, err)
		}
	}
}
```
Then append these tests:
```go
func TestStepsAssembledInOrder(t *testing.T) {
	database := newTestDB(t)
	insertRecipe(t, database, Recipe{
		ID: "r", Name: "R", Cuisine: "Test", Difficulty: "Easy",
		Steps:       []string{"first", "second", "third"},
		Ingredients: []RecipeIngredient{{IngredientID: "egg", Name: "Egg"}},
	})

	rec, err := NewRepo(database).GetByID(context.Background(), "r")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if rec.Cuisine != "Test" || rec.Difficulty != "Easy" {
		t.Fatalf("metadata wrong: %+v", rec)
	}
	want := []string{"first", "second", "third"}
	if len(rec.Steps) != 3 || rec.Steps[0] != want[0] || rec.Steps[2] != want[2] {
		t.Fatalf("steps wrong/out of order: %v", rec.Steps)
	}
}

func TestInPantryDerivation(t *testing.T) {
	database := newTestDB(t)
	insertRecipe(t, database, Recipe{
		ID: "have", Name: "Have",
		Ingredients: []RecipeIngredient{{IngredientID: "egg", Name: "Egg"}, {IngredientID: "milk", Name: "Milk"}},
	})
	insertRecipe(t, database, Recipe{
		ID: "missing", Name: "Missing",
		Ingredients: []RecipeIngredient{{IngredientID: "egg", Name: "Egg"}, {IngredientID: "flour", Name: "Flour"}},
	})
	insertRecipe(t, database, Recipe{ID: "empty", Name: "Empty"}) // no ingredients
	addPantry(t, database, "egg", "milk")                          // not flour

	list, err := NewRepo(database).List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := map[string]bool{}
	for _, r := range list {
		got[r.ID] = r.InPantry
	}
	if !got["have"] {
		t.Fatal("have should be in pantry")
	}
	if got["missing"] {
		t.Fatal("missing should not be in pantry (flour absent)")
	}
	if got["empty"] {
		t.Fatal("empty (no ingredients) should not be in pantry")
	}
}
```

- [ ] **Step 10: Update the seed + service tests for the new fields**

In `internal/recipe/repo_test.go`, append a seed-level pantry/steps check:
```go
func TestSeedSetsMetadataStepsAndPantry(t *testing.T) {
	database := newTestDB(t)
	if err := SeedIfEmpty(database); err != nil {
		t.Fatalf("seed: %v", err)
	}
	repo := NewRepo(database)

	pasta, err := repo.GetByID(context.Background(), "tomato-pasta")
	if err != nil {
		t.Fatalf("get pasta: %v", err)
	}
	if pasta.Cuisine != "Italian" || len(pasta.Steps) == 0 {
		t.Fatalf("pasta metadata/steps wrong: %+v", pasta)
	}
	if !pasta.InPantry {
		t.Fatal("tomato-pasta should be in pantry per the fixture")
	}

	stir, err := repo.GetByID(context.Background(), "veggie-stir-fry")
	if err != nil {
		t.Fatalf("get stir-fry: %v", err)
	}
	if stir.InPantry {
		t.Fatal("veggie-stir-fry should NOT be in pantry per the fixture")
	}
}
```
Then open `internal/recipe/service_test.go` and check whether any assertion references `Instructions` / `GetInstructions`. If so, remove those lines (the field no longer exists). Add this assertion to the existing `TestServiceListRecipes` (or as a new test) verifying the new fields map:
```go
func TestServiceMapsMetadataAndPantry(t *testing.T) {
	svc := newSeededService(t)
	resp, err := svc.GetRecipe(context.Background(),
		connect.NewRequest(&recipev1.GetRecipeRequest{Id: "tomato-pasta"}))
	if err != nil {
		t.Fatalf("GetRecipe: %v", err)
	}
	r := resp.Msg.Recipe
	if r.GetCuisine() != "Italian" || len(r.GetSteps()) == 0 || !r.GetInPantry() {
		t.Fatalf("mapped recipe wrong: %+v", r)
	}
}
```

- [ ] **Step 11: Verify the full suite + boot**

Run:
```bash
go vet ./... && go test ./... 2>&1 | grep -E "ok|FAIL"
DINNERWISE_DB="$(mktemp -d)/dinnerwise.db" ADDR=:8089 go run ./cmd/server/ &
sleep 2
curl -s -X POST http://localhost:8089/internal.recipe.v1.RecipeService/ListRecipes \
  -H "Content-Type: application/json" -H "Connect-Protocol-Version: 1" -d '{}' | head -c 400
echo
kill %1 2>/dev/null
```
Expected: `ok` for `internal/recipe` and all other packages (meal seeding still works — its recipe FKs resolve); the JSON shows `cuisine`, `steps`, and `inPantry` on recipes (tomato-pasta `inPantry:true`). If port busy, pick another.

- [ ] **Step 12: Commit**

Generated code is gitignored; commit only the hand-written files.
```bash
git add internal/recipe/schema.sql internal/recipe/recipe.go internal/recipe/v1/recipe.proto internal/recipe/repo.go internal/recipe/seed.go internal/recipe/service.go internal/recipe/repo_test.go internal/recipe/service_test.go internal/recipe/fixtures/
git commit -m "feat: recipe cuisine/difficulty, steps, and pantry-derived in_pantry

Replace instructions with an ordered recipe_step table; add cuisine/difficulty
columns and a global pantry_item set; repo derives in_pantry; proto reserves
field 3 and adds cuisine/difficulty/steps/in_pantry.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Recipe list badge + detail numbered steps (frontend)

**Files:** Modify `web/app/src/routes/recipes.tsx`, `web/app/src/routes/recipe.tsx`.

**Interfaces:**
- Consumes: regenerated TS `Recipe` with `cuisine`, `difficulty`, `steps: string[]`, `inPantry: boolean` (no `instructions`).
- Produces: list cards with cuisine meta + "in pantry" badge; detail with cuisine/difficulty meta, numbered steps, and an in-pantry indicator.

Note: no frontend unit tests — verify with `tsc -b`/`eslint`/`vite build` + runtime.

- [ ] **Step 1: Add cuisine meta + in-pantry badge to the list**

In `web/app/src/routes/recipes.tsx`, replace the inner card markup (the `<Link>` body — the thumb div + the name/meta `<div className="min-w-0">`) so each card shows cuisine and a pantry badge. Replace this block:
```tsx
              <div className="min-w-0">
                <div className="truncate font-medium">{r.name}</div>
                <div className="mt-0.5 truncate font-mono text-xs text-muted-foreground">
                  ⏱ {r.totalMinutes} min · serves {r.servings}
                </div>
              </div>
```
with:
```tsx
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="truncate font-medium">{r.name}</span>
                  {r.inPantry && (
                    <span className="flex-none rounded-md border border-emerald-500/35 bg-emerald-500/10 px-1.5 py-0.5 font-mono text-[10px] text-emerald-400">
                      in pantry
                    </span>
                  )}
                </div>
                <div className="mt-0.5 truncate font-mono text-xs text-muted-foreground">
                  ⏱ {r.totalMinutes} min · {r.cuisine}
                </div>
              </div>
```
(If the surrounding `<Link>` className lacks `items-center`, leave it as-is; the existing card already uses `flex items-center gap-3.5`.)

- [ ] **Step 2: Verify the list typechecks**

Run (from `web/app`): `npx tsc -b`
Expected: clean (no reference to a removed `instructions` field).

- [ ] **Step 3: Replace the detail Method block with numbered steps + metadata**

In `web/app/src/routes/recipe.tsx`:

(a) Replace the meta line under the title:
```tsx
          <p className="mt-2 font-mono text-sm text-muted-foreground">
            ⏱ {recipe.totalMinutes} min · serves {recipe.servings}
          </p>
```
with:
```tsx
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <span className="font-mono text-sm text-muted-foreground">
              ⏱ {recipe.totalMinutes} min · {recipe.cuisine} · {recipe.difficulty}
            </span>
            {recipe.inPantry && (
              <span className="rounded-md border border-emerald-500/35 bg-emerald-500/10 px-1.5 py-0.5 font-mono text-[10px] text-emerald-400">
                in pantry
              </span>
            )}
          </div>
```

(b) Replace the entire Method `<section>` (the one that derived `methodLines` from `recipe.instructions`) with a numbered-steps section, and delete the now-unused `methodLines` constant:
```tsx
      <section>
        <div className="mb-3 font-mono text-xs uppercase tracking-[0.1em] text-muted-foreground">
          Method
        </div>
        <ol className="flex flex-col gap-3">
          {recipe.steps.map((step, i) => (
            <li key={i} className="flex gap-3">
              <span className="flex h-6 w-6 flex-none items-center justify-center rounded-lg border border-primary/40 bg-primary/10 font-mono text-xs text-primary">
                {i + 1}
              </span>
              <span className="text-sm leading-relaxed text-foreground/85">{step}</span>
            </li>
          ))}
        </ol>
      </section>
```
Also remove the line `const methodLines = recipe.instructions...` (and any reference to it) — `recipe.instructions` no longer exists.

- [ ] **Step 4: Verify typecheck, lint, build**

Run (from `web/app`): `npx tsc -b && pnpm lint && pnpm build`
Expected: all clean (no `recipe.instructions` references remain). Return to repo root: `cd ../..`

- [ ] **Step 5: Runtime check**

Run (from repo root):
```bash
DINNERWISE_DB="$(mktemp -d)/dinnerwise.db" ADDR=:8088 go run ./cmd/server/ &
(cd web/app && VITE_API_URL=http://localhost:8088 pnpm dev --port 5179 &)
sleep 4
curl -s -o /dev/null -w "recipes %{http_code}\n" http://localhost:5179/recipes
kill %1 %2 2>/dev/null; pkill -f vite 2>/dev/null
```
Expected: `recipes 200`. Controller's visual check: the list shows an "in pantry" badge on Tomato Pasta and Sheet-Pan Chicken (not Stir-Fry); the meta line shows cuisine; a recipe detail shows `⏱ … · cuisine · difficulty` and a numbered Method list.

- [ ] **Step 6: Commit**

```bash
git add web/app/src/routes/recipes.tsx web/app/src/routes/recipe.tsx
git commit -m "feat: recipe in-pantry badge, cuisine/difficulty, numbered steps

List cards show cuisine + an in-pantry badge; detail shows cuisine/difficulty,
an in-pantry indicator, and numbered Method steps.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage:**
- Schema: cuisine/difficulty cols, drop instructions, recipe_step, pantry_item → Task 1 Step 1.
- Struct (Cuisine/Difficulty/Steps/InPantry, no Instructions) → Step 2.
- Proto: add fields, reserve 3, regen → Step 3.
- Fixtures: cuisine/difficulty/steps + pantry.json → Steps 4–5.
- Repo: select metadata, assemble steps, pantrySet + in_pantry (empty→false) → Step 6.
- Seed: steps + pantry inserts, idempotent, FK order → Step 7.
- Service mapping (drop instructions) → Step 8.
- Tests: steps order, in_pantry true/false/empty, seed metadata+pantry, service mapping, fix instructions refs → Steps 9–10.
- Full suite incl. meal still green + boot → Step 11.
- Frontend list badge + cuisine; detail cuisine/difficulty + numbered steps + indicator → Task 2.

**Placeholder scan:** none — concrete code/commands throughout. (Step 10 instructs checking `service_test.go` for `Instructions` refs and removing them; this is a conditional edit, not a placeholder — the field removal makes any such reference a compile error the implementer must resolve.)

**Type consistency:** `Recipe` fields (`Cuisine`,`Difficulty`,`Steps`,`InPantry`; no `Instructions`) used identically across recipe.go/repo.go/seed.go/service.go/tests. `toProtoRecipe` maps to proto `Cuisine/Difficulty/Steps/InPantry`. Generated TS camelCase (`cuisine`,`difficulty`,`steps`,`inPantry`) used in routes; `recipe.instructions` removed from both routes. `inPantry`/`steps` are the only new frontend fields. Pantry fixture ids match ingredient ids in recipes.json.
