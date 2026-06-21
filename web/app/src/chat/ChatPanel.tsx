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
