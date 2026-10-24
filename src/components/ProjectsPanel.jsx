import { useMemo, useState } from "react"
import { HARNESS_IDS, harnessName, plural, relativeTime } from "../lib/format.js"
import { readPrefs, writePref } from "../lib/prefs.js"
import { HarnessIcon } from "./HarnessIcon.jsx"
import { LiveIcon } from "./Running.jsx"
import { UserMenu } from "./UserMenu.jsx"

function initials(name) {
  const words = String(name || "?").split(/[^A-Za-z0-9]+/).filter(Boolean)
  return ((words[0]?.[0] || "?") + (words[1]?.[0] || "")).toUpperCase()
}

/** A stable hue per project, for the collapsed rail's badges. */
function hue(key) {
  let h = 0
  for (const ch of String(key)) h = (h * 31 + ch.charCodeAt(0)) % 360
  return h
}

/** "running" while an agent works in the project, "finished" once one just finished, else "". */
export function projectLive(project) {
  return project.running ? "running" : project.finished ? "finished" : ""
}

/** "2 working, 1 just finished": the project's live conversations, or "". */
export function liveLabel(project) {
  const parts = []
  if (project.running) parts.push(`${project.running} working`)
  if (project.finished) parts.push(`${project.finished} just finished`)
  return parts.join(", ")
}

export function filterProjects(projects, text) {
  const needle = String(text || "").trim().toLowerCase()
  if (!needle) return projects
  return projects.filter((p) => `${p.name} ${p.path}`.toLowerCase().includes(needle))
}

/** A project whose folder is gone (projects without a folder, like no-workspace, aren't). */
export function isMissing(project) {
  return Boolean(project.path) && !project.exists
}

/**
 * The projects the panel lists: all of them with `showMissing`, else those
 * whose folder still exists, plus the selected one so it never vanishes.
 */
export function listedProjects(projects, { showMissing = false, selectedKey = "" } = {}) {
  if (showMissing) return projects
  return projects.filter((p) => !isMissing(p) || p.key === selectedKey)
}

/**
 * First column: every project with agent conversations, with a count per harness. Projects
 * whose folder no longer exists are hidden until the count in the header is clicked; projects
 * an agent is working in, or has just finished in, carry a live icon. Collapses to a rail
 * of initials badges.
 */
export function ProjectsPanel({
  projects,
  loading,
  error,
  selectedKey,
  onSelect,
  collapsed,
  onToggle,
  onRefresh,
  user,
  onLogout,
  onOpenSettings,
  onOpenShell,
}) {
  const [text, setText] = useState("")
  const [showMissing, setShowMissing] = useState(() => Boolean(readPrefs().showMissingProjects))
  const listed = useMemo(
    () => listedProjects(projects || [], { showMissing, selectedKey }),
    [projects, showMissing, selectedKey]
  )
  const shown = useMemo(() => filterProjects(listed, text), [listed, text])
  const missing = (projects || []).filter(isMissing).length
  const toggleMissing = () => {
    writePref("showMissingProjects", !showMissing)
    setShowMissing(!showMissing)
  }

  if (collapsed) {
    return (
      <nav className="ah-projects ah-projects--collapsed" aria-label="Projects">
        <button type="button" className="ah-icon-btn" onClick={onToggle} title="Expand projects" aria-label="Expand projects">
          <i className="fa-solid fa-angles-right" aria-hidden="true"></i>
        </button>
        <ul className="ah-rail">
          {listed.map((p) => {
            const live = projectLive(p)
            return (
              <li key={p.key}>
                <button
                  type="button"
                  className={`ah-rail__item${p.key === selectedKey ? " is-active" : ""}${live ? ` is-${live}` : ""}`}
                  style={{ "--ah-hue": hue(p.key) }}
                  title={live ? `${p.path || p.name}\n${liveLabel(p)}` : p.path || p.name}
                  aria-label={live ? `${p.name}, ${liveLabel(p)}` : p.name}
                  aria-current={p.key === selectedKey ? "true" : undefined}
                  onClick={() => onSelect(p.key)}
                >
                  {initials(p.name)}
                  {live ? <span className={`ah-rail__live ah-live--${live}`} aria-hidden="true"></span> : null}
                </button>
              </li>
            )
          })}
        </ul>
        <UserMenu user={user} onLogout={onLogout} onOpenSettings={onOpenSettings} compact />
      </nav>
    )
  }

  return (
    <nav className="ah-projects" aria-label="Projects">
      <header className="ah-panel-head">
        <div className="ah-brand">
          <i className="fa-solid fa-layer-group" aria-hidden="true"></i>
          <span>Projects</span>
          {projects && missing ? (
            <button
              type="button"
              className="ah-count ah-count--toggle"
              id="ahProjectsCount"
              onClick={toggleMissing}
              aria-pressed={showMissing}
              title={
                showMissing
                  ? `${plural(projects.length, "project")}. Click to hide the ${missing} whose folder no longer exists`
                  : `${plural(listed.length, "project")}. Click to also show ${missing} whose folder no longer exists`
              }
            >
              {listed.length}
            </button>
          ) : projects ? (
            <span className="ah-count" id="ahProjectsCount">
              {projects.length}
            </span>
          ) : null}
        </div>
        <div className="ah-panel-head__actions">
          <button type="button" className="ah-icon-btn" onClick={onRefresh} title="Reload" aria-label="Reload projects">
            <i className={`fa-solid fa-rotate${loading ? " fa-spin" : ""}`} aria-hidden="true"></i>
          </button>
          <button type="button" className="ah-icon-btn" onClick={onToggle} title="Collapse projects" aria-label="Collapse projects">
            <i className="fa-solid fa-angles-left" aria-hidden="true"></i>
          </button>
        </div>
      </header>
      <div className="ah-search">
        <i className="fa-solid fa-magnifying-glass" aria-hidden="true"></i>
        <input
          type="search"
          placeholder="Filter projects"
          aria-label="Filter projects"
          value={text}
          onChange={(e) => setText(e.target.value)}
        />
      </div>
      {error ? <p className="ah-error">Could not load projects: {error.message}</p> : null}
      <ul className="ah-list" id="ahProjects">
        {shown.map((p) => {
          const live = projectLive(p)
          return (
            <li key={p.key} className="ah-project-row">
              <button
                type="button"
                className={`ah-project${p.key === selectedKey ? " is-active" : ""}${p.exists ? "" : " is-missing"}${live ? ` is-${live}` : ""}`}
                aria-current={p.key === selectedKey ? "true" : undefined}
                title={p.exists ? p.path : `${p.path || p.name} (folder no longer exists)`}
                data-project={p.key}
                onClick={() => onSelect(p.key)}
              >
                <span className="ah-project__top">
                  <span className="ah-project__name">{p.name}</span>
                  <LiveIcon state={live} count={p.running || p.finished} title={liveLabel(p)} />
                  <span className="ah-project__when">{relativeTime(p.updatedAt)}</span>
                </span>
                {p.path ? (
                  <span className="ah-project__path">
                    <bdi>{p.path}</bdi>
                  </span>
                ) : null}
                <span className="ah-project__counts">
                  {HARNESS_IDS.filter((h) => p.counts?.[h]).map((h) => (
                    <span key={h} title={`${p.counts[h]} ${harnessName(h)} conversations`}>
                      <HarnessIcon harness={h} size={12} /> {p.counts[h]}
                    </span>
                  ))}
                </span>
              </button>
              {p.exists && p.path ? (
                <button
                  type="button"
                  className="ah-icon-btn ah-project__shell"
                  onClick={() => onOpenShell(p)}
                  title={`Open a shell in ${p.path}`}
                  aria-label={`Open a shell in ${p.name}`}
                >
                  <i className="fa-solid fa-terminal" aria-hidden="true"></i>
                </button>
              ) : null}
            </li>
          )
        })}
        {projects && !shown.length ? <li className="ah-empty">No projects match.</li> : null}
      </ul>
      <footer className="ah-panel-foot">
        <UserMenu user={user} onLogout={onLogout} onOpenSettings={onOpenSettings} />
      </footer>
    </nav>
  )
}
