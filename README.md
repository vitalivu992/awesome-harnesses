# awesome-harness

Browse your local coding-agent conversations — **Claude Code**, **Cursor**, **Codex**, **pi**,
**OpenCode** and **Kilo Code** — in one UI, as a chat:
your prompts on the right, the agent's answer on the left (rendered markdown), the tool calls
and thinking behind each answer collapsed under **Work details**. Copy a command to resume any
conversation in its CLI.

It reads the harnesses' own transcript files and databases and never writes to them. It is a browser for
existing conversations, not a live view: use the reload buttons to pick up new messages.

## Run

```bash
make up          # start the background service (builds + installs the user systemd unit the first time)
make down        # stop it
make install     # rebuild and restart after pulling changes
make logs        # follow the service log
# or, without systemd:
make build && ./bin/server
```

`make` alone lists every target.

Open http://127.0.0.1:8767 and log in with the username and password of your computer
account (the ones of your desktop session).

Building:

| OS | Password check | Needs to build |
|---|---|---|
| Linux | PAM | cgo + PAM headers (`sudo apt install libpam0g-dev` on Debian/Ubuntu) |
| macOS | PAM (OpenPAM / Open Directory) | cgo: Xcode command line tools (`xcode-select --install`) |
| Windows | `LogonUserW` | nothing extra (pure Go) |

`make up` / `install` use systemd (Linux only); elsewhere run `make build && ./bin/server`.

| Setting (env or `.env`) | Default | |
|---|---|---|
| `HOST` | `127.0.0.1` | Transcripts hold code, prompts and secrets: keep it on loopback. |
| `PORT` | `8767` | |
| `PAM_SERVICE` | `login` | Linux, macOS: PAM service that checks the password (`/etc/pam.d/<name>`). |
| `CLAUDE_DIR` | `~/.claude` | |
| `CURSOR_DIR` | `~/.cursor` | |
| `CURSOR_LOGS_DIR` | `~/.cursor-server/data/logs` | Cursor server logs: model and turn times of Cursor app chats. |
| `CODEX_DIR` | `$CODEX_HOME`, else `~/.codex` | |
| `PI_DIR` | `~/.pi` | |
| `OPENCODE_DB` | `$XDG_DATA_HOME/opencode/opencode.db` (`~/.local/share/…`) | |
| `KILO_DB` | `$XDG_DATA_HOME/kilo/kilo.db` (`~/.local/share/…`) | |

A harness that isn't installed simply has no conversations.

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
**Log out** (bottom of the projects panel), or when the server restarts (sessions are kept
in memory). A browser that restores the last session on start also restores this cookie.

## The UI

- **Projects** (left, collapsible to a rail of initials): every directory an agent ran in,
  with the number of conversations per harness and the last activity. A project used from
  several harnesses is one entry. Projects whose folder is gone are greyed out.
- **Conversations**: title, status icon, harness + model, prompt and tool-call counts, last
  update. Filter by text (title, model, branch) and by harness (one chip per harness the
  project has).
- **Conversation**: header with harness/model, branch, start time, session id; **Show work**
  expands every answer's work details; **Resume** opens the resume command with a
  *Bypass permissions* option and a copy button.

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
