// One-line descriptions and icons for agent tool calls. Each harness names
// its tools differently: Bash (Claude Code), Shell (Cursor), exec_command /
// shell (Codex), bash (pi, OpenCode, Kilo); Edit / StrReplace / apply_patch / edit…

const ICONS = [
  [/^(bash|shell|run_terminal_cmd|bashoutput|killshell|monitor|exec_command|shell_command|write_stdin|exec|codemode)$/i, "fa-terminal"],
  [/^(read|read_file|notebookread|view_image)$/i, "fa-file-lines"],
  [/^(edit|multiedit|write|strreplace|str_replace|edit_file|notebookedit|delete|apply_patch|patch|multi_edit)$/i, "fa-pen-to-square"],
  [/^(grep|glob|search|codebase_search|semanticsearch|file_search|ls|list|list_dir|find|tool_search)$/i, "fa-magnifying-glass"],
  [/^(webfetch|websearch|web_search|fetch)$/i, "fa-globe"],
  [/^(task|agent|subagent|sendmessage)$/i, "fa-robot"],
  [/^(todowrite|todo_write|taskcreate|taskupdate|update_plan|todoread)$/i, "fa-list-check"],
  [/^(skill)$/i, "fa-wand-magic-sparkles"],
  [/^(askuserquestion|ask_question|request_user_input|question)$/i, "fa-circle-question"],
  [/^mcp/i, "fa-plug"],
]

export function toolIcon(name) {
  for (const [re, icon] of ICONS) if (re.test(String(name || ""))) return icon
  return "fa-wrench"
}

function shortPath(path) {
  const s = String(path || "")
  const parts = s.split("/").filter(Boolean)
  return parts.length > 3 ? `…/${parts.slice(-3).join("/")}` : s
}

function firstLine(s, max = 160) {
  const line = String(s || "").trim().split("\n")[0]
  return line.length > max ? `${line.slice(0, max)}…` : line
}

/**
 * A shell command from a tool input: a string, or an argv array where
 * ["bash", "-lc", "<script>"] means the script (Codex `shell`).
 */
export function shellCommand(input) {
  const i = input && typeof input === "object" ? input : {}
  const cmd = i.command ?? i.cmd
  if (Array.isArray(cmd)) {
    const k = cmd.findIndex((a) => a === "-lc" || a === "-c")
    return k >= 0 && k + 1 < cmd.length ? String(cmd[k + 1]) : cmd.join(" ")
  }
  return typeof cmd === "string" ? cmd : ""
}

/** The files an apply_patch input touches ("*** Update File: path"). */
function patchFiles(patch) {
  return [...String(patch).matchAll(/^\*\*\* (?:Add|Update|Delete) File: (.+)$/gm)].map((m) => shortPath(m[1].trim()))
}

/** The most telling part of a tool call's input, e.g. the command or file. */
export function toolSummary(name, input) {
  const tool = String(name || "").toLowerCase()
  // Freeform inputs: Codex apply_patch (a patch) and exec (code-mode JavaScript).
  if (typeof input === "string") {
    if (tool === "apply_patch") return patchFiles(input).join(", ") || firstLine(input)
    // Code mode: `await tools.exec_command({cmd: "…"})` — show the command.
    const cmd = input.match(/\bcmd:\s*"((?:[^"\\]|\\.)*)"/)
    if (cmd) return firstLine(cmd[1].replace(/\\"/g, '"'))
    return firstLine(input)
  }
  const i = input && typeof input === "object" ? input : {}
  const path = i.file_path || i.path || i.target_file || i.notebook_path || i.filePath
  switch (tool) {
    case "bash":
    case "shell":
    case "run_terminal_cmd":
    case "exec_command":
    case "shell_command":
      return firstLine(shellCommand(i) || i.description)
    case "update_plan":
      return Array.isArray(i.plan) ? `${i.plan.length} steps` : ""
    case "read":
    case "read_file":
      return shortPath(path) + (i.offset ? ` (from line ${i.offset})` : "")
    case "grep":
      return `${i.pattern ?? ""}${path ? ` in ${shortPath(path)}` : ""}${i.glob ? ` (${i.glob})` : ""}`
    case "glob":
      return `${i.pattern ?? i.glob_pattern ?? ""}${path ? ` in ${shortPath(path)}` : ""}`
    case "webfetch":
    case "fetch":
      return i.url || ""
    case "websearch":
    case "web_search":
      return i.query || i.search_term || ""
    case "task":
    case "agent":
    case "subagent":
      return firstLine(i.description || i.prompt)
    case "skill":
      return [i.skill || i.name, i.args].filter(Boolean).join(" ")
    case "todowrite":
    case "todo_write":
      return Array.isArray(i.todos) ? `${i.todos.length} todos` : ""
  }
  if (path) return shortPath(path)
  for (const key of ["command", "pattern", "query", "url", "description", "prompt", "name"]) {
    if (typeof i[key] === "string" && i[key]) return firstLine(i[key])
  }
  return ""
}

/** "Bash", or "playwright · browser_click" for an MCP tool (mcp__server__tool). */
export function toolLabel(name) {
  const m = String(name || "").match(/^mcp__(.+?)__(.+)$/)
  return m ? `${m[1]} · ${m[2]}` : String(name || "tool")
}
