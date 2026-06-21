# Dinnerwise

An agent that answers the most repetitive question in any household — **what's for
dinner?** — and keeps re-planning in real time as reality interferes while you shop.

It's not a recipe browser. It knows who's eating (and their hard limits + tastes),
what's already in the pantry, and the week's budget. It plans a week of dinners,
generates the shopping list, and — the headline — **adapts the whole plan on the fly**
when the store is out of something, a price spikes, or there's a deal worth grabbing.

See [`DESIGN.md`](./DESIGN.md) for the full product + architecture rationale.

## What it does

1. **Plans the week** — a budget-aware weekly dinner plan that respects every member's
   hard limits (allergies, diets, vetoes), optimizes soft preferences, and prefers what's
   already in the pantry. It explains each pick and shows what it ruled out and *why*.
2. **Generates a pantry-aware shopping list** — needed minus on-hand, priced.
3. **Adapts in real time** (Shopping Mode) — flag an out-of-stock item, a price spike, or
   a deal, and the agent re-plans *holistically* across the week and budget, then explains
   the trade-off it made (with a confidence and a budget delta).

## Design in one breath

- **Deterministic safety, LLM judgment.** Hard constraints (allergies/vetoes) are enforced
  in code — the model never gets to violate them. The LLM does the *judgment*: which
  eligible meals to pick, how to adapt, and why. Each agent step validates the model's
  output and falls back to a deterministic choice if the model errs.
- **Tiered models.** A frontier model plans and adapts; a small "nano" model does narrow,
  schema-constrained ingredient enrichment (cached). Swappable via config.
- **Self-observable.** Every agent step, tool call, and model call is an OpenTelemetry
  span carrying model + per-tier token usage, exported to a local Grafana/Tempo stack.

## Architecture

```
React + Vite (Connect-Query)  ──Connect/HTTP──▶  Go server (ConnectRPC, h2c)
                                                   ├─ auth (hand-rolled sessions)
                                                   ├─ planner  ─┐
                                                   ├─ adapter   ├─ internal/ai ─▶ OpenAI-compatible
                                                   ├─ tools     ┘                 (Ollama / OpenAI)
                                                   ├─ MongoDB
                                                   └─ OpenTelemetry ─▶ LGTM (Grafana/Tempo)
```

- **Backend:** Go + [ConnectRPC](https://connectrpc.com), MongoDB, OpenTelemetry.
- **Frontend:** React + Vite + Tailwind, `@connectrpc/connect-query` + TanStack Query.
- **Contracts:** Protobuf via [buf](https://buf.build) → Go handlers + a typed TS client.
- **AI:** `internal/ai` wraps any OpenAI-compatible endpoint. Defaults to local Ollama
  (qwen3); point it at OpenAI by changing env.
- **Dev loop:** [Tilt](https://tilt.dev) deploys everything to a local Kubernetes cluster
  with fast live-reload.

## Running it

**Prerequisites:** a Kubernetes context (k3s, Docker Desktop, kind, …), `tilt`, `kubectl`,
Docker, Node 20+/`pnpm`, Go 1.25+, and an LLM endpoint (a reachable [Ollama](https://ollama.com)
or an OpenAI API key).

```bash
cp .env.example .env          # adjust models / endpoint if needed
tilt up                       # builds + deploys server, mongo, and the LGTM stack
```

Then:

- App: http://localhost:5173 — sign up, then **Plan my week**, then the **Shopping** tab.
- Grafana (agent traces): http://localhost:3000 — Explore → Tempo → `service.name = dinnerwise`.

### Model configuration

`internal/ai` talks to any OpenAI-compatible endpoint. Set in `.env`:

```bash
# Local Ollama (default)
AI_BASE_URL=http://ollama.ai:11434/v1
AI_API_KEY=ollama
AI_MODEL=qwen3:8b          # frontier: planning + adaptation
AI_NANO_MODEL=qwen3:1.7b   # nano: ingredient enrichment

# …or OpenAI
# AI_BASE_URL=https://api.openai.com/v1
# AI_API_KEY=sk-...
# AI_MODEL=gpt-5
# AI_NANO_MODEL=gpt-5-nano
```

A small local model gets the plumbing working but is weak at schema adherence and
reasoning; a frontier model (GPU-hosted or OpenAI) is the quality path. The architecture
makes the swap a config change, no code edits.

## Repo layout

```
cmd/dinnerwise/        server entrypoint, middleware, observability bootstrap
cmd/env-config/        renders .env into a k8s Secret for Tilt
internal/
  ai/                  provider-agnostic LLM client (tiers, JSON mode, token tracing)
  auth/                hand-rolled session auth + Mongo repos
  domain/              data model, repos, embedded fixtures, seeding
  tools/               deterministic tools (pantry diff, price, substitutes) + enrichment
  planner/             eligibility/accommodation analysis + LLM weekly selection
  adapter/             real-time holistic re-planning on shopping events
  observability/       OpenTelemetry tracing + logging
  <svc>/v1/            protobuf + generated Connect handlers
packages/api/          generated TypeScript Connect-Query client
web/app/               React + Vite frontend
local-k8s/             Tilt-deployed manifests (server, mongo, lgtm)
```

## Tests

```bash
go test -short ./...          # unit tests (constraint analysis, shopping math, …)
make gen                      # regenerate Go + TS from protos (needs buf)
```

The LLM path has an env-gated integration test:

```bash
kubectl -n ai port-forward svc/ollama 11434:11434 &
AI_INTEGRATION=1 AI_NANO_MODEL=qwen3:1.7b go test ./internal/ai/ -run Integration -v
```
