import { defineConfig, devices } from "@playwright/test"

// The server runs over a generated fake home (e2e/make-fixtures.mjs), never
// the real ~/.claude and ~/.cursor, and is built with -tags e2e: its login
// accepts only demo / demo-password instead of asking PAM. E2E_BASE_URL points the specs at a
// running server instead (they then need the fixtures served by it).
const baseURL = process.env.E2E_BASE_URL || "http://127.0.0.1:8768"
const home = ".e2e-home"

export default defineConfig({
  testDir: "e2e",
  testMatch: /.*\.spec\.js/,
  fullyParallel: true,
  use: { baseURL, trace: "retain-on-failure" },
  projects: [
    { name: "chromium", testIgnore: /(shell|live)\.spec\.js/, use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } } },
    // The live spec adds a project with the newest transcript: it runs after the specs that expect demo_app first.
    {
      name: "live",
      testMatch: /live\.spec\.js/,
      dependencies: ["chromium"],
      use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } },
    },
    // Logging out (auth.spec) ends every shell, and the live project would be listed first: the shell specs run last.
    {
      name: "shells",
      testMatch: /shell\.spec\.js/,
      dependencies: ["live"],
      use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } },
    },
  ],
  webServer: process.env.E2E_BASE_URL
    ? undefined
    : {
        // Every harness location points into the fake home (the databases don't exist there).
        command:
          `node e2e/make-fixtures.mjs && pnpm build && PORT=8768 CLAUDE_DIR=${home}/.claude CURSOR_DIR=${home}/.cursor CURSOR_LOGS_DIR=${home}/.cursor-server/data/logs ` +
          `CODEX_DIR=${home}/.codex PI_DIR=${home}/.pi OPENCODE_DB=${home}/opencode.db KILO_DB=${home}/kilo.db go run -tags e2e ./cmd/server`,
        url: `${baseURL}/health`,
        reuseExistingServer: !process.env.CI,
        timeout: 120_000,
      },
})
