import { expect, test } from "@playwright/test"
import { resolve } from "node:path"

const PROJECT = resolve(".e2e-home/work/demo_app")

// page.request shares the page's cookies, so the session cookie applies to the page.
test.beforeEach(async ({ page }) => {
  const res = await page.request.post("/api/auth/login", { data: { username: "demo", password: "demo-password" } })
  expect(res.ok()).toBeTruthy()
})

test("browses a project's Claude Code and Cursor conversations", async ({ page }) => {
  await page.goto("/")
  // The most recent project opens by itself and lists both harnesses.
  await expect(page.locator("#ahProjects [data-project]").first()).toContainText("demo_app")
  const list = page.locator("#ahConversations")
  await expect(list.locator("[data-conversation]")).toHaveCount(4)
  await expect(list).toContainText("Health endpoint")
  await expect(list).toContainText("Fix the login form")

  await list.getByText("Health endpoint").click()
  await expect(page).toHaveURL(/c=claude%3A11111111/)
  const chat = page.locator("#ahChat")
  await expect(chat.locator(".ah-msg--user")).toContainText("Add a health endpoint")
  await expect(chat.locator(".ah-msg--agent .ah-msg__who")).toHaveText("Claude Code - Opus 5.5")
  // The final answer is rendered markdown; the work is collapsed.
  await expect(chat.locator(".markdown-body table")).toContainText("/health")
  await expect(chat.locator(".ah-steps")).toHaveCount(0)
  await chat.locator(".ah-work__toggle").click()
  await expect(chat.locator(".ah-step-row")).toHaveCount(2)
  await expect(chat.locator(".ah-step--thinking")).toContainText("Check main.go")
  await chat.locator(".ah-step--tool .ah-step__head").click()
  await expect(chat.locator(".ah-code--output")).toContainText("ok  demo")

  await list.getByText("Fix the login form").click()
  await expect(chat.locator(".ah-msg--agent .ah-msg__who")).toHaveText("Cursor")
  await expect(chat.locator(".markdown-body strong")).toHaveText("validates")
})

test("copies the resume command, with bypass permissions", async ({ page, context }) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"])
  await page.goto(`/?project=${encodeURIComponent(PROJECT)}&c=claude:11111111-aaaa-4bbb-8ccc-000000000001`)
  await page.locator("#ahResume").click()
  const command = page.locator("#ahResumeCommand")
  await expect(command).toHaveText(`cd ${PROJECT} && claude --resume 11111111-aaaa-4bbb-8ccc-000000000001`)
  await page.locator("#ahResumeBypass").check()
  await expect(command).toContainText("--dangerously-skip-permissions")
  await page.locator("#ahResumeCopy").click()
  await expect(page.locator("#ahResumeCopy")).toContainText("Copied")
  expect(await page.evaluate(() => navigator.clipboard.readText())).toContain("--dangerously-skip-permissions")
})

test("collapses the projects panel and filters by harness", async ({ page }) => {
  await page.goto(`/?project=${encodeURIComponent(PROJECT)}`)
  await page.getByRole("button", { name: "Collapse projects" }).click()
  await expect(page.locator(".ah-rail__item")).toHaveCount(1)
  await page.getByRole("button", { name: "Expand projects" }).click()
  await expect(page.locator("#ahProjects")).toBeVisible()

  // "All" shows name and count; the other harnesses only their icon until selected.
  const chips = page.locator(".ah-segmented button")
  await expect(chips).toHaveText([/All\s*4/, "", "", "", ""])
  for (const [i, name] of ["All 4", "Claude Code 1", "Cursor 1", "Codex 1", "pi 1"].entries()) {
    await expect(chips.nth(i)).toHaveAccessibleName(name)
  }
  await chips.nth(2).click()
  await expect(chips).toHaveText([/All\s*4/, "", /Cursor\s*1/, "", ""])
  await expect(page.locator("#ahConversations [data-conversation]")).toHaveCount(1)
  await expect(page.locator("#ahConversations")).toContainText("Fix the login form")
})

test("hides projects whose folder is gone until the count is clicked", async ({ page }) => {
  await page.goto("/")
  const rows = page.locator("#ahProjects [data-project]")
  await expect(rows).toHaveCount(1)
  const count = page.locator("#ahProjectsCount")
  await expect(count).toHaveText("1")
  await count.click()
  await expect(count).toHaveText("2")
  await expect(rows).toHaveCount(2)
  await expect(rows.nth(1)).toContainText("old_app")
  await expect(rows.nth(1)).toHaveClass(/is-missing/)
  await count.click()
  await expect(rows).toHaveCount(1)
})

test("shows Codex and pi conversations with their resume commands", async ({ page }) => {
  await page.goto(`/?project=${encodeURIComponent(PROJECT)}&c=codex:cx-1`)
  const chat = page.locator("#ahChat")
  await expect(chat.locator(".ah-msg--agent .ah-msg__who")).toHaveText("Codex - GPT 5.5")
  await expect(chat.locator(".markdown-body strong")).toHaveText("1.27")
  await chat.locator(".ah-work__toggle").click()
  await chat.locator(".ah-step--tool .ah-step__head").click()
  await expect(chat.locator(".ah-code--shell")).toHaveText("go version")
  await expect(chat.locator(".ah-code--output")).toHaveText("go1.27.1")
  await page.locator("#ahResume").click()
  await page.locator("#ahResumeBypass").check()
  await expect(page.locator("#ahResumeCommand")).toHaveText(`cd ${PROJECT} && codex resume cx-1 --dangerously-bypass-approvals-and-sandbox`)

  await page.goto(`/?project=${encodeURIComponent(PROJECT)}&c=pi:pi-1`)
  await expect(chat.locator(".ah-msg--agent .ah-msg__who")).toHaveText("pi - GPT 5.6 Luna")
  await page.locator("#ahResume").click()
  await expect(page.locator("#ahResumeCommand")).toHaveText(`cd ${PROJECT} && pi --session pi-1`)
  await expect(page.locator("#ahResumeBypass")).toHaveCount(0)
  await expect(page.locator("#ahResumeNoBypass")).toContainText("doesn't ask for permissions")
})
