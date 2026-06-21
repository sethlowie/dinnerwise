import { createRootRoute, Link, Outlet } from "@tanstack/react-router";
import { ThemeToggle } from "../theme";
import { ChatProvider } from "../chat/ChatProvider";
import { ChatPanel } from "../chat/ChatPanel";
import { useChat } from "../chat/chatContext";

function Shell() {
  const { turns } = useChat();
  const active = turns.length > 0;

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <header className="border-b border-border">
        <div className="mx-auto flex w-full max-w-5xl items-center justify-between px-4 py-3">
          <nav className="flex items-center gap-4">
            <span className="font-semibold">dinnerwise</span>
            <Link to="/" className="text-muted-foreground [&.active]:text-foreground">
              Home
            </Link>
            <Link
              to="/recipes"
              className="text-muted-foreground [&.active]:text-foreground"
            >
              Recipes
            </Link>
          </nav>
          <ThemeToggle />
        </div>
      </header>

      {active ? (
        <div className="flex min-h-0 flex-1">
          <main className="flex-1 overflow-auto px-4 py-8">
            <div className="mx-auto max-w-3xl">
              <Outlet />
            </div>
          </main>
          <aside className="flex w-96 flex-col border-l border-border">
            <ChatPanel />
          </aside>
        </div>
      ) : (
        <main className="flex flex-1 items-center justify-center px-4">
          <ChatPanel hero />
        </main>
      )}
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
