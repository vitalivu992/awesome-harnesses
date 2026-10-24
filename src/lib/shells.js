// Pure helpers for the terminal panel (components/Shells.jsx, Terminal.jsx).

/** The last folder of a POSIX or Windows path: "/a/b/" → "b", "C:\\x" → "x", "/" → "/". */
export function folderName(path) {
  const s = String(path || "")
  const trimmed = s.replace(/[\\/]+$/, "")
  if (!trimmed) return s ? s[0] : ""
  const parts = trimmed.split(/[\\/]/)
  return parts[parts.length - 1] || trimmed
}

/** Tooltip of a shell's tab: "Shell 2 · ~/x/app · npm test", or "… · exited (1)". */
export function shellTitle(shell) {
  const parts = [`Shell ${shell.n}`, shell.cwd]
  if (shell.exited) parts.push(`exited (${shell.exitCode})`)
  else if (shell.command) parts.push(shell.command)
  return parts.filter(Boolean).join(" · ")
}

/** The WebSocket URL that attaches to shell `id`, for the page at `loc`. */
export function attachURL(id, loc = window.location) {
  const scheme = loc.protocol === "https:" ? "wss:" : "ws:"
  return `${scheme}//${loc.host}/api/shells/${encodeURIComponent(id)}/attach`
}

/** The next tab to show when `id` closes: the one after it, else the one before. */
export function neighbourId(shells, id) {
  const i = shells.findIndex((s) => s.id === id)
  if (i < 0) return shells[0]?.id || ""
  return (shells[i + 1] || shells[i - 1])?.id || ""
}

// xterm.js colours per colour mode (GitHub's terminal palette).
const THEMES = {
  light: {
    background: "#ffffff",
    foreground: "#1f2328",
    cursor: "#0969da",
    selectionBackground: "#0969da33",
    black: "#24292f",
    red: "#cf222e",
    green: "#116329",
    yellow: "#4d2d00",
    blue: "#0969da",
    magenta: "#8250df",
    cyan: "#1b7c83",
    white: "#6e7781",
    brightBlack: "#57606a",
    brightRed: "#a40e26",
    brightGreen: "#1a7f37",
    brightYellow: "#633c01",
    brightBlue: "#218bff",
    brightMagenta: "#a475f9",
    brightCyan: "#3192aa",
    brightWhite: "#8c959f",
  },
  dark: {
    background: "#0d1117",
    foreground: "#e6edf3",
    cursor: "#4493f8",
    selectionBackground: "#4493f844",
    black: "#484f58",
    red: "#ff7b72",
    green: "#3fb950",
    yellow: "#d29922",
    blue: "#58a6ff",
    magenta: "#bc8cff",
    cyan: "#39c5cf",
    white: "#b1bac4",
    brightBlack: "#6e7681",
    brightRed: "#ffa198",
    brightGreen: "#56d364",
    brightYellow: "#e3b341",
    brightBlue: "#79c0ff",
    brightMagenta: "#d2a8ff",
    brightCyan: "#56d4dd",
    brightWhite: "#ffffff",
  },
}

export function terminalTheme(mode) {
  return THEMES[mode] || THEMES.light
}
