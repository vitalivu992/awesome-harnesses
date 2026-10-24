import { useEffect, useState } from "react"
import { harnessName } from "../lib/format.js"
import { Composer } from "./Composer.jsx"
import { HarnessIcon } from "./HarnessIcon.jsx"
import { RunningBar } from "./Running.jsx"
import { useRuns } from "./Runs.jsx"

/**
 * The main column for a new conversation in the selected project: the
 * harness, model and prompt. Once sent it opens the conversation as soon as
 * its id is known (at once for Claude Code and pi, which take one; else when
 * the CLI reports it).
 */
export function NewConversation({ project, harness, onHarnessChange, onStarted, onCancel, onOpenSettings }) {
  const runs = useRuns()
  const [runId, setRunId] = useState("")
  const run = runs.runs.find((r) => r.id === runId) || null

  useEffect(() => {
    if (run?.session) onStarted(run.harness, run.session)
  }, [run?.session, run?.harness, onStarted])

  const cwd = project?.exists ? project.path : ""
  const disabledReason = !project
    ? "Select a project first."
    : !cwd
      ? "This project's folder doesn't exist any more: there is nowhere to run the agent."
      : ""
  const waiting = run && !run.session && run.running

  return (
    <div className="ah-view ah-new" id="ahNewConversation">
      <header className="ah-view__head">
        <div className="ah-view__titles">
          <h1 className="ah-view__title">
            <i className="fa-regular fa-pen-to-square" aria-hidden="true"></i> New conversation
          </h1>
          <div className="ah-view__meta">
            <span className="ah-chip">
              <HarnessIcon harness={harness} size={12} /> {harnessName(harness)}
            </span>
            {project ? (
              <span className="ah-chip" title={project.path}>
                <i className="fa-regular fa-folder" aria-hidden="true"></i> {project.path || project.name}
              </span>
            ) : null}
          </div>
        </div>
        <div className="ah-view__actions">
          <button type="button" className="ah-btn" onClick={onCancel}>
            Cancel
          </button>
        </div>
        {waiting ? <RunningBar className="ah-progress--edge" /> : null}
      </header>
      <div className="ah-new__body">
        <div className="ah-new__intro">
          <HarnessIcon harness={harness} size={36} />
          <p className="ah-new__lead">
            What should {harnessName(harness)} do in <strong>{project?.name || "this project"}</strong>?
          </p>
          <p className="ah-new__hint">
            It runs on the server without its interactive UI and shows up in the conversation list. Type <kbd>/</kbd> for commands and
            skills.
          </p>
        </div>
        {waiting ? (
          <p className="ah-new__status" role="status">
            <i className="fa-solid fa-circle-notch fa-spin" aria-hidden="true"></i> Starting {harnessName(run.harness)}…
          </p>
        ) : null}
        {run && !run.running && !run.session ? (
          <div className="ah-run-error" role="alert">
            <p>
              <i className="fa-solid fa-triangle-exclamation" aria-hidden="true"></i> {harnessName(run.harness)} didn't start the
              conversation.
            </p>
            {run.error ? <pre className="ah-code">{run.error}</pre> : null}
          </div>
        ) : null}
        <Composer
          harness={harness}
          onHarnessChange={waiting ? undefined : onHarnessChange}
          cwd={cwd}
          run={waiting ? run : null}
          disabledReason={disabledReason}
          autoFocus
          onSent={(r) => setRunId(r.id)}
          onOpenSettings={onOpenSettings}
        />
      </div>
    </div>
  )
}
