// Pure helpers for the file viewer (components/FileViewer.jsx): unified diff
// parsing, per-line highlighting, languages and the conflict choices.
// Unit-tested in tests/viewer.test.mjs.

/**
 * Parses `git diff` output into files of hunks with numbered lines:
 * [{ header: [lines before the first hunk], hunks: [{ header, lines: [{ kind, text, old, new }] }] }].
 * kind: add | del | ctx | meta. A combined diff (diff --cc, a conflict) has one
 * marker column per parent; a line is add / del when any column says so.
 */
export function parseDiff(text) {
  const files = []
  let file = null
  let hunk = null
  let cols = 1
  let oldNo = 0
  let newNo = 0
  const lines = String(text || "").split("\n")
  if (lines.length && lines[lines.length - 1] === "") lines.pop()
  for (const line of lines) {
    if (line.startsWith("diff ")) {
      file = { header: [line], hunks: [] }
      files.push(file)
      hunk = null
      continue
    }
    if (!file) {
      file = { header: [], hunks: [] }
      files.push(file)
    }
    const m = /^(@{2,}) -(\d+)(?:,\d+)?(?: -\d+(?:,\d+)?)* \+(\d+)(?:,\d+)? @{2,}(.*)$/.exec(line)
    if (m) {
      cols = m[1].length - 1
      oldNo = Number(m[2])
      newNo = Number(m[3])
      hunk = { header: line, lines: [] }
      file.hunks.push(hunk)
      continue
    }
    if (!hunk) {
      file.header.push(line)
      continue
    }
    if (line.startsWith("\\")) {
      hunk.lines.push({ kind: "meta", text: line })
      continue
    }
    const marks = line.slice(0, cols)
    const body = line.slice(cols)
    if (marks.includes("+")) hunk.lines.push({ kind: "add", text: body, new: newNo++ })
    else if (marks.includes("-")) hunk.lines.push({ kind: "del", text: body, old: oldNo++ })
    else hunk.lines.push({ kind: "ctx", text: body, old: oldNo++, new: newNo++ })
  }
  return files
}

/** Lines added and removed in a parsed diff. */
export function diffStats(files) {
  let add = 0
  let del = 0
  for (const f of files) {
    for (const h of f.hunks) {
      for (const l of h.lines) {
        if (l.kind === "add") add++
        else if (l.kind === "del") del++
      }
    }
  }
  return { add, del }
}

/** The file name of a diff: `b/` side, else `a/` (a deletion), from its `diff --git` line. */
export function diffFileName(file) {
  const first = file?.header?.[0] || ""
  const cc = /^diff --(?:cc|combined) (.+)$/.exec(first)
  if (cc) return cc[1]
  const m = /^diff --git a\/(.+) b\/(.+)$/.exec(first)
  return m ? m[2] : ""
}

/** A conflict marker line in a diff or a file (<<<<<<<, =======, >>>>>>>, |||||||). */
export const isMarker = (text) => /^(<{7}|={7}|>{7}|\|{7})( |$)/.test(text || "")

/** `{ name, dir }` of a path ("src/a.js" → "a.js", "src"). */
export function splitPath(path) {
  const p = String(path || "").replace(/\/+$/, "")
  const i = p.lastIndexOf("/")
  return i < 0 ? { name: p, dir: "" } : { name: p.slice(i + 1), dir: p.slice(0, i) }
}

/** The hljs language of a file name, "" when unknown. */
export function languageOf(path) {
  const name = splitPath(path).name.toLowerCase()
  if (name === "dockerfile") return "dockerfile"
  if (name === "makefile") return "makefile"
  const ext = name.includes(".") ? name.slice(name.lastIndexOf(".") + 1) : ""
  return (
    {
      js: "javascript",
      mjs: "javascript",
      cjs: "javascript",
      jsx: "javascript",
      ts: "typescript",
      tsx: "typescript",
      go: "go",
      py: "python",
      rb: "ruby",
      rs: "rust",
      java: "java",
      kt: "kotlin",
      c: "c",
      h: "c",
      cpp: "cpp",
      cc: "cpp",
      hpp: "cpp",
      cs: "csharp",
      php: "php",
      swift: "swift",
      sh: "bash",
      bash: "bash",
      zsh: "bash",
      json: "json",
      yml: "yaml",
      yaml: "yaml",
      toml: "ini",
      ini: "ini",
      md: "markdown",
      html: "xml",
      xml: "xml",
      svg: "xml",
      css: "css",
      scss: "scss",
      less: "less",
      sql: "sql",
      lua: "lua",
      pl: "perl",
      r: "r",
      diff: "diff",
      patch: "diff",
    }[ext] || ""
  )
}

/**
 * hljs HTML of a whole file → one HTML string per line: the spans open at a
 * line break are closed at its end and opened again on the next line, so
 * multi-line comments and strings keep their colour. A final newline adds no line.
 */
export function splitHighlighted(html) {
  const lines = []
  const open = []
  let cur = ""
  for (const m of String(html).matchAll(/(<span[^>]*>)|(<\/span>)|(\n)|([^<\n]+)/g)) {
    if (m[1]) {
      open.push(m[1])
      cur += m[1]
    } else if (m[2]) {
      open.pop()
      cur += m[2]
    } else if (m[3]) {
      lines.push(cur + "</span>".repeat(open.length))
      cur = open.join("")
    } else {
      cur += m[4]
    }
  }
  if (cur !== open.join("") || !lines.length) lines.push(cur + "</span>".repeat(open.length))
  return lines
}

/** How many lines a file's text has (a final newline adds none). */
export function lineCount(text) {
  if (!text) return 0
  const n = text.split("\n").length
  return text.endsWith("\n") ? n - 1 : n
}

/**
 * The conflict resolutions, in the order the viewer shows them: the first two
 * as buttons, the others in the … menu (they keep both sides' lines).
 */
export const RESOLUTIONS = [
  { take: "theirs", label: "Take theirs", hint: "The whole file becomes the incoming side's version." },
  { take: "mine", label: "Take mine", hint: "The whole file becomes your side's version." },
  { take: "theirs-mine", label: "Take theirs, then mine", hint: "Keeps both: each conflict becomes the incoming lines followed by yours.", both: true },
  { take: "mine-theirs", label: "Take mine, then theirs", hint: "Keeps both: each conflict becomes your lines followed by the incoming ones.", both: true },
]

/** A conflict's two letters in words (git's "us" = HEAD). */
export const CONFLICT_KINDS = {
  UU: "both modified",
  AA: "both added",
  DD: "both deleted",
  DU: "deleted by us",
  UD: "deleted by them",
  AU: "added by us",
  UA: "added by them",
}

/** True when both sides still have the file (only then can they be combined). */
export const bothSides = (code) => code === "UU" || code === "AA"
