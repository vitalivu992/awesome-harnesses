import { useEffect, useRef, useState } from "react"
import { bypassFlag, resumeCommand } from "../lib/resume.js"
import { harnessName } from "../lib/format.js"
import { readPrefs, writePref } from "../lib/prefs.js"

async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    // Older browsers / non-secure origins: fall back to a hidden textarea.
    const area = document.createElement("textarea")
    area.value = text
    area.setAttribute("readonly", "")
    area.style.position = "fixed"
    area.style.opacity = "0"
    document.body.appendChild(area)
    area.select()
    const ok = document.execCommand("copy")
    area.remove()
    return ok
  }
}

/**
 * "Resume" popover: the CLI command that continues the conversation, a
 * bypass-permissions option (remembered), and a copy button.
 */
export function ResumeButton({ conversation }) {
  const [open, setOpen] = useState(false)
  const [bypass, setBypass] = useState(() => Boolean(readPrefs().bypass))
  const [copied, setCopied] = useState(false)
  const ref = useRef(null)
  const command = resumeCommand(conversation, { bypass })

  useEffect(() => {
    if (!open) return undefined
    const onDown = (ev) => {
      if (!ref.current?.contains(ev.target)) setOpen(false)
    }
    const onKey = (ev) => ev.key === "Escape" && setOpen(false)
    document.addEventListener("mousedown", onDown)
    document.addEventListener("keydown", onKey)
    return () => {
      document.removeEventListener("mousedown", onDown)
      document.removeEventListener("keydown", onKey)
    }
  }, [open])

  useEffect(() => setCopied(false), [command])

  const toggleBypass = (value) => {
    setBypass(value)
    writePref("bypass", value)
  }

  const copy = async () => {
    if (await copyText(command)) {
      setCopied(true)
      setTimeout(() => setCopied(false), 1600)
    }
  }

  return (
    <div className="ah-resume" ref={ref}>
      <button
        type="button"
        className="ah-btn ah-btn--primary"
        id="ahResume"
        aria-expanded={open}
        aria-haspopup="dialog"
        onClick={() => setOpen((v) => !v)}
      >
        <i className="fa-solid fa-terminal" aria-hidden="true"></i> Resume
        <i className="fa-solid fa-chevron-down ah-btn__caret" aria-hidden="true"></i>
      </button>
      {open ? (
        <div className="ah-popover" role="dialog" aria-label="Resume in terminal">
          <p className="ah-popover__title">Continue in {conversation.harness === "cursor" ? "cursor-agent" : "the Claude Code CLI"}</p>
          <pre className="ah-command" id="ahResumeCommand">
            <code>{command}</code>
          </pre>
          <label className="ah-check">
            <input type="checkbox" id="ahResumeBypass" checked={bypass} onChange={(e) => toggleBypass(e.target.checked)} />
            <span>
              Bypass permissions <code>{bypassFlag(conversation.harness)}</code>
            </span>
          </label>
          {bypass ? (
            <p className="ah-popover__warn">
              <i className="fa-solid fa-triangle-exclamation" aria-hidden="true"></i> The agent runs every tool and command
              without asking.
            </p>
          ) : null}
          {!conversation.resumable ? (
            <p className="ah-popover__warn">
              <i className="fa-solid fa-circle-info" aria-hidden="true"></i> This chat was started in the{" "}
              {harnessName(conversation.harness)} app, not the CLI; the CLI may not find it. Open it in the app instead.
            </p>
          ) : null}
          <div className="ah-popover__actions">
            <button type="button" className="ah-btn ah-btn--primary" id="ahResumeCopy" onClick={copy}>
              <i className={`fa-solid ${copied ? "fa-check" : "fa-copy"}`} aria-hidden="true"></i> {copied ? "Copied" : "Copy command"}
            </button>
          </div>
        </div>
      ) : null}
    </div>
  )
}
