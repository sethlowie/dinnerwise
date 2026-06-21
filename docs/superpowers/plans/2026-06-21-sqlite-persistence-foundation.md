# SQLite Persistence Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a pure-Go SQLite persistence layer and prove the pattern end-to-end on a recipes/ingredients slice.

**Architecture:** A domain-agnostic `internal/db` package opens the connection and applies schema; a self-contained `internal/recipe` package owns its schema, structs, hand-written SQL repo, and JSON-fixture seeding. `cmd/server/main.go` wires open → migrate → seed at boot. New domains later are a new package plus two wiring lines.

**Tech Stack:** Go 1.25, stdlib `database/sql`, `modernc.org/sqlite` (pure Go, no CGO), `//go:embed` for schema + fixtures.

## Global Constraints

- Driver MUST be `modernc.org/sqlite` (pure Go). Do NOT use `mattn/go-sqlite3` (CGO).
- No ORM, no query codegen (no `sqlc`), no migration CLI (no `goose`).
- Schema is idempotent `CREATE TABLE IF NOT EXISTS`, applied at startup.
- Pragmas set via DSN so they apply per pooled connection: `foreign_keys(ON)`, `journal_mode(WAL)`, `busy_timeout(5000)`.
- IDs are human-readable `TEXT` slugs.
- Repo structs are plain Go (not proto types).
- Module path: `github.com/sethlowie/dinnerwise`.
- Commit messages end with: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

---

## File Structure

- Create: `internal/db/db.go` — `Open(path)`, `ApplySchema(db, schema)`. Generic, no domain knowledge.
- Create: `internal/db/db_test.go` — connection + pragma tests.
- Create: `internal/recipe/recipe.go` — `Recipe`, `RecipeIngredient` structs.
- Create: `internal/recipe/schema.sql` — recipe/ingredient/recipe_ingredient DDL.
- Create: `internal/recipe/migrate.go` — embeds `schema.sql`, `Migrate(db)`.
- Create: `internal/recipe/repo.go` — `Repo`, `List`, `GetByID`, `ErrNotFound`.
- Create: `internal/recipe/seed.go` — embeds fixtures, `SeedIfEmpty(db)`.
- Create: `internal/recipe/fixtures/recipes.json` — seed data.
- Create: `internal/recipe/repo_test.go` — repo, seed, cascade tests.
- Modify: `cmd/server/main.go` — wire db open/migrate/seed + boot log.
- Modify: `.gitignore` — ignore `*.db`, `*.db-wal`, `*.db-shm`.
- Modify: `go.mod` / `go.sum` — add `modernc.org/sqlite`.

---

## Task 1: Database connection package

**Files:**
- Create: `internal/db/db.go`
- Test: `internal/db/db_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `func Open(path string) (*sql.DB, error)` — opens SQLite at `path` with dinnerwise pragmas, pings.
  - `func ApplySchema(database *sql.DB, schema string) error` — runs idempotent DDL.

- [ ] **Step 1: Add the driver dependency**

Run:
```bash
go get modernc.org/sqlite@v1.53.0
```
Expected: `go.mod` gains `require modernc.org/sqlite v1.53.0`.

- [ ] **Step 2: Write the failing test**

Create `internal/db/db_test.go`:
```go
package db_test

import (
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
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/db/`
Expected: FAIL — `undefined: db.Open` / `undefined: db.ApplySchema` (package doesn't compile).

- [ ] **Step 4: Write minimal implementation**

Create `internal/db/db.go`:
```go
// Package db holds domain-agnostic SQLite plumbing: opening a connection with
// dinnerwise's pragmas and applying schema. It knows nothing about any domain.
package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open opens (creating if needed) the SQLite database at path with the pragmas
// dinnerwise relies on, then verifies the connection. Pragmas are set in the
// DSN so modernc applies them to every pooled connection.
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=foreign_keys(ON)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)",
		path,
	)
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := database.Ping(); err != nil {
		database.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return database, nil
}

// ApplySchema executes idempotent DDL against the database. Domain packages
// pass their embedded schema.sql here from their Migrate function.
func ApplySchema(database *sql.DB, schema string) error {
	if _, err := database.Exec(schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go mod tidy && go test ./internal/db/`
Expected: PASS (`ok  github.com/sethlowie/dinnerwise/internal/db`).

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/db/
git commit -m "feat: add pure-Go SQLite connection package

modernc.org/sqlite via database/sql; Open sets foreign_keys/WAL/busy_timeout
pragmas in the DSN. ApplySchema runs idempotent DDL.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Recipe schema, structs, and migration

**Files:**
- Create: `internal/recipe/recipe.go`
- Create: `internal/recipe/schema.sql`
- Create: `internal/recipe/migrate.go`
- Test: `internal/recipe/repo_test.go` (created here, grown in later tasks)

**Interfaces:**
- Consumes: `db.ApplySchema` from Task 1.
- Produces:
  - Type `Recipe{ ID, Name, Instructions string; Servings, TotalMinutes int; Ingredients []RecipeIngredient }`.
  - Type `RecipeIngredient{ IngredientID, Name string; Quantity float64; Unit string }`.
  - `func Migrate(database *sql.DB) error` — creates recipe tables (idempotent).

- [ ] **Step 1: Write the failing test**

Create `internal/recipe/repo_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/recipe/`
Expected: FAIL — `undefined: Migrate` (package doesn't compile).

- [ ] **Step 3: Write the domain structs**

Create `internal/recipe/recipe.go`:
```go
// Package recipe is a self-contained domain slice: it owns its schema, seed
// data, and a hand-written SQL repository. Repo structs are plain Go and are
// mapped to proto in the service layer if a service is ever exposed.
package recipe

// Recipe is a recipe with its assembled ingredient lines.
type Recipe struct {
	ID           string
	Name         string
	Instructions string
	Servings     int
	TotalMinutes int
	Ingredients  []RecipeIngredient
}

// RecipeIngredient is one ingredient line on a recipe (ingredient name joined
// in for convenience).
type RecipeIngredient struct {
	IngredientID string
	Name         string
	Quantity     float64
	Unit         string
}
```

- [ ] **Step 4: Write the schema**

Create `internal/recipe/schema.sql`:
```sql
CREATE TABLE IF NOT EXISTS recipe (
  id            TEXT PRIMARY KEY,
  name          TEXT NOT NULL,
  instructions  TEXT NOT NULL DEFAULT '',
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
```

- [ ] **Step 5: Write the migration**

Create `internal/recipe/migrate.go`:
```go
package recipe

import (
	"database/sql"
	_ "embed"

	"github.com/sethlowie/dinnerwise/internal/db"
)

//go:embed schema.sql
var schema string

// Migrate creates the recipe tables if they do not exist. Idempotent.
func Migrate(database *sql.DB) error {
	return db.ApplySchema(database, schema)
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/recipe/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/recipe/
git commit -m "feat: add recipe schema, structs, and migration

recipe/ingredient/recipe_ingredient tables (embedded schema.sql), plain Go
domain structs, idempotent Migrate.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Recipe repository (List, GetByID)

**Files:**
- Create: `internal/recipe/repo.go`
- Test: `internal/recipe/repo_test.go` (append)

**Interfaces:**
- Consumes: `Recipe`, `RecipeIngredient`, `Migrate` from Task 2; `db.Open` from Task 1.
- Produces:
  - `var ErrNotFound error`
  - `func NewRepo(database *sql.DB) *Repo`
  - `func (r *Repo) List(ctx context.Context) ([]Recipe, error)` — all recipes ordered by name, ingredients assembled.
  - `func (r *Repo) GetByID(ctx context.Context, id string) (Recipe, error)` — one recipe; `ErrNotFound` if absent.

- [ ] **Step 1: Write the failing test**

Append to `internal/recipe/repo_test.go`:
```go
import (
	"context"
	"errors"
)

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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/recipe/`
Expected: FAIL — `undefined: NewRepo` / `undefined: ErrNotFound`.

- [ ] **Step 3: Write minimal implementation**

Create `internal/recipe/repo.go`:
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/recipe/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/recipe/repo.go internal/recipe/repo_test.go
git commit -m "feat: add recipe repository with List and GetByID

Hand-written SQL, N+1-free ingredient assembly, ErrNotFound for missing ids.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Fixture seeding

**Files:**
- Create: `internal/recipe/fixtures/recipes.json`
- Create: `internal/recipe/seed.go`
- Test: `internal/recipe/repo_test.go` (append)

**Interfaces:**
- Consumes: the recipe tables (Task 2), `Repo.List` (Task 3).
- Produces:
  - `func SeedIfEmpty(database *sql.DB) error` — inserts fixtures only when `recipe` is empty; idempotent.

- [ ] **Step 1: Write the fixtures**

Create `internal/recipe/fixtures/recipes.json` (note `olive-oil` is shared, exercising the ingredient conflict path):
```json
[
  {
    "id": "sheet-pan-chicken",
    "name": "Sheet-Pan Chicken & Veg",
    "instructions": "Toss everything with oil and roast at 220C for 30 minutes.",
    "servings": 4,
    "totalMinutes": 40,
    "ingredients": [
      { "id": "chicken-thigh", "name": "Chicken thighs", "quantity": 800, "unit": "g" },
      { "id": "broccoli", "name": "Broccoli", "quantity": 1, "unit": "head" },
      { "id": "olive-oil", "name": "Olive oil", "quantity": 2, "unit": "tbsp" }
    ]
  },
  {
    "id": "tomato-pasta",
    "name": "Weeknight Tomato Pasta",
    "instructions": "Simmer the sauce, cook the pasta, then combine.",
    "servings": 4,
    "totalMinutes": 25,
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
    "instructions": "Fry the tofu, add the veg and sauce, then serve over rice.",
    "servings": 3,
    "totalMinutes": 30,
    "ingredients": [
      { "id": "tofu", "name": "Firm tofu", "quantity": 400, "unit": "g" },
      { "id": "bell-pepper", "name": "Bell pepper", "quantity": 2, "unit": "whole" },
      { "id": "soy-sauce", "name": "Soy sauce", "quantity": 3, "unit": "tbsp" },
      { "id": "rice", "name": "Rice", "quantity": 300, "unit": "g" }
    ]
  }
]
```

- [ ] **Step 2: Write the failing test**

Append to `internal/recipe/repo_test.go`:
```go
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
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/recipe/`
Expected: FAIL — `undefined: SeedIfEmpty`.

- [ ] **Step 4: Write minimal implementation**

Create `internal/recipe/seed.go`:
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
// idempotent and safe to call on every startup. All inserts run in one
// transaction; shared ingredients are de-duplicated via ON CONFLICT DO NOTHING.
func SeedIfEmpty(database *sql.DB) error {
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM recipe`).Scan(&count); err != nil {
		return fmt.Errorf("count recipes: %w", err)
	}
	if count > 0 {
		return nil
	}

	var seeds []seedRecipe
	if err := json.Unmarshal(recipesFixture, &seeds); err != nil {
		return fmt.Errorf("parse fixtures: %w", err)
	}

	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("begin seed tx: %w", err)
	}
	defer tx.Rollback()

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
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/recipe/`
Expected: PASS (all recipe tests).

- [ ] **Step 6: Commit**

```bash
git add internal/recipe/seed.go internal/recipe/fixtures/ internal/recipe/repo_test.go
git commit -m "feat: seed recipes from embedded JSON fixtures

SeedIfEmpty inserts fixtures in a transaction only when empty; shared
ingredients de-duplicate via ON CONFLICT. Tests cover idempotency and FK
cascade.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: Wire persistence into the server

**Files:**
- Modify: `cmd/server/main.go`
- Modify: `.gitignore`

**Interfaces:**
- Consumes: `db.Open` (Task 1), `recipe.Migrate`, `recipe.SeedIfEmpty`, `recipe.NewRepo().List` (Tasks 2–4).
- Produces: a server that opens, migrates, seeds, and logs the recipe count at boot.

- [ ] **Step 1: Ignore the database files**

Append to `.gitignore`:
```gitignore

# SQLite database files
*.db
*.db-wal
*.db-shm
```

- [ ] **Step 2: Wire db open/migrate/seed into main**

Edit `cmd/server/main.go`. Replace the imports block:
```go
import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/sethlowie/dinnerwise/internal/db"
	"github.com/sethlowie/dinnerwise/internal/foo"
	"github.com/sethlowie/dinnerwise/internal/foo/v1/foov1connect"
	"github.com/sethlowie/dinnerwise/internal/recipe"
)
```

Replace the body of `func main()` (everything from `addr :=` through the `ListenAndServe` block) with:
```go
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	dbPath := os.Getenv("DINNERWISE_DB")
	if dbPath == "" {
		dbPath = "dinnerwise.db"
	}

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("server: open db: %v", err)
	}
	defer database.Close()

	if err := recipe.Migrate(database); err != nil {
		log.Fatalf("server: migrate: %v", err)
	}
	if err := recipe.SeedIfEmpty(database); err != nil {
		log.Fatalf("server: seed: %v", err)
	}
	recipes, err := recipe.NewRepo(database).List(context.Background())
	if err != nil {
		log.Fatalf("server: list recipes: %v", err)
	}
	log.Printf("server: %d recipes loaded from %s", len(recipes), dbPath)

	mux := http.NewServeMux()
	mux.Handle(foov1connect.NewFooServiceHandler(foo.NewService()))

	log.Printf("server: listening on %s", addr)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		log.Fatalf("server: %v", err)
	}
```

- [ ] **Step 3: Verify the build and full test suite**

Run: `go build ./... && go test ./...`
Expected: build succeeds; `ok` for `internal/db` and `internal/recipe`.

- [ ] **Step 4: Verify the server boots and seeds**

Run:
```bash
DINNERWISE_DB="$(mktemp -d)/dinnerwise.db" ADDR=:8099 go run ./cmd/server/ &
sleep 2
curl -s -X POST http://localhost:8099/internal.foo.v1.FooService/GetFoo \
  -H "Content-Type: application/json" -H "Connect-Protocol-Version: 1" -d '{"id":"x"}'
echo
kill %1 2>/dev/null
```
Expected: a boot log line `server: 3 recipes loaded from ...`, and the Foo round trip returns `{"data":{"foo":"hello from foo x"}}`.

- [ ] **Step 5: Confirm no database file was committed**

Run: `git status --porcelain | grep -E '\.db' || echo "clean"`
Expected: `clean`.

- [ ] **Step 6: Commit**

```bash
git add cmd/server/main.go .gitignore
git commit -m "feat: wire SQLite persistence into the server

Open/migrate/seed at boot, log recipe count; ignore *.db files. DB path from
DINNERWISE_DB (default ./dinnerwise.db).

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage:**
- Pure-Go SQLite / no CGO → Task 1 (modernc driver, Global Constraints).
- stdlib + hand-written SQL, no ORM/codegen → Tasks 1, 3.
- Layering (`internal/db` generic, `internal/recipe` self-contained, main wires) → Tasks 1, 2, 5.
- Pragmas via DSN (foreign_keys/WAL/busy_timeout) → Task 1, verified in `TestOpenEnablesForeignKeys`.
- Idempotent per-domain schema-at-startup → Task 2 (`Migrate`, `TestMigrateIsIdempotent...`).
- Recipe schema (3 tables, slug IDs, lean ingredient) → Task 2.
- Repo `List` (N+1-free) + `GetByID` + not-found → Task 3.
- Seed from embedded JSON, seed-if-empty, dedupe shared ingredients → Task 4.
- Config (`DINNERWISE_DB`) + gitignore `*.db*` → Task 5.
- Testing: temp DB per test, idempotency, assembly, not-found, FK cascade → Tasks 1–4.
- Out of scope (no Connect service, no enrichment/other domains) → respected; main only lists for a boot log.

**Placeholder scan:** none — every code/step is concrete.

**Type consistency:** `db.Open`, `db.ApplySchema`, `recipe.Migrate`, `recipe.SeedIfEmpty`, `recipe.NewRepo`, `Repo.List`, `Repo.GetByID`, `ErrNotFound`, and struct fields (`Recipe`, `RecipeIngredient`) are used identically across tasks and the main wiring. Fixture JSON keys (`totalMinutes`) match the `seedRecipe` tags.
