# Real OpenAI Agent (Slice 6a) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the scripted `respond()` with a real gpt-5-nano agent that calls the recipe/meal repos as tools and streams its work over the unchanged `AskEvent` contract, with a scripted fallback when no API key is set.

**Architecture:** A bounded tool-calling loop drives the OpenAI Responses API (non-streaming per round). Repo queries are exposed as function tools; the model also calls a `navigate` tool to drive the UI. Each round's results (thinking, tool_call, reference, navigate) are streamed to the client as they complete; the final assistant text is chunked into `TextDelta` events. The OpenAI call sits behind a narrow `llmClient` interface so the loop is unit-testable offline and slice 6b can wrap it with Sigil.

**Tech Stack:** Go 1.25, ConnectRPC server-streaming, `github.com/openai/openai-go/v3` (Responses API), `github.com/joho/godotenv`, SQLite repos (existing).

## Global Constraints

- OpenAI Go SDK module: `github.com/openai/openai-go/v3` (target v3.41.0). Import the `responses`, `option`, and `shared` subpackages as needed.
- Default model: `gpt-5-nano` (from `OPENAI_MODEL`, fallback to this literal). API key from `OPENAI_API_KEY`.
- The proto / `AskEvent` contract does NOT change. No edits to `internal/agent/v1/*` or `make gen`. No frontend changes.
- Scripted `respond()` stays and is the fallback when `OPENAI_API_KEY` is empty.
- Tool-call loop is capped at **5 rounds** per turn.
- Reference cards are capped at **3** per search result set (matches today).
- `.env` is gitignored; never print secret values in code, logs, or commits.
- Every task ends green: `go build ./...` and `go test ./...`.
- Commit messages end with: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`
- No live OpenAI calls in the default test run. The one live test is gated behind `OPENAI_API_KEY` + `DINNERWISE_LIVE=1` and skipped otherwise.

---

## File Structure

- `internal/config/config.go` (new) — env config (`Config`, `Load`, `HasOpenAI`).
- `internal/config/config_test.go` (new) — defaults + HasOpenAI.
- `internal/agent/filters.go` (new) — shared recipe/meal selection helpers (DRY).
- `internal/agent/filters_test.go` (new).
- `internal/agent/script.go` (modify) — use the shared helpers.
- `internal/agent/tools.go` (new) — tool specs + executors over repos.
- `internal/agent/tools_test.go` (new).
- `internal/agent/llm.go` (new) — `llmClient` interface, `llmAgent`, `Run` loop, system prompt.
- `internal/agent/llm_test.go` (new) — `Run` driven by a stub `llmClient`.
- `internal/agent/openai_client.go` (new) — real `llmClient` over openai-go Responses.
- `internal/agent/service.go` (modify) — `NewService(cfg, recipes, meals)` picks backend; LLM `Ask`.
- `internal/agent/live_test.go` (new) — opt-in end-to-end test (gated).
- `cmd/server/main.go` (modify) — godotenv, config, client wiring, backend log.
- `go.mod` / `go.sum` (modify) — add deps.

---

## Task 1: Config package + dependencies

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Produces: `config.Config{ OpenAIAPIKey string; OpenAIModel string }`, `config.Load() config.Config`, `(Config).HasOpenAI() bool`. `OpenAIModel` defaults to `"gpt-5-nano"`.

- [ ] **Step 1: Add dependencies**

Run:
```bash
go get github.com/openai/openai-go/v3@v3.41.0
go get github.com/joho/godotenv@latest
```
Expected: `go.mod` gains both requires; `go.sum` updated.

- [ ] **Step 2: Write the failing test**

`internal/config/config_test.go`:
```go
package config

import "testing"

func TestLoadDefaultsModel(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENAI_MODEL", "")
	c := Load()
	if c.OpenAIModel != "gpt-5-nano" {
		t.Fatalf("default model = %q, want gpt-5-nano", c.OpenAIModel)
	}
	if c.HasOpenAI() {
		t.Fatal("HasOpenAI() should be false with empty key")
	}
}

func TestLoadReadsEnv(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("OPENAI_MODEL", "gpt-5")
	c := Load()
	if !c.HasOpenAI() {
		t.Fatal("HasOpenAI() should be true")
	}
	if c.OpenAIModel != "gpt-5" {
		t.Fatalf("model = %q, want gpt-5", c.OpenAIModel)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/config/`
Expected: FAIL (package/`Load` undefined).

- [ ] **Step 4: Implement**

`internal/config/config.go`:
```go
// Package config loads runtime configuration from the environment. main loads
// .env (via godotenv) before calling Load, so values may originate there.
package config

import "os"

// Config holds runtime settings. Slice 6b extends this with OTLP/Sigil fields.
type Config struct {
	OpenAIAPIKey string
	OpenAIModel  string
}

// Load reads configuration from the process environment.
func Load() Config {
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-5-nano"
	}
	return Config{
		OpenAIAPIKey: os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:  model,
	}
}

// HasOpenAI reports whether a real OpenAI agent can be constructed.
func (c Config) HasOpenAI() bool { return c.OpenAIAPIKey != "" }
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/config/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/config/
git commit -m "feat: config package + openai/godotenv deps

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Shared selection helpers (DRY) + refactor script.go

**Files:**
- Create: `internal/agent/filters.go`
- Test: `internal/agent/filters_test.go`
- Modify: `internal/agent/script.go`

**Interfaces:**
- Consumes: `recipe.Recipe` (fields `ID,Name,Cuisine,TotalMinutes,InPantry,Ingredients[]{Name}`), `meal.Meal`.
- Produces:
  - `selectByIngredient(rs []recipe.Recipe, ingredient string) []recipe.Recipe`
  - `selectInPantry(rs []recipe.Recipe) []recipe.Recipe`
  - `selectMaxMinutes(rs []recipe.Recipe, max int) []recipe.Recipe`
  - `recipeSubtitle(r recipe.Recipe) string`, `mealSubtitle(m meal.Meal) string` (moved here from script.go).

- [ ] **Step 1: Write the failing test**

`internal/agent/filters_test.go`:
```go
package agent

import (
	"testing"

	"github.com/sethlowie/dinnerwise/internal/recipe"
)

func r(id string, min int, pantry bool, ings ...string) recipe.Recipe {
	var ri []recipe.RecipeIngredient
	for _, n := range ings {
		ri = append(ri, recipe.RecipeIngredient{Name: n})
	}
	return recipe.Recipe{ID: id, TotalMinutes: min, InPantry: pantry, Ingredients: ri}
}

func ids(rs []recipe.Recipe) []string {
	out := []string{}
	for _, x := range rs {
		out = append(out, x.ID)
	}
	return out
}

func TestSelectByIngredient(t *testing.T) {
	rs := []recipe.Recipe{r("a", 0, false, "Chicken Thigh"), r("b", 0, false, "Tofu")}
	got := ids(selectByIngredient(rs, "chicken"))
	if len(got) != 1 || got[0] != "a" {
		t.Fatalf("got %v, want [a]", got)
	}
}

func TestSelectInPantry(t *testing.T) {
	rs := []recipe.Recipe{r("a", 0, true), r("b", 0, false)}
	if got := ids(selectInPantry(rs)); len(got) != 1 || got[0] != "a" {
		t.Fatalf("got %v, want [a]", got)
	}
}

func TestSelectMaxMinutes(t *testing.T) {
	rs := []recipe.Recipe{r("a", 25, false), r("b", 40, false)}
	if got := ids(selectMaxMinutes(rs, 30)); len(got) != 1 || got[0] != "a" {
		t.Fatalf("got %v, want [a]", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -run TestSelect`
Expected: FAIL (helpers undefined).

- [ ] **Step 3: Implement helpers**

`internal/agent/filters.go`:
```go
package agent

import (
	"fmt"
	"strings"

	"github.com/sethlowie/dinnerwise/internal/meal"
	"github.com/sethlowie/dinnerwise/internal/recipe"
)

// selectByIngredient keeps recipes with an ingredient name containing the
// (lower-cased) substring.
func selectByIngredient(rs []recipe.Recipe, ingredient string) []recipe.Recipe {
	ingredient = strings.ToLower(ingredient)
	var out []recipe.Recipe
	for _, r := range rs {
		for _, i := range r.Ingredients {
			if strings.Contains(strings.ToLower(i.Name), ingredient) {
				out = append(out, r)
				break
			}
		}
	}
	return out
}

// selectInPantry keeps recipes the user can cook now.
func selectInPantry(rs []recipe.Recipe) []recipe.Recipe {
	var out []recipe.Recipe
	for _, r := range rs {
		if r.InPantry {
			out = append(out, r)
		}
	}
	return out
}

// selectMaxMinutes keeps recipes at or under max total minutes.
func selectMaxMinutes(rs []recipe.Recipe, max int) []recipe.Recipe {
	var out []recipe.Recipe
	for _, r := range rs {
		if r.TotalMinutes <= max {
			out = append(out, r)
		}
	}
	return out
}

func recipeSubtitle(r recipe.Recipe) string {
	return fmt.Sprintf("%s · %d min", r.Cuisine, r.TotalMinutes)
}

func mealSubtitle(m meal.Meal) string {
	return fmt.Sprintf("%d★ · cooked %d×", m.Rating, m.TimesCooked)
}
```

- [ ] **Step 4: Refactor script.go to use the helpers**

In `internal/agent/script.go`, delete the now-duplicated `recipeSubtitle`/`mealSubtitle` funcs (they live in filters.go now) and replace the inline filter loops with the helpers:
- `tonight` case: replace the manual `for _, r := range rs { if r.InPantry ... }` with `have := selectInPantry(rs)`.
- `quick` case: replace with `quick := selectMaxMinutes(rs, 30)`.
- `ingredient` case: replace the manual ingredient match loop with `match := selectByIngredient(rs, ing)`.

Leave `recipeRefs`, `intentFrom`, event constructors, and the favorites case unchanged.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/agent/`
Expected: PASS — new helper tests AND the existing scripted tests (`script_test.go`, `service_test.go`) stay green (behavior identical).

- [ ] **Step 6: Commit**

```bash
git add internal/agent/filters.go internal/agent/filters_test.go internal/agent/script.go
git commit -m "refactor: extract shared recipe/meal selection helpers

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Tool layer (specs + executors over repos)

**Files:**
- Create: `internal/agent/tools.go`
- Test: `internal/agent/tools_test.go`

**Interfaces:**
- Consumes: `recipe.Repo.List(ctx) ([]recipe.Recipe, error)`, `meal.Repo.List(ctx, sort string, favoritesOnly bool) ([]meal.Meal, error)`, the Task 2 helpers, and `*agentv1.AskEvent` constructors from `script.go` (`referenceEvent`, `navigateEvent`).
- Produces:
  - `type toolResult struct { Events []*agentv1.AskEvent; Summary string }` — `Events` are streamed to the client; `Summary` is the compact JSON returned to the model.
  - `executeTool(ctx context.Context, recipes *recipe.Repo, meals *meal.Repo, name, argsJSON string) (toolResult, error)`
  - `toolDefs() []responses.ToolUnionParam` — the three function-tool definitions for the Responses API.
  - tool name constants: `toolSearchRecipes = "search_recipes"`, `toolSearchMeals = "search_meals"`, `toolNavigate = "navigate"`.

- [ ] **Step 1: Write the failing test**

`internal/agent/tools_test.go` (uses the seed helpers against a temp DB; recipes seeded before meals for FK):
```go
package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sethlowie/dinnerwise/internal/db"
	"github.com/sethlowie/dinnerwise/internal/meal"
	"github.com/sethlowie/dinnerwise/internal/recipe"
)

func seededRepos(t *testing.T) (*recipe.Repo, *meal.Repo) {
	t.Helper()
	database, err := db.Open(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	if err := recipe.Migrate(database); err != nil {
		t.Fatal(err)
	}
	if err := recipe.SeedIfEmpty(database); err != nil {
		t.Fatal(err)
	}
	if err := meal.Migrate(database); err != nil {
		t.Fatal(err)
	}
	if err := meal.SeedIfEmpty(database); err != nil {
		t.Fatal(err)
	}
	return recipe.NewRepo(database), meal.NewRepo(database)
}

func TestExecuteSearchRecipesIngredient(t *testing.T) {
	recipes, meals := seededRepos(t)
	res, err := executeTool(context.Background(), recipes, meals,
		toolSearchRecipes, `{"ingredient":"chicken"}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Events) == 0 {
		t.Fatal("expected reference events")
	}
	if !strings.Contains(strings.ToLower(res.Summary), "chicken") &&
		!json.Valid([]byte(res.Summary)) {
		t.Fatalf("summary should be valid JSON: %q", res.Summary)
	}
}

func TestExecuteNavigate(t *testing.T) {
	recipes, meals := seededRepos(t)
	res, err := executeTool(context.Background(), recipes, meals,
		toolNavigate, `{"to":"/recipes","search":{"pantry":"1"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Events) != 1 || res.Events[0].GetNavigate() == nil {
		t.Fatalf("expected one navigate event, got %#v", res.Events)
	}
	if res.Events[0].GetNavigate().GetTo() != "/recipes" {
		t.Fatalf("navigate.to = %q", res.Events[0].GetNavigate().GetTo())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -run TestExecute`
Expected: FAIL (`executeTool` undefined).

- [ ] **Step 3: Implement**

`internal/agent/tools.go`:
```go
package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/responses"
	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
	"github.com/sethlowie/dinnerwise/internal/meal"
	"github.com/sethlowie/dinnerwise/internal/recipe"
)

const (
	toolSearchRecipes = "search_recipes"
	toolSearchMeals   = "search_meals"
	toolNavigate      = "navigate"
)

type toolResult struct {
	Events  []*agentv1.AskEvent
	Summary string
}

type searchRecipesArgs struct {
	Ingredient string `json:"ingredient"`
	InPantry   bool   `json:"in_pantry"`
	MaxMinutes int    `json:"max_minutes"`
}

type searchMealsArgs struct {
	FavoritesOnly bool   `json:"favorites_only"`
	Sort          string `json:"sort"`
}

type navigateArgs struct {
	To     string            `json:"to"`
	Search map[string]string `json:"search"`
}

// executeTool runs a model tool call and returns events to stream plus a compact
// JSON summary to feed back to the model.
func executeTool(ctx context.Context, recipes *recipe.Repo, meals *meal.Repo, name, argsJSON string) (toolResult, error) {
	switch name {
	case toolSearchRecipes:
		var a searchRecipesArgs
		_ = json.Unmarshal([]byte(argsJSON), &a)
		rs, err := recipes.List(ctx)
		if err != nil {
			return toolResult{}, err
		}
		if a.Ingredient != "" {
			rs = selectByIngredient(rs, a.Ingredient)
		}
		if a.InPantry {
			rs = selectInPantry(rs)
		}
		if a.MaxMinutes > 0 {
			rs = selectMaxMinutes(rs, a.MaxMinutes)
		}
		evs := recipeRefs(rs) // capped at 3, references in dock
		return toolResult{Events: evs, Summary: summarizeRecipes(rs)}, nil

	case toolSearchMeals:
		var a searchMealsArgs
		_ = json.Unmarshal([]byte(argsJSON), &a)
		sort := a.Sort
		if sort == "" {
			sort = "recent"
		}
		ms, err := meals.List(ctx, sort, a.FavoritesOnly)
		if err != nil {
			return toolResult{}, err
		}
		top := ms
		if len(top) > 3 {
			top = top[:3]
		}
		var evs []*agentv1.AskEvent
		for _, m := range top {
			evs = append(evs, referenceEvent("meal", m.ID, m.Name, mealSubtitle(m)))
		}
		return toolResult{Events: evs, Summary: summarizeMeals(ms)}, nil

	case toolNavigate:
		var a navigateArgs
		_ = json.Unmarshal([]byte(argsJSON), &a)
		return toolResult{
			Events:  []*agentv1.AskEvent{navigateEvent(a.To, a.Search)},
			Summary: fmt.Sprintf(`{"navigated":%q}`, a.To),
		}, nil
	}
	return toolResult{}, fmt.Errorf("unknown tool %q", name)
}

func summarizeRecipes(rs []recipe.Recipe) string {
	type row struct {
		ID, Name, Cuisine string
		Minutes           int
		InPantry          bool
	}
	out := make([]row, 0, len(rs))
	for _, r := range rs {
		out = append(out, row{r.ID, r.Name, r.Cuisine, r.TotalMinutes, r.InPantry})
	}
	b, _ := json.Marshal(map[string]any{"count": len(rs), "recipes": out})
	return string(b)
}

func summarizeMeals(ms []meal.Meal) string {
	type row struct {
		ID, Name    string
		Rating      int
		TimesCooked int
	}
	out := make([]row, 0, len(ms))
	for _, m := range ms {
		out = append(out, row{m.ID, m.Name, m.Rating, m.TimesCooked})
	}
	b, _ := json.Marshal(map[string]any{"count": len(ms), "meals": out})
	return string(b)
}

// toolDefs returns the function-tool definitions advertised to the model.
func toolDefs() []responses.ToolUnionParam {
	strObj := func(props map[string]any, required ...string) map[string]any {
		m := map[string]any{"type": "object", "properties": props}
		if len(required) > 0 {
			m["required"] = required
		}
		return m
	}
	return []responses.ToolUnionParam{
		{OfFunction: &responses.FunctionToolParam{
			Name:        toolSearchRecipes,
			Description: openai.String("Search the user's recipes. Filter by ingredient (substring), in_pantry (cookable now), or max_minutes."),
			Parameters: strObj(map[string]any{
				"ingredient":  map[string]string{"type": "string"},
				"in_pantry":   map[string]string{"type": "boolean"},
				"max_minutes": map[string]string{"type": "integer"},
			}),
		}},
		{OfFunction: &responses.FunctionToolParam{
			Name:        toolSearchMeals,
			Description: openai.String("Search the user's cooked meals log. favorites_only keeps 4★+; sort is 'recent' or 'rating'."),
			Parameters: strObj(map[string]any{
				"favorites_only": map[string]string{"type": "boolean"},
				"sort":           map[string]string{"type": "string"},
			}),
		}},
		{OfFunction: &responses.FunctionToolParam{
			Name:        toolNavigate,
			Description: openai.String("Navigate the UI. to is a path like '/recipes' or '/meals'; search is string key/values like {\"pantry\":\"1\"} or {\"sort\":\"rating\",\"fav\":\"1\"}."),
			Parameters: strObj(map[string]any{
				"to":     map[string]string{"type": "string"},
				"search": map[string]any{"type": "object", "additionalProperties": map[string]string{"type": "string"}},
			}, "to"),
		}},
	}
}
```

NOTE: `recipeRefs`, `referenceEvent`, `navigateEvent` already exist in `script.go`. If the `responses`/`openai` field names differ in v3.41.0 (e.g. `FunctionToolParam.Parameters` type), the build step below will catch it — adjust to the installed API; keep the three tools and their JSON shapes identical.

- [ ] **Step 4: Run tests**

Run: `go build ./... && go test ./internal/agent/ -run 'TestExecute|TestSelect'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/tools.go internal/agent/tools_test.go
git commit -m "feat: agent tool layer (search_recipes/search_meals/navigate)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Agent loop (`llmClient` interface + `llmAgent.Run`), stub-tested

**Files:**
- Create: `internal/agent/llm.go`
- Test: `internal/agent/llm_test.go`

**Interfaces:**
- Consumes: `executeTool`, `textChunks`, `thinkingEvent`, `toolCallEvent`, `doneEvent` (from script.go/tools.go).
- Produces:
  - `type llmToolCall struct { CallID, Name, Arguments string }`
  - `type llmTurn struct { Text string; ToolCalls []llmToolCall; ResponseID string }`
  - `type llmClient interface { Respond(ctx context.Context, prev string, toolOutputs []llmToolOutput, userText string) (llmTurn, error) }`
    where `type llmToolOutput struct { CallID, Output string }`. On the first round `prev == ""` and `userText` is set; on follow-up rounds `prev` = previous `ResponseID`, `userText == ""`, and `toolOutputs` carries results.
  - `type llmAgent struct { recipes *recipe.Repo; meals *meal.Repo; client llmClient; maxRounds int }`
  - `func (a *llmAgent) Run(ctx context.Context, userText string, emit func(*agentv1.AskEvent) error) error`

- [ ] **Step 1: Write the failing test**

`internal/agent/llm_test.go`:
```go
package agent

import (
	"context"
	"testing"

	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
)

// stubClient returns a fixed script of turns, ignoring inputs.
type stubClient struct {
	turns []llmTurn
	i     int
}

func (s *stubClient) Respond(_ context.Context, _ string, _ []llmToolOutput, _ string) (llmTurn, error) {
	t := s.turns[s.i]
	s.i++
	return t, nil
}

func collect(t *testing.T, a *llmAgent, text string) []*agentv1.AskEvent {
	t.Helper()
	var got []*agentv1.AskEvent
	if err := a.Run(context.Background(), text, func(e *agentv1.AskEvent) error {
		got = append(got, e)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return got
}

func kinds(evs []*agentv1.AskEvent) []string {
	var k []string
	for _, e := range evs {
		switch e.Event.(type) {
		case *agentv1.AskEvent_Thinking:
			k = append(k, "thinking")
		case *agentv1.AskEvent_ToolCall:
			k = append(k, "tool_call")
		case *agentv1.AskEvent_Reference:
			k = append(k, "reference")
		case *agentv1.AskEvent_Navigate:
			k = append(k, "navigate")
		case *agentv1.AskEvent_Text:
			k = append(k, "text")
		case *agentv1.AskEvent_Done:
			k = append(k, "done")
		}
	}
	return k
}

func TestRunToolThenText(t *testing.T) {
	recipes, meals := seededRepos(t)
	client := &stubClient{turns: []llmTurn{
		{ToolCalls: []llmToolCall{{CallID: "c1", Name: toolSearchRecipes, Arguments: `{"ingredient":"chicken"}`}}, ResponseID: "r1"},
		{Text: "Here are chicken recipes."},
	}}
	a := &llmAgent{recipes: recipes, meals: meals, client: client, maxRounds: 5}
	got := kinds(collect(t, a, "chicken please"))
	// thinking + tool_call precede the references; text then done at the end.
	if got[0] != "thinking" || got[1] != "tool_call" {
		t.Fatalf("want thinking,tool_call first; got %v", got)
	}
	if got[len(got)-1] != "done" {
		t.Fatalf("want done last; got %v", got)
	}
	var sawRef, sawText bool
	for _, k := range got {
		sawRef = sawRef || k == "reference"
		sawText = sawText || k == "text"
	}
	if !sawRef || !sawText {
		t.Fatalf("want reference and text; got %v", got)
	}
}

func TestRunMaxRoundsCap(t *testing.T) {
	recipes, meals := seededRepos(t)
	// Always calls a tool -> would loop forever without the cap.
	always := llmTurn{ToolCalls: []llmToolCall{{CallID: "c", Name: toolSearchRecipes, Arguments: `{}`}}, ResponseID: "r"}
	client := &stubClient{turns: []llmTurn{always, always, always, always, always, always, always}}
	a := &llmAgent{recipes: recipes, meals: meals, client: client, maxRounds: 5}
	got := kinds(collect(t, a, "loop"))
	if got[len(got)-1] != "done" {
		t.Fatalf("want done last even at cap; got %v", got)
	}
	if client.i > 5 {
		t.Fatalf("client called %d times, want <= 5", client.i)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -run TestRun`
Expected: FAIL (`llmAgent`/`llmClient` undefined).

- [ ] **Step 3: Implement**

`internal/agent/llm.go`:
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

const systemPrompt = `You are Sous, a concise kitchen copilot for a home cook.
You have tools to search the user's recipes and cooked-meal log, and to navigate
the app's UI. Use the tools to look things up — never invent recipes or meals
that the tools did not return. After finding what the user asked for, call the
navigate tool to take them to the right view, then give a one or two sentence
spoken summary.

Navigation targets:
- /recipes with search {"ingredient":"<x>"} | {"pantry":"1"} | {"maxMinutes":"30"}
- /meals with search {"sort":"rating","fav":"1"} | {"sort":"recent"}

Keep replies short and warm.`

type llmToolCall struct {
	CallID    string
	Name      string
	Arguments string
}

type llmToolOutput struct {
	CallID string
	Output string
}

type llmTurn struct {
	Text       string
	ToolCalls  []llmToolCall
	ResponseID string
}

// llmClient is the narrow seam over the OpenAI Responses API. Slice 6b wraps the
// real implementation with Sigil; tests use a stub.
type llmClient interface {
	Respond(ctx context.Context, prev string, toolOutputs []llmToolOutput, userText string) (llmTurn, error)
}

type llmAgent struct {
	recipes   *recipe.Repo
	meals     *meal.Repo
	client    llmClient
	maxRounds int
}

func newLLMAgent(client llmClient, recipes *recipe.Repo, meals *meal.Repo) *llmAgent {
	return &llmAgent{recipes: recipes, meals: meals, client: client, maxRounds: 5}
}

// Run drives the tool-calling loop, emitting AskEvents as each round completes.
func (a *llmAgent) Run(ctx context.Context, userText string, emit func(*agentv1.AskEvent) error) error {
	prev := ""
	var outputs []llmToolOutput
	first := true

	for round := 0; round < a.maxRounds; round++ {
		text := ""
		if first {
			text = userText
		}
		turn, err := a.client.Respond(ctx, prev, outputs, text)
		if err != nil {
			return err
		}
		first = false
		prev = turn.ResponseID
		outputs = nil

		// No tool calls -> final answer.
		if len(turn.ToolCalls) == 0 {
			if err := emitText(turn.Text, emit); err != nil {
				return err
			}
			return emit(doneEvent())
		}

		// Execute each tool call, streaming its events and collecting outputs.
		for _, tc := range turn.ToolCalls {
			if err := emit(thinkingEvent(thinkingFor(tc))); err != nil {
				return err
			}
			if err := emit(toolCallEvent(tc.Name, detailFor(tc))); err != nil {
				return err
			}
			res, err := executeTool(ctx, a.recipes, a.meals, tc.Name, tc.Arguments)
			if err != nil {
				res = toolResult{Summary: fmt.Sprintf(`{"error":%q}`, err.Error())}
			}
			for _, ev := range res.Events {
				if err := emit(ev); err != nil {
					return err
				}
			}
			outputs = append(outputs, llmToolOutput{CallID: tc.CallID, Output: res.Summary})
		}
	}

	// Hit the round cap: ask once more for a final summary, best-effort.
	turn, err := a.client.Respond(ctx, prev, outputs, "")
	if err == nil {
		if err := emitText(turn.Text, emit); err != nil {
			return err
		}
	}
	return emit(doneEvent())
}

func emitText(s string, emit func(*agentv1.AskEvent) error) error {
	for _, ev := range textChunks(s) {
		if err := emit(ev); err != nil {
			return err
		}
	}
	return nil
}

func thinkingFor(tc llmToolCall) string {
	switch tc.Name {
	case toolSearchRecipes:
		return "Searching your recipes…"
	case toolSearchMeals:
		return "Checking your meals log…"
	case toolNavigate:
		return "Opening the right view…"
	}
	return "Working…"
}

func detailFor(tc llmToolCall) string {
	a := strings.TrimSpace(tc.Arguments)
	if a == "" || a == "{}" {
		return tc.Name
	}
	return a
}
```

NOTE: the cap path calls `Respond` a 6th time (the "one more for a summary"); the `TestRunMaxRoundsCap` stub supplies 7 turns so this is safe, and the assertion is `client.i <= 5` for the *loop* — adjust the assertion to `<= 6` if counting the final summary call. Implementer: make the test and code agree; the invariant that matters is the loop body runs at most `maxRounds` times and `done` is always emitted.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/agent/ -run TestRun`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/agent/llm.go internal/agent/llm_test.go
git commit -m "feat: agent tool-calling loop behind llmClient seam

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: Real OpenAI Responses adapter

**Files:**
- Create: `internal/agent/openai_client.go`

**Interfaces:**
- Consumes: `toolDefs()`, the `llmClient` interface + `llmTurn`/`llmToolCall`/`llmToolOutput` types from Task 4.
- Produces: `func newOpenAIClient(apiKey, model string) llmClient` returning an implementation backed by `openai.Client`.

- [ ] **Step 1: Implement the adapter**

`internal/agent/openai_client.go`:
```go
package agent

import (
	"context"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"
)

type openAIClient struct {
	client openai.Client
	model  string
}

func newOpenAIClient(apiKey, model string) llmClient {
	c := openai.NewClient(option.WithAPIKey(apiKey))
	return &openAIClient{client: c, model: model}
}

func (o *openAIClient) Respond(ctx context.Context, prev string, toolOutputs []llmToolOutput, userText string) (llmTurn, error) {
	params := responses.ResponseNewParams{
		Model: shared.ChatModel(o.model),
		Tools: toolDefs(),
	}
	// Reasoning effort low keeps nano fast. Remove if the field/name differs.
	params.Reasoning = shared.ReasoningParam{Effort: shared.ReasoningEffortLow}

	if prev != "" {
		params.PreviousResponseID = openai.String(prev)
	}

	var items []responses.ResponseInputItemUnionParam
	if userText != "" {
		// First turn: system instructions + the user's message.
		params.Instructions = openai.String(systemPrompt)
		items = append(items, responses.ResponseInputItemParamOfMessage(userText, responses.EasyInputMessageRoleUser))
	}
	for _, out := range toolOutputs {
		items = append(items, responses.ResponseInputItemUnionParam{
			OfFunctionCallOutput: &responses.ResponseInputItemFunctionCallOutputParam{
				CallID: out.CallID,
				Output: responses.ResponseInputItemFunctionCallOutputOutputUnionParam{
					OfString: openai.String(out.Output),
				},
			},
		})
	}
	if len(items) > 0 {
		params.Input = responses.ResponseNewParamsInputUnion{OfInputItemList: items}
	}

	resp, err := o.client.Responses.New(ctx, params)
	if err != nil {
		return llmTurn{}, err
	}

	turn := llmTurn{ResponseID: resp.ID}
	for _, item := range resp.Output {
		if item.Type == "function_call" {
			fc := item.AsFunctionCall()
			turn.ToolCalls = append(turn.ToolCalls, llmToolCall{
				CallID: fc.CallID, Name: fc.Name, Arguments: fc.Arguments,
			})
		}
	}
	turn.Text = resp.OutputText() // convenience accumulator for assistant text
	return turn, nil
}
```

NOTE (verify against v3.41.0 — the build step is the gate; keep behavior, adjust names):
- Constructing a user message input item: the helper may be `responses.ResponseInputItemParamOfMessage(...)` or you may build `responses.EasyInputMessageParam{Role: ..., Content: ...}` wrapped in the union. Use whatever the SDK exposes; the goal is one user message on the first turn.
- `resp.OutputText()` is the documented convenience for concatenated text; if absent, walk `resp.Output` for `output_text` content.
- `shared.ReasoningParam` / `ReasoningEffortLow` names: confirm; if the model rejects reasoning params, drop that line.
- `shared.ChatModel(o.model)` vs `responses.ResponsesModel(...)`: use whichever the `Model` field accepts.

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: compiles. Fix any API-name drift per the notes (do NOT change the `llmClient` contract or tool JSON shapes).

- [ ] **Step 3: Commit**

```bash
git add internal/agent/openai_client.go
git commit -m "feat: OpenAI Responses adapter implementing llmClient

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Service + server wiring (backend selection)

**Files:**
- Modify: `internal/agent/service.go`
- Modify: `cmd/server/main.go`

**Interfaces:**
- Consumes: `config.Config`, `newOpenAIClient`, `newLLMAgent`, existing scripted `respond`.
- Produces: `func NewService(cfg config.Config, recipes *recipe.Repo, meals *meal.Repo) agentv1connect.AgentServiceHandler`. Keep `NewServiceWithDelay(recipes, meals, d)` for the scripted-path tests.

- [ ] **Step 1: Update the service**

In `internal/agent/service.go`:
- Add fields to `Service`: `agent *llmAgent` (nil when scripted).
- Rewrite `NewService`:
```go
func NewService(cfg config.Config, recipes *recipe.Repo, meals *meal.Repo) agentv1connect.AgentServiceHandler {
	if cfg.HasOpenAI() {
		client := newOpenAIClient(cfg.OpenAIAPIKey, cfg.OpenAIModel)
		return &Service{recipes: recipes, meals: meals, agent: newLLMAgent(client, recipes, meals)}
	}
	return &Service{recipes: recipes, meals: meals, delay: 60 * time.Millisecond}
}
```
- In `Ask`, branch on `s.agent`:
```go
func (s *Service) Ask(ctx context.Context, req *connect.Request[agentv1.AskRequest], stream *connect.ServerStream[agentv1.AskEvent]) error {
	if s.agent != nil {
		emit := func(ev *agentv1.AskEvent) error { return stream.Send(ev) }
		err := s.agent.Run(ctx, req.Msg.GetText(), emit)
		if err != nil {
			// Best-effort graceful close: a short apology then done.
			_ = stream.Send(textEvent("Sorry — I hit a problem reaching the model. Try again?"))
			_ = stream.Send(doneEvent())
			return nil
		}
		return nil
	}
	// scripted path (unchanged)
	events, err := respond(ctx, s.recipes, s.meals, req.Msg.GetText())
	...
}
```
- Add `"github.com/sethlowie/dinnerwise/internal/config"` to imports.

- [ ] **Step 2: Update scripted tests' constructor calls**

Any test that calls `NewService(recipes, meals)` must switch to `NewServiceWithDelay(recipes, meals, 0)` (the scripted path) so it doesn't require config. Grep first:
```bash
grep -rn "NewService(" internal/agent/
```
Update call sites in `service_test.go`/`script_test.go` accordingly.

- [ ] **Step 3: Update main.go**

In `cmd/server/main.go`:
- At the top of `main`: `_ = godotenv.Load()` then `cfg := config.Load()`.
- Replace the agent handler line:
```go
mux.Handle(agentv1connect.NewAgentServiceHandler(
	agent.NewService(cfg, recipe.NewRepo(database), meal.NewRepo(database)),
))
```
- Log the backend: `if cfg.HasOpenAI() { log.Printf("server: agent backend = openai (%s)", cfg.OpenAIModel) } else { log.Print("server: agent backend = scripted (no OPENAI_API_KEY)") }`
- Add imports: `"github.com/joho/godotenv"`, `"github.com/sethlowie/dinnerwise/internal/config"`.

- [ ] **Step 4: Build and test**

Run: `go build ./... && go test ./...`
Expected: PASS (LLM path covered by stub tests; scripted path unchanged).

- [ ] **Step 5: Commit**

```bash
git add internal/agent/service.go cmd/server/main.go internal/agent/*_test.go
git commit -m "feat: select openai vs scripted agent backend by config

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: Opt-in live integration test

**Files:**
- Create: `internal/agent/live_test.go`

**Interfaces:**
- Consumes: `newOpenAIClient`, `newLLMAgent`, `seededRepos`.

- [ ] **Step 1: Write the gated test**

`internal/agent/live_test.go`:
```go
package agent

import (
	"context"
	"os"
	"testing"

	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
)

// TestLiveOpenAI runs one real turn. Skipped unless OPENAI_API_KEY is set and
// DINNERWISE_LIVE=1, so the default suite never calls the network.
func TestLiveOpenAI(t *testing.T) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" || os.Getenv("DINNERWISE_LIVE") != "1" {
		t.Skip("set OPENAI_API_KEY and DINNERWISE_LIVE=1 to run the live test")
	}
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-5-nano"
	}
	recipes, meals := seededRepos(t)
	a := newLLMAgent(newOpenAIClient(key, model), recipes, meals)

	var text, done bool
	err := a.Run(context.Background(), "What can I cook tonight?", func(e *agentv1.AskEvent) error {
		switch e.Event.(type) {
		case *agentv1.AskEvent_Text:
			text = true
		case *agentv1.AskEvent_Done:
			done = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !text || !done {
		t.Fatalf("expected text and done; text=%v done=%v", text, done)
	}
}
```

- [ ] **Step 2: Verify it skips by default**

Run: `go test ./internal/agent/ -run TestLiveOpenAI -v`
Expected: SKIP.

- [ ] **Step 3: (Manual, optional) run it live**

Run: `DINNERWISE_LIVE=1 go test ./internal/agent/ -run TestLiveOpenAI -v` (with `.env` loaded / key exported).
Expected: PASS, one real turn against gpt-5-nano.

- [ ] **Step 4: Commit**

```bash
git add internal/agent/live_test.go
git commit -m "test: opt-in live OpenAI agent integration test

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review

- **Spec coverage:** config (T1) · Responses API + model (T5) · LLM-driven navigate tool (T3 tool + T4 loop) · scripted fallback (T6) · live streaming of events with chunked final text (T4/T6, deviation noted) · godotenv (T1/T6) · bounded 5-round loop (T4) · DRY filter extraction (T2) · tool tests on seeded DB (T3) · stubbed loop tests (T4) · scripted tests stay green (T2/T6) · optional live test (T7) · contract unchanged (no proto task). Covered.
- **Deviation from spec:** spec said "real token stream"; plan uses non-streaming rounds with chunked final text for reliability (events still stream live as rounds complete). Flagged to the user at handoff.
- **Type consistency:** `llmClient.Respond` signature, `llmTurn`/`llmToolCall`/`llmToolOutput`, `toolResult`, tool-name constants, and `NewService(cfg, …)` are used consistently across T3–T7. The openai-go field names in T3/T5 are explicitly marked "verify against installed SDK; build is the gate."
- **Placeholder scan:** none — every step has real code or a real command. The "NOTE: verify" callouts are deliberate SDK-drift guards on the two OpenAI-touching tasks, with the build step as the check, not deferred work.
