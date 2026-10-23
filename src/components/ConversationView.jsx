import { useEffect, useState } from "react"
import { agentLabel, clockTime, duration, longDateTime, plural, statusMeta } from "../lib/format.js"
import { ClientBadge, HarnessIcon } from "./HarnessIcon.jsx"
import { Markdown } from "./Markdown.jsx"
import { ResumeButton } from "./ResumeButton.jsx"
import { StatusIcon } from "./StatusIcon.jsx"
import { Step } from "./Step.jsx"

/** "14 tool calls · 3 thoughts · 2 notes" for a turn's steps. */
export function workSummary(steps) {
  const n = { tool: 0, thinking: 0, text: 0 }
  for (const s of steps || []) n[s.kind] = (n[s.kind] || 0) + 1
  return [
    n.tool && plural(n.tool, "tool call"),
    n.thinking && plural(n.thinking, "thought"),
    n.text && plural(n.text, "note"),
  ]
    .filter(Boolean)
    .join(" · ")
}

function UserMessage({ entry }) {
  const [open, setOpen] = useState(false)
  const long = entry.prompt.length > 1200 || entry.prompt.split("\n").length > 18
  if (!entry.prompt) return null
  return (
    <div className="ah-msg ah-msg--user">
      <div className="ah-msg__meta">
        <span className="ah-msg__who">You</span>
        {entry.promptAt ? (
          <time dateTime={entry.promptAt} title={longDateTime(entry.promptAt)}>
            {clockTime(entry.promptAt)}
          </time>
        ) : null}
      </div>
      <div className={`ah-bubble ah-bubble--user${long && !open ? " is-clamped" : ""}`}>{entry.prompt}</div>
      {long ? (
        <button type="button" className="ah-link-btn" onClick={() => setOpen((v) => !v)}>
          {open ? "Show less" : "Show full message"}
        </button>
      ) : null}
    </div>
  )
}

function AssistantMessage({ entry, harness, index, expandAll }) {
  const [open, setOpen] = useState(expandAll)
  useEffect(() => setOpen(expandAll), [expandAll])
  const steps = entry.steps || []
  const took = duration(entry.promptAt, entry.endedAt)
  const status = entry.status || "done"
  if (!steps.length && !entry.final && status === "done") return null
  return (
    <div className="ah-msg ah-msg--agent" data-turn={index}>
      <div className="ah-avatar" aria-hidden="true">
        <HarnessIcon harness={harness} size={18} />
      </div>
      <div className="ah-msg__main">
        <div className="ah-msg__meta">
          <span className="ah-msg__who">{agentLabel(harness, entry.model)}</span>
          {entry.endedAt ? (
            <time dateTime={entry.endedAt} title={longDateTime(entry.endedAt)}>
              {clockTime(entry.endedAt)}
            </time>
          ) : null}
        </div>
        {steps.length ? (
          <div className={`ah-work${open ? " is-open" : ""}`}>
            <button type="button" className="ah-work__toggle" aria-expanded={open} onClick={() => setOpen((v) => !v)}>
              <i className={`fa-solid fa-chevron-right ah-work__chev${open ? " is-open" : ""}`} aria-hidden="true"></i>
              <span>{took ? `Worked for ${took}` : "Work details"}</span>
              <span className="ah-work__summary">{workSummary(steps)}</span>
            </button>
            {open ? (
              <ol className="ah-steps" aria-label="Work details">
                {steps.map((step, i) => (
                  <Step key={i} step={step} />
                ))}
              </ol>
            ) : null}
          </div>
        ) : null}
        {entry.final ? (
          <div className="ah-bubble ah-bubble--agent">
            <Markdown docId={`turn-${index}`}>{entry.final}</Markdown>
          </div>
        ) : null}
        {status !== "done" ? (
          <p className={`ah-turn-status ah-turn-status--${status}`}>
            <StatusIcon status={status} /> {statusMeta(status).label}
            {entry.error ? <span className="ah-turn-status__error">: {entry.error}</span> : null}
          </p>
        ) : null}
      </div>
    </div>
  )
}

function EventRow({ entry }) {
  const [open, setOpen] = useState(false)
  return (
    <div className="ah-event">
      <button
        type="button"
        className="ah-event__pill"
        disabled={!entry.detail}
        aria-expanded={entry.detail ? open : undefined}
        onClick={() => setOpen((v) => !v)}
      >
        <i className="fa-solid fa-circle-info" aria-hidden="true"></i>
        <span>{entry.text}</span>
        {entry.at ? <time dateTime={entry.at}>{clockTime(entry.at)}</time> : null}
        {entry.detail ? <i className={`fa-solid fa-chevron-down ah-event__chev${open ? " is-open" : ""}`} aria-hidden="true"></i> : null}
      </button>
      {open && entry.detail ? <pre className="ah-code ah-event__detail">{entry.detail}</pre> : null}
    </div>
  )
}

function Header({ conversation, expandAll, onExpandAll }) {
  const c = conversation
  const models = (c.models || []).length ? c.models : [""]
  return (
    <header className="ah-view__head">
      <div className="ah-view__titles">
        <h1 className="ah-view__title" title={c.title}>
          <StatusIcon status={c.status} active={c.active} /> {c.title}
        </h1>
        <div className="ah-view__meta">
          {models.map((m) => (
            <span key={m} className="ah-chip">
              <HarnessIcon harness={c.harness} size={12} /> {agentLabel(c.harness, m)}
            </span>
          ))}
          <ClientBadge conversation={c} />
          {c.gitBranch ? (
            <span className="ah-chip" title="Git branch">
              <i className="fa-solid fa-code-branch" aria-hidden="true"></i> {c.gitBranch}
            </span>
          ) : null}
          <span className="ah-chip" title={`Started ${longDateTime(c.startedAt)}\nUpdated ${longDateTime(c.updatedAt)}`}>
            <i className="fa-regular fa-clock" aria-hidden="true"></i> {longDateTime(c.startedAt || c.updatedAt)}
          </span>
          <span className="ah-chip">
            {plural(c.turns, "prompt")} · {plural(c.toolCalls, "tool call")}
          </span>
          <span className="ah-chip ah-chip--mono" title="Session id">
            {c.id}
          </span>
        </div>
      </div>
      <div className="ah-view__actions">
        <button
          type="button"
          className="ah-btn"
          id="ahExpandAll"
          aria-pressed={expandAll}
          onClick={() => onExpandAll(!expandAll)}
          title="Show or hide the tool calls and thinking of every answer"
        >
          <i className={`fa-solid ${expandAll ? "fa-compress" : "fa-expand"}`} aria-hidden="true"></i>
          {expandAll ? "Hide work" : "Show work"}
        </button>
        <ResumeButton conversation={c} />
      </div>
    </header>
  )
}

/** Main column: the conversation as a chat, work details collapsed by default. */
export function ConversationView({ conversation, loading, error }) {
  const [expandAll, setExpandAll] = useState(false)
  useEffect(() => setExpandAll(false), [conversation?.id])

  if (error) return <div className="ah-view ah-view--empty"><p className="ah-error">Could not load the conversation: {error.message}</p></div>
  if (!conversation) {
    return (
      <div className="ah-view ah-view--empty">
        <p className="ah-empty ah-empty--center">
          {loading ? "Loading conversation…" : "Select a conversation to read it."}
        </p>
      </div>
    )
  }
  const c = conversation
  return (
    <div className="ah-view" aria-busy={loading || undefined}>
      <Header conversation={c} expandAll={expandAll} onExpandAll={setExpandAll} />
      <div className="ah-view__scroll" id="ahChat">
        <div className="ah-chat">
          {c.entries.map((entry, i) =>
            entry.kind === "event" ? (
              <EventRow key={i} entry={entry} />
            ) : (
              <div key={i} className="ah-turn">
                <UserMessage entry={entry} />
                <AssistantMessage entry={entry} harness={c.harness} index={i} expandAll={expandAll} />
              </div>
            )
          )}
          {!c.entries.length ? <p className="ah-empty ah-empty--center">This conversation has no messages.</p> : null}
        </div>
      </div>
    </div>
  )
}
