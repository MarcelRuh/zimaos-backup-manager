import { useEffect, useState } from "react";

export type ThemeMode = "light" | "dark" | "system";

function resolve(mode: ThemeMode) {
  if (mode === "system") {
    return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  }
  return mode;
}

export function useTheme(initial: ThemeMode = "system") {
  const [mode, setMode] = useState<ThemeMode>(initial);

  useEffect(() => {
    const applied = resolve(mode);
    document.documentElement.classList.toggle("dark", applied === "dark");
  }, [mode]);

  useEffect(() => {
    if (mode !== "system") return;
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => document.documentElement.classList.toggle("dark", mq.matches);
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
  }, [mode]);

  return { mode, setMode };
}
