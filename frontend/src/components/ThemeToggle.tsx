"use client";

import { useEffect, useState } from "react";

type ThemeMode = "light" | "dark";

function applyTheme(mode: ThemeMode) {
  const root = document.documentElement;
  if (mode === "dark") {
    root.classList.add("dark");
  } else {
    root.classList.remove("dark");
  }
  localStorage.setItem("theme", mode);
  window.dispatchEvent(new Event("theme-changed"));
}

export default function ThemeToggle() {
  const [mode, setMode] = useState<ThemeMode>("light");
  const [ready, setReady] = useState(false);

  useEffect(() => {
    const saved = (localStorage.getItem("theme") as ThemeMode | null) || null;
    const prefersDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
    const next: ThemeMode = saved || (prefersDark ? "dark" : "light");
    setMode(next);
    applyTheme(next);
    setReady(true);
  }, []);

  if (!ready) return null;

  return (
    <button
      type="button"
      onClick={() => {
        const next: ThemeMode = mode === "light" ? "dark" : "light";
        setMode(next);
        applyTheme(next);
      }}
      className="fixed bottom-5 right-5 z-[70] rounded-full border border-[var(--app-border)] bg-[var(--app-surface)] px-3 py-2 text-xs font-semibold text-[var(--app-fg)] shadow-lg"
      aria-label="Toggle theme"
      title="Toggle light/dark theme"
    >
      {mode === "light" ? "🌙 Dark" : "☀️ Light"}
    </button>
  );
}
