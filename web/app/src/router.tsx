import { createRouter } from "@tanstack/react-router";
import { rootRoute } from "./routes/__root";
import { indexRoute } from "./routes/index";
import { recipesRoute } from "./routes/recipes";
import { recipeDetailRoute } from "./routes/recipe";

const routeTree = rootRoute.addChildren([
  indexRoute,
  recipesRoute,
  recipeDetailRoute,
]);

export const router = createRouter({ routeTree });

// Register the router instance for full type inference across the app.
declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
