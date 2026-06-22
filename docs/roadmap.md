# Roadmap — with more time

Dinnerwise is a time-boxed exercise. The agent works end to end, but it's a
**one-off**: the tool loop, transport, and observability are clean, yet
everything is wired directly to this product's recipe/meal domain. Now that the
working contract is proven, this doc lays out the path from "one agent" to a
**reusable, observable agent runtime**, plus the productionization work that
shipping for real would need.

Roughly in priority order.

## 1. A reusable agent core (headline)

Today you could not build a second agent from this code without copying and
rewriting it. The *shape* is reusable; nothing is *parameterized*.

### What's generic vs. hardwired

Genuinely reusable in shape, and worth keeping:

- **The tool-calling loop** (`internal/agent/llm.go`, `Run`) — round cap, echo
  assistant tool calls and their outputs back into the conversation, stream
  events per round, best-effort final summary.
- **The `llmClient` seam** over the OpenAI Responses API, with a scripted
  fallback (`script.go`) behind the *same* interface so the app runs offline.
- **The `AskEvent` streaming contract** and the OpenTelemetry + Sigil cost/trace
  wiring.

But hardwired to dinnerwise everywhere it matters:

- `llmAgent` holds concrete `*recipe.Repo` and `*meal.Repo`, not an abstraction.
- `executeTool` is a `switch` on three tool names taking those repos directly
  (`tools.go`). Adding a tool means editing that switch.
- `toolDefs()` hardcodes the same three tools, and `systemPrompt`, `thinkingFor`,
  and `detailFor` all enumerate them by name.
- The `reference` and `navigate` events are recipe/meal/route-shaped in the proto
  itself.

So a second agent today means copying the loop and rewriting the switch, the tool
definitions, the prompt, the "thinking" strings, and the event mapping. The
reusable nucleus is ~50 lines of loop, entangled with concrete types and
string switches.

### The missing abstraction

The loop should depend on **"a set of tools" and "a system prompt"**, not on
recipes and meals:

- A **`Tool` interface** — `Name() / JSONSchema() / Execute(ctx, args) →
  (events, summary)` — plus a registry. `executeTool`'s switch and `toolDefs()`
  collapse into "iterate the registry."
- **Agent configuration as parameters** — system prompt, tool set, model, and
  `maxRounds` passed in, not package-level consts and concrete struct fields.
  Construction becomes `agent.New(systemPrompt, tools...)`.
- **A domain-agnostic event/reference contract** — today `reference` and
  `navigate` assume recipe/meal/route semantics. A reusable core needs either a
  generic reference shape or a typed extension point so the transport doesn't
  bake in one product's vocabulary.

With those, dinnerwise becomes *one consumer* of a generic core: the repos and
the recipe/meal reference shapes move out into the tool implementations.

### Two routes

**(a) Build a thin core ourselves (Go).** Extract the loop + `llmClient` seam +
tool registry into a small package; dinnerwise's tools become one consumer.
Low-magic, keeps the Sigil/OTel instrumentation and the offline scripted
fallback exactly as they are. Most of the work is the `Tool` interface and
de-domaining the event contract — the loop itself barely changes.

**(b) Adopt a third-party agent framework.** Gains: tool orchestration,
multi-agent handoffs, memory/state, and retries we'd otherwise build. Costs: we
would remap or lose our own `AskEvent` streaming contract, the Sigil
instrumentation, and the scripted offline fallback — all of which are currently
strengths. There's also a Go-ecosystem caveat: the mature agent frameworks
(LangGraph, the OpenAI Agents SDK) are Python-first, so in Go we'd largely be
hand-rolling against the SDK anyway.

**Recommendation:** route (a). The hard parts (streaming transport, tools,
observability, offline fallback) are already built and are *assets*; the only
thing missing is the `Tool`/registry seam. Extracting it is a smaller, lower-risk
step than adopting a framework and giving those assets up — and it keeps the
observability story (the Grafana-relevant part) entirely in our hands.

## 2. Real token streaming from OpenAI

**We are not using OpenAI's streaming API.** The agent calls the blocking
`Responses.New` (`openai_client.go`), waits for the entire model turn, then
`textChunks()` slices the finished text into a few `TextDelta` events to *mimic*
streaming on the wire (`script.go` says as much in its own comment). The
token-by-token feel in the browser is cosmetic — text only appears after the
whole turn (reasoning + any tool calls) completes, so real first-token latency is
unchanged.

This matters most for the **final spoken answer** (the turn with no tool calls),
which is exactly where a user is waiting on text.

What it would take:

- Swap `Responses.New` → `Responses.NewStreaming`.
- Consume the SSE event stream: `response.output_text.delta` → emit a real
  `TextDelta` per delta; `response.function_call_arguments.delta` / `.done` to
  assemble tool calls incrementally; `response.completed` for final usage.
- **Reshape the `Respond` seam.** Today it returns a *finished* `llmTurn`, and
  `Run` decides tool-vs-final only after the full turn is back. Streaming means
  emitting text deltas as they arrive while tool calls may still appear in the
  same turn — so the seam changes from "return a finished turn" to "emit as it
  arrives" (a callback/iterator, like `Run` already uses downstream). This is the
  real work.
- Move cost capture: `recordCost` currently reads `resp.Usage`; with streaming it
  reads usage off the terminal `response.completed` event.
- Confirm the Sigil provider supports the streaming call (today
  `sigilopenai.ResponsesNew` wraps the blocking variant) so we don't lose
  token/cost instrumentation in the process.

The scripted fallback already streams chunk-by-chunk, so the client side needs no
change — this is purely a backend/seam refactor.

## 3. Productionization

Table stakes for shipping this as a real product rather than a demo:

- **Persist conversations server-side.** History lives only in the browser today
  and is lost on reload. Storing turns in SQLite (the agent already resends
  history each turn, so the wire contract wouldn't change) would survive reloads,
  enable multi-device continuity, and give a basis for resumable sessions. The
  agent stays stateless either way — the server just becomes the source of truth
  for the transcript instead of the client.
- **Per-user accounts and data scoping.** Recipes, meals, and conversations are
  global today; real use needs auth and per-user ownership.
- **Context-window management.** History is capped at a hard 10 turns
  (`maxHistoryTurns`); summarizing older turns would scale conversations past
  that without dropping context.
- **Agent/tool error states in the UI.** Tool failures currently fold into the
  model's reply; surfacing them explicitly would make failures legible.
