import { useEffect, useRef, useState, type FormEvent } from "react";
import { useRouter, type NavigateOptions } from "@tanstack/react-router";
import { useChat } from "./chatContext";
import type { Turn } from "./chatContext";
import { initials, thumbStyle, tintFor } from "../lib/thumb";

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

// Map a turn's meta events onto ordered "Working" step rows. The scripted
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
  const router = useRouter();

  // Keep the thread pinned to the latest content as the reply streams in.
  const scrollRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [turns]);

  function openRef(ref: { kind: string; id: string }) {
    const opts = {
      to: ref.kind === "meal" ? "/meals/$id" : "/recipes/$id",
      params: { id: ref.id },
    } as unknown as NavigateOptions;
    void router.navigate(opts);
  }

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
      // In the hero this input is the seed that grows into the full chat dock:
      // it shares "dock" with the docked <aside>, so the view transition morphs
      // the centered pill into the whole right panel (and back). In the dock it
      // carries no name — it's just content inside the already-named panel.
      style={{ viewTransitionName: hero ? "dock" : undefined }}
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
          <div className="font-semibold">Dinnerwise</div>
          <div className="mt-0.5 flex items-center gap-2">
            <span className={`h-1.5 w-1.5 rounded-full ${st.dot}`} />
            <span className="font-mono text-xs text-muted-foreground">
              {st.label}
            </span>
          </div>
        </div>
      </div>

      <div ref={scrollRef} className="flex-1 space-y-5 overflow-auto p-5">
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
                      <div key={i} className="flex items-start gap-2.5 text-sm">
                        {r.active ? (
                          <span className="spinner mt-1 h-3 w-3 flex-none rounded-full border-2 border-primary border-t-transparent" />
                        ) : (
                          <span className="flex-none text-primary">✓</span>
                        )}
                        <span
                          className={`min-w-0 break-words ${
                            r.active ? "text-foreground" : "text-muted-foreground"
                          }`}
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

              {t.assistant.references.length > 0 && (
                <div className="flex flex-col gap-2">
                  {t.assistant.references.map((ref) => (
                    <button
                      key={`${ref.kind}-${ref.id}`}
                      onClick={() => openRef(ref)}
                      className="flex items-center gap-3 rounded-xl border border-border bg-card/60 p-2.5 text-left transition-colors hover:border-primary/40"
                    >
                      <div
                        className="flex h-9 w-9 flex-none items-center justify-center rounded-lg font-mono text-xs font-semibold"
                        style={thumbStyle(tintFor(ref.id))}
                      >
                        {initials(ref.title)}
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="truncate text-sm font-medium">{ref.title}</div>
                        <div className="truncate font-mono text-xs text-muted-foreground">
                          {ref.subtitle}
                        </div>
                      </div>
                      <span className="flex-none font-mono text-muted-foreground">→</span>
                    </button>
                  ))}
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
