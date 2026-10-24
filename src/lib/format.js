// Labels and times for the UI: harness and model names, statuses, dates.

// In display order (project counts, harness filter).
export const HARNESSES = {
  claude: { id: "claude", name: "Claude Code", cli: "claude" },
  cursor: { id: "cursor", name: "Cursor", cli: "cursor-agent" },
  codex: { id: "codex", name: "Codex", cli: "codex" },
  pi: { id: "pi", name: "pi", cli: "pi" },
  opencode: { id: "opencode", name: "OpenCode", cli: "opencode" },
  kilo: { id: "kilo", name: "Kilo Code", cli: "kilo" },
}

export const HARNESS_IDS = Object.keys(HARNESSES)

export function harnessName(id) {
  return HARNESSES[id]?.name || id || ""
}

const UPPER_WORDS = new Set(["gpt", "glm", "o1", "o3", "o4"])

function capitalize(word) {
  if (UPPER_WORDS.has(word.toLowerCase())) return word.toUpperCase()
  return word ? word[0].toUpperCase() + word.slice(1) : word
}

/**
 * A readable model name: "claude-opus-5-5" → "Opus 5.5",
 * "cline-pass/deepseek-v4-flash" → "Deepseek V4 Flash",
 * "claude-sonnet-4-5-20250929" → "Sonnet 4.5", "claude-4.6-opus-high-thinking"
 * → "Opus 4.6 High Thinking", "claude-opus-5-5-high" → "Opus 5.5 High",
 * "composer-2.5-fast" → "Composer 2.5 Fast".
 */
export function modelLabel(id) {
  if (!id) return ""
  // "provider/model" (OpenCode, Kilo) → "model"; "[1m]" context suffix dropped.
  const s = String(id).replace(/\[[^\]]*\]$/, "").trim().split("/").pop()
  let m = s.match(/^claude-(opus|sonnet|haiku|fable)-(\d+)(?:-(\d{1,2}))?(?:-\d{8})?((?:-[a-z]+)*)$/i)
  if (m) {
    const rest = m[4].split("-").filter(Boolean).map(capitalize)
    return [capitalize(m[1]), `${m[2]}${m[3] ? `.${m[3]}` : ""}`, ...rest].join(" ")
  }
  m = s.match(/^claude-(\d+(?:\.\d+)?)-(opus|sonnet|haiku|fable)((?:-[a-z0-9.]+)*)$/i)
  if (m) {
    const rest = m[3].split("-").filter(Boolean).map(capitalize)
    return [capitalize(m[2]), m[1], ...rest].join(" ")
  }
  return s.split("-").filter(Boolean).map(capitalize).join(" ")
}

/** "Claude Code - Opus 5.5", or just the harness when the model is unknown. */
export function agentLabel(harness, model) {
  const name = harnessName(harness)
  const label = modelLabel(model)
  return label ? `${name} - ${label}` : name
}

/**
 * Where a Cursor chat was started, so the user knows where to continue it: the
 * CLI (cursor-agent keeps a chat store for it, so it is resumable) or the app.
 * null for the other harnesses.
 */
export function cursorClient(conversation) {
  if (conversation?.harness !== "cursor") return null
  return conversation.resumable
    ? { id: "cli", label: "CLI", icon: "fa-terminal", hint: "Started in the Cursor CLI: continue it with cursor-agent --resume" }
    : { id: "app", label: "App", icon: "fa-desktop", hint: "Started in the Cursor app: continue it in the Cursor app" }
}

export const STATUSES = {
  done: { label: "Completed", icon: "fa-circle-check" },
  error: { label: "Ended with an error", icon: "fa-circle-xmark" },
  aborted: { label: "Interrupted", icon: "fa-circle-stop" },
  incomplete: { label: "Unfinished", icon: "fa-circle-half-stroke" },
}

/** "Good morning" … "Good evening" by the hour of `date`, "Hi" late at night. */
export function greeting(date = new Date()) {
  const h = date.getHours()
  if (h >= 5 && h < 12) return "Good morning"
  if (h >= 12 && h < 18) return "Good afternoon"
  if (h >= 18 && h < 23) return "Good evening"
  return "Hi"
}

export function statusMeta(status) {
  return STATUSES[status] || STATUSES.done
}

function toDate(iso) {
  if (!iso) return null
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? null : d
}

const pad = (n) => String(n).padStart(2, "0")

/** List date: "14:05" today, "Oct 23 14:05" this year, else "2025-10-23". */
export function shortDateTime(iso, now = new Date()) {
  const d = toDate(iso)
  if (!d) return ""
  const time = `${pad(d.getHours())}:${pad(d.getMinutes())}`
  if (d.toDateString() === now.toDateString()) return time
  if (d.getFullYear() === now.getFullYear()) {
    return `${d.toLocaleString("en-US", { month: "short" })} ${d.getDate()} ${time}`
  }
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

/** "Fri, Oct 23, 2026, 14:05". */
export function longDateTime(iso) {
  const d = toDate(iso)
  if (!d) return ""
  return d.toLocaleString("en-US", {
    weekday: "short",
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  })
}

/** "14:05:09". */
export function clockTime(iso) {
  const d = toDate(iso)
  if (!d) return ""
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

/** "3 min ago", "2 h ago", "5 d ago". */
export function relativeTime(iso, now = Date.now()) {
  const d = toDate(iso)
  if (!d) return ""
  const s = Math.max(0, Math.round((now - d.getTime()) / 1000))
  if (s < 60) return "just now"
  if (s < 3600) return `${Math.floor(s / 60)} min ago`
  if (s < 86400) return `${Math.floor(s / 3600)} h ago`
  if (s < 86400 * 30) return `${Math.floor(s / 86400)} d ago`
  if (s < 86400 * 365) return `${Math.floor(s / (86400 * 30))} mo ago`
  return `${Math.floor(s / (86400 * 365))} y ago`
}

/** "850 ms", "12 s", "3m 20s", "1h 5m" between two timestamps, or "". */
export function duration(fromIso, toIso) {
  const a = toDate(fromIso)
  const b = toDate(toIso)
  if (!a || !b) return ""
  return formatMs(b.getTime() - a.getTime())
}

export function formatMs(ms) {
  if (!(ms >= 0)) return ""
  if (ms < 1000) return `${Math.round(ms)} ms`
  const s = Math.round(ms / 1000)
  if (s < 60) return `${s} s`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m ${pad(s % 60)}s`
  return `${Math.floor(m / 60)}h ${pad(m % 60)}m`
}

export function plural(n, word, many = `${word}s`) {
  return `${n} ${n === 1 ? word : many}`
}
