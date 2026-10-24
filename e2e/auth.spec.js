import { expect, test } from "@playwright/test"

// The e2e server (-tags e2e) accepts demo / demo-password only.

test("the API and the app need a login", async ({ page, request }) => {
  expect((await request.get("/api/projects")).status()).toBe(401)
  await page.goto("/")
  await expect(page.locator("#ahLogin")).toBeVisible()
  await expect(page.locator("#ahProjects")).toHaveCount(0)
})

test("logs in with an unlock sequence, then logs out", async ({ page }) => {
  await page.goto("/")
  await page.getByLabel("Username").fill("demo")
  await page.getByLabel("Password").fill("wrong")
  await page.locator("#ahLoginSubmit").click()
  await expect(page.getByRole("alert")).toContainText("Wrong username or password")

  await page.getByLabel("Password").fill("demo-password")
  await page.locator("#ahLoginSubmit").click()
  // The app loads under the unlock sequence, which then removes itself.
  await expect(page.locator("#ahLogin")).toHaveClass(/is-unlocking/)
  await expect(page.getByRole("status")).toHaveAccessibleName("Welcome back, demo")
  await expect(page.locator("#ahProjects")).toContainText("demo_app")
  await expect(page.locator("#ahLogin")).toHaveCount(0, { timeout: 5000 })

  // A reload keeps the session (it is a browser-session cookie) and skips the sequence.
  await page.reload()
  await expect(page.locator("#ahProjects")).toContainText("demo_app")
  await expect(page.locator("#ahLogin")).toHaveCount(0)
  // The panel's footer greets the user by the time of day.
  await expect(page.locator(".ah-user__name")).toHaveText(/^(Good (morning|afternoon|evening)|Hi), demo$/)

  // Log out from the account menu, after a confirmation; the login screen comes in over the app.
  await page.locator("#ahUserMenu").click()
  await page.locator("#ahLogout").click()
  await expect(page.locator("#ahLogoutDialog")).toBeVisible()
  await page.getByRole("button", { name: "Cancel" }).click()
  await expect(page.locator("#ahLogoutDialog")).toBeHidden()
  await expect(page.locator("#ahLogin")).toHaveCount(0)
  await page.locator("#ahUserMenu").click()
  await page.locator("#ahLogout").click()
  await page.locator("#ahLogoutConfirm").click()
  await expect(page.locator("#ahLogin")).toHaveClass(/is-locking/)
  await expect(page.locator("#ahProjects")).toHaveCount(0)
  await expect(page.locator("#ahLogin")).not.toHaveClass(/is-locking/)
  expect((await page.request.get("/api/projects")).status()).toBe(401)
})

test("the account menu switches the theme", async ({ page }) => {
  const res = await page.request.post("/api/auth/login", { data: { username: "demo", password: "demo-password" } })
  expect(res.ok()).toBeTruthy()
  await page.goto("/")
  await page.locator("#ahUserMenu").click()
  await page.getByRole("menuitemradio", { name: "Dark theme" }).click()
  await expect(page.locator("html")).toHaveAttribute("data-color-mode", "dark")
  await expect(page.getByRole("menuitemradio", { name: "Dark theme" })).toHaveAttribute("aria-checked", "true")
  await page.getByRole("menuitemradio", { name: "Light theme" }).click()
  await expect(page.locator("html")).toHaveAttribute("data-color-mode", "light")
  await page.keyboard.press("Escape")
  await expect(page.getByRole("menu")).toHaveCount(0)
})

test("a click skips the unlock sequence", async ({ page }) => {
  // Freeze timers so the sequence can't end on its own before the click (a busy run is slow).
  await page.clock.install({ time: new Date("2026-01-01T00:00:00Z") })
  await page.goto("/")
  await page.clock.pauseAt(new Date("2026-01-01T01:00:00Z"))
  await page.getByLabel("Username").fill("demo")
  await page.getByLabel("Password").fill("demo-password")
  await page.getByLabel("Password").press("Enter")
  await expect(page.locator("#ahLogin")).toHaveClass(/is-unlocking/)
  await page.locator("#ahLogin").click()
  await expect(page.locator("#ahLogin")).toHaveCount(0)
})

test("the session cookie ends with the browser", async ({ page, context }) => {
  await page.goto("/")
  await page.getByLabel("Username").fill("demo")
  await page.getByLabel("Password").fill("demo-password")
  await page.locator("#ahLoginSubmit").click()
  await expect(page.locator("#ahProjects")).toBeVisible()
  const cookie = (await context.cookies()).find((c) => c.name === "ah_session")
  expect(cookie.expires).toBe(-1) // session cookie
  expect(cookie.httpOnly).toBe(true)
  expect(cookie.sameSite).toBe("Strict")
})
