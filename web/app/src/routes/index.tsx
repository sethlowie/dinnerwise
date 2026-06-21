import { createRoute, Link } from "@tanstack/react-router";
import { rootRoute } from "./__root";

function Home() {
  return (
    <div className="space-y-4">
      <div className="font-mono text-xs uppercase tracking-[0.12em] text-muted-foreground">
        Your kitchen
      </div>
      <h1 className="text-3xl font-semibold tracking-tight">Welcome back</h1>
      <p className="max-w-md text-muted-foreground">
        Ask Sous about your recipes and what to cook — or jump straight in.
      </p>
      <Link
        to="/recipes"
        className="inline-flex items-center gap-2 rounded-xl bg-primary px-4 py-2.5 text-sm font-medium text-primary-foreground hover:opacity-90"
      >
        Browse recipes →
      </Link>
    </div>
  );
}

export const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: Home,
});
