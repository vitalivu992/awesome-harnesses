import { readPrefs, writePref } from "./prefs.js"

// Theme pref "system" | "light" | "dark", applied as Primer's colour-mode
// attributes on <html> so Primer's tokens (--bgColor-default, …) follow it.
// index.html runs the same steps inline before first paint.

const DARK_QUERY = "(prefers-color-scheme: dark)"
export const THEMES = ["system", "light", "dark"]

export function resolveColorMode(pref, systemDark) {
  if (pref === "light" || pref === "dark") return pref
  return systemDark ? "dark" : "light"
}

let state = null
const listeners = new Set()

function compute() {
  const pref = THEMES.includes(readPrefs().theme) ? readPrefs().theme : "system"
  const dark = typeof matchMedia === "function" && matchMedia(DARK_QUERY).matches
  return { pref, mode: resolveColorMode(pref, dark) }
}

function update() {
  const next = compute()
  if (state && next.pref === state.pref && next.mode === state.mode) return
  state = next
  const root = document.documentElement
  root.setAttribute("data-color-mode", state.mode)
  root.setAttribute("data-light-theme", "light")
  root.setAttribute("data-dark-theme", "dark")
  for (const listener of listeners) listener()
}

export function initTheme() {
  update()
  if (typeof matchMedia === "function") matchMedia(DARK_QUERY).addEventListener("change", update)
}

export function getThemeState() {
  if (!state) state = compute()
  return state
}

export function subscribeTheme(listener) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function setTheme(pref) {
  writePref("theme", pref)
  update()
}
