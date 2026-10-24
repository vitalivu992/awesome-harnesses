import { forwardRef, useEffect, useImperativeHandle, useLayoutEffect, useRef, useState } from "react"
import { applyCompletion, completionAt, highlightSegments, matchCommands } from "../lib/prompt.js"

const MAX_HEIGHT = 320

/**
 * The prompt textarea. Typing "/" at the start (or "$" anywhere, for Codex
 * skills) opens the harness's commands and skills; ↑/↓ pick, Enter or Tab
 * inserts, Esc closes. A known command in the text is highlighted: the text
 * is drawn by a mirror under the transparent textarea, with the command
 * marked. Enter sends, Shift+Enter is a new line.
 */
export const PromptBox = forwardRef(function PromptBox(
  { value, onChange, onSubmit, commands, mentions = false, disabled = false, placeholder, autoFocus = false, id },
  ref
) {
  const area = useRef(null)
  const mirror = useRef(null)
  const list = useRef(null)
  const [token, setToken] = useState(null)
  const [active, setActive] = useState(0)
  const [closedAt, setClosedAt] = useState(null)

  useImperativeHandle(ref, () => ({ focus: () => area.current?.focus() }), [])

  const matches = token ? matchCommands(commands, token.query) : []
  const open = Boolean(token && matches.length && closedAt !== token.start)

  // Grow with the text up to MAX_HEIGHT, then scroll (the mirror follows).
  useLayoutEffect(() => {
    const el = area.current
    if (!el) return
    el.style.height = "auto"
    el.style.height = `${Math.min(el.scrollHeight, MAX_HEIGHT)}px`
    if (mirror.current) mirror.current.scrollTop = el.scrollTop
  }, [value])

  useEffect(() => {
    if (autoFocus) area.current?.focus()
  }, [autoFocus])

  useEffect(() => setActive(0), [token?.query])

  useEffect(() => {
    list.current?.querySelector(".is-active")?.scrollIntoView({ block: "nearest" })
  }, [active, open])

  const track = (el) => {
    const t = el.selectionStart === el.selectionEnd ? completionAt(el.value, el.selectionStart, { mentions }) : null
    setToken(t)
    if (!t) setClosedAt(null)
  }

  const choose = (cmd) => {
    if (!token) return
    const next = applyCompletion(value, token, cmd.insert)
    onChange(next.text)
    setToken(null)
    requestAnimationFrame(() => {
      const el = area.current
      if (!el) return
      el.focus()
      el.setSelectionRange(next.caret, next.caret)
    })
  }

  const onKeyDown = (e) => {
    if (open) {
      if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        e.preventDefault()
        const step = e.key === "ArrowDown" ? 1 : -1
        setActive((i) => (i + step + matches.length) % matches.length)
        return
      }
      if (e.key === "Enter" || e.key === "Tab") {
        e.preventDefault()
        choose(matches[active] || matches[0])
        return
      }
      if (e.key === "Escape") {
        e.preventDefault()
        setClosedAt(token.start)
        return
      }
    }
    if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault()
      onSubmit?.()
    }
  }

  const segments = highlightSegments(value, commands)

  return (
    <div className={`ah-prompt${disabled ? " is-disabled" : ""}`}>
      <div className="ah-prompt__mirror" ref={mirror} aria-hidden="true">
        {segments.map((seg, i) =>
          seg.command ? (
            <mark key={i} className={`ah-prompt__cmd ah-prompt__cmd--${seg.command.kind}`}>
              {seg.text}
            </mark>
          ) : (
            <span key={i}>{seg.text}</span>
          )
        )}
        {/* A trailing newline only takes room with something after it. */}
        {"\u200b"}
      </div>
      <textarea
        ref={area}
        id={id}
        className="ah-prompt__input"
        rows={1}
        value={value}
        placeholder={placeholder}
        disabled={disabled}
        spellCheck="true"
        aria-label="Prompt"
        aria-autocomplete="list"
        aria-expanded={open}
        aria-controls={open ? `${id}-commands` : undefined}
        aria-activedescendant={open ? `${id}-command-${active}` : undefined}
        onChange={(e) => {
          onChange(e.target.value)
          track(e.target)
        }}
        onSelect={(e) => track(e.currentTarget)}
        onKeyDown={onKeyDown}
        onBlur={() => setTimeout(() => setToken(null), 120)}
        onScroll={(e) => mirror.current && (mirror.current.scrollTop = e.currentTarget.scrollTop)}
      />
      {open ? (
        <ul className="ah-complete" role="listbox" id={`${id}-commands`} ref={list} aria-label="Commands and skills">
          {matches.map((c, i) => (
            <li
              key={c.insert}
              id={`${id}-command-${i}`}
              role="option"
              aria-selected={i === active}
              className={`ah-complete__item${i === active ? " is-active" : ""}`}
              onMouseDown={(e) => {
                e.preventDefault()
                choose(c)
              }}
              onMouseEnter={() => setActive(i)}
            >
              <i className={`fa-solid ${c.kind === "skill" ? "fa-wand-magic-sparkles" : "fa-terminal"}`} aria-hidden="true"></i>
              <span className="ah-complete__name">{c.insert}</span>
              <span className="ah-complete__desc">{c.description}</span>
              <span className="ah-complete__source">{c.source === "user" || c.source === "project" ? c.source : `plugin ${c.source}`}</span>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  )
})
