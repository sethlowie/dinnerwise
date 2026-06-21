import { createRootRoute, Link, Outlet } from "@tanstack/react-router";
import { ThemeToggle } from "../theme";

function RootLayout() {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <header className="border-b border-border">
        <div className="mx-auto flex max-w-3xl items-center justify-between px-4 py-3">
          <nav className="flex items-center gap-4">
            <span className="font-semibold">dinnerwise</span>
            <Link
              to="/"
              className="text-muted-foreground [&.active]:text-foreground"
            >
              Home
            </Link>
            <Link
              to="/recipes"
              className="text-muted-foreground [&.active]:text-foreground"
            >
              Recipes
            </Link>
          </nav>
          <ThemeToggle />
        </div>
      </header>
      <main className="mx-auto max-w-3xl px-4 py-8">
        <Outlet />
      </main>
    </div>
  );
}

export const rootRoute = createRootRoute({ component: RootLayout });
