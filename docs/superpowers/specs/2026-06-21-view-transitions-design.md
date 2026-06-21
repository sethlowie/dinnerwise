# View-Transitions Shell Morph + Route-Driven Home (Slice 5 of 5) — Design

**Date:** 2026-06-21
**Status:** Approved (design)
**Arc:** Sous design — final slice (1 visual ✓ · 2 Meals ✓ · 3 metadata/pantry ✓ · 4 agent scenarios ✓ · **5 shell-morph polish**)

## Goal

Replace the planned bespoke "traveling orb" with a coherent **View-Transitions
shell morph**: the centered home input grows and docks to the side, the sidebar
grows in, the dock slides in — driven by route changes. Also make the home /
centered view **route-driven** so clicking Home reliably returns to the clean
centered input.

## Why this over a traveling orb

The mockup hand-rolled rAF orb physics only because its host re-rendered too fast
for CSS transitions; our real app has no such constraint. The whole shell
reconfiguring is a bigger, more coherent payoff than one element flying across,
and making the layout route-driven lets us use TanStack Router's **native** view
transitions instead of manual `startViewTransition`/`flushSync`.

## Decisions (locked)

- **Layout is route-driven:** on `/` → centered hero (no sidebar/dock); every
  other route → split (sidebar + Outlet + dock). Replaces the `turns.length > 0`
  check. The chat thread persists in `ChatProvider` (just not shown on home).
- **View Transitions via the router:** `createRouter({ defaultViewTransition:
  true })` wraps every navigation in `document.startViewTransition` (graceful
  degradation built in). CSS does the morph.
- **The input is the shared morphing element** (`view-transition-name:
  ask-input`); the orb stays a static pulse riding inside it.
- **No-navigate-from-home fallback:** an ask from `/` that finishes without the
  agent navigating falls back to `/recipes` (frontend-only) so a reply is never
  stranded on the dock-less home.
- Respect `prefers-reduced-motion`.

## Route-driven layout (`web/app/src/routes/__root.tsx`)

`Shell` reads the current pathname (`useRouterState({ select: (s) =>
s.location.pathname })`). `const isHome = pathname === "/"`:
- **isHome:** render only the centered hero (`<ChatPanel hero />`) on the Sous
  background; no sidebar, no dock, Outlet not rendered.
- **else:** `<Sidebar/>` + `<main><Outlet/></main>` + `<ChatPanel/>` (dock).

`ChatProvider` still wraps `Shell` (state persists across navigation).

`web/app/src/routes/index.tsx`: the `/` route component becomes a stub
(`function Home() { return null; }`) — the Shell renders the hero for `/`, so the
index outlet content is no longer used. (The route must still exist so `/`
matches.)

## View Transitions (`router.tsx` + `index.css`)

- `web/app/src/router.tsx`: `createRouter({ routeTree, defaultViewTransition:
  true })`. Every navigation (Home link, agent `router.navigate`, reference-card
  nav, the home-ask fallback) is then wrapped in a view transition automatically;
  on unsupported browsers the navigation just applies instantly.
- `web/app/src/chat/ChatPanel.tsx`: add `style={{ viewTransitionName:
  "ask-input" }}` to the input pill wrapper. It renders in both hero and docked
  modes but only one `ChatPanel` is mounted at a time (Shell branches), so a
  single `ask-input` element exists per state — the browser FLIPs it from the
  centered position to the docked position.
- `web/app/src/index.css`:
  - Keyframes for the sidebar and dock entering, wired via
    `::view-transition-new(root)` / a named group, e.g. a subtle
    fade+slide so the chrome "grows in" rather than hard-cutting.
  - `@media (prefers-reduced-motion: reduce) { ::view-transition-group(*),
    ::view-transition-old(*), ::view-transition-new(*) { animation: none !important; } }`
    so reduced-motion users get an instant swap.
  - Default cross-fade (the browser's built-in) covers the hero's
    greeting/chips fading out and the dock thread fading in.

(Exact `view-transition-name` set and keyframes are tuned during
implementation against the running app; the input morph + chrome fade is the
baseline, with the reduced-motion guard required.)

## No-navigate-from-home fallback (`web/app/src/chat/ChatProvider.tsx`)

In `ask(text)`: capture whether the call started on the home route
(`window.location.pathname === "/"`), and track a `navigated` flag set when a
`navigate` event is processed during the turn. On `done`, if `!navigated` and the
ask started on home, `router.navigate({ to: "/recipes" })` so the reply + dock
become visible. (Turns started from within the app — already in the split — need
no fallback.)

## The orb

Unchanged from slice 1 — a static CSS pulse (`.orb`). It sits inside the input
pill, so it travels with the input during the `ask-input` morph for free. No
rAF/FLIP orb code.

## Testing

View Transitions and route-driven layout are visual/runtime behaviors with no
unit-test harness:
- `tsc -b`, `eslint`, `vite build` clean.
- Runtime eyeball: from `/`, ask "what are my favorites" → the centered input
  **morphs/docks to the right**, the sidebar grows in, the dock streams the
  reply, the left pane is the filtered Meals list; clicking **Home** returns to
  the centered input; a "none" ask from home lands on `/recipes` with the reply;
  with OS reduced-motion on, transitions are instant; on a browser without View
  Transitions (or the check failing) navigation still works (instant).
- `go test ./...` unaffected (frontend-only) — run to confirm nothing broke.

## Tradeoffs & notes

- Firefox SPA view-transition support is still partial; acceptable because it
  degrades to an instant (functional) swap. Chrome/Safari get the full morph.
- React 19's experimental `<ViewTransition>` is not used; the router-level
  `defaultViewTransition` is the stable path today.
- The home hero no longer shows the conversation; the conversation lives in the
  dock on content routes. Home is a clean launcher; the thread is preserved.
- `view-transition-name` must be unique per snapshot — only one `ChatPanel`
  (hence one `ask-input`) is mounted at a time, so this holds.

## Out of scope

Bespoke orb travel (replaced); any agent/behavioral change (the fallback is
frontend routing only); broader animation of cards/lists beyond the
shell/route morph.
