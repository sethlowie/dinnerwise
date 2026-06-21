import { createRoute, getRouteApi, Link } from "@tanstack/react-router";
import { useQuery } from "@connectrpc/connect-query";
import { listRecipes } from "../gen/internal/recipe/v1/recipe-RecipeService_connectquery";
import { rootRoute } from "./__root";

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
    <div className="space-y-4">
      <div className="flex items-center gap-3">
        <h1 className="text-2xl font-semibold">Recipes</h1>
        {ingredient && (
          <button
            onClick={() => navigate({ search: {} })}
            className="rounded-full bg-accent px-2 py-0.5 text-xs text-accent-foreground"
          >
            ingredient: {ingredient} ✕
          </button>
        )}
      </div>
      <ul className="grid gap-4 sm:grid-cols-2">
        {recipes.map((r) => (
          <li key={r.id}>
            <Link
              to="/recipes/$id"
              params={{ id: r.id }}
              className="block rounded-lg border border-border bg-card p-4 text-card-foreground hover:border-primary"
            >
              <h2 className="font-medium">{r.name}</h2>
              <p className="mt-1 text-sm text-muted-foreground">
                ⏱ {r.totalMinutes} min · serves {r.servings}
              </p>
              <div className="mt-3 flex flex-wrap gap-1">
                {r.ingredients.map((ing) => (
                  <span
                    key={ing.ingredientId}
                    className="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground"
                  >
                    {ing.name}
                  </span>
                ))}
              </div>
            </Link>
          </li>
        ))}
      </ul>
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
