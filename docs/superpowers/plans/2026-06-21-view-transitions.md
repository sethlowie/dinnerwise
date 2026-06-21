# View-Transitions Shell Morph + Route-Driven Home (Slice 5) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the centered hero route-driven (`/` = hero, else split) and animate the home↔app change with router-native View Transitions (the input morphs center→docked, sidebar/dock slide in), replacing the bespoke traveling orb.

**Architecture:** Drive the layout off the current route instead of chat state, enable `defaultViewTransition` on the router so every navigation is wrapped in `document.startViewTransition`, and let CSS `view-transition-name` + `::view-transition-*` keyframes do the morph. A frontend-only fallback navigates home asks that don't navigate to `/recipes`.

**Tech Stack:** React 19 + Vite, TanStack Router (native view transitions), Tailwind v4 / CSS View Transitions API.

## Global Constraints

- Frontend only — no Go/proto changes (`go test ./...` must stay green). No new deps.
- Layout is route-driven: `/` → centered hero; every other route → split (sidebar + Outlet + dock). Chat thread persists in `ChatProvider`.
- View Transitions via `createRouter({ defaultViewTransition: true })` — no manual `startViewTransition`/`flushSync`. Must degrade gracefully (router handles unsupported browsers) and honor `prefers-reduced-motion`.
- The input pill is the shared morphing element (`view-transition-name: ask-input`); the orb stays a static pulse.
- No bare `any`; route files under `src/routes/**` are covered by the eslint react-refresh override.
- No frontend unit-test harness — verify with `tsc -b`/`eslint`/`vite build` + a runtime eyeball.
- Commit messages end with: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

---

## File Structure

- Modify: `web/app/src/router.tsx` (`defaultViewTransition: true`).
- Modify: `web/app/src/routes/__root.tsx` (route-driven Shell; dock `view-transition-name`).
- Modify: `web/app/src/routes/index.tsx` (stub — Shell renders the hero for `/`).
- Modify: `web/app/src/chat/ChatProvider.tsx` (no-navigate-from-home fallback).
- Modify: `web/app/src/chat/ChatPanel.tsx` (`view-transition-name` on the input pill).
- Modify: `web/app/src/chat/Sidebar.tsx` (`view-transition-name` on the aside).
- Modify: `web/app/src/index.css` (View-Transition keyframes + reduced-motion guard).

---

## Task 1: Route-driven layout + View Transitions enabled + home fallback

**Files:** Modify `web/app/src/router.tsx`, `web/app/src/routes/__root.tsx`, `web/app/src/routes/index.tsx`, `web/app/src/chat/ChatProvider.tsx`.

**Interfaces:**
- Consumes: existing `ChatProvider`, `ChatPanel`, `Sidebar`, `agentClient`, route tree.
- Produces: layout driven by `pathname` (`/` → hero, else split); router `defaultViewTransition: true`; an ask started on `/` that finishes without navigating falls back to `/recipes`.

- [ ] **Step 1: Enable router view transitions**

In `web/app/src/router.tsx`, change the `createRouter` call:
```tsx
export const router = createRouter({ routeTree, defaultViewTransition: true });
```
(Leave the rest of the file unchanged.)

- [ ] **Step 2: Make the Shell route-driven**

Replace the entire contents of `web/app/src/routes/__root.tsx` with:
```tsx
import { createRootRoute, Outlet, useRouterState } from "@tanstack/react-router";
import { ChatProvider } from "../chat/ChatProvider";
import { ChatPanel } from "../chat/ChatPanel";
import { Sidebar } from "../chat/Sidebar";

function Shell() {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  const isHome = pathname === "/";

  if (isHome) {
    return (
      <div className="relative flex min-h-screen items-center justify-center overflow-hidden bg-background text-foreground">
        <ChatPanel hero />
      </div>
    );
  }

  return (
    <div className="flex min-h-screen bg-background text-foreground">
      <Sidebar />
      <main className="min-w-0 flex-1 overflow-auto">
        <div className="mx-auto max-w-3xl px-6 py-8">
          <Outlet />
        </div>
      </main>
      <aside
        style={{ viewTransitionName: "dock" }}
        className="flex h-screen w-[360px] flex-none flex-col border-l border-border bg-card/70 backdrop-blur"
      >
        <ChatPanel />
      </aside>
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
(Note: `useChat` is no longer imported — the Shell branches on the route, not chat state. The dock `view-transition-name: dock` is added now; its keyframes land in Task 2 — until then it cross-fades, which is harmless.)

- [ ] **Step 3: Stub the index route**

Replace the entire contents of `web/app/src/routes/index.tsx` with:
```tsx
import { createRoute } from "@tanstack/react-router";
import { rootRoute } from "./__root";

// The centered hero for "/" is rendered by the root Shell, so this route's
// outlet content is unused — the route only needs to exist so "/" matches.
function Home() {
  return null;
}

export const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: Home,
});
```

- [ ] **Step 4: Add the no-navigate-from-home fallback**

In `web/app/src/chat/ChatProvider.tsx`, make three edits inside `ask`:

(a) immediately after `setIsStreaming(true);`, add:
```tsx
    const startedOnHome = window.location.pathname === "/";
    let navigated = false;
```

(b) in the `navigate` case, after `void router.navigate(opts);`, add:
```tsx
              navigated = true;
```

(c) immediately after the `for await (…) { … }` loop closes (still inside `try`, before the `} finally {`), add:
```tsx
        if (!navigated && startedOnHome) {
          void router.navigate({ to: "/recipes" });
        }
```

- [ ] **Step 5: Verify typecheck, lint, build**

Run (from `web/app`): `npx tsc -b && pnpm lint && pnpm build`
Expected: all clean (no unused `useChat`/`Link` imports remain; `defaultViewTransition` and `useRouterState` resolve). Return to repo root: `cd ../..`

- [ ] **Step 6: Verify backend untouched + runtime**

Run (from repo root):
```bash
go test ./... 2>&1 | grep -E "ok|FAIL"
DINNERWISE_DB="$(mktemp -d)/dinnerwise.db" ADDR=:8086 go run ./cmd/server/ &
(cd web/app && VITE_API_URL=http://localhost:8086 pnpm dev --port 5181 &)
sleep 4
curl -s -o /dev/null -w "home %{http_code}\n" http://localhost:5181/
curl -s -o /dev/null -w "recipes %{http_code}\n" http://localhost:5181/recipes
kill %1 %2 2>/dev/null; pkill -f vite 2>/dev/null
```
Expected: backend `ok` lines (unchanged); `home 200`, `recipes 200`. Controller's manual check: `/` shows the centered hero; navigating to `/recipes` shows the split (sidebar + dock); clicking **Home** returns to the centered hero; asking an unrecognized prompt ("hello") from the hero lands on `/recipes` with the reply in the dock (fallback). Transitions are the browser default cross-fade at this point (the morph polish is Task 2).

- [ ] **Step 7: Commit**

```bash
git add web/app/src/router.tsx web/app/src/routes/__root.tsx web/app/src/routes/index.tsx web/app/src/chat/ChatProvider.tsx
git commit -m "feat: route-driven home + router view transitions + home fallback

Shell branches on the route (/ = centered hero, else split) so Home returns to
the centered input; enable defaultViewTransition; a no-navigate ask from home
falls back to /recipes so replies are never stranded.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: The shell morph (named elements + keyframes)

**Files:** Modify `web/app/src/chat/ChatPanel.tsx`, `web/app/src/chat/Sidebar.tsx`, `web/app/src/index.css`.

**Interfaces:**
- Consumes: `defaultViewTransition` enabled (Task 1); the `dock` name on the dock aside (Task 1).
- Produces: `ask-input` (input pill) FLIPs center→docked; `sidebar` slides in; `dock` slides in; reduced-motion → instant.

Note: View Transitions are visual/runtime — verify with `tsc`/`eslint`/`vite build` + an eyeball.

- [ ] **Step 1: Name the input pill as the shared morph element**

In `web/app/src/chat/ChatPanel.tsx`, find the `inputPill` form's opening tag:
```tsx
    <form
      onSubmit={onSubmit}
      className="flex items-center gap-2.5 rounded-2xl border border-border bg-muted/40 py-2 pl-5 pr-2 shadow-lg"
    >
```
and add the `style` so it reads:
```tsx
    <form
      onSubmit={onSubmit}
      style={{ viewTransitionName: "ask-input" }}
      className="flex items-center gap-2.5 rounded-2xl border border-border bg-muted/40 py-2 pl-5 pr-2 shadow-lg"
    >
```
(Only one `ChatPanel` — hero or dock — is mounted at a time, so a single `ask-input` exists per state and the browser FLIPs it between the centered and docked positions.)

- [ ] **Step 2: Name the sidebar**

In `web/app/src/chat/Sidebar.tsx`, add `style` to the `<aside>` opening tag:
```tsx
    <aside
      style={{ viewTransitionName: "sidebar" }}
      className="flex w-52 flex-none flex-col border-r border-border bg-card/60 p-5 backdrop-blur"
    >
```

- [ ] **Step 3: Add the View-Transition keyframes**

Append to the end of `web/app/src/index.css`:
```css
/* ---- View Transitions: shell morph (centered hero <-> docked app) ---- */
/* The router wraps navigations in document.startViewTransition. Named elements
   below FLIP/slide between their hero and docked positions; everything else
   cross-fades (the browser default). */
::view-transition-old(root),
::view-transition-new(root) {
  animation-duration: 0.4s;
  animation-timing-function: cubic-bezier(0.4, 0, 0.2, 1);
}

::view-transition-group(ask-input) {
  animation-duration: 0.5s;
  animation-timing-function: cubic-bezier(0.5, 0.05, 0.2, 1);
}

@keyframes vtInLeft {
  from { opacity: 0; transform: translateX(-16px); }
  to { opacity: 1; transform: none; }
}
@keyframes vtOutLeft {
  from { opacity: 1; transform: none; }
  to { opacity: 0; transform: translateX(-16px); }
}
::view-transition-new(sidebar) { animation: vtInLeft 0.4s cubic-bezier(0.4, 0, 0.2, 1) both; }
::view-transition-old(sidebar) { animation: vtOutLeft 0.3s ease both; }

@keyframes vtInRight {
  from { opacity: 0; transform: translateX(16px); }
  to { opacity: 1; transform: none; }
}
@keyframes vtOutRight {
  from { opacity: 1; transform: none; }
  to { opacity: 0; transform: translateX(16px); }
}
::view-transition-new(dock) { animation: vtInRight 0.4s cubic-bezier(0.4, 0, 0.2, 1) both; }
::view-transition-old(dock) { animation: vtOutRight 0.3s ease both; }

@media (prefers-reduced-motion: reduce) {
  ::view-transition-group(*),
  ::view-transition-old(*),
  ::view-transition-new(*) {
    animation: none !important;
  }
}
```

- [ ] **Step 4: Verify typecheck, lint, build**

Run (from `web/app`): `npx tsc -b && pnpm lint && pnpm build`
Expected: all clean (`viewTransitionName` is a valid `CSSProperties` key — no `any`). Return to repo root: `cd ../..`

- [ ] **Step 5: Runtime check (the morph)**

Run (from repo root):
```bash
DINNERWISE_DB="$(mktemp -d)/dinnerwise.db" ADDR=:8085 go run ./cmd/server/ &
(cd web/app && VITE_API_URL=http://localhost:8085 pnpm dev --port 5182 &)
sleep 4
curl -s -o /dev/null -w "home %{http_code}\n" http://localhost:5182/
kill %1 %2 2>/dev/null; pkill -f vite 2>/dev/null
```
Expected: `home 200`. Controller's visual check (Chrome/Safari): from `/`, ask "what are my favorites" → the centered input **morphs/docks to the right**, the **sidebar slides in** from the left and the **dock from the right**, the left pane is the filtered Meals list; clicking **Home** reverses it back to the centered input; with OS "reduce motion" enabled the change is instant; on a browser without View Transitions the navigation still works (instant swap).

- [ ] **Step 6: Commit**

```bash
git add web/app/src/chat/ChatPanel.tsx web/app/src/chat/Sidebar.tsx web/app/src/index.css
git commit -m "feat: View-Transitions shell morph (input docks, chrome slides in)

Name the input pill (ask-input) so it FLIPs center->docked; slide the sidebar
and dock in via ::view-transition-* keyframes; honor prefers-reduced-motion.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage:**
- Route-driven layout (`/` hero, else split); Home returns to centered → Task 1 Steps 2–3.
- Router-native View Transitions (`defaultViewTransition: true`) → Task 1 Step 1.
- No-navigate-from-home fallback → `/recipes` → Task 1 Step 4.
- Input morph (`ask-input`), sidebar/dock slide-in → Task 2 Steps 1–3 (+ dock name in Task 1).
- `prefers-reduced-motion` instant + graceful degradation → Task 2 Step 3 (CSS) + router's built-in support.
- Orb stays a static pulse (unchanged) → not modified.
- Frontend-only; `go test` unaffected → Task 1 Step 6.

**Placeholder scan:** none — concrete code/commands throughout; the keyframe values are concrete (a reviewer/runtime eyeball may tune durations, which is expected for a visual slice).

**Type consistency:** `useRouterState({ select })` and `defaultViewTransition` are TanStack Router APIs (router is on v1.170.x). `viewTransitionName` is a standard `CSSProperties` key. `view-transition-name` values (`ask-input`, `sidebar`, `dock`) match between the `style` attributes (ChatPanel input, Sidebar aside, __root dock aside) and the `::view-transition-*` selectors in `index.css`. `router.navigate({ to: "/recipes" })` targets an existing typed route. Shell no longer imports `useChat`; index no longer imports `Link`.
