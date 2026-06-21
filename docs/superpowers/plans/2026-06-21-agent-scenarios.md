# Agent Reference Cards + Scenarios + Replay (Slice 4) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend the scripted agent with the design's scenarios (favorites/tonight/quick/ingredient) that query the real repos to navigate and return reference cards, plus a replay action — no LLM.

**Architecture:** The agent's `script(text)` becomes `respond(ctx, recipes, meals, text)` that queries `recipe.Repo`/`meal.Repo` per intent and emits thinking → tool_call → text → `Reference` events → done. The dock renders reference cards (→ detail pages) and a replay button; `/recipes` gains `pantry`/`maxMinutes` filters that the scenarios navigate to.

**Tech Stack:** Go 1.25, ConnectRPC server-streaming, buf v2, React 19 + Vite, TanStack Router, Connect-Query, Tailwind v4.

## Global Constraints

- No LLM — behavior is hand-written intent matching + canned templates, but results come from real repo queries.
- Agent service depends on `recipe.Repo` + `meal.Repo` (no import cycle). Generated code gitignored (`make gen`); commit only hand-written files.
- Scenario → navigate: favorites `/meals{sort:rating,fav:1}`, tonight `/recipes{pantry:1}`, quick `/recipes{maxMinutes:30}`, ingredient `/recipes{ingredient:X}`.
- quick = `total_minutes ≤ 30` (data-driven; design said 20). References capped at 3 per scenario.
- Reference subtitles: recipe → `"{cuisine} · {totalMinutes} min"`; meal → `"{rating}★ · cooked {timesCooked}×"`.
- No `any` on the frontend; reference/navigate use the `NavigateOptions` cast.
- Module path `github.com/sethlowie/dinnerwise`. Commit messages end with: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

---

## File Structure

- Modify: `internal/agent/v1/agent.proto` (Reference event), `internal/agent/script.go` (→ scenario engine), `internal/agent/service.go` (repo deps, respond), `internal/agent/script_test.go` + `internal/agent/service_test.go` (scenario tests), `cmd/server/main.go` (wiring).
- Modify: `web/app/src/routes/recipes.tsx` (pantry + maxMinutes filters).
- Modify: `web/app/src/chat/chatContext.ts`, `web/app/src/chat/ChatProvider.tsx`, `web/app/src/chat/ChatPanel.tsx` (references + replay + cards).

---

## Task 1: Agent scenarios + reference protocol (backend)

**Files:** Modify `internal/agent/v1/agent.proto`, `script.go`, `service.go`, `script_test.go`, `service_test.go`, `cmd/server/main.go`.

**Interfaces:**
- Consumes: `recipe.Repo` (`List(ctx)`, `Recipe{ID,Name,Cuisine,TotalMinutes,Ingredients,InPantry}`), `meal.Repo` (`List(ctx,sort,favoritesOnly)`, `Meal{ID,Name,Rating,TimesCooked}`).
- Produces: proto `Reference` (oneof case 6); `respond(ctx, *recipe.Repo, *meal.Repo, text) ([]*agentv1.AskEvent, error)`; `NewService(recipes *recipe.Repo, meals *meal.Repo) agentv1connect.AgentServiceHandler`; `NewServiceWithDelay(recipes, meals, d)`.

- [ ] **Step 1: Add the Reference event to the proto**

In `internal/agent/v1/agent.proto`, add the message (after `Done`):
```proto
message Reference {
  string kind = 1;
  string id = 2;
  string title = 3;
  string subtitle = 4;
}
```
and add to the `AskEvent` oneof (after `Done done = 5;`):
```proto
    Reference reference = 6;
```

- [ ] **Step 2: Regenerate**

Run: `make gen`
Expected: regenerated Go/TS include `Reference` and `AskEvent_Reference`.

- [ ] **Step 3: Write the failing scenario tests**

Replace the entire contents of `internal/agent/script_test.go` with:
```go
package agent

import (
	"context"
	"path/filepath"
	"testing"

	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
	"github.com/sethlowie/dinnerwise/internal/db"
	"github.com/sethlowie/dinnerwise/internal/meal"
	"github.com/sethlowie/dinnerwise/internal/recipe"
)

// newTestRepos returns recipe+meal repos backed by a migrated, seeded temp DB
// (recipes seeded before meals so meal FKs resolve).
func newTestRepos(t *testing.T) (*recipe.Repo, *meal.Repo) {
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
	if err := meal.Migrate(database); err != nil {
		t.Fatalf("meal migrate: %v", err)
	}
	if err := meal.SeedIfEmpty(database); err != nil {
		t.Fatalf("meal seed: %v", err)
	}
	return recipe.NewRepo(database), meal.NewRepo(database)
}

func navOf(events []*agentv1.AskEvent) *agentv1.Navigate {
	for _, e := range events {
		if n, ok := e.Event.(*agentv1.AskEvent_Navigate); ok {
			return n.Navigate
		}
	}
	return nil
}

func refsOf(events []*agentv1.AskEvent) []*agentv1.Reference {
	var out []*agentv1.Reference
	for _, e := range events {
		if r, ok := e.Event.(*agentv1.AskEvent_Reference); ok {
			out = append(out, r.Reference)
		}
	}
	return out
}

func TestRespondFavorites(t *testing.T) {
	recipes, meals := newTestRepos(t)
	events, err := respond(context.Background(), recipes, meals, "what are my favorite meals")
	if err != nil {
		t.Fatalf("respond: %v", err)
	}
	nav := navOf(events)
	if nav == nil || nav.GetTo() != "/meals" || nav.GetSearch()["sort"] != "rating" || nav.GetSearch()["fav"] != "1" {
		t.Fatalf("favorites navigate wrong: %+v", nav)
	}
	refs := refsOf(events)
	if len(refs) == 0 || refs[0].GetKind() != "meal" {
		t.Fatalf("favorites refs wrong: %+v", refs)
	}
}

func TestRespondTonight(t *testing.T) {
	recipes, meals := newTestRepos(t)
	events, err := respond(context.Background(), recipes, meals, "what can I cook tonight")
	if err != nil {
		t.Fatalf("respond: %v", err)
	}
	nav := navOf(events)
	if nav == nil || nav.GetTo() != "/recipes" || nav.GetSearch()["pantry"] != "1" {
		t.Fatalf("tonight navigate wrong: %+v", nav)
	}
	ids := map[string]bool{}
	for _, r := range refsOf(events) {
		if r.GetKind() != "recipe" {
			t.Fatalf("tonight ref not a recipe: %+v", r)
		}
		ids[r.GetId()] = true
	}
	if !ids["tomato-pasta"] {
		t.Fatal("tonight should include in-pantry tomato-pasta")
	}
	if ids["veggie-stir-fry"] {
		t.Fatal("tonight should NOT include not-in-pantry veggie-stir-fry")
	}
}

func TestRespondQuick(t *testing.T) {
	recipes, meals := newTestRepos(t)
	events, err := respond(context.Background(), recipes, meals, "suggest something quick")
	if err != nil {
		t.Fatalf("respond: %v", err)
	}
	nav := navOf(events)
	if nav == nil || nav.GetTo() != "/recipes" || nav.GetSearch()["maxMinutes"] != "30" {
		t.Fatalf("quick navigate wrong: %+v", nav)
	}
	for _, r := range refsOf(events) {
		if r.GetId() == "sheet-pan-chicken" {
			t.Fatal("quick should exclude 40-min sheet-pan-chicken")
		}
	}
}

func TestRespondIngredient(t *testing.T) {
	recipes, meals := newTestRepos(t)
	events, err := respond(context.Background(), recipes, meals, "recipes with chicken")
	if err != nil {
		t.Fatalf("respond: %v", err)
	}
	nav := navOf(events)
	if nav == nil || nav.GetTo() != "/recipes" || nav.GetSearch()["ingredient"] != "chicken" {
		t.Fatalf("ingredient navigate wrong: %+v", nav)
	}
	found := false
	for _, r := range refsOf(events) {
		if r.GetId() == "sheet-pan-chicken" {
			found = true
		}
	}
	if !found {
		t.Fatal("ingredient=chicken should reference sheet-pan-chicken")
	}
}

func TestRespondNone(t *testing.T) {
	recipes, meals := newTestRepos(t)
	events, err := respond(context.Background(), recipes, meals, "hello there")
	if err != nil {
		t.Fatalf("respond: %v", err)
	}
	if navOf(events) != nil {
		t.Fatal("none should not navigate")
	}
	if len(refsOf(events)) != 0 {
		t.Fatal("none should have no references")
	}
	if _, ok := events[len(events)-1].Event.(*agentv1.AskEvent_Done); !ok {
		t.Fatal("expected Done last")
	}
}

func TestRespondOrdering(t *testing.T) {
	recipes, meals := newTestRepos(t)
	events, _ := respond(context.Background(), recipes, meals, "what are my favorites")
	// thinking(0) before tool_call before text before reference before done.
	idx := func(match func(*agentv1.AskEvent) bool) int {
		for i, e := range events {
			if match(e) {
				return i
			}
		}
		return -1
	}
	tool := idx(func(e *agentv1.AskEvent) bool { _, ok := e.Event.(*agentv1.AskEvent_ToolCall); return ok })
	text := idx(func(e *agentv1.AskEvent) bool { _, ok := e.Event.(*agentv1.AskEvent_Text); return ok })
	ref := idx(func(e *agentv1.AskEvent) bool { _, ok := e.Event.(*agentv1.AskEvent_Reference); return ok })
	done := idx(func(e *agentv1.AskEvent) bool { _, ok := e.Event.(*agentv1.AskEvent_Done); return ok })
	if !(tool < text && text < ref && ref < done) {
		t.Fatalf("ordering wrong: tool=%d text=%d ref=%d done=%d", tool, text, ref, done)
	}
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `go test ./internal/agent/ -run TestRespond`
Expected: FAIL — `undefined: respond`.

- [ ] **Step 5: Rewrite the scenario engine**

Replace the entire contents of `internal/agent/script.go` with:
```go
package agent

import (
	"context"
	"fmt"
	"strings"

	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
	"github.com/sethlowie/dinnerwise/internal/meal"
	"github.com/sethlowie/dinnerwise/internal/recipe"
)

// knownIngredients are the keywords the scripted (no-LLM) agent recognizes.
var knownIngredients = []string{
	"chicken", "tomato", "tofu", "garlic", "broccoli", "rice", "pasta",
}

// intentFrom maps user text to a scenario (keyword-scripted, first match wins).
// For the ingredient scenario it also returns the matched ingredient keyword.
func intentFrom(text string) (intent, ingredient string) {
	t := strings.ToLower(text)
	switch {
	case strings.Contains(t, "favorite") || strings.Contains(t, "best"):
		return "favorites", ""
	case strings.Contains(t, "quick") || strings.Contains(t, "fast"):
		return "quick", ""
	case strings.Contains(t, "tonight") || strings.Contains(t, "cook") ||
		strings.Contains(t, "dinner") || strings.Contains(t, "have"):
		return "tonight", ""
	}
	for _, ing := range knownIngredients {
		if strings.Contains(t, ing) {
			return "ingredient", ing
		}
	}
	return "none", ""
}

// respond is the scripted agent: it queries the real repos for the matched
// intent and returns the event sequence to stream. Not pure (DB access) but
// deterministic given the data. The real LLM agent would replace this, calling
// the same repos as tools.
func respond(ctx context.Context, recipes *recipe.Repo, meals *meal.Repo, text string) ([]*agentv1.AskEvent, error) {
	intent, ing := intentFrom(text)
	switch intent {
	case "favorites":
		ms, err := meals.List(ctx, "rating", true)
		if err != nil {
			return nil, err
		}
		evs := []*agentv1.AskEvent{
			thinkingEvent("Opening your Meals log…"),
			thinkingEvent("Sorting by your ratings…"),
			toolCallEvent("search_meals", "favorites · rating>=4"),
		}
		evs = append(evs, textChunks(fmt.Sprintf(
			"You've rated %d meals four stars or higher. Here are your favorites.", len(ms)))...)
		evs = append(evs, navigateEvent("/meals", map[string]string{"sort": "rating", "fav": "1"}))
		top := ms
		if len(top) > 3 {
			top = top[:3]
		}
		for _, m := range top {
			evs = append(evs, referenceEvent("meal", m.ID, m.Name, mealSubtitle(m)))
		}
		return append(evs, doneEvent()), nil

	case "tonight":
		rs, err := recipes.List(ctx)
		if err != nil {
			return nil, err
		}
		var have []recipe.Recipe
		for _, r := range rs {
			if r.InPantry {
				have = append(have, r)
			}
		}
		evs := []*agentv1.AskEvent{
			thinkingEvent("Checking what's in your pantry…"),
			thinkingEvent("Matching recipes you can make now…"),
			toolCallEvent("search_recipes", "in_pantry=true"),
		}
		evs = append(evs, textChunks(fmt.Sprintf(
			"You can cook %d recipes tonight without a shop.", len(have)))...)
		evs = append(evs, navigateEvent("/recipes", map[string]string{"pantry": "1"}))
		evs = append(evs, recipeRefs(have)...)
		return append(evs, doneEvent()), nil

	case "quick":
		rs, err := recipes.List(ctx)
		if err != nil {
			return nil, err
		}
		var quick []recipe.Recipe
		for _, r := range rs {
			if r.TotalMinutes <= 30 {
				quick = append(quick, r)
			}
		}
		evs := []*agentv1.AskEvent{
			thinkingEvent("Filtering by cook time…"),
			thinkingEvent("Keeping 30 minutes or less…"),
			toolCallEvent("search_recipes", "max_minutes=30"),
		}
		evs = append(evs, textChunks(fmt.Sprintf(
			"%d recipes come in at 30 minutes or less.", len(quick)))...)
		evs = append(evs, navigateEvent("/recipes", map[string]string{"maxMinutes": "30"}))
		evs = append(evs, recipeRefs(quick)...)
		return append(evs, doneEvent()), nil

	case "ingredient":
		rs, err := recipes.List(ctx)
		if err != nil {
			return nil, err
		}
		var match []recipe.Recipe
		for _, r := range rs {
			for _, i := range r.Ingredients {
				if strings.Contains(strings.ToLower(i.Name), ing) {
					match = append(match, r)
					break
				}
			}
		}
		evs := []*agentv1.AskEvent{
			thinkingEvent("Searching recipes with " + ing + "…"),
			toolCallEvent("search_recipes", "ingredient="+ing),
		}
		evs = append(evs, textChunks("Here are the recipes with "+ing+".")...)
		evs = append(evs, navigateEvent("/recipes", map[string]string{"ingredient": ing}))
		evs = append(evs, recipeRefs(match)...)
		return append(evs, doneEvent()), nil

	default:
		return []*agentv1.AskEvent{
			thinkingEvent("Hmm, let me think…"),
			textEvent("I can help you find recipes and meals — try \"what are my favorites\", \"what can I cook tonight\", \"something quick\", or an ingredient like chicken."),
			doneEvent(),
		}, nil
	}
}

func recipeRefs(rs []recipe.Recipe) []*agentv1.AskEvent {
	if len(rs) > 3 {
		rs = rs[:3]
	}
	out := make([]*agentv1.AskEvent, 0, len(rs))
	for _, r := range rs {
		out = append(out, referenceEvent("recipe", r.ID, r.Name, recipeSubtitle(r)))
	}
	return out
}

func recipeSubtitle(r recipe.Recipe) string {
	return fmt.Sprintf("%s · %d min", r.Cuisine, r.TotalMinutes)
}

func mealSubtitle(m meal.Meal) string {
	return fmt.Sprintf("%d★ · cooked %d×", m.Rating, m.TimesCooked)
}

// textChunks splits a reply into a few TextDelta events to mimic token streaming.
func textChunks(s string) []*agentv1.AskEvent {
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}
	const n = 3
	size := (len(words) + n - 1) / n
	var out []*agentv1.AskEvent
	for i := 0; i < len(words); i += size {
		end := i + size
		if end > len(words) {
			end = len(words)
		}
		chunk := strings.Join(words[i:end], " ")
		if end < len(words) {
			chunk += " "
		}
		out = append(out, textEvent(chunk))
	}
	return out
}

func textEvent(s string) *agentv1.AskEvent {
	return &agentv1.AskEvent{Event: &agentv1.AskEvent_Text{Text: &agentv1.TextDelta{Text: s}}}
}

func thinkingEvent(s string) *agentv1.AskEvent {
	return &agentv1.AskEvent{Event: &agentv1.AskEvent_Thinking{Thinking: &agentv1.Thinking{Text: s}}}
}

func toolCallEvent(name, detail string) *agentv1.AskEvent {
	return &agentv1.AskEvent{Event: &agentv1.AskEvent_ToolCall{ToolCall: &agentv1.ToolCall{Name: name, Detail: detail}}}
}

func navigateEvent(to string, search map[string]string) *agentv1.AskEvent {
	return &agentv1.AskEvent{Event: &agentv1.AskEvent_Navigate{Navigate: &agentv1.Navigate{To: to, Search: search}}}
}

func referenceEvent(kind, id, title, subtitle string) *agentv1.AskEvent {
	return &agentv1.AskEvent{Event: &agentv1.AskEvent_Reference{Reference: &agentv1.Reference{
		Kind: kind, Id: id, Title: title, Subtitle: subtitle,
	}}}
}

func doneEvent() *agentv1.AskEvent {
	return &agentv1.AskEvent{Event: &agentv1.AskEvent_Done{Done: &agentv1.Done{}}}
}
```

- [ ] **Step 6: Run scenario tests to verify they pass**

Run: `go test ./internal/agent/ -run TestRespond`
Expected: PASS.

- [ ] **Step 7: Rewrite the service to take repos and stream respond**

Replace the entire contents of `internal/agent/service.go` with:
```go
// Package agent holds the (currently scripted, no-LLM) AgentService: it streams
// typed events — assistant text plus meta (thinking, tool calls), a navigate
// action, and reference cards — in response to user text. Scenarios query the
// recipe/meal repos, prefiguring the tools a real LLM agent would call.
package agent

import (
	"context"
	"time"

	connect "connectrpc.com/connect"
	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
	"github.com/sethlowie/dinnerwise/internal/agent/v1/agentv1connect"
	"github.com/sethlowie/dinnerwise/internal/meal"
	"github.com/sethlowie/dinnerwise/internal/recipe"
)

// Service implements agentv1connect.AgentServiceHandler by streaming the events
// produced by respond(). delay paces the stream to simulate token streaming.
type Service struct {
	recipes *recipe.Repo
	meals   *meal.Repo
	delay   time.Duration
}

// NewService returns a handler with a lifelike streaming delay.
func NewService(recipes *recipe.Repo, meals *meal.Repo) agentv1connect.AgentServiceHandler {
	return &Service{recipes: recipes, meals: meals, delay: 60 * time.Millisecond}
}

// NewServiceWithDelay returns a handler with an explicit delay (use 0 in tests).
func NewServiceWithDelay(recipes *recipe.Repo, meals *meal.Repo, d time.Duration) agentv1connect.AgentServiceHandler {
	return &Service{recipes: recipes, meals: meals, delay: d}
}

func (s *Service) Ask(
	ctx context.Context,
	req *connect.Request[agentv1.AskRequest],
	stream *connect.ServerStream[agentv1.AskEvent],
) error {
	events, err := respond(ctx, s.recipes, s.meals, req.Msg.GetText())
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	for _, ev := range events {
		if err := stream.Send(ev); err != nil {
			return err
		}
		if s.delay > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(s.delay):
			}
		}
	}
	return nil
}
```

- [ ] **Step 8: Update the streaming integration test**

Replace the entire contents of `internal/agent/service_test.go` with:
```go
package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	connect "connectrpc.com/connect"
	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
	"github.com/sethlowie/dinnerwise/internal/agent/v1/agentv1connect"
)

func TestAskStreamsScenarioEvents(t *testing.T) {
	recipes, meals := newTestRepos(t)
	mux := http.NewServeMux()
	mux.Handle(agentv1connect.NewAgentServiceHandler(NewServiceWithDelay(recipes, meals, 0)))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := agentv1connect.NewAgentServiceClient(http.DefaultClient, srv.URL)
	stream, err := client.Ask(context.Background(),
		connect.NewRequest(&agentv1.AskRequest{Text: "what are my favorites"}))
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}

	var sawNavigate, sawReference, sawDone bool
	for stream.Receive() {
		switch stream.Msg().Event.(type) {
		case *agentv1.AskEvent_Navigate:
			sawNavigate = true
		case *agentv1.AskEvent_Reference:
			sawReference = true
		case *agentv1.AskEvent_Done:
			sawDone = true
		}
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream error: %v", err)
	}
	if !sawNavigate || !sawReference || !sawDone {
		t.Fatalf("missing events: navigate=%v reference=%v done=%v", sawNavigate, sawReference, sawDone)
	}
}
```

- [ ] **Step 9: Wire the repos into the server**

In `cmd/server/main.go`, replace the line:
```go
	mux.Handle(agentv1connect.NewAgentServiceHandler(agent.NewService()))
```
with:
```go
	mux.Handle(agentv1connect.NewAgentServiceHandler(
		agent.NewService(recipe.NewRepo(database), meal.NewRepo(database)),
	))
```
(`recipe` and `meal` are already imported in main.go.)

- [ ] **Step 10: Verify the full suite + boot**

Run:
```bash
go vet ./... && go test ./... 2>&1 | grep -E "ok|FAIL"
go build ./...
```
Expected: `ok` for `internal/agent` and all packages; build clean.

- [ ] **Step 11: Commit**

Generated code is gitignored; commit only the hand-written files.
```bash
git add internal/agent/v1/agent.proto internal/agent/script.go internal/agent/service.go internal/agent/script_test.go internal/agent/service_test.go cmd/server/main.go
git commit -m "feat: agent scenarios (favorites/tonight/quick) + reference cards

Add a Reference event; the scripted agent now queries the recipe/meal repos
per intent to navigate and return reference cards. Service takes repo deps.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: /recipes pantry + maxMinutes filters (frontend)

**Files:** Modify `web/app/src/routes/recipes.tsx`.

**Interfaces:**
- Consumes: generated TS `Recipe` (`inPantry`, `totalMinutes`, `ingredients`).
- Produces: `recipesRoute` accepts `pantry?: boolean` and `maxMinutes?: number` search params (plus `ingredient`); these are the targets the tonight/quick scenarios navigate to.

Note: no frontend unit tests — verify with `tsc -b`/`eslint`/`vite build` + runtime.

- [ ] **Step 1: Replace recipes.tsx with the multi-filter version**

Replace the entire contents of `web/app/src/routes/recipes.tsx` with:
```tsx
import { createRoute, getRouteApi, Link } from "@tanstack/react-router";
import { useQuery } from "@connectrpc/connect-query";
import { listRecipes } from "../gen/internal/recipe/v1/recipe-RecipeService_connectquery";
import { rootRoute } from "./__root";
import { initials, thumbStyle, tintFor } from "../lib/thumb";

const routeApi = getRouteApi("/recipes");

type RecipeSearch = {
  ingredient?: string;
  pantry?: boolean;
  maxMinutes?: number;
};

function Recipes() {
  const { ingredient, pantry, maxMinutes } = routeApi.useSearch();
  const navigate = routeApi.useNavigate();
  const { data, error, isPending } = useQuery(listRecipes, {});

  if (isPending) return <p className="text-muted-foreground">Loading…</p>;
  if (error)
    return <p className="text-red-600 dark:text-red-400">{error.message}</p>;

  const term = ingredient?.toLowerCase().trim() ?? "";
  const recipes = data.recipes.filter((r) => {
    if (term && !r.ingredients.some((i) => i.name.toLowerCase().includes(term)))
      return false;
    if (pantry && !r.inPantry) return false;
    if (maxMinutes !== undefined && r.totalMinutes > maxMinutes) return false;
    return true;
  });

  const clear = (key: keyof RecipeSearch) =>
    navigate({ search: (p: RecipeSearch) => ({ ...p, [key]: undefined }) });

  return (
    <div className="space-y-6">
      <div>
        <div className="mb-2 font-mono text-xs uppercase tracking-[0.12em] text-muted-foreground">
          Your kitchen
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <h1 className="text-3xl font-semibold tracking-tight">Recipes</h1>
          {ingredient && (
            <button
              onClick={() => clear("ingredient")}
              className="rounded-full border border-primary/40 bg-accent px-3 py-1 text-xs text-accent-foreground"
            >
              ingredient: {ingredient} ✕
            </button>
          )}
          {pantry && (
            <button
              onClick={() => clear("pantry")}
              className="rounded-full border border-emerald-500/40 bg-emerald-500/10 px-3 py-1 text-xs text-emerald-400"
            >
              in pantry ✕
            </button>
          )}
          {maxMinutes !== undefined && (
            <button
              onClick={() => clear("maxMinutes")}
              className="rounded-full border border-primary/40 bg-accent px-3 py-1 text-xs text-accent-foreground"
            >
              ≤ {maxMinutes} min ✕
            </button>
          )}
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        {recipes.map((r) => {
          const tint = tintFor(r.id);
          return (
            <Link
              key={r.id}
              to="/recipes/$id"
              params={{ id: r.id }}
              className="flex items-center gap-3.5 rounded-2xl border border-border bg-card/60 p-4 transition-colors hover:border-primary/40"
            >
              <div
                className="flex h-12 w-12 flex-none items-center justify-center rounded-xl font-mono text-sm font-semibold"
                style={thumbStyle(tint)}
              >
                {initials(r.name)}
              </div>
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
            </Link>
          );
        })}
      </div>
    </div>
  );
}

export const recipesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/recipes",
  validateSearch: (search: Record<string, unknown>): RecipeSearch => {
    const max =
      typeof search.maxMinutes === "number"
        ? search.maxMinutes
        : typeof search.maxMinutes === "string" && search.maxMinutes !== "" && !Number.isNaN(Number(search.maxMinutes))
          ? Number(search.maxMinutes)
          : undefined;
    return {
      ingredient: typeof search.ingredient === "string" ? search.ingredient : undefined,
      pantry:
        search.pantry === true || search.pantry === "true" || search.pantry === "1"
          ? true
          : undefined,
      maxMinutes: max,
    };
  },
  component: Recipes,
});
```

- [ ] **Step 2: Verify typecheck, lint, build**

Run (from `web/app`): `npx tsc -b && pnpm lint && pnpm build`
Expected: all clean. Return to repo root: `cd ../..`

- [ ] **Step 3: Commit**

```bash
git add web/app/src/routes/recipes.tsx
git commit -m "feat: pantry and maxMinutes filters on /recipes

Typed pantry/maxMinutes search params (string-coerced for agent navigation)
compose with the ingredient filter; clearable chips for each.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Reference cards + replay in the dock (frontend)

**Files:** Modify `web/app/src/chat/chatContext.ts`, `web/app/src/chat/ChatProvider.tsx`, `web/app/src/chat/ChatPanel.tsx`.

**Interfaces:**
- Consumes: generated `reference` event (`{ kind, id, title, subtitle }`); `lib/thumb`; `useRouter`.
- Produces: `AssistantMessage.references`; the dock renders reference cards (→ detail) and a replay button.

- [ ] **Step 1: Add references to the assistant message type**

In `web/app/src/chat/chatContext.ts`, change the `AssistantMessage` type to add `references`:
```ts
export type AssistantMessage = {
  thinking: string[];
  toolCalls: { name: string; detail: string }[];
  text: string;
  done: boolean;
  references: { kind: string; id: string; title: string; subtitle: string }[];
};
```

- [ ] **Step 2: Handle the reference event in the provider**

In `web/app/src/chat/ChatProvider.tsx`:

(a) add `references: []` to `emptyAssistant`:
```ts
const emptyAssistant: AssistantMessage = {
  thinking: [],
  toolCalls: [],
  text: "",
  done: false,
  references: [],
};
```

(b) add a `case` to the event switch (after the `text` case, before `navigate`):
```ts
            case "reference":
              update((a) => ({
                ...a,
                references: [
                  ...a.references,
                  {
                    kind: event.value.kind,
                    id: event.value.id,
                    title: event.value.title,
                    subtitle: event.value.subtitle,
                  },
                ],
              }));
              break;
```

- [ ] **Step 3: Render reference cards + replay in the dock**

In `web/app/src/chat/ChatPanel.tsx`:

(a) update the imports at the top:
```tsx
import { useState, type FormEvent } from "react";
import { useRouter, type NavigateOptions } from "@tanstack/react-router";
import { useChat } from "./chatContext";
import type { Turn } from "./chatContext";
import { initials, thumbStyle, tintFor } from "../lib/thumb";
```

(b) inside `ChatPanel`, after `const { turns, isStreaming, ask } = useChat();`, add:
```tsx
  const router = useRouter();

  function openRef(ref: { kind: string; id: string }) {
    const opts = {
      to: ref.kind === "meal" ? "/meals/$id" : "/recipes/$id",
      params: { id: ref.id },
    } as unknown as NavigateOptions;
    void router.navigate(opts);
  }
  function replay(text: string) {
    if (!isStreaming) ask(text);
  }
```

(c) in the dock turn rendering, immediately after the reply block (the
`{t.assistant.text && ( … )}` element), add the references + replay:
```tsx
              {t.assistant.references.length > 0 && (
                <div className="flex flex-col gap-2">
                  {t.assistant.references.map((ref) => (
                    <button
                      key={`${ref.kind}-${ref.id}`}
                      onClick={() => openRef(ref)}
                      className="flex items-center gap-3 rounded-xl border border-border bg-card/60 p-2.5 text-left transition-colors hover:border-primary/40"
                    >
                      <div
                        className="flex h-9 w-9 flex-none items-center justify-center rounded-lg font-mono text-xs font-semibold"
                        style={thumbStyle(tintFor(ref.id))}
                      >
                        {initials(ref.title)}
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="truncate text-sm font-medium">{ref.title}</div>
                        <div className="truncate font-mono text-xs text-muted-foreground">
                          {ref.subtitle}
                        </div>
                      </div>
                      <span className="flex-none font-mono text-muted-foreground">→</span>
                    </button>
                  ))}
                </div>
              )}

              {t.assistant.done && (
                <button
                  onClick={() => replay(t.userText)}
                  className="self-start font-mono text-xs text-primary hover:opacity-80"
                >
                  ↻ replay this run
                </button>
              )}
```

- [ ] **Step 4: Verify typecheck, lint, build**

Run (from `web/app`): `npx tsc -b && pnpm lint && pnpm build`
Expected: all clean (the `reference` case narrows; `NavigateOptions` cast resolves). Return to repo root: `cd ../..`

- [ ] **Step 5: Runtime check (full scenario flow)**

Run (from repo root):
```bash
DINNERWISE_DB="$(mktemp -d)/dinnerwise.db" ADDR=:8087 go run ./cmd/server/ &
(cd web/app && VITE_API_URL=http://localhost:8087 pnpm dev --port 5180 &)
sleep 4
curl -s -o /dev/null -w "home %{http_code}\n" http://localhost:5180/
kill %1 %2 2>/dev/null; pkill -f vite 2>/dev/null
```
Expected: `home 200`. Controller's visual check: from the hero, ask "what are my favorites" → dock streams steps + reply + **meal reference cards**, left pane navigates to `/meals?sort=rating&fav=1`; a card opens that meal; "what can I cook tonight" → recipe refs + `/recipes?pantry=1`; "something quick" → `/recipes?maxMinutes=30`; "↻ replay this run" re-runs.

- [ ] **Step 6: Commit**

```bash
git add web/app/src/chat/chatContext.ts web/app/src/chat/ChatProvider.tsx web/app/src/chat/ChatPanel.tsx
git commit -m "feat: agent reference cards and replay in the dock

Render the reference event as clickable result cards that open the detail page;
add a replay-this-run action.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage:**
- Reference event in proto → Task 1 Step 1.
- Scenario engine querying repos (favorites/tonight/quick/ingredient/none), navigate targets, references, ordering → Task 1 (`respond`, tests).
- Service repo deps + streaming respond → Task 1 Steps 7–8; wiring → Step 9.
- quick=30, refs capped at 3, subtitle formats → Task 1 (`script.go`).
- /recipes pantry + maxMinutes filters (string-coerced) → Task 2.
- references in AssistantMessage + reference event handling → Task 3 Steps 1–2.
- reference cards (→ detail) + replay → Task 3 Step 3.
- Testing: respond per intent + ordering + streaming integration (Task 1); tsc/eslint/build + runtime (Tasks 2–3).
- Out of scope (LLM, orb) absent.

**Placeholder scan:** none — concrete code/commands throughout.

**Type consistency:** `respond(ctx, *recipe.Repo, *meal.Repo, string) ([]*agentv1.AskEvent, error)` and `NewService(recipes, meals)`/`NewServiceWithDelay(recipes, meals, d)` used consistently across service, tests, and main wiring. Event constructors (`referenceEvent` etc.) and `AskEvent_Reference` match the proto. Frontend `reference` case fields (`kind/id/title/subtitle`) match `AssistantMessage.references` and the generated camelCase. Navigate search keys (`sort`,`fav`,`pantry`,`maxMinutes`,`ingredient`) match `recipesRoute`/`mealsRoute` validateSearch params. `openRef` targets `/meals/$id`÷`/recipes/$id` (existing routes).
