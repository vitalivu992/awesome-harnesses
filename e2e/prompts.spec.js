import { expect, test } from "@playwright/test"
import { mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs"
import { join, resolve } from "node:path"

// Prompts sent from the UI run the fake `claude` of e2e/make-fixtures.mjs (the server's
// LAUNCH_PATH): it appends the prompt and "Echo: <prompt>" to the transcript. This spec has its
// own project, which is the newest while it runs: it runs after the specs that expect demo_app first.
const PROJECT = resolve(".e2e-home/work/prompt_app")
const DIR = resolve(".e2e-home/.claude/projects", `-${PROJECT.replace(/[^A-Za-z0-9]+/g, "-").replace(/^-|-$/g, "")}`)
const ID = "33333333-aaaa-4bbb-8ccc-000000000003"
const LOG = resolve(".e2e-home/claude-args.log")
const project = encodeURIComponent(PROJECT)

const base = { cwd: PROJECT, isSidechain: false }
const lines = (...l) => l.map((x) => JSON.stringify(x)).join("\n") + "\n"
// The argv of the fake claude's last run.
const lastArgs = () => JSON.parse(readFileSync(LOG, "utf8").trim().split("\n").pop())

test.describe.configure({ mode: "serial" })

test.beforeEach(async ({ page }) => {
  rmSync(DIR, { recursive: true, force: true })
  mkdirSync(PROJECT, { recursive: true })
  mkdirSync(DIR, { recursive: true })
  writeFileSync(
    join(DIR, `${ID}.jsonl`),
    lines(
      { ...base, type: "user", timestamp: "2026-10-21T10:00:00Z", message: { role: "user", content: "Plan the release" } },
      {
        ...base,
        type: "assistant",
        timestamp: "2026-10-21T10:00:05Z",
        message: { model: "claude-opus-5-5", content: [{ type: "text", text: "Here is the plan." }], stop_reason: "end_turn" },
      }
    )
  )
  const res = await page.request.post("/api/auth/login", { data: { username: "demo", password: "demo-password" } })
  expect(res.ok()).toBeTruthy()
})

// The project goes away again, so the specs after this one see demo_app first.
test.afterAll(() => rmSync(DIR, { recursive: true, force: true }))

test("sends a message with a skill to a conversation and shows the answer", async ({ page }) => {
  await page.goto(`/?project=${project}&c=claude:${ID}`)
  const chat = page.locator("#ahChat")
  await expect(chat).toContainText("Here is the plan.")

  // "/" offers the harness's commands and skills; Enter picks one.
  const prompt = page.locator("#ahPrompt")
  await prompt.fill("/rev")
  const options = page.getByRole("listbox", { name: "Commands and skills" }).getByRole("option")
  await expect(options).toHaveCount(1)
  await expect(options.first()).toContainText("/review-code")
  await expect(options.first()).toContainText("Review the changed code for bugs")
  await prompt.press("Enter")
  await expect(prompt).toHaveValue("/review-code ")
  await prompt.pressSequentially("the diff")
  // The skill is highlighted and named under the box.
  await expect(page.locator(".ah-prompt__cmd--skill")).toHaveText("/review-code")
  await expect(page.locator(".ah-composer__uses")).toContainText("Uses the skill /review-code")

  await prompt.press("Enter")
  await expect(prompt).toHaveValue("")
  await expect(chat.locator(".ah-msg--agent").last()).toContainText("Echo: /review-code the diff")
  await expect(page.locator("#ahSend")).toBeVisible()
  await expect(page.locator("#ahPending")).toHaveCount(0)
  expect(lastArgs()).toEqual(["-p", "--resume", ID, "--", "/review-code the diff"])
})

test("starts a conversation with the model, effort and fast mode from the settings", async ({ page }) => {
  await page.goto(`/?project=${project}`)

  // Settings: the right panel, from the account menu.
  await page.locator("#ahUserMenu").click()
  await page.locator("#ahOpenSettings").click()
  const claude = page.locator('.ah-hset[data-harness="claude"]')
  await expect(claude.locator(".ah-hset__body")).toBeVisible()
  await claude.getByRole("button", { name: "Add model" }).click()
  await claude.getByLabel("Model 1", { exact: true }).fill("opus")
  await claude.getByLabel("Effort for model 1").selectOption("high")
  await claude.locator("#ahFast-claude").check({ force: true })
  await claude.getByLabel("Preview with model").selectOption("opus")
  await expect(page.locator("#ahPreview-claude")).toContainText(
    `cd ${PROJECT} && claude -p --session-id new-session-uuid --model opus --effort high --settings '{"fastMode":true}' -- 'Your prompt'`
  )
  await page.locator("#ahSettingsWide").click()
  await expect(page.locator("#ahSettings")).toHaveClass(/is-wide/)
  await page.locator("#ahSettingsClose").click()
  await expect(page.locator("#ahSettings")).toHaveCount(0)

  // New: the second column's button; the filter shows all, so the default harness.
  await page.locator("#ahNewConversationBtn").click()
  await expect(page).toHaveURL(/new=claude/)
  const view = page.locator("#ahNewConversation")
  await expect(view.getByRole("radio", { name: "Claude Code" })).toHaveAttribute("aria-checked", "true")
  await expect(page.locator("#ahComposerModel")).toHaveValue("opus")
  await expect(view.locator(".ah-composer__chips")).toContainText("High effort")
  await expect(view.locator(".ah-composer__chips")).toContainText("Fast")
  await page.locator("#ahPrompt").fill("Write the docs")
  await page.locator("#ahComposerCommand").click()
  await expect(page.locator("#ahComposerPreview")).toContainText("--model opus --effort high")
  await expect(page.locator("#ahComposerPreview")).toContainText("'Write the docs'")

  await page.locator("#ahSend").click()
  // The new conversation opens as soon as it has an id, and fills in.
  await expect(page).toHaveURL(/c=claude%3A[0-9a-f-]{36}/)
  await expect(page.locator("#ahChat .ah-msg--agent")).toContainText("Echo: Write the docs")
  await expect(page.locator("#ahConversations")).toContainText("Write the docs")
  const args = lastArgs()
  expect(args.slice(0, 2)).toEqual(["-p", "--session-id"])
  expect(args.slice(3)).toEqual(["--model", "opus", "--effort", "high", "--settings", '{"fastMode":true}', "--", "Write the docs"])
  expect(page.url()).toContain(args[2])
})

test("New uses the harness the filter shows", async ({ page }) => {
  await page.goto(`/?project=${encodeURIComponent(resolve(".e2e-home/work/demo_app"))}`)
  await page.locator(".ah-segmented--harness button[aria-label^='Codex']").click()
  await page.locator("#ahNewConversationBtn").click()
  await expect(page).toHaveURL(/new=codex/)
  await expect(page.locator("#ahNewConversation").getByRole("radio", { name: "Codex" })).toHaveAttribute("aria-checked", "true")
  // The fake home has no codex: the box says so.
  await expect(page.locator(".ah-composer__warn")).toContainText("codex isn't installed")
  await page.locator("#ahNewConversation").getByRole("button", { name: "Cancel" }).click()
  await expect(page).not.toHaveURL(/new=/)
})

test("shows why a prompt failed", async ({ page }) => {
  await page.goto(`/?project=${project}&c=claude:${ID}`)
  await page.locator("#ahPrompt").fill("please fail")
  await page.locator("#ahSend").click()
  const error = page.locator("#ahRunError")
  await expect(error).toContainText("The prompt failed (exit code 2)")
  await expect(error).toContainText("Error: simulated failure")
  await error.getByRole("button", { name: "Dismiss" }).click()
  await expect(error).toHaveCount(0)
})
