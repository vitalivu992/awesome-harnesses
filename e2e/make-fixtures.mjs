// Writes a fake home for e2e: one project used from Claude Code, Cursor, Codex and pi.
// Run by playwright.config.js before it starts the server.
import { mkdirSync, rmSync, writeFileSync } from "node:fs"
import { dirname, join, resolve } from "node:path"

export const HOME = resolve(".e2e-home")
export const PROJECT = join(HOME, "work", "demo_app")

const slug = (p) => p.replace(/[^A-Za-z0-9]+/g, "-").replace(/^-|-$/g, "")

function write(path, lines) {
  mkdirSync(dirname(path), { recursive: true })
  writeFileSync(path, lines.map((l) => JSON.stringify(l)).join("\n") + "\n")
}

export function makeFixtures() {
  rmSync(HOME, { recursive: true, force: true })
  mkdirSync(PROJECT, { recursive: true })
  const base = { cwd: PROJECT, gitBranch: "main", isSidechain: false }
  write(join(HOME, ".claude", "projects", `-${slug(PROJECT)}`, "11111111-aaaa-4bbb-8ccc-000000000001.jsonl"), [
    { ...base, type: "user", timestamp: "2026-10-20T10:00:00Z", message: { role: "user", content: "Add a health endpoint" } },
    {
      ...base,
      type: "assistant",
      timestamp: "2026-10-20T10:00:02Z",
      thinkingDurationMs: 1200,
      message: { model: "claude-opus-5-5", content: [{ type: "thinking", thinking: "Check main.go for the router." }], stop_reason: "tool_use" },
    },
    {
      ...base,
      type: "assistant",
      timestamp: "2026-10-20T10:00:03Z",
      message: { model: "claude-opus-5-5", content: [{ type: "tool_use", id: "t1", name: "Bash", input: { command: "go test ./...", description: "Run tests" } }], stop_reason: "tool_use" },
    },
    { ...base, type: "user", timestamp: "2026-10-20T10:00:08Z", message: { role: "user", content: [{ type: "tool_result", tool_use_id: "t1", content: "ok  demo 0.01s" }] } },
    {
      ...base,
      type: "assistant",
      timestamp: "2026-10-20T10:00:10Z",
      message: {
        model: "claude-opus-5-5",
        content: [{ type: "text", text: "## Done\n\nAdded `GET /health`.\n\n| Route | Status |\n|---|---|\n| /health | 200 |" }],
        stop_reason: "end_turn",
      },
    },
    { type: "ai-title", aiTitle: "Health endpoint" },
  ])
  write(join(HOME, ".cursor", "projects", slug(PROJECT), "agent-transcripts", "c-1", "c-1.jsonl"), [
    { role: "user", message: { content: [{ type: "text", text: "<timestamp>Monday, Oct 19, 2026, 9:05 AM (UTC)</timestamp>\n<user_query>\nFix the login form\n</user_query>" }] } },
    { role: "assistant", message: { content: [{ type: "tool_use", name: "Read", input: { path: `${PROJECT}/login.js` } }] } },
    { role: "assistant", message: { content: [{ type: "text", text: "The form now **validates** the email." }] } },
    { type: "turn_ended", status: "success" },
  ])
  write(join(HOME, ".codex", "sessions", "2026", "10", "18", "rollout-2026-10-18T08-00-00-cx-1.jsonl"), [
    { timestamp: "2026-10-18T08:00:00Z", type: "session_meta", payload: { id: "cx-1", cwd: PROJECT, source: "cli" } },
    { timestamp: "2026-10-18T08:00:00Z", type: "turn_context", payload: { cwd: PROJECT, model: "gpt-5.5", summary: "auto" } },
    { timestamp: "2026-10-18T08:00:01Z", type: "response_item", payload: { type: "message", role: "user", content: [{ type: "input_text", text: "Bump the Go version" }] } },
    {
      timestamp: "2026-10-18T08:00:02Z",
      type: "response_item",
      payload: { type: "function_call", name: "exec_command", arguments: JSON.stringify({ cmd: "go version" }), call_id: "k1" },
    },
    { timestamp: "2026-10-18T08:00:03Z", type: "response_item", payload: { type: "function_call_output", call_id: "k1", output: "go1.27.1" } },
    { timestamp: "2026-10-18T08:00:04Z", type: "response_item", payload: { type: "message", role: "assistant", content: [{ type: "output_text", text: "Bumped to **1.27**." }] } },
    { timestamp: "2026-10-18T08:00:05Z", type: "event_msg", payload: { type: "task_complete" } },
  ])
  write(join(HOME, ".pi", "agent", "sessions", "--demo--", "2026-10-17T08-00-00_pi-1.jsonl"), [
    { type: "session", version: 3, id: "pi-1", timestamp: "2026-10-17T08:00:00Z", cwd: PROJECT },
    { type: "message", id: "u1", parentId: null, timestamp: "2026-10-17T08:00:01Z", message: { role: "user", content: [{ type: "text", text: "Explain the router" }] } },
    {
      type: "message",
      id: "a1",
      parentId: "u1",
      timestamp: "2026-10-17T08:00:03Z",
      message: { role: "assistant", content: [{ type: "text", text: "It uses `http.ServeMux`." }], model: "gpt-5.6-luna", stopReason: "stop" },
    },
  ])
}

if (import.meta.url === `file://${process.argv[1]}`) makeFixtures()
