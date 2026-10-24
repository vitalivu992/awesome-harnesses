// The prompt box's slash commands and skills: which token the caret is in,
// what it completes to, and which part of the text is a known command (shown
// highlighted). Commands come from GET /api/harnesses/{id}/commands; their
// `insert` is what the prompt starts with ("/fix-backlog", "/skill:wiki"), or
// for Codex skills a "$name" mention that may stand anywhere.

const SPACE = /\s/
const NAME = /[A-Za-z0-9_-]/

/**
 * The command token being typed at `caret`: "/…" at the start of the prompt,
 * or a "$…" mention anywhere when the harness has those. `{start, end, query}` or null.
 */
export function completionAt(text, caret, { mentions = false } = {}) {
  const s = String(text || "")
  let start = caret
  while (start > 0 && !SPACE.test(s[start - 1])) start--
  const ch = s[start]
  if (ch !== "/" && !(ch === "$" && mentions)) return null
  if (ch === "/" && s.slice(0, start).trim() !== "") return null
  let end = caret
  while (end < s.length && !SPACE.test(s[end])) end++
  return { start, end, query: s.slice(start, caret) }
}

/** Commands matching what was typed: prefix matches first, then the ones containing it. */
export function matchCommands(commands, query, limit = 50) {
  const q = String(query || "").toLowerCase()
  const trigger = q[0]
  const word = q.slice(1)
  const pool = (commands || []).filter((c) => c.insert[0] === trigger)
  const prefix = []
  const inside = []
  for (const c of pool) {
    const name = c.insert.slice(1).toLowerCase()
    if (name.startsWith(word)) prefix.push(c)
    else if (word && (name.includes(word) || (c.description || "").toLowerCase().includes(word))) inside.push(c)
  }
  return [...prefix, ...inside].slice(0, limit)
}

/** Replaces the token with the command (and a space after it): `{text, caret}`. */
export function applyCompletion(text, token, insert) {
  const s = String(text || "")
  const after = s.slice(token.end)
  const gap = after && SPACE.test(after[0]) ? "" : " "
  const next = s.slice(0, token.start) + insert + gap + after
  return { text: next, caret: token.start + insert.length + 1 }
}

const ends = (s, i) => i >= s.length || SPACE.test(s[i])

/**
 * The text cut into `{ text, command }` segments, `command` set on the
 * known command at the start of the prompt and on "$" mentions.
 */
export function highlightSegments(text, commands) {
  const s = String(text || "")
  const marks = []
  const lead = s.length - s.trimStart().length
  const slash = (commands || [])
    .filter((c) => c.insert[0] !== "$" && s.startsWith(c.insert, lead) && ends(s, lead + c.insert.length))
    .sort((a, b) => b.insert.length - a.insert.length)[0]
  if (slash) marks.push({ start: lead, end: lead + slash.insert.length, command: slash })

  // A mention ends where the name can't go on ("$playwright," is one).
  const mentions = (commands || []).filter((c) => c.insert[0] === "$").sort((a, b) => b.insert.length - a.insert.length)
  for (let i = s.indexOf("$"); i >= 0 && mentions.length; i = s.indexOf("$", i + 1)) {
    if (i > 0 && !SPACE.test(s[i - 1])) continue
    const command = mentions.find((c) => s.startsWith(c.insert, i) && !NAME.test(s[i + c.insert.length] || ""))
    if (command) marks.push({ start: i, end: i + command.insert.length, command })
  }
  marks.sort((a, b) => a.start - b.start)

  const out = []
  let at = 0
  for (const mark of marks) {
    if (mark.start < at) continue
    if (mark.start > at) out.push({ text: s.slice(at, mark.start), command: null })
    out.push({ text: s.slice(mark.start, mark.end), command: mark.command })
    at = mark.end
  }
  if (at < s.length || !out.length) out.push({ text: s.slice(at), command: null })
  return out
}

/** The commands the prompt uses (for the "uses /fix-backlog" hint). */
export function usedCommands(text, commands) {
  return highlightSegments(text, commands)
    .filter((seg) => seg.command)
    .map((seg) => seg.command)
}
