import { createRoute, getRouteApi, Link } from "@tanstack/react-router";
import { useQuery } from "@connectrpc/connect-query";
import { getMeal } from "../gen/internal/meal/v1/meal-MealService_connectquery";
import { rootRoute } from "./__root";
import { initials, thumbStyle, tintFor } from "../lib/thumb";

const routeApi = getRouteApi("/meals/$id");

function MealDetail() {
  const { id } = routeApi.useParams();
  const { data, error, isPending } = useQuery(getMeal, { id });

  if (isPending) return <p className="text-muted-foreground">Loading…</p>;
  if (error)
    return <p className="text-red-600 dark:text-red-400">{error.message}</p>;

  const meal = data.meal;
  if (!meal) return <p className="text-muted-foreground">Not found.</p>;
  const tint = tintFor(meal.id);

  return (
    <article className="space-y-7">
      <Link
        to="/meals"
        className="font-mono text-sm text-muted-foreground hover:text-foreground"
      >
        ← Meals
      </Link>

      <div className="flex items-start gap-5">
        <div
          className="flex h-16 w-16 flex-none items-center justify-center rounded-2xl font-mono text-lg font-semibold"
          style={thumbStyle(tint)}
        >
          {initials(meal.name)}
        </div>
        <div>
          <h1 className="text-3xl font-semibold tracking-tight">{meal.name}</h1>
          <div className="mt-2 flex items-center gap-3">
            <span className="tracking-widest">
              <span className="text-amber-400">{"★".repeat(meal.rating)}</span>
              <span className="text-muted-foreground/40">{"★".repeat(5 - meal.rating)}</span>
            </span>
            <span className="font-mono text-sm text-muted-foreground">{meal.cuisine}</span>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div className="rounded-2xl border border-border bg-card/60 p-4">
          <div className="font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
            Times cooked
          </div>
          <div className="mt-1 text-2xl font-semibold">{meal.timesCooked}</div>
        </div>
        <div className="rounded-2xl border border-border bg-card/60 p-4">
          <div className="font-mono text-[11px] uppercase tracking-wider text-muted-foreground">
            Last eaten
          </div>
          <div className="mt-1 text-2xl font-semibold">{meal.lastCooked || "—"}</div>
        </div>
      </div>

      <section>
        <div className="mb-3 font-mono text-xs uppercase tracking-[0.1em] text-muted-foreground">
          Recent cooks
        </div>
        <div className="flex flex-col gap-2">
          {data.recentCooks.map((c, i) => (
            <div
              key={i}
              className="flex items-center justify-between rounded-xl border border-border bg-card/40 px-4 py-2.5"
            >
              <span className="font-mono text-sm text-muted-foreground">{c.cookedOn}</span>
              <span className="text-sm text-foreground/80">{c.note}</span>
            </div>
          ))}
        </div>
      </section>

      {meal.recipeId && (
        <Link
          to="/recipes/$id"
          params={{ id: meal.recipeId }}
          className="inline-flex items-center gap-2 rounded-xl bg-primary px-4 py-2.5 text-sm font-medium text-primary-foreground hover:opacity-90"
        >
          View recipe →
        </Link>
      )}
    </article>
  );
}

export const mealDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/meals/$id",
  component: MealDetail,
});
