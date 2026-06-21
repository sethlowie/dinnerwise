import { createRoute, Link } from "@tanstack/react-router";
import { rootRoute } from "./__root";

function Home() {
  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-semibold">dinnerwise</h1>
      <p className="text-muted-foreground">
        React → Connect → Go scaffold with TanStack Router and Query, themed
        with semantic Tailwind tokens that respond to light/dark.
      </p>
      <Link
        to="/foo"
        search={{ id: "123" }}
        className="inline-block rounded-lg bg-primary px-4 py-2 text-primary-foreground hover:opacity-90"
      >
        Try the Foo demo →
      </Link>
    </div>
  );
}

export const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: Home,
});
