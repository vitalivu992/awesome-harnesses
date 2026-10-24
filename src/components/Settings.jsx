import { useEffect, useRef, useState } from "react"
import { HARNESS_IDS, harnessName } from "../lib/format.js"
import { readPrefs, writePref } from "../lib/prefs.js"
import { configuredModels, effortsFor, launchRequest } from "../lib/settings.js"
import { useSettings } from "../hooks/useSettings.js"
import { useHarnesses, useModels } from "../hooks/useHarnessData.js"
import { CommandPreview } from "./Composer.jsx"
import { HarnessIcon } from "./HarnessIcon.jsx"

function EffortSelect({ value, efforts, onChange, id, label }) {
  const options = value && !efforts.includes(value) ? [...efforts, value] : efforts
  return (
    <label className="ah-select ah-select--small">
      <span className="ah-visually-hidden">{label}</span>
      <select id={id} value={value} onChange={(e) => onChange(e.target.value)} aria-label={label}>
        <option value="">Default effort</option>
        {options.map((e) => (
          <option key={e} value={e}>
            {e}
            {efforts.includes(e) ? "" : " (not supported)"}
          </option>
        ))}
      </select>
      <i className="fa-solid fa-chevron-down" aria-hidden="true"></i>
    </label>
  )
}

function Switch({ checked, onChange, id, children, disabled = false }) {
  return (
    <label className={`ah-switch${disabled ? " is-disabled" : ""}`}>
      <input type="checkbox" role="switch" id={id} checked={checked} disabled={disabled} onChange={(e) => onChange(e.target.checked)} />
      <span className="ah-switch__track" aria-hidden="true"></span>
      <span className="ah-switch__label">{children}</span>
    </label>
  )
}

/**
 * One harness: its models (each with an effort; none = the harness's default
 * model), the default model's effort, fast mode, bypass, and the command a
 * prompt would run with these settings.
 */
function HarnessSettings({ id, info, settings, update, cwd, open, onToggle }) {
  const h = settings.harnesses[id]
  const [refresh, setRefresh] = useState(0)
  const suggestions = useModels(id, { enabled: open, version: refresh })
  const known = new Map((suggestions.data || []).map((m) => [m.id, m]))
  const [previewModel, setPreviewModel] = useState("")
  const models = configuredModels(settings, id)
  const listId = `ahModels-${id}`
  useEffect(() => {
    if (previewModel && !models.some((m) => m.id.trim() === previewModel)) setPreviewModel("")
  }, [models, previewModel])

  const setModels = (next) => update(id, { models: next })
  const setRow = (i, patch) => setModels(h.models.map((m, j) => (j === i ? { ...m, ...patch } : m)))
  const move = (i, step) => {
    const next = [...h.models]
    const [row] = next.splice(i, 1)
    next.splice(i + step, 0, row)
    setModels(next)
  }

  return (
    <section className={`ah-hset${open ? " is-open" : ""}`} data-harness={id} aria-labelledby={`ahHset-${id}`}>
      <button type="button" className="ah-hset__head" id={`ahHset-${id}`} aria-expanded={open} onClick={onToggle}>
        <HarnessIcon harness={id} size={18} />
        <span className="ah-hset__name">{harnessName(id)}</span>
        <span className="ah-hset__summary">
          {models.length ? `${models.length} model${models.length === 1 ? "" : "s"}` : "default model"}
          {h.fast && info?.fast ? " · fast" : ""}
          {h.bypass && info?.bypass ? " · bypass" : ""}
        </span>
        {info && !info.found ? (
          <span className="ah-hset__missing" title={`${info.cli} isn't on the server's PATH`}>
            not installed
          </span>
        ) : null}
        <i className={`fa-solid fa-chevron-down ah-hset__chev${open ? " is-open" : ""}`} aria-hidden="true"></i>
      </button>
      {open ? (
        <div className="ah-hset__body">
          <p className="ah-hset__cli">
            <code>{info?.cli || id}</code>{" "}
            {info ? info.found ? <span title={info.found}>{info.found}</span> : <span className="ah-hset__missing">not found on PATH</span> : null}
          </p>

          <h4 className="ah-hset__label">Models</h4>
          <p className="ah-hset__hint">The prompt box offers these, first one first. Leave the list empty to use {harnessName(id)}'s default model.</p>
          {h.models.length ? (
            <ol className="ah-models">
              {h.models.map((m, i) => (
                <li key={i} className="ah-models__row">
                  <input
                    type="text"
                    className="ah-input"
                    list={listId}
                    value={m.id}
                    placeholder="model id"
                    aria-label={`Model ${i + 1}`}
                    spellCheck="false"
                    autoFocus={!m.id && i === h.models.length - 1}
                    onChange={(e) => setRow(i, { id: e.target.value })}
                  />
                  {info?.efforts?.length ? (
                    <EffortSelect
                      label={`Effort for model ${i + 1}`}
                      value={m.effort}
                      efforts={effortsFor(info, known.get(m.id.trim()))}
                      onChange={(effort) => setRow(i, { effort })}
                    />
                  ) : null}
                  <span className="ah-models__tools">
                    <button type="button" className="ah-icon-btn" disabled={i === 0} onClick={() => move(i, -1)} aria-label="Move up" title="Move up">
                      <i className="fa-solid fa-arrow-up" aria-hidden="true"></i>
                    </button>
                    <button
                      type="button"
                      className="ah-icon-btn"
                      onClick={() => setModels(h.models.filter((_, j) => j !== i))}
                      aria-label={`Remove ${m.id || "model"}`}
                      title="Remove"
                    >
                      <i className="fa-solid fa-trash-can" aria-hidden="true"></i>
                    </button>
                  </span>
                </li>
              ))}
            </ol>
          ) : null}
          <datalist id={listId}>
            {(suggestions.data || []).map((m) => (
              <option key={m.id} value={m.id}>
                {m.name || m.id}
              </option>
            ))}
          </datalist>
          <div className="ah-hset__row">
            <button type="button" className="ah-btn ah-btn--small" onClick={() => setModels([...h.models, { id: "", effort: "" }])}>
              <i className="fa-solid fa-plus" aria-hidden="true"></i> Add model
            </button>
            <span className="ah-hset__suggest">
              {suggestions.loading ? (
                <>
                  <i className="fa-solid fa-spinner fa-spin" aria-hidden="true"></i> Asking {info?.cli || id} for its models…
                </>
              ) : suggestions.error ? (
                <span className="ah-hset__missing">{suggestions.error.message}</span>
              ) : suggestions.data ? (
                <>
                  {suggestions.data.length ? `${suggestions.data.length} models to pick from` : `${info?.cli || id} listed no models`}{" "}
                  <button type="button" className="ah-link-btn" onClick={() => setRefresh((n) => n + 1)}>
                    Refresh
                  </button>
                </>
              ) : null}
            </span>
          </div>

          {info?.efforts?.length ? (
            <div className="ah-hset__field">
              <h4 className="ah-hset__label">Thinking effort of the default model</h4>
              <EffortSelect label="Effort of the default model" value={h.effort} efforts={info.efforts} onChange={(effort) => update(id, { effort })} />
            </div>
          ) : (
            <p className="ah-hset__note">
              <i className="fa-solid fa-circle-info" aria-hidden="true"></i> {info?.effortNote || "No effort setting."}
            </p>
          )}
          {info?.efforts?.length && info.effortNote ? <p className="ah-hset__hint">{info.effortNote}</p> : null}

          <div className="ah-hset__field">
            {info?.fast ? (
              <Switch id={`ahFast-${id}`} checked={h.fast} onChange={(fast) => update(id, { fast })}>
                <strong>Fast mode</strong>
                <span className="ah-hset__hint">Faster output for a higher usage cost. Off by default.</span>
              </Switch>
            ) : (
              <p className="ah-hset__note">
                <i className="fa-solid fa-bolt" aria-hidden="true"></i> {info?.fastNote || "No fast mode."}
              </p>
            )}
          </div>

          <div className="ah-hset__field">
            {info?.bypass ? (
              <>
                <Switch id={`ahBypass-${id}`} checked={h.bypass} onChange={(bypass) => update(id, { bypass })}>
                  <strong>Bypass permissions</strong> <code>{info.bypass}</code>
                  <span className="ah-hset__hint">
                    A prompt sent from here can't ask for approval: without this, what needs approval is denied.
                  </span>
                </Switch>
                {h.bypass ? (
                  <p className="ah-popover__warn">
                    <i className="fa-solid fa-triangle-exclamation" aria-hidden="true"></i> The agent runs every tool and command without asking.
                  </p>
                ) : null}
              </>
            ) : (
              <p className="ah-hset__note">
                <i className="fa-solid fa-shield-halved" aria-hidden="true"></i> {harnessName(id)} doesn't ask for permissions: nothing to bypass.
              </p>
            )}
          </div>

          <div className="ah-hset__field">
            <div className="ah-hset__preview-head">
              <h4 className="ah-hset__label">Command preview</h4>
              <label className="ah-select ah-select--small">
                <span className="ah-visually-hidden">Preview with model</span>
                <select value={previewModel} onChange={(e) => setPreviewModel(e.target.value)} aria-label="Preview with model">
                  <option value="">Default model</option>
                  {models.map((m) => (
                    <option key={m.id} value={m.id.trim()}>
                      {m.id.trim()}
                    </option>
                  ))}
                </select>
                <i className="fa-solid fa-chevron-down" aria-hidden="true"></i>
              </label>
            </div>
            <p className="ah-hset__hint">A new conversation {cwd ? "in the selected project" : "in your home folder"} runs:</p>
            <CommandPreview req={launchRequest(settings, { harness: id, model: previewModel, prompt: "Your prompt", cwd })} id={`ahPreview-${id}`} />
          </div>
        </div>
      ) : null}
    </section>
  )
}

/**
 * The Settings panel on the right: how prompts are sent to each harness.
 * Saved in this browser as it is edited. It can widen (remembered) and closes
 * with Esc or ×. `focus` opens that harness's section.
 */
export function SettingsPanel({ onClose, cwd, focus = "" }) {
  const [settings, save] = useSettings()
  const harnesses = useHarnesses()
  const [wide, setWide] = useState(() => Boolean(readPrefs().settingsWide))
  const [openId, setOpenId] = useState(() => focus || settings.defaultHarness)
  const panel = useRef(null)

  useEffect(() => {
    if (focus) setOpenId(focus)
  }, [focus])

  useEffect(() => {
    panel.current?.focus()
  }, [])

  const update = (id, patch) =>
    save((s) => ({ ...s, harnesses: { ...s.harnesses, [id]: { ...s.harnesses[id], ...patch } } }))

  const toggleWide = () => {
    writePref("settingsWide", !wide)
    setWide(!wide)
  }

  return (
    <aside
      className={`ah-settings${wide ? " is-wide" : ""}`}
      id="ahSettings"
      aria-labelledby="ahSettingsTitle"
      tabIndex={-1}
      ref={panel}
      onKeyDown={(e) => {
        if (e.key === "Escape" && !e.defaultPrevented) onClose()
      }}
    >
      <header className="ah-panel-head ah-settings__head">
        <h2 className="ah-settings__title" id="ahSettingsTitle">
          <i className="fa-solid fa-gear" aria-hidden="true"></i> Settings
        </h2>
        <div className="ah-panel-head__actions">
          <button
            type="button"
            className="ah-icon-btn"
            id="ahSettingsWide"
            onClick={toggleWide}
            aria-pressed={wide}
            title={wide ? "Narrower" : "Wider"}
            aria-label={wide ? "Make the settings narrower" : "Make the settings wider"}
          >
            <i className={`fa-solid ${wide ? "fa-down-left-and-up-right-to-center" : "fa-up-right-and-down-left-from-center"}`} aria-hidden="true"></i>
          </button>
          <button type="button" className="ah-icon-btn" id="ahSettingsClose" onClick={onClose} title="Close (Esc)" aria-label="Close the settings">
            <i className="fa-solid fa-xmark" aria-hidden="true"></i>
          </button>
        </div>
      </header>
      <div className="ah-settings__scroll">
        <section className="ah-settings__section">
          <h3 className="ah-settings__heading">New conversations</h3>
          <label className="ah-settings__field">
            <span>
              Default harness
              <span className="ah-hset__hint">For New while the conversation filter shows all harnesses.</span>
            </span>
            <span className="ah-select">
              <select
                id="ahDefaultHarness"
                value={settings.defaultHarness}
                onChange={(e) => save((s) => ({ ...s, defaultHarness: e.target.value }))}
              >
                {HARNESS_IDS.map((h) => (
                  <option key={h} value={h}>
                    {harnessName(h)}
                  </option>
                ))}
              </select>
              <i className="fa-solid fa-chevron-down" aria-hidden="true"></i>
            </span>
          </label>
        </section>
        <section className="ah-settings__section">
          <h3 className="ah-settings__heading">Harnesses</h3>
          <p className="ah-hset__hint">
            Prompts run the harness CLI on the server without its interactive UI, in the conversation's folder. Saved in this browser.
          </p>
          {harnesses.error ? <p className="ah-error">Could not load the harnesses: {harnesses.error.message}</p> : null}
          {HARNESS_IDS.map((id) => (
            <HarnessSettings
              key={id}
              id={id}
              info={harnesses.byId[id]}
              settings={settings}
              update={update}
              cwd={cwd}
              open={openId === id}
              onToggle={() => setOpenId(openId === id ? "" : id)}
            />
          ))}
        </section>
      </div>
    </aside>
  )
}
