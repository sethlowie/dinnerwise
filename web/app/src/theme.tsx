import {
  useCallback,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import {
  STORAGE_KEY,
  ThemeContext,
  readStoredTheme,
  systemPrefersDark,
  useTheme,
  type Theme,
} from "./theme-context";

function applyResolved(resolved: "light" | "dark") {
  document.documentElement.classList.toggle("dark", resolved === "dark");
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(readStoredTheme);
  const [resolved, setResolved] = useState<"light" | "dark">(() =>
    readStoredTheme() === "system"
      ? systemPrefersDark()
        ? "dark"
        : "light"
      : (readStoredTheme() as "light" | "dark"),
  );

  // Keep the <html> class and resolved scheme in sync with the choice, and
  // follow the OS when the choice is "system".
  useEffect(() => {
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const compute = () => (theme === "system" ? media.matches : theme === "dark");

    const sync = () => {
      const dark = compute();
      setResolved(dark ? "dark" : "light");
      applyResolved(dark ? "dark" : "light");
    };

    sync();
    if (theme === "system") {
      media.addEventListener("change", sync);
      return () => media.removeEventListener("change", sync);
    }
  }, [theme]);

  const setTheme = useCallback((next: Theme) => {
    if (next === "system") localStorage.removeItem(STORAGE_KEY);
    else localStorage.setItem(STORAGE_KEY, next);
    setThemeState(next);
  }, []);

  const toggle = useCallback(() => {
    setThemeState((prev) => {
      const current =
        prev === "system" ? (systemPrefersDark() ? "dark" : "light") : prev;
      const next = current === "dark" ? "light" : "dark";
      localStorage.setItem(STORAGE_KEY, next);
      return next;
    });
  }, []);

  return (
    <ThemeContext value={{ theme, resolved, setTheme, toggle }}>
      {children}
    </ThemeContext>
  );
}

export function ThemeToggle() {
  const { resolved, toggle } = useTheme();
  return (
    <button
      onClick={toggle}
      aria-label="Toggle color theme"
      className="rounded-lg border border-border bg-card px-3 py-1.5 text-sm text-card-foreground hover:bg-accent hover:text-accent-foreground"
    >
      {resolved === "dark" ? "☀ Light" : "☾ Dark"}
    </button>
  );
}
