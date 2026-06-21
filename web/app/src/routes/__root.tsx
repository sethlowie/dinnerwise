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
