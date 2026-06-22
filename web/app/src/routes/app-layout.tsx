import { createRoute, Outlet } from "@tanstack/react-router";
import { rootRoute } from "./__root";
import { ChatPanel } from "../chat/ChatPanel";
import { Sidebar } from "../chat/Sidebar";

// The docked app shell: sidebar + page content + chat dock. Lives at the route
// level (a pathless layout route) rather than in the root component so the
// hero <-> shell swap is part of the match render TanStack flushes inside the
// view transition — that's what makes the enter/leave-app morph capture.
function AppLayout() {
  return (
    <div className="flex h-screen overflow-hidden bg-background text-foreground">
      <Sidebar />
      <main className="min-w-0 flex-1 overflow-auto">
        <div className="mx-auto max-w-3xl px-6 py-8">
          <Outlet />
        </div>
      </main>
      <aside
        style={{ viewTransitionName: "dock" }}
        className="flex h-screen w-[440px] flex-none flex-col border-l border-border bg-card/70 backdrop-blur"
      >
        <ChatPanel />
      </aside>
    </div>
  );
}

export const appLayoutRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: "app",
  component: AppLayout,
});
