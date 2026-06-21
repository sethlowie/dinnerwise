# Agent Reference Cards + Scenarios + Replay (Slice 4 of 5) — Design

**Date:** 2026-06-21
**Status:** Approved (design)
**Arc:** Sous design (1 visual ✓ · 2 Meals ✓ · 3 recipe metadata + pantry ✓ · **4 agent reference cards + scenarios + replay** · 5 orb physics)

## Goal

Make the (still scripted, no-LLM) agent actually use the domain we've built:
emit **reference cards** (result recipes/meals in the dock that open detail
pages), handle the design's **scenarios** (favorites / tonight / quick) plus the
existing ingredient search, and add a **replay** action. Scenarios query the
real `recipe`/`meal` repos, prefiguring the tools a real LLM would later call.

## Decisions (locked)

- **Stay scripted.** No LLM. The agent's behavior is hand-written intent
  matching + canned step/reply templates, but results come from real repo
  queries. The LLM remains a separate future effort.
- **Scenarios:** favorites, tonight, quick, ingredient (+ a no-intent fallback).
- **Agent service depends on `recipe.Repo` + `meal.Repo`** so scenarios return
  real results (titles/meta), and the navigation/reference data stays correct.
- **quick threshold = 30 minutes** (the design said 20, but our seeded recipes
  are 25/30/40 min, so 20 returns nothing). Reply worded "30 minutes or less".

## Protocol (`internal/agent/v1/agent.proto`)

Add a reference event; everything else unchanged:
```proto
message Reference {
  string kind     = 1;   // "recipe" | "meal"
  string id       = 2;
  string title    = 3;
  string subtitle = 4;   // e.g. "Italian · 25 min" or "★★★★★ · cooked 7×"
}

message AskEvent {
  oneof event {
    TextDelta text      = 1;
    Thinking  thinking  = 2;
    ToolCall  tool_call = 3;
    Navigate  navigate  = 4;
    Done      done      = 5;
    Reference reference = 6;   // NEW — emitted (one per result) before Done
  }
}
```
Regenerate (`make gen`).

## Scenario engine (`internal/agent`)

The service gains repo deps:
```go
func NewService(recipes *recipe.Repo, meals *meal.Repo) agentv1connect.AgentServiceHandler
```
(No import cycle: agent imports recipe/meal; neither imports agent.)

Replace the pure `script(text)` with `respond(ctx, text) ([]*agentv1.AskEvent, error)`
that queries repos. `Ask` streams the returned events with the existing delay.

`intentFrom(text)` (keyword-scripted, lower-cased):
- `favorite` | `best` → **favorites**
- `tonight` | `cook` | `dinner` | `have` → **tonight**
- `quick` | `fast` → **quick**
- a known ingredient keyword (chicken, tomato, tofu, garlic, broccoli, rice,
  pasta) → **ingredient**
- else → **none**

Per scenario, `respond` emits in order: a few `thinking` step labels → one
`tool_call` (the "query") → a `text` reply in a few chunks → a `reference` per
result → `done`. (Thinking-before-tool keeps the dock's Working-step order
correct without changing the dock's step model.)

| Intent | Query | Navigate | References | Reply (gist) |
|---|---|---|---|---|
| favorites | `meals.List("rating", true)`, top 3 | `/meals` {sort:rating, fav:1} | those meals (kind=meal) | "You've rated N meals 4★+. Your top is …" |
| tonight | `recipes.List()` where `InPantry` | `/recipes` {pantry:1} | those recipes (kind=recipe) | "You can cook N recipes without a shop: …" |
| quick | `recipes.List()` where `TotalMinutes ≤ 30` | `/recipes` {maxMinutes:30} | those recipes | "N recipes in 30 minutes or less: …" |
| ingredient (X) | `recipes.List()` where any ingredient name contains X | `/recipes` {ingredient:X} | matching recipes | "Here are the recipes with X." |
| none | — | — (no navigate) | — | "I can help you find recipes and meals — try 'what are my favorites', 'what can I cook tonight', or an ingredient." |

Reference subtitles: recipes → `"{cuisine} · {totalMinutes} min"`; meals →
`"{rating}★ · cooked {timesCooked}×"`. Counts in reply text computed from the
query results.

## Recipe filters (`web/app/src/routes/recipes.tsx`)

Add to `validateSearch` (alongside `ingredient`):
- `pantry?: boolean` — when true, keep only `r.inPantry`.
- `maxMinutes?: number` — when set, keep only `r.totalMinutes <= maxMinutes`.

Client-side filtering composes all active params (ingredient ∧ pantry ∧
maxMinutes). The tonight/quick scenarios navigate here; favorites navigates to
`/meals` (sort/fav already supported).

## Frontend agent UX

- **`ChatProvider`** (`chatContext.ts` + `ChatProvider.tsx`): add
  `references: { kind: string; id: string; title: string; subtitle: string }[]`
  to `AssistantMessage`; handle the `reference` event case → append. Add
  `replay()` to the context = re-ask the last turn's `userText` (no-op if empty
  or streaming).
- **`ChatPanel` dock**: after a turn's reply, render its **reference cards** —
  a tinted thumb (`lib/thumb` by id), title, subtitle, and a `→`; clicking calls
  `router.navigate` to `/meals/$id` (kind=meal) or `/recipes/$id` (kind=recipe)
  via a `NavigateOptions` cast. Below the cards (or in the header), a
  **"↻ replay this run"** button calling `replay()`.

## Server wiring (`cmd/server/main.go`)

```go
mux.Handle(agentv1connect.NewAgentServiceHandler(
    agent.NewService(recipe.NewRepo(database), meal.NewRepo(database)),
))
```

## Testing

- **Go (`internal/agent`)**, against a seeded recipe+meal DB (reuse the seed
  helpers; meals FK recipes so seed recipes first):
  - favorites → a `Navigate` to `/meals` with `sort=rating`,`fav=1` and ≥1
    `Reference` with `kind=meal`.
  - tonight → `Navigate /recipes` `pantry=1` and recipe references that are all
    in-pantry (e.g. include `tomato-pasta`, exclude `veggie-stir-fry`).
  - quick → `Navigate /recipes` `maxMinutes=30` and references all ≤30 min.
  - ingredient ("chicken") → `Navigate /recipes` `ingredient=chicken` + refs.
  - none ("hello") → no `Navigate`, no `Reference`; ends `Done`.
  - event ordering: `thinking…` → `tool_call` → `text…` → `reference…` → `done`.
  - keep the httptest streaming integration test green.
- **Frontend:** `tsc -b`/`eslint`/`vite build` clean; runtime — ask each
  scenario; the dock streams steps + reply + reference cards; a card opens the
  right detail; replay re-runs; `/recipes?pantry=1` and `?maxMinutes=30` filter.

## Components / files

- Modify: `internal/agent/v1/agent.proto` (Reference), `internal/agent/script.go`
  → rename/replace with scenario logic (`respond`, `intentFrom`, scenario
  builders, reference/subtitle helpers), `internal/agent/service.go` (repo deps,
  `respond` streaming), `internal/agent/script_test.go` + `service_test.go`
  (scenario tests), `cmd/server/main.go` (wiring).
- Modify: `web/app/src/chat/chatContext.ts` (references + replay types),
  `web/app/src/chat/ChatProvider.tsx` (reference event + replay),
  `web/app/src/chat/ChatPanel.tsx` (reference cards + replay button),
  `web/app/src/routes/recipes.tsx` (pantry + maxMinutes filters).

## Tradeoffs & notes

- The scripted scenarios query repos directly — deliberate: it keeps results
  correct and mirrors the tool calls a real LLM agent will make later.
- quick = ≤30 min is a data-driven deviation from the design's 20.
- Reference navigation uses the same generic `NavigateOptions` cast as the
  existing `navigate` handler; typed/validated client actions remain future work.
- Intent matching is keyword-scripted and order-sensitive (first match wins);
  fine for the demo.

## Out of scope

The real LLM (separate effort); traveling-orb physics (slice 5); editing
the pantry/logging cooks from the UI; multi-turn agent memory.
