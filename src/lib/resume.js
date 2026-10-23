// The terminal command that continues a conversation in its harness CLI.

const SAFE = /^[A-Za-z0-9_./:@%+=,-]+$/

/** POSIX shell quoting: bare when safe, else single quotes. */
export function shellQuote(value) {
  const s = String(value ?? "")
  if (s && SAFE.test(s)) return s
  return `'${s.replace(/'/g, `'\\''`)}'`
}

/**
 * `cd <cwd> && claude --resume <id>` (Claude Code) or
 * `cd <cwd> && cursor-agent --resume <id>` (Cursor); `bypass` adds the
 * flag that skips permission prompts (--dangerously-skip-permissions /
 * --force). Both CLIs look sessions up by the directory they run in, hence the cd.
 */
export function resumeCommand(conversation, { bypass = false } = {}) {
  if (!conversation?.id) return ""
  const cli =
    conversation.harness === "cursor"
      ? ["cursor-agent", "--resume", shellQuote(conversation.id), bypass && "--force"]
      : ["claude", "--resume", shellQuote(conversation.id), bypass && "--dangerously-skip-permissions"]
  const command = cli.filter(Boolean).join(" ")
  const cwd = conversation.cwd || conversation.projectPath
  return cwd ? `cd ${shellQuote(cwd)} && ${command}` : command
}

export function bypassFlag(harness) {
  return harness === "cursor" ? "--force" : "--dangerously-skip-permissions"
}
