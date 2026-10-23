// The terminal command that continues a conversation in its harness CLI.

const SAFE = /^[A-Za-z0-9_./:@%+=,-]+$/

/** POSIX shell quoting: bare when safe, else single quotes. */
export function shellQuote(value) {
  const s = String(value ?? "")
  if (s && SAFE.test(s)) return s
  return `'${s.replace(/'/g, `'\\''`)}'`
}

// Per harness: the resume argv and the flag that skips permission prompts
// ("" when the interactive CLI has none, with `noBypass` saying why).
const RESUME = {
  claude: { argv: (id) => ["claude", "--resume", id], bypass: "--dangerously-skip-permissions" },
  cursor: { argv: (id) => ["cursor-agent", "--resume", id], bypass: "--force" },
  codex: { argv: (id) => ["codex", "resume", id], bypass: "--dangerously-bypass-approvals-and-sandbox" },
  pi: { argv: (id) => ["pi", "--session", id], bypass: "", noBypass: "pi doesn't ask for permissions, so there is nothing to bypass." },
  opencode: { argv: (id) => ["opencode", "--session", id], bypass: "--auto" },
  kilo: {
    argv: (id) => ["kilo", "--session", id],
    bypass: "",
    noBypass: "The Kilo TUI has no bypass flag (only non-interactive `kilo run` has one).",
  },
}

/**
 * `cd <cwd> && <cli> <resume args> <id>` for the conversation's harness;
 * `bypass` adds the flag that skips permission prompts where there is one.
 * The CLIs look sessions up by (or resume into) the directory they run in,
 * hence the cd.
 */
export function resumeCommand(conversation, { bypass = false } = {}) {
  const spec = RESUME[conversation?.harness]
  if (!spec || !conversation.id) return ""
  const argv = spec.argv(shellQuote(conversation.id))
  if (bypass && spec.bypass) argv.push(spec.bypass)
  const command = argv.join(" ")
  const cwd = conversation.cwd || conversation.projectPath
  return cwd ? `cd ${shellQuote(cwd)} && ${command}` : command
}

/** The harness's bypass-permissions flag, or "". */
export function bypassFlag(harness) {
  return RESUME[harness]?.bypass || ""
}

/** Why a harness has no bypass flag, or "". */
export function noBypassReason(harness) {
  return RESUME[harness]?.noBypass || ""
}
