# Sous Visual Redesign (Slice 1 of 5) — Design

**Date:** 2026-06-21
**Status:** Approved (design)
**Design source:** claude.ai/design project "Recipe AI Agent Demo" → `Sous.dc.html`

## The full Sous arc (planned, built slice by slice)

The Sous design is too large for one spec. It decomposes into 5 slices, each its
own spec → plan → build cycle:

1. **Visual redesign** (this spec) — adopt the Sous look on existing surfaces
   using real data + the real agent stream. No backend changes.
2. **Meals domain** — Go schema/repo/service + proto for logged meals (rating,
   times-cooked, history); Meals list + Meal detail screens; sidebar gains Meals.
3. **Recipe metadata + pantry** — cuisine, difficulty, "in pantry" badge; the
   data + UI the "tonight"/"quick" scenarios rely on.
4. **Agent reference cards + scenarios** — extend the agent protocol to emit
   reference items; render result cards in the dock + a "replay this run" action.
5. **Traveling-orb animation** — the rAF orb that flies from the home input into
   the dock, sidebar/dock slide choreography polish.

This document specifies **slice 1 only**. Later slices get their own specs.

## Goal (slice 1)

Make the app look and feel like Sous — theme, fonts, the sidebar + agent-dock
shell, the home hero, the streaming dock, and Recipes — driven by our existing
`RecipeService` and `AgentService` stream. No backend changes.

## Decisions (locked)

- **Theme:** keep the light/dark toggle. **Dark** = the Sous palette (as
  designed). **Light** = a coherent derived Sous-light variant (invented, same
  accent/fonts). Both via the existing semantic-token system in `index.css`.
- **Shell:** chrome-free centered hero when the chat thread is empty; left
  sidebar + outlet + right agent dock when active. Replaces the top header.
- **Sidebar nav now:** Home, Recipes. (Meals added in slice 2.)
- **Orb:** static pulsing orb in the home input and dock header. The traveling
  animation is slice 5.
- **Dock mapping:** our real `thinking`/`tool_call` events → Sous "Working"
  steps; `text` → streaming reply with caret; streaming state → status label.
  Reference cards and "replay" are slice 4 (omitted now).
- **Recipe detail:** render `instructions` as a "Method" block. Numbered
  structured steps come with the recipe-metadata slice.
- No backend changes in this slice.

## Theme & fonts (`web/app/src/index.css`, `web/app/index.html`)

Retune the existing token system (`:root` light, `.dark` dark, `@theme inline`):

Dark (Sous, as designed):
- `--background: #08080b` (oklch/hex ok); `--foreground: #f4f4f7`
- `--card: rgba(19,19,26,0.92)` surface; `--muted` translucent white
  (`rgba(244,244,247,0.06)`); `--muted-foreground: rgba(244,244,247,0.5)`
- `--primary: #7c6cf5`; `--primary-foreground: #ffffff`
- `--accent` / accent tints for thumbnails: purple `#7c6cf5`, green `#5eb888`,
  warm `#e0775e` / `#e0a14e`, gold star `#e9b949` (used as utility values, not
  all semantic tokens)
- `--border: rgba(255,255,255,0.07)`; `--ring: #7c6cf5`

Light (derived Sous-light): light neutral bg, dark foreground, the **same**
`--primary` `#7c6cf5`, borders as subtle dark-on-light. Chosen for coherence;
not part of the original Sous design.

Fonts: add to `index.html` `<head>` the Google Fonts link for
`Schibsted Grotesk` (400–700) + `JetBrains Mono` (400,500). In `@theme`:
`--font-sans: "Schibsted Grotesk", ...` and `--font-mono: "JetBrains Mono", ...`
so `font-sans`/`font-mono` utilities resolve to them. Body uses sans; mono is
applied to kicker/eyebrow labels and meta text.

## Shell / layout (`web/app/src/routes/__root.tsx`)

`Shell` reads `useChat()` (existing). Three zones:
- **Empty thread:** centered hero only (no sidebar/dock), on the Sous background
  (subtle radial gradient blobs are a nice-to-have, optional).
- **Active (thread non-empty):** a left **`Sidebar`** (fixed, ~198px) + the main
  `<Outlet/>` (center) + the right **`ChatPanel`** dock (~332px).
- The current top `<header>` (nav + ThemeToggle) is removed; nav moves into the
  sidebar, and the `ThemeToggle` is placed in the sidebar footer.

New component `web/app/src/chat/Sidebar.tsx` (or `routes`-local): brand mark
("Sous"), nav links (Home → `/`, Recipes → `/recipes`) with active styling via
TanStack Router's `[&.active]`, and a footer with the ThemeToggle. Built so a
`Meals` link drops in later.

## Home hero (`web/app/src/chat/ChatPanel.tsx` hero mode, or a `Hero` component)

- Time-based greeting ("Good morning/afternoon/evening. What are we cooking?").
- Mono kicker ("Your kitchen copilot").
- Sous input pill: text input + a circular **orb** button (static pulse glow via
  a CSS keyframe; reuse/extend `index.css` animations). Submitting calls the
  existing `ask()`.
- Suggestion chips (3–4) that call `ask()` with prompts the scripted agent
  matches, e.g. "What recipes have chicken?", "Show me tomato recipes",
  "What can I make with tofu?".

## Agent dock (`web/app/src/chat/ChatPanel.tsx` docked mode)

Restyle the existing thread rendering to the Sous dock:
- Header: orb + "Sous" + a status row (`statusDot` + label).
- Per turn: a right-aligned **user bubble**; a **"Working" box** listing each
  `thinking`/`tool_call` event as a step row — the latest shows a spinner while
  `isStreaming`, earlier ones show a check; the **reply** bubble streams `text`
  with a blinking caret that hides when the turn's `done` is set.
- Status label derived from streaming/turn state: `Working…` (meta arriving,
  no text yet) → `Replying…` (text streaming) → `Replied` (done) / `Ready`.
- Existing `navigate` handling unchanged (drives the outlet).
- No reference cards / replay (slice 4). Keep multi-turn rendering.

The `AssistantMessage` shape already carries `thinking[]`, `toolCalls[]`, `text`,
`done` — sufficient. A small helper maps it to ordered step rows + status.

## Recipes (`web/app/src/routes/recipes.tsx`, `recipe.tsx`)

- **List:** Sous cards — a square "thumb" showing the recipe's initials on a
  deterministic per-recipe tint (hash the id into a small tint palette), the
  name, and a mono meta line (`⏱ {totalMinutes} min · serves {servings}`).
  Keep the existing `ingredient` filter + clearable chip, restyled.
- **Detail:** Sous layout — back link, big tinted thumb + title + mono meta,
  **Ingredients** as a bulleted list (accent dots), **Method** rendering
  `instructions` as a block (split on newlines into lines if present; numbered
  structured steps are deferred to the metadata slice).
- A shared `thumb(id|name)` helper computes initials + tint (used by list,
  detail, and later the dock/meals).

## Components / files

- Modify: `web/app/index.html` (fonts), `web/app/src/index.css` (tokens, fonts, orb keyframes).
- Modify: `web/app/src/routes/__root.tsx` (Sous shell: sidebar + outlet + dock).
- Create: `web/app/src/chat/Sidebar.tsx` (nav + brand + theme toggle footer).
- Modify: `web/app/src/chat/ChatPanel.tsx` (Sous hero + Sous dock; greeting, orb, chips, Working steps, streaming reply, status).
- Create: `web/app/src/lib/thumb.ts` (initials + deterministic tint helper).
- Modify: `web/app/src/routes/recipes.tsx`, `web/app/src/routes/recipe.tsx` (Sous cards + detail).
- Possibly modify: `web/app/src/routes/index.tsx` (home content shown in the outlet when navigating Home while active — restyle or simplify).

## Testing

- No backend changes → `go test ./...` unaffected (run it to confirm nothing broke).
- Frontend: `tsc -b`, `eslint`, `vite build` all clean.
- Runtime check: the app renders in the Sous look (dark by default), the toggle
  flips to the light variant, and from the home hero, asking "what recipes have
  chicken" streams Working steps + reply in the dock and navigates the outlet to
  a filtered `/recipes`.

## Tradeoffs & notes

- The light Sous variant is invented (Sous is dark-only); it exists only to keep
  the toggle meaningful.
- Mapping free-form `thinking`/`tool_call` events onto Sous's discrete
  "Working" steps is approximate — there's no per-step "done" signal, so we
  treat each event as a step and spin the latest. Good enough; the real agent
  (later) can emit explicit step states if we want.
- Rendering `instructions` as a Method block is a fidelity gap vs Sous's numbered
  steps; closed by the recipe-metadata slice.
- Deterministic tint from id hash means thumbnails are stable but not curated.

## Out of scope (this slice)

Meals domain; recipe cuisine/difficulty/pantry; agent reference cards, scenarios,
replay; traveling-orb physics. All are later slices in the arc above.
