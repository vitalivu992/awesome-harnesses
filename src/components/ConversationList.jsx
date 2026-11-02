import { useEffect, useMemo, useRef, useState } from "react"
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

/** Rows per page: drawing a long list in full made the UI lag. */
export const PAGE_SIZE = 50

/**
 * Page `page` (0-based, clamped to the last page) of `conversations`: its rows, the page
 * number used, the page count, and the 0-based index of its first row.
 */
export function pageConversations(conversations, page) {
  const list = conversations || []
  const pages = Math.max(1, Math.ceil(list.length / PAGE_SIZE))
  const p = Math.min(Math.max(0, page), pages - 1)
  const start = p * PAGE_SIZE
  return { visible: list.slice(start, start + PAGE_SIZE), page: p, pages, start, total: list.length }
}

/** The page that holds the conversation `key` (`harness:id`), or -1 when it isn't listed. */
export function pageOf(conversations, key) {
  const at = (conversations || []).findIndex((c) => `${c.harness}:${c.id}` === key)
  return at < 0 ? -1 : Math.floor(at / PAGE_SIZE)
}

/**
 * Second column: the selected project's conversations, newest first. The ones an agent is
 * working on, or has just finished, carry a live icon. "New" starts a conversation with
 * the harness the filter shows (`onNew(harness)`, "" for All).
 */
export function ConversationList({ project, conversations, loading, error, selectedId, onSelect, onRefresh, onNew, creating = false }) {
  const [text, setText] = useState("")
  const [harness, setHarness] = useState("")
  const [pageNo, setPageNo] = useState(0)
  const listRef = useRef(null)
  // The selection last brought into view: each one is revealed once, so the list polls
  // don't pull the user back while they page.
  const revealed = useRef("")
  const shown = useMemo(() => filterConversations(conversations, { harness, text }), [conversations, harness, text])
  // Back to the first page when the project or the filter changes.
  useEffect(() => {
    setPageNo(0)
    revealed.current = ""
  }, [project?.key])
  useEffect(() => setPageNo(0), [harness, text])
  // Open the page that holds the selected conversation (a deep link to an old one).
  useEffect(() => {
    if (!selectedId || revealed.current === selectedId) return
    const p = pageOf(shown, selectedId)
    if (p < 0) return
    revealed.current = selectedId
    setPageNo(p)
  }, [shown, selectedId])
  const page = useMemo(() => pageConversations(shown, pageNo), [shown, pageNo])
  const goTo = (p) => {
    setPageNo(p)
    if (listRef.current) listRef.current.scrollTop = 0
  }
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
        <div className="ah-panel-head__actions">
          <button type="button" className="ah-icon-btn" onClick={onRefresh} title="Reload" aria-label="Reload conversations">
            <i className={`fa-solid fa-rotate${loading ? " fa-spin" : ""}`} aria-hidden="true"></i>
          </button>
          <button
            type="button"
            className={`ah-icon-btn ah-new-btn${creating ? " is-open" : ""}`}
            id="ahNewConversationBtn"
            onClick={() => onNew?.(harness)}
            disabled={!project.exists}
            title={
              project.exists
                ? `New ${harness ? `${harnessName(harness)} ` : ""}conversation in ${project.name}`
                : "The project's folder doesn't exist any more"
            }
            aria-label="New conversation"
          >
            <i className="fa-regular fa-pen-to-square" aria-hidden="true"></i>
          </button>
        </div>
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
      <ul className="ah-list" id="ahConversations" ref={listRef}>
        {page.visible.map((c) => {
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
      {page.pages > 1 ? (
        <nav className="ah-pager" aria-label="Conversation pages">
          <button
            type="button"
            className="ah-icon-btn"
            onClick={() => goTo(page.page - 1)}
            disabled={page.page === 0}
            title="Newer conversations"
            aria-label="Previous page"
          >
            <i className="fa-solid fa-chevron-left" aria-hidden="true"></i>
          </button>
          <span className="ah-pager__range" aria-live="polite">
            {page.start + 1}–{page.start + page.visible.length} of {page.total}
          </span>
          <button
            type="button"
            className="ah-icon-btn"
            onClick={() => goTo(page.page + 1)}
            disabled={page.page === page.pages - 1}
            title="Older conversations"
            aria-label="Next page"
          >
            <i className="fa-solid fa-chevron-right" aria-hidden="true"></i>
          </button>
        </nav>
      ) : null}
    </section>
  )
}
