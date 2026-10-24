import { Suspense, createContext, lazy, useCallback, useContext, useEffect, useMemo, useRef, useState } from "react"
import { createShell, fetchShells, killShell } from "../lib/api.js"
import { readPrefs, writePref } from "../lib/prefs.js"
import { folderName, neighbourId, shellTitle } from "../lib/shells.js"

// xterm.js only loads once a shell is open.
const Terminal = lazy(() => import("./Terminal.jsx"))

const ShellContext = createContext(null)

/** `{ shells, openShell({cwd, run}), … }` from the nearest ShellProvider. */
export function useShells() {
  return useContext(ShellContext)
}

/**
 * The browser's shells: the list (restored from the server on load, so a
 * reload keeps them), the selected tab and whether the panel is open.
 */
export function ShellProvider({ children }) {
  const [shells, setShells] = useState([])
  const [activeId, setActiveId] = useState("")
  const [open, setOpen] = useState(false)
  const [error, setError] = useState(null)

  const refresh = useCallback(async () => {
    try {
      const list = await fetchShells()
      setShells(list)
      setActiveId((id) => (list.some((s) => s.id === id) ? id : list[0]?.id || ""))
    } catch (err) {
      setError(err)
    }
  }, [])

  useEffect(() => {
    refresh()
  }, [refresh])

  const openShell = useCallback(async ({ cwd = "", run = "" } = {}) => {
    setError(null)
    const shell = await createShell({ cwd, run })
    setShells((list) => [...list, shell].sort((a, b) => a.n - b.n))
    setActiveId(shell.id)
    setOpen(true)
    return shell
  }, [])

  const closeShell = useCallback(
    async (id) => {
      setActiveId((current) => (current === id ? neighbourId(shells, id) : current))
      setShells((list) => list.filter((s) => s.id !== id))
      if (shells.length <= 1) setOpen(false)
      try {
        await killShell(id)
      } catch (err) {
        if (err.status !== 404) setError(err)
      }
    },
    [shells]
  )

  const updateShell = useCallback((info) => {
    setShells((list) => list.map((s) => (s.id === info.id ? { ...s, ...info } : s)))
  }, [])

  const select = useCallback((id) => {
    setActiveId(id)
    setOpen(true)
  }, [])

  const value = useMemo(
    () => ({ shells, activeId, open, error, setOpen, setError, select, openShell, closeShell, updateShell, refresh }),
    [shells, activeId, open, error, select, openShell, closeShell, updateShell, refresh]
  )
  return <ShellContext.Provider value={value}>{children}</ShellContext.Provider>
}

function ShellTab({ shell, active, onSelect, onClose }) {
  return (
    <div className={`ah-shell-tab${active ? " is-active" : ""}${shell.exited ? " is-exited" : ""}`} title={shellTitle(shell)}>
      <button
        type="button"
        role="tab"
        className="ah-shell-tab__main"
        id={`ahShellTab-${shell.id}`}
        aria-selected={active}
        aria-controls={`ahShellPanel-${shell.id}`}
        onClick={onSelect}
      >
        <span className="ah-shell-tab__n">{shell.n}</span>
        <span className="ah-shell-tab__folder">{folderName(shell.cwd)}</span>
      </button>
      <button
        type="button"
        className="ah-shell-tab__close"
        onClick={onClose}
        title={`Close shell ${shell.n}`}
        aria-label={`Close shell ${shell.n}`}
      >
        <i className="fa-solid fa-xmark" aria-hidden="true"></i>
      </button>
    </div>
  )
}

const MIN_HEIGHT = 120

/**
 * The terminal panel at the bottom of the main column. Its tab bar is the
 * status bar: minimized, only the bar shows (tab number and folder; the
 * running command is in the tooltip); open, the selected tab's terminal
 * shows under it and the top edge drags to resize. `defaultCwd` is where "+" opens a shell.
 */
export function ShellPanel({ defaultCwd }) {
  const { shells, activeId, open, error, setOpen, setError, select, openShell, closeShell, updateShell, refresh } = useShells()
  const [height, setHeight] = useState(() => Number(readPrefs().shellHeight) || 320)
  const panel = useRef(null)

  const startResize = (ev) => {
    ev.preventDefault()
    const startY = ev.clientY
    const startH = height
    const max = (panel.current?.parentElement?.clientHeight || window.innerHeight) - 120
    let next = startH
    const move = (e) => {
      next = Math.max(MIN_HEIGHT, Math.min(max, startH + startY - e.clientY))
      setHeight(next)
    }
    const up = () => {
      window.removeEventListener("pointermove", move)
      window.removeEventListener("pointerup", up)
      document.body.classList.remove("is-resizing-shell")
      writePref("shellHeight", next)
    }
    document.body.classList.add("is-resizing-shell")
    window.addEventListener("pointermove", move)
    window.addEventListener("pointerup", up)
  }

  const newShell = () => openShell({ cwd: defaultCwd || "" }).catch(setError)
  const shown = open && shells.length > 0

  return (
    <section className={`ah-shells${shown ? " is-open" : ""}`} ref={panel} aria-label="Terminal" id="ahShells">
      {shown ? (
        <div
          className="ah-shells__resize"
          role="separator"
          aria-orientation="horizontal"
          aria-label="Resize terminal"
          onPointerDown={startResize}
        />
      ) : null}
      <div className="ah-shells__bar">
        <button
          type="button"
          className="ah-shells__label"
          onClick={() => (shells.length ? setOpen(!shown) : newShell())}
          title={shells.length ? (shown ? "Minimize the terminal" : "Show the terminal") : "Open a shell"}
        >
          <i className="fa-solid fa-terminal" aria-hidden="true"></i> Terminal
        </button>
        <div className="ah-shells__tabs" role="tablist" aria-label="Shells">
          {shells.map((s) => (
            <ShellTab
              key={s.id}
              shell={s}
              active={shown && s.id === activeId}
              onSelect={() => (shown && s.id === activeId ? setOpen(false) : select(s.id))}
              onClose={() => closeShell(s.id)}
            />
          ))}
        </div>
        <button
          type="button"
          className="ah-icon-btn"
          id="ahShellNew"
          onClick={newShell}
          title={`New shell${defaultCwd ? ` in ${defaultCwd}` : ""}`}
          aria-label="New shell"
        >
          <i className="fa-solid fa-plus" aria-hidden="true"></i>
        </button>
        {error ? (
          <span className="ah-shells__error" role="alert">
            <i className="fa-solid fa-triangle-exclamation" aria-hidden="true"></i> {error.message}
            <button type="button" className="ah-link-btn" onClick={() => setError(null)}>
              Dismiss
            </button>
          </span>
        ) : null}
        {shells.length ? (
          <button
            type="button"
            className="ah-icon-btn ah-shells__toggle"
            id="ahShellToggle"
            onClick={() => setOpen(!shown)}
            title={shown ? "Minimize the terminal" : "Show the terminal"}
            aria-label={shown ? "Minimize the terminal" : "Show the terminal"}
            aria-expanded={shown}
          >
            <i className={`fa-solid ${shown ? "fa-chevron-down" : "fa-chevron-up"}`} aria-hidden="true"></i>
          </button>
        ) : null}
      </div>
      {shells.length ? (
        <div className="ah-shells__body" style={{ height }} hidden={!shown}>
          <Suspense fallback={<p className="ah-empty">Loading terminal…</p>}>
            {shells.map((s) => (
              <Terminal key={s.id} shell={s} active={shown && s.id === activeId} onStatus={updateShell} onGone={refresh} />
            ))}
          </Suspense>
        </div>
      ) : null}
    </section>
  )
}
