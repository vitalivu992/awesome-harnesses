import { useEffect, useLayoutEffect, useRef, useState } from "react"
import { agentLabel, clockTime, duration, longDateTime, plural, statusMeta } from "../lib/format.js"
import { atBottom, isWorking } from "../lib/live.js"
import { Composer } from "./Composer.jsx"
import { ClientBadge, HarnessIcon } from "./HarnessIcon.jsx"
import { Markdown } from "./Markdown.jsx"
import { ResumeButton } from "./ResumeButton.jsx"
import { RunningBar, WorkingWord } from "./Running.jsx"
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
  if (!entry.prompt) return null
  const long = entry.prompt.length > 1200 || entry.prompt.split("\n").length > 18
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

/** A prompt sent from here that the transcript doesn't have yet. */
function PendingMessage({ run }) {
  return (
    <div className="ah-turn ah-turn--pending" id="ahPending">
      <div className="ah-msg ah-msg--user">
        <div className="ah-msg__meta">
          <span className="ah-msg__who">You</span>
          <span className="ah-msg__sending">Sent</span>
        </div>
        <div className="ah-bubble ah-bubble--user">{run.prompt}</div>
      </div>
      <p className="ah-turn-status ah-turn-status--live">
        <i className="fa-solid fa-circle-notch fa-spin" aria-hidden="true"></i> Waiting for the agent…
      </p>
    </div>
  )
}

/** How the last prompt sent from here failed, until dismissed. */
function RunError({ run, onDismiss }) {
  return (
    <div className="ah-run-error" role="alert" id="ahRunError">
      <p>
        <i className="fa-solid fa-triangle-exclamation" aria-hidden="true"></i> The prompt failed
        {run.exitCode ? ` (exit code ${run.exitCode})` : ""}.
        <button type="button" className="ah-link-btn" onClick={onDismiss}>
          Dismiss
        </button>
      </p>
      <pre className="ah-code">{run.error}</pre>
    </div>
  )
}

/** `live`: the turn the agent is still working on. */
function AssistantMessage({ entry, harness, index, expandAll, live }) {
  const [open, setOpen] = useState(expandAll)
  useEffect(() => setOpen(expandAll), [expandAll])
  const steps = entry.steps || []
  const took = duration(entry.promptAt, entry.endedAt)
  const status = entry.status || "done"
  if (!steps.length && !entry.final && status === "done" && !live) return null
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
              <span>{live ? (took ? `Working for ${took}` : "Working") : took ? `Worked for ${took}` : "Work details"}</span>
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
        {live ? (
          <p className="ah-turn-status ah-turn-status--live">
            <i className="fa-solid fa-circle-notch fa-spin" aria-hidden="true"></i> <WorkingWord />
          </p>
        ) : status !== "done" ? (
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

function Header({ conversation, working, expandAll, onExpandAll }) {
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
          {c.active ? (
            <span className="ah-chip ah-chip--live" title="Updates as the session writes to its transcript">
              <i className="fa-solid fa-tower-broadcast" aria-hidden="true"></i> {working ? "Working" : "Live"}
            </span>
          ) : null}
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
      {working ? <RunningBar className="ah-progress--edge" /> : null}
    </header>
  )
}

/** Why a prompt can't be sent to the conversation from here, or "". */
function composerBlocked(c, ours) {
  if (!c.resumable) return `This chat was started in the ${c.harness === "cursor" ? "Cursor app" : "app"}: continue it there.`
  if (isWorking(c) && !ours) return "The agent is working on this conversation elsewhere: wait for it to finish."
  return ""
}

/**
 * Main column: the conversation as a chat, work details collapsed by default,
 * and the prompt box under it. While the agent is working the work is shown,
 * and the chat follows new messages when it is scrolled to the bottom; once
 * the turn ends the work collapses and the final answer is left. `run` is the
 * newest prompt sent from here (`sentTurns`: the turns there were then).
 */
export function ConversationView({ conversation, loading, error, run = null, sentTurns = 0, onDismissRun, onOpenSettings }) {
  const running = Boolean(run?.running)
  const working = isWorking(conversation) || running
  const [expandAll, setExpandAll] = useState(working)
  useEffect(() => setExpandAll(working), [conversation?.id, working])

  const scrollRef = useRef(null)
  const follow = useRef(false)
  const shownId = useRef("")
  useLayoutEffect(() => {
    const el = scrollRef.current
    if (!el || !conversation) return
    const opened = shownId.current !== conversation.id
    shownId.current = conversation.id
    if ((opened && working) || (!opened && follow.current)) el.scrollTop = el.scrollHeight
    follow.current = atBottom(el)
  }, [conversation, working, expandAll])

  const failed = run && !run.running && run.error && !run.stopped ? run : null
  if (error && !failed) return <div className="ah-view ah-view--empty"><p className="ah-error">Could not load the conversation: {error.message}</p></div>
  if (!conversation) {
    // A new conversation whose transcript isn't written yet (or whose CLI failed first).
    if (run) {
      return (
        <div className="ah-view">
          <div className="ah-view__scroll" id="ahChat">
            <div className="ah-chat">
              {running ? <PendingMessage run={run} /> : null}
              {failed ? <RunError run={failed} onDismiss={() => onDismissRun?.(failed.id)} /> : null}
            </div>
          </div>
        </div>
      )
    }
    return (
      <div className="ah-view ah-view--empty">
        <p className="ah-empty ah-empty--center">
          {loading ? "Loading conversation…" : "Select a conversation to read it."}
        </p>
      </div>
    )
  }
  const c = conversation
  const pending = running && c.turns <= sentTurns
  const onSent = () => {
    follow.current = true
    requestAnimationFrame(() => {
      const el = scrollRef.current
      if (el) el.scrollTop = el.scrollHeight
    })
  }
  return (
    <div className="ah-view" aria-busy={loading || undefined}>
      <Header conversation={c} working={working} expandAll={expandAll} onExpandAll={setExpandAll} />
      <div className="ah-view__scroll" id="ahChat" ref={scrollRef} onScroll={(e) => (follow.current = atBottom(e.currentTarget))}>
        <div className="ah-chat">
          {c.entries.map((entry, i) =>
            entry.kind === "event" ? (
              <EventRow key={i} entry={entry} />
            ) : (
              <div key={i} className="ah-turn">
                <UserMessage entry={entry} />
                <AssistantMessage
                  entry={entry}
                  harness={c.harness}
                  index={i}
                  expandAll={expandAll}
                  live={working && !pending && i === c.entries.length - 1}
                />
              </div>
            )
          )}
          {!c.entries.length && !pending ? <p className="ah-empty ah-empty--center">This conversation has no messages.</p> : null}
          {pending ? <PendingMessage run={run} /> : null}
          {failed ? <RunError run={failed} onDismiss={() => onDismissRun?.(failed.id)} /> : null}
        </div>
      </div>
      <div className="ah-view__composer">
        <Composer
          key={`${c.harness}:${c.id}`}
          harness={c.harness}
          cwd={c.cwd || c.projectPath}
          session={c.id}
          turns={c.turns}
          run={running ? run : null}
          disabledReason={composerBlocked(c, running)}
          onSent={onSent}
          onOpenSettings={onOpenSettings}
        />
      </div>
    </div>
  )
}
