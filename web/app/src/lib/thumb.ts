import type { CSSProperties } from "react";

const TINTS = ["#7c6cf5", "#5eb888", "#e0775e", "#e0a14e", "#d9c24e", "#9a8cf0"];

// Deterministic tint from a stable seed (e.g. recipe id) so a given item always
// gets the same color.
export function tintFor(seed: string): string {
  let h = 0;
  for (let i = 0; i < seed.length; i++) {
    h = (h * 31 + seed.charCodeAt(i)) >>> 0;
  }
  return TINTS[h % TINTS.length];
}

// Up to two initials from a name (e.g. "Tomato Pasta" -> "TP").
export function initials(name: string): string {
  const w = name.split(/\s+/).filter(Boolean);
  const a = w[0]?.[0] ?? "";
  const b = w[1]?.[0] ?? w[0]?.[1] ?? "";
  return (a + b).toUpperCase();
}

// Inline style for a tinted "thumb" box (gradient fill + matching border/text).
export function thumbStyle(tint: string): CSSProperties {
  return {
    background: `linear-gradient(140deg, ${tint}33, ${tint}14)`,
    border: `1px solid ${tint}40`,
    color: tint,
  };
}
