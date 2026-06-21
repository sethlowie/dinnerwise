# RecipeService — proto → Connect → React Slice — Design

**Date:** 2026-06-21
**Status:** Approved (design)
**Branch:** feat/sqlite-persistence (extends the SQLite persistence foundation)

## Goal

Expose the recipe persistence layer to the frontend through the project's
proto → Connect → React generation pipeline, and surface recipes in the UI.
Replaces the scaffolding `foo` example with the first real domain service.

## Architecture (three layers, already partly built)

```
proto: internal/recipe/v1/recipe.proto   ← API contract (generates Go + TS)
        ↓ RecipeService (Connect handler)
service: internal/recipe/service.go       ← maps repo.Recipe ↔ proto Recipe
        ↓
storage: internal/recipe/repo.go (Repo)   ← SQL (built in the persistence foundation)
```

Per the dora convention: **proto is the API-layer contract only**; the domain
`Recipe`/`RecipeIngredient` structs stay plain Go, mapped to proto in the
service layer. No import cycle — `internal/recipe` imports `internal/recipe/v1`,
never the reverse.

## Decisions (locked)

- RPCs: `ListRecipes` and `GetRecipe`.
- Frontend: add `/recipes` (list) and `/recipes/$id` (detail); **remove the foo
  example entirely**.
- Service impl lives in `package recipe` (cohesive with the repo), mirroring how
  `foo.NewService()` implemented `foov1connect.FooServiceHandler`.
- Generated code stays gitignored (existing convention); `buf generate` produces
  it.

## Proto (`internal/recipe/v1/recipe.proto`)

Package `internal.recipe.v1`. Messages mirror the repo structs:

```proto
syntax = "proto3";
package internal.recipe.v1;

message Recipe {
  string id = 1;
  string name = 2;
  string instructions = 3;
  int32 servings = 4;
  int32 total_minutes = 5;
  repeated RecipeIngredient ingredients = 6;
}

message RecipeIngredient {
  string ingredient_id = 1;
  string name = 2;
  double quantity = 3;
  string unit = 4;
}

message ListRecipesRequest {}
message ListRecipesResponse { repeated Recipe recipes = 1; }
message GetRecipeRequest { string id = 1; }
message GetRecipeResponse { Recipe recipe = 1; }

service RecipeService {
  rpc ListRecipes(ListRecipesRequest) returns (ListRecipesResponse);
  rpc GetRecipe(GetRecipeRequest) returns (GetRecipeResponse);
}
```

`buf generate` (existing `buf.gen.yaml`) emits:
- Go: `internal/recipe/v1/recipe.pb.go`, `internal/recipe/v1/recipev1connect/recipe.connect.go`
- TS: `web/app/src/gen/internal/recipe/v1/recipe_pb.ts`, `…/recipe-RecipeService_connectquery.ts`

## Service layer (`internal/recipe/service.go`, `package recipe`)

```go
type Service struct { repo *Repo }

func NewService(repo *Repo) recipev1connect.RecipeServiceHandler {
    return &Service{repo: repo}
}
```

- `ListRecipes` → `repo.List(ctx)`; on error `connect.NewError(connect.CodeInternal, err)`;
  map each domain `Recipe` to proto via `toProtoRecipe`; return `ListRecipesResponse`.
- `GetRecipe` → `repo.GetByID(ctx, req.Msg.GetId())`; `errors.Is(err, ErrNotFound)`
  → `connect.NewError(connect.CodeNotFound, err)`; other error → `CodeInternal`;
  else return `GetRecipeResponse{Recipe: toProtoRecipe(r)}`.
- `toProtoRecipe(Recipe) *recipev1.Recipe` — maps fields, converts `int`→`int32`,
  builds nested `[]*recipev1.RecipeIngredient`.

Naming: local domain `Recipe` and proto `recipev1.Recipe` coexist (different
qualifiers); the mapping helper bridges them.

## Server wiring (`cmd/server/main.go`)

Build the repo once and mount the recipe handler; remove the foo handler:

```go
repo := recipe.NewRepo(database)
mux := http.NewServeMux()
mux.Handle(recipev1connect.NewRecipeServiceHandler(recipe.NewService(repo)))
```

Keep db open/migrate/seed, the boot recipe-count log, and `withCORS`.

## Frontend (`web/app`)

- **`/recipes`** (`routes/recipes.tsx`): `useQuery(listRecipes, {})` renders recipe
  cards — name, ⏱ `totalMinutes`, `servings`, and ingredient names rendered as
  chips. Each card links to the detail route. Themed with semantic tokens
  (`bg-card`, `text-foreground`, `bg-primary`, etc.). Loading + error states.
- **`/recipes/$id`** (`routes/recipe.tsx`): typed path param `id`,
  `useQuery(getRecipe, {id})`; shows name, meta, instructions, and the full
  ingredient list (quantity + unit + name). Loading + error (incl. not-found)
  states. Back link to `/recipes`.
- **Nav** (`routes/__root.tsx`): `Home | Recipes` (Foo removed).
- **Home** (`routes/index.tsx`): CTA links to `/recipes` instead of `/foo`.
- Router registration (`router.tsx`): add `recipesRoute`, `recipeDetailRoute`;
  remove `fooRoute`.
- `transport.ts` and `App.tsx` (providers) unchanged — the Connect transport is
  generic and reused.

## Remove foo

- Delete: `internal/foo/` (service.go), `internal/foo/v1/` (foo.proto + generated
  `foo.pb.go`), `internal/foo/v1/foov1connect/` (generated).
- Delete: `web/app/src/routes/foo.tsx`, `web/app/src/gen/internal/foo/`.
- Edit: remove foo route from `router.tsx`; remove foo handler/import from
  `cmd/server/main.go`; remove Foo nav link from `__root.tsx`; update `index.tsx`
  CTA.

## Testing

- `internal/recipe/service_test.go` (package `recipe`): migrate + seed a temp DB
  (reuse the `newTestDB` helper / `db.Open` + `Migrate` + `SeedIfEmpty`), then:
  - `ListRecipes` returns 3 recipes, each with its ingredients mapped (assert a
    known recipe's field values + ingredient count).
  - `GetRecipe` for a seeded id returns the mapped recipe with ingredients.
  - `GetRecipe` for a missing id returns an error with
    `connect.CodeOf(err) == connect.CodeNotFound`.
- Frontend: `tsc -b`, `eslint`, `vite build` all clean; runtime check that
  `/recipes` lists seeded recipes and a card link opens `/recipes/$id`.

## Out of scope

- Mutations (create/update/delete recipes).
- Pagination/filtering on ListRecipes (the agent-aware typed-search filtering
  comes with a later feature).
- OTel interceptors / auth (deferred, as in the persistence foundation).

## Tradeoffs & notes

- `ListRecipes` returns all recipes unpaginated — fine at fixture scale; revisit
  with filtering when the agent needs it.
- Removing foo loses the minimal reference example, but RecipeService becomes the
  canonical end-to-end example, which is more representative.
