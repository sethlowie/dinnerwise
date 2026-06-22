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
  references: [],
};

// The agent navigates by path string. List views ("/recipes", "/meals") carry
// search filters; a concrete detail path ("/recipes/<id>", "/meals/<id>") is
// turned into param navigation so it matches the typed $id routes.
function toNavigateOptions(
  to: string,
  search: Record<string, string>,
): NavigateOptions {
  const recipe = /^\/recipes\/(.+)$/.exec(to);
  if (recipe) {
    return { to: "/recipes/$id", params: { id: recipe[1] } } as unknown as NavigateOptions;
  }
  const meal = /^\/meals\/(.+)$/.exec(to);
  if (meal) {
    return { to: "/meals/$id", params: { id: meal[1] } } as unknown as NavigateOptions;
  }
  return { to, search } as unknown as NavigateOptions;
}

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

    // Submitting from home: jump into the app immediately so the chat dock and
    // the agent's "thinking" are visible right away (the hero input morphs into
    // the dock) instead of waiting for the agent's first navigate event. The
    // agent's own navigate event refines the destination as the turn streams.
    if (window.location.pathname === "/") {
      void router.navigate({ to: "/recipes" });
    }

    const update = (fn: (a: AssistantMessage) => AssistantMessage) =>
      setTurns((prev) =>
        prev.map((t) => (t.id === id ? { ...t, assistant: fn(t.assistant) } : t)),
      );

    // Prior turns (with a spoken reply) become the conversation history we send;
    // `turns` here is the pre-append snapshot, so it excludes the new turn.
    const history = turns
      .filter((t) => t.assistant.text.trim() !== "")
      .map((t) => ({ userText: t.userText, assistantText: t.assistant.text }));

    void (async () => {
      try {
        for await (const ev of agentClient.ask({ text, history })) {
          const event = ev.event;
          switch (event.case) {
            case "thinking":
              update((a) => ({ ...a, thinking: [...a.thinking, event.value.text] }));
              break;
            case "toolCall":
              update((a) => ({
                ...a,
                toolCalls: [
                  ...a.toolCalls,
                  { name: event.value.name, detail: event.value.detail },
                ],
              }));
              break;
            case "text":
              update((a) => ({ ...a, text: a.text + event.value.text }));
              break;
            case "reference":
              update((a) => ({
                ...a,
                references: [
                  ...a.references,
                  {
                    kind: event.value.kind,
                    id: event.value.id,
                    title: event.value.title,
                    subtitle: event.value.subtitle,
                  },
                ],
              }));
              break;
            case "navigate": {
              void router.navigate(
                toNavigateOptions(event.value.to, event.value.search),
              );
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
