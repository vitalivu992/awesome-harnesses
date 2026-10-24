import { useEffect, useRef, useState } from "react"
import { bypassFlag, noBypassReason, resumeCommand } from "../lib/resume.js"
import { HARNESSES, harnessName } from "../lib/format.js"
import { readPrefs, writePref } from "../lib/prefs.js"
import { useShells } from "./Shells.jsx"

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
 * bypass-permissions option (remembered), a copy button and a button that
 * runs it in a new shell of the terminal panel.
 */
export function ResumeButton({ conversation }) {
  const [open, setOpen] = useState(false)
  const [bypass, setBypass] = useState(() => Boolean(readPrefs().bypass))
  const [copied, setCopied] = useState(false)
  const [shellError, setShellError] = useState("")
  const shells = useShells()
  const ref = useRef(null)
  const flag = bypassFlag(conversation.harness)
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

  useEffect(() => {
    setCopied(false)
    setShellError("")
  }, [command])

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

  const runInShell = async () => {
    setShellError("")
    try {
      await shells.openShell({ cwd: conversation.cwd || conversation.projectPath || "", run: command })
      setOpen(false)
    } catch (err) {
      setShellError(err.message)
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
          <p className="ah-popover__title">
            Continue in <code>{HARNESSES[conversation.harness]?.cli || conversation.harness}</code>
          </p>
          <pre className="ah-command" id="ahResumeCommand">
            <code>{command}</code>
          </pre>
          {flag ? (
            <label className="ah-check">
              <input type="checkbox" id="ahResumeBypass" checked={bypass} onChange={(e) => toggleBypass(e.target.checked)} />
              <span>
                Bypass permissions <code>{flag}</code>
              </span>
            </label>
          ) : (
            <p className="ah-popover__note" id="ahResumeNoBypass">
              <i className="fa-solid fa-circle-info" aria-hidden="true"></i> {noBypassReason(conversation.harness)}
            </p>
          )}
          {bypass && flag ? (
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
          {shellError ? (
            <p className="ah-popover__warn" role="alert">
              <i className="fa-solid fa-triangle-exclamation" aria-hidden="true"></i> Could not open a shell: {shellError}
            </p>
          ) : null}
          <div className="ah-popover__actions">
            {shells ? (
              <button type="button" className="ah-btn" id="ahResumeShell" onClick={runInShell} title="Open a shell in the conversation's folder and run the command">
                <i className="fa-solid fa-terminal" aria-hidden="true"></i> Run in shell
              </button>
            ) : null}
            <button type="button" className="ah-btn ah-btn--primary" id="ahResumeCopy" onClick={copy}>
              <i className={`fa-solid ${copied ? "fa-check" : "fa-copy"}`} aria-hidden="true"></i> {copied ? "Copied" : "Copy command"}
            </button>
          </div>
        </div>
      ) : null}
    </div>
  )
}
