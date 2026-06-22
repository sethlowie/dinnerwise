# Dinnerwise

An agentic "what's for dinner?" copilot. Ask in plain language — *"what can I
cook tonight?"*, *"something quick with chicken"*, *"what are my favorites?"* —
and Dinnerwise searches your recipes and cook history, drives the UI to the
right view, and explains what it found. The home screen is a single input that
**morphs into a docked chat panel** as the agent works.

## How it works

The agent is a real LLM (OpenAI, Responses API) with the app's own data exposed
as **tools**:

- `search_recipes(ingredient?, in_pantry?, max_minutes?)`
- `search_meals(favorites_only?, sort?)`
- `navigate(to, search)` — the model drives the UI itself (list views with
  filters, or a specific recipe/meal detail page)

Each turn streams typed events to the browser over a server-streaming RPC —
`thinking`, `tool_call`, `text` deltas, `reference` cards, `navigate`, `done` —
so you watch the agent reason, call tools, and move the app in real time. The
conversation is multi-turn: prior turns travel with each request, so follow-ups
like *"now just the ones with chicken"* resolve against context.

```
Browser (React + TanStack Router)
   │  Connect RPC (unary + server-streaming)
   ▼
Go server ── RecipeService / MealService ──▶ SQLite
   └──────── AgentService ──▶ OpenAI (Responses API; repos as tools)
                  │
                  └── OpenTelemetry + Grafana Sigil ──▶ otel-lgtm (traces, metrics, cost)
```

If no `OPENAI_API_KEY` is set, the agent falls back to a scripted backend over
the **same streaming contract**, so the app is fully runnable offline.

## Tech stack

- **Backend:** Go, [ConnectRPC](https://connectrpc.com) (Protobuf, server
  streaming), pure-Go SQLite (`modernc.org/sqlite`), hand-written SQL.
- **Agent:** `openai-go` v3 Responses API (`gpt-5` family), bounded tool-calling
  loop. Runs **statelessly** — the full conversation is resent each turn rather
  than chained server-side (required by a Zero-Data-Retention org, and a clean
  fit for horizontal scaling).
- **Frontend:** React 19 + Vite, TanStack Router (typed, route-aware so the
  agent can navigate), Tailwind v4 semantic-token theming (light/dark), the
  View Transitions API for the hero↔dock shell morph.
- **Observability:** OpenTelemetry GenAI semantic conventions + Grafana's
  [Sigil](https://grafana.com/docs/grafana-cloud/machine-learning/ai-observability/)
  AI-Observability SDK, exported to a local `grafana/otel-lgtm` stack. Captures
  per-turn token usage, latency, an approximate $ cost, and a full agent trace
  (`agent.ask` → `agent.tool` → `gen_ai` generation).
- **Codegen:** `buf` generates Go and TypeScript from the `.proto` contracts.

## Project layout

```
cmd/server/            # entrypoint: wiring, config, observability bootstrap
internal/
  agent/               # the LLM agent: tool loop, OpenAI adapter, cost, scripted fallback
  recipe/  meal/        # domain slices: schema.sql, repo (hand-written SQL), seed, service
  config/  db/  observability/
  */v1/                # protobuf-defined messages + Connect handlers
web/app/               # React + Vite client (TanStack Router, chat dock)
deploy/otel-lgtm/      # k8s manifests for the local Grafana observability stack
Tiltfile               # `tilt up` deploys otel-lgtm with port-forwards
```

Each domain slice owns its schema, repo, seed fixtures, and Connect service —
the proto messages are the contract between Go and the React client.

## Running locally

**Prerequisites:** Go 1.25+, Node + pnpm, and (optional) an OpenAI API key.

1. **Configure** — `cp .env.example .env` and set your key (omit it to run the
   scripted fallback agent, fully offline):
   ```sh
   OPENAI_API_KEY=sk-...        # omit for the scripted fallback
   OPENAI_MODEL=gpt-5-nano      # any model your account can access
   ```

2. **Backend** — `make run` (serves Connect on `:8080`; seeds a local SQLite DB
   on first run).

3. **Frontend** — `make web` (Vite dev server; open the printed URL).

4. **Ask** — from the home input try *"what can I cook tonight?"* and watch the
   input dock and the agent work.

Other targets: `make gen` (regenerate from protos), `make test` (Go tests),
`make build` (server binary), `make db-shell`.

### Observability (optional)

`make obs` deploys `grafana/otel-lgtm` via Tilt and port-forwards Grafana to
`localhost:3000` (admin/admin) and OTLP to `localhost:4318`. Add to `.env`:

```sh
OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
OTEL_SERVICE_NAME=dinnerwise
```

Run the server and chat a bit, then open Grafana → the **Dinnerwise — AI
Observability** dashboard for token usage, latency, cost, and traces.
Observability is additive: with no OTLP endpoint set, the server runs unchanged.

## Notable design decisions

- **Plumbing first, smarts behind a stable contract.** The streaming transport,
  tools, and UI were built against a scripted backend, then the real LLM dropped
  in behind the identical `AskEvent` contract — no client changes.
- **Stateless agent.** No server-side session or response chaining; conversation
  history rides in the request. Simple to scale, and the only option under a
  Zero-Data-Retention OpenAI org.
- **The model drives navigation.** Rather than rule-based routing, the agent
  calls a `navigate` tool, so behavior lives in one place and shows up in traces.
- **Route-driven UI + View Transitions.** The centered hero and the docked app
  are route states; entering/leaving the app is a real view-transition morph
  (the input grows into the chat panel), with within-app navigations kept clean.
