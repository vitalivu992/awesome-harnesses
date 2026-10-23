import { defineConfig, devices } from "@playwright/test"

// The server runs over a generated fake home (e2e/make-fixtures.mjs), never
// the real ~/.claude and ~/.cursor. E2E_BASE_URL points the specs at a
// running server instead (they then need the fixtures served by it).
const baseURL = process.env.E2E_BASE_URL || "http://127.0.0.1:8768"
const home = ".e2e-home"

export default defineConfig({
  testDir: "e2e",
  testMatch: /.*\.spec\.js/,
  fullyParallel: true,
  use: { baseURL, trace: "retain-on-failure" },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } } }],
  webServer: process.env.E2E_BASE_URL
    ? undefined
    : {
        command: `node e2e/make-fixtures.mjs && pnpm build && PORT=8768 CLAUDE_DIR=${home}/.claude CURSOR_DIR=${home}/.cursor go run ./cmd/server`,
        url: `${baseURL}/health`,
        reuseExistingServer: !process.env.CI,
        timeout: 120_000,
      },
})
