# awesome-harness

Browse your local **Claude Code** and **Cursor** agent conversations in one UI, as a chat:
your prompts on the right, the agent's answer on the left (rendered markdown), the tool calls
and thinking behind each answer collapsed under **Work details**. Copy a command to resume any
conversation in its CLI.

It reads the harnesses' own transcript files and never writes to them. It is a browser for
existing conversations, not a live view: use the reload buttons to pick up new messages.

## Run

```bash
make install     # build bin/server (web UI embedded) + install/enable/restart the user systemd unit
# or, without systemd:
make build && ./bin/server
```

Open http://127.0.0.1:8767.

| Setting (env or `.env`) | Default | |
|---|---|---|
| `HOST` | `127.0.0.1` | Transcripts hold code, prompts and secrets: keep it on loopback. |
| `PORT` | `8767` | |
| `CLAUDE_DIR` | `~/.claude` | |
| `CURSOR_DIR` | `~/.cursor` | |

## The UI

- **Projects** (left, collapsible to a rail of initials): every directory an agent ran in,
  with the number of Claude Code / Cursor transcripts and the last activity. A project used from
  both harnesses is one entry. Projects whose folder is gone are greyed out.
- **Conversations**: title, status icon, harness + model, prompt and tool-call counts, last
  update. Filter by text (title, model, branch) and by harness.
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

## What is read

| | Claude Code | Cursor |
|---|---|---|
| Transcripts | `~/.claude/projects/<slug>/<session>.jsonl` | `~/.cursor/projects/<slug>/agent-transcripts/<id>/<id>.jsonl` (IDE and CLI) |
| Project path | `cwd` recorded on each message | folder slug matched against the filesystem |
| Title | `/rename` title, agent name, AI title, else first prompt | first prompt |
| Model | per message | cursor-agent chat store (`~/.cursor/chats/<md5(workspace)>/<id>/store.db`), else `~/.cursor/ai-tracking/ai-code-tracking.db`, else unknown |
| Timestamps | every message and tool result | the `<timestamp>` some prompts carry, the chat store's creation time, file times |
| Tool output | yes | not recorded by Cursor |
| Resumable | yes | only chats with a cursor-agent chat store (IDE chats: open them in Cursor) |

Subagent transcripts are not shown separately; their result appears as the `Task`/`Agent`
tool output in the parent conversation.

## Develop

```bash
make dev         # build the web app, run the Go server on :8767
make dev-web     # Vite with HMR on :5173, proxies /api to :8767
make test        # go test + vitest + playwright (e2e runs over generated fixtures, not your real data)
```
