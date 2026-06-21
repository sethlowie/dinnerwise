# Agent Transport Plumbing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stream typed agent events (text/thinking/tool_call/navigate/done) from a scripted Go backend to a React chat panel that renders them and executes backend-driven navigation — no LLM.

**Architecture:** A Connect server-streaming `AgentService.Ask` returns a oneof `AskEvent` stream produced by a pure `script(text)` function (hardcoded). The frontend consumes the stream with a direct `createClient` + `for await` loop in a `ChatProvider`, rendering events and calling `router.navigate` on `navigate` events. `/recipes` gains a typed `ingredient` search param and filters client-side.

**Tech Stack:** Go 1.25, ConnectRPC (server-streaming), buf v2, React 19 + Vite, TanStack Router, `@connectrpc/connect` client, Tailwind v4.

## Global Constraints

- No LLM. Backend behavior is the hardcoded `script(text)` function.
- Transport is a Connect **server-streaming** RPC. Frontend consumes via `createClient(AgentService, transport)` + `for await` — NOT `streamedQuery`.
- Generated code is gitignored — never commit `*.pb.go`, `*.connect.go`, `*_pb.ts`, `*_connectquery.ts`. Regenerate with `make gen`.
- Generated Go proto pkg imported as `agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"`; connect pkg `agentv1connect`.
- The `navigate` action is generic `{to, map<string,string> search}`; the frontend casts via `NavigateOptions` (no `any`).
- Scripted keyword list: `chicken, tomato, tofu, garlic, broccoli, rice, pasta`.
- Streaming delay ~60ms between events for the demo; 0 in tests.
- Chat hook/context live in a `.ts` module; components in `.tsx` (Fast Refresh hygiene, mirroring `theme.tsx`/`theme-context.ts`).
- Module path: `github.com/sethlowie/dinnerwise`. Commit messages end with: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

---

## File Structure

- Create: `internal/agent/v1/agent.proto` — AgentService contract.
- Create: `internal/agent/script.go` — pure `script(text) []*agentv1.AskEvent` + event constructors.
- Create: `internal/agent/service.go` — `Service`, `NewService`, `NewServiceWithDelay`, streaming `Ask`.
- Create: `internal/agent/script_test.go` — unit tests for `script`.
- Create: `internal/agent/service_test.go` — streaming integration test (httptest + client).
- Modify: `cmd/server/main.go` — mount AgentService handler.
- Modify: `web/app/src/routes/recipes.tsx` — typed `ingredient` search + client-side filter + chip.
- Create: `web/app/src/chat/agentClient.ts` — Connect client for AgentService.
- Create: `web/app/src/chat/chatContext.ts` — context, types, `useChat`.
- Create: `web/app/src/chat/ChatProvider.tsx` — provider: thread state, `ask`, streaming loop, navigation.
- Create: `web/app/src/chat/ChatPanel.tsx` — thread + input UI (hero + docked modes).
- Modify: `web/app/src/routes/__root.tsx` — wrap in ChatProvider; hero vs split layout.

Generated (not committed): `internal/agent/v1/agent.pb.go`, `…/agentv1connect/agent.connect.go`, `web/app/src/gen/internal/agent/v1/agent_pb.ts`, `…/agent-AgentService_connectquery.ts`.

---

## Task 1: AgentService proto + scripted backend (Go)

**Files:**
- Create: `internal/agent/v1/agent.proto`
- Create: `internal/agent/script.go`
- Create: `internal/agent/service.go`
- Test: `internal/agent/script_test.go`, `internal/agent/service_test.go`

**Interfaces:**
- Consumes: nothing (new package).
- Produces:
  - generated `agentv1` types: `AskRequest`, `AskEvent` (oneof: `AskEvent_Text`, `AskEvent_Thinking`, `AskEvent_ToolCall`, `AskEvent_Navigate`, `AskEvent_Done`), `TextDelta`, `Thinking`, `ToolCall`, `Navigate`, `Done`.
  - generated `agentv1connect.AgentServiceHandler` + `NewAgentServiceHandler` + `NewAgentServiceClient`.
  - `func NewService() agentv1connect.AgentServiceHandler`
  - `func NewServiceWithDelay(d time.Duration) agentv1connect.AgentServiceHandler`

- [ ] **Step 1: Write the proto**

Create `internal/agent/v1/agent.proto`:
```proto
syntax = "proto3";

package internal.agent.v1;

message AskRequest {
  string text = 1;
}

message TextDelta { string text = 1; }
message Thinking { string text = 1; }
message ToolCall {
  string name = 1;
  string detail = 2;
}
message Navigate {
  string to = 1;
  map<string, string> search = 2;
}
message Done {}

message AskEvent {
  oneof event {
    TextDelta text = 1;
    Thinking thinking = 2;
    ToolCall tool_call = 3;
    Navigate navigate = 4;
    Done done = 5;
  }
}

service AgentService {
  rpc Ask(AskRequest) returns (stream AskEvent);
}
```

- [ ] **Step 2: Generate code**

Run: `make gen`
Expected: succeeds; `internal/agent/v1/agent.pb.go`, `internal/agent/v1/agentv1connect/agent.connect.go`, and `web/app/src/gen/internal/agent/v1/*.ts` exist.
Verify: `ls internal/agent/v1/agentv1connect/agent.connect.go web/app/src/gen/internal/agent/v1/`

- [ ] **Step 3: Write the failing script test**

Create `internal/agent/script_test.go`:
```go
package agent

import (
	"testing"

	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
)

func TestScriptMatchEmitsToolCallAndNavigate(t *testing.T) {
	events := script("what recipes have chicken?")

	var sawToolCall bool
	var nav *agentv1.Navigate
	for _, e := range events {
		switch ev := e.Event.(type) {
		case *agentv1.AskEvent_ToolCall:
			sawToolCall = true
		case *agentv1.AskEvent_Navigate:
			nav = ev.Navigate
		}
	}
	if !sawToolCall {
		t.Fatal("expected a tool_call event")
	}
	if nav == nil {
		t.Fatal("expected a navigate event")
	}
	if nav.GetTo() != "/recipes" {
		t.Fatalf("navigate.to = %q, want /recipes", nav.GetTo())
	}
	if nav.GetSearch()["ingredient"] != "chicken" {
		t.Fatalf("search[ingredient] = %q, want chicken", nav.GetSearch()["ingredient"])
	}
	if _, ok := events[len(events)-1].Event.(*agentv1.AskEvent_Done); !ok {
		t.Fatal("expected last event to be Done")
	}
}

func TestScriptMatchOrdering(t *testing.T) {
	events := script("chicken please")
	if len(events) < 4 {
		t.Fatalf("too few events: %d", len(events))
	}
	if _, ok := events[0].Event.(*agentv1.AskEvent_Thinking); !ok {
		t.Fatal("first event should be Thinking")
	}
	if _, ok := events[1].Event.(*agentv1.AskEvent_ToolCall); !ok {
		t.Fatal("second event should be ToolCall")
	}
	// Navigate must come before Done.
	navIdx, doneIdx := -1, -1
	for i, e := range events {
		switch e.Event.(type) {
		case *agentv1.AskEvent_Navigate:
			navIdx = i
		case *agentv1.AskEvent_Done:
			doneIdx = i
		}
	}
	if navIdx == -1 || doneIdx == -1 || navIdx > doneIdx {
		t.Fatalf("expected Navigate before Done (nav=%d done=%d)", navIdx, doneIdx)
	}
}

func TestScriptNoMatchHasNoNavigate(t *testing.T) {
	events := script("hello there")
	for _, e := range events {
		if _, ok := e.Event.(*agentv1.AskEvent_Navigate); ok {
			t.Fatal("did not expect a navigate event for an unmatched query")
		}
	}
	if _, ok := events[len(events)-1].Event.(*agentv1.AskEvent_Done); !ok {
		t.Fatal("expected last event to be Done")
	}
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `go test ./internal/agent/ -run TestScript`
Expected: FAIL — compile error `undefined: script`.

- [ ] **Step 5: Write the script**

Create `internal/agent/script.go`:
```go
package agent

import (
	"strings"

	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
)

// knownIngredients are the keywords the scripted (no-LLM) backend recognizes.
// They mirror the seeded recipe fixtures.
var knownIngredients = []string{
	"chicken", "tomato", "tofu", "garlic", "broccoli", "rice", "pasta",
}

// script is the entire hardcoded backend behavior: given user text it returns
// the sequence of events to stream. It is a pure function so it can be unit
// tested without any streaming machinery. The real LLM agent will replace this.
func script(text string) []*agentv1.AskEvent {
	lower := strings.ToLower(text)
	for _, ing := range knownIngredients {
		if strings.Contains(lower, ing) {
			return []*agentv1.AskEvent{
				thinkingEvent("Looking for recipes with " + ing + "…"),
				toolCallEvent("search_recipes", "ingredient="+ing),
				textEvent("Here"),
				textEvent(" are the recipes"),
				textEvent(" with " + ing + "."),
				navigateEvent("/recipes", map[string]string{"ingredient": ing}),
				doneEvent(),
			}
		}
	}
	return []*agentv1.AskEvent{
		thinkingEvent("No specific ingredient mentioned…"),
		textEvent("I can help you find recipes — try asking about an ingredient like chicken."),
		doneEvent(),
	}
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

func doneEvent() *agentv1.AskEvent {
	return &agentv1.AskEvent{Event: &agentv1.AskEvent_Done{Done: &agentv1.Done{}}}
}
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/agent/ -run TestScript`
Expected: PASS.

- [ ] **Step 7: Write the failing streaming test**

Create `internal/agent/service_test.go`:
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

func TestAskStreamsScriptedEvents(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle(agentv1connect.NewAgentServiceHandler(NewServiceWithDelay(0)))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := agentv1connect.NewAgentServiceClient(http.DefaultClient, srv.URL)
	stream, err := client.Ask(context.Background(),
		connect.NewRequest(&agentv1.AskRequest{Text: "chicken"}))
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}

	var sawNavigate, sawDone bool
	for stream.Receive() {
		switch stream.Msg().Event.(type) {
		case *agentv1.AskEvent_Navigate:
			sawNavigate = true
		case *agentv1.AskEvent_Done:
			sawDone = true
		}
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream error: %v", err)
	}
	if !sawNavigate {
		t.Fatal("expected a navigate event over the stream")
	}
	if !sawDone {
		t.Fatal("expected a done event over the stream")
	}
}
```

- [ ] **Step 8: Run test to verify it fails**

Run: `go test ./internal/agent/ -run TestAsk`
Expected: FAIL — compile error `undefined: NewServiceWithDelay` / `NewService`.

- [ ] **Step 9: Write the service**

Create `internal/agent/service.go`:
```go
// Package agent holds the (currently scripted, no-LLM) AgentService: it streams
// typed events — assistant text plus meta (thinking, tool calls) and a
// navigate action — in response to user text.
package agent

import (
	"context"
	"time"

	connect "connectrpc.com/connect"
	agentv1 "github.com/sethlowie/dinnerwise/internal/agent/v1"
	"github.com/sethlowie/dinnerwise/internal/agent/v1/agentv1connect"
)

// Service implements agentv1connect.AgentServiceHandler by streaming the events
// produced by script(). delay paces the stream to simulate token streaming.
type Service struct {
	delay time.Duration
}

// NewService returns a handler with a lifelike streaming delay.
func NewService() agentv1connect.AgentServiceHandler {
	return &Service{delay: 60 * time.Millisecond}
}

// NewServiceWithDelay returns a handler with an explicit delay (use 0 in tests).
func NewServiceWithDelay(d time.Duration) agentv1connect.AgentServiceHandler {
	return &Service{delay: d}
}

func (s *Service) Ask(
	ctx context.Context,
	req *connect.Request[agentv1.AskRequest],
	stream *connect.ServerStream[agentv1.AskEvent],
) error {
	for _, ev := range script(req.Msg.GetText()) {
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

- [ ] **Step 10: Run tests + vet**

Run: `go vet ./internal/agent/ && go test ./internal/agent/`
Expected: PASS (all script + streaming tests).

- [ ] **Step 11: Commit**

Generated code is gitignored; commit only the proto + hand-written Go.
```bash
git add internal/agent/v1/agent.proto internal/agent/script.go internal/agent/service.go internal/agent/script_test.go internal/agent/service_test.go
git commit -m "feat: add scripted AgentService streaming RPC

Server-streaming Ask emits typed events (text/thinking/tool_call/navigate/
done) from a pure script(text) function; no LLM. Unit tests cover script
ordering/navigate; an httptest integration test covers the stream.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Wire AgentService into the server

**Files:**
- Modify: `cmd/server/main.go`

**Interfaces:**
- Consumes: `agent.NewService`, `agentv1connect.NewAgentServiceHandler`.
- Produces: server mounts AgentService at `/internal.agent.v1.AgentService/`.

- [ ] **Step 1: Add imports**

In `cmd/server/main.go`, add to the import block (keep existing imports):
```go
	"github.com/sethlowie/dinnerwise/internal/agent"
	"github.com/sethlowie/dinnerwise/internal/agent/v1/agentv1connect"
```

- [ ] **Step 2: Mount the handler**

In `cmd/server/main.go`, immediately after the existing line:
```go
	mux.Handle(recipev1connect.NewRecipeServiceHandler(recipe.NewService(repo)))
```
add:
```go
	mux.Handle(agentv1connect.NewAgentServiceHandler(agent.NewService()))
```

- [ ] **Step 3: Verify build + full suite**

Run: `go build ./... && go test ./...`
Expected: build succeeds; `ok` for `internal/agent`, `internal/db`, `internal/recipe`.

- [ ] **Step 4: Verify the server boots**

Run:
```bash
DINNERWISE_DB="$(mktemp -d)/dinnerwise.db" ADDR=:8095 go run ./cmd/server/ &
sleep 2
curl -s -o /dev/null -w "recipes %{http_code}\n" -X POST \
  http://localhost:8095/internal.recipe.v1.RecipeService/ListRecipes \
  -H "Content-Type: application/json" -H "Connect-Protocol-Version: 1" -d '{}'
kill %1 2>/dev/null
```
Expected: boot log `server: 3 recipes loaded …`, then `recipes 200`. (AgentService streaming is covered by the Go integration test in Task 1; it is not curl-friendly due to enveloped framing.)

- [ ] **Step 5: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: mount AgentService on the server

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: /recipes client-side ingredient filter

**Files:**
- Modify: `web/app/src/routes/recipes.tsx`

**Interfaces:**
- Consumes: existing `listRecipes` hook; `rootRoute`.
- Produces: `recipesRoute` now declares `validateSearch` → `{ ingredient?: string }`; the agent's `navigate({to:"/recipes", search:{ingredient}})` lands on a filtered list.

- [ ] **Step 1: Replace recipes.tsx with the filtered version**

Replace the entire contents of `web/app/src/routes/recipes.tsx` with:
```tsx
import { createRoute, getRouteApi, Link } from "@tanstack/react-router";
import { useQuery } from "@connectrpc/connect-query";
import { listRecipes } from "../gen/internal/recipe/v1/recipe-RecipeService_connectquery";
import { rootRoute } from "./__root";

const routeApi = getRouteApi("/recipes");

function Recipes() {
  const { ingredient } = routeApi.useSearch();
  const navigate = routeApi.useNavigate();
  const { data, error, isPending } = useQuery(listRecipes, {});

  if (isPending) return <p className="text-muted-foreground">Loading…</p>;
  if (error)
    return <p className="text-red-600 dark:text-red-400">{error.message}</p>;

  const term = ingredient?.toLowerCase().trim() ?? "";
  const recipes = term
    ? data.recipes.filter((r) =>
        r.ingredients.some((i) => i.name.toLowerCase().includes(term)),
      )
    : data.recipes;

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <h1 className="text-2xl font-semibold">Recipes</h1>
        {ingredient && (
          <button
            onClick={() => navigate({ search: {} })}
            className="rounded-full bg-accent px-2 py-0.5 text-xs text-accent-foreground"
          >
            ingredient: {ingredient} ✕
          </button>
        )}
      </div>
      <ul className="grid gap-4 sm:grid-cols-2">
        {recipes.map((r) => (
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
  validateSearch: (search: Record<string, unknown>): { ingredient?: string } => ({
    ingredient: typeof search.ingredient === "string" ? search.ingredient : undefined,
  }),
  component: Recipes,
});
```

- [ ] **Step 2: Verify typecheck, lint, build**

Run (from `web/app`): `npx tsc -b && pnpm lint && pnpm build`
Expected: `tsc` clean, eslint 0 problems, `vite build` succeeds. Return to repo root: `cd ../..`

- [ ] **Step 3: Verify the filter manually**

Run:
```bash
DINNERWISE_DB="$(mktemp -d)/dinnerwise.db" ADDR=:8094 go run ./cmd/server/ &
(cd web/app && VITE_API_URL=http://localhost:8094 pnpm dev --port 5175 &)
sleep 4
curl -s -o /dev/null -w "filtered %{http_code}\n" "http://localhost:5175/recipes?ingredient=chicken"
kill %1 %2 2>/dev/null; pkill -f vite 2>/dev/null
```
Expected: `filtered 200`. (Visual confirmation that only chicken recipes show is the controller's runtime check.)

- [ ] **Step 4: Commit**

```bash
git add web/app/src/routes/recipes.tsx
git commit -m "feat: client-side ingredient filter on /recipes

Typed ingredient search param filters the fetched recipes; clearable chip.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Chat UX — provider, panel, layout, streaming + navigation

**Files:**
- Create: `web/app/src/chat/agentClient.ts`
- Create: `web/app/src/chat/chatContext.ts`
- Create: `web/app/src/chat/ChatProvider.tsx`
- Create: `web/app/src/chat/ChatPanel.tsx`
- Modify: `web/app/src/routes/__root.tsx`

**Interfaces:**
- Consumes: generated `AgentService` from `../gen/internal/agent/v1/agent_pb`; existing `transport`; `recipesRoute`'s `ingredient` search (navigation target).
- Produces: a persistent chat panel; `useChat()` hook; backend `navigate` events drive `router.navigate`.

Note: no frontend unit-test harness — verified by `tsc -b`, `eslint`, `vite build`, and a runtime check.

- [ ] **Step 1: Create the Connect client**

Create `web/app/src/chat/agentClient.ts`:
```ts
import { createClient } from "@connectrpc/connect";
import { AgentService } from "../gen/internal/agent/v1/agent_pb";
import { transport } from "../transport";

// Server-streaming client; agentClient.ask({text}) returns an AsyncIterable<AskEvent>.
export const agentClient = createClient(AgentService, transport);
```

- [ ] **Step 2: Create the context, types, and hook**

Create `web/app/src/chat/chatContext.ts`:
```ts
import { createContext, useContext } from "react";

export type AssistantMessage = {
  thinking: string[];
  toolCalls: { name: string; detail: string }[];
  text: string;
  done: boolean;
};

export type Turn = {
  id: number;
  userText: string;
  assistant: AssistantMessage;
};

export type ChatContextValue = {
  turns: Turn[];
  isStreaming: boolean;
  ask: (text: string) => void;
};

export const ChatContext = createContext<ChatContextValue | null>(null);

export function useChat(): ChatContextValue {
  const ctx = useContext(ChatContext);
  if (!ctx) throw new Error("useChat must be used within a ChatProvider");
  return ctx;
}
```

- [ ] **Step 3: Create the provider (streaming loop + navigation)**

Create `web/app/src/chat/ChatProvider.tsx`:
```tsx
import { useRef, useState, type ReactNode } from "react";
import { useRouter, type NavigateOptions } from "@tanstack/react-router";
import { agentClient } from "./agentClient";
import {
  ChatContext,
  type AssistantMessage,
  type Turn,
} from "./chatContext";

const emptyAssistant: AssistantMessage = {
  thinking: [],
  toolCalls: [],
  text: "",
  done: false,
};

export function ChatProvider({ children }: { children: ReactNode }) {
  const router = useRouter();
  const [turns, setTurns] = useState<Turn[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const nextId = useRef(0);

  function ask(text: string) {
    const id = nextId.current++;
    setTurns((prev) => [
      ...prev,
      { id, userText: text, assistant: { ...emptyAssistant } },
    ]);
    setIsStreaming(true);

    const update = (fn: (a: AssistantMessage) => AssistantMessage) =>
      setTurns((prev) =>
        prev.map((t) => (t.id === id ? { ...t, assistant: fn(t.assistant) } : t)),
      );

    void (async () => {
      try {
        for await (const ev of agentClient.ask({ text })) {
          switch (ev.event.case) {
            case "thinking":
              update((a) => ({ ...a, thinking: [...a.thinking, ev.event.value.text] }));
              break;
            case "toolCall":
              update((a) => ({
                ...a,
                toolCalls: [
                  ...a.toolCalls,
                  { name: ev.event.value.name, detail: ev.event.value.detail },
                ],
              }));
              break;
            case "text":
              update((a) => ({ ...a, text: a.text + ev.event.value.text }));
              break;
            case "navigate": {
              const opts = {
                to: ev.event.value.to,
                search: ev.event.value.search,
              } as unknown as NavigateOptions;
              void router.navigate(opts);
              break;
            }
            case "done":
              update((a) => ({ ...a, done: true }));
              break;
          }
        }
      } finally {
        setIsStreaming(false);
      }
    })();
  }

  return (
    <ChatContext value={{ turns, isStreaming, ask }}>{children}</ChatContext>
  );
}
```

- [ ] **Step 4: Create the chat panel**

Create `web/app/src/chat/ChatPanel.tsx`:
```tsx
import { useState, type FormEvent } from "react";
import { useChat } from "./chatContext";

export function ChatPanel({ hero = false }: { hero?: boolean }) {
  const { turns, isStreaming, ask } = useChat();
  const [input, setInput] = useState("");

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    const text = input.trim();
    if (!text || isStreaming) return;
    setInput("");
    ask(text);
  }

  const form = (
    <form onSubmit={onSubmit} className="flex gap-2">
      <input
        value={input}
        onChange={(e) => setInput(e.target.value)}
        placeholder="Ask about dinner…"
        className="flex-1 rounded-lg border border-border bg-card px-3 py-2 text-card-foreground"
      />
      <button
        disabled={isStreaming}
        className="rounded-lg bg-primary px-4 py-2 text-primary-foreground disabled:opacity-50"
      >
        Ask
      </button>
    </form>
  );

  if (hero) {
    return (
      <div className="w-full max-w-xl space-y-4 text-center">
        <h1 className="text-2xl font-semibold">What's for dinner?</h1>
        {form}
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <div className="flex-1 space-y-4 overflow-auto p-4">
        {turns.map((t) => (
          <div key={t.id} className="space-y-2">
            <div className="ml-auto w-fit rounded-lg bg-primary px-3 py-1.5 text-sm text-primary-foreground">
              {t.userText}
            </div>
            {t.assistant.thinking.length > 0 && (
              <details className="text-xs text-muted-foreground">
                <summary className="cursor-pointer">thinking</summary>
                {t.assistant.thinking.map((th, i) => (
                  <p key={i} className="italic">
                    {th}
                  </p>
                ))}
              </details>
            )}
            {t.assistant.toolCalls.map((tc, i) => (
              <p key={i} className="text-xs text-muted-foreground">
                🔧 {tc.name}({tc.detail})
              </p>
            ))}
            {t.assistant.text && (
              <p className="text-sm text-foreground">{t.assistant.text}</p>
            )}
          </div>
        ))}
      </div>
      <div className="border-t border-border p-4">{form}</div>
    </div>
  );
}
```

- [ ] **Step 5: Rewire the root layout**

Replace the entire contents of `web/app/src/routes/__root.tsx` with:
```tsx
import { createRootRoute, Link, Outlet } from "@tanstack/react-router";
import { ThemeToggle } from "../theme";
import { ChatProvider } from "../chat/ChatProvider";
import { ChatPanel } from "../chat/ChatPanel";
import { useChat } from "../chat/chatContext";

function Shell() {
  const { turns } = useChat();
  const active = turns.length > 0;

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <header className="border-b border-border">
        <div className="mx-auto flex w-full max-w-5xl items-center justify-between px-4 py-3">
          <nav className="flex items-center gap-4">
            <span className="font-semibold">dinnerwise</span>
            <Link to="/" className="text-muted-foreground [&.active]:text-foreground">
              Home
            </Link>
            <Link
              to="/recipes"
              className="text-muted-foreground [&.active]:text-foreground"
            >
              Recipes
            </Link>
          </nav>
          <ThemeToggle />
        </div>
      </header>

      {active ? (
        <div className="flex min-h-0 flex-1">
          <main className="flex-1 overflow-auto px-4 py-8">
            <div className="mx-auto max-w-3xl">
              <Outlet />
            </div>
          </main>
          <aside className="flex w-96 flex-col border-l border-border">
            <ChatPanel />
          </aside>
        </div>
      ) : (
        <main className="flex flex-1 items-center justify-center px-4">
          <ChatPanel hero />
        </main>
      )}
    </div>
  );
}

function RootLayout() {
  return (
    <ChatProvider>
      <Shell />
    </ChatProvider>
  );
}

export const rootRoute = createRootRoute({ component: RootLayout });
```

- [ ] **Step 6: Verify typecheck, lint, build**

Run (from `web/app`): `npx tsc -b && pnpm lint && pnpm build`
Expected: `tsc` clean (the `NavigateOptions` cast resolves; `ev.event.case` oneof narrows), eslint 0 problems (chat hook/context in `.ts`, components in `.tsx`; route files covered by the existing override), `vite build` succeeds. Return to repo root: `cd ../..`

- [ ] **Step 7: Runtime check (full plumbing end-to-end)**

Run (from repo root):
```bash
DINNERWISE_DB="$(mktemp -d)/dinnerwise.db" ADDR=:8093 go run ./cmd/server/ &
(cd web/app && VITE_API_URL=http://localhost:8093 pnpm dev --port 5176 &)
sleep 4
curl -s -o /dev/null -w "home %{http_code}\n" http://localhost:5176/
kill %1 %2 2>/dev/null; pkill -f vite 2>/dev/null
```
Expected: `home 200`. Controller's manual runtime check: from the home hero input, ask "what recipes have chicken" → the chat panel docks on the right and streams a thinking block, a `🔧 search_recipes(ingredient=chicken)` line, and the answer text, and the left pane navigates to `/recipes?ingredient=chicken` showing only chicken recipes.

- [ ] **Step 8: Commit**

```bash
git add web/app/src/chat/ web/app/src/routes/__root.tsx
git commit -m "feat: app-wide chat panel with backend-driven navigation

Home input docks into a right chat panel; ChatProvider streams AgentService
events (text/thinking/tool_call) and executes navigate events via the router.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage:**
- Server-streaming `Ask` with oneof events → Task 1 (proto + service).
- Scripted, no-LLM behavior + keyword list + ordering → Task 1 (`script`, tests).
- Backend mounted → Task 2.
- Direct `createClient` + `for await` consumption (not streamedQuery) → Task 4 (agentClient, ChatProvider).
- Home input → docked right chat panel; persists across navigation → Task 4 (`__root` Shell, ChatProvider above Outlet).
- Render text/thinking/tool_call meta → Task 4 (ChatPanel).
- Backend-driven navigation via `router.navigate` → Task 4 (ChatProvider navigate case).
- `/recipes` typed `ingredient` search + client-side filter + clearable chip → Task 3.
- Streaming delay (60ms; 0 in tests) → Task 1 (`NewService`/`NewServiceWithDelay`).
- Testing: `script` unit tests + streaming integration test; frontend tsc/eslint/build/runtime → Tasks 1, 3, 4.

**Placeholder scan:** none — every step has concrete code/commands.

**Type consistency:** Go event constructors (`textEvent`, `thinkingEvent`, `toolCallEvent`, `navigateEvent`, `doneEvent`) and oneof wrappers (`AskEvent_Text/Thinking/ToolCall/Navigate/Done`) are used consistently across `script.go`/tests. `NewService`/`NewServiceWithDelay` used in service + tests + main. Frontend oneof cases (`"text"`,`"thinking"`,`"toolCall"`,`"navigate"`,`"done"`) match the generated camelCase localNames for proto fields (`tool_call`→`toolCall`); `useChat`/`ChatProvider`/`ChatPanel`/`agentClient` names align across the chat module; `recipesRoute` `ingredient` search param matches the navigate target in Task 1's `script`.
