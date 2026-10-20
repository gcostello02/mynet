import { useCallback, useEffect, useState } from "react";

export type ThemePref = "dark" | "light" | "system";
const KEY = "mynet_theme"; // appearance preference only — never secrets

function stored(): ThemePref {
  try {
    const v = localStorage.getItem(KEY);
    if (v === "light" || v === "dark" || v === "system") return v;
  } catch {
    /* storage unavailable */
  }
  return "dark";
}

export function resolveTheme(pref: ThemePref): "dark" | "light" {
  if (pref !== "system") return pref;
  return window.matchMedia?.("(prefers-color-scheme: light)").matches ? "light" : "dark";
}

export function applyTheme(pref: ThemePref) {
  const root = document.documentElement;
  const t = resolveTheme(pref);
  root.classList.toggle("light", t === "light");
  root.classList.toggle("dark", t === "dark");
}

/** Apply the saved preference before first paint. */
export function initTheme() {
  applyTheme(stored());
}

export function useTheme() {
  const [pref, setPrefState] = useState<ThemePref>(stored);

  useEffect(() => {
    applyTheme(pref);
    try {
      localStorage.setItem(KEY, pref);
    } catch {
      /* ignore */
    }
    if (pref !== "system") return;
    const mq = window.matchMedia("(prefers-color-scheme: light)");
    const on = () => applyTheme("system");
    mq.addEventListener("change", on);
    return () => mq.removeEventListener("change", on);
  }, [pref]);

  const setPref = useCallback((p: ThemePref) => setPrefState(p), []);
  const toggle = useCallback(() => setPrefState((p) => (resolveTheme(p) === "dark" ? "light" : "dark")), []);
  return { pref, setPref, theme: resolveTheme(pref), toggle };
}
