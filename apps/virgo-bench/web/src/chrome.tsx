import { useEffect, useState } from "react";
import type { ReactNode } from "react";

// Keep in sync with the inline script in index.html, which applies the saved
// theme before first paint.
const THEME_KEY = "virgo-bench-theme";

export type ThemePref = "system" | "light" | "dark";

function readPref(): ThemePref {
  try {
    const v = localStorage.getItem(THEME_KEY);
    return v === "light" || v === "dark" ? v : "system";
  } catch {
    return "system";
  }
}

export function useTheme(): [ThemePref, (p: ThemePref) => void] {
  const [pref, setPref] = useState<ThemePref>(readPref);
  useEffect(() => {
    const root = document.documentElement;
    if (pref === "system") delete root.dataset.theme;
    else root.dataset.theme = pref;
    try {
      if (pref === "system") localStorage.removeItem(THEME_KEY);
      else localStorage.setItem(THEME_KEY, pref);
    } catch {
      /* storage unavailable; the choice lasts for this page only */
    }
  }, [pref]);
  return [pref, setPref];
}

const icon = (path: ReactNode) => (
  <svg
    viewBox="0 0 24 24"
    width="16"
    height="16"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
    aria-hidden="true"
  >
    {path}
  </svg>
);

export const Icon = {
  sun: icon(
    <>
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
    </>,
  ),
  moon: icon(<path d="M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z" />),
  monitor: icon(
    <>
      <rect x="2" y="3" width="20" height="14" rx="2" />
      <path d="M8 21h8M12 17v4" />
    </>,
  ),
  back: icon(<path d="M19 12H5M12 19l-7-7 7-7" />),
  trash: icon(<path d="M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6" />),
  x: icon(<path d="M18 6 6 18M6 6l12 12" />),
  chevron: icon(<path d="m9 18 6-6-6-6" />),
};

export function Logo({ size = 24 }: { size?: number }) {
  return (
    <svg viewBox="0 0 32 32" width={size} height={size} aria-hidden="true" className="logo">
      <rect width="32" height="32" rx="7" fill="var(--logo-tile)" />
      <rect x="8" y="17" width="4" height="7" rx="1" fill="var(--logo-bar)" />
      <rect x="14" y="12" width="4" height="12" rx="1" fill="var(--logo-bar)" />
      <rect x="20" y="8" width="4" height="16" rx="1" fill="var(--logo-bar)" />
    </svg>
  );
}

export function ThemeToggle() {
  const [pref, setPref] = useTheme();
  const opts: { value: ThemePref; label: string; icon: ReactNode }[] = [
    { value: "light", label: "Light", icon: Icon.sun },
    { value: "system", label: "System", icon: Icon.monitor },
    { value: "dark", label: "Dark", icon: Icon.moon },
  ];
  return (
    <div className="segmented" role="radiogroup" aria-label="Color theme">
      {opts.map((o) => (
        <button
          key={o.value}
          type="button"
          role="radio"
          aria-checked={pref === o.value}
          title={`${o.label} theme`}
          className={pref === o.value ? "on" : ""}
          onClick={() => setPref(o.value)}
        >
          {o.icon}
          <span className="sr-only">{o.label}</span>
        </button>
      ))}
    </div>
  );
}
