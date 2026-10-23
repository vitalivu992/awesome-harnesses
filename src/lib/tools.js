// One-line descriptions and icons for agent tool calls (Claude Code and
// Cursor name their tools differently: Bash/Shell, Edit/StrReplace, …).

const ICONS = [
  [/^(bash|shell|run_terminal_cmd|bashoutput|killshell|monitor)$/i, "fa-terminal"],
  [/^(read|read_file|notebookread)$/i, "fa-file-lines"],
  [/^(edit|multiedit|write|strreplace|str_replace|edit_file|notebookedit|delete|apply_patch)$/i, "fa-pen-to-square"],
  [/^(grep|glob|search|codebase_search|semanticsearch|file_search|ls|list_dir)$/i, "fa-magnifying-glass"],
  [/^(webfetch|websearch|web_search|fetch)$/i, "fa-globe"],
  [/^(task|agent|subagent|sendmessage)$/i, "fa-robot"],
  [/^(todowrite|todo_write|taskcreate|taskupdate)$/i, "fa-list-check"],
  [/^(skill)$/i, "fa-wand-magic-sparkles"],
  [/^(askuserquestion|ask_question)$/i, "fa-circle-question"],
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

/** The most telling part of a tool call's input, e.g. the command or file. */
export function toolSummary(name, input) {
  const i = input && typeof input === "object" ? input : {}
  const path = i.file_path || i.path || i.target_file || i.notebook_path || i.filePath
  switch (String(name || "").toLowerCase()) {
    case "bash":
    case "shell":
    case "run_terminal_cmd":
      return firstLine(i.command || i.description)
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
      return [i.skill, i.args].filter(Boolean).join(" ")
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
