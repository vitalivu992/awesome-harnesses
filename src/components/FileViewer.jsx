import { useEffect, useMemo, useRef, useState } from "react"
import hljs from "highlight.js/lib/common"
import { fetchFileContent, fetchGitDiff, resolveConflict } from "../lib/api.js"
import { copyText } from "../lib/clipboard.js"
import { plural } from "../lib/format.js"
import { joinPath, sizeLabel } from "../lib/explorer.js"
import {
  CONFLICT_KINDS,
  RESOLUTIONS,
  bothSides,
  diffFileName,
  diffStats,
  isMarker,
  languageOf,
  lineCount,
  parseDiff,
  splitHighlighted,
  splitPath,
} from "../lib/viewer.js"
import { useFetch } from "../hooks/useFetch.js"

// Larger files are shown without colours.
const MAX_HIGHLIGHT = 300_000

/**
 * The viewer over the conversation, left of the Explorer: a file of the
 * Files tab (GitHub-style, line numbers + highlighting), or a change of the
 * Git tab as a unified diff. A file in a merge conflict gets the resolutions
 * in its header: Take theirs / Take mine, and in the … menu the two that
 * keep both sides. Esc closes it.
 *
 * base: the project's folder (paths under it are shown relative).
 * target: { kind: "file", path (absolute), notice? }
 *       | { kind: "diff", cwd, root, staged, entry }
 */
export function FileViewer({ target, base, onClose, onOpen, onResolved }) {
  const [rev, setRev] = useState(0)
  const isDiff = target.kind === "diff"
  const entry = isDiff ? target.entry : null
  const file = useFetch((signal) => fetchFileContent(target.path, { signal }), [target.path, rev], { enabled: !isDiff })
  const diff = useFetch((signal) => fetchGitDiff(target.cwd, entry, target.staged, { signal }), [target.cwd, entry?.path, entry?.x, target.staged, rev], {
    enabled: isDiff,
  })
  const root = useRef(null)

  useEffect(() => {
    const onKey = (e) => {
      if (e.key !== "Escape" || e.defaultPrevented) return
      // Esc in a text field elsewhere (the prompt box) belongs to that field.
      const t = e.target
      if (t instanceof HTMLElement && t.matches("input, textarea, [contenteditable]") && !root.current?.contains(t)) return
      onClose()
    }
    document.addEventListener("keydown", onKey)
    return () => document.removeEventListener("keydown", onKey)
  }, [onClose])

  const path = isDiff ? entry.path : target.path
  // A file of the project shows its folder relative to the project.
  const shownPath = !isDiff && base && path.startsWith(`${base.replace(/\/+$/, "")}/`) ? path.slice(base.replace(/\/+$/, "").length + 1) : path
  const { name, dir } = splitPath(shownPath)
  const conflict = isDiff
    ? entry.x === "!"
      ? { cwd: target.cwd, path: entry.path, both: bothSides(entry.conflict), code: entry.conflict }
      : null
    : file.data?.conflict || null
  const absPath = isDiff ? joinPath(target.root || target.cwd, entry.path) : target.path
  const badge = conflict ? "Conflict" : isDiff ? (entry.x === "?" ? "Untracked" : target.staged ? "Staged" : "Changes") : ""

  return (
    <section className="ah-viewer" id="ahViewer" ref={root} aria-label={`${isDiff ? "Diff of" : "File"} ${path}`} data-path={path} data-mode={isDiff ? "diff" : "file"}>
      <header className="ah-viewer__head">
        <div className="ah-viewer__title-row">
          <i className={`fa-solid ${isDiff ? "fa-code-compare" : "fa-file-code"} ah-viewer__icon`} aria-hidden="true"></i>
          <span className="ah-viewer__title" title={entry?.orig ? `${entry.orig} → ${path}` : absPath}>
            <strong>{name}</strong>
            <span className="ah-viewer__dir">{entry?.orig ? `from ${entry.orig}` : dir}</span>
          </span>
          {badge ? <span className={`ah-viewer__badge is-${badge.toLowerCase()}`}>{badge}</span> : null}
          <span className="ah-viewer__actions">
            {isDiff && entry.x !== "D" && entry.y !== "D" ? (
              <button type="button" className="ah-icon-btn" id="ahViewerOpenFile" onClick={() => onOpen({ kind: "file", path: absPath })} title="Open the file" aria-label="Open the file">
                <i className="fa-solid fa-file-lines" aria-hidden="true"></i>
              </button>
            ) : null}
            {!isDiff && file.data && !file.data.binary ? <CopyButton text={file.data.text} /> : null}
            <button type="button" className="ah-icon-btn" id="ahViewerReload" onClick={() => setRev((n) => n + 1)} title="Reload" aria-label="Reload">
              <i className="fa-solid fa-rotate-right" aria-hidden="true"></i>
            </button>
            <button type="button" className="ah-icon-btn" id="ahViewerClose" onClick={onClose} title="Close (Esc)" aria-label="Close the viewer">
              <i className="fa-solid fa-xmark" aria-hidden="true"></i>
            </button>
          </span>
        </div>
        {conflict ? (
          <ConflictBar
            key={`${conflict.cwd}:${conflict.path}`}
            conflict={conflict}
            onDone={(r) => {
              onResolved?.()
              onOpen({ kind: "file", path: absPath, notice: `Resolved with “${r.label}”. The file is staged.` })
            }}
          />
        ) : null}
        {target.notice ? (
          <p className="ah-viewer__notice" role="status">
            <i className="fa-solid fa-circle-check" aria-hidden="true"></i> {target.notice}
          </p>
        ) : null}
      </header>
      {isDiff ? <DiffBody state={diff} path={path} /> : <FileBody state={file} path={path} conflicted={Boolean(conflict)} />}
    </section>
  )
}

function CopyButton({ text }) {
  const [copied, setCopied] = useState(false)
  useEffect(() => {
    if (!copied) return undefined
    const t = setTimeout(() => setCopied(false), 1500)
    return () => clearTimeout(t)
  }, [copied])
  return (
    <button type="button" className="ah-icon-btn" id="ahViewerCopy" onClick={() => copyText(text).then(setCopied)} title={copied ? "Copied" : "Copy the file's contents"} aria-label="Copy the file's contents">
      <i className={`fa-solid ${copied ? "fa-check" : "fa-copy"}`} aria-hidden="true"></i>
    </button>
  )
}

/**
 * The resolutions of a conflicted file: Take theirs / Take mine as buttons,
 * the two that keep both sides in the … menu (only when both sides still
 * have the file). One click resolves and stages it.
 */
function ConflictBar({ conflict, onDone }) {
  const [busy, setBusy] = useState("")
  const [error, setError] = useState(null)
  const [menuOpen, setMenuOpen] = useState(false)
  const menuRoot = useRef(null)
  const menuButton = useRef(null)
  const menu = useRef(null)

  useEffect(() => {
    if (!menuOpen) return undefined
    menu.current?.querySelector("[role=menuitem]")?.focus()
    const outside = (e) => !menuRoot.current?.contains(e.target) && setMenuOpen(false)
    document.addEventListener("pointerdown", outside)
    return () => document.removeEventListener("pointerdown", outside)
  }, [menuOpen])

  const take = (r) => {
    setMenuOpen(false)
    setBusy(r.take)
    setError(null)
    resolveConflict(conflict.cwd, conflict.path, r.take).then(
      () => {
        setBusy("")
        onDone(r)
      },
      (err) => {
        setBusy("")
        setError(err)
      }
    )
  }

  const onMenuKey = (e) => {
    if (e.key === "Escape") {
      e.preventDefault()
      setMenuOpen(false)
      menuButton.current?.focus()
      return
    }
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return
    e.preventDefault()
    const items = [...menu.current.querySelectorAll("[role=menuitem]")]
    const i = items.indexOf(document.activeElement)
    items[(i + (e.key === "ArrowDown" ? 1 : -1) + items.length) % items.length]?.focus()
  }

  const [primary, advanced] = [RESOLUTIONS.filter((r) => !r.both), RESOLUTIONS.filter((r) => r.both)]
  return (
    <div className="ah-viewer__conflict" id="ahConflictBar">
      <span className="ah-viewer__conflict-kind">
        <i className="fa-solid fa-triangle-exclamation" aria-hidden="true"></i> Merge conflict
        {conflict.code && CONFLICT_KINDS[conflict.code] ? ` (${CONFLICT_KINDS[conflict.code]})` : ""}
      </span>
      <span className="ah-viewer__conflict-actions">
        {primary.map((r) => (
          <button key={r.take} type="button" className="ah-btn ah-btn--small" data-take={r.take} disabled={Boolean(busy)} onClick={() => take(r)} title={r.hint}>
            {busy === r.take ? <i className="fa-solid fa-spinner fa-spin" aria-hidden="true"></i> : null} {r.label}
          </button>
        ))}
        {conflict.both ? (
          <span className="ah-viewer__more" ref={menuRoot}>
            <button
              type="button"
              ref={menuButton}
              className={`ah-icon-btn${menuOpen ? " is-open" : ""}`}
              id="ahConflictMore"
              disabled={Boolean(busy)}
              onClick={() => setMenuOpen(!menuOpen)}
              title="More resolutions"
              aria-label="More resolutions"
              aria-haspopup="menu"
              aria-expanded={menuOpen}
            >
              {advanced.some((r) => r.take === busy) ? <i className="fa-solid fa-spinner fa-spin" aria-hidden="true"></i> : <i className="fa-solid fa-ellipsis" aria-hidden="true"></i>}
            </button>
            {menuOpen ? (
              <div className="ah-menu ah-menu--down" role="menu" aria-label="More resolutions" ref={menu} onKeyDown={onMenuKey}>
                {advanced.map((r) => (
                  <button key={r.take} type="button" role="menuitem" className="ah-menu__item" data-take={r.take} onClick={() => take(r)} title={r.hint}>
                    <i className="fa-solid fa-layer-group" aria-hidden="true"></i>
                    <span>{r.label}</span>
                  </button>
                ))}
              </div>
            ) : null}
          </span>
        ) : null}
      </span>
      {error ? (
        <p className="ah-viewer__conflict-error" role="alert">
          {error.message}
        </p>
      ) : null}
    </div>
  )
}

function FileBody({ state, path, conflicted }) {
  const f = state.data
  if (state.error) {
    return (
      <div className="ah-viewer__body">
        <p className={/no such file/i.test(state.error.message) ? "ah-empty" : "ah-error"}>
          {/no such file/i.test(state.error.message) ? "The file is not there (deleted)." : `Could not open the file: ${state.error.message}`}
        </p>
      </div>
    )
  }
  if (!f) return <Loading />
  return (
    <div className="ah-viewer__body" id="ahViewerFile">
      <p className="ah-viewer__info">
        {f.binary ? "Binary file" : plural(lineCount(f.text), "line")} · {sizeLabel(f.size) || "0 B"}
        {f.truncated ? ` · only the first ${sizeLabel(f.text.length)} shown` : ""}
      </p>
      {f.binary ? (
        <p className="ah-empty">A binary file: not shown.</p>
      ) : !f.text ? (
        <p className="ah-empty">An empty file.</p>
      ) : (
        <Code text={f.text} lang={f.text.length <= MAX_HIGHLIGHT ? languageOf(path) : ""} markers={conflicted} />
      )}
    </div>
  )
}

/** A file with line numbers, highlighted as a whole (comments spanning lines keep their colour). */
function Code({ text, lang, markers }) {
  const { html, raw } = useMemo(() => {
    const body = text.endsWith("\n") ? text.slice(0, -1) : text
    const raw = body.split("\n")
    const html = lang && hljs.getLanguage(lang) ? splitHighlighted(hljs.highlight(body, { language: lang, ignoreIllegals: true }).value) : raw.map(escape)
    return { html, raw }
  }, [text, lang])
  return (
    <table className="ah-code">
      <tbody>
        {html.map((line, i) => (
          <tr key={i} className={markers && isMarker(raw[i]) ? "is-marker" : undefined}>
            <td className="ah-code__no" data-line={i + 1}></td>
            <td className="ah-code__line" dangerouslySetInnerHTML={{ __html: line || "​" }} />
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function DiffBody({ state, path }) {
  const files = useMemo(() => parseDiff(state.data?.diff || ""), [state.data])
  if (state.error) {
    return (
      <div className="ah-viewer__body">
        <p className="ah-error">Could not read the diff: {state.error.message}</p>
      </div>
    )
  }
  if (!state.data) return <Loading />
  const stats = diffStats(files)
  return (
    <div className="ah-viewer__body" id="ahViewerDiff">
      <p className="ah-viewer__info">
        <span className="ah-viewer__add">+{stats.add}</span> <span className="ah-viewer__del">−{stats.del}</span>
        {files.length > 1 ? ` · ${plural(files.length, "file")}` : ""}
      </p>
      {!state.data.diff ? <p className="ah-empty">No textual changes (a mode change, an empty file, or already staged).</p> : null}
      {files.map((f, i) => (
        <DiffFile key={i} file={f} name={files.length > 1 ? diffFileName(f) : ""} lang={languageOf(diffFileName(f) || path)} />
      ))}
      {state.data.truncated ? <p className="ah-empty">The diff is too large and was cut.</p> : null}
    </div>
  )
}

function DiffFile({ file, name, lang }) {
  // The header lines worth showing: binary notices, mode changes, renames.
  const notes = file.header.filter((l) => /^(Binary files|old mode|new mode|deleted file|new file|similarity|rename from|rename to)/.test(l))
  const highlight = lang && hljs.getLanguage(lang)
  return (
    <table className="ah-code ah-diff">
      <tbody>
        {name ? (
          <tr className="is-file">
            <td colSpan={3}>
              <i className="fa-solid fa-file-lines" aria-hidden="true"></i> {name}
            </td>
          </tr>
        ) : null}
        {notes.map((l, i) => (
          <tr key={`n${i}`} className="is-meta">
            <td className="ah-code__no"></td>
            <td className="ah-code__no"></td>
            <td className="ah-code__line">{l}</td>
          </tr>
        ))}
        {file.hunks.map((h, hi) => [
          <tr key={`h${hi}`} className="is-hunk">
            <td className="ah-code__no"></td>
            <td className="ah-code__no"></td>
            <td className="ah-code__line">{h.header}</td>
          </tr>,
          ...h.lines.map((l, li) => (
            <tr key={`${hi}-${li}`} className={`is-${l.kind}${isMarker(l.text) ? " is-marker" : ""}`}>
              <td className="ah-code__no" data-line={l.old ?? ""}></td>
              <td className="ah-code__no" data-line={l.new ?? ""}></td>
              <td className="ah-code__line">
                <span className="ah-diff__mark" data-mark={l.kind === "add" ? "+" : l.kind === "del" ? "-" : " "}></span>
                {highlight && l.kind !== "meta" && !isMarker(l.text) ? (
                  <span dangerouslySetInnerHTML={{ __html: hljs.highlight(l.text, { language: lang, ignoreIllegals: true }).value || "​" }} />
                ) : (
                  <span>{l.text || "​"}</span>
                )}
              </td>
            </tr>
          )),
        ])}
      </tbody>
    </table>
  )
}

function Loading() {
  return (
    <div className="ah-viewer__body">
      <p className="ah-tree__hint">
        <i className="fa-solid fa-spinner fa-spin" aria-hidden="true"></i> Loading…
      </p>
    </div>
  )
}

function escape(s) {
  return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;")
}
