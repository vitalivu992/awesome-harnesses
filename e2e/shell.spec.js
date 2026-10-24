import { expect, test } from "@playwright/test"
import { resolve } from "node:path"

const PROJECT = resolve(".e2e-home/work/demo_app")

test.beforeEach(async ({ page }) => {
  const res = await page.request.post("/api/auth/login", { data: { username: "demo", password: "demo-password" } })
  expect(res.ok()).toBeTruthy()
})

// Shells are server-wide and every page lists them all: one test at a time.
test.describe.configure({ mode: "serial" })

test("opens shells from the project list, streams commands, minimizes and closes", async ({ page }) => {
  await page.goto(`/?project=${encodeURIComponent(PROJECT)}`)
  const row = page.locator("#ahProjects li").first()
  await row.hover()
  await row.getByRole("button", { name: "Open a shell in demo_app" }).click()

  const tabs = page.locator(".ah-shell-tab")
  await expect(tabs).toHaveCount(1)
  await expect(tabs.first()).toContainText("1")
  await expect(tabs.first()).toContainText("demo_app")
  const screen = page.locator(".ah-terminal:not([hidden]) .xterm-rows")
  await expect(screen).toBeVisible()

  // Typed input runs in the project folder and the output streams back.
  await page.locator(".ah-terminal:not([hidden])").click()
  await page.keyboard.type("echo e2e-$((6*7)); pwd")
  await page.keyboard.press("Enter")
  await expect(screen).toContainText("e2e-42")
  await expect(screen).toContainText(PROJECT)

  // The tab shows the running command, then the prompt again.
  await page.keyboard.type("sleep 30")
  await page.keyboard.press("Enter")
  // macOS gives only the program name (proc_darwin.go), Linux the whole command line.
  await expect(tabs.first()).toContainText(process.platform === "darwin" ? "sleep" : "sleep 30")
  await page.keyboard.press("Control+C")
  await expect(tabs.first()).not.toContainText("sleep")

  // A second tab from "+", then switch back.
  await page.locator("#ahShellNew").click()
  await expect(tabs).toHaveCount(2)
  await expect(tabs.nth(1)).toHaveClass(/is-active/)
  await tabs.first().getByRole("tab").click()
  await expect(tabs.first()).toHaveClass(/is-active/)
  await expect(screen).toContainText("e2e-42")

  // Minimize to the status bar and back.
  await page.locator("#ahShellToggle").click()
  await expect(page.locator(".ah-shells__body")).toBeHidden()
  await expect(tabs).toHaveCount(2)
  await tabs.nth(1).getByRole("tab").click()
  await expect(page.locator(".ah-shells__body")).toBeVisible()

  // A reload keeps the shells; close ends them.
  await page.reload()
  await expect(tabs).toHaveCount(2)
  await tabs.first().getByRole("button", { name: "Close shell 1" }).click()
  await expect(tabs).toHaveCount(1)
  await tabs.first().getByRole("button", { name: "Close shell 2" }).click()
  await expect(tabs).toHaveCount(0)
  expect((await (await page.request.get("/api/shells")).json()).shells).toHaveLength(0)
})

test("runs the resume command in a new shell", async ({ page }) => {
  await page.goto(`/?project=${encodeURIComponent(PROJECT)}&c=claude:11111111-aaaa-4bbb-8ccc-000000000001`)
  await page.locator("#ahResume").click()
  await page.locator("#ahResumeShell").click()
  await expect(page.locator(".ah-popover")).toHaveCount(0)
  // claude isn't installed in the test environment: the command is typed and runs.
  await expect(page.locator(".ah-terminal:not([hidden]) .xterm-rows")).toContainText(
    `cd ${PROJECT} && claude --resume 11111111-aaaa-4bbb-8ccc-000000000001`
  )
  await expect(page.locator(".ah-shell-tab")).toHaveCount(1)
  await page.getByRole("button", { name: "Close shell 1" }).click()
  await expect(page.locator(".ah-shell-tab")).toHaveCount(0)
})
