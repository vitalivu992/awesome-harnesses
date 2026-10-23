# AGENTS.md

User-facing features and what data is read: [README.md](README.md).

## Commands

```bash
make build                  # pnpm build → web/dist (go:embed) + go build -o bin/server
make dev | make dev-web     # Go server on :8767 | Vite HMR on :5173 proxying /api
go test ./...               # Go tests (parsers, catalog, api)
pnpm test:unit              # vitest (tests/*.test.mjs; JSX helpers import fine)
pnpm test:e2e               # playwright: e2e/make-fixtures.mjs writes a fake home to .e2e-home/,
                            # then the config starts the server on :8768 over it
```

JS tooling is **pnpm**. `@playwright/test` is pinned to 1.60.0 to match the browsers installed
on this machine. `web/dist/.gitkeep` keeps the embed pattern valid on a fresh clone; run
`make web` before building for real pages.

## Architecture

Read-only Go server + React SPA, one binary. No database: transcripts are parsed on request and
summaries cached in memory by file size + mtime.

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
  trailing `[REDACTED]` is dropped. `turn_ended` gives the status (abort errors → aborted);
  transcripts from before mid-2026 have no markers, so a turn with a final answer counts as done.
  `store.go` reads the cursor-agent chat store (hex JSON meta + newest `"modelName"` in blobs) and
  the AI code tracking db, both opened `mode=ro&immutable=1` so Cursor's locks are never touched.
- `internal/catalog` — finds transcript folders and maps them to project paths: Claude via the
  `cwd` in the newest transcript, Cursor by walking the filesystem with the folder slug
  (`resolveSlug`; `-` can stand for `/ - _ .` or space), then a Claude path with the same slug,
  then `guessSlug` (deepest existing dir + rest). Projects merge on that path. Cursor chats
  without a workspace (`~/.cursor/projects/agent-transcripts`) are project `cursor:no-workspace`.
  `enrich` adds what the file lacks (Cursor cwd, resumability, model, creation time; active flag).
  Sessions with zero turns are hidden. Conversation ids are validated before globbing.
- `internal/api` — `GET /api/projects`, `GET /api/conversations?project=<key>`,
  `GET /api/conversations/{harness}/{id}`. `cmd/server` adds gzip for `/api/*` and an access log,
  and listens on `HOST` (default `127.0.0.1`).
- `web/` — `go:embed` of `web/dist` + SPA fallback handler (404s unknown `/api/*` and `/health`).

### Frontend (`src/`)

- `App.jsx` — three columns; the selection is in the URL (`?project=<key>&c=<harness>:<id>`).
  `useFetch` aborts stale loads.
- `components/` — `ProjectsPanel` (collapsible rail), `ConversationList`, `ConversationView`
  (user bubble right, agent left with `agentLabel` "Claude Code - Opus 5.5", `Work details`
  collapsed by default, global Show/Hide work), `Step` (thinking/note/tool rows; tool input shown
  as shell/diff/JSON, output on click), `ResumeButton`, `Markdown` (markdown-it + hljs + GitHub
  alerts; mermaid lazy-loaded, `securityLevel: "strict"`, `html: false`).
- `lib/` — pure helpers with unit tests: `format.js` (`modelLabel`, dates, durations),
  `resume.js` (`resumeCommand`, POSIX quoting), `tools.js` (tool icon/summary across both
  harnesses' tool names), `markdown.js`, `theme.js`, `prefs.js` (localStorage `ah_prefs`: theme,
  collapsed panel, bypass checkbox — UI prefs only, reads/writes never throw).
- Styling: Primer CSS tokens + Font Awesome; app colours are `--ah-*` variables in
  `src/styles/app.css` with dark values under `[data-color-mode="dark"]` (set on `<html>` by
  `theme.js` and the inline script in `index.html`).
