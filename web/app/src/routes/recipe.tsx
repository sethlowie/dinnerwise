import { createRoute, getRouteApi, Link } from "@tanstack/react-router";
import { useQuery } from "@connectrpc/connect-query";
import { getRecipe } from "../gen/internal/recipe/v1/recipe-RecipeService_connectquery";
import { rootRoute } from "./__root";
import { initials, thumbStyle, tintFor } from "../lib/thumb";

const routeApi = getRouteApi("/recipes/$id");

function RecipeDetail() {
  const { id } = routeApi.useParams();
  const { data, error, isPending } = useQuery(getRecipe, { id });

  if (isPending) return <p className="text-muted-foreground">Loading…</p>;
  if (error)
    return <p className="text-red-600 dark:text-red-400">{error.message}</p>;

  const recipe = data.recipe;
  if (!recipe) return <p className="text-muted-foreground">Not found.</p>;

  const tint = tintFor(recipe.id);
  const methodLines = recipe.instructions
    .split(/\n+/)
    .map((l) => l.trim())
    .filter(Boolean);

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
          <p className="mt-2 font-mono text-sm text-muted-foreground">
            ⏱ {recipe.totalMinutes} min · serves {recipe.servings}
          </p>
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
        <div className="flex flex-col gap-3 text-sm leading-relaxed text-foreground/85">
          {methodLines.map((line, i) => (
            <p key={i}>{line}</p>
          ))}
        </div>
      </section>
    </article>
  );
}

export const recipeDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/recipes/$id",
  component: RecipeDetail,
});
