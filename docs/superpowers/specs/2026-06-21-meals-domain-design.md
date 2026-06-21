# Meals Domain (Slice 2 of 5) — Design

**Date:** 2026-06-21
**Status:** Approved (design)
**Arc:** Sous design, slice 2 of 5 (1 visual redesign ✓ · **2 Meals domain** · 3 recipe metadata/pantry · 4 agent reference cards + scenarios · 5 orb physics)

## Goal

Add a logged-meals domain: a `MealService` backed by SQLite (meals + a cook log
with derived counts), and Sous-styled `/meals` list + `/meals/$id` detail
screens, with the sidebar gaining a Meals link. Mirrors the recipe slice's
structure (schema → repo → seed → proto → service → routes).

## Decisions (locked)

- **Server-side sort/filter** on `ListMeals` (the Go repo does `ORDER BY` /
  `WHERE`). **URL search params are the source of truth**: `/meals` reads typed
  `sort`/`fav` params and passes them as `ListMeals` request args. This keeps the
  router-aware story intact so slice 4's agent can drive Meals by navigating to
  `/meals?sort=rating&fav=1`.
- **Counts derived from a full cook log.** No stored `times_cooked`/`last_cooked`
  columns; `times_cooked = COUNT(meal_cook)`, `last_cooked = MAX(cooked_on)`. The
  seeder writes the full cook log.
- **Meal→recipe link** included: `meal.recipe_id` nullable FK; detail links to
  `/recipes/$id` when set. The agent *navigating* to filtered Meals is slice 4.

## Schema (`internal/meal/schema.sql`)

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
  cooked_on TEXT NOT NULL,          -- ISO date "2026-06-18"
  note      TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_meal_cook_meal ON meal_cook(meal_id);
```

ISO date strings sort lexically, so `MAX(cooked_on)` / `ORDER BY cooked_on` are
correct. `recipe(id)` FK is valid because the recipe tables already exist (and
both packages migrate before the server serves).

## Repo (`internal/meal`)

Domain structs (plain Go, mapped to proto in the service layer):
```go
type Meal struct {
    ID          string
    Name        string
    Cuisine     string
    Rating      int
    TimesCooked int
    LastCooked  string   // "" if never cooked
    RecipeID    string   // "" if no linked recipe
}
type Cook struct { CookedOn string; Note string }
```

- `List(ctx, sort string, favoritesOnly bool) ([]Meal, error)` —
  ```sql
  SELECT m.id, m.name, m.cuisine, m.rating, COALESCE(m.recipe_id,''),
         COUNT(mc.id) AS times_cooked, COALESCE(MAX(mc.cooked_on),'') AS last_cooked
  FROM meal m LEFT JOIN meal_cook mc ON mc.meal_id = m.id
  [WHERE m.rating >= 4]                       -- when favoritesOnly
  GROUP BY m.id
  ORDER BY <recent: last_cooked DESC, m.name | rating: m.rating DESC, times_cooked DESC, m.name>
  ```
  `sort` accepts `"recent"` (default) and `"rating"`; unknown → `recent`.
- `GetByID(ctx, id) (Meal, []Cook, error)` — the same aggregate for one meal,
  plus recent cooks (`SELECT cooked_on, note FROM meal_cook WHERE meal_id=?
  ORDER BY cooked_on DESC LIMIT 5`). Returns `ErrNotFound` if the meal is absent.
- `ErrNotFound` sentinel (mirrors recipe).

## Seed (`internal/meal/seed.go` + `internal/meal/fixtures/meals.json`)

Fixtures port the design's 11 meals: `{id, name, cuisine, rating, timesCooked,
last, recipeId?}` (e.g. Miso-Glazed Salmon / Japanese / 5★ / 7 / Jun 18;
Sun-Dried Tomato Pasta → `recipe_id` `tomato-pasta`; Veggie Stir-Fry →
`veggie-stir-fry`; Lemon Herb Roast Chicken → `sheet-pan-chicken`; the rest
`recipeId` null).

`SeedIfEmpty(db)` (idempotent; insert only when `meal` is empty, one
transaction): inserts each meal, then **generates its full cook log** — for a
meal with `timesCooked = n` and `last = D`, writes `n` `meal_cook` rows dated
weekly back from `D`, attaching a short rotating note to the most recent few
(e.g. "Weeknight dinner", "Tried it with extra garlic"), `''` for older ones.
So `COUNT`/`MAX` reproduce the design's numbers. Dates are computed from a fixed
base date in the fixture (no `time.Now()` in seed, for deterministic data).

## Proto (`internal/meal/v1/meal.proto`) → `MealService`

```proto
package internal.meal.v1;

message Meal {
  string id = 1; string name = 2; string cuisine = 3;
  int32 rating = 4; int32 times_cooked = 5; string last_cooked = 6;
  string recipe_id = 7;   // "" if none
}
message Cook { string cooked_on = 1; string note = 2; }

message ListMealsRequest { string sort = 1; bool favorites_only = 2; }
message ListMealsResponse { repeated Meal meals = 1; }
message GetMealRequest { string id = 1; }
message GetMealResponse { Meal meal = 1; repeated Cook recent_cooks = 2; }

service MealService {
  rpc ListMeals(ListMealsRequest) returns (ListMealsResponse);
  rpc GetMeal(GetMealRequest) returns (GetMealResponse);
}
```
`buf generate` emits Go + TS (gitignored).

## Service (`internal/meal/service.go`, `package meal`)

`NewService(repo *Repo) mealv1connect.MealServiceHandler`:
- `ListMeals` → `repo.List(ctx, req.Msg.GetSort(), req.Msg.GetFavoritesOnly())`;
  map to proto; `CodeInternal` on error.
- `GetMeal` → `repo.GetByID(ctx, id)`; `ErrNotFound` → `CodeNotFound`; map meal +
  cooks to proto.
- `toProtoMeal(Meal) *mealv1.Meal` helper (int→int32). No import cycle.

`cmd/server/main.go`: build `mealRepo := meal.NewRepo(database)`; `meal.Migrate` +
`meal.SeedIfEmpty` at boot (after recipe seed, since meals FK recipes); mount
`mealv1connect.NewMealServiceHandler(meal.NewService(mealRepo))`.

## Frontend

- **`/meals` (`web/app/src/routes/meals.tsx`)** — `validateSearch` →
  `{ sort?: "recent"|"rating"; fav?: boolean }`. `useQuery(listMeals, { sort:
  sort ?? "recent", favoritesOnly: fav ?? false })`. Sous list: kicker "Your
  kitchen", heading "Meals"; rows with tinted thumb (`lib/thumb` by meal id),
  name, mono `cuisine · cooked N×`, ★rating (gold filled / muted empty), last
  date. **Sort chip** (Recent ⇄ Rating) and **favorites ★ chip** call
  `navigate({ search })` to update the URL params (which drive the query).
- **`/meals/$id` (`web/app/src/routes/meal.tsx`)** — `useQuery(getMeal,{id})`.
  Sous detail: back link, big thumb + name + (stars + `cuisine`), two stat cards
  (**Times cooked** = `times_cooked`, **Last eaten** = `last_cooked`), **Recent
  cooks** list (date + note from `recent_cooks`), and a **View recipe →** link to
  `/recipes/$id` when `meal.recipe_id` is non-empty.
- **Sidebar (`web/app/src/chat/Sidebar.tsx`)** — add a **Meals** link between
  Home and Recipes (`to="/meals"`).
- Router (`router.tsx`): register `mealsRoute` + `mealDetailRoute`.

## Components / files

- Create: `internal/meal/v1/meal.proto`, `internal/meal/meal.go` (structs),
  `internal/meal/schema.sql`, `internal/meal/migrate.go`, `internal/meal/repo.go`,
  `internal/meal/seed.go`, `internal/meal/fixtures/meals.json`,
  `internal/meal/repo_test.go`, `internal/meal/service.go`,
  `internal/meal/service_test.go`.
- Modify: `cmd/server/main.go` (wire meal migrate/seed/handler).
- Create: `web/app/src/routes/meals.tsx`, `web/app/src/routes/meal.tsx`.
- Modify: `web/app/src/router.tsx` (register routes), `web/app/src/chat/Sidebar.tsx` (Meals link).

## Testing

- **Go repo:** seed idempotent; derived `times_cooked`/`last_cooked` correct vs
  fixtures; `List` ordering for `recent` and `rating`; `favoritesOnly` keeps only
  rating≥4; `GetByID` returns meal + recent cooks (newest first) + recipe link;
  not-found → `ErrNotFound`; FK cascade removes cook rows on meal delete;
  `recipe_id` SET NULL behavior is acceptable (not exercised destructively).
- **Go service:** `ListMeals` passes sort/favorites through and maps results;
  unknown sort defaults to recent; `GetMeal` not-found → `connect.CodeNotFound`;
  recent_cooks mapped.
- **Frontend:** `tsc -b`, `eslint`, `vite build` clean; runtime check — `/meals`
  renders rows; toggling the sort/favorites chips changes the URL params and the
  list; a meal detail shows stats + recent cooks; a linked meal's "View recipe"
  opens `/recipes/$id`.

## Tradeoffs & notes

- Deriving counts means the seeder writes the full log (~56 rows across 11
  meals); generated in code from `{timesCooked, last}`, not hand-authored.
- Server-side sort/filter + URL-param source-of-truth: the route translates
  params → request args; the server does the work. Slice 4's agent only needs to
  set the params.
- `last_cooked` as an ISO string is fine for sort/display; if we later need
  relative formatting ("3 days ago") that's a frontend concern.
- Only `recent`/`rating` sorts now (YAGNI); more can be added when needed.

## Out of scope

Recipe cuisine/difficulty/pantry (slice 3); agent scenarios/reference cards that
drive Meals (slice 4); orb physics (slice 5); meal mutations (logging a cook from
the UI).
