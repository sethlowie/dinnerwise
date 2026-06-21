# Meals Domain (Slice 2) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a logged-meals domain (MealService over SQLite with a derived-count cook log, server-side sort/filter, meal→recipe link) plus Sous-styled `/meals` and `/meals/$id` screens.

**Architecture:** Mirror the recipe slice: `internal/meal` owns its schema/structs/repo/seed, a `meal/v1` proto + a `package meal` service mapping the repo onto proto. Counts are derived (`COUNT`/`MAX`) from a `meal_cook` log. `ListMeals` sorts/filters server-side; the `/meals` route's typed URL params drive the request. The server migrates/seeds recipes before meals (meals FK recipes).

**Tech Stack:** Go 1.25, `database/sql`/SQLite (`modernc.org/sqlite`), ConnectRPC, buf v2, React 19 + Vite, TanStack Router, Connect-Query, Tailwind v4.

## Global Constraints

- stdlib `database/sql` + hand-written SQL; no ORM/codegen. Pure-Go driver via existing `internal/db`.
- Generated code is gitignored (`make gen`); commit only hand-written files. Proto Go pkg imported as `mealv1 "github.com/sethlowie/dinnerwise/internal/meal/v1"`; connect pkg `mealv1connect`.
- Counts are derived (no stored `times_cooked`/`last_cooked`); seeder writes the full cook log.
- `ListMeals` is server-side sort/filter; the route passes URL params (`sort`, `fav`) as request args.
- Seed is idempotent (insert only when `meal` is empty, one transaction); dates computed from fixture ISO dates (no `time.Now()` in seed).
- Recipe migrate+seed must run before meal migrate+seed (FK). Meal tests seed recipes first.
- Frontend: no unit-test harness — verify with `tsc -b`/`eslint`/`vite build` + runtime. Route files covered by the `src/routes/**` eslint override. No `any`.
- Module path `github.com/sethlowie/dinnerwise`. Commit messages end with: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

---

## File Structure

- Create: `internal/meal/v1/meal.proto`, `internal/meal/meal.go`, `internal/meal/schema.sql`, `internal/meal/migrate.go`, `internal/meal/repo.go`, `internal/meal/seed.go`, `internal/meal/fixtures/meals.json`, `internal/meal/repo_test.go`, `internal/meal/service.go`, `internal/meal/service_test.go`.
- Modify: `cmd/server/main.go` (wire meal migrate/seed/handler).
- Create: `web/app/src/routes/meals.tsx`, `web/app/src/routes/meal.tsx`.
- Modify: `web/app/src/router.tsx`, `web/app/src/chat/Sidebar.tsx`.

---

## Task 1: Meal schema, structs, and migration

**Files:** Create `internal/meal/meal.go`, `internal/meal/schema.sql`, `internal/meal/migrate.go`; Test `internal/meal/repo_test.go`.

**Interfaces:**
- Consumes: `db.Open`, `db.ApplySchema`; `recipe.Migrate`, `recipe.SeedIfEmpty` (test setup, FK targets).
- Produces: `Meal`, `Cook` structs; `func Migrate(*sql.DB) error`; test helper `newTestDB(t) *sql.DB`.

- [ ] **Step 1: Write the failing test**

Create `internal/meal/repo_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/meal/`
Expected: FAIL — `undefined: Migrate`.

- [ ] **Step 3: Write the structs**

Create `internal/meal/meal.go`:
```go
// Package meal is the logged-meals domain slice: schema, repo, seed, and a
// MealService mapping. Cook counts are derived from the meal_cook log.
package meal

// Meal is a logged meal with derived cook stats.
type Meal struct {
	ID          string
	Name        string
	Cuisine     string
	Rating      int
	TimesCooked int
	LastCooked  string // "" if never cooked
	RecipeID    string // "" if no linked recipe
}

// Cook is one entry in a meal's cook log.
type Cook struct {
	CookedOn string
	Note     string
}
```

- [ ] **Step 4: Write the schema**

Create `internal/meal/schema.sql`:
```sql
CREATE TABLE IF NOT EXISTS meal (
  id        TEXT PRIMARY KEY,
  name      TEXT NOT NULL,
  cuisine   TEXT NOT NULL DEFAULT '',
  rating    INTEGER NOT NULL DEFAULT 0,
  recipe_id TEXT REFERENCES recipe(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS meal_cook (
  id        INTEGER PRIMARY KEY,
  meal_id   TEXT NOT NULL REFERENCES meal(id) ON DELETE CASCADE,
  cooked_on TEXT NOT NULL,
  note      TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_meal_cook_meal ON meal_cook(meal_id);
```

- [ ] **Step 5: Write the migration**

Create `internal/meal/migrate.go`:
```go
package meal

import (
	"database/sql"
	_ "embed"

	"github.com/sethlowie/dinnerwise/internal/db"
)

//go:embed schema.sql
var schema string

// Migrate creates the meal tables if they do not exist. Idempotent. Requires
// the recipe table to already exist (meal.recipe_id references it).
func Migrate(database *sql.DB) error {
	return db.ApplySchema(database, schema)
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/meal/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/meal/meal.go internal/meal/schema.sql internal/meal/migrate.go internal/meal/repo_test.go
git commit -m "feat: add meal schema, structs, and migration

meal + meal_cook tables (recipe_id FK, cook log); plain Go structs; idempotent
Migrate. Test helper seeds recipes first so FKs resolve.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Meal repo (List, GetByID)

**Files:** Create `internal/meal/repo.go`; Test `internal/meal/repo_test.go` (append).

**Interfaces:**
- Consumes: `Meal`, `Cook`, `Migrate`, `newTestDB` (Task 1).
- Produces: `var ErrNotFound`; `func NewRepo(*sql.DB) *Repo`; `func (*Repo) List(ctx, sort string, favoritesOnly bool) ([]Meal, error)`; `func (*Repo) GetByID(ctx, id string) (Meal, []Cook, error)`.

- [ ] **Step 1: Write the failing test**

Append to `internal/meal/repo_test.go`:
```go
import (
	"context"
	"errors"
)

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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/meal/ -run 'TestList|TestGetByID'`
Expected: FAIL — `undefined: NewRepo` / `ErrNotFound`.

- [ ] **Step 3: Write the repo**

Create `internal/meal/repo.go`:
```go
package meal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrNotFound is returned by GetByID when no meal matches the id.
var ErrNotFound = errors.New("meal not found")

// Repo is a thin hand-written SQL repository over the meal tables.
type Repo struct {
	db *sql.DB
}

func NewRepo(database *sql.DB) *Repo {
	return &Repo{db: database}
}

// List returns meals with derived cook stats. sort is "recent" (default) or
// "rating"; favoritesOnly keeps rating>=4. Sort/filter are server-side.
func (r *Repo) List(ctx context.Context, sort string, favoritesOnly bool) ([]Meal, error) {
	where := ""
	if favoritesOnly {
		where = "WHERE m.rating >= 4"
	}
	order := "ORDER BY last_cooked DESC, m.name"
	if sort == "rating" {
		order = "ORDER BY m.rating DESC, times_cooked DESC, m.name"
	}
	query := fmt.Sprintf(`
		SELECT m.id, m.name, m.cuisine, m.rating, COALESCE(m.recipe_id, ''),
		       COUNT(mc.id) AS times_cooked, COALESCE(MAX(mc.cooked_on), '') AS last_cooked
		FROM meal m
		LEFT JOIN meal_cook mc ON mc.meal_id = m.id
		%s
		GROUP BY m.id
		%s`, where, order)

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query meals: %w", err)
	}
	defer rows.Close()

	meals := []Meal{}
	for rows.Next() {
		var m Meal
		if err := rows.Scan(
			&m.ID, &m.Name, &m.Cuisine, &m.Rating, &m.RecipeID, &m.TimesCooked, &m.LastCooked,
		); err != nil {
			return nil, fmt.Errorf("scan meal: %w", err)
		}
		meals = append(meals, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate meals: %w", err)
	}
	return meals, nil
}

// GetByID returns one meal with derived stats plus its recent cooks
// (newest first), or ErrNotFound.
func (r *Repo) GetByID(ctx context.Context, id string) (Meal, []Cook, error) {
	var m Meal
	err := r.db.QueryRowContext(ctx, `
		SELECT m.id, m.name, m.cuisine, m.rating, COALESCE(m.recipe_id, ''),
		       COUNT(mc.id), COALESCE(MAX(mc.cooked_on), '')
		FROM meal m
		LEFT JOIN meal_cook mc ON mc.meal_id = m.id
		WHERE m.id = ?
		GROUP BY m.id`, id).
		Scan(&m.ID, &m.Name, &m.Cuisine, &m.Rating, &m.RecipeID, &m.TimesCooked, &m.LastCooked)
	if errors.Is(err, sql.ErrNoRows) {
		return Meal{}, nil, ErrNotFound
	}
	if err != nil {
		return Meal{}, nil, fmt.Errorf("query meal %q: %w", id, err)
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT cooked_on, note FROM meal_cook WHERE meal_id = ? ORDER BY cooked_on DESC LIMIT 5`, id)
	if err != nil {
		return Meal{}, nil, fmt.Errorf("query cooks for %q: %w", id, err)
	}
	defer rows.Close()
	cooks := []Cook{}
	for rows.Next() {
		var c Cook
		if err := rows.Scan(&c.CookedOn, &c.Note); err != nil {
			return Meal{}, nil, fmt.Errorf("scan cook: %w", err)
		}
		cooks = append(cooks, c)
	}
	if err := rows.Err(); err != nil {
		return Meal{}, nil, fmt.Errorf("iterate cooks: %w", err)
	}
	return m, cooks, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/meal/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/meal/repo.go internal/meal/repo_test.go
git commit -m "feat: add meal repo with derived-count List and GetByID

Server-side sort (recent/rating) + favorites filter; counts derived via
COUNT/MAX over the cook log; GetByID returns recent cooks + recipe link.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Meal seed (fixtures + generated cook log)

**Files:** Create `internal/meal/fixtures/meals.json`, `internal/meal/seed.go`; Test `internal/meal/repo_test.go` (append).

**Interfaces:**
- Consumes: meal tables (Task 1), `Repo.List`/`GetByID` (Task 2).
- Produces: `func SeedIfEmpty(*sql.DB) error`.

- [ ] **Step 1: Write the fixtures**

Create `internal/meal/fixtures/meals.json`:
```json
[
  { "id": "salmon", "name": "Miso-Glazed Salmon", "cuisine": "Japanese", "rating": 5, "timesCooked": 7, "last": "2026-06-18", "recipeId": "" },
  { "id": "pasta", "name": "Sun-Dried Tomato Pasta", "cuisine": "Italian", "rating": 5, "timesCooked": 5, "last": "2026-06-15", "recipeId": "tomato-pasta" },
  { "id": "curry", "name": "Thai Green Curry", "cuisine": "Thai", "rating": 5, "timesCooked": 4, "last": "2026-06-09", "recipeId": "" },
  { "id": "beef", "name": "Korean Beef Bowl", "cuisine": "Korean", "rating": 4, "timesCooked": 6, "last": "2026-06-12", "recipeId": "" },
  { "id": "chicken", "name": "Lemon Herb Roast Chicken", "cuisine": "American", "rating": 4, "timesCooked": 3, "last": "2026-06-06", "recipeId": "sheet-pan-chicken" },
  { "id": "shakshuka", "name": "Shakshuka", "cuisine": "Middle Eastern", "rating": 4, "timesCooked": 8, "last": "2026-06-03", "recipeId": "" },
  { "id": "tacos", "name": "Black Bean Tacos", "cuisine": "Mexican", "rating": 4, "timesCooked": 5, "last": "2026-05-30", "recipeId": "" },
  { "id": "pizza", "name": "Margherita Pizza", "cuisine": "Italian", "rating": 3, "timesCooked": 2, "last": "2026-05-28", "recipeId": "" },
  { "id": "caesar", "name": "Caesar Salad", "cuisine": "American", "rating": 3, "timesCooked": 4, "last": "2026-05-25", "recipeId": "" },
  { "id": "stirfry", "name": "Veggie Stir-Fry", "cuisine": "Chinese", "rating": 3, "timesCooked": 3, "last": "2026-05-20", "recipeId": "veggie-stir-fry" },
  { "id": "oats", "name": "Overnight Oats", "cuisine": "Breakfast", "rating": 2, "timesCooked": 9, "last": "2026-05-19", "recipeId": "" }
]
```

- [ ] **Step 2: Write the failing test**

Append to `internal/meal/repo_test.go`:
```go
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
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/meal/ -run TestSeed`
Expected: FAIL — `undefined: SeedIfEmpty`.

- [ ] **Step 4: Write the seed**

Create `internal/meal/seed.go`:
```go
package meal

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"
)

//go:embed fixtures/meals.json
var mealsFixture []byte

type seedMeal struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Cuisine     string `json:"cuisine"`
	Rating      int    `json:"rating"`
	TimesCooked int    `json:"timesCooked"`
	Last        string `json:"last"`     // ISO date "2006-01-02"
	RecipeID    string `json:"recipeId"` // "" if none
}

var cookNotes = []string{"Weeknight dinner", "Doubled the batch", "Extra garlic"}

// SeedIfEmpty loads fixture meals only when the meal table is empty. It is
// idempotent. Because cook counts are derived, it generates the full cook log:
// for a meal with timesCooked n and last date D, it writes n rows dated weekly
// back from D, attaching a rotating note to the most recent few. All inserts
// run in one transaction.
func SeedIfEmpty(database *sql.DB) error {
	var seeds []seedMeal
	if err := json.Unmarshal(mealsFixture, &seeds); err != nil {
		return fmt.Errorf("parse fixtures: %w", err)
	}

	tx, err := database.Begin()
	if err != nil {
		return fmt.Errorf("begin seed tx: %w", err)
	}
	defer tx.Rollback()

	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM meal`).Scan(&count); err != nil {
		return fmt.Errorf("count meals: %w", err)
	}
	if count > 0 {
		return nil
	}

	for _, m := range seeds {
		var recipeID any
		if m.RecipeID != "" {
			recipeID = m.RecipeID
		}
		if _, err := tx.Exec(
			`INSERT INTO meal (id, name, cuisine, rating, recipe_id) VALUES (?, ?, ?, ?, ?)`,
			m.ID, m.Name, m.Cuisine, m.Rating, recipeID,
		); err != nil {
			return fmt.Errorf("insert meal %q: %w", m.ID, err)
		}

		last, err := time.Parse("2006-01-02", m.Last)
		if err != nil {
			return fmt.Errorf("parse last for %q: %w", m.ID, err)
		}
		for i := 0; i < m.TimesCooked; i++ {
			day := last.AddDate(0, 0, -7*i).Format("2006-01-02")
			note := ""
			if i < len(cookNotes) {
				note = cookNotes[i]
			}
			if _, err := tx.Exec(
				`INSERT INTO meal_cook (meal_id, cooked_on, note) VALUES (?, ?, ?)`,
				m.ID, day, note,
			); err != nil {
				return fmt.Errorf("insert cook for %q: %w", m.ID, err)
			}
		}
	}
	return tx.Commit()
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go vet ./internal/meal/ && go test ./internal/meal/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/meal/seed.go internal/meal/fixtures/ internal/meal/repo_test.go
git commit -m "feat: seed meals and generate the cook log

Ports the 11 design meals; generates timesCooked-many cook rows weekly back
from each meal's last date so derived COUNT/MAX match. Idempotent.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: MealService proto + service

**Files:** Create `internal/meal/v1/meal.proto`, `internal/meal/service.go`; Test `internal/meal/service_test.go`.

**Interfaces:**
- Consumes: `Repo`, `NewRepo`, `SeedIfEmpty`, `newTestDB`, `Meal`, `Cook`, `ErrNotFound`.
- Produces: generated `mealv1` types + `mealv1connect.MealServiceHandler`/`NewMealServiceHandler`; `func NewService(repo *Repo) mealv1connect.MealServiceHandler`.

- [ ] **Step 1: Write the proto**

Create `internal/meal/v1/meal.proto`:
```proto
syntax = "proto3";

package internal.meal.v1;

message Meal {
  string id = 1;
  string name = 2;
  string cuisine = 3;
  int32 rating = 4;
  int32 times_cooked = 5;
  string last_cooked = 6;
  string recipe_id = 7;
}

message Cook {
  string cooked_on = 1;
  string note = 2;
}

message ListMealsRequest {
  string sort = 1;
  bool favorites_only = 2;
}
message ListMealsResponse { repeated Meal meals = 1; }

message GetMealRequest { string id = 1; }
message GetMealResponse {
  Meal meal = 1;
  repeated Cook recent_cooks = 2;
}

service MealService {
  rpc ListMeals(ListMealsRequest) returns (ListMealsResponse);
  rpc GetMeal(GetMealRequest) returns (GetMealResponse);
}
```

- [ ] **Step 2: Generate code**

Run: `make gen`
Expected: `internal/meal/v1/meal.pb.go`, `internal/meal/v1/mealv1connect/meal.connect.go`, and `web/app/src/gen/internal/meal/v1/*.ts` exist.
Verify: `ls internal/meal/v1/mealv1connect/meal.connect.go web/app/src/gen/internal/meal/v1/`

- [ ] **Step 3: Write the failing test**

Create `internal/meal/service_test.go`:
```go
package meal

import (
	"context"
	"testing"

	connect "connectrpc.com/connect"
	mealv1 "github.com/sethlowie/dinnerwise/internal/meal/v1"
)

func newSeededService(t *testing.T) *Service {
	t.Helper()
	database := newTestDB(t)
	if err := SeedIfEmpty(database); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return &Service{repo: NewRepo(database)}
}

func TestServiceListMealsFavoritesByRating(t *testing.T) {
	svc := newSeededService(t)
	resp, err := svc.ListMeals(context.Background(),
		connect.NewRequest(&mealv1.ListMealsRequest{Sort: "rating", FavoritesOnly: true}))
	if err != nil {
		t.Fatalf("ListMeals: %v", err)
	}
	if len(resp.Msg.Meals) != 7 { // design: 7 meals rated >= 4
		t.Fatalf("favorites = %d, want 7", len(resp.Msg.Meals))
	}
	for _, m := range resp.Msg.Meals {
		if m.GetRating() < 4 {
			t.Fatalf("non-favorite leaked: %+v", m)
		}
	}
	// top by rating is one of the 5-star meals
	if resp.Msg.Meals[0].GetRating() != 5 {
		t.Fatalf("top rating = %d, want 5", resp.Msg.Meals[0].GetRating())
	}
}

func TestServiceGetMeal(t *testing.T) {
	svc := newSeededService(t)
	resp, err := svc.GetMeal(context.Background(),
		connect.NewRequest(&mealv1.GetMealRequest{Id: "salmon"}))
	if err != nil {
		t.Fatalf("GetMeal: %v", err)
	}
	if resp.Msg.Meal.GetTimesCooked() != 7 {
		t.Fatalf("times_cooked = %d, want 7", resp.Msg.Meal.GetTimesCooked())
	}
	if len(resp.Msg.RecentCooks) == 0 {
		t.Fatal("expected recent cooks")
	}
}

func TestServiceGetMealNotFound(t *testing.T) {
	svc := newSeededService(t)
	_, err := svc.GetMeal(context.Background(),
		connect.NewRequest(&mealv1.GetMealRequest{Id: "nope"}))
	if got := connect.CodeOf(err); got != connect.CodeNotFound {
		t.Fatalf("code = %v, want NotFound", got)
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/meal/ -run TestService`
Expected: FAIL — `undefined: Service`.

- [ ] **Step 5: Write the service**

Create `internal/meal/service.go`:
```go
package meal

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	mealv1 "github.com/sethlowie/dinnerwise/internal/meal/v1"
	"github.com/sethlowie/dinnerwise/internal/meal/v1/mealv1connect"
)

// Service implements mealv1connect.MealServiceHandler by mapping the domain
// Repo onto proto. Sort/filter happen in the repo (server-side).
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) mealv1connect.MealServiceHandler {
	return &Service{repo: repo}
}

func (s *Service) ListMeals(
	ctx context.Context,
	req *connect.Request[mealv1.ListMealsRequest],
) (*connect.Response[mealv1.ListMealsResponse], error) {
	meals, err := s.repo.List(ctx, req.Msg.GetSort(), req.Msg.GetFavoritesOnly())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*mealv1.Meal, len(meals))
	for i := range meals {
		out[i] = toProtoMeal(meals[i])
	}
	return connect.NewResponse(&mealv1.ListMealsResponse{Meals: out}), nil
}

func (s *Service) GetMeal(
	ctx context.Context,
	req *connect.Request[mealv1.GetMealRequest],
) (*connect.Response[mealv1.GetMealResponse], error) {
	m, cooks, err := s.repo.GetByID(ctx, req.Msg.GetId())
	if errors.Is(err, ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	recent := make([]*mealv1.Cook, len(cooks))
	for i, c := range cooks {
		recent[i] = &mealv1.Cook{CookedOn: c.CookedOn, Note: c.Note}
	}
	return connect.NewResponse(&mealv1.GetMealResponse{
		Meal:        toProtoMeal(m),
		RecentCooks: recent,
	}), nil
}

func toProtoMeal(m Meal) *mealv1.Meal {
	return &mealv1.Meal{
		Id:          m.ID,
		Name:        m.Name,
		Cuisine:     m.Cuisine,
		Rating:      int32(m.Rating),
		TimesCooked: int32(m.TimesCooked),
		LastCooked:  m.LastCooked,
		RecipeId:    m.RecipeID,
	}
}
```

- [ ] **Step 6: Run tests + vet**

Run: `go vet ./internal/meal/ && go test ./internal/meal/`
Expected: PASS.

- [ ] **Step 7: Commit**

Generated code is gitignored; commit only the proto + hand-written Go.
```bash
git add internal/meal/v1/meal.proto internal/meal/service.go internal/meal/service_test.go
git commit -m "feat: add MealService (ListMeals, GetMeal)

Proto + Connect service mapping the meal repo to proto; server-side sort/
favorites pass through; GetMeal maps ErrNotFound to CodeNotFound.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: Wire MealService into the server

**Files:** Modify `cmd/server/main.go`.

**Interfaces:**
- Consumes: `meal.Migrate`, `meal.SeedIfEmpty`, `meal.NewRepo`, `meal.NewService`, `mealv1connect.NewMealServiceHandler`.
- Produces: server migrates/seeds meals (after recipes) and mounts MealService.

- [ ] **Step 1: Add imports**

In `cmd/server/main.go`, add to the import block:
```go
	"github.com/sethlowie/dinnerwise/internal/meal"
	"github.com/sethlowie/dinnerwise/internal/meal/v1/mealv1connect"
```

- [ ] **Step 2: Migrate + seed meals after recipes**

In `cmd/server/main.go`, immediately after the existing `recipe.SeedIfEmpty(database)` error-check block, add:
```go
	if err := meal.Migrate(database); err != nil {
		log.Fatalf("server: meal migrate: %v", err)
	}
	if err := meal.SeedIfEmpty(database); err != nil {
		log.Fatalf("server: meal seed: %v", err)
	}
```

- [ ] **Step 3: Mount the handler**

In `cmd/server/main.go`, immediately after the existing `mux.Handle(agentv1connect.NewAgentServiceHandler(agent.NewService()))` line, add:
```go
	mux.Handle(mealv1connect.NewMealServiceHandler(meal.NewService(meal.NewRepo(database))))
```

- [ ] **Step 4: Verify build, full suite, boot**

Run:
```bash
go build ./... && go test ./... 2>&1 | grep -E "ok|FAIL"
DINNERWISE_DB="$(mktemp -d)/dinnerwise.db" ADDR=:8091 go run ./cmd/server/ &
sleep 2
curl -s -X POST http://localhost:8091/internal.meal.v1.MealService/ListMeals \
  -H "Content-Type: application/json" -H "Connect-Protocol-Version: 1" \
  -d '{"sort":"rating","favoritesOnly":true}' | head -c 200
echo
kill %1 2>/dev/null
```
Expected: build + `ok` for `internal/meal` (and others); JSON response with a `meals` array (7 favorites). If port busy, pick another.

- [ ] **Step 5: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: mount MealService and seed meals at boot

Migrate + seed meals after recipes (FK order); mount the MealService handler.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Meals UI (/meals + /meals/$id) + sidebar

**Files:** Create `web/app/src/routes/meals.tsx`, `web/app/src/routes/meal.tsx`; Modify `web/app/src/router.tsx`, `web/app/src/chat/Sidebar.tsx`.

**Interfaces:**
- Consumes: generated `listMeals`/`getMeal` from `../gen/internal/meal/v1/meal-MealService_connectquery`; `rootRoute`; `lib/thumb` (`tintFor`, `initials`, `thumbStyle`).
- Produces: `mealsRoute` (`/meals`, typed search `{ sort?, fav? }`), `mealDetailRoute` (`/meals/$id`) registered in `router.tsx`; sidebar Meals link.

Note: no frontend unit tests — verify with `tsc -b`/`eslint`/`vite build` + runtime.

- [ ] **Step 1: Create the meals list route**

Create `web/app/src/routes/meals.tsx`:
```tsx
import { createRoute, getRouteApi, Link } from "@tanstack/react-router";
import { useQuery } from "@connectrpc/connect-query";
import { listMeals } from "../gen/internal/meal/v1/meal-MealService_connectquery";
import { rootRoute } from "./__root";
import { initials, thumbStyle, tintFor } from "../lib/thumb";

const routeApi = getRouteApi("/meals");

function Meals() {
  const { sort, fav } = routeApi.useSearch();
  const navigate = routeApi.useNavigate();
  const activeSort = sort ?? "recent";
  const { data, error, isPending } = useQuery(listMeals, {
    sort: activeSort,
    favoritesOnly: fav ?? false,
  });

  if (isPending) return <p className="text-muted-foreground">Loading…</p>;
  if (error)
    return <p className="text-red-600 dark:text-red-400">{error.message}</p>;

  return (
    <div className="space-y-6">
      <div>
        <div className="mb-2 font-mono text-xs uppercase tracking-[0.12em] text-muted-foreground">
          Your kitchen
        </div>
        <h1 className="text-3xl font-semibold tracking-tight">Meals</h1>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <button
          onClick={() =>
            navigate({
              search: (p) => ({ ...p, sort: activeSort === "recent" ? "rating" : "recent" }),
            })
          }
          className="rounded-xl border border-border bg-muted/40 px-3 py-1.5 text-sm hover:border-primary/40"
        >
          <span className="font-mono text-xs uppercase text-muted-foreground">sort </span>
          {activeSort === "rating" ? "Rating" : "Recent"}
        </button>
        <button
          onClick={() => navigate({ search: (p) => ({ ...p, fav: fav ? undefined : true }) })}
          className={`rounded-xl border px-3 py-1.5 text-sm ${
            fav
              ? "border-primary/40 bg-accent text-accent-foreground"
              : "border-border bg-muted/40 hover:border-primary/40"
          }`}
        >
          <span className="mr-1 text-amber-400">★</span>Favorites
        </button>
      </div>

      <div className="flex flex-col gap-2.5">
        {data.meals.map((m) => {
          const tint = tintFor(m.id);
          return (
            <Link
              key={m.id}
              to="/meals/$id"
              params={{ id: m.id }}
              className="flex items-center gap-3.5 rounded-2xl border border-border bg-card/60 p-4 transition-colors hover:border-primary/40"
            >
              <div
                className="flex h-11 w-11 flex-none items-center justify-center rounded-xl font-mono text-sm font-semibold"
                style={thumbStyle(tint)}
              >
                {initials(m.name)}
              </div>
              <div className="min-w-0 flex-1">
                <div className="truncate font-medium">{m.name}</div>
                <div className="mt-0.5 truncate font-mono text-xs text-muted-foreground">
                  {m.cuisine} · cooked {m.timesCooked}×
                </div>
              </div>
              <div className="flex-none text-right">
                <div className="text-sm tracking-widest">
                  <span className="text-amber-400">{"★".repeat(m.rating)}</span>
                  <span className="text-muted-foreground/40">{"★".repeat(5 - m.rating)}</span>
                </div>
                <div className="mt-1 font-mono text-xs text-muted-foreground">
                  {m.lastCooked || "—"}
                </div>
              </div>
            </Link>
          );
        })}
      </div>
    </div>
  );
}

export const mealsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/meals",
  validateSearch: (
    search: Record<string, unknown>,
  ): { sort?: "recent" | "rating"; fav?: boolean } => ({
    sort: search.sort === "rating" ? "rating" : search.sort === "recent" ? "recent" : undefined,
    fav: search.fav === true || search.fav === "true" ? true : undefined,
  }),
  component: Meals,
});
```

- [ ] **Step 2: Create the meal detail route**

Create `web/app/src/routes/meal.tsx`:
```tsx
import { createRoute, getRouteApi, Link } from "@tanstack/react-router";
import { useQuery } from "@connectrpc/connect-query";
import { getMeal } from "../gen/internal/meal/v1/meal-MealService_connectquery";
import { rootRoute } from "./__root";
import { initials, thumbStyle, tintFor } from "../lib/thumb";

const routeApi = getRouteApi("/meals/$id");

function MealDetail() {
  const { id } = routeApi.useParams();
  const { data, error, isPending } = useQuery(getMeal, { id });

  if (isPending) return <p className="text-muted-foreground">Loading…</p>;
  if (error)
    return <p className="text-red-600 dark:text-red-400">{error.message}</p>;

  const meal = data.meal;
  if (!meal) return <p className="text-muted-foreground">Not found.</p>;
  const tint = tintFor(meal.id);

  return (
    <article className="space-y-7">
      <Link
        to="/meals"
        className="font-mono text-sm text-muted-foreground hover:text-foreground"
      >
        ← Meals
      </Link>

      <div className="flex items-start gap-5">
        <div
          className="flex h-16 w-16 flex-none items-center justify-center rounded-2xl font-mono text-lg font-semibold"
          style={thumbStyle(tint)}
        >
          {initials(meal.name)}
        </div>
        <div>
          <h1 className="text-3xl font-semibold tracking-tight">{meal.name}</h1>
          <div className="mt-2 flex items-center gap-3">
            <span className="tracking-widest">
              <span className="text-amber-400">{"★".repeat(meal.rating)}</span>
              <span className="text-muted-foreground/40">{"★".repeat(5 - meal.rating)}</span>
            </span>
            <span className="font-mono text-sm text-muted-foreground">{meal.cuisine}</span>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="rounded-2xl border border-border bg-card/60 p-4">
          <div className="font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
            Times cooked
          </div>
          <div className="mt-1 text-2xl font-semibold">{meal.timesCooked}</div>
        </div>
        <div className="rounded-2xl border border-border bg-card/60 p-4">
          <div className="font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
            Last eaten
          </div>
          <div className="mt-1 text-2xl font-semibold">{meal.lastCooked || "—"}</div>
        </div>
      </div>

      <section>
        <div className="mb-3 font-mono text-xs uppercase tracking-[0.1em] text-muted-foreground">
          Recent cooks
        </div>
        <div className="flex flex-col gap-2">
          {data.recentCooks.map((c, i) => (
            <div
              key={i}
              className="flex items-center justify-between rounded-xl border border-border bg-card/40 px-4 py-2.5"
            >
              <span className="font-mono text-sm text-muted-foreground">{c.cookedOn}</span>
              <span className="text-sm text-foreground/80">{c.note}</span>
            </div>
          ))}
        </div>
      </section>

      {meal.recipeId && (
        <Link
          to="/recipes/$id"
          params={{ id: meal.recipeId }}
          className="inline-flex items-center gap-2 rounded-xl bg-primary px-4 py-2.5 text-sm font-medium text-primary-foreground hover:opacity-90"
        >
          View recipe →
        </Link>
      )}
    </article>
  );
}

export const mealDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/meals/$id",
  component: MealDetail,
});
```

- [ ] **Step 3: Register the routes**

Replace the entire contents of `web/app/src/router.tsx` with:
```tsx
import { createRouter } from "@tanstack/react-router";
import { rootRoute } from "./routes/__root";
import { indexRoute } from "./routes/index";
import { recipesRoute } from "./routes/recipes";
import { recipeDetailRoute } from "./routes/recipe";
import { mealsRoute } from "./routes/meals";
import { mealDetailRoute } from "./routes/meal";

const routeTree = rootRoute.addChildren([
  indexRoute,
  recipesRoute,
  recipeDetailRoute,
  mealsRoute,
  mealDetailRoute,
]);

export const router = createRouter({ routeTree });

// Register the router instance for full type inference across the app.
declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
```

- [ ] **Step 4: Add the sidebar Meals link**

In `web/app/src/chat/Sidebar.tsx`, between the Home `<Link>` and the Recipes `<Link>`, add:
```tsx
        <Link
          to="/meals"
          className="flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-muted-foreground hover:bg-muted [&.active]:bg-accent [&.active]:text-foreground"
        >
          <span className="h-1.5 w-1.5 rounded-full bg-current opacity-60" />
          Meals
        </Link>
```

- [ ] **Step 5: Verify typecheck, lint, build**

Run (from `web/app`): `npx tsc -b && pnpm lint && pnpm build`
Expected: all clean (typed `to="/meals"`, `to="/meals/$id"`, `params`, and `validateSearch` resolve). Return to repo root: `cd ../..`

- [ ] **Step 6: Runtime check**

Run (from repo root):
```bash
DINNERWISE_DB="$(mktemp -d)/dinnerwise.db" ADDR=:8090 go run ./cmd/server/ &
(cd web/app && VITE_API_URL=http://localhost:8090 pnpm dev --port 5178 &)
sleep 4
curl -s -o /dev/null -w "meals %{http_code}\n" "http://localhost:5178/meals?sort=rating&fav=true"
kill %1 %2 2>/dev/null; pkill -f vite 2>/dev/null
```
Expected: `meals 200`. Controller's visual check: `/meals` lists meals; the Sort and Favorites chips change the URL params and the list; a meal detail shows stats + recent cooks; a linked meal's "View recipe →" opens `/recipes/$id`; sidebar shows Home · Meals · Recipes.

- [ ] **Step 7: Commit**

```bash
git add web/app/src/routes/meals.tsx web/app/src/routes/meal.tsx web/app/src/router.tsx web/app/src/chat/Sidebar.tsx
git commit -m "feat: Meals UI (/meals + /meals/\$id) and sidebar link

Sous meals list with URL-param sort/favorites driving ListMeals; meal detail
with stats, recent cooks, and a recipe link; sidebar gains Meals.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage:**
- Schema (meal + meal_cook, recipe_id FK SET NULL, cascade) → Task 1.
- Repo: derived counts, server-side sort (recent/rating) + favorites filter, GetByID with recent cooks + link + not-found → Task 2.
- Seed: 11 design meals + generated full cook log, idempotent, recipe links → Task 3.
- Proto + service (ListMeals server-side sort/filter, GetMeal not-found→CodeNotFound) → Task 4.
- Wire after recipes (FK order); mount handler → Task 5.
- /meals typed URL params drive ListMeals; /meals/$id detail; sidebar Meals → Task 6.
- Testing: repo (sort/filter/derived/history/not-found), service (mapping/not-found), frontend tsc/eslint/build + runtime → Tasks 2–4, 6.

**Placeholder scan:** none. (Task 6 Step 1 includes a clarifying note that the unused `stars` helper snippet must NOT be added — the rating uses inline `★`.repeat; flagged explicitly so it isn't transcribed as dead code.)

**Type consistency:** `Meal`/`Cook` fields and `List(sort,favoritesOnly)`/`GetByID`→`(Meal,[]Cook,error)` used identically across repo, seed tests, service. `toProtoMeal` field mapping matches the proto (`Id/TimesCooked/LastCooked/RecipeId`). Generated TS camelCase (`timesCooked`,`lastCooked`,`recipeId`,`favoritesOnly`,`recentCooks`,`cookedOn`) used in routes. `mealsRoute`/`mealDetailRoute` registered in router; `to="/meals"`,`to="/meals/$id"` typed. `lib/thumb` helpers reused from slice 1.
