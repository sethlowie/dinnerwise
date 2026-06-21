import { createRoute } from "@tanstack/react-router";
import { rootRoute } from "./__root";

// The centered hero for "/" is rendered by the root Shell, so this route's
// outlet content is unused — the route only needs to exist so "/" matches.
function Home() {
  return null;
}

export const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: Home,
});
