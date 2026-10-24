import { expect, test } from "@playwright/test"
import { appendFileSync, mkdirSync, rmSync, writeFileSync } from "node:fs"
import { join, resolve } from "node:path"

// A session the agent is still working on, in its own project so the other
// specs' project order and counts don't change (this project runs after them).
const LIVE = resolve(".e2e-home/work/live_app")
const DIR = resolve(".e2e-home/.claude/projects", `-${LIVE.replace(/[^A-Za-z0-9]+/g, "-").replace(/^-|-$/g, "")}`)
const ID = "22222222-aaaa-4bbb-8ccc-000000000002"
const FILE = join(DIR, `${ID}.jsonl`)

const base = { cwd: LIVE, isSidechain: false }
const lines = (...l) => l.map((x) => JSON.stringify(x)).join("\n") + "\n"
const toolUse = (id, command, at) => ({
  ...base,
  type: "assistant",
  timestamp: at,
  message: { model: "claude-opus-5-5", content: [{ type: "tool_use", id, name: "Bash", input: { command } }], stop_reason: "tool_use" },
})
const toolResult = (id, content, at) => ({ ...base, type: "user", timestamp: at, message: { role: "user", content: [{ type: "tool_result", tool_use_id: id, content }] } })

test.beforeEach(async ({ page }) => {
  mkdirSync(LIVE, { recursive: true })
  mkdirSync(DIR, { recursive: true })
  writeFileSync(
    FILE,
    lines(
      { ...base, type: "user", timestamp: "2026-10-20T10:00:00Z", message: { role: "user", content: "Run the test suite" } },
      toolUse("t1", "go test ./...", "2026-10-20T10:00:02Z"),
      toolResult("t1", "ok  live 0.01s", "2026-10-20T10:00:04Z")
    )
  )
  const res = await page.request.post("/api/auth/login", { data: { username: "demo", password: "demo-password" } })
  expect(res.ok()).toBeTruthy()
})

test.afterEach(() => rmSync(FILE, { force: true }))

test("follows a working session and collapses its work when it ends", async ({ page }) => {
  await page.goto(`/?project=${encodeURIComponent(LIVE)}&c=claude:${ID}`)
  const chat = page.locator("#ahChat")
  // Working: the work is shown and the turn says so.
  await expect(page.locator("#ahExpandAll")).toHaveText(/Hide work/)
  await expect(page.locator(".ah-chip--live")).toHaveText(/Working/)
  await expect(chat.locator(".ah-turn-status--live")).toHaveText(/Working…/)
  await expect(chat.locator(".ah-step-row")).toHaveCount(1)

  // The agent writes another tool call: it shows up without a reload.
  appendFileSync(FILE, lines(toolUse("t2", "go vet ./...", "2026-10-20T10:00:06Z"), toolResult("t2", "", "2026-10-20T10:00:07Z")))
  await expect(chat.locator(".ah-step-row")).toHaveCount(2)

  // The turn ends: the work collapses and the final answer is left.
  appendFileSync(
    FILE,
    lines({
      ...base,
      type: "assistant",
      timestamp: "2026-10-20T10:00:09Z",
      message: { model: "claude-opus-5-5", content: [{ type: "text", text: "All **green**." }], stop_reason: "end_turn" },
    })
  )
  await expect(chat.locator(".markdown-body strong")).toHaveText("green")
  await expect(chat.locator(".ah-steps")).toHaveCount(0)
  await expect(chat.locator(".ah-turn-status--live")).toHaveCount(0)
  await expect(page.locator("#ahExpandAll")).toHaveText(/Show work/)
  await expect(page.locator(".ah-chip--live")).toHaveText(/Live/)
})
