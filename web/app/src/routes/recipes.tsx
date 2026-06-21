import { createRoute, getRouteApi, Link } from "@tanstack/react-router";
import { useQuery } from "@connectrpc/connect-query";
import { listRecipes } from "../gen/internal/recipe/v1/recipe-RecipeService_connectquery";
import { rootRoute } from "./__root";
import { initials, thumbStyle, tintFor } from "../lib/thumb";

const routeApi = getRouteApi("/recipes");

function Recipes() {
  const { ingredient } = routeApi.useSearch();
  const navigate = routeApi.useNavigate();
  const { data, error, isPending } = useQuery(listRecipes, {});

  if (isPending) return <p className="text-muted-foreground">Loading…</p>;
  if (error)
    return <p className="text-red-600 dark:text-red-400">{error.message}</p>;

  const term = ingredient?.toLowerCase().trim() ?? "";
  const recipes = term
    ? data.recipes.filter((r) =>
        r.ingredients.some((i) => i.name.toLowerCase().includes(term)),
      )
    : data.recipes;

  return (
    <div className="space-y-6">
      <div>
        <div className="mb-2 font-mono text-xs uppercase tracking-[0.12em] text-muted-foreground">
          Your kitchen
        </div>
        <div className="flex items-center gap-3">
          <h1 className="text-3xl font-semibold tracking-tight">Recipes</h1>
          {ingredient && (
            <button
              onClick={() => navigate({ search: {} })}
              className="rounded-full border border-primary/40 bg-accent px-3 py-1 text-xs text-accent-foreground"
            >
              ingredient: {ingredient} ✕
            </button>
          )}
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        {recipes.map((r) => {
          const tint = tintFor(r.id);
          return (
            <Link
              key={r.id}
              to="/recipes/$id"
              params={{ id: r.id }}
              className="flex items-center gap-3.5 rounded-2xl border border-border bg-card/60 p-4 transition-colors hover:border-primary/40"
            >
              <div
                className="flex h-12 w-12 flex-none items-center justify-center rounded-xl font-mono text-sm font-semibold"
                style={thumbStyle(tint)}
              >
                {initials(r.name)}
              </div>
              <div className="min-w-0">
                <div className="truncate font-medium">{r.name}</div>
                <div className="mt-0.5 truncate font-mono text-xs text-muted-foreground">
                  ⏱ {r.totalMinutes} min · serves {r.servings}
                </div>
              </div>
            </Link>
          );
        })}
      </div>
    </div>
  );
}

export const recipesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/recipes",
  validateSearch: (search: Record<string, unknown>): { ingredient?: string } => ({
    ingredient: typeof search.ingredient === "string" ? search.ingredient : undefined,
  }),
  component: Recipes,
});
