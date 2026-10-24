import { useEffect, useRef } from "react"
import { Terminal as XTerm } from "@xterm/xterm"
import { FitAddon } from "@xterm/addon-fit"
import "@xterm/xterm/css/xterm.css"
import { useTheme } from "../hooks/useTheme.js"
import { attachURL, terminalTheme } from "../lib/shells.js"

const encoder = new TextEncoder()
// Reattach after a dropped WebSocket (a slow tab, the network); the server
// replays the scrollback, so the screen is reset first.
const RETRIES = 3

/**
 * One shell's terminal: xterm.js on the shell's WebSocket. Stays mounted
 * while hidden (another tab, the panel minimized) so its screen is kept.
 */
export default function Terminal({ shell, active, onStatus, onGone }) {
  const box = useRef(null)
  const term = useRef(null)
  const fit = useRef(null)
  const { mode } = useTheme()
  const callbacks = useRef({ onStatus, onGone })
  callbacks.current = { onStatus, onGone }

  useEffect(() => {
    const t = new XTerm({
      cursorBlink: true,
      fontFamily: 'ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, "Liberation Mono", monospace',
      fontSize: 13,
      scrollback: 5000,
      theme: terminalTheme(mode),
    })
    const f = new FitAddon()
    t.loadAddon(f)
    t.open(box.current)
    term.current = t
    fit.current = f

    // Ctrl+Shift+C copies the selection (Ctrl+C is the shell's interrupt).
    t.attachCustomKeyEventHandler((ev) => {
      if (ev.type === "keydown" && ev.ctrlKey && ev.shiftKey && ev.code === "KeyC") {
        const text = t.getSelection()
        if (text) navigator.clipboard?.writeText(text).catch(() => {})
        return false
      }
      return true
    })

    let ws = null
    let closed = false
    let exited = false
    let tries = 0
    let retry = 0
    const sendSize = () => {
      if (ws?.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: "resize", cols: t.cols, rows: t.rows }))
    }
    const connect = () => {
      ws = new WebSocket(attachURL(shell.id))
      ws.binaryType = "arraybuffer"
      let replayed = false
      ws.onopen = () => {
        tries = 0
        sendSize()
      }
      ws.onmessage = (ev) => {
        if (typeof ev.data !== "string") {
          if (!replayed) {
            t.reset()
            replayed = true
          }
          t.write(new Uint8Array(ev.data))
          return
        }
        replayed = true
        let msg = null
        try {
          msg = JSON.parse(ev.data)
        } catch {
          return
        }
        if (msg.type === "status" || msg.type === "exit") callbacks.current.onStatus?.(msg.shell)
        if (msg.type === "exit" && !exited) {
          exited = true
          t.write(`\r\n\x1b[2m[process exited with code ${msg.shell.exitCode}]\x1b[0m\r\n`)
        }
      }
      ws.onclose = () => {
        if (closed || exited) return
        if (tries++ < RETRIES) {
          retry = setTimeout(connect, 500 * tries)
        } else {
          t.write("\r\n\x1b[2m[disconnected]\x1b[0m\r\n")
          callbacks.current.onGone?.()
        }
      }
    }
    connect()

    const input = t.onData((data) => {
      if (ws?.readyState === WebSocket.OPEN) ws.send(encoder.encode(data))
    })
    const binary = t.onBinary((data) => {
      if (ws?.readyState === WebSocket.OPEN) ws.send(Uint8Array.from(data, (c) => c.charCodeAt(0) & 0xff))
    })
    const resized = t.onResize(sendSize)
    // Fit whenever the box changes size (shown, panel dragged, window resized);
    // a hidden box measures 0 and is skipped.
    const observer = new ResizeObserver(() => {
      if (box.current?.offsetWidth && box.current.offsetHeight) f.fit()
    })
    observer.observe(box.current)

    return () => {
      closed = true
      clearTimeout(retry)
      observer.disconnect()
      input.dispose()
      binary.dispose()
      resized.dispose()
      ws?.close()
      t.dispose()
    }
    // One terminal per shell id; the theme is updated below.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [shell.id])

  useEffect(() => {
    if (term.current) term.current.options.theme = terminalTheme(mode)
  }, [mode])

  useEffect(() => {
    if (!active || !term.current) return
    const frame = requestAnimationFrame(() => {
      if (box.current?.offsetWidth) fit.current?.fit()
      term.current?.focus()
    })
    return () => cancelAnimationFrame(frame)
  }, [active])

  return (
    <div
      className="ah-terminal"
      ref={box}
      hidden={!active}
      role="tabpanel"
      id={`ahShellPanel-${shell.id}`}
      aria-labelledby={`ahShellTab-${shell.id}`}
      data-shell={shell.n}
    />
  )
}
