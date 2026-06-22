import { Link } from "@tanstack/react-router";
import { ThemeToggle } from "../theme";

export function Sidebar() {
  return (
    <aside
      style={{ viewTransitionName: "sidebar" }}
      className="flex w-52 flex-none flex-col border-r border-border bg-card/60 p-5 backdrop-blur"
    >
      <div className="mb-8 flex items-center gap-3">
        <div className="brand-mark h-7 w-7 rounded-[9px]" />
        <span className="text-base font-semibold tracking-tight">Dinnerwise</span>
      </div>

      <nav className="flex flex-col gap-1">
        <Link
          to="/"
          className="flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-muted-foreground hover:bg-muted [&.active]:bg-accent [&.active]:text-foreground"
        >
          <span className="h-1.5 w-1.5 rounded-full bg-current opacity-60" />
          Home
        </Link>
        <Link
          to="/meals"
          className="flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-muted-foreground hover:bg-muted [&.active]:bg-accent [&.active]:text-foreground"
        >
          <span className="h-1.5 w-1.5 rounded-full bg-current opacity-60" />
          Meals
        </Link>
        <Link
          to="/recipes"
          className="flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium text-muted-foreground hover:bg-muted [&.active]:bg-accent [&.active]:text-foreground"
        >
          <span className="h-1.5 w-1.5 rounded-full bg-current opacity-60" />
          Recipes
        </Link>
      </nav>

      <div className="mt-auto">
        <ThemeToggle />
      </div>
    </aside>
  );
}
