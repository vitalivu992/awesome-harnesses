import { useMemo, useState } from "react"
import { HARNESS_IDS, harnessName, relativeTime } from "../lib/format.js"
import { HarnessIcon } from "./HarnessIcon.jsx"
import { ThemeToggle } from "./ThemeToggle.jsx"

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

export function filterProjects(projects, text) {
  const needle = String(text || "").trim().toLowerCase()
  if (!needle) return projects
  return projects.filter((p) => `${p.name} ${p.path}`.toLowerCase().includes(needle))
}

/**
 * First column: every project with agent conversations, with a count per harness. Collapses to
 * a rail of initials badges.
 */
export function ProjectsPanel({ projects, loading, error, selectedKey, onSelect, collapsed, onToggle, onRefresh }) {
  const [text, setText] = useState("")
  const shown = useMemo(() => filterProjects(projects || [], text), [projects, text])

  if (collapsed) {
    return (
      <nav className="ah-projects ah-projects--collapsed" aria-label="Projects">
        <button type="button" className="ah-icon-btn" onClick={onToggle} title="Expand projects" aria-label="Expand projects">
          <i className="fa-solid fa-angles-right" aria-hidden="true"></i>
        </button>
        <ul className="ah-rail">
          {(projects || []).map((p) => (
            <li key={p.key}>
              <button
                type="button"
                className={`ah-rail__item${p.key === selectedKey ? " is-active" : ""}`}
                style={{ "--ah-hue": hue(p.key) }}
                title={p.path || p.name}
                aria-label={p.name}
                aria-current={p.key === selectedKey ? "true" : undefined}
                onClick={() => onSelect(p.key)}
              >
                {initials(p.name)}
              </button>
            </li>
          ))}
        </ul>
        <ThemeToggle compact />
      </nav>
    )
  }

  return (
    <nav className="ah-projects" aria-label="Projects">
      <header className="ah-panel-head">
        <div className="ah-brand">
          <i className="fa-solid fa-layer-group" aria-hidden="true"></i>
          <span>Projects</span>
          {projects ? <span className="ah-count">{projects.length}</span> : null}
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
        {shown.map((p) => (
          <li key={p.key}>
            <button
              type="button"
              className={`ah-project${p.key === selectedKey ? " is-active" : ""}${p.exists ? "" : " is-missing"}`}
              aria-current={p.key === selectedKey ? "true" : undefined}
              title={p.exists ? p.path : `${p.path || p.name} (folder no longer exists)`}
              data-project={p.key}
              onClick={() => onSelect(p.key)}
            >
              <span className="ah-project__top">
                <span className="ah-project__name">{p.name}</span>
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
          </li>
        ))}
        {projects && !shown.length ? <li className="ah-empty">No projects match.</li> : null}
      </ul>
      <footer className="ah-panel-foot">
        <ThemeToggle />
      </footer>
    </nav>
  )
}
