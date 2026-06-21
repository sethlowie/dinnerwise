# Real OpenAI Agent (Slice 6a) — Design

**Date:** 2026-06-21
**Status:** Draft (design) — awaiting user review
**Arc:** LLM integration (**6a real OpenAI agent** · 6b Sigil/OTel AI Observability + local LGTM)

## Goal

Replace the scripted `respond()` with a real **gpt-5-nano** agent that calls the
existing `recipe`/`meal` repos as **tools** and streams its work over the
**unchanged `AskEvent` contract** (text, thinking, tool_call, navigate,
reference, done). This is the long-deferred "stub smarts behind the same
contract" payoff: the scripted scenarios already query the real repos, so those
queries become the model's tools and the frontend needs no change.

Observability (Sigil SDK, OTel export, local LGTM, cost tracking) is **slice
6b** — but 6a leaves a clean seam for it (the OpenAI client is injected behind an
interface).

## Decisions (locked)

- **Model:** `gpt-5-nano`, via the official Go SDK `github.com/openai/openai-go`,
  using the **Responses API** (OpenAI's recommended path for reasoning +
  tool-calling). Reasoning effort set low/minimal for nano speed. Model name and
  key come from env (`OPENAI_MODEL` default `gpt-5-nano`, `OPENAI_API_KEY`).
- **LLM-driven navigation:** a `navigate` tool the model calls to drive the UI
  (vs deterministic post-processing). Navigation becomes a visible tool call.
- **Scripted fallback:** when `OPENAI_API_KEY` is unset, keep today's scripted
  `respond()` so the demo runs fully offline. `NewService` chooses the backend.
- **Live streaming:** `Service.Ask` streams events as the agent produces them
  (real token stream); the artificial per-event delay applies only to the
  scripted fallback path.
- **.env loading:** add `github.com/joho/godotenv`; the server auto-loads `.env`
  on startup (dev convenience; `.env` stays gitignored).
- **Bounded loop:** at most **5** tool-call rounds per turn (cost/runaway guard);
  on hitting the cap the agent finalizes with whatever text it has.
- **Contract unchanged:** no proto change, no frontend change. `agent.proto`
  stays as-is.

## Architecture

```
AskRequest ──> Service.Ask
                  │  (OPENAI_API_KEY set?)
        ┌─────────┴───────────┐
       yes                    no
        │                     │
   llmAgent.Run          respond() (scripted, unchanged)
        │                     │
        └──── stream.Send(AskEvent) ────> client
```

### New package: `internal/config`

`config.Load() (Config, error)` — reads env (after godotenv loads `.env` in
`main`). Fields for 6a:
- `OpenAIAPIKey string` (from `OPENAI_API_KEY`)
- `OpenAIModel  string` (from `OPENAI_MODEL`, default `gpt-5-nano`)

`Config.HasOpenAI() bool` → `OpenAIAPIKey != ""`. (6b extends this struct with
OTLP/Sigil fields; keep it small now.)

### New file: `internal/agent/llm.go`

The real agent. Depends on the repos and a narrow OpenAI seam:

```go
// llmClient is the seam 6b swaps for a Sigil-wrapped client. It is the subset
// of the OpenAI Responses streaming API the agent uses.
type llmClient interface {
    StreamResponse(ctx context.Context, req responseRequest) (responseStream, error)
}
```

`llmAgent` holds `recipes *recipe.Repo`, `meals *meal.Repo`, `client llmClient`,
`model string`, `maxRounds int`. `Run(ctx, userText, emit func(*agentv1.AskEvent) error) error`
runs the tool loop, calling `emit` for each event as it is produced.

The exact `responseRequest`/`responseStream` shapes and the openai-go calls are
pinned in the plan; the agent logic depends only on this interface so it is unit
testable with a stub and swappable in 6b.

### Tools exposed to the model

| Tool | Args | Backed by | Produces |
|---|---|---|---|
| `search_recipes` | `ingredient?`, `in_pantry?` (bool), `max_minutes?` (int) | `recipe.Repo.List` + filter | recipe `reference` events |
| `search_meals` | `favorites_only?` (bool), `sort?` ("recent"\|"rating") | `meal.Repo.List` | meal `reference` events |
| `navigate` | `to` (string), `search` (object of string→string) | — | a `navigate` event |

Tool result content returned to the model is a compact JSON summary (ids, names,
key meta) so it can speak about results and decide whether to navigate. The
filtering logic mirrors the scripted scenarios (pantry = all ingredients in
pantry; quick = `total_minutes <= max_minutes`; ingredient substring match) and
should be **extracted into shared helpers** in `internal/recipe`/`internal/meal`
or `internal/agent` so the scripted path and the tools agree (DRY — today that
logic lives inline in `script.go`).

### Event mapping (OpenAI stream → AskEvent)

- Model decides to call a tool → emit a `thinking` line (e.g. "Searching
  recipes…") then a `tool_call` event (`name`, `detail` = a human arg summary).
- Tool executes → for `search_*`, emit one `reference` per result (cap 3, as
  today, via shared helpers `recipeSubtitle`/`mealSubtitle`); for `navigate`,
  emit a `navigate` event.
- Assistant text deltas → `text` events (streamed).
- Turn ends (model stops, or `maxRounds` hit) → `done`.
- If the model emits reasoning-summary deltas, they may map to `thinking`;
  otherwise thinking lines are synthesized from tool intents (nano rarely emits
  rich summaries). Keep this best-effort — the dock's Working UI just needs
  ordered rows.

### System prompt

A concise kitchen-assistant prompt: you help find recipes and meals; use the
tools to look things up; after finding results, **navigate** the user to the
right view (`/recipes` with filters, or `/meals` with sort/fav) and give a short
spoken summary; never invent recipes/meals not returned by tools. The navigate
targets and search params match the routes the frontend already validates
(`/recipes` {ingredient,pantry,maxMinutes}, `/meals` {sort,fav}).

### `internal/agent/service.go`

- `NewService(cfg config.Config, recipes, meals)` returns the LLM-backed handler
  when `cfg.HasOpenAI()`, else the scripted handler (current behavior, keeping
  `NewServiceWithDelay` for tests).
- `Ask` for the LLM path: call `llmAgent.Run(ctx, text, emit)` where `emit`
  wraps `stream.Send`; no artificial delay. On a hard error after partial
  output, emit an apologetic `text` + `done` and return nil (don't tear the
  stream with a 500); on an error before any output, return
  `connect.NewError(connect.CodeInternal, err)`.

### `cmd/server/main.go`

- `godotenv.Load()` (ignore "file not found"), then `config.Load()`.
- Build the OpenAI client (plain official client in 6a; the Sigil wrapper lands
  in 6b) when `cfg.HasOpenAI()`; log which backend is active (LLM vs scripted).
- Wire `agent.NewService(cfg, recipe.NewRepo(db), meal.NewRepo(db))`.

## Testing

- **Tool impls** (`internal/agent`): against a seeded temp DB (reuse seed
  helpers; recipes before meals for FK) — `search_recipes` ingredient/pantry/
  max_minutes filters return the expected ids (pasta+chicken in pantry,
  stir-fry not; ≤30 set); `search_meals` favorites/sort. Deterministic, no
  network.
- **Agent loop** (`internal/agent`): a **stub `llmClient`** scripted to emit a
  fixed sequence (tool_call → tool result consumed → text → stop) drives
  `llmAgent.Run`; assert the emitted `AskEvent` sequence and ordering
  (thinking→tool_call→reference→text→done), navigate events, and the
  `maxRounds` cap (a stub that always calls a tool stops at 5 and still emits
  `done`). No OpenAI calls.
- **Scripted fallback** tests stay green (unchanged behavior when no key).
- **Optional live integration test**: gated behind `OPENAI_API_KEY` +
  `DINNERWISE_LIVE=1`, skipped by default, that runs one real turn end-to-end.
- `go test ./...`, `go build ./...` green; frontend untouched (no web tests
  needed — manual eyeball that the dock streams a real reply and navigates).

## Tradeoffs & notes

- **Responses API** chosen over Chat Completions per OpenAI's guidance for
  reasoning/tool models; the plan verifies the openai-go Responses streaming +
  function-tool + tool-output shapes against the installed version before
  writing loop code.
- The `llmClient` interface is deliberately narrow so 6b can wrap it with
  Sigil's OpenAI provider without touching agent logic — and so the loop is
  testable offline.
- DRY: scenario filter logic currently inlined in `script.go` is extracted to
  shared helpers consumed by both the scripted path and the tools, so they can't
  drift.
- Cost: nano + low reasoning effort + a 5-round cap keeps per-turn cost tiny;
  the live test is opt-in so CI never spends money.
- The model may navigate to a view with zero results (it's told not to invent
  data, but it can still choose to navigate); acceptable — the list views
  already render empty states.

## Out of scope (→ 6b or later)

- Sigil SDK, OTel `TracerProvider`/`MeterProvider`, OTLP export, the
  `grafana/otel-lgtm` container, dashboards, cost/token verification (**6b**).
- Multi-turn conversation memory (each Ask is one turn; the frontend already
  shows history). Streaming reasoning summaries as rich thinking. Editing
  pantry / logging cooks via tools. Swapping to larger models (env change only,
  once cost tracking is verified).
