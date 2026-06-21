import { createRoute, getRouteApi } from "@tanstack/react-router";
import { useQuery } from "@connectrpc/connect-query";
import { getFoo } from "../gen/internal/foo/v1/foo-FooService_connectquery";
import { rootRoute } from "./__root";

const routeApi = getRouteApi("/foo");

// The `id` lives in the URL as typed, validated search state — not component
// state. This is the pattern that lets an agent both *read* page context
// (route + typed search) and *drive* it (navigate with a validated search
// patch).
function FooPage() {
  const { id } = routeApi.useSearch();
  const navigate = routeApi.useNavigate();

  const { data, error, isFetching } = useQuery(getFoo, { id });

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-semibold">FooService demo</h1>
        <p className="text-muted-foreground">
          The <code>id</code> is typed search state in the URL — try editing it.
        </p>
      </div>

      <input
        className="w-full rounded-lg border border-border bg-card px-3 py-2 text-card-foreground"
        value={id}
        onChange={(e) =>
          navigate({ search: { id: e.target.value }, replace: true })
        }
        placeholder="id"
      />

      {isFetching && <p className="text-muted-foreground">Loading…</p>}
      {error && <p className="text-red-600 dark:text-red-400">{error.message}</p>}
      {data && (
        <pre className="rounded-lg bg-muted p-3 text-sm text-foreground">
          {data.data?.foo}
        </pre>
      )}
    </div>
  );
}

export const fooRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/foo",
  validateSearch: (search: Record<string, unknown>): { id: string } => ({
    id: typeof search.id === "string" ? search.id : "123",
  }),
  component: FooPage,
});
