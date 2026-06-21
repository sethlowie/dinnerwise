import { createRoute } from "@tanstack/react-router";
import { rootRoute } from "./__root";
import { ChatPanel } from "../chat/ChatPanel";

// The centered hero for "/". Rendered as a route match so leaving/entering it
// swaps with the docked shell inside the view transition (the hero input grows
// into the full dock panel — both share view-transition-name: dock).
function Home() {
  return (
    <div className="relative flex h-screen items-center justify-center overflow-hidden bg-background text-foreground">
      <ChatPanel hero />
    </div>
  );
}

export const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: Home,
});
