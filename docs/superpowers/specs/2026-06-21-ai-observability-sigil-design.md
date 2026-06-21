# Grafana AI Observability — Sigil + OTel (Slice 6b) — Design

**Date:** 2026-06-21
**Status:** Draft (design) — awaiting user review
**Arc:** LLM integration (6a real OpenAI agent ✓ · **6b Sigil/OTel AI Observability + local LGTM**)

## Goal

Instrument the gpt-5 agent (slice 6a) with **Grafana AI Observability (Sigil)**
and **OpenTelemetry**, exporting traces + metrics to a **local `grafana/otel-lgtm`**
stack. Ship a provisioned Grafana dashboard showing token usage, latency, call
rate, approximate **$ cost**, and a **full agent trace** per turn (Ask → tool
rounds → tool calls → reply). Local-first; Grafana Cloud is a later env flip.

## What we learned (feasibility)

- Sigil's OpenAI provider (`github.com/grafana/sigil-sdk/go-providers/openai`)
  has first-class **Responses API** support: `ResponsesNew(...)` /
  `ResponsesNewStreaming(...)` wrappers and `Responses*` **mapper** functions
  for manual recording. Our agent uses `Responses.New`, so it fits.
- The Sigil SDK emits **standard OTLP traces + metrics** using OTel **GenAI
  semantic conventions** (`gen_ai.client.token.usage`,
  `gen_ai.client.operation.duration`, etc.) → consumable by any OTel backend
  (local Tempo/Mimir).
- Sigil's proprietary **generation ingest** (the Cloud conversation-browser /
  evals UI) is **not self-hostable**; locally we run Sigil with generation
  export **`none`** (instrumentation-only) — which is also the right posture for
  this ZDR org. We still get all OTel metrics + spans.

## Decisions (locked)

- **Local-first, OTel-only export.** Traces + metrics → local `grafana/otel-lgtm`
  via OTLP. Sigil `GenerationExport.Protocol = none`. Cloud later = env flip
  (basic auth + Cloud OTLP gateway), no code change.
- **Full agent trace.** OTel parent span `agent.ask` per request; child span
  `agent.tool` per tool call (attrs: tool name); the Sigil-wrapped `Responses.New`
  contributes the `gen_ai` generation span + token/latency metrics under it.
- **Provisioned dashboard** shipped in the compose so Grafana shows data on
  first run.
- **Local $ cost** via a small per-model price table → a `gen_ai.client.cost.usd`
  metric recorded from each response's token usage. (Tokens themselves come from
  Sigil; we only add the dollar figure to avoid double-counting.)
- **Graceful degradation.** If `OTEL_EXPORTER_OTLP_ENDPOINT` is unset, skip OTel
  + Sigil setup (no-op tracer, no exporter errors) so the app still runs without
  the collector. Observability is additive, never required.
- **New deps:** `github.com/grafana/sigil-sdk/go`,
  `github.com/grafana/sigil-sdk/go-providers/openai`, OTel SDK
  (`go.opentelemetry.io/otel`, `.../sdk`, `.../contrib/exporters/autoexport`,
  OTLP exporters).

## Architecture

```
HTTP Ask ─▶ Service.Ask ──(span: agent.ask)──▶ llmAgent.Run
                                                  │
                              per round: Respond ─┼─(Sigil ResponsesNew)─▶ OpenAI
                                                  │     └─ gen_ai span + token/latency metrics + cost
                              per tool: ──────────┴─(span: agent.tool)──▶ executeTool

OTel TracerProvider/MeterProvider ──OTLP──▶ grafana/otel-lgtm (Tempo + Mimir)
                                                  └─ Grafana :3000 (provisioned dashboard)
```

### New package: `internal/observability`

```go
type Providers struct {
    Tracer trace.Tracer
    Sigil  *sigil.Client   // nil when disabled
    // cost instrument lives here or in agent; see Cost below
}

// Init sets up OTel TracerProvider + MeterProvider (OTLP via autoexport, reading
// OTEL_* env), a resource (service.name, service.version), the global providers,
// and a Sigil client with generation export = none. Returns a shutdown func that
// flushes/closes everything. If cfg.OTLPEndpoint == "" it returns a no-op
// Providers and a no-op shutdown.
func Init(ctx context.Context, cfg config.Config) (*Providers, func(context.Context) error, error)
```

- TracerProvider: `sdktrace.NewTracerProvider(WithBatcher(autoexport span exporter), WithResource(res))`.
- MeterProvider: `sdkmetric.NewMeterProvider(WithReader(autoexport metric reader), WithResource(res))`.
- Sigil: `cfg := sigil.DefaultConfig(); cfg.GenerationExport.Protocol = none; sigil.NewClient(cfg)`.
- Sets `otel.SetTracerProvider`/`SetMeterProvider` so the Sigil provider and our spans share them.

### `internal/config` additions

- `OTLPEndpoint string` (from `OTEL_EXPORTER_OTLP_ENDPOINT`), `ServiceName string`
  (default `dinnerwise`), `HasObservability() bool` (= OTLPEndpoint != "").
  (autoexport reads the OTLP env vars directly; we only need the gate + service name.)

### `internal/agent` changes

- **Generation capture:** `openAIClient` gains a `*sigil.Client`. `Respond` wraps
  the model call with the Sigil provider — preferred: the provider's
  `openai.ResponsesNew(ctx, sigilClient, "openai", params, opts...)`; if that
  wrapper's signature doesn't fit our injected client, fall back to
  `o.client.Responses.New` + a `Responses*` **mapper** to record the generation.
  Either path emits the `gen_ai` span + token/latency metrics. When the Sigil
  client is nil (observability disabled), call `Responses.New` directly.
- **Agent spans:** start `agent.ask` in `Service.Ask` (LLM path) around
  `llmAgent.Run`; in `Run`, wrap each `executeTool` call in an `agent.tool` child
  span (attribute `gen_ai.tool.name`). The context carrying the parent span flows
  into `Respond`, so the generation span nests correctly.
- **Cost:** a `cost.go` helper with a `map[string]modelPrice{in,out per 1K
  tokens}` (gpt-5.4 + a sane default). After each response, read token usage and
  record `gen_ai.client.cost.usd` (a Float64Counter) with a `gen_ai.request.model`
  attribute. Tokens are left to Sigil to avoid double counting.

### `cmd/server/main.go`

- After `config.Load()`: `providers, shutdown, err := observability.Init(ctx, cfg)`;
  `defer shutdown(context.Background())`. Pass `providers.Tracer` and
  `providers.Sigil` into `agent.NewService(cfg, providers, recipes, meals)`
  (extend the constructor). Log whether observability is on.

### Local stack + dashboard (`deploy/otel-lgtm/`)

- `docker-compose.yml` running `grafana/otel-lgtm` (Grafana :3000, OTLP gRPC
  :4317 / HTTP :4318), with a provisioned **dashboard JSON** mounted into the
  image's dashboard provisioning path.
- `make obs` (up) / `make obs-down`. Document the env to run the server against
  it: `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318`,
  `OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf`, `OTEL_SERVICE_NAME=dinnerwise`
  (add these to `.env`).
- Dashboard panels: tokens in/out (`gen_ai.client.token.usage`), operation
  latency (`gen_ai.client.operation.duration`), calls over time, **approx $/turn
  & cumulative $** (`gen_ai.client.cost.usd`), and a recent-traces/Tempo panel.

## Testing

- **`internal/observability`:** `Init` with `OTLPEndpoint==""` returns no-op
  Providers + callable no-op shutdown (no panics, no exporter). With an endpoint
  set, `Init` builds non-nil providers and `shutdown` is callable (don't assert
  on a live collector).
- **Cost map** (`internal/agent`): `costUSD(model, inTok, outTok)` returns the
  expected dollar figure for gpt-5.4 and falls back to the default price for an
  unknown model; zero tokens → 0.
- **Agent spans** (`internal/agent`): run `llmAgent.Run` with the stub client +
  an **in-memory span recorder** (`tracetest.NewSpanRecorder`/`InMemoryExporter`);
  assert a parent `agent.ask`-style span and one `agent.tool` child per tool call
  exist with the tool-name attribute. (Wire the tracer into the agent so tests
  inject a recording TracerProvider.)
- Existing agent/config tests stay green; the live test still works (now also
  emits telemetry when the endpoint is set — still skipped by default).
- `go build ./...`, `go vet ./...`, `go test ./...` green.
- **Manual:** `make obs`, run the server with the OTLP env + a real turn, open
  Grafana → dashboard shows tokens/latency/cost and the trace tree.

## Tradeoffs & notes

- Generation export `none` means no Cloud conversation-browser/evals UI locally;
  we get OTel metrics + traces + our dashboard. Cloud is an env flip later.
- Local $ cost is approximate (static price table), not real billing — clearly a
  demo figure. The price table is one obvious place to update per model.
- Sigil owns token metrics; we own the cost metric — no double counting.
- The exact Sigil/OTel/autoexport symbol names are pinned in the plan against the
  installed versions (build is the gate, as in 6a).

## Out of scope

- Grafana Cloud generation ingest / the hosted AI Observability app + online
  evals (env-flip follow-up). Tracing the React frontend. Per-user/session
  cost attribution. Alerting on cost/latency. Swapping models (env only).
