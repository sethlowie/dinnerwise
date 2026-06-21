import { createRouter } from "@tanstack/react-router";
import { rootRoute } from "./routes/__root";
import { indexRoute } from "./routes/index";
import { recipesRoute } from "./routes/recipes";
import { recipeDetailRoute } from "./routes/recipe";
import { mealsRoute } from "./routes/meals";
import { mealDetailRoute } from "./routes/meal";

const routeTree = rootRoute.addChildren([
  indexRoute,
  recipesRoute,
  recipeDetailRoute,
  mealsRoute,
  mealDetailRoute,
]);

export const router = createRouter({ routeTree });

// Register the router instance for full type inference across the app.
declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
