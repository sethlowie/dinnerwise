# SQLite Persistence Foundation — Design

**Date:** 2026-06-21
**Status:** Approved (design)

## Goal

Establish dinnerwise's persistence layer with the smallest possible dependency
and setup footprint, and prove the pattern end-to-end on one real domain slice
(recipes + ingredients). Future tables should be "copy the pattern," not new
infrastructure decisions.

## Why SQLite (pure Go)

The earlier locked choice was MongoDB, picked for low setup friction. SQLite via
the **pure-Go `modernc.org/sqlite` driver** is even lower friction and a better
fit:

- **No server / container.** The database is a single file; nothing to start,
  trivial to seed, reset, and clone-and-run.
- **No CGO.** `modernc.org/sqlite` is pure Go — no C toolchain. This avoids the
  exact failure (`sqlc`/CGO `go install` breaking on macOS) that soured the
  earlier SQL attempt. We must NOT use `mattn/go-sqlite3` (CGO).
- **Relational fit.** The domain is relational (recipes ↔ ingredients, plans →
  recipes, pantry, prices) and the "constraint spine" (filter by allergy/diet,
  then rank) maps naturally to SQL `WHERE` clauses.
- **Innovation hook (future):** a read-only SQL query tool is a clean, safe
  capability to hand the agent over a small schema.

This supersedes the MongoDB note in prior project memory.

## Decisions (locked)

- **Access layer:** stdlib `database/sql` + `modernc.org/sqlite`, hand-written
  SQL in thin repos. No ORM, no query codegen (no `sqlc`), no CGO.
- **Scope now:** persistence plumbing + the recipes/ingredients slice. Defer the
  full domain model (household, pantry, prices, meal plans, ratings, ingredient
  enrichment) to when those features are built.
- **Migrations:** idempotent `CREATE TABLE IF NOT EXISTS` schema applied at
  startup. No versioned migration tool. (Tradeoff acknowledged below.)
- **Schema ownership:** per-domain — each domain package embeds and owns its
  own `schema.sql` and seed, rather than one central schema file.

## Architecture

Layering keeps `internal/db` domain-agnostic so each new domain is additive:

- **`internal/db`** — generic infrastructure only:
  - `Open(path string) (*sql.DB, error)` — builds the DSN with pragmas, opens,
    pings, sets pool options.
  - `ApplySchema(db *sql.DB, schema string) error` — helper to run an embedded
    schema string (used by domain packages' `Migrate`).
  - Knows nothing about recipes or any domain type.
- **`internal/recipe`** — self-contained domain package:
  - `recipe.go` — domain structs (`Recipe`, `Ingredient`, `RecipeIngredient`).
  - `schema.sql` (embedded) + `Migrate(db) error`.
  - `repo.go` — `Repo` with hand-written SQL: `List(ctx)`, `GetByID(ctx, id)`.
  - `seed.go` + `fixtures/recipes.json` (embedded) — `SeedIfEmpty(db) error`.
  - `repo_test.go` — tests against a fresh temp DB.
- **`cmd/server/main.go`** — orchestrates wiring:
  ```go
  database, err := db.Open(dbPath)
  recipe.Migrate(database)
  recipe.SeedIfEmpty(database)
  ```
  A future domain is one new package plus two lines here.

## Connection & pragmas

Open with a DSN that sets pragmas per pooled connection (modernc applies
`_pragma` query params on each new connection):

```
file:<path>?_pragma=foreign_keys(ON)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)
```

- `foreign_keys(ON)` — SQLite defaults this OFF; required for FK enforcement.
- `journal_mode(WAL)` — better read/write concurrency.
- `busy_timeout(5000)` — wait rather than immediately erroring on a locked DB.

Pool: a single-user demo is fine with conservative settings; FK/pragma
correctness comes from the DSN (applied per connection), not pool size.

## Schema (recipe slice)

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

IDs are human-readable `TEXT` slugs (e.g. `"sheet-pan-chicken"`,
`"olive-oil"`), which makes fixtures readable and joins debuggable.

`ingredient` is intentionally lean now. Enrichment fields (dietary flags,
functional roles, substitutes) are deferred to the enrichment feature.

## Domain structs

```go
type Recipe struct {
    ID           string
    Name         string
    Instructions string
    Servings     int
    TotalMinutes int
    Ingredients  []RecipeIngredient
}

type RecipeIngredient struct {
    IngredientID string
    Name         string  // joined from ingredient
    Quantity     float64
    Unit         string
}
```

Per dora convention, repo structs are NOT proto types; mapping repo ↔ proto
happens in the service layer if/when a `RecipeService` is exposed. (No Connect
service is in scope for this foundation.)

## Repo behavior

- `List(ctx) ([]Recipe, error)` — all recipes with their ingredients assembled.
  Avoid N+1: fetch recipes, then fetch all `recipe_ingredient` joined to
  `ingredient` and group in Go.
- `GetByID(ctx, id) (Recipe, error)` — one recipe with ingredients; distinct
  not-found error (`sql.ErrNoRows` wrapped) so callers can map to 404 later.

## Seeding

- `//go:embed fixtures/recipes.json` holds a small set of real recipes, each
  with its ingredient lines.
- `SeedIfEmpty(db)` runs at startup: if `recipe` is empty, insert fixtures
  (recipes, distinct ingredients, and join rows) inside a transaction.
  Idempotent — safe on every boot. Ingredient inserts use
  `INSERT ... ON CONFLICT(name) DO NOTHING` (or pre-dedupe) so shared
  ingredients across recipes don't collide.

## Config & gitignore

- DB path from env `DINNERWISE_DB`, default `./dinnerwise.db`.
- Add to `.gitignore`: `*.db`, `*.db-wal`, `*.db-shm`.

## Testing (TDD)

- Repo tests create a fresh temp DB file per test (`t.TempDir()`), run
  `Migrate`, insert/seed, then assert query results. No running server needed.
- Cover: migrate is idempotent (run twice), seed-if-empty inserts once and is a
  no-op the second time, `List` assembles ingredients correctly, `GetByID`
  returns ingredients and a not-found error for a missing id, FK cascade on
  recipe delete removes join rows.

## Tradeoffs & known unknowns

- **No versioned migrations.** Idempotent startup schema is lowest-friction and
  fine while the schema is additive and the demo DB is disposable. If we need to
  evolve columns/data non-trivially, upgrade to a lightweight library-mode
  migrator (e.g. goose as a library with embedded SQL) — no CLI install.
- **Per-domain schema** means no single place to see the whole schema; acceptable
  for clarity/isolation, revisit if cross-domain constraints grow.
- **Concurrency.** SQLite is single-writer; perfectly fine for this app's scale.
  WAL + busy_timeout cover the demo's needs.

## Out of scope

- Full domain model beyond recipes/ingredients.
- Any Connect/RPC service exposing recipes.
- Ingredient enrichment, prices, meal plans, ratings, pantry, household.
