# AGENTS.md

User-facing features and what data is read: [README.md](README.md).

## Commands

```bash
make build                  # pnpm build → web/dist (go:embed) + go build -o bin/server
make dev | make dev-web     # Go server on :8767 | Vite HMR on :5173 proxying /api
make run                    # build, then bin/server in the foreground with .env applied
make up|down|install|logs   # per-user service via ctl.sh: systemd user unit (Linux),
                            # launchd agent dev.awesome-harness (macOS; plist generated)
go test ./...               # Go tests (parsers, catalog, api)
pnpm test:unit              # vitest (tests/*.test.mjs; JSX helpers import fine)
pnpm test:e2e               # playwright: e2e/make-fixtures.mjs writes a fake home to .e2e-home/,
                            # then the config starts the server on :8768 over it — every harness
                            # location env var must point there (add one when adding a harness).
                            # It runs `go run -tags e2e`: the login accepts demo / demo-password
```

The server needs cgo on Linux (PAM headers, `libpam0g-dev`) and macOS (Xcode CLT); Windows
is pure Go (`GOOS=windows go build ./cmd/server` cross-compiles). JS tooling is **pnpm**. `@playwright/test` is pinned to 1.60.0 to match the browsers installed
on this machine. `web/dist/.gitkeep` keeps the embed pattern valid on a fresh clone; run
`make web` before building for real pages.

## Architecture

Read-only Go server + React SPA, one binary, plus the terminal panel's shells (`internal/shell`). No database of its own: transcripts (and the
OpenCode/Kilo databases) are read on request and summaries cached in memory by file size + mtime
(session update time for database rows). Harness ids (`conv.Claude` … `conv.Kilo`) are the same
strings in the API, URLs and `src/lib/format.js` `HARNESSES`.

- `internal/conv` — the harness-neutral model: `Conversation` = `Summary` + `Entries`; an entry
  is a `turn` (prompt, steps, final answer, status) or an `event` (compaction, local command
  output). `Entry.SplitFinal` moves the trailing text steps of a turn into `Final`, so "final
  answer" means the text after the last tool call or thought. `Finish` derives counts, models,
  status and a fallback title. Long tool inputs/outputs are capped (`MaxText`).
- `internal/claude` — Claude Code JSONL. An assistant message spans several lines (one per
  content block); tool results arrive in the next user line and are matched by `tool_use_id`.
  Skipped: `isSidechain` lines, `isMeta` user lines (skill text, caveats), `<system-reminder>`.
  `[Request interrupted by user…]` marks the turn aborted; `isApiErrorMessage` marks it error;
  a slash/`!` command the agent never answered becomes an event. Turn status otherwise comes from
  the last `stop_reason` (`end_turn` → done, else incomplete).
- `internal/cursor` — Cursor agent transcripts (no model, no tool output, no per-line times).
  The prompt is the `<user_query>` (else the text minus leading `<tag>…</tag>` context blocks);
  trailing `[REDACTED]` is dropped. A user line with only context blocks (subagent catalog, a
  bare `<timestamp>`) is not a prompt and starts no turn. `turn_ended` gives the status (abort errors → aborted);
  transcripts from before mid-2026 have no markers, so a turn with a final answer counts as done.
  `store.go` reads the cursor-agent chat store (hex JSON meta + newest `"modelName"` in blobs) and
  the AI code tracking db, both opened `mode=ro&immutable=1` so Cursor's locks are never touched.
  `logs.go` reads app-chat turns from the Cursor server logs
  (`<logs>/*/exthost*/anysphere.cursor-agent-host/*.log`: "Selected Agent Host turn runtime" =
  start + `modelId`, "Turn terminal blob reads" = end; local times, no zone) and matches them to
  turns whose prompt `<timestamp>` (minute precision) is within 90 s, in order. App chats keep
  their metadata on the editor's machine (Remote-SSH), so these logs are the only local source.
- `internal/codex` — Codex rollouts. Only `response_item` lines build the chat (the `event_msg`
  user/agent messages duplicate them); user messages that are injected context
  (`<environment_context>`, `# AGENTS.md instructions`, `<codex_internal_context`…) are skipped,
  `<turn_aborted>` marks the turn aborted. Tool calls pair with outputs by `call_id`;
  `function_call.arguments` is a JSON string (decoded), `custom_tool_call.input` (apply_patch,
  code-mode exec) a raw string. Older outputs are `{"output", "metadata": {"exit_code"}}`. Status
  from `task_complete` / `turn_aborted` / `error`; model from `turn_context` /
  `thread_settings_applied`. Careful: field shapes vary by line type (`summary` is a string in
  `turn_context`, an array in `reasoning`) — decode them as `json.RawMessage`, or the whole line
  fails to decode. `ReadMeta` (first line) gives id/cwd/subagent for listing; `ReadIndex` the
  thread names.
- `internal/pi` — pi sessions: entries form a tree via `parentId`; `activeBranch` walks from the
  last entry to the root. `toolCall` ↔ `toolResult` by id; status from `stopReason`;
  `bashExecution` and `custom_message` become events; title from the last `session_info.name`.
- `internal/opencode` — OpenCode and Kilo (same schema). `List` skips child (subagent) and
  archived sessions; `Read` builds turns from user messages (non-synthetic text parts) and
  assistant messages' reasoning/text/tool parts (tool `state` holds input, output/error, times).
  Opened `mode=ro` **without** `immutable` — both run in WAL mode and recent rows live in the
  -wal file. `opencodetest.Seed` builds a test database (also used by the catalog tests).
- `internal/catalog` — each harness is a lister in `harnesses.go` returning `item`s (harness, id,
  file or db path, project key, size, mtime) plus a `parse` case. Project path: Claude via the
  `cwd` in the newest transcript; Cursor by walking the filesystem with the folder slug
  (`resolveSlug`; `-` can stand for `/ - _ .` or space), then a Claude path with the same slug,
  then `guessSlug` (deepest existing dir + rest); Codex/pi from their first line (memoized in
  `heads`); OpenCode/Kilo from `session.directory`. Projects merge on that path; no path →
  `no-workspace`. `enrich` adds what the source lacks (cwd = project dir, Cursor resumability /
  model / creation time, Codex thread names, active flag). Sessions with zero turns are hidden.
  `Conversation` looks the id up among the listed items (ids validated first). Adding a
  harness: parser package → constant in `conv` → lister + `parse` case + `Config` field (+ env
  var in `cmd/server`, e2e config) → `HARNESSES`, resume spec, icon in the frontend.
- `internal/auth` — login in front of the API. `Handler.Guard` answers 401 to `/api/*` without
  a session (except `/api/auth/*`; the bundle and `/health` stay public). `POST /api/auth/login`
  (JSON only, a CSRF guard) runs the `Authenticator` one at a time, then sets `ah_session`:
  HttpOnly, SameSite=Strict, no expiry (gone when the browser exits); sessions are an in-memory
  map keyed by the token's sha256. `System(service)` is the OS password check, one file per OS:
  `pam.go` (Linux + macOS: cgo, a C conversation function, no Go callbacks; `PAM_SERVICE`,
  default `login`; only the server's user, compared after PAM so timing is the same) and
  `logon_windows.go` (`LogonUserW` network logon via advapi32, same account compared by SID,
  2 s delay on failure). `system_e2e.go` (build tag `e2e`) replaces both with
  demo / demo-password for Playwright. `AUTH_DISABLED=1` (`Handler.NoLogin`) turns the login off:
  every request is the server's user — for agents driving a browser against the running service.
- `internal/shell` — the terminal panel: login shells in PTYs (`aymanbagabas/go-pty`: creack/pty
  on Unix, ConPTY on Windows), kept in a server-wide `Manager` (tab number = lowest free).
  `GET/POST /api/shells`, `DELETE /api/shells/{id}`, WebSocket `GET /api/shells/{id}/attach`
  (`coder/websocket`; its Origin check stops other sites/ports): binary = terminal bytes, text =
  JSON (`resize` in; `status`/`exit` with the shell's info out). 512 KB scrollback replayed on
  attach; a slow WebSocket is dropped and the browser reattaches. `run` is typed at the first
  prompt (first output + 250 ms quiet). Every second the foreground process group (TIOCGPGRP
  via `Control`, never `Fd()`, which makes the master blocking) gives the tab's folder and
  command: `/proc` on Linux, libproc + sysctl on macOS (cgo), nothing on Windows. The server
  drops its slave fd after start; exit = `Wait` + 300 ms drain, then close. Stop = SIGHUP to
  the group, close, SIGKILL after 3 s. Logout (`auth.Handler.OnLogout`) ends all shells, so
  the e2e shell spec runs in its own project after the others. `cmd/server` strips its own
  settings (`serverVars`) from the shells' environment, skips gzip for upgrades, and
  `statusWriter.Unwrap` lets the upgrade hijack.
- `internal/api` — `GET /api/projects`, `GET /api/conversations?project=<key>`,
  `GET /api/conversations/{harness}/{id}` (ETag = `Catalog.ConversationIfChanged` version: size +
  mtime + active flag, so it also changes when the active window passes; `If-None-Match` → 304
  without parsing). `cmd/server` adds gzip for `/api/*` and an access log,
  and listens on `HOST` (default `127.0.0.1`).
- `web/` — `go:embed` of `web/dist` + SPA fallback handler (404s unknown `/api/*` and `/health`).

### Frontend (`src/`)

- `App.jsx` — `AuthGate` asks `/api/auth/session`, shows `LoginScreen` or the browser; a 401 from
  any API call (`LOGGED_OUT_EVENT` in `lib/api.js`) returns to the login. Right after a login the
  browser mounts under `LoginScreen`, which plays the unlock sequence (redaction bars from
  `lib/redaction.js` peel away, welcome line, aperture via the `--ah-hole` `@property`; click or
  key skips; reduced motion = fade) — `UNLOCK_MS` matches the CSS timings.
- `App.jsx` (browser) — three columns; the selection is in the URL (`?project=<key>&c=<harness>:<id>`).
  `useFetch` aborts stale loads. `hooks/useLiveConversation.js` loads the open conversation and
  polls it with its ETag (`lib/live.js` `pollDelay`: 2 s while active, 15 s otherwise, paused
  while the page is hidden). `isWorking` = active + last turn `incomplete`: `ConversationView`
  then shows the work, marks the last turn "Working…" and keeps the chat at the bottom if it was
  there; when it flips back the work collapses. `e2e/live.spec.js` appends to a transcript
  mid-test, in its own Playwright project (chromium → live → shells) because it adds the newest project.
- `components/Shells.jsx` — `ShellProvider` (shell list restored from `/api/shells`, active tab,
  open/minimized; `useShells().openShell({cwd, run})` from the project rows and the Resume
  popover) and `ShellPanel` (tab bar = status bar, drag-to-resize, height in prefs).
  `Terminal.jsx` (lazy: xterm.js + fit addon) keeps one xterm per shell mounted while hidden,
  resets before a reattach's scrollback replay; `lib/shells.js` holds its pure helpers.
- `components/` — `ProjectsPanel` (collapsible rail; user + **Log out** in the footer), `ConversationList`, `ConversationView`
  (user bubble right, agent left with `agentLabel` "Claude Code - Opus 5.5" / "Codex - GPT 5.5", `Work details`
  collapsed by default, global Show/Hide work), `Step` (thinking/note/tool rows; tool input shown
  as shell/diff/JSON, output on click), `ResumeButton`, `Markdown` (markdown-it + hljs + GitHub
  alerts; mermaid lazy-loaded, `securityLevel: "strict"`, `html: false`).
- `lib/` — pure helpers with unit tests: `format.js` (`modelLabel`, dates, durations),
  `resume.js` (per-harness resume argv + bypass flag, or `noBypass` reason when the CLI has none;
  POSIX quoting), `tools.js` (tool icon/summary across the harnesses' tool names, incl. Codex
  argv `command` arrays, apply_patch and code-mode `exec` strings), `markdown.js`, `theme.js`, `prefs.js` (localStorage `ah_prefs`: theme,
  collapsed panel, bypass checkbox — UI prefs only, reads/writes never throw).
- Styling: Primer CSS tokens + Font Awesome; app colours are `--ah-*` variables in
  `src/styles/app.css` with dark values under `[data-color-mode="dark"]` (set on `<html>` by
  `theme.js` and the inline script in `index.html`).
