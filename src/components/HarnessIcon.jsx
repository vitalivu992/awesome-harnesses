import { harnessName } from "../lib/format.js"

const ICONS = {
  claude: "/agent-icons/claude.svg",
  cursor: "/agent-icons/cursor.ico",
}

/** The harness logo; `size` in px. */
export function HarnessIcon({ harness, size = 14, className = "" }) {
  const src = ICONS[harness]
  if (!src) return <i className={`fa-solid fa-robot ${className}`} aria-hidden="true"></i>
  return (
    <img
      className={`ah-harness-icon ah-harness-icon--${harness} ${className}`.trim()}
      src={src}
      width={size}
      height={size}
      alt=""
      title={harnessName(harness)}
    />
  )
}
