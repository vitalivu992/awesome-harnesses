import { THEMES, setTheme } from "../lib/theme.js"
import { useTheme } from "../hooks/useTheme.js"

const META = {
  system: { icon: "fa-circle-half-stroke", label: "System theme" },
  light: { icon: "fa-sun", label: "Light theme" },
  dark: { icon: "fa-moon", label: "Dark theme" },
}

/** Cycles system → light → dark. */
export function ThemeToggle({ compact = false }) {
  const { pref } = useTheme()
  const next = THEMES[(THEMES.indexOf(pref) + 1) % THEMES.length]
  const meta = META[pref]
  return (
    <button
      type="button"
      className="ah-icon-btn ah-theme-toggle"
      onClick={() => setTheme(next)}
      title={`${meta.label} (click for ${META[next].label.toLowerCase()})`}
      aria-label={meta.label}
    >
      <i className={`fa-solid ${meta.icon}`} aria-hidden="true"></i>
      {compact ? null : <span>{meta.label}</span>}
    </button>
  )
}
