// Package opencode reads OpenCode sessions from its SQLite database
// (~/.local/share/opencode/opencode.db). Kilo Code's CLI is an OpenCode fork
// with the same schema (~/.local/share/kilo/kilo.db), so one reader serves
// both. Tables: session (directory, title, parent_id for subagent sessions),
// message (one row per user/assistant message, JSON in `data`) and part (the
// message's text, reasoning and tool parts, JSON in `data`).
package opencode

import (
	"database/sql"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"awesome-harness/internal/conv"

	_ "modernc.org/sqlite"
)

// Session is a row of the session list.
type Session struct {
	ID        string
	Directory string
	Updated   time.Time
}

// Open opens a database read-only. Not immutable: OpenCode keeps it in WAL
// mode, and the newest messages are still in the -wal file.
func Open(path string) (*sql.DB, error) {
	u := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro&_pragma=busy_timeout(3000)"}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// List returns the top-level, non-archived sessions (subagent sessions are
// shown through the parent's task tool call).
func List(db *sql.DB) ([]Session, error) {
	rows, err := db.Query(`SELECT id, directory, time_updated FROM session
		WHERE parent_id IS NULL AND time_archived IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var s Session
		var updated int64
		if rows.Scan(&s.ID, &s.Directory, &updated) == nil {
			s.Updated = time.UnixMilli(updated)
			out = append(out, s)
		}
	}
	return out, rows.Err()
}

type messageData struct {
	Role    string `json:"role"`
	ModelID string `json:"modelID"`
	Finish  string `json:"finish"`
	Time    struct {
		Created   int64 `json:"created"`
		Completed int64 `json:"completed"`
	} `json:"time"`
	Error *struct {
		Name string `json:"name"`
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	} `json:"error"`
}

type partData struct {
	Type      string `json:"type"`
	Text      string `json:"text"`
	Synthetic bool   `json:"synthetic"`
	Ignored   bool   `json:"ignored"`
	Tool      string `json:"tool"`
	Time      struct {
		Start int64 `json:"start"`
		End   int64 `json:"end"`
	} `json:"time"`
	State struct {
		Status string          `json:"status"`
		Input  json.RawMessage `json:"input"`
		Output string          `json:"output"`
		Error  string          `json:"error"`
		Time   struct {
			Start int64 `json:"start"`
			End   int64 `json:"end"`
		} `json:"time"`
	} `json:"state"`
}

func iso(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format(time.RFC3339Nano)
}

// Read loads one session in full. harness is conv.OpenCode or conv.Kilo.
func Read(db *sql.DB, harness, id string) (*conv.Conversation, error) {
	c := &conv.Conversation{}
	c.ID, c.Harness, c.Resumable = id, harness, true
	var title string
	var created, updated int64
	err := db.QueryRow(`SELECT directory, title, time_created, time_updated FROM session WHERE id = ?`, id).
		Scan(&c.Cwd, &title, &created, &updated)
	if err != nil {
		return nil, err
	}
	// Untitled sessions are named "New session - <ISO time>"; the first prompt says more.
	if !strings.HasPrefix(title, "New session - ") && !strings.HasPrefix(title, "Child session - ") {
		c.Title = title
	}
	c.StartedAt, c.UpdatedAt = iso(created), iso(updated)

	parts := map[string][]partData{}
	rows, err := db.Query(`SELECT message_id, data FROM part WHERE session_id = ? ORDER BY time_created, id`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var mid, data string
		var p partData
		if rows.Scan(&mid, &data) == nil && json.Unmarshal([]byte(data), &p) == nil {
			parts[mid] = append(parts[mid], p)
		}
	}
	rows.Close()

	rows, err = db.Query(`SELECT id, data FROM message WHERE session_id = ? ORDER BY time_created, id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	b := builder{c: c, cur: -1}
	for rows.Next() {
		var mid, data string
		var m messageData
		if rows.Scan(&mid, &data) != nil || json.Unmarshal([]byte(data), &m) != nil {
			continue
		}
		b.message(&m, parts[mid])
	}
	b.closeTurn()
	c.Finish()
	return c, rows.Err()
}

type builder struct {
	c    *conv.Conversation
	cur  int
	last *messageData // the open turn's last assistant message
}

func (b *builder) message(m *messageData, parts []partData) {
	at := iso(m.Time.Created)
	if m.Role == "user" {
		var texts []string
		for _, p := range parts {
			if p.Type == "text" && !p.Synthetic && !p.Ignored && strings.TrimSpace(p.Text) != "" {
				texts = append(texts, p.Text)
			}
		}
		if len(texts) == 0 {
			return
		}
		b.closeTurn()
		b.c.Entries = append(b.c.Entries, conv.Entry{Kind: conv.EntryTurn, Prompt: strings.TrimSpace(strings.Join(texts, "\n\n")), PromptAt: at})
		b.cur = len(b.c.Entries) - 1
		return
	}
	if b.cur < 0 {
		b.c.Entries = append(b.c.Entries, conv.Entry{Kind: conv.EntryTurn, PromptAt: at})
		b.cur = len(b.c.Entries) - 1
	}
	e := &b.c.Entries[b.cur]
	b.last = m
	if m.ModelID != "" {
		e.Model = m.ModelID
	}
	e.EndedAt = at
	if m.Time.Completed > 0 {
		e.EndedAt = iso(m.Time.Completed)
	}
	for _, p := range parts {
		switch p.Type {
		case "reasoning":
			if strings.TrimSpace(p.Text) != "" {
				s := conv.Step{Kind: conv.StepThinking, At: iso(p.Time.Start), Text: p.Text}
				if p.Time.End > p.Time.Start && p.Time.Start > 0 {
					s.DurationMs = p.Time.End - p.Time.Start
				}
				e.Steps = append(e.Steps, s)
			}
		case "text":
			if !p.Synthetic && strings.TrimSpace(p.Text) != "" {
				stepAt := iso(p.Time.Start)
				if stepAt == "" {
					stepAt = at
				}
				e.Steps = append(e.Steps, conv.Step{Kind: conv.StepText, At: stepAt, Text: p.Text})
			}
		case "tool":
			input, cut := conv.CapInput(p.State.Input)
			out := p.State.Output
			if p.State.Status == "error" {
				out = p.State.Error
			}
			out, cutOut := conv.CapText(out)
			e.Steps = append(e.Steps, conv.Step{
				Kind: conv.StepTool, Tool: p.Tool, Input: input,
				At: iso(p.State.Time.Start), Output: out, OutputAt: iso(p.State.Time.End),
				IsError: p.State.Status == "error", Truncated: cut || cutOut,
			})
		}
	}
}

// closeTurn: the last assistant message's error decides error/aborted;
// otherwise a final answer (or finish "stop") means done.
func (b *builder) closeTurn() {
	if b.cur < 0 {
		return
	}
	e := &b.c.Entries[b.cur]
	m := b.last
	b.cur, b.last = -1, nil
	e.SplitFinal()
	switch {
	case m != nil && m.Error != nil && m.Error.Name == "MessageAbortedError":
		e.Status = conv.StatusAborted
	case m != nil && m.Error != nil:
		e.Status = conv.StatusError
		e.Error = m.Error.Data.Message
		if e.Error == "" {
			e.Error = m.Error.Name
		}
	case e.Final != "" || (m != nil && m.Finish == "stop"):
		e.Status = conv.StatusDone
	default:
		e.Status = conv.StatusIncomplete
	}
}
