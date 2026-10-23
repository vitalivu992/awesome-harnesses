import { statusMeta } from "../lib/format.js"

/** Conversation / turn status icon, coloured by status (styles: .ah-status--*). */
export function StatusIcon({ status, active = false }) {
  const key = active ? "active" : status || "done"
  const meta = statusMeta(key)
  return (
    <i
      className={`fa-solid ${meta.icon} ah-status ah-status--${key}`}
      title={meta.label}
      aria-label={meta.label}
      role="img"
    ></i>
  )
}
