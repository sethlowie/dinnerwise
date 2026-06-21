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
              const opts = {
                to: event.value.to,
                search: event.value.search,
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
