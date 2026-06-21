# Recipe Metadata + Pantry (Slice 3 of 5) — Design

**Date:** 2026-06-21
**Status:** Approved (design)
**Arc:** Sous design (1 visual ✓ · 2 Meals ✓ · **3 recipe metadata + pantry** · 4 agent reference cards + scenarios · 5 orb physics)

## Goal

Enrich the existing `recipe` domain with the metadata the Sous design shows —
**cuisine**, **difficulty**, structured numbered **steps** (replacing the single
`instructions` blob), and an **"in pantry"** badge derived from a real pantry
set. No new package; this modifies `internal/recipe` and its routes in place.

## Decisions (locked)

- **Real pantry set:** a `pantry_item` table of ingredient ids on hand; a recipe
  is `in_pantry` when it has ingredients **and every** ingredient is in the
  pantry. Computed server-side, exposed as `in_pantry` per recipe. (Sets up
  slice 4's "what can I cook tonight" matching.)
- **Structured steps:** replace the single `instructions` column/field with an
  ordered `recipe_step` table and a `repeated string steps` proto field. Honors
  slice 1's deferral of numbered steps to "the recipe-metadata slice."
- **No migration tooling:** schema stays idempotent `CREATE TABLE IF NOT EXISTS`;
  the local `dinnerwise.db` is disposable and must be deleted to pick up the new
  columns. Tests use fresh temp DBs.
- Pantry is a single **global** set (no household model).
- Pantry/"tonight" **filtering** and the agent scenarios that drive it are
  **slice 4**; only the data + badge land here.

## Schema (`internal/recipe/schema.sql`)

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
-- `instructions` column removed.

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

Because schema is `CREATE TABLE IF NOT EXISTS`, a pre-existing `recipe` table
won't gain the new columns — the disposable `dinnerwise.db` must be deleted
(documented in the plan; tests use `t.TempDir`).

## Structs (`internal/recipe/recipe.go`)

```go
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
// Instructions removed. RecipeIngredient unchanged.
```

## Repo (`internal/recipe/repo.go`)

- A small helper loads the pantry set: `pantrySet(ctx) (map[string]struct{}, error)`
  from `SELECT ingredient_id FROM pantry_item`.
- `List(ctx)`: select `id,name,cuisine,difficulty,servings,total_minutes`; assemble
  ingredients (existing N+1-free pattern) and steps (`SELECT recipe_id, text FROM
  recipe_step ORDER BY recipe_id, position`); compute `InPantry` in Go = `len(
  ingredients) > 0 && every ingredient.id ∈ pantrySet`.
- `GetByID(ctx, id)`: same for one recipe (steps `WHERE recipe_id=? ORDER BY
  position`); `ErrNotFound` unchanged.

## Seed + fixtures (`internal/recipe/seed.go`, `fixtures/recipes.json`)

- `recipes.json`: each recipe gains `cuisine`, `difficulty`, and `steps` (array of
  strings, replacing `instructions`); keeps `ingredients`.
- A pantry fixture (`fixtures/pantry.json`, an array of ingredient ids, embedded)
  seeds the on-hand set: `["olive-oil","garlic","spaghetti","canned-tomato",
  "chicken-thigh","broccoli"]` → **tomato-pasta** and **sheet-pan-chicken**
  read in-pantry; **veggie-stir-fry** does not (missing tofu/bell-pepper/
  soy-sauce/rice).
- `SeedIfEmpty` (one transaction, idempotent on empty `recipe`) now also inserts
  `recipe_step` rows (positions 1..n) and `pantry_item` rows (after ingredients,
  FK). Ingredient inserts unchanged (`ON CONFLICT DO NOTHING`).

## Proto (`internal/recipe/v1/recipe.proto`)

`Recipe` gains:
```proto
string cuisine = 7;
string difficulty = 8;
repeated string steps = 9;
bool in_pantry = 10;
```
and **removes** `string instructions = 3;` (leave field number 3 reserved/unused
— do not reuse it). `RecipeIngredient`, `ListRecipes`, `GetRecipe` unchanged.
Regenerate (`make gen`).

## Service (`internal/recipe/service.go`)

`toProtoRecipe` maps `Cuisine`, `Difficulty`, `Steps`, `InPantry`; drops
`Instructions`.

## Frontend

- **`recipes.tsx` (list):** meta line shows `⏱ {totalMinutes} min · {cuisine}`;
  an **"in pantry"** badge (green pill, per the design) when `r.inPantry`. Existing
  ingredient filter chip unchanged.
- **`recipe.tsx` (detail):** meta line `⏱ {totalMinutes} min · {cuisine} ·
  {difficulty}`; replace the Method-from-`instructions` block with **numbered
  steps** rendered from `steps` (numbered badge per the Sous design); show an
  "in pantry" indicator when `inPantry`.

## Testing

- **Go repo:** cuisine/difficulty returned; steps assembled in `position` order;
  `in_pantry` true when all of a recipe's ingredients are in the pantry, false
  when one is missing; empty-ingredient recipe → `in_pantry` false. `GetByID`
  returns steps + in_pantry; not-found unchanged.
- **Go seed:** steps seeded (count/order), pantry seeded, idempotent; the
  expected recipes read in-pantry vs not.
- **Go service:** new fields mapped (cuisine/difficulty/steps/in_pantry); no
  instructions field. Update any existing recipe repo/service tests that
  referenced `instructions`.
- **Frontend:** `tsc -b`/`eslint`/`vite build` clean; runtime check — list shows
  the in-pantry badge on pasta + chicken (not stir-fry); detail shows numbered
  steps + cuisine/difficulty.

## Tradeoffs & notes

- Dropping `instructions` is a clean break; safe because everything is internal
  and the DB is disposable. Reserve proto field 3.
- `in_pantry` computed in Go from a loaded pantry set (simple, testable) rather
  than a correlated SQL subquery; fine at fixture scale.
- Single global pantry — no household/ownership. Revisit if multi-household lands.
- No pantry *filter* yet; the agent-driven "tonight" view that consumes
  `in_pantry` is slice 4.

## Out of scope

Pantry/"tonight" filtering + agent scenarios (slice 4); reference cards/replay (4);
orb physics (5); household model; editing the pantry from the UI.
