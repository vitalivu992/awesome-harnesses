import { useCallback, useEffect, useState } from "react"
import {
  commitStaged,
  discardChanges,
  fetchDir,
  fetchGitLog,
  fetchGitStatus,
  stageFiles,
  unstageFiles,
} from "../lib/api.js"
import { readPrefs, writePref } from "../lib/prefs.js"
import { plural, relativeTime } from "../lib/format.js"
import { canDiscard, entryLabel, entryPath, fileIcon, joinPath, sizeLabel, sortEntries, statusMeta, viewerKey } from "../lib/explorer.js"
import { useFetch } from "../hooks/useFetch.js"

/**
 * One folder of the file tree: its entries load when it is expanded (the
 * root when the Files tab opens), each subfolder the same way again.
 */
function TreeDir({ path, name, depth, dirs, toggle, projectKey, onOpen, selected }) {
  const state = dirs[path]
  const open = Boolean(state?.open)
  const entries = state?.entries
  const childKey = `${projectKey}:${path}`
  return (
    <li className="ah-tree__node">
      <button
        type="button"
        className={`ah-tree__row${open ? " is-open" : ""}`}
        style={{ "--ah-depth": depth }}
        aria-expanded={open}
        onClick={() => toggle(path)}
        title={path}
      >
        <i className={`fa-solid ${open ? "fa-chevron-down" : "fa-chevron-right"} ah-tree__chev`} aria-hidden="true"></i>
        <i className={`fa-solid ${open ? "fa-folder-open" : "fa-folder"} ah-tree__icon`} aria-hidden="true"></i>
        <span className="ah-tree__name">{name}</span>
      </button>
      {open ? (
        state.error ? (
          <p className="ah-tree__hint ah-tree__hint--error" style={{ "--ah-depth": depth + 1 }}>
            {state.error.message}
          </p>
        ) : !entries ? (
          <p className="ah-tree__hint" style={{ "--ah-depth": depth + 1 }}>
            <i className="fa-solid fa-spinner fa-spin" aria-hidden="true"></i> Loading…
          </p>
        ) : entries.length ? (
          <ul className="ah-tree">
            {entries.map((e) =>
              e.dir ? (
                <TreeDir
                  key={childKey + e.name}
                  path={joinPath(path, e.name)}
                  name={e.name}
                  depth={depth + 1}
                  dirs={dirs}
                  toggle={toggle}
                  projectKey={projectKey}
                  onOpen={onOpen}
                  selected={selected}
                />
              ) : (
                <TreeFile key={childKey + e.name} path={joinPath(path, e.name)} entry={e} depth={depth + 1} onOpen={onOpen} selected={selected} />
              )
            )}
          </ul>
        ) : (
          <p className="ah-tree__hint" style={{ "--ah-depth": depth + 1 }}>
            (empty)
          </p>
        )
      ) : null}
    </li>
  )
}

/** A file of the tree: a click opens it in the viewer. */
function TreeFile({ path, entry, depth, onOpen, selected }) {
  const target = { kind: "file", path }
  const isSelected = selected === viewerKey(target)
  return (
    <li className="ah-tree__node">
      <button
        type="button"
        className={`ah-tree__row ah-tree__file${isSelected ? " is-selected" : ""}`}
        style={{ "--ah-depth": depth }}
        title={path}
        aria-current={isSelected ? "true" : undefined}
        onClick={() => onOpen(target)}
      >
        <span className="ah-tree__chev" aria-hidden="true"></span>
        <i className={`fa-solid ${fileIcon(entry.name)} ah-tree__icon`} aria-hidden="true"></i>
        <span className="ah-tree__name">{entry.name}</span>
        <span className="ah-tree__size">{sizeLabel(entry.size)}</span>
      </button>
    </li>
  )
}

function FilesTab({ cwd, projectKey, onOpen, selected }) {
  // path → {open, entries, error, loading}: loaded folders stay loaded while the tab is open.
  const [dirs, setDirs] = useState({})
  useEffect(() => setDirs({}), [projectKey])

  const fetchInto = useCallback((path, open) => {
    setDirs((d) => ({ ...d, [path]: { ...d[path], open, loading: true } }))
    fetchDir(path).then(
      (entries) => setDirs((d) => ({ ...d, [path]: { open, entries: sortEntries(entries) } })),
      (error) => setDirs((d) => ({ ...d, [path]: { open, error } }))
    )
  }, [])

  const toggle = useCallback(
    (path) => {
      setDirs((d) => {
        const state = d[path] || {}
        const open = !state.open
        if (open && !state.entries && !state.error && !state.loading) fetchInto(path, open)
        return { ...d, [path]: { ...state, open } }
      })
    },
    [fetchInto]
  )

  // The root folder loads when the tab opens.
  useEffect(() => {
    if (cwd) fetchInto(cwd, true)
  }, [cwd, fetchInto])

  const root = dirs[cwd]
  const rootName = cwd.split("/").filter(Boolean).pop() || cwd

  return (
    <div className="ah-explorer__scroll" id="ahExplorerFiles">
      {!cwd ? (
        <p className="ah-empty">This project has no folder on this machine.</p>
      ) : root?.error ? (
        <p className="ah-error">Could not list the folder: {root.error.message}</p>
      ) : root?.entries ? (
        <ul className="ah-tree">
          <TreeDir path={cwd} name={rootName} depth={0} dirs={dirs} toggle={toggle} projectKey={projectKey} onOpen={onOpen} selected={selected} />
        </ul>
      ) : (
        <p className="ah-tree__hint">
          <i className="fa-solid fa-spinner fa-spin" aria-hidden="true"></i> Loading…
        </p>
      )}
    </div>
  )
}

/** One file under Staged or Changes: status chip, name, hover actions. */
function GitRow({ entry, side, cwd, root, onAction, busy, onOpen, selected }) {
  const [confirming, setConfirming] = useState(false)
  const [letter, kind, label] = statusMeta(side === "staged" ? entry.x : entry.y)
  const path = entryPath(entry)
  const name = entryLabel(entry)
  const target = { kind: "diff", cwd, root, staged: side === "staged", entry }
  const isSelected = selected === viewerKey(target)
  const run = (fn) => () => {
    setConfirming(false)
    onAction(fn)
  }
  return (
    <li className={`ah-git__row ah-git__row--${kind}${isSelected ? " is-selected" : ""}`} data-path={path}>
      <span className={`ah-git__letter ah-git__letter--${kind}`} title={label}>
        {letter}
      </span>
      <button
        type="button"
        className="ah-git__path"
        title={`${entry.orig ? `${entry.orig} → ${path}` : path}\nOpen the diff`}
        aria-current={isSelected ? "true" : undefined}
        onClick={() => onOpen(target)}
      >
        {name}
      </button>
      {side === "staged" ? (
        <button
          type="button"
          className="ah-icon-btn"
          disabled={busy}
          onClick={run(() => unstageFiles(cwd, [path]))}
          title="Unstage (back to Changes)"
          aria-label={`Unstage ${name}`}
        >
          <i className="fa-solid fa-minus" aria-hidden="true"></i>
        </button>
      ) : confirming ? (
        <span className="ah-git__confirm">
          <button type="button" className="ah-btn ah-btn--small" disabled={busy} onClick={run(() => discardChanges(cwd, [path]))}>
            Discard
          </button>
          <button type="button" className="ah-btn ah-btn--small" onClick={() => setConfirming(false)}>
            Keep
          </button>
        </span>
      ) : (
        <span className="ah-git__actions">
          {canDiscard(entry) ? (
            <button
              type="button"
              className="ah-icon-btn"
              disabled={busy}
              onClick={() => setConfirming(true)}
              title="Discard changes (restore the last committed content)"
              aria-label={`Discard changes in ${name}`}
            >
              <i className="fa-solid fa-rotate-left" aria-hidden="true"></i>
            </button>
          ) : null}
          <button
            type="button"
            className="ah-icon-btn"
            disabled={busy}
            onClick={run(() => stageFiles(cwd, [path]))}
            title="Stage (to Staged)"
            aria-label={`Stage ${name}`}
          >
            <i className="fa-solid fa-plus" aria-hidden="true"></i>
          </button>
        </span>
      )}
    </li>
  )
}

function GitTab({ cwd, projectKey, onOpen, selected, version }) {
  const [bump, setBump] = useState(0)
  const [busy, setBusy] = useState(false)
  const [actionError, setActionError] = useState(null)
  const [message, setMessage] = useState("")
  const [committed, setCommitted] = useState("")
  // The status refreshes itself: a running agent changes files under it.
  const status = useFetch((signal) => fetchGitStatus(cwd, { signal }), [cwd, projectKey, bump, version], {
    enabled: Boolean(cwd),
    poll: () => 5000,
  })
  const log = useFetch((signal) => fetchGitLog(cwd, { signal }), [cwd, projectKey, bump, version], { enabled: Boolean(cwd) })
  const data = status.data
  const staged = data?.staged || []
  const changes = data?.changes || []

  const refresh = useCallback(() => setBump((n) => n + 1), [])
  const onAction = useCallback(
    (fn) => {
      setBusy(true)
      setActionError(null)
      fn().then(
        () => {
          setBusy(false)
          refresh()
        },
        (err) => {
          setBusy(false)
          setActionError(err)
        }
      )
    },
    [refresh]
  )

  const commit = () => {
    setBusy(true)
    setActionError(null)
    commitStaged(cwd, message)
      .then(({ commit: hash }) => {
        setBusy(false)
        setMessage("")
        setCommitted(hash)
        setTimeout(() => setCommitted(""), 4000)
        refresh()
      })
      .catch((err) => {
        setBusy(false)
        setActionError(err)
      })
  }

  if (!cwd) {
    return (
      <div className="ah-explorer__scroll" id="ahExplorerGit">
        <p className="ah-empty">This project has no folder on this machine.</p>
      </div>
    )
  }
  return (
    <div className="ah-explorer__scroll" id="ahExplorerGit">
      {status.error ? (
        status.error.message.includes("not a git repository") ? (
          <p className="ah-empty">
            <i className="fa-solid fa-code-branch" aria-hidden="true"></i> {cwd.split("/").filter(Boolean).pop()} is not in a git repository.
          </p>
        ) : (
          <p className="ah-error">Could not read the git status: {status.error.message}</p>
        )
      ) : (
        <>
          <p className="ah-git__branch" title={cwd}>
            <i className="fa-solid fa-code-branch" aria-hidden="true"></i> {data?.branch || "…"}
          </p>

          <h4 className="ah-git__head">
            Staged <span className="ah-count">{staged.length}</span>
          </h4>
          {staged.length ? (
            <ul className="ah-git__list">
              {staged.map((e) => (
                <GitRow key={`s:${e.path}`} entry={e} side="staged" cwd={cwd} root={data.root} onAction={onAction} busy={busy} onOpen={onOpen} selected={selected} />
              ))}
            </ul>
          ) : (
            <p className="ah-git__empty">Nothing staged. Stage files from Changes below.</p>
          )}

          <h4 className="ah-git__head">
            Changes <span className="ah-count">{changes.length}</span>
          </h4>
          {changes.length ? (
            <ul className="ah-git__list">
              {changes.map((e) => (
                <GitRow key={`c:${e.path}`} entry={e} side="changes" cwd={cwd} root={data.root} onAction={onAction} busy={busy} onOpen={onOpen} selected={selected} />
              ))}
            </ul>
          ) : (
            <p className="ah-git__empty">No changes. Working tree clean.</p>
          )}

          <div className="ah-git__commit">
            <textarea
              className="ah-git__message"
              placeholder={`Commit message (${plural(staged.length, "file")} staged)`}
              value={message}
              rows={2}
              spellCheck="false"
              disabled={!staged.length}
              onChange={(e) => setMessage(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && (e.metaKey || e.ctrlKey) && message.trim() && staged.length) commit()
              }}
            />
            <button
              type="button"
              className="ah-btn ah-btn--small ah-btn--primary"
              disabled={busy || !message.trim() || !staged.length}
              onClick={commit}
            >
              <i className={`fa-solid ${busy ? "fa-spinner fa-spin" : "fa-check"}`} aria-hidden="true"></i> Commit
            </button>
          </div>
          {committed ? (
            <p className="ah-git__done">
              <i className="fa-solid fa-check" aria-hidden="true"></i> Committed as <code>{committed}</code>
            </p>
          ) : null}
          {actionError ? (
            <p className="ah-error" role="alert">
              {actionError.message}
            </p>
          ) : null}

          <h4 className="ah-git__head">
            <i className="fa-solid fa-clock-rotate-left" aria-hidden="true"></i> Recent commits
          </h4>
          {log.error ? (
            <p className="ah-git__empty">No commits yet.</p>
          ) : log.data?.length ? (
            <ul className="ah-git__log">
              {log.data.map((c) => (
                <li key={c.hash} className="ah-git__commit-row" title={`${c.subject}\n${c.author}`}>
                  <code className="ah-git__hash">{c.hash}</code>
                  <span className="ah-git__subject">{c.subject}</span>
                  <time dateTime={c.date} title={c.date}>
                    {relativeTime(c.date)}
                  </time>
                </li>
              ))}
            </ul>
          ) : (
            <p className="ah-tree__hint">
              <i className="fa-solid fa-spinner fa-spin" aria-hidden="true"></i> Loading…
            </p>
          )}
        </>
      )}
    </div>
  )
}

/**
 * The Explorer on the right: nothing when closed (the default; the button
 * that opens it is on the terminal bar), and when open the selected
 * project's file tree and git state —
 * stage, unstage, discard and commit, with the log under it. Like VS Code's
 * explorer, one of Files / Git is shown at a time. A file (Files) or a change
 * (Git) opens in the viewer through `onOpen(target)`; `selected` is the open
 * one's `viewerKey`, `gitVersion` reloads the git state (a conflict resolved).
 */
export function Explorer({ project, open, onToggle, onOpen, selected, gitVersion }) {
  const [tab, setTab] = useState(() => (readPrefs().explorerTab === "git" ? "git" : "files"))
  const cwd = project?.exists && project?.path ? project.path : ""
  const projectKey = project?.key || ""
  const selectTab = (t) => {
    writePref("explorerTab", t)
    setTab(t)
  }

  // Closed, it takes no room: the terminal bar has its button.
  if (!open) return null

  return (
    <aside className="ah-explorer" id="ahExplorer" aria-labelledby="ahExplorerFilesTab">
      <header className="ah-panel-head ah-explorer__head">
        <div className="ah-explorer__tabs" role="tablist" aria-label="Explorer views">
          <button
            type="button"
            role="tab"
            id="ahExplorerFilesTab"
            className={`ah-explorer__tab${tab === "files" ? " is-active" : ""}`}
            aria-selected={tab === "files"}
            onClick={() => selectTab("files")}
          >
            <i className="fa-solid fa-folder-tree" aria-hidden="true"></i> Files
          </button>
          <button
            type="button"
            role="tab"
            id="ahExplorerGitTab"
            className={`ah-explorer__tab${tab === "git" ? " is-active" : ""}`}
            aria-selected={tab === "git"}
            onClick={() => selectTab("git")}
          >
            <i className="fa-solid fa-code-branch" aria-hidden="true"></i> Git
          </button>
        </div>
        <div className="ah-panel-head__actions">
          <button type="button" className="ah-icon-btn" id="ahExplorerClose" onClick={onToggle} title="Hide the explorer" aria-label="Hide the explorer">
            <i className="fa-solid fa-angles-right" aria-hidden="true"></i>
          </button>
        </div>
      </header>
      {tab === "files" ? (
        <FilesTab cwd={cwd} projectKey={projectKey} onOpen={onOpen} selected={selected} />
      ) : (
        <GitTab cwd={cwd} projectKey={projectKey} onOpen={onOpen} selected={selected} version={gitVersion} />
      )}
    </aside>
  )
}
