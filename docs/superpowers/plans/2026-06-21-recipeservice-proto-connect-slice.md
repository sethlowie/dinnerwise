# RecipeService proto → Connect → React Slice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Expose recipes to the frontend through proto → Connect → React generation, with `/recipes` and `/recipes/$id` pages, and remove the `foo` example.

**Architecture:** A new `recipe/v1` proto defines `RecipeService` (`ListRecipes`, `GetRecipe`); `buf generate` emits Go + TS. A service in `package recipe` maps the existing `Repo` (domain structs) onto proto. The server mounts the handler; React consumes generated Connect-Query hooks on two routes. `foo` is deleted.

**Tech Stack:** Go 1.25, ConnectRPC, buf v2, `database/sql`/SQLite (existing), React 19 + Vite, TanStack Router + Connect-Query, Tailwind v4.

## Global Constraints

- Proto is the API-layer contract only; domain structs stay plain Go, mapped to proto in the service layer. No import cycle (`internal/recipe` → `internal/recipe/v1`, never reverse).
- Generated code is gitignored — never commit `*.pb.go`, `*.connect.go`, `*_pb.ts`, `*_connectquery.ts`. Regenerate with `make gen` (runs `buf generate`).
- Service impl lives in `package recipe` (same package as the repo), mirroring how `foo.NewService()` implemented `foov1connect.FooServiceHandler`.
- The generated proto Go package is imported as alias `recipev1` ("github.com/sethlowie/dinnerwise/internal/recipe/v1") and the connect package is `recipev1connect`.
- Module path: `github.com/sethlowie/dinnerwise`.
- Commit messages end with: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

---

## File Structure

- Create: `internal/recipe/v1/recipe.proto` — RecipeService contract (messages + service).
- Create: `internal/recipe/service.go` — `Service`, `NewService`, `ListRecipes`, `GetRecipe`, `toProtoRecipe`.
- Create: `internal/recipe/service_test.go` — service-layer tests (list, get, not-found).
- Modify: `cmd/server/main.go` — mount RecipeService handler; drop foo handler/import; build repo once.
- Delete: `internal/foo/` (whole tree: `service.go`, `v1/foo.proto`, generated `v1/foo.pb.go`, `v1/foov1connect/`).
- Create: `web/app/src/routes/recipes.tsx` — `/recipes` list page.
- Create: `web/app/src/routes/recipe.tsx` — `/recipes/$id` detail page.
- Modify: `web/app/src/router.tsx` — register recipe routes; remove foo route.
- Modify: `web/app/src/routes/__root.tsx` — nav `Home | Recipes`.
- Modify: `web/app/src/routes/index.tsx` — CTA links `/recipes`.
- Delete: `web/app/src/routes/foo.tsx`, `web/app/src/gen/internal/foo/` (stale generated TS).

Generated (not committed): `internal/recipe/v1/recipe.pb.go`, `internal/recipe/v1/recipev1connect/recipe.connect.go`, `web/app/src/gen/internal/recipe/v1/recipe_pb.ts`, `…/recipe-RecipeService_connectquery.ts`.

---

## Task 1: RecipeService proto + Connect service (Go)

**Files:**
- Create: `internal/recipe/v1/recipe.proto`
- Create: `internal/recipe/service.go`
- Test: `internal/recipe/service_test.go`

**Interfaces:**
- Consumes: `Repo`, `NewRepo`, `Recipe`, `RecipeIngredient`, `ErrNotFound`, `Migrate`, `SeedIfEmpty`, and the test helper `newTestDB(t)` (all in `package recipe`, from the persistence foundation).
- Produces:
  - generated `recipev1` types: `Recipe`, `RecipeIngredient`, `ListRecipesRequest/Response`, `GetRecipeRequest/Response`.
  - generated `recipev1connect.RecipeServiceHandler` + `recipev1connect.NewRecipeServiceHandler`.
  - `func NewService(repo *Repo) recipev1connect.RecipeServiceHandler`.

- [ ] **Step 1: Write the proto contract**

Create `internal/recipe/v1/recipe.proto`:
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

message ListRecipesResponse {
  repeated Recipe recipes = 1;
}

message GetRecipeRequest {
  string id = 1;
}

message GetRecipeResponse {
  Recipe recipe = 1;
}

service RecipeService {
  rpc ListRecipes(ListRecipesRequest) returns (ListRecipesResponse);
  rpc GetRecipe(GetRecipeRequest) returns (GetRecipeResponse);
}
```

- [ ] **Step 2: Generate Go + TS**

Run: `make gen`
Expected: succeeds; these files now exist:
```
internal/recipe/v1/recipe.pb.go
internal/recipe/v1/recipev1connect/recipe.connect.go
web/app/src/gen/internal/recipe/v1/recipe_pb.ts
web/app/src/gen/internal/recipe/v1/recipe-RecipeService_connectquery.ts
```
Verify with: `ls internal/recipe/v1/recipev1connect/recipe.connect.go web/app/src/gen/internal/recipe/v1/`

- [ ] **Step 3: Write the failing test**

Create `internal/recipe/service_test.go`:
```go
package recipe

import (
	"context"
	"testing"

	connect "connectrpc.com/connect"
	recipev1 "github.com/sethlowie/dinnerwise/internal/recipe/v1"
)

// newSeededService returns a Service backed by a migrated, fixture-seeded
// temp database.
func newSeededService(t *testing.T) *Service {
	t.Helper()
	database := newTestDB(t) // migrates
	if err := SeedIfEmpty(database); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return &Service{repo: NewRepo(database)}
}

func TestServiceListRecipes(t *testing.T) {
	svc := newSeededService(t)
	resp, err := svc.ListRecipes(context.Background(),
		connect.NewRequest(&recipev1.ListRecipesRequest{}))
	if err != nil {
		t.Fatalf("ListRecipes: %v", err)
	}
	if len(resp.Msg.Recipes) != 3 {
		t.Fatalf("recipes = %d, want 3", len(resp.Msg.Recipes))
	}
	// List orders by name; first fixture by name is "Sheet-Pan Chicken & Veg".
	first := resp.Msg.Recipes[0]
	if first.Id != "sheet-pan-chicken" {
		t.Fatalf("first recipe id = %q, want sheet-pan-chicken", first.Id)
	}
	if len(first.Ingredients) != 3 {
		t.Fatalf("first recipe ingredients = %d, want 3", len(first.Ingredients))
	}
}

func TestServiceGetRecipe(t *testing.T) {
	svc := newSeededService(t)
	resp, err := svc.GetRecipe(context.Background(),
		connect.NewRequest(&recipev1.GetRecipeRequest{Id: "tomato-pasta"}))
	if err != nil {
		t.Fatalf("GetRecipe: %v", err)
	}
	if resp.Msg.Recipe.GetName() != "Weeknight Tomato Pasta" {
		t.Fatalf("name = %q, want Weeknight Tomato Pasta", resp.Msg.Recipe.GetName())
	}
	if len(resp.Msg.Recipe.Ingredients) != 4 {
		t.Fatalf("ingredients = %d, want 4", len(resp.Msg.Recipe.Ingredients))
	}
}

func TestServiceGetRecipeNotFound(t *testing.T) {
	svc := newSeededService(t)
	_, err := svc.GetRecipe(context.Background(),
		connect.NewRequest(&recipev1.GetRecipeRequest{Id: "does-not-exist"}))
	if got := connect.CodeOf(err); got != connect.CodeNotFound {
		t.Fatalf("code = %v, want %v", got, connect.CodeNotFound)
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/recipe/ -run TestService`
Expected: FAIL — compile error `undefined: Service` (and `NewService` not yet defined).

- [ ] **Step 5: Write the service**

Create `internal/recipe/service.go`:
```go
package recipe

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	recipev1 "github.com/sethlowie/dinnerwise/internal/recipe/v1"
	"github.com/sethlowie/dinnerwise/internal/recipe/v1/recipev1connect"
)

// Service implements recipev1connect.RecipeServiceHandler by mapping the
// domain Repo onto the proto API.
type Service struct {
	repo *Repo
}

// NewService returns a RecipeServiceHandler backed by repo.
func NewService(repo *Repo) recipev1connect.RecipeServiceHandler {
	return &Service{repo: repo}
}

func (s *Service) ListRecipes(
	ctx context.Context,
	req *connect.Request[recipev1.ListRecipesRequest],
) (*connect.Response[recipev1.ListRecipesResponse], error) {
	recs, err := s.repo.List(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*recipev1.Recipe, len(recs))
	for i := range recs {
		out[i] = toProtoRecipe(recs[i])
	}
	return connect.NewResponse(&recipev1.ListRecipesResponse{Recipes: out}), nil
}

func (s *Service) GetRecipe(
	ctx context.Context,
	req *connect.Request[recipev1.GetRecipeRequest],
) (*connect.Response[recipev1.GetRecipeResponse], error) {
	rec, err := s.repo.GetByID(ctx, req.Msg.GetId())
	if errors.Is(err, ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&recipev1.GetRecipeResponse{Recipe: toProtoRecipe(rec)}), nil
}

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
		Instructions: r.Instructions,
		Servings:     int32(r.Servings),
		TotalMinutes: int32(r.TotalMinutes),
		Ingredients:  ingredients,
	}
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/recipe/`
Expected: PASS (service tests + existing repo/seed tests).

- [ ] **Step 7: Commit**

Generated files are gitignored; commit only the proto + hand-written Go.
```bash
git add internal/recipe/v1/recipe.proto internal/recipe/service.go internal/recipe/service_test.go
git commit -m "feat: add RecipeService (ListRecipes, GetRecipe)

Proto contract + Connect service mapping the recipe repo onto proto;
GetRecipe maps ErrNotFound to connect.CodeNotFound. Generated code is
produced by make gen and gitignored.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Wire server, remove foo backend

**Files:**
- Modify: `cmd/server/main.go`
- Delete: `internal/foo/` (entire tree)

**Interfaces:**
- Consumes: `db.Open`, `recipe.Migrate`, `recipe.SeedIfEmpty`, `recipe.NewRepo`, `recipe.NewService`, `recipev1connect.NewRecipeServiceHandler`.
- Produces: a server mounting RecipeService at `/internal.recipe.v1.RecipeService/`; foo fully removed from the backend.

- [ ] **Step 1: Replace main.go imports and body**

Edit `cmd/server/main.go`. Replace the import block with:
```go
import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/sethlowie/dinnerwise/internal/db"
	"github.com/sethlowie/dinnerwise/internal/recipe"
	"github.com/sethlowie/dinnerwise/internal/recipe/v1/recipev1connect"
)
```

Replace the body of `func main()` (from `addr :=` through the `ListenAndServe` block) with:
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

	repo := recipe.NewRepo(database)
	recipes, err := repo.List(context.Background())
	if err != nil {
		log.Fatalf("server: list recipes: %v", err)
	}
	log.Printf("server: %d recipes loaded from %s", len(recipes), dbPath)

	mux := http.NewServeMux()
	mux.Handle(recipev1connect.NewRecipeServiceHandler(recipe.NewService(repo)))

	log.Printf("server: listening on %s", addr)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		log.Fatalf("server: %v", err)
	}
```
Leave the `withCORS` function (below `main`) unchanged.

- [ ] **Step 2: Delete the foo backend tree**

Run:
```bash
rm -rf internal/foo
```
This removes `internal/foo/service.go`, `internal/foo/v1/foo.proto`, the generated `internal/foo/v1/foo.pb.go`, and `internal/foo/v1/foov1connect/`.

- [ ] **Step 3: Confirm codegen is consistent (no foo regenerated)**

Run: `make gen && git status --porcelain internal/foo`
Expected: `make gen` succeeds and produces no `internal/foo` files (foo.proto is gone); `git status` shows only deletions under `internal/foo` (the committed `service.go` and `v1/foo.proto`).

- [ ] **Step 4: Verify build and full test suite**

Run: `go build ./... && go test ./...`
Expected: build succeeds (no references to `foo` remain); `ok` for `internal/db` and `internal/recipe`.

- [ ] **Step 5: Verify the server serves RecipeService**

Run:
```bash
DINNERWISE_DB="$(mktemp -d)/dinnerwise.db" ADDR=:8097 go run ./cmd/server/ &
sleep 2
curl -s -X POST http://localhost:8097/internal.recipe.v1.RecipeService/ListRecipes \
  -H "Content-Type: application/json" -H "Connect-Protocol-Version: 1" -d '{}'
echo
kill %1 2>/dev/null
```
Expected: a boot log line `server: 3 recipes loaded from ...`, and a JSON response whose `recipes` array has 3 entries (names include "Weeknight Tomato Pasta").

- [ ] **Step 6: Commit**

```bash
git add cmd/server/main.go
git add -A internal/foo
git commit -m "feat: serve RecipeService; remove foo backend

Mount RecipeService handler (build repo once), drop the foo example service,
proto, and generated code.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Recipes UI (/recipes + /recipes/$id), remove foo frontend

**Files:**
- Create: `web/app/src/routes/recipes.tsx`
- Create: `web/app/src/routes/recipe.tsx`
- Modify: `web/app/src/router.tsx`
- Modify: `web/app/src/routes/__root.tsx`
- Modify: `web/app/src/routes/index.tsx`
- Delete: `web/app/src/routes/foo.tsx`, `web/app/src/gen/internal/foo/`

**Interfaces:**
- Consumes: generated `listRecipes`, `getRecipe` from `../gen/internal/recipe/v1/recipe-RecipeService_connectquery` (produced by `make gen` in Task 1); `rootRoute` from `./__root`; existing `transport`/providers.
- Produces: `recipesRoute` (`/recipes`), `recipeDetailRoute` (`/recipes/$id`) registered in `router.tsx`.

Note: there is no frontend test harness in this project; this task is verified by `tsc -b` (type-checks the typed routes/links/params), `eslint`, `vite build`, and a runtime check — not unit tests.

- [ ] **Step 1: Create the list route**

Create `web/app/src/routes/recipes.tsx`:
```tsx
import { createRoute, Link } from "@tanstack/react-router";
import { useQuery } from "@connectrpc/connect-query";
import { listRecipes } from "../gen/internal/recipe/v1/recipe-RecipeService_connectquery";
import { rootRoute } from "./__root";

function Recipes() {
  const { data, error, isPending } = useQuery(listRecipes, {});

  if (isPending) return <p className="text-muted-foreground">Loading…</p>;
  if (error)
    return <p className="text-red-600 dark:text-red-400">{error.message}</p>;

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold">Recipes</h1>
      <ul className="grid gap-4 sm:grid-cols-2">
        {data.recipes.map((r) => (
          <li key={r.id}>
            <Link
              to="/recipes/$id"
              params={{ id: r.id }}
              className="block rounded-lg border border-border bg-card p-4 text-card-foreground hover:border-primary"
            >
              <h2 className="font-medium">{r.name}</h2>
              <p className="mt-1 text-sm text-muted-foreground">
                ⏱ {r.totalMinutes} min · serves {r.servings}
              </p>
              <div className="mt-3 flex flex-wrap gap-1">
                {r.ingredients.map((ing) => (
                  <span
                    key={ing.ingredientId}
                    className="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground"
                  >
                    {ing.name}
                  </span>
                ))}
              </div>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}

export const recipesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/recipes",
  component: Recipes,
});
```

- [ ] **Step 2: Create the detail route**

Create `web/app/src/routes/recipe.tsx`:
```tsx
import { createRoute, getRouteApi, Link } from "@tanstack/react-router";
import { useQuery } from "@connectrpc/connect-query";
import { getRecipe } from "../gen/internal/recipe/v1/recipe-RecipeService_connectquery";
import { rootRoute } from "./__root";

const routeApi = getRouteApi("/recipes/$id");

function RecipeDetail() {
  const { id } = routeApi.useParams();
  const { data, error, isPending } = useQuery(getRecipe, { id });

  if (isPending) return <p className="text-muted-foreground">Loading…</p>;
  if (error)
    return <p className="text-red-600 dark:text-red-400">{error.message}</p>;

  const recipe = data.recipe;
  if (!recipe) return <p className="text-muted-foreground">Not found.</p>;

  return (
    <article className="space-y-4">
      <Link
        to="/recipes"
        className="text-sm text-muted-foreground hover:text-foreground"
      >
        ← Recipes
      </Link>
      <h1 className="text-2xl font-semibold">{recipe.name}</h1>
      <p className="text-sm text-muted-foreground">
        ⏱ {recipe.totalMinutes} min · serves {recipe.servings}
      </p>
      <section>
        <h2 className="font-medium">Ingredients</h2>
        <ul className="mt-2 space-y-1 text-sm">
          {recipe.ingredients.map((ing) => (
            <li key={ing.ingredientId} className="text-foreground">
              {ing.quantity} {ing.unit} {ing.name}
            </li>
          ))}
        </ul>
      </section>
      <section>
        <h2 className="font-medium">Instructions</h2>
        <p className="mt-2 whitespace-pre-line text-foreground">
          {recipe.instructions}
        </p>
      </section>
    </article>
  );
}

export const recipeDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/recipes/$id",
  component: RecipeDetail,
});
```

- [ ] **Step 3: Register routes, remove foo route**

Replace the entire contents of `web/app/src/router.tsx` with:
```tsx
import { createRouter } from "@tanstack/react-router";
import { rootRoute } from "./routes/__root";
import { indexRoute } from "./routes/index";
import { recipesRoute } from "./routes/recipes";
import { recipeDetailRoute } from "./routes/recipe";

const routeTree = rootRoute.addChildren([
  indexRoute,
  recipesRoute,
  recipeDetailRoute,
]);

export const router = createRouter({ routeTree });

// Register the router instance for full type inference across the app.
declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
```

- [ ] **Step 4: Update nav**

In `web/app/src/routes/__root.tsx`, replace the foo `<Link>` block:
```tsx
            <Link
              to="/foo"
              search={{ id: "123" }}
              className="text-muted-foreground [&.active]:text-foreground"
            >
              Foo
            </Link>
```
with:
```tsx
            <Link
              to="/recipes"
              className="text-muted-foreground [&.active]:text-foreground"
            >
              Recipes
            </Link>
```

- [ ] **Step 5: Update the home CTA**

In `web/app/src/routes/index.tsx`, replace the foo `<Link>` block:
```tsx
      <Link
        to="/foo"
        search={{ id: "123" }}
        className="inline-block rounded-lg bg-primary px-4 py-2 text-primary-foreground hover:opacity-90"
      >
        Try the Foo demo →
      </Link>
```
with:
```tsx
      <Link
        to="/recipes"
        className="inline-block rounded-lg bg-primary px-4 py-2 text-primary-foreground hover:opacity-90"
      >
        Browse recipes →
      </Link>
```

- [ ] **Step 6: Delete the foo frontend**

Run:
```bash
git rm web/app/src/routes/foo.tsx
rm -rf web/app/src/gen/internal/foo
```

- [ ] **Step 7: Verify typecheck, lint, and build**

Run:
```bash
cd web/app && npx tsc -b && pnpm lint && pnpm build
```
Expected: `tsc` clean (typed `to="/recipes/$id"` + `params`/`useParams` resolve), eslint reports 0 problems, `vite build` succeeds. (Return to repo root afterward: `cd ../..`)

- [ ] **Step 8: Runtime check (recipes render end-to-end)**

Run (from repo root):
```bash
DINNERWISE_DB="$(mktemp -d)/dinnerwise.db" ADDR=:8096 go run ./cmd/server/ &
(cd web/app && VITE_API_URL=http://localhost:8096 pnpm dev --port 5174 &)
sleep 4
curl -s http://localhost:5174/recipes -o /dev/null -w "vite %{http_code}\n"
curl -s -X POST http://localhost:8096/internal.recipe.v1.RecipeService/ListRecipes \
  -H "Content-Type: application/json" -H "Connect-Protocol-Version: 1" -d '{}' | head -c 200
echo
kill %1 %2 2>/dev/null; pkill -f "vite" 2>/dev/null
```
Expected: `vite 200`, and the ListRecipes JSON shows the 3 seeded recipes. (Full visual confirmation of the rendered page is the controller's runtime check.)

- [ ] **Step 9: Commit**

```bash
git add web/app/src/routes/recipes.tsx web/app/src/routes/recipe.tsx web/app/src/router.tsx web/app/src/routes/__root.tsx web/app/src/routes/index.tsx
git rm web/app/src/routes/foo.tsx
git commit -m "feat: recipes UI (/recipes + /recipes/\$id); remove foo frontend

List page with recipe cards linking to a typed detail route; nav and home
CTA point at recipes; foo route and generated TS removed.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage:**
- Proto contract (ListRecipes/GetRecipe, messages mirroring repo) → Task 1.
- Codegen Go + TS via buf → Task 1 Step 2 (`make gen`).
- Service mapping repo↔proto, ErrNotFound→CodeNotFound → Task 1 (`service.go`, tests).
- Server wiring (mount handler, build repo once) → Task 2.
- Remove foo (backend tree + frontend route/gen + nav/home/router refs) → Tasks 2 (backend) and 3 (frontend).
- `/recipes` list with cards (name, time, servings, ingredient chips) → Task 3 Step 1.
- `/recipes/$id` detail (typed param, instructions + ingredients, back link) → Task 3 Step 2.
- Nav `Home | Recipes`, home CTA → Task 3 Steps 4–5.
- Testing: service tests (list/get/not-found) → Task 1; tsc/eslint/build/runtime → Task 3 Steps 7–8.
- No import cycle / generated code gitignored / service in package recipe → Global Constraints + Task 1.

**Placeholder scan:** none — all steps contain concrete code and commands.

**Type consistency:** `NewService(repo *Repo) recipev1connect.RecipeServiceHandler`, `toProtoRecipe`, and the `recipev1`/`recipev1connect` aliases are used identically in Tasks 1–2. Proto field names (`Id`, `TotalMinutes`, `IngredientId`) match generated Go; TS camelCase (`totalMinutes`, `ingredientId`) matches generated TS. Connect-Query exports `listRecipes`/`getRecipe` match the proto rpc names. Route names `recipesRoute`/`recipeDetailRoute` are defined in Task 3 and registered in the same task.
