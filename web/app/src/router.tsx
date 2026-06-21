import { createRouter } from "@tanstack/react-router";
import { rootRoute } from "./routes/__root";
import { indexRoute } from "./routes/index";
import { appLayoutRoute } from "./routes/app-layout";
import { recipesRoute } from "./routes/recipes";
import { recipeDetailRoute } from "./routes/recipe";
import { mealsRoute } from "./routes/meals";
import { mealDetailRoute } from "./routes/meal";

const routeTree = rootRoute.addChildren([
  indexRoute,
  appLayoutRoute.addChildren([
    recipesRoute,
    recipeDetailRoute,
    mealsRoute,
    mealDetailRoute,
  ]),
]);

export const router = createRouter({
  routeTree,
  // Tag every navigation so the CSS can tell the shell morph apart from a
  // plain content swap: entering the app (home -> route), leaving it
  // (route -> home), or moving between app routes.
  defaultViewTransition: {
    types: ({ fromLocation, toLocation }) => {
      const from = fromLocation?.pathname ?? "/";
      const to = toLocation.pathname;
      if (from === "/" && to !== "/") return ["enter-app"];
      if (from !== "/" && to === "/") return ["leave-app"];
      return ["within-app"];
    },
  },
});

// Register the router instance for full type inference across the app.
declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
