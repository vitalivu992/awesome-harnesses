import { cursorClient, harnessName } from "../lib/format.js"

const ICONS = {
  claude: "/agent-icons/claude.svg",
  cursor: "/agent-icons/cursor.ico",
  codex: "/agent-icons/codex.ico",
  pi: "/agent-icons/pi.svg",
  opencode: "/agent-icons/opencode.svg",
  kilo: "/agent-icons/kilo.svg",
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

/** "CLI" / "App" for a Cursor chat, so the user knows where to continue it; nothing for other harnesses. */
export function ClientBadge({ conversation }) {
  const client = cursorClient(conversation)
  if (!client) return null
  return (
    <span className={`ah-client ah-client--${client.id}`} title={client.hint}>
      <i className={`fa-solid ${client.icon}`} aria-hidden="true"></i> {client.label}
    </span>
  )
}
