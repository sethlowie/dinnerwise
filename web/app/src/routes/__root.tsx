import { createRootRoute, Outlet } from "@tanstack/react-router";
import { ChatProvider } from "../chat/ChatProvider";

// The root only owns the chat state (so it persists across home <-> app) and an
// Outlet. The hero ("/") and the docked shell (pathless "app" layout route)
// are rendered as route matches, which TanStack flushes inside the view
// transition — letting the hero input morph into the dock panel between them.
function RootLayout() {
  return (
    <ChatProvider>
      <Outlet />
    </ChatProvider>
  );
}

export const rootRoute = createRootRoute({ component: RootLayout });
