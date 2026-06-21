import { createContext, useContext } from "react";

export type Theme = "light" | "dark" | "system";

export const STORAGE_KEY = "theme";

export type ThemeContextValue = {
  /** The user's choice, including "system". */
  theme: Theme;
  /** The actually-applied scheme after resolving "system". */
  resolved: "light" | "dark";
  setTheme: (theme: Theme) => void;
  /** Flip between light/dark (resolves "system" first, then overrides). */
  toggle: () => void;
};

export const ThemeContext = createContext<ThemeContextValue | null>(null);

export function systemPrefersDark() {
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

export function readStoredTheme(): Theme {
  const stored = localStorage.getItem(STORAGE_KEY);
  return stored === "light" || stored === "dark" ? stored : "system";
}

export function useTheme() {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error("useTheme must be used within a ThemeProvider");
  return ctx;
}
