# Awesome Harnesses

A viewer for local data from **Claude Code**, **Cursor**, **Codex**, **pi**,
**OpenCode**, and **Kilo Code**. Awesome Harnesses is a tool for viewing data from these
coding-agent harnesses, not a harness itself. It reads their transcript files and databases
without writing to them.

It browses saved sessions and displays their conversation content, including prompts,
answers, tool calls, and thinking. The open conversation and both lists refresh by themselves
while a session is live. You can also send a message to a conversation or start a new one:
the server runs the harness's own CLI without its interactive UI (`claude -p`, `codex exec`, …),
and the conversation shows its work as the CLI writes the transcript.

## Run

```bash
make up          # start the background service (builds + installs it the first time)
make down        # stop it
make install     # rebuild and restart after pulling changes
make status      # is it running?
make logs        # follow the service log
# or in the foreground:
make run         # build, then run bin/server with .env applied
```

The Makefile targets call `./ctl.sh` (`./ctl.sh help`), which you can also run directly. The
service is per-user: a systemd user unit on Linux (`~/.config/systemd/user/awesome-harness.service`,
log in the journal) and a launchd agent on macOS (`~/Library/LaunchAgents/dev.awesome-harness.plist`,
log in `~/Library/Logs/awesome-harness.log`), both generated with this checkout's path — after
moving the checkout, run `make install` again. Both start
at login and restart after a crash. `make install` creates `.env` from `.env.example` the
first time; the service reads it at every start, so edit it and `make restart`.

`make` alone lists every target.

Open http://127.0.0.1:8767 and log in with the username and password of your computer
account (the ones of your desktop session).

Building:

| OS | Password check | Needs to build |
|---|---|---|
| Linux | PAM | cgo + PAM headers (`sudo apt install libpam0g-dev` on Debian/Ubuntu) |
| macOS | PAM (OpenPAM / Open Directory) | cgo: Xcode command line tools (`xcode-select --install`) |
| Windows | `LogonUserW` | nothing extra (pure Go) |

`make up` / `install` use systemd on Linux and launchd on macOS; on Windows run
`go build -o bin/server.exe ./cmd/server` after `pnpm build`, and start `bin\server.exe`.

| Setting (env or `.env`) | Default | |
|---|---|---|
| `HOST` | `127.0.0.1` | Transcripts hold code, prompts and secrets: keep it on loopback. |
| `PORT` | `8767` | |
| `PAM_SERVICE` | `login` | Linux, macOS: PAM service that checks the password (`/etc/pam.d/<name>`). |
| `AUTH_DISABLED` | off | `1` turns the login off (local testing, agents driving a browser). Anyone who reaches the port then also gets a shell. |
| `CLAUDE_DIR` | `~/.claude` | |
| `CURSOR_DIR` | `~/.cursor` | |
| `CURSOR_LOGS_DIR` | `~/.cursor-server/data/logs` | Cursor server logs: model and turn times of Cursor app chats. |
| `CODEX_DIR` | `$CODEX_HOME`, else `~/.codex` | |
| `PI_DIR` | `~/.pi` | |
| `OPENCODE_DB` | `$XDG_DATA_HOME/opencode/opencode.db` (`~/.local/share/…`) | |
| `KILO_DB` | `$XDG_DATA_HOME/kilo/kilo.db` (`~/.local/share/…`) | |
| `LAUNCH_PATH` | your login shell's `PATH` | Where the harness CLIs are looked up for prompts sent from the UI. By default the server asks `$SHELL -l -i` once every few minutes, so CLIs installed with nvm or into `~/.local/bin` are found from the service too. |

A harness that isn't installed simply has no local data to show.

## Login

Every API call needs a login. The server asks the OS to check the password, like a console
or desktop login: PAM on Linux and macOS, `LogonUserW` on Windows. It lets in only the user
it runs as, because the transcripts are that user's files. A wrong password takes 2–3 s to
answer, and the server checks one password at a time.

- macOS: use the short account name (`whoami`), not the full name.
- Windows: `name`, `DOMAIN\name` or `name@domain` all work. Windows Hello (PIN, fingerprint)
  can't be checked: use the account password — for a Microsoft account, its email and online
  password. Failed logins count towards the Windows account lockout policy.

The session is a cookie with no expiry: it ends when you quit the browser, when you click
**Log out** (the ⋮ menu at the bottom of the projects panel, after a confirmation), or when the server restarts (sessions are kept
in memory). A browser that restores the last session on start also restores this cookie.

## The UI

- **Projects** (left, collapsible to a rail of initials): every directory an agent ran in,
  with the number of conversations per harness and the last activity. A project used from
  several harnesses is one entry. Projects whose folder is gone are hidden: click the count
  next to **Projects** to show them (greyed out) or hide them again. The footer greets you
  by the time of day ("Good morning, <user>") and has a ⋮ menu with **Settings**, the theme (system, light, dark) and **Log out**.
- **Conversations**: title, status icon, harness + model, prompt and tool-call counts, last
  update. Filter by text (title, model, branch) and by harness: **All** with its count, then
  one icon per harness the project has; the selected harness also shows its name and count.
  The ✎ button in the header starts a **new conversation** in the project, with the harness
  the filter shows (with **All**: the default harness from the settings).
- **Conversation**: header with harness/model, branch, start time, session id; **Show work**
  expands every answer's work details; **Resume** opens the resume command with a
  *Bypass permissions* option, a copy button and **Run in shell** (opens a terminal tab in the
  conversation's folder and runs the command).
- **Prompt box** (under the conversation, and on the new-conversation page, where the harness
  is picked too): the model list comes from the settings (**Default model** = the harness's
  own), and chips show the effort, fast mode and bypass that apply. Enter sends, Shift+Enter
  is a new line. Typing `/` at the start (`$` anywhere for Codex skills) lists the harness's
  slash commands and skills for the folder, with descriptions; ↑/↓ and Enter or Tab insert
  one, and a known command is highlighted in the text (commands violet, skills orange), with
  "Uses the skill …" under the box. **Command** shows the exact command line. While the
  prompt runs, it shows as sent until the transcript has it, the view polls fast, and **Stop**
  interrupts the CLI (then kills it after 5 s). If the CLI fails, its last error lines show
  in the chat. A conversation another window is working on, or a Cursor app chat, can't be
  sent to.
- **Settings** (⋮ menu → Settings): a panel on the right (⤢ makes it wider, over the
  conversation; Esc or × closes it), saved in this browser. The default harness for **New**,
  then per harness: the models the prompt box offers, in order (suggestions from the CLI:
  Claude's aliases, Codex's model cache, `cursor-agent models`, `pi --list-models`,
  `opencode`/`kilo models`), each with its thinking effort (the values that model supports
  where the CLI says so); the effort of the default model; **Fast mode** (Claude Code, Codex;
  off by default); **Bypass permissions** (a prompt sent from here can't ask for approval);
  and a **command preview** of what a new conversation would run with those settings.
- **Live sessions**: an open conversation refreshes by itself (every 2 s while its transcript
  changed in the last 2 minutes, every 15 s otherwise) and new messages appear as the agent
  writes them. While the agent is working, the work is shown, a word ("Pondering…", "Brewing…"
  — a new one per prompt) sits under its latest text and the chat follows the newest step;
  when the turn ends the work collapses and the final answer is left.
- **Running sessions**: a project or conversation an agent is working on (transcript changed
  in the last 2 minutes, last turn not finished) shows a flashing blue dot; once the turn
  ends it turns into a green check until the 2 minutes pass (a dot on the collapsed rail's
  badges). The open conversation's header carries a Windows 11-style progress bar while the
  agent works (its **Live** chip shows once the turn has ended). Both lists reload by themselves: every 4 s while something in them is live,
  every 15 s otherwise.
- **Terminal** (bottom of the conversation column): any number of shell tabs. Open one with the
  terminal button on a project row (hover), **+** in the terminal bar (current project's
  folder) or **Run in shell**. Each tab shows its number (counting up from the server's
  start) and the current folder; the tooltip adds the running command. Click a tab to switch, **×** to end the shell, the chevron (or the active tab) to
  minimize the panel to its status bar. Drag the top edge to resize. Shells are login shells
  (`$SHELL -l`; PowerShell on Windows) that keep running when the page reloads (the last
  512 KB of output is replayed) and end on **Log out**. Ctrl+Shift+C copies the selection.
  The folder and command are read from `/proc` on Linux and libproc on macOS; Windows tabs
  show the starting folder only.

Status icons: ✓ completed, ✗ ended with an error, ■ interrupted by the user, ◐ unfinished
(the transcript stops mid-turn), pulsing dot = file changed in the last 2 minutes.

Resume commands:

| Harness | Command | Bypass permissions adds |
|---|---|---|
| Claude Code | `cd <cwd> && claude --resume <session id>` | `--dangerously-skip-permissions` |
| Cursor | `cd <workspace> && cursor-agent --resume <chat id>` | `--force` |
| Codex | `cd <cwd> && codex resume <session id>` | `--dangerously-bypass-approvals-and-sandbox` |
| pi | `cd <cwd> && pi --session <session id>` | — (pi doesn't ask for permissions) |
| OpenCode | `cd <dir> && opencode --session <session id>` | `--auto` |
| Kilo Code | `cd <dir> && kilo --session <session id>` | — (only non-interactive `kilo run` has one) |

Prompts sent from the UI (the server builds the command; the preview shows it):

| Harness | New conversation / continue | Model, effort, fast mode | Commands and skills |
|---|---|---|---|
| Claude Code | `claude -p --session-id <new uuid>` / `--resume <id>` | `--model`, `--effort`, `--settings '{"fastMode":true}'` | `/command`, `/skill`, `/plugin:name`: Claude expands them |
| Cursor | `cursor-agent -p --output-format stream-json --trust` (`--resume <id>`) | `--model` (effort and speed are in Cursor's model names) | `.cursor/commands` templates are expanded by the server; skills become an instruction to use them |
| Codex | `codex exec --json --skip-git-repo-check` / `codex exec resume … <id>` | `--model`, `-c model_reasoning_effort=…`, `-c service_tier="fast"` | `$skill` mentions (Codex expands them); `/prompts:name` is expanded by the server |
| pi | `pi -p --session-id <new uuid>` / `--session <id>` | `--model`, `--thinking` | `/template` and `/skill:name`: pi expands them |
| OpenCode / Kilo | `opencode run --format json` (`--session <id>`) | `--model`, `--variant` | commands are sent as `--command <name>`; skills become an instruction |

Each runs in the conversation's folder (the project's for a new one), with the bypass flag of
the resume table when it is on (Kilo: `kilo run --auto`). The id of a new Codex, Cursor or OpenCode/Kilo conversation
comes from the CLI's JSON output.

## What is read

| | Claude Code | Cursor | Codex | pi | OpenCode / Kilo Code |
|---|---|---|---|---|---|
| Where | `~/.claude/projects/<slug>/<session>.jsonl` | `~/.cursor/projects/<slug>/agent-transcripts/<id>/<id>.jsonl` (IDE and CLI) | `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl` | `~/.pi/agent/sessions/--<slug>--/*.jsonl` | `session` / `message` / `part` tables of `opencode.db` / `kilo.db` (SQLite) |
| Project path | `cwd` on each message | folder slug matched against the filesystem | `session_meta.cwd` | session header `cwd` | `session.directory` |
| Title | `/rename` title, agent name, AI title, else first prompt | first prompt | thread name (`session_index.jsonl`), else first prompt | `/name`, else first prompt | session title (unless the default "New session - …"), else first prompt |
| Model | per message | cursor-agent chat store; app chats: per turn from the Cursor server logs (only as old as the log folders Cursor keeps); else `~/.cursor/ai-tracking/ai-code-tracking.db`, else unknown | `turn_context` | per message | per message |
| Timestamps | every message and tool result | the `<timestamp>` some prompts carry, turn start/end from the Cursor server logs, chat store creation time, file times | every line | every entry | every message and part |
| Tool output | yes | not recorded by Cursor | yes | yes | yes |
| Status | `stop_reason`, interrupts, API errors | `turn_ended` | `task_complete`, `turn_aborted`, `error` | `stopReason` | message `error`, `finish` |
| Resumable | yes | only chats with a cursor-agent chat store (badge **CLI**); other chats show **App**: open them in Cursor | yes | yes | yes |

Left out: subagent sessions (Claude Code / Cursor `subagents/`, Codex sessions spawned by
another, OpenCode/Kilo child sessions) — their result appears as the `Task`/`Agent` tool
output in the parent conversation; archived OpenCode/Kilo sessions; sessions with no prompt.
pi sessions are trees (rewinds branch them): the branch ending at the last entry is shown.

## Develop

```bash
make dev         # build the web app, run the Go server on :8767
make dev-web     # Vite with HMR on :5173, proxies /api to :8767
make test        # go test + vitest + playwright (e2e runs over generated fixtures, not your real data)
```
