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
