import { createRoute, getRouteApi, Link } from "@tanstack/react-router";
import { useQuery } from "@connectrpc/connect-query";
import { getRecipe } from "../gen/internal/recipe/v1/recipe-RecipeService_connectquery";
import { rootRoute } from "./__root";

const routeApi = getRouteApi("/recipes/$id");

function RecipeDetail() {
  const { id } = routeApi.useParams();
  const { data, error, isPending } = useQuery(getRecipe, { id });

  if (isPending) return <p className="text-muted-foreground">Loading…</p>;
  if (error)
    return <p className="text-red-600 dark:text-red-400">{error.message}</p>;

  const recipe = data.recipe;
  if (!recipe) return <p className="text-muted-foreground">Not found.</p>;

  return (
    <article className="space-y-4">
      <Link
        to="/recipes"
        className="text-sm text-muted-foreground hover:text-foreground"
      >
        ← Recipes
      </Link>
      <h1 className="text-2xl font-semibold">{recipe.name}</h1>
      <p className="text-sm text-muted-foreground">
        ⏱ {recipe.totalMinutes} min · serves {recipe.servings}
      </p>
      <section>
        <h2 className="font-medium">Ingredients</h2>
        <ul className="mt-2 space-y-1 text-sm">
          {recipe.ingredients.map((ing) => (
            <li key={ing.ingredientId} className="text-foreground">
              {ing.quantity} {ing.unit} {ing.name}
            </li>
          ))}
        </ul>
      </section>
      <section>
        <h2 className="font-medium">Instructions</h2>
        <p className="mt-2 whitespace-pre-line text-foreground">
          {recipe.instructions}
        </p>
      </section>
    </article>
  );
}

export const recipeDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/recipes/$id",
  component: RecipeDetail,
});
