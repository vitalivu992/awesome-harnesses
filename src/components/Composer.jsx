import { useEffect, useRef, useState } from "react"
import { previewRun } from "../lib/api.js"
import { HARNESS_IDS, harnessName, modelLabel } from "../lib/format.js"
import { readPrefs, writePref } from "../lib/prefs.js"
import { usedCommands } from "../lib/prompt.js"
import { configuredModels, launchRequest } from "../lib/settings.js"
import { useSettings } from "../hooks/useSettings.js"
import { useCommands, useHarnesses } from "../hooks/useHarnessData.js"
import { HarnessIcon } from "./HarnessIcon.jsx"
import { PromptBox } from "./PromptBox.jsx"
import { useRuns } from "./Runs.jsx"

/** The model the prompt box starts with: the last one used with the harness if still configured, else the first. */
function initialModel(settings, harness) {
  const models = configuredModels(settings, harness).map((m) => m.id.trim())
  const last = readPrefs().composerModels?.[harness]
  if (last !== undefined && (last === "" || models.includes(last))) return last
  return models[0] || ""
}

function effortLabel(effort) {
  return effort ? `${effort[0].toUpperCase()}${effort.slice(1)} effort` : "Default effort"
}

/**
 * The command a request would run, asked of the server (which builds it) a
 * moment after the last change.
 */
function useCommandPreview(req, enabled) {
  const [state, setState] = useState({ plan: null, error: "" })
  const key = JSON.stringify(req)
  useEffect(() => {
    if (!enabled) return undefined
    const ctrl = new AbortController()
    const timer = setTimeout(() => {
      previewRun(JSON.parse(key), { signal: ctrl.signal }).then(
        (plan) => setState({ plan, error: "" }),
        (err) => !ctrl.signal.aborted && setState({ plan: null, error: err.message })
      )
    }, 200)
    return () => {
      clearTimeout(timer)
      ctrl.abort()
    }
  }, [key, enabled])
  return state
}

/** The command line with its notes (settings the harness ignores, how a command is sent). */
export function CommandPreview({ req, id }) {
  const { plan, error } = useCommandPreview(req, true)
  if (error) return <p className="ah-preview__error">{error}</p>
  return (
    <div className="ah-preview" id={id}>
      <pre className="ah-command">
        <code>{plan ? plan.command : "…"}</code>
      </pre>
      {plan?.notes?.length ? (
        <ul className="ah-preview__notes">
          {plan.notes.map((n) => (
            <li key={n}>
              <i className="fa-solid fa-circle-info" aria-hidden="true"></i> {n}
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  )
}

/**
 * Where a prompt is written and sent: to a conversation (`session`) or as a
 * new one in `cwd`. `onHarnessChange` makes the harness selectable (a new
 * conversation). The model list, efforts, fast mode and bypass come from the
 * settings; the bar shows them, and "Command" the exact CLI command.
 */
export function Composer({
  harness,
  onHarnessChange,
  cwd,
  session = "",
  turns = 0,
  run = null,
  disabledReason = "",
  autoFocus = false,
  onSent,
  onOpenSettings,
}) {
  const [settings] = useSettings()
  const runs = useRuns()
  const harnesses = useHarnesses()
  const commands = useCommands(harness, cwd)
  const [text, setText] = useState("")
  const [model, setModel] = useState(() => initialModel(settings, harness))
  const [showCommand, setShowCommand] = useState(() => Boolean(readPrefs().showCommand))
  const [sending, setSending] = useState(false)
  const [error, setError] = useState("")
  const box = useRef(null)

  useEffect(() => setModel(initialModel(settings, harness)), [harness, settings])

  const models = configuredModels(settings, harness)
  const hs = settings.harnesses[harness]
  const info = harnesses.byId[harness]
  const req = launchRequest(settings, { harness, model, prompt: text, session, cwd })
  const running = Boolean(run?.running)
  const disabled = Boolean(disabledReason)
  const used = usedCommands(text, commands.data)
  const placeholder = disabled
    ? disabledReason
    : session
      ? `Message ${harnessName(harness)}… (/ for commands and skills)`
      : `Ask ${harnessName(harness)} to start something… (/ for commands and skills)`

  const pickModel = (id) => {
    setModel(id)
    writePref("composerModels", { ...readPrefs().composerModels, [harness]: id })
  }

  const toggleCommand = () => {
    writePref("showCommand", !showCommand)
    setShowCommand(!showCommand)
  }

  const send = async () => {
    if (!text.trim() || sending || running || disabled) return
    setSending(true)
    setError("")
    try {
      const started = await runs.start(req, { turns })
      setText("")
      onSent?.(started)
    } catch (err) {
      setError(err.message)
    } finally {
      setSending(false)
      box.current?.focus()
    }
  }

  return (
    <form
      className={`ah-composer${running ? " is-running" : ""}`}
      aria-label={session ? "Send a message" : "Start a conversation"}
      onSubmit={(e) => {
        e.preventDefault()
        send()
      }}
    >
      {error ? (
        <p className="ah-composer__error" role="alert">
          <i className="fa-solid fa-triangle-exclamation" aria-hidden="true"></i> {error}
        </p>
      ) : null}
      {info && !info.found ? (
        <p className="ah-composer__warn">
          <i className="fa-solid fa-circle-info" aria-hidden="true"></i> <code>{info.cli}</code> isn't installed on the server (not found on
          PATH).
        </p>
      ) : null}
      <PromptBox
        ref={box}
        id="ahPrompt"
        value={text}
        onChange={setText}
        onSubmit={send}
        commands={commands.data}
        mentions={harness === "codex"}
        disabled={disabled}
        placeholder={placeholder}
        autoFocus={autoFocus}
      />
      {used.length ? (
        <p className="ah-composer__uses">
          <i className={`fa-solid ${used[0].kind === "skill" ? "fa-wand-magic-sparkles" : "fa-terminal"}`} aria-hidden="true"></i>
          {used.map((c) => (
            <span key={c.insert} title={c.description}>
              Uses the {c.kind} <strong className={c.kind === "skill" ? "is-skill" : ""}>{c.insert}</strong>
              {c.description ? ` — ${c.description}` : ""}
            </span>
          ))}
        </p>
      ) : null}
      <div className="ah-composer__bar">
        {onHarnessChange ? (
          <div className="ah-segmented ah-segmented--harness ah-composer__harness" role="radiogroup" aria-label="Harness">
            {HARNESS_IDS.map((h) => (
              <button
                key={h}
                type="button"
                role="radio"
                aria-checked={h === harness}
                aria-label={harnessName(h)}
                title={harnessName(h)}
                className={`${h === harness ? "is-active" : "is-icon"}`}
                data-harness={h}
                onClick={() => onHarnessChange(h)}
              >
                <HarnessIcon harness={h} size={13} />
                {h === harness ? harnessName(h) : null}
              </button>
            ))}
          </div>
        ) : null}
        <label className="ah-select" title="Model (configure the list in Settings)">
          <span className="ah-visually-hidden">Model</span>
          <select id="ahComposerModel" value={model} onChange={(e) => pickModel(e.target.value)} disabled={disabled}>
            <option value="">Default model</option>
            {models.map((m) => (
              <option key={m.id} value={m.id.trim()}>
                {modelLabel(m.id.trim())} ({m.id.trim()})
              </option>
            ))}
          </select>
          <i className="fa-solid fa-chevron-down" aria-hidden="true"></i>
        </label>
        <button type="button" className="ah-composer__chips" onClick={onOpenSettings} title="Change in Settings">
          {info?.efforts?.length ? <span className="ah-chip">{effortLabel(req.effort)}</span> : null}
          {hs?.fast && info?.fast ? (
            <span className="ah-chip ah-chip--fast">
              <i className="fa-solid fa-bolt" aria-hidden="true"></i> Fast
            </span>
          ) : null}
          {hs?.bypass && info?.bypass ? (
            <span className="ah-chip ah-chip--warn" title="The agent runs every tool without asking">
              <i className="fa-solid fa-shield-halved" aria-hidden="true"></i> Bypass
            </span>
          ) : null}
        </button>
        <span className="ah-composer__spacer"></span>
        <button
          type="button"
          className={`ah-icon-btn${showCommand ? " is-open" : ""}`}
          id="ahComposerCommand"
          aria-pressed={showCommand}
          aria-controls="ahComposerPreview"
          onClick={toggleCommand}
          title="Show the command that will run"
        >
          <i className="fa-solid fa-code" aria-hidden="true"></i> Command
        </button>
        {running ? (
          <button type="button" className="ah-btn ah-btn--danger" id="ahStop" onClick={() => runs.stop(run.id).catch((e) => setError(e.message))}>
            <i className="fa-solid fa-stop" aria-hidden="true"></i> Stop
          </button>
        ) : (
          <button type="submit" className="ah-btn ah-btn--primary" id="ahSend" disabled={!text.trim() || sending || disabled}>
            <i className={`fa-solid ${sending ? "fa-spinner fa-spin" : "fa-paper-plane"}`} aria-hidden="true"></i> Send
          </button>
        )}
      </div>
      {showCommand && !disabled ? <CommandPreview req={{ ...req, prompt: text.trim() ? text : "Your prompt" }} id="ahComposerPreview" /> : null}
    </form>
  )
}
