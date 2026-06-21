# Grafana AI Observability — Sigil + OTel (Slice 6b) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Instrument the gpt-5 agent with Grafana AI Observability (Sigil) + OpenTelemetry, exporting traces + metrics to a local `grafana/otel-lgtm` stack, with a provisioned dashboard showing tokens, latency, approximate $ cost, and a full per-turn agent trace.

**Architecture:** An `internal/observability` package bootstraps OTel TracerProvider/MeterProvider (OTLP via autoexport) plus a Sigil client (generation export = none), returning a shutdown func — or a no-op when no OTLP endpoint is configured. The agent's OpenAI call is wrapped by Sigil's `ResponsesNew` to capture generations (tokens/latency/gen_ai metrics); a parent `agent.ask` span and per-tool `agent.tool` spans give the full trace; a small price-map records a `gen_ai.client.cost.usd` metric. A docker-compose runs otel-lgtm with a provisioned dashboard.

**Tech Stack:** Go 1.25, `github.com/grafana/sigil-sdk/go` + `/go-providers/openai`, OpenTelemetry Go SDK + `contrib/exporters/autoexport`, `grafana/otel-lgtm`.

## Global Constraints

- Sigil import paths: core `github.com/grafana/sigil-sdk/go/sigil`; OpenAI provider `github.com/grafana/sigil-sdk/go-providers/openai`. OpenAI SDK stays `github.com/openai/openai-go/v3`.
- Generation export is **none** (instrumentation-only); never point the generation ingest at Cloud in this slice. OTLP traces/metrics go to the local stack via the standard `OTEL_EXPORTER_OTLP_*` env.
- Observability is **additive**: if `OTEL_EXPORTER_OTLP_ENDPOINT` is unset, `observability.Init` returns a no-op (nil Sigil, no-op tracer) and the server runs exactly as in 6a. Never make a turn fail because telemetry failed.
- Service resource name: `dinnerwise` (from `OPENAI`/`OTEL_SERVICE_NAME` → config `ServiceName`, default `dinnerwise`).
- Tokens are owned by Sigil's `gen_ai.client.token.usage`; we ONLY add `gen_ai.client.cost.usd`. Do not emit our own token metric (no double count).
- The `AskEvent` contract and the scripted fallback path are unchanged.
- Don't log/commit secrets; `.env` stays gitignored.
- Every task ends green: `go build ./...`, `go vet ./...`, `go test ./...`.
- Commit messages end with: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`
- SDK symbol names from web docs (e.g. `GenerationExportProtocolNone`, `ResponsesNew` arg order, `autoexport` funcs) MUST be verified against the installed versions; `go build`/`go doc` is the gate. Keep the documented behavior; adapt names.

---

## File Structure

- `internal/config/config.go` (modify) — add `OTLPEndpoint`, `ServiceName`, `HasObservability()`.
- `internal/observability/observability.go` (new) — `Providers`, `Init`, shutdown.
- `internal/observability/observability_test.go` (new) — no-op + enabled paths.
- `internal/agent/cost.go` (new) — price map, `costUSD`, `recordCost`.
- `internal/agent/cost_test.go` (new).
- `internal/agent/openai_client.go` (modify) — Sigil-wrapped Responses + cost.
- `internal/agent/llm.go` (modify) — tracer + `agent.tool` spans.
- `internal/agent/service.go` (modify) — `NewService(cfg, providers, recipes, meals)`; `agent.ask` span.
- `internal/agent/llm_test.go` (modify) — in-memory span assertions.
- `cmd/server/main.go` (modify) — `observability.Init`, pass providers, shutdown.
- `deploy/otel-lgtm/docker-compose.yml`, `dashboard.json`, `provisioning.yaml` (new).
- `Makefile` (modify) — `obs` / `obs-down` targets.
- `.env` (modify, gitignored) — OTLP env (done by the controller, documented here).
- `go.mod` / `go.sum` (modify).

---

## Task 1: Config additions + dependencies

**Files:** Modify `internal/config/config.go`, `internal/config/config_test.go`, `go.mod`, `go.sum`

**Interfaces:**
- Produces: `Config.OTLPEndpoint string` (from `OTEL_EXPORTER_OTLP_ENDPOINT`), `Config.ServiceName string` (from `OTEL_SERVICE_NAME`, default `dinnerwise`), `(Config).HasObservability() bool` (= `OTLPEndpoint != ""`).

- [ ] **Step 1: Add dependencies**

Run:
```bash
go get github.com/grafana/sigil-sdk/go/sigil@latest
go get github.com/grafana/sigil-sdk/go-providers/openai@latest
go get go.opentelemetry.io/otel@latest
go get go.opentelemetry.io/otel/sdk@latest
go get go.opentelemetry.io/contrib/exporters/autoexport@latest
```
Expected: modules resolve into go.mod/go.sum. (autoexport pulls the OTLP trace/metric exporters transitively.) If a `go/sigil` subpath isn't a separate module, `go get github.com/grafana/sigil-sdk/go@latest` instead — verify the importable path with `go list -m all | grep sigil`.

- [ ] **Step 2: Write the failing test**

Add to `internal/config/config_test.go`:
```go
func TestObservability(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_SERVICE_NAME", "")
	c := Load()
	if c.HasObservability() {
		t.Fatal("HasObservability should be false with no endpoint")
	}
	if c.ServiceName != "dinnerwise" {
		t.Fatalf("default ServiceName = %q, want dinnerwise", c.ServiceName)
	}

	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4318")
	t.Setenv("OTEL_SERVICE_NAME", "custom")
	c = Load()
	if !c.HasObservability() || c.OTLPEndpoint != "http://localhost:4318" {
		t.Fatalf("HasObservability/endpoint wrong: %+v", c)
	}
	if c.ServiceName != "custom" {
		t.Fatalf("ServiceName = %q, want custom", c.ServiceName)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestObservability`
Expected: FAIL (fields/methods undefined).

- [ ] **Step 4: Implement**

In `internal/config/config.go`, extend `Config` and `Load`:
```go
type Config struct {
	OpenAIAPIKey string
	OpenAIModel  string
	OTLPEndpoint string
	ServiceName  string
}

func Load() Config {
	model := os.Getenv("OPENAI_MODEL")
	if model == "" {
		model = "gpt-5-nano"
	}
	service := os.Getenv("OTEL_SERVICE_NAME")
	if service == "" {
		service = "dinnerwise"
	}
	return Config{
		OpenAIAPIKey: os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:  model,
		OTLPEndpoint: os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
		ServiceName:  service,
	}
}

// HasObservability reports whether OTel export is configured.
func (c Config) HasObservability() bool { return c.OTLPEndpoint != "" }
```

- [ ] **Step 5: Run tests** — `go test ./internal/config/` → PASS.

- [ ] **Step 6: Commit**
```bash
git add go.mod go.sum internal/config/
git commit -m "feat: observability config + sigil/otel deps

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: observability package (OTel + Sigil bootstrap)

**Files:** Create `internal/observability/observability.go`, `internal/observability/observability_test.go`

**Interfaces:**
- Produces:
  - `type Providers struct { Tracer trace.Tracer; Sigil *sigil.Client }`
  - `func Init(ctx context.Context, cfg config.Config) (*Providers, func(context.Context) error, error)` — when `!cfg.HasObservability()`, returns a `Providers{Tracer: noop tracer, Sigil: nil}` and a no-op shutdown. When enabled, builds OTel TracerProvider + MeterProvider (OTLP via autoexport), sets them global, builds a Sigil client with generation export = none, and returns a shutdown that flushes/closes all.

- [ ] **Step 1: Write the failing test**

`internal/observability/observability_test.go`:
```go
package observability

import (
	"context"
	"testing"

	"github.com/sethlowie/dinnerwise/internal/config"
)

func TestInitDisabledIsNoop(t *testing.T) {
	p, shutdown, err := Init(context.Background(), config.Config{}) // no endpoint
	if err != nil {
		t.Fatal(err)
	}
	if p == nil || p.Tracer == nil {
		t.Fatal("expected non-nil Providers with a (no-op) Tracer")
	}
	if p.Sigil != nil {
		t.Fatal("expected nil Sigil when disabled")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("no-op shutdown should not error: %v", err)
	}
	// Tracer must be usable (no-op) without panicking.
	_, span := p.Tracer.Start(context.Background(), "x")
	span.End()
}
```

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/observability/` → FAIL (package missing).

- [ ] **Step 3: Implement**

`internal/observability/observability.go`:
```go
// Package observability wires OpenTelemetry (traces + metrics over OTLP) and a
// Grafana Sigil client for AI Observability. It is additive: with no OTLP
// endpoint configured, Init returns a no-op so the app runs unchanged.
package observability

import (
	"context"

	"github.com/grafana/sigil-sdk/go/sigil"
	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/sethlowie/dinnerwise/internal/config"
)

const instrumentationScope = "github.com/sethlowie/dinnerwise/internal/agent"

type Providers struct {
	Tracer trace.Tracer
	Sigil  *sigil.Client
}

func Init(ctx context.Context, cfg config.Config) (*Providers, func(context.Context) error, error) {
	if !cfg.HasObservability() {
		return &Providers{Tracer: noop.NewTracerProvider().Tracer(instrumentationScope)}, func(context.Context) error { return nil }, nil
	}

	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceName(cfg.ServiceName)))
	if err != nil {
		return nil, nil, err
	}

	spanExp, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return nil, nil, err
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(spanExp), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)

	metricReader, err := autoexport.NewMetricReader(ctx)
	if err != nil {
		return nil, nil, err
	}
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(metricReader), sdkmetric.WithResource(res))
	otel.SetMeterProvider(mp)

	// Sigil: instrumentation-only. Generation export off (no proprietary ingest).
	scfg := sigil.DefaultConfig()
	scfg.GenerationExport.Protocol = sigil.GenerationExportProtocolNone
	sclient := sigil.NewClient(scfg)

	shutdown := func(ctx context.Context) error {
		_ = sclient.Shutdown(ctx)
		_ = mp.Shutdown(ctx)
		return tp.Shutdown(ctx)
	}
	return &Providers{Tracer: tp.Tracer(instrumentationScope), Sigil: sclient}, shutdown, nil
}
```

VERIFY against installed SDKs (build is the gate; keep behavior, adapt names):
- `sigil.GenerationExportProtocolNone` — confirm the "none" constant name (`go doc github.com/grafana/sigil-sdk/go/sigil | grep -i protocol`). If `DefaultConfig` already defaults to none, you may drop the assignment.
- `sigil.NewClient` may return `(*Client, error)` or `*Client` — adapt.
- `noop.NewTracerProvider` import path is `go.opentelemetry.io/otel/trace/noop`.
- `autoexport.NewMetricReader` returns a `Reader`; confirm exact type used by `sdkmetric.WithReader`.

- [ ] **Step 4: Run tests** — `go build ./... && go test ./internal/observability/` → PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/observability/
git commit -m "feat: observability bootstrap (OTel + Sigil, no-op when disabled)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Cost metric (price map)

**Files:** Create `internal/agent/cost.go`, `internal/agent/cost_test.go`

**Interfaces:**
- Produces:
  - `func costUSD(model string, inTokens, outTokens int64) float64` — pure; uses a per-model price table (USD per 1M tokens) with a default fallback.
  - `func recordCost(ctx context.Context, model string, inTokens, outTokens int64)` — records `gen_ai.client.cost.usd` (Float64Counter) on the global meter with attribute `gen_ai.request.model`. Safe when no real meter is configured (global no-op).

- [ ] **Step 1: Write the failing test**

`internal/agent/cost_test.go`:
```go
package agent

import (
	"math"
	"testing"
)

func TestCostUSDKnownModel(t *testing.T) {
	// gpt-5.4 priced in the table; 1M in + 1M out should equal in+out price.
	got := costUSD("gpt-5.4", 1_000_000, 1_000_000)
	want := priceTable["gpt-5.4"].inPerM + priceTable["gpt-5.4"].outPerM
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("costUSD = %v, want %v", got, want)
	}
}

func TestCostUSDUnknownModelUsesDefault(t *testing.T) {
	got := costUSD("totally-unknown", 1_000_000, 0)
	if math.Abs(got-defaultPrice.inPerM) > 1e-9 {
		t.Fatalf("costUSD = %v, want default %v", got, defaultPrice.inPerM)
	}
}

func TestCostUSDZero(t *testing.T) {
	if got := costUSD("gpt-5.4", 0, 0); got != 0 {
		t.Fatalf("costUSD zero tokens = %v, want 0", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/agent/ -run TestCost` → FAIL.

- [ ] **Step 3: Implement**

`internal/agent/cost.go`:
```go
package agent

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// modelPrice is USD per 1,000,000 tokens. Approximate, for a local demo cost
// figure — not real billing. Update as pricing/models change.
type modelPrice struct {
	inPerM  float64
	outPerM float64
}

var priceTable = map[string]modelPrice{
	"gpt-5.4":     {inPerM: 1.25, outPerM: 10.0},
	"gpt-5-nano":  {inPerM: 0.05, outPerM: 0.40},
}

// defaultPrice is used for models not in the table.
var defaultPrice = modelPrice{inPerM: 1.0, outPerM: 3.0}

func priceFor(model string) modelPrice {
	if p, ok := priceTable[model]; ok {
		return p
	}
	return defaultPrice
}

// costUSD computes an approximate dollar cost for a generation.
func costUSD(model string, inTokens, outTokens int64) float64 {
	p := priceFor(model)
	return float64(inTokens)/1e6*p.inPerM + float64(outTokens)/1e6*p.outPerM
}

var costCounter metric.Float64Counter

func costInstrument() metric.Float64Counter {
	if costCounter != nil {
		return costCounter
	}
	m := otel.GetMeterProvider().Meter("github.com/sethlowie/dinnerwise/internal/agent")
	c, err := m.Float64Counter("gen_ai.client.cost.usd",
		metric.WithDescription("Approximate USD cost of generations"),
		metric.WithUnit("{USD}"))
	if err != nil {
		return nil
	}
	costCounter = c
	return c
}

// recordCost adds an approximate dollar cost for one generation.
func recordCost(ctx context.Context, model string, inTokens, outTokens int64) {
	c := costInstrument()
	if c == nil {
		return
	}
	c.Add(ctx, costUSD(model, inTokens, outTokens),
		metric.WithAttributes(attribute.String("gen_ai.request.model", model)))
}
```

NOTE: caching `costCounter` in a package var is fine here (the meter provider is set once at startup before requests). If a reviewer prefers, build the instrument once in `observability.Init` and pass it down — but the global-meter approach keeps cost.go self-contained and testable.

- [ ] **Step 4: Run tests** — `go test ./internal/agent/ -run TestCost` → PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/agent/cost.go internal/agent/cost_test.go
git commit -m "feat: approximate per-model cost metric (gen_ai.client.cost.usd)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Sigil generation capture in the adapter + service/main wiring

**Files:** Modify `internal/agent/openai_client.go`, `internal/agent/service.go`, `cmd/server/main.go`

**Interfaces:**
- Consumes: `observability.Providers`, `sigil.Client`, the provider package `sigilopenai "github.com/grafana/sigil-sdk/go-providers/openai"`, `recordCost`.
- Produces: `newOpenAIClient(apiKey, model string, sclient *sigil.Client) llmClient`; `NewService(cfg config.Config, providers *observability.Providers, recipes *recipe.Repo, meals *meal.Repo) agentv1connect.AgentServiceHandler`.

- [ ] **Step 1: Wrap the OpenAI call with Sigil + record cost**

In `internal/agent/openai_client.go`:
- Add field `sigil *sigil.Client` to `openAIClient`; update `newOpenAIClient(apiKey, model string, sclient *sigil.Client)` to store it.
- In `Respond`, replace the call site:
```go
var resp *responses.Response
if o.sigil != nil {
	resp, err = sigilopenai.ResponsesNew(ctx, o.sigil, o.client, params)
} else {
	resp, err = o.client.Responses.New(ctx, params)
}
if err != nil {
	return llmTurn{}, err
}
// Approximate $ cost from usage (tokens themselves are recorded by Sigil).
recordCost(ctx, o.model, int64(resp.Usage.InputTokens), int64(resp.Usage.OutputTokens))
```
- Imports: add `sigilopenai "github.com/grafana/sigil-sdk/go-providers/openai"` and `"github.com/grafana/sigil-sdk/go/sigil"`.

VERIFY (build is the gate): exact `sigilopenai.ResponsesNew` signature/return — the docs show `ResponsesNew(ctx, sigilClient, providerClient, req, opts...)` mirroring `ChatCompletionsNew`. If it returns a wrapper rather than `*responses.Response`, adapt the mapping. If the wrapper doesn't accept our injected `o.client`, fall back to calling `o.client.Responses.New` then `sigilopenai.ResponsesFromRequestResponse(params, resp)` to record. Confirm `resp.Usage.InputTokens`/`OutputTokens` field names via `go doc github.com/openai/openai-go/v3/responses.ResponseUsage`.

- [ ] **Step 2: Thread providers through the service**

In `internal/agent/service.go`:
- Update `NewService`:
```go
func NewService(cfg config.Config, providers *observability.Providers, recipes *recipe.Repo, meals *meal.Repo) agentv1connect.AgentServiceHandler {
	if cfg.HasOpenAI() {
		var sclient *sigil.Client
		if providers != nil {
			sclient = providers.Sigil
		}
		client := newOpenAIClient(cfg.OpenAIAPIKey, cfg.OpenAIModel, sclient)
		return &Service{recipes: recipes, meals: meals, agent: newLLMAgent(client, recipes, meals)}
	}
	return &Service{recipes: recipes, meals: meals, delay: 60 * time.Millisecond}
}
```
- Add imports `"github.com/sethlowie/dinnerwise/internal/observability"` and `"github.com/grafana/sigil-sdk/go/sigil"`.
- (The `agent.ask` span is added in Task 5; leave `Ask` otherwise unchanged here.)

- [ ] **Step 3: Wire main.go**

In `cmd/server/main.go`, after `cfg := config.Load()`:
```go
providers, shutdownObs, err := observability.Init(context.Background(), cfg)
if err != nil {
	log.Fatalf("server: observability init: %v", err)
}
defer func() { _ = shutdownObs(context.Background()) }()
if cfg.HasObservability() {
	log.Printf("server: observability on, exporting to %s", cfg.OTLPEndpoint)
} else {
	log.Print("server: observability off (no OTEL_EXPORTER_OTLP_ENDPOINT)")
}
```
and update the agent handler:
```go
agent.NewService(cfg, providers, recipe.NewRepo(database), meal.NewRepo(database)),
```
Add import `"github.com/sethlowie/dinnerwise/internal/observability"`.

- [ ] **Step 4: Build, vet, test**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: PASS. Existing agent tests construct the service via `NewServiceWithDelay` (scripted) so they're unaffected; if any call site of `NewService` exists in tests, update it to pass `nil` providers.

- [ ] **Step 5: Live smoke (manual, opt-in)**

With `.env` (key + `OPENAI_MODEL=gpt-5.4`) and an OTLP endpoint exported:
```bash
set -a; . ./.env; set +a
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf DINNERWISE_LIVE=1 \
  go test ./internal/agent/ -run TestLiveOpenAI -v
```
Expected: PASS (only if otel-lgtm is up from Task 6; otherwise run after Task 6). This is a manual check, not a gated CI step.

- [ ] **Step 6: Commit**
```bash
git add internal/agent/openai_client.go internal/agent/service.go cmd/server/main.go
git commit -m "feat: capture generations via Sigil + record approximate cost

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: Full agent trace (agent.ask + agent.tool spans)

**Files:** Modify `internal/agent/llm.go`, `internal/agent/service.go`, `internal/agent/llm_test.go`

**Interfaces:**
- Consumes: `trace.Tracer` (from `providers.Tracer`).
- Produces: `newLLMAgent(client llmClient, recipes *recipe.Repo, meals *meal.Repo, tracer trace.Tracer) *llmAgent` (tracer may be nil → use a no-op). `agent.ask` span started in `Service.Ask`; `agent.tool` child span per tool call in `Run`.

- [ ] **Step 1: Write the failing test**

In `internal/agent/llm_test.go`, add a span-recording test:
```go
func TestRunEmitsSpans(t *testing.T) {
	recipes, meals := seededRepos(t)
	client := &stubClient{turns: []llmTurn{
		{ToolCalls: []llmToolCall{{CallID: "c1", Name: toolSearchRecipes, Arguments: `{"ingredient":"chicken"}`}}},
		{Text: "done"},
	}}
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	a := newLLMAgent(client, recipes, meals, tp.Tracer("test"))

	ctx, parent := tp.Tracer("test").Start(context.Background(), "agent.ask")
	if err := a.Run(ctx, "chicken", func(*agentv1.AskEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	parent.End()

	var sawTool bool
	for _, s := range sr.Ended() {
		if s.Name() == "agent.tool" {
			sawTool = true
		}
	}
	if !sawTool {
		t.Fatal("expected an agent.tool span")
	}
}
```
Imports to add: `sdktrace "go.opentelemetry.io/otel/sdk/trace"`, `"go.opentelemetry.io/otel/sdk/trace/tracetest"`.

- [ ] **Step 2: Run test to verify it fails** — `go test ./internal/agent/ -run TestRunEmitsSpans` → FAIL (newLLMAgent arity / no span).

- [ ] **Step 3: Implement**

In `internal/agent/llm.go`:
- Add `tracer trace.Tracer` field to `llmAgent`; update `newLLMAgent` to accept it, defaulting nil → `noop.NewTracerProvider().Tracer("agent")`.
- In `Run`, wrap each tool execution in a child span:
```go
res, err := func() (toolResult, error) {
	ctx, span := a.tracer.Start(ctx, "agent.tool",
		trace.WithAttributes(attribute.String("gen_ai.tool.name", tc.Name)))
	defer span.End()
	return executeTool(ctx, a.recipes, a.meals, tc.Name, tc.Arguments)
}()
```
- Imports: `"go.opentelemetry.io/otel/attribute"`, `"go.opentelemetry.io/otel/trace"`, `"go.opentelemetry.io/otel/trace/noop"`.

In `internal/agent/service.go`:
- Pass `providers.Tracer` into `newLLMAgent` (guard nil providers → nil tracer).
- In `Ask` (LLM path), start the parent span:
```go
ctx, span := s.tracer.Start(ctx, "agent.ask")
defer span.End()
```
Store the tracer on `Service` (set in `NewService` from `providers.Tracer`, default no-op).

- [ ] **Step 4: Run tests** — `go test ./internal/agent/` → PASS (new span test + all prior).

- [ ] **Step 5: Commit**
```bash
git add internal/agent/llm.go internal/agent/service.go internal/agent/llm_test.go
git commit -m "feat: full agent trace (agent.ask parent + agent.tool spans)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Local otel-lgtm stack + provisioned dashboard

**Files:** Create `deploy/otel-lgtm/docker-compose.yml`, `deploy/otel-lgtm/dashboard.json`, `deploy/otel-lgtm/provisioning.yaml`; modify `Makefile`.

- [ ] **Step 1: docker-compose**

`deploy/otel-lgtm/docker-compose.yml`:
```yaml
services:
  lgtm:
    image: grafana/otel-lgtm:latest
    container_name: dinnerwise-lgtm
    ports:
      - "3000:3000"   # Grafana
      - "4317:4317"   # OTLP gRPC
      - "4318:4318"   # OTLP HTTP
    volumes:
      - ./dashboard.json:/otel-lgtm/grafana/conf/provisioning/dashboards/custom/dinnerwise.json:ro
      - ./provisioning.yaml:/otel-lgtm/grafana/conf/provisioning/dashboards/custom.yaml:ro
```

- [ ] **Step 2: provisioning + dashboard**

`deploy/otel-lgtm/provisioning.yaml`:
```yaml
apiVersion: 1
providers:
  - name: "dinnerwise"
    type: file
    options:
      path: /otel-lgtm/grafana/conf/provisioning/dashboards/custom
      foldersFromFilesStructure: false
```

`deploy/otel-lgtm/dashboard.json`: a minimal Grafana dashboard (schemaVersion ~39) with panels querying the otel-lgtm datasources (Prometheus/Mimir for metrics, Tempo for traces):
- Stat/timeseries: `sum by (gen_ai_response_model) (rate(gen_ai_client_token_usage_total[5m]))` (tokens) — VERIFY exact metric name as exported (OTLP→Prometheus mangles `gen_ai.client.token.usage` to `gen_ai_client_token_usage` with unit suffix; check in Grafana Explore after first run and fix the queries).
- Timeseries: operation latency from `gen_ai_client_operation_duration_*`.
- Stat: cumulative cost `sum(gen_ai_client_cost_usd_total)`.
- A Tempo "recent traces" panel filtered to service `dinnerwise`.

Because exact exported metric names depend on the collector's OTLP→Prometheus naming, the implementer should bring the stack up (Task 6 step 4), generate one live turn, confirm the metric names in Grafana Explore, and bake the verified names into `dashboard.json`. Ship a working dashboard, not a guessed one.

- [ ] **Step 3: Makefile targets**

Add to `Makefile`:
```make
.PHONY: obs obs-down
obs: ## Start the local Grafana otel-lgtm stack
	docker compose -f deploy/otel-lgtm/docker-compose.yml up -d
	@echo "Grafana: http://localhost:3000 (admin/admin). OTLP HTTP: http://localhost:4318"

obs-down: ## Stop the local otel-lgtm stack
	docker compose -f deploy/otel-lgtm/docker-compose.yml down
```

- [ ] **Step 4: Bring it up and verify end-to-end (manual)**

```bash
make obs
set -a; . ./.env; set +a
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf DINNERWISE_LIVE=1 \
  go test ./internal/agent/ -run TestLiveOpenAI -v
```
Open http://localhost:3000 → the dinnerwise dashboard shows tokens/latency/cost; Explore (Tempo) shows an `agent.ask` trace with `agent.tool` and `gen_ai` child spans. Fix metric names in `dashboard.json` if panels are empty, then re-check.

- [ ] **Step 5: Document the run env**

Append to `.env` (controller does this; it's gitignored): `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318`, `OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf`. Note in the commit body that these enable observability.

- [ ] **Step 6: Commit**
```bash
git add deploy/otel-lgtm/ Makefile
git commit -m "feat: local grafana/otel-lgtm stack + provisioned dashboard

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review

- **Spec coverage:** OTel+Sigil bootstrap with no-op gate (T2) · config (T1) · generation capture via Sigil ResponsesNew (T4) · full agent trace agent.ask/agent.tool (T5) · local $ cost price map (T3) · otel-lgtm + provisioned dashboard (T6) · generation export none / OTel-only (T2) · graceful degradation (T2/T4/T5 nil guards) · tokens owned by Sigil, only cost added (T3/T4) · main wiring + shutdown (T4). Covered.
- **Type consistency:** `NewService(cfg, providers, recipes, meals)` (T4) and `newLLMAgent(..., tracer)` (T5) are the two integration-point signature changes; both call sites (service.go, main.go, tests) are updated in the task that changes them. `Providers{Tracer, Sigil}` used consistently in T2/T4/T5. `recordCost`/`costUSD` (T3) consumed in T4.
- **Placeholder scan:** none — every step has real code or a real command. SDK-name "VERIFY" callouts (Sigil constants, ResponsesNew shape, exported Prometheus metric names) are explicit drift guards with build/Explore as the check, not deferred work.
- **Deviation risk:** the exported Prometheus metric names for the dashboard can't be known until the collector runs; T6 explicitly bakes them in after a live turn rather than guessing — a working dashboard is the deliverable.
