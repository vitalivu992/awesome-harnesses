import { useMemo, useState } from "react"
import { HARNESS_IDS, harnessName, longDateTime, modelLabel, plural, relativeTime, shortDateTime, statusMeta } from "../lib/format.js"
import { liveState } from "../lib/live.js"
import { ClientBadge, HarnessIcon } from "./HarnessIcon.jsx"
import { LiveIcon } from "./Running.jsx"

/** Conversations of one harness ("" = any) whose title or model contains `text`. */
export function filterConversations(conversations, { harness = "", text = "" } = {}) {
  const needle = String(text || "").trim().toLowerCase()
  return (conversations || []).filter((c) => {
    if (harness && c.harness !== harness) return false
    if (!needle) return true
    return [c.title, c.id, c.gitBranch, ...(c.models || []).map(modelLabel)].some((v) =>
      String(v || "").toLowerCase().includes(needle)
    )
  })
}

/**
 * Second column: the selected project's conversations, newest first. The ones an agent is
 * working on, or has just finished, carry a live icon.
 */
export function ConversationList({ project, conversations, loading, error, selectedId, onSelect, onRefresh }) {
  const [text, setText] = useState("")
  const [harness, setHarness] = useState("")
  const shown = useMemo(() => filterConversations(conversations, { harness, text }), [conversations, harness, text])
  const counts = useMemo(() => {
    const c = { "": conversations?.length || 0 }
    for (const conv of conversations || []) c[conv.harness] = (c[conv.harness] || 0) + 1
    return c
  }, [conversations])
  // "All" plus the harnesses this project has conversations in.
  const filters = ["", ...HARNESS_IDS.filter((h) => counts[h])]

  if (!project) {
    return (
      <section className="ah-convs" aria-label="Conversations">
        <p className="ah-empty ah-empty--center">Select a project.</p>
      </section>
    )
  }

  return (
    <section className="ah-convs" aria-label="Conversations">
      <header className="ah-panel-head">
        <div className="ah-convs__project" title={project.path}>
          <span className="ah-convs__name">{project.name}</span>
          {project.path ? (
            <span className="ah-convs__path">
              <bdi>{project.path}</bdi>
            </span>
          ) : null}
        </div>
        <button type="button" className="ah-icon-btn" onClick={onRefresh} title="Reload" aria-label="Reload conversations">
          <i className={`fa-solid fa-rotate${loading ? " fa-spin" : ""}`} aria-hidden="true"></i>
        </button>
      </header>
      <div className="ah-search">
        <i className="fa-solid fa-magnifying-glass" aria-hidden="true"></i>
        <input
          type="search"
          placeholder="Filter by title, model, branch"
          aria-label="Filter conversations"
          value={text}
          onChange={(e) => setText(e.target.value)}
        />
      </div>
      <div className="ah-segmented ah-segmented--harness" role="group" aria-label="Harness">
        {filters.map((h) => {
          // "All" and the selected harness show name and count; the others only their icon.
          const full = !h || harness === h
          const label = h ? harnessName(h) : "All"
          return (
            <button
              key={h || "all"}
              type="button"
              className={`${harness === h ? "is-active" : ""}${full ? "" : " is-icon"}`}
              aria-pressed={harness === h}
              aria-label={`${label} ${counts[h] || 0}`}
              title={`${label}: ${plural(counts[h] || 0, "conversation")}`}
              onClick={() => setHarness(h)}
            >
              {h ? <HarnessIcon harness={h} size={12} /> : null}
              {full ? (
                <>
                  {label}
                  <span className="ah-count">{counts[h] || 0}</span>
                </>
              ) : null}
            </button>
          )
        })}
      </div>
      {error ? <p className="ah-error">Could not load conversations: {error.message}</p> : null}
      {loading && !conversations ? <p className="ah-empty">Loading conversations…</p> : null}
      <ul className="ah-list" id="ahConversations">
        {shown.map((c) => {
          const key = `${c.harness}:${c.id}`
          const model = modelLabel(c.models?.[c.models.length - 1])
          const live = liveState(c)
          return (
            <li key={key}>
              <button
                type="button"
                className={`ah-conv${key === selectedId ? " is-active" : ""}${c.status === "error" ? " is-error" : ""}${live ? ` is-${live}` : ""}`}
                aria-current={key === selectedId ? "true" : undefined}
                data-conversation={key}
                onClick={() => onSelect(key)}
              >
                <span className="ah-conv__top">
                  <span className="ah-conv__title">{c.title}</span>
                  <LiveIcon state={live} title={live === "finished" ? "The agent just finished" : "The agent is working"} />
                  {c.status === "error" ? (
                    <i
                      className="fa-solid fa-circle-exclamation ah-conv__issue"
                      title={statusMeta("error").label}
                      aria-label={statusMeta("error").label}
                      role="img"
                    ></i>
                  ) : null}
                </span>
                <span className="ah-conv__meta">
                  <span className="ah-conv__agent" title={harnessName(c.harness)}>
                    <HarnessIcon harness={c.harness} size={12} />
                    {model || harnessName(c.harness)}
                  </span>
                  <ClientBadge conversation={c} />
                  <span title={`${plural(c.turns, "prompt")}, ${plural(c.toolCalls, "tool call")}`}>
                    <i className="fa-regular fa-comment" aria-hidden="true"></i> {c.turns}
                    <i className="fa-solid fa-wrench ah-conv__tools" aria-hidden="true"></i> {c.toolCalls}
                  </span>
                  <time
                    className="ah-conv__when"
                    dateTime={c.updatedAt}
                    title={`Started ${longDateTime(c.startedAt)}\nUpdated ${longDateTime(c.updatedAt)} (${relativeTime(c.updatedAt)})`}
                  >
                    {shortDateTime(c.updatedAt)}
                  </time>
                </span>
              </button>
            </li>
          )
        })}
        {conversations && !shown.length ? <li className="ah-empty">No conversations match.</li> : null}
      </ul>
    </section>
  )
}
