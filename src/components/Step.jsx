import { useState } from "react"
import { clockTime, duration, formatMs } from "../lib/format.js"
import { toolIcon, toolLabel, toolSummary } from "../lib/tools.js"

function prettyInput(input) {
  if (input == null) return ""
  if (typeof input === "string") return input
  return JSON.stringify(input, null, 2)
}

/** The parts of a tool input worth showing as text blocks instead of JSON. */
function ToolInput({ tool, input }) {
  const i = input && typeof input === "object" ? input : null
  const name = String(tool || "").toLowerCase()
  if (i && (name === "bash" || name === "shell") && typeof i.command === "string") {
    return (
      <>
        {i.description ? <p className="ah-step__desc">{i.description}</p> : null}
        <pre className="ah-code ah-code--shell">
          <code>{i.command}</code>
        </pre>
      </>
    )
  }
  if (i && typeof i.old_string === "string" && typeof i.new_string === "string") {
    return (
      <>
        <p className="ah-step__desc">{i.file_path || i.path}</p>
        <pre className="ah-code ah-code--del">
          <code>{i.old_string}</code>
        </pre>
        <pre className="ah-code ah-code--add">
          <code>{i.new_string}</code>
        </pre>
      </>
    )
  }
  if (i && typeof i.content === "string" && (i.file_path || i.path)) {
    return (
      <>
        <p className="ah-step__desc">{i.file_path || i.path}</p>
        <pre className="ah-code ah-code--add">
          <code>{i.content}</code>
        </pre>
      </>
    )
  }
  return (
    <pre className="ah-code">
      <code>{prettyInput(input)}</code>
    </pre>
  )
}

function ToolStep({ step }) {
  const [open, setOpen] = useState(false)
  const summary = toolSummary(step.tool, step.input)
  const took = duration(step.at, step.outputAt)
  return (
    <div className={`ah-step ah-step--tool${step.isError ? " is-error" : ""}`}>
      <button type="button" className="ah-step__head" aria-expanded={open} onClick={() => setOpen((v) => !v)}>
        <i className={`fa-solid fa-chevron-right ah-step__chev${open ? " is-open" : ""}`} aria-hidden="true"></i>
        <i className={`fa-solid ${toolIcon(step.tool)} ah-step__icon`} aria-hidden="true"></i>
        <span className="ah-step__tool">{toolLabel(step.tool)}</span>
        <span className="ah-step__summary">{summary}</span>
        {step.isError ? <span className="ah-step__badge">error</span> : null}
        {took ? <span className="ah-step__took">{took}</span> : null}
      </button>
      {open ? (
        <div className="ah-step__body">
          <div className="ah-step__label">Input</div>
          <ToolInput tool={step.tool} input={step.input} />
          {step.output !== undefined && step.output !== "" ? (
            <>
              <div className="ah-step__label">
                {step.isError ? "Error" : "Output"}
                {step.outputAt ? <time dateTime={step.outputAt}> · {clockTime(step.outputAt)}</time> : null}
              </div>
              <pre className={`ah-code ah-code--output${step.isError ? " is-error" : ""}`}>
                <code>{step.output}</code>
              </pre>
            </>
          ) : null}
          {step.truncated ? <p className="ah-step__note">Long values were cut for display.</p> : null}
        </div>
      ) : null}
    </div>
  )
}

function TextStep({ step }) {
  const [open, setOpen] = useState(false)
  const thinking = step.kind === "thinking"
  const long = step.text.length > 420 || step.text.split("\n").length > 6
  return (
    <div className={`ah-step ah-step--${step.kind}`}>
      <div className="ah-step__line">
        <i className={`fa-solid ${thinking ? "fa-brain" : "fa-comment-dots"} ah-step__icon`} aria-hidden="true"></i>
        <span className="ah-step__tool">{thinking ? "Thinking" : "Note"}</span>
        {step.durationMs ? <span className="ah-step__took">{formatMs(step.durationMs)}</span> : null}
      </div>
      <div className={`ah-step__text${long && !open ? " is-clamped" : ""}`}>{step.text}</div>
      {long ? (
        <button type="button" className="ah-link-btn" onClick={() => setOpen((v) => !v)}>
          {open ? "Show less" : "Show more"}
        </button>
      ) : null}
    </div>
  )
}

/** One step of a turn's work: a thought, a note, or a tool call (click for input/output). */
export function Step({ step }) {
  const time = clockTime(step.at)
  return (
    <li className="ah-step-row">
      <time className="ah-step-row__time" dateTime={step.at} title={step.at}>
        {time}
      </time>
      <div className="ah-step-row__body">{step.kind === "tool" ? <ToolStep step={step} /> : <TextStep step={step} />}</div>
    </li>
  )
}
