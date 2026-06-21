import { createRoute, getRouteApi, Link } from "@tanstack/react-router";
import { useQuery } from "@connectrpc/connect-query";
import { listRecipes } from "../gen/internal/recipe/v1/recipe-RecipeService_connectquery";
import { rootRoute } from "./__root";
import { initials, thumbStyle, tintFor } from "../lib/thumb";

const routeApi = getRouteApi("/recipes");

type RecipeSearch = {
  ingredient?: string;
  pantry?: boolean;
  maxMinutes?: number;
};

function Recipes() {
  const { ingredient, pantry, maxMinutes } = routeApi.useSearch();
  const navigate = routeApi.useNavigate();
  const { data, error, isPending } = useQuery(listRecipes, {});

  if (isPending) return <p className="text-muted-foreground">Loading…</p>;
  if (error)
    return <p className="text-red-600 dark:text-red-400">{error.message}</p>;

  const term = ingredient?.toLowerCase().trim() ?? "";
  const recipes = data.recipes.filter((r) => {
    if (term && !r.ingredients.some((i) => i.name.toLowerCase().includes(term)))
      return false;
    if (pantry && !r.inPantry) return false;
    if (maxMinutes !== undefined && r.totalMinutes > maxMinutes) return false;
    return true;
  });

  const clear = (key: keyof RecipeSearch) =>
    navigate({ search: (p: RecipeSearch) => ({ ...p, [key]: undefined }) });

  return (
    <div className="space-y-6">
      <div>
        <div className="mb-2 font-mono text-xs uppercase tracking-[0.12em] text-muted-foreground">
          Your kitchen
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <h1 className="text-3xl font-semibold tracking-tight">Recipes</h1>
          {ingredient && (
            <button
              onClick={() => clear("ingredient")}
              className="rounded-full border border-primary/40 bg-accent px-3 py-1 text-xs text-accent-foreground"
            >
              ingredient: {ingredient} ✕
            </button>
          )}
          {pantry && (
            <button
              onClick={() => clear("pantry")}
              className="rounded-full border border-emerald-500/40 bg-emerald-500/10 px-3 py-1 text-xs text-emerald-400"
            >
              in pantry ✕
            </button>
          )}
          {maxMinutes !== undefined && (
            <button
              onClick={() => clear("maxMinutes")}
              className="rounded-full border border-primary/40 bg-accent px-3 py-1 text-xs text-accent-foreground"
            >
              ≤ {maxMinutes} min ✕
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
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="truncate font-medium">{r.name}</span>
                  {r.inPantry && (
                    <span className="flex-none rounded-md border border-emerald-500/35 bg-emerald-500/10 px-1.5 py-0.5 font-mono text-[10px] text-emerald-400">
                      in pantry
                    </span>
                  )}
                </div>
                <div className="mt-0.5 truncate font-mono text-xs text-muted-foreground">
                  ⏱ {r.totalMinutes} min · {r.cuisine}
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
  validateSearch: (search: Record<string, unknown>): RecipeSearch => {
    const max =
      typeof search.maxMinutes === "number"
        ? search.maxMinutes
        : typeof search.maxMinutes === "string" && search.maxMinutes !== "" && !Number.isNaN(Number(search.maxMinutes))
          ? Number(search.maxMinutes)
          : undefined;
    return {
      ingredient: typeof search.ingredient === "string" ? search.ingredient : undefined,
      pantry:
        search.pantry === true || search.pantry === "true" || search.pantry === "1"
          ? true
          : undefined,
      maxMinutes: max,
    };
  },
  component: Recipes,
});
