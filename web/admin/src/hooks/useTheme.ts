import { useCallback, useEffect, useState } from "react";

type Theme = "dark" | "light";

function getInitial(): Theme {
  const stored = localStorage.getItem("mynet_theme");
  if (stored === "light" || stored === "dark") return stored;
  return "dark";
}

export function useTheme() {
  const [theme, setThemeState] = useState<Theme>(getInitial);

  useEffect(() => {
    const root = document.documentElement;
    root.classList.remove("dark", "light");
    root.classList.add(theme);
    localStorage.setItem("mynet_theme", theme);
  }, [theme]);

  const toggle = useCallback(() => {
    setThemeState((t) => (t === "dark" ? "light" : "dark"));
  }, []);

  return { theme, toggle };
}
