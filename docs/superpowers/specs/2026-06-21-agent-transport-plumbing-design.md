# Agent Transport Plumbing (Scripted, No LLM) — Design

**Date:** 2026-06-21
**Status:** Approved (design)

## Goal

Build the end-to-end plumbing for an app-wide agent **without any LLM**: send
user text to the server, stream back a typed sequence of events (assistant text,
plus meta — thinking and tool calls), and let the **backend trigger client-side
navigation**. The backend is a hardcoded/scripted handler. The real LLM agent
drops in behind this same contract later.

Rationale for the no-agent approach: the streaming protocol, the chat UX, and the
backend-driven navigation mechanism are the hard, novel parts and are worth
dialing in independently. A scripted backend lets us nail the contract and the UX
without LLM complexity; "smarts" slot in behind the same interface later.

## Scope

In: a Connect server-streaming `AgentService.Ask`, a scripted handler, a chat
panel UX (home input → docked right panel), executing `navigate` events via the
router, and minimal client-side ingredient filtering at `/recipes` (the navigate
target made real).

Out (later slices): real LLM agent behind this contract, real tool execution,
server-side recipe filtering, page-context in the request, typed/validated action
schemas, multi-turn memory/persistence.

## Decisions (locked)

- **No LLM.** Backend responses/behaviors are hardcoded/scripted.
- **Transport:** Connect **server-streaming** RPC (`Ask`). Frontend consumes via
  the generated Connect client directly (`createClient` + `for await`), NOT
  `streamedQuery` — the stream carries side-effecting `navigate` events that
  should fire inline mid-stream, which a direct loop handles cleanly.
  (`streamedQuery` exists in our installed `@tanstack/react-query` 5.101.0 and
  remains an easy swap if we later only need to render accumulating chunks.)
- **Layout:** centered home input → docks into a **persistent right chat panel**;
  router `<Outlet/>` on the left. Chat state persists across route changes.
- **Navigate action:** generic `Navigate { string to; map<string,string> search }`;
  the frontend casts for `router.navigate`. Typed/validated actions come with the
  real agent.
- **Scripted matcher:** keys off a small hardcoded ingredient keyword list.
- **Destination:** `/recipes` gains a typed `ingredient` search param and filters
  the fetched recipes **client-side** (no backend/proto filtering changes).

## Architecture

```
React chat input ──Ask(text)──▶ Connect server-stream ──▶ scripted handler
      ▲                                                         │
      │  AskEvent stream (text / thinking / tool_call / navigate / done)
      │                                                         │
  ChatProvider (for-await loop)                                 │
      ├─ append text/thinking/tool_call to the active turn      │
      └─ on `navigate` → router.navigate({ to, search }) ──▶ /recipes?ingredient=chicken
                                                                  (client-side filter)
```

## Proto — `AgentService` (`internal/agent/v1/agent.proto`)

Package `internal.agent.v1`. Server-streaming RPC:

```proto
syntax = "proto3";
package internal.agent.v1;

message AskRequest {
  string text = 1;
}

message TextDelta { string text = 1; }
message Thinking  { string text = 1; }
message ToolCall  {
  string name   = 1;   // e.g. "search_recipes"
  string detail = 2;   // human-readable args, e.g. "ingredient=chicken"
}
message Navigate {
  string to = 1;                      // route path, e.g. "/recipes"
  map<string, string> search = 2;     // e.g. {"ingredient": "chicken"}
}
message Done {}

message AskEvent {
  oneof event {
    TextDelta text      = 1;
    Thinking  thinking  = 2;
    ToolCall  tool_call = 3;
    Navigate  navigate  = 4;
    Done      done      = 5;
  }
}

service AgentService {
  rpc Ask(AskRequest) returns (stream AskEvent);
}
```

`buf generate` (`make gen`) emits Go (`agent.pb.go`, `agentv1connect/agent.connect.go`)
and TS (`agent_pb.ts`, `agent-AgentService_connectquery.ts`). All gitignored.

## Backend — scripted handler (`internal/agent/service.go`, `package agent`)

Two pieces, so the logic is testable without streaming machinery:

1. **`script(text string) []*agentv1.AskEvent`** — pure function, the entire
   scripted behavior. Lowercase the text; scan for a known ingredient keyword
   from a small hardcoded list (e.g. `chicken, tomato, tofu, garlic, broccoli,
   rice, pasta`). 
   - **Match** (found ingredient `X`): emit, in order —
     `Thinking{"Looking for recipes with X…"}`,
     `ToolCall{name:"search_recipes", detail:"ingredient=X"}`,
     a few `TextDelta` chunks forming a sentence (e.g. "Here", " are the recipes",
     " with X."),
     `Navigate{to:"/recipes", search:{"ingredient":X}}`,
     `Done{}`.
   - **No match:** `Thinking{"…"}`, one `TextDelta` help message
     ("I can help you find recipes — try asking about an ingredient like chicken."),
     `Done{}` (no `Navigate`).

2. **`Service`** implementing `agentv1connect.AgentServiceHandler`:
   ```go
   func (s *Service) Ask(ctx, req *connect.Request[AskRequest],
       stream *connect.ServerStream[AskEvent]) error {
       for _, ev := range script(req.Msg.GetText()) {
           if err := stream.Send(ev); err != nil { return err }
           sleepStream(s.delay) // simulate token streaming; 0 in tests
       }
       return nil
   }
   ```
   `NewService()` defaults `delay` to a small value (e.g. 60ms) for a lifelike
   demo; a `NewServiceWithDelay(0)` (or exported field) is used by tests.

Mounted in `cmd/server/main.go` alongside RecipeService:
`mux.Handle(agentv1connect.NewAgentServiceHandler(agent.NewService()))`.

## Frontend

- **Client:** `createClient(AgentService, transport)` (transport.ts is reused).
- **`ChatProvider`** (`web/app/src/chat/ChatProvider.tsx`) — React context holding
  the thread and exposing `ask(text)`:
  - Thread is an array of turns: each turn has the user's `text` and an assistant
    message accumulating `{ thinking[], toolCalls[], text }`.
  - `ask(text)` pushes a user turn + empty assistant message, then
    `for await (const ev of client.ask({ text }))` switches on `ev.event.case`:
    `text` → append to assistant text; `thinking` → append; `tool_call` → append;
    `navigate` → `router.navigate({ to: ev.value.to, search: ev.value.search })`
    (cast `to` as needed); `done` → mark complete.
  - Tracks an `isStreaming` flag for input disabling.
  - Lives inside the root layout so it persists across navigation; uses the
    `router` instance (or `useRouter`) for navigation.
- **Layout (`web/app/src/routes/__root.tsx`):**
  - Empty thread → centered hero input (the home screen).
  - Non-empty thread → split: `<Outlet/>` (left) + `<ChatPanel/>` (right).
  - Keep the existing header/nav and `ThemeToggle`.
- **`ChatPanel`** (`web/app/src/chat/ChatPanel.tsx`) — renders the thread and the
  input. User turns as bubbles; assistant text as prose; `thinking` in a dim
  collapsible/italic block; tool calls as a `🔧 search_recipes(ingredient=chicken)`
  line. Input submits to `ask()`; disabled while streaming. (Style is rough; we
  refine later.)
- **`/recipes` filtering (`web/app/src/routes/recipes.tsx`):**
  - Add `validateSearch` for an optional `ingredient?: string`.
  - Filter the fetched recipes client-side:
    `recipes.filter(r => !ingredient || r.ingredients.some(i => i.name.toLowerCase().includes(ingredient.toLowerCase())))`.
  - Show an active-filter chip (e.g. "ingredient: chicken ✕") that clears the
    param when dismissed.

## Testing

- **Backend:** unit-test `script()` (pure function):
  - "what recipes have chicken" → events include a `ToolCall` and a `Navigate`
    whose `to == "/recipes"` and `search["ingredient"] == "chicken"`, ending in
    `Done`.
  - a query with no known ingredient → no `Navigate` event; ends in `Done`.
  - event ordering is `Thinking` → `ToolCall` → `TextDelta…` → `Navigate` → `Done`
    on a match.
- **Frontend:** no unit-test harness; verify via `tsc -b`, `eslint`, `vite build`,
  and a runtime check: from home, ask "what recipes have chicken" → the panel
  docks right and streams thinking/tool/text, and the app navigates to
  `/recipes?ingredient=chicken` showing only chicken recipes.

## Tradeoffs & notes

- The scripted backend is intentionally dumb; it exists to exercise every event
  type and the navigation mechanism. Replacing `script()` with an LLM-driven
  agent loop is the next slice and needs no protocol change.
- The generic `Navigate{to, search}` trades type-safety for simplicity now; the
  real agent slice will introduce validated, typed client actions.
- Client-side filtering at `/recipes` is a stopgap that keeps this slice focused
  on transport/UX; server-side filtering is a later slice.
- Streaming delay is a demo affordance; tests use 0 to stay fast.
