# Sous Visual Redesign (Slice 1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restyle the existing app to the Sous look (theme, fonts, sidebar + agent-dock shell, home hero + orb, streaming dock, Recipes) using our real data and real agent stream — no backend changes.

**Architecture:** Retune the existing semantic-token CSS (dark = Sous palette, light = derived). Replace the top header with a Sous three-zone shell (sidebar / outlet / agent dock). Restyle `ChatPanel` into the Sous hero + dock, mapping our `thinking`/`tool_call`/`text` stream events onto Sous's Working-steps + streaming-reply + status treatment. Restyle Recipes with tinted initial "thumbs".

**Tech Stack:** React 19 + Vite, TanStack Router, Tailwind v4 (semantic tokens + `@theme`), Connect-Query (existing), Schibsted Grotesk + JetBrains Mono (Google Fonts).

## Global Constraints

- Frontend only — no Go/proto changes. `go test ./...` must remain green.
- Keep the light/dark toggle: **dark** = Sous palette, **light** = derived Sous-light. Same `--primary` `#7c6cf5` family + same fonts in both.
- No new runtime deps. Generated code stays gitignored.
- No frontend unit-test harness — verify each task with `npx tsc -b && pnpm lint && pnpm build` (run from `web/app`) plus a runtime check; not TDD.
- eslint: components only in `.tsx`; non-component modules in `.ts`. Route files are covered by the existing `src/routes/**` react-refresh override.
- Commit messages end with: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`

---

## File Structure

- Modify: `web/app/index.html` — Google Fonts link.
- Modify: `web/app/src/index.css` — Sous tokens (dark + light), `@theme` fonts, orb/caret/spin keyframes + helper classes.
- Create: `web/app/src/chat/Sidebar.tsx` — brand + nav (Home/Recipes) + theme toggle footer.
- Modify: `web/app/src/routes/__root.tsx` — Sous shell (sidebar / outlet / dock); remove top header.
- Modify: `web/app/src/chat/ChatPanel.tsx` — Sous hero (greeting, orb input, chips) + Sous dock (Working steps, streaming reply, status).
- Create: `web/app/src/lib/thumb.ts` — `tintFor`, `initials`, `thumbStyle` helpers.
- Modify: `web/app/src/routes/recipes.tsx` — Sous recipe cards + restyled filter chip.
- Modify: `web/app/src/routes/recipe.tsx` — Sous recipe detail.

---

## Task 1: Sous theme tokens, fonts, and keyframes

**Files:**
- Modify: `web/app/index.html`
- Modify: `web/app/src/index.css`

**Interfaces:**
- Consumes: nothing.
- Produces: the Sous palette on existing tokens (`bg-background`, `bg-card`, `text-foreground`, `bg-primary`, `border-border`, etc.); `font-sans` = Schibsted Grotesk, `font-mono` = JetBrains Mono; CSS helper classes `.orb`, `.brand-mark`, `.caret`, `.spinner`.

- [ ] **Step 1: Add the fonts to index.html**

In `web/app/index.html`, add inside `<head>` (after the viewport meta line):
```html
    <link rel="preconnect" href="https://fonts.googleapis.com" />
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
    <link
      href="https://fonts.googleapis.com/css2?family=Schibsted+Grotesk:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap"
      rel="stylesheet"
    />
```

- [ ] **Step 2: Rewrite index.css with the Sous tokens**

Replace the entire contents of `web/app/src/index.css` with:
```css
@import "tailwindcss";

@custom-variant dark (&:where(.dark, .dark *));

/* Semantic tokens. Dark = the Sous palette (as designed). Light = a derived
   Sous-light variant (same purple accent + fonts) so the toggle stays useful. */
:root {
  color-scheme: light;

  --background: #f6f6fa;
  --foreground: #1b1924;

  --card: #ffffff;
  --card-foreground: #1b1924;

  --muted: rgba(20, 18, 34, 0.05);
  --muted-foreground: rgba(27, 25, 36, 0.55);

  --primary: #6a57f0;
  --primary-foreground: #ffffff;

  --accent: rgba(124, 108, 245, 0.12);
  --accent-foreground: #2a2440;

  --border: rgba(20, 18, 34, 0.1);
  --ring: #6a57f0;

  --radius: 0.875rem;
}

.dark {
  color-scheme: dark;

  --background: #08080b;
  --foreground: #f4f4f7;

  --card: #13131a;
  --card-foreground: #f4f4f7;

  --muted: rgba(255, 255, 255, 0.06);
  --muted-foreground: rgba(244, 244, 247, 0.5);

  --primary: #7c6cf5;
  --primary-foreground: #ffffff;

  --accent: rgba(124, 108, 245, 0.14);
  --accent-foreground: #f4f4f7;

  --border: rgba(255, 255, 255, 0.07);
  --ring: #7c6cf5;

  --radius: 0.875rem;
}

@theme inline {
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-card: var(--card);
  --color-card-foreground: var(--card-foreground);
  --color-muted: var(--muted);
  --color-muted-foreground: var(--muted-foreground);
  --color-primary: var(--primary);
  --color-primary-foreground: var(--primary-foreground);
  --color-accent: var(--accent);
  --color-accent-foreground: var(--accent-foreground);
  --color-border: var(--border);
  --color-ring: var(--ring);

  --radius-lg: var(--radius);

  --font-sans: "Schibsted Grotesk", ui-sans-serif, system-ui, -apple-system,
    sans-serif;
  --font-mono: "JetBrains Mono", ui-monospace, monospace;
}

body {
  @apply bg-background text-foreground font-sans antialiased;
}

/* Sous motifs */
@keyframes orbPulse {
  0%, 100% {
    box-shadow: 0 0 0 0 rgba(124, 108, 245, 0.35), 0 0 28px 5px rgba(124, 108, 245, 0.4);
  }
  50% {
    box-shadow: 0 0 0 6px rgba(124, 108, 245, 0), 0 0 48px 12px rgba(124, 108, 245, 0.58);
  }
}
@keyframes blink {
  0%, 100% { opacity: 1; }
  50% { opacity: 0; }
}
@keyframes spin {
  to { transform: rotate(360deg); }
}

.orb {
  background: radial-gradient(circle at 35% 30%, #b3a6ff, #7c6cf5 62%, #5a48d6);
  animation: orbPulse 3.5s ease-in-out infinite;
}
.brand-mark {
  background: linear-gradient(135deg, #8e7dfb, #6a57f0);
  box-shadow: 0 4px 16px rgba(124, 108, 245, 0.45);
}
.caret {
  animation: blink 0.8s steps(1) infinite;
}
.spinner {
  animation: spin 0.8s linear infinite;
}
```

- [ ] **Step 3: Verify typecheck, lint, build**

Run (from `web/app`): `npx tsc -b && pnpm lint && pnpm build`
Expected: all clean. Return to repo root: `cd ../..`

- [ ] **Step 4: Commit**

```bash
git add web/app/index.html web/app/src/index.css
git commit -m "feat: Sous theme tokens, fonts, and motifs

Dark = Sous palette, light = derived variant; Schibsted Grotesk + JetBrains
Mono; orb/caret/spinner keyframes and helper classes.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Sous shell + sidebar

**Files:**
- Create: `web/app/src/chat/Sidebar.tsx`
- Modify: `web/app/src/routes/__root.tsx`

**Interfaces:**
- Consumes: `useChat` (existing), `ChatPanel` (existing), `ThemeToggle` from `../theme`.
- Produces: `Sidebar` component; a root layout that renders a centered hero when the thread is empty and a sidebar/outlet/dock split when active.

- [ ] **Step 1: Create the sidebar**

Create `web/app/src/chat/Sidebar.tsx`:
```tsx
import { Link } from "@tanstack/react-router";
import { ThemeToggle } from "../theme";

export function Sidebar() {
  return (
    <aside className="flex w-52 flex-none flex-col border-r border-border bg-card/60 p-5 backdrop-blur">
      <div className="mb-8 flex items-center gap-3">
        <div className="brand-mark h-7 w-7 rounded-[9px]" />
        <span className="text-base font-semibold tracking-tight">Sous</span>
      </div>

      <nav className="flex flex-col gap-1">
        <Link
          to="/"
          className="flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-muted-foreground hover:bg-muted [&.active]:bg-accent [&.active]:text-foreground"
        >
          <span className="h-1.5 w-1.5 rounded-full bg-current opacity-60" />
          Home
        </Link>
        <Link
          to="/recipes"
          className="flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-muted-foreground hover:bg-muted [&.active]:bg-accent [&.active]:text-foreground"
        >
          <span className="h-1.5 w-1.5 rounded-full bg-current opacity-60" />
          Recipes
        </Link>
      </nav>

      <div className="mt-auto">
        <ThemeToggle />
      </div>
    </aside>
  );
}
```

- [ ] **Step 2: Rewrite the root layout shell**

Replace the entire contents of `web/app/src/routes/__root.tsx` with:
```tsx
import { createRootRoute, Outlet } from "@tanstack/react-router";
import { ChatProvider } from "../chat/ChatProvider";
import { ChatPanel } from "../chat/ChatPanel";
import { Sidebar } from "../chat/Sidebar";
import { useChat } from "../chat/chatContext";

function Shell() {
  const { turns } = useChat();
  const active = turns.length > 0;

  if (!active) {
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
      <aside className="flex h-screen w-[360px] flex-none flex-col border-l border-border bg-card/70 backdrop-blur">
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

- [ ] **Step 3: Verify typecheck, lint, build**

Run (from `web/app`): `npx tsc -b && pnpm lint && pnpm build`
Expected: all clean (the old `ChatPanel` still renders inside the new shell). Return to repo root: `cd ../..`

- [ ] **Step 4: Commit**

```bash
git add web/app/src/chat/Sidebar.tsx web/app/src/routes/__root.tsx
git commit -m "feat: Sous shell with sidebar and agent dock

Centered hero when idle; sidebar + outlet + right dock when active. Replaces
the top header; theme toggle moves to the sidebar footer.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Sous home hero + agent dock

**Files:**
- Modify: `web/app/src/chat/ChatPanel.tsx`

**Interfaces:**
- Consumes: `useChat` (`turns`, `isStreaming`, `ask`); `Turn`/`AssistantMessage` from `chatContext` (fields: `userText`, `assistant.thinking[]`, `assistant.toolCalls[{name,detail}]`, `assistant.text`, `assistant.done`).
- Produces: `ChatPanel` with `hero` (home) and docked modes in the Sous style.

- [ ] **Step 1: Rewrite ChatPanel**

Replace the entire contents of `web/app/src/chat/ChatPanel.tsx` with:
```tsx
import { useState, type FormEvent } from "react";
import { useChat } from "./chatContext";
import type { Turn } from "./chatContext";

const CHIPS = [
  "What recipes have chicken?",
  "Show me tomato recipes",
  "What can I make with tofu?",
];

function greeting(): string {
  const h = new Date().getHours();
  const part = h < 12 ? "morning" : h < 18 ? "afternoon" : "evening";
  return `Good ${part}. What are we cooking?`;
}

type StepRow = { label: string; active: boolean };

// Map a turn's meta events onto ordered Sous "Working" step rows. The scripted
// backend emits thinking before tool_call before text, so rendering thinking
// rows then tool-call rows preserves arrival order. The latest row spins while
// the turn is still working and no reply text has arrived yet.
function stepRows(turn: Turn, streaming: boolean): StepRow[] {
  const a = turn.assistant;
  const rows: StepRow[] = [
    ...a.thinking.map((t) => ({ label: t, active: false })),
    ...a.toolCalls.map((tc) => ({ label: `${tc.name}(${tc.detail})`, active: false })),
  ];
  if (rows.length > 0 && streaming && a.text === "" && !a.done) {
    rows[rows.length - 1].active = true;
  }
  return rows;
}

function status(turns: Turn[], streaming: boolean): { label: string; dot: string } {
  const last = turns[turns.length - 1];
  if (!last) return { label: "Ready", dot: "bg-muted-foreground" };
  if (last.assistant.done) return { label: "Replied", dot: "bg-emerald-400" };
  if (last.assistant.text !== "") return { label: "Replying…", dot: "bg-primary" };
  if (streaming) return { label: "Working…", dot: "bg-amber-400" };
  return { label: "Ready", dot: "bg-muted-foreground" };
}

export function ChatPanel({ hero = false }: { hero?: boolean }) {
  const { turns, isStreaming, ask } = useChat();
  const [input, setInput] = useState("");

  function submit(text: string) {
    const t = text.trim();
    if (!t || isStreaming) return;
    setInput("");
    ask(t);
  }
  function onSubmit(e: FormEvent) {
    e.preventDefault();
    submit(input);
  }

  const inputPill = (
    <form
      onSubmit={onSubmit}
      className="flex items-center gap-2.5 rounded-2xl border border-border bg-muted/40 py-2 pl-5 pr-2 shadow-lg"
    >
      <input
        value={input}
        onChange={(e) => setInput(e.target.value)}
        placeholder="Ask anything about your kitchen…"
        className="min-w-0 flex-1 bg-transparent text-base outline-none placeholder:text-muted-foreground"
      />
      <button
        type="submit"
        disabled={isStreaming}
        aria-label="Ask"
        className="orb h-11 w-11 flex-none rounded-full disabled:opacity-60"
      />
    </form>
  );

  if (hero) {
    return (
      <div className="w-full max-w-xl px-6">
        <div className="mb-4 font-mono text-xs uppercase tracking-[0.14em] text-primary">
          Your kitchen copilot
        </div>
        <h1 className="mb-3 text-4xl font-semibold leading-tight tracking-tight">
          {greeting()}
        </h1>
        <p className="mb-8 max-w-md text-lg text-muted-foreground">
          Ask about your meals, your recipes, what to cook — I'll find it, open
          it, and walk you through.
        </p>
        {inputPill}
        <div className="mt-5 flex flex-wrap gap-2.5">
          {CHIPS.map((c) => (
            <button
              key={c}
              onClick={() => submit(c)}
              className="flex items-center gap-2 rounded-xl border border-border bg-muted/40 px-4 py-2.5 text-sm text-foreground/80 hover:border-primary/40 hover:text-foreground"
            >
              <span className="h-1.5 w-1.5 rounded-full bg-primary" />
              {c}
            </button>
          ))}
        </div>
      </div>
    );
  }

  const st = status(turns, isStreaming);

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center gap-3 border-b border-border px-5 py-4">
        <div className="orb h-9 w-9 flex-none rounded-full" />
        <div className="min-w-0 flex-1">
          <div className="font-semibold">Sous</div>
          <div className="mt-0.5 flex items-center gap-2">
            <span className={`h-1.5 w-1.5 rounded-full ${st.dot}`} />
            <span className="font-mono text-xs text-muted-foreground">
              {st.label}
            </span>
          </div>
        </div>
      </div>

      <div className="flex-1 space-y-5 overflow-auto p-5">
        {turns.map((t, ti) => {
          const rows = stepRows(t, isStreaming && ti === turns.length - 1);
          return (
            <div key={t.id} className="space-y-3">
              <div className="ml-auto w-fit max-w-[85%] rounded-2xl rounded-br-sm bg-primary px-3.5 py-2 text-sm text-primary-foreground shadow">
                {t.userText}
              </div>

              {rows.length > 0 && (
                <div className="rounded-2xl border border-border bg-muted/30 p-4">
                  <div className="mb-3 font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
                    Working
                  </div>
                  <div className="flex flex-col gap-2">
                    {rows.map((r, i) => (
                      <div key={i} className="flex items-center gap-2.5 text-sm">
                        {r.active ? (
                          <span className="spinner h-3 w-3 flex-none rounded-full border-2 border-primary border-t-transparent" />
                        ) : (
                          <span className="flex-none text-primary">✓</span>
                        )}
                        <span
                          className={
                            r.active ? "text-foreground" : "text-muted-foreground"
                          }
                        >
                          {r.label}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {t.assistant.text && (
                <div className="rounded-2xl rounded-tl-sm border border-primary/20 bg-primary/[0.06] px-4 py-3 text-sm leading-relaxed">
                  {t.assistant.text}
                  {!t.assistant.done && (
                    <span className="caret ml-0.5 text-primary">▋</span>
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>

      <div className="border-t border-border p-4">{inputPill}</div>
    </div>
  );
}
```

- [ ] **Step 2: Verify typecheck, lint, build**

Run (from `web/app`): `npx tsc -b && pnpm lint && pnpm build`
Expected: all clean. Return to repo root: `cd ../..`

- [ ] **Step 3: Commit**

```bash
git add web/app/src/chat/ChatPanel.tsx
git commit -m "feat: Sous home hero and agent dock

Hero with greeting, orb input, and suggestion chips; dock maps thinking/
tool_call events to Working steps, streams the reply with a caret, and shows
a status label.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Sous recipes list + detail (+ thumb helper)

**Files:**
- Create: `web/app/src/lib/thumb.ts`
- Modify: `web/app/src/routes/recipes.tsx`
- Modify: `web/app/src/routes/recipe.tsx`

**Interfaces:**
- Consumes: `listRecipes`/`getRecipe` hooks (existing); `getRouteApi`, `validateSearch` `{ ingredient?: string }` on `recipesRoute` (existing).
- Produces: `tintFor(seed)`, `initials(name)`, `thumbStyle(tint)` helpers; Sous-styled recipe list + detail.

- [ ] **Step 1: Create the thumb helper**

Create `web/app/src/lib/thumb.ts`:
```ts
import type { CSSProperties } from "react";

const TINTS = ["#7c6cf5", "#5eb888", "#e0775e", "#e0a14e", "#d9c24e", "#9a8cf0"];

// Deterministic tint from a stable seed (e.g. recipe id) so a given item always
// gets the same color.
export function tintFor(seed: string): string {
  let h = 0;
  for (let i = 0; i < seed.length; i++) {
    h = (h * 31 + seed.charCodeAt(i)) >>> 0;
  }
  return TINTS[h % TINTS.length];
}

// Up to two initials from a name (e.g. "Tomato Pasta" -> "TP").
export function initials(name: string): string {
  const w = name.split(/\s+/).filter(Boolean);
  const a = w[0]?.[0] ?? "";
  const b = w[1]?.[0] ?? w[0]?.[1] ?? "";
  return (a + b).toUpperCase();
}

// Inline style for a tinted "thumb" box (gradient fill + matching border/text).
export function thumbStyle(tint: string): CSSProperties {
  return {
    background: `linear-gradient(140deg, ${tint}33, ${tint}14)`,
    border: `1px solid ${tint}40`,
    color: tint,
  };
}
```

- [ ] **Step 2: Rewrite the recipes list**

Replace the entire contents of `web/app/src/routes/recipes.tsx` with:
```tsx
import { createRoute, getRouteApi, Link } from "@tanstack/react-router";
import { useQuery } from "@connectrpc/connect-query";
import { listRecipes } from "../gen/internal/recipe/v1/recipe-RecipeService_connectquery";
import { rootRoute } from "./__root";
import { initials, thumbStyle, tintFor } from "../lib/thumb";

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
    <div className="space-y-6">
      <div>
        <div className="mb-2 font-mono text-xs uppercase tracking-[0.12em] text-muted-foreground">
          Your kitchen
        </div>
        <div className="flex items-center gap-3">
          <h1 className="text-3xl font-semibold tracking-tight">Recipes</h1>
          {ingredient && (
            <button
              onClick={() => navigate({ search: {} })}
              className="rounded-full border border-primary/40 bg-accent px-3 py-1 text-xs text-accent-foreground"
            >
              ingredient: {ingredient} ✕
            </button>
          )}
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        {recipes.map((r) => {
          const tint = tintFor(r.id);
          return (
            <Link
              key={r.id}
              to="/recipes/$id"
              params={{ id: r.id }}
              className="flex items-center gap-3.5 rounded-2xl border border-border bg-card/60 p-4 transition-colors hover:border-primary/40"
            >
              <div
                className="flex h-12 w-12 flex-none items-center justify-center rounded-xl font-mono text-sm font-semibold"
                style={thumbStyle(tint)}
              >
                {initials(r.name)}
              </div>
              <div className="min-w-0">
                <div className="truncate font-medium">{r.name}</div>
                <div className="mt-0.5 truncate font-mono text-xs text-muted-foreground">
                  ⏱ {r.totalMinutes} min · serves {r.servings}
                </div>
              </div>
            </Link>
          );
        })}
      </div>
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

- [ ] **Step 3: Rewrite the recipe detail**

Replace the entire contents of `web/app/src/routes/recipe.tsx` with:
```tsx
import { createRoute, getRouteApi, Link } from "@tanstack/react-router";
import { useQuery } from "@connectrpc/connect-query";
import { getRecipe } from "../gen/internal/recipe/v1/recipe-RecipeService_connectquery";
import { rootRoute } from "./__root";
import { initials, thumbStyle, tintFor } from "../lib/thumb";

const routeApi = getRouteApi("/recipes/$id");

function RecipeDetail() {
  const { id } = routeApi.useParams();
  const { data, error, isPending } = useQuery(getRecipe, { id });

  if (isPending) return <p className="text-muted-foreground">Loading…</p>;
  if (error)
    return <p className="text-red-600 dark:text-red-400">{error.message}</p>;

  const recipe = data.recipe;
  if (!recipe) return <p className="text-muted-foreground">Not found.</p>;

  const tint = tintFor(recipe.id);
  const methodLines = recipe.instructions
    .split(/\n+/)
    .map((l) => l.trim())
    .filter(Boolean);

  return (
    <article className="space-y-7">
      <Link
        to="/recipes"
        className="font-mono text-sm text-muted-foreground hover:text-foreground"
      >
        ← Recipes
      </Link>

      <div className="flex items-start gap-5">
        <div
          className="flex h-16 w-16 flex-none items-center justify-center rounded-2xl font-mono text-lg font-semibold"
          style={thumbStyle(tint)}
        >
          {initials(recipe.name)}
        </div>
        <div>
          <h1 className="text-3xl font-semibold tracking-tight">{recipe.name}</h1>
          <p className="mt-2 font-mono text-sm text-muted-foreground">
            ⏱ {recipe.totalMinutes} min · serves {recipe.servings}
          </p>
        </div>
      </div>

      <section>
        <div className="mb-3 font-mono text-xs uppercase tracking-[0.1em] text-muted-foreground">
          Ingredients
        </div>
        <ul className="flex flex-col gap-2">
          {recipe.ingredients.map((ing) => (
            <li
              key={ing.ingredientId}
              className="flex items-baseline gap-3 text-sm text-foreground/85"
            >
              <span className="mt-1.5 h-1.5 w-1.5 flex-none rounded-full bg-primary" />
              {ing.quantity} {ing.unit} {ing.name}
            </li>
          ))}
        </ul>
      </section>

      <section>
        <div className="mb-3 font-mono text-xs uppercase tracking-[0.1em] text-muted-foreground">
          Method
        </div>
        <div className="flex flex-col gap-3 text-sm leading-relaxed text-foreground/85">
          {methodLines.map((line, i) => (
            <p key={i}>{line}</p>
          ))}
        </div>
      </section>
    </article>
  );
}

export const recipeDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/recipes/$id",
  component: RecipeDetail,
});
```

- [ ] **Step 4: Verify typecheck, lint, build**

Run (from `web/app`): `npx tsc -b && pnpm lint && pnpm build`
Expected: all clean. Return to repo root: `cd ../..`

- [ ] **Step 5: Verify backend still green + runtime check**

Run (from repo root):
```bash
go test ./... 2>&1 | grep -E "ok|FAIL"
DINNERWISE_DB="$(mktemp -d)/dinnerwise.db" ADDR=:8092 go run ./cmd/server/ &
(cd web/app && VITE_API_URL=http://localhost:8092 pnpm dev --port 5177 &)
sleep 4
curl -s -o /dev/null -w "home %{http_code}\n" http://localhost:5177/
curl -s -o /dev/null -w "recipes %{http_code}\n" "http://localhost:5177/recipes?ingredient=chicken"
kill %1 %2 2>/dev/null; pkill -f vite 2>/dev/null
```
Expected: backend `ok` lines (unchanged), `home 200`, `recipes 200`. Controller's manual visual check: dark Sous look renders; toggle flips to the light variant; from the home hero, asking "what recipes have chicken" docks the panel with Working steps + streaming reply and navigates the outlet to a filtered, Sous-styled `/recipes`.

- [ ] **Step 6: Commit**

```bash
git add web/app/src/lib/thumb.ts web/app/src/routes/recipes.tsx web/app/src/routes/recipe.tsx
git commit -m "feat: Sous recipe list and detail

Tinted initial thumbnails, mono meta lines, Sous detail layout (bulleted
ingredients + Method block). Adds a deterministic thumb helper.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Self-Review

**Spec coverage:**
- Theme dark=Sous / light=derived + fonts + motifs → Task 1.
- Shell: chrome-free hero when empty; sidebar + outlet + dock when active; header removed; toggle in sidebar → Task 2.
- Sidebar nav Home/Recipes (Meals later) → Task 2.
- Home hero: greeting, kicker, orb input, suggestion chips matching the scripted agent → Task 3.
- Dock: Working steps from thinking/tool_call, streaming reply + caret, status label; navigate unchanged; multi-turn → Task 3.
- Recipes list (tinted thumbs, mono meta, filter chip) + detail (ingredients + Method from instructions) → Task 4.
- thumb helper (initials + deterministic tint) → Task 4.
- No backend changes; go test stays green; frontend verified via tsc/eslint/build + runtime → all tasks + Task 4 Step 5.
- Static orb (no travel), ref cards/replay omitted, Meals/metadata deferred → respected (not implemented).

**Placeholder scan:** none — every step has complete file contents or exact commands.

**Type consistency:** `useChat`/`Turn`/`AssistantMessage` field names (`thinking`, `toolCalls{name,detail}`, `text`, `done`, `userText`) match `chatContext`/`ChatProvider`. `tintFor`/`initials`/`thumbStyle` signatures match between `thumb.ts` and both recipe routes. `ChatPanel` `hero` prop and `Sidebar`/`ChatProvider`/`ChatPanel` imports align with `__root.tsx`. `recipesRoute` `ingredient` search param unchanged. `to="/"` / `to="/recipes"` / `to="/recipes/$id"` are existing typed routes.
