import { createRoute, getRouteApi, Link } from "@tanstack/react-router";
import { useQuery } from "@connectrpc/connect-query";
import { listMeals } from "../gen/internal/meal/v1/meal-MealService_connectquery";
import { rootRoute } from "./__root";
import { initials, thumbStyle, tintFor } from "../lib/thumb";

const routeApi = getRouteApi("/meals");

function Meals() {
  const { sort, fav } = routeApi.useSearch();
  const navigate = routeApi.useNavigate();
  const activeSort = sort ?? "recent";
  const { data, error, isPending } = useQuery(listMeals, {
    sort: activeSort,
    favoritesOnly: fav ?? false,
  });

  if (isPending) return <p className="text-muted-foreground">Loading…</p>;
  if (error)
    return <p className="text-red-600 dark:text-red-400">{error.message}</p>;

  return (
    <div className="space-y-6">
      <div>
        <div className="mb-2 font-mono text-xs uppercase tracking-[0.12em] text-muted-foreground">
          Your kitchen
        </div>
        <h1 className="text-3xl font-semibold tracking-tight">Meals</h1>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <button
          onClick={() =>
            navigate({
              search: (p) => ({ ...p, sort: activeSort === "recent" ? "rating" : "recent" }),
            })
          }
          className="rounded-xl border border-border bg-muted/40 px-3 py-1.5 text-sm hover:border-primary/40"
        >
          <span className="font-mono text-xs uppercase text-muted-foreground">sort </span>
          {activeSort === "rating" ? "Rating" : "Recent"}
        </button>
        <button
          onClick={() => navigate({ search: (p) => ({ ...p, fav: fav ? undefined : true }) })}
          className={`rounded-xl border px-3 py-1.5 text-sm ${
            fav
              ? "border-primary/40 bg-accent text-accent-foreground"
              : "border-border bg-muted/40 hover:border-primary/40"
          }`}
        >
          <span className="mr-1 text-amber-400">★</span>Favorites
        </button>
      </div>

      <div className="flex flex-col gap-2.5">
        {data.meals.map((m) => {
          const tint = tintFor(m.id);
          return (
            <Link
              key={m.id}
              to="/meals/$id"
              params={{ id: m.id }}
              className="flex items-center gap-3.5 rounded-2xl border border-border bg-card/60 p-4 transition-colors hover:border-primary/40"
            >
              <div
                className="flex h-11 w-11 flex-none items-center justify-center rounded-xl font-mono text-sm font-semibold"
                style={thumbStyle(tint)}
              >
                {initials(m.name)}
              </div>
              <div className="min-w-0 flex-1">
                <div className="truncate font-medium">{m.name}</div>
                <div className="mt-0.5 truncate font-mono text-xs text-muted-foreground">
                  {m.cuisine} · cooked {m.timesCooked}×
                </div>
              </div>
              <div className="flex-none text-right">
                <div className="text-sm tracking-widest">
                  <span className="text-amber-400">{"★".repeat(m.rating)}</span>
                  <span className="text-muted-foreground/40">{"★".repeat(5 - m.rating)}</span>
                </div>
                <div className="mt-1 font-mono text-xs text-muted-foreground">
                  {m.lastCooked || "—"}
                </div>
              </div>
            </Link>
          );
        })}
      </div>
    </div>
  );
}

export const mealsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/meals",
  validateSearch: (
    search: Record<string, unknown>,
  ): { sort?: "recent" | "rating"; fav?: boolean } => ({
    sort: search.sort === "rating" ? "rating" : search.sort === "recent" ? "recent" : undefined,
    fav: search.fav === true || search.fav === "true" || search.fav === "1" ? true : undefined,
  }),
  component: Meals,
});
