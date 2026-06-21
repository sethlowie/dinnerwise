import { createRoute, getRouteApi, Link } from "@tanstack/react-router";
import { useQuery } from "@connectrpc/connect-query";
import { getRecipe } from "../gen/internal/recipe/v1/recipe-RecipeService_connectquery";
import { appLayoutRoute } from "./app-layout";
import { initials, thumbStyle, tintFor } from "../lib/thumb";

const routeApi = getRouteApi("/app/recipes/$id");

function RecipeDetail() {
  const { id } = routeApi.useParams();
  const { data, error, isPending } = useQuery(getRecipe, { id });

  if (isPending) return <p className="text-muted-foreground">Loading…</p>;
  if (error)
    return <p className="text-red-600 dark:text-red-400">{error.message}</p>;

  const recipe = data.recipe;
  if (!recipe) return <p className="text-muted-foreground">Not found.</p>;

  const tint = tintFor(recipe.id);

  return (
    <article className="space-y-7">
      <Link
        to="/recipes"
        className="font-mono text-sm text-muted-foreground hover:text-foreground"
      >
        ← Recipes
      </Link>

      <div className="flex items-start gap-5">
        <div
          className="flex h-16 w-16 flex-none items-center justify-center rounded-2xl font-mono text-lg font-semibold"
          style={thumbStyle(tint)}
        >
          {initials(recipe.name)}
        </div>
        <div>
          <h1 className="text-3xl font-semibold tracking-tight">{recipe.name}</h1>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <span className="font-mono text-sm text-muted-foreground">
              ⏱ {recipe.totalMinutes} min · {recipe.cuisine} · {recipe.difficulty}
            </span>
            {recipe.inPantry && (
              <span className="rounded-md border border-emerald-500/35 bg-emerald-500/10 px-1.5 py-0.5 font-mono text-[10px] text-emerald-400">
                in pantry
              </span>
            )}
          </div>
        </div>
      </div>

      <section>
        <div className="mb-3 font-mono text-xs uppercase tracking-[0.1em] text-muted-foreground">
          Ingredients
        </div>
        <ul className="flex flex-col gap-2">
          {recipe.ingredients.map((ing) => (
            <li
              key={ing.ingredientId}
              className="flex items-baseline gap-3 text-sm text-foreground/85"
            >
              <span className="mt-1.5 h-1.5 w-1.5 flex-none rounded-full bg-primary" />
              {ing.quantity} {ing.unit} {ing.name}
            </li>
          ))}
        </ul>
      </section>

      <section>
        <div className="mb-3 font-mono text-xs uppercase tracking-[0.1em] text-muted-foreground">
          Method
        </div>
        <ol className="flex flex-col gap-3">
          {recipe.steps.map((step, i) => (
            <li key={i} className="flex gap-3">
              <span className="flex h-6 w-6 flex-none items-center justify-center rounded-lg border border-primary/40 bg-primary/10 font-mono text-xs text-primary">
                {i + 1}
              </span>
              <span className="text-sm leading-relaxed text-foreground/85">{step}</span>
            </li>
          ))}
        </ol>
      </section>
    </article>
  );
}

export const recipeDetailRoute = createRoute({
  getParentRoute: () => appLayoutRoute,
  path: "/recipes/$id",
  component: RecipeDetail,
});
