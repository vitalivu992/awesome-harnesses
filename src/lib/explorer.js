// Pure helpers for the Explorer panel (components/Explorer.jsx): git status
// labels, file-tree icons and ordering. Unit-tested in tests/explorer.test.mjs.

// The porcelain codes of an unresolved merge conflict.
const CONFLICTS = new Set(["DD", "AU", "UD", "UA", "DU", "AA", "UU"])

const STATUS_KINDS = {
  M: ["M", "modified", "Modified"],
  A: ["A", "added", "Added"],
  D: ["D", "deleted", "Deleted"],
  R: ["R", "renamed", "Renamed"],
  C: ["C", "copied", "Copied"],
  "?": ["U", "untracked", "Untracked"],
  "!": ["!", "conflict", "Conflict"],
}

/**
 * What to show for one status letter (x for a staged file, y for a change):
 * `{ letter, kind, label }`. Unknown letters ("T" typechange, …) show as M.
 */
export function statusMeta(letter) {
  return STATUS_KINDS[letter] || STATUS_KINDS.M
}

/** A status entry's display name: the file, with the old path of a rename. */
export function entryLabel(entry) {
  const name = String(entry?.path || "").replace(/\/$/, "")
  const base = name.split("/").pop() || name
  return entry?.orig ? `${base} (from ${entry.orig.split("/").pop()})` : base
}

/** The full path with the trailing slash of an untracked folder kept for git. */
export function entryPath(entry) {
  return String(entry?.path || "")
}

/** True when discard (restore the last committed content) can work: tracked in HEAD. */
export function canDiscard(entry) {
  return entry && entry.x !== "?" && entry.x !== "!"
}

/** Folders first, then files, names case-insensitive. */
export function sortEntries(entries) {
  return [...(entries || [])].sort((a, b) => {
    if (Boolean(a.dir) !== Boolean(b.dir)) return a.dir ? -1 : 1
    return String(a.name).localeCompare(String(b.name), undefined, { sensitivity: "base", numeric: true })
  })
}

const ICONS = {
  code: "fa-file-code",
  text: "fa-file-lines",
  image: "fa-file-image",
  audio: "fa-file-audio",
  video: "fa-file-video",
  archive: "fa-file-zipper",
  pdf: "fa-file-pdf",
  table: "fa-file-csv",
  data: "fa-file-code",
  font: "fa-font",
}

const EXT_KIND = {}
for (const ext of "js jsx ts tsx mjs cjs go py rb rs java c h cpp hpp cs php swift kt scala sh bash zsh fish vue svelte".split(" ")) EXT_KIND[ext] = "code"
for (const ext of "css scss sass less".split(" ")) EXT_KIND[ext] = "code"
for (const ext of "html htm xml svg md markdown txt log ini cfg conf env example".split(" ")) EXT_KIND[ext] = "text"
for (const ext of "json yaml yml toml lock edn".split(" ")) EXT_KIND[ext] = "data"
for (const ext of "png jpg jpeg gif webp bmp ico tiff".split(" ")) EXT_KIND[ext] = "image"
for (const ext of "mp3 wav ogg flac m4a".split(" ")) EXT_KIND[ext] = "audio"
for (const ext of "mp4 webm mkv mov avi".split(" ")) EXT_KIND[ext] = "video"
for (const ext of "zip tar gz bz2 xz 7z tgz".split(" ")) EXT_KIND[ext] = "archive"
EXT_KIND.pdf = "pdf"
EXT_KIND.csv = "table"
EXT_KIND.tsv = "table"
for (const ext of "ttf otf woff woff2".split(" ")) EXT_KIND[ext] = "font"

/** A Font Awesome file icon for a name ("" → the plain file icon). */
export function fileIcon(name) {
  const ext = String(name || "").split(".").pop()?.toLowerCase() || ""
  return ICONS[EXT_KIND[ext]] || "fa-file"
}

/** `dir/name` with duplicate slashes collapsed. */
export function joinPath(dir, name) {
  return `${String(dir || "").replace(/\/+$/, "")}/${name}`
}

/** "1.2 KB" for a file's size (folders show nothing). */
export function sizeLabel(bytes) {
  if (!bytes || bytes < 0) return ""
  const units = ["B", "KB", "MB", "GB", "TB"]
  let n = bytes
  let i = 0
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024
    i++
  }
  return `${n % 1 === 0 || n >= 100 ? Math.round(n) : n.toFixed(1)} ${units[i]}`
}

/**
 * Identifies what the viewer shows, so the tree and the git lists can mark
 * the open row: a file by its absolute path, a change by its side and path.
 */
export function viewerKey(target) {
  if (!target) return ""
  if (target.kind === "file") return `file:${target.path}`
  return `diff:${target.staged ? "staged" : "changes"}:${target.entry?.path || ""}`
}
