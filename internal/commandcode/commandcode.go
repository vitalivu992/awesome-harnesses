// Package commandcode reads Command Code (cmdc) sessions:
// ~/.commandcode/projects/<cwd slug>/<id>.jsonl, with <id>.meta.json beside it
// (title, model) and <id>.checkpoints.jsonl (rewind points, not read). The
// format is pi's session tree (version 3): a {type: "session", id, cwd} header,
// then entries with id + parentId; the conversation shown is the branch that
// ends at the last entry. Messages use Anthropic-style content blocks: the
// assistant's tool_use blocks are answered by tool_result blocks in the next
// user message. A user message's meta.source says who wrote it: "user" (a
// prompt), "tool" (tool results), none (a notice such as "Insufficient
// credits" or "Interrupted by user").
package commandcode

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"awesome-harness/internal/conv"
)

type entry struct {
	Type       string          `json:"type"`
	ID         string          `json:"id"`
	ParentID   *string         `json:"parentId"`
	Timestamp  string          `json:"timestamp"`
	Cwd        string          `json:"cwd"`
	Message    *message        `json:"message"`
	Model      string          `json:"model"` // assistant messages, model_change
	Name       string          `json:"name"`  // session_info
	Summary    string          `json:"summary"`
	CustomType string          `json:"customType"`
	Content    json.RawMessage `json:"content"`
	Display    bool            `json:"display"`
}

type message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
	Meta    struct {
		Source      string `json:"source"`
		IsMeta      bool   `json:"isMeta"`
		IsAutomated bool   `json:"isAutomated"`
	} `json:"meta"`
}

type block struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

// The notice Command Code writes when the user stops a turn.
const interrupted = "Interrupted by user"

// Header is a session's first line.
type Header struct {
	ID        string `json:"id"`
	Cwd       string `json:"cwd"`
	Timestamp string `json:"timestamp"`
}

// ReadHeader returns a session file's header line.
func ReadHeader(path string) (*Header, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := bufio.NewReaderSize(f, 1<<20).ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	var e entry
	if json.Unmarshal(raw, &e) != nil || e.Type != "session" || e.ID == "" {
		return nil, errors.New("commandcode: no session header")
	}
	return &Header{ID: e.ID, Cwd: e.Cwd, Timestamp: e.Timestamp}, nil
}

// MetaPath is the meta file beside a session file.
func MetaPath(path string) string {
	return strings.TrimSuffix(path, ".jsonl") + ".meta.json"
}

// ParseFile reads a session file, with the title from its meta file.
func ParseFile(path string) (*conv.Conversation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	c, err := Parse(f)
	if err != nil {
		return nil, err
	}
	if st, err := f.Stat(); err == nil {
		c.SizeBytes = st.Size()
	}
	var meta struct {
		Title string `json:"title"`
	}
	if b, err := os.ReadFile(MetaPath(path)); err == nil && json.Unmarshal(b, &meta) == nil && strings.TrimSpace(meta.Title) != "" {
		c.Title = strings.TrimSpace(meta.Title)
	}
	return c, nil
}

// Parse reads a session from r. Lines that aren't valid JSON are skipped.
func Parse(r io.Reader) (*conv.Conversation, error) {
	c := &conv.Conversation{}
	c.Harness = conv.CommandCode
	c.Resumable = true
	var all []*entry
	byID := map[string]*entry{}
	br := bufio.NewReaderSize(r, 1<<20)
	for {
		raw, err := br.ReadBytes('\n')
		var e entry
		if len(strings.TrimSpace(string(raw))) > 0 && json.Unmarshal(raw, &e) == nil {
			if e.Type == "session" {
				c.ID, c.Cwd, c.StartedAt = e.ID, e.Cwd, e.Timestamp
			} else {
				all = append(all, &e)
				if e.ID != "" {
					byID[e.ID] = &e
				}
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	branch := activeBranch(all, byID)
	b := builder{c: c, cur: -1, tools: map[string][2]int{}}
	for _, e := range branch {
		if e.Timestamp != "" {
			if c.StartedAt == "" {
				c.StartedAt = e.Timestamp
			}
			c.UpdatedAt = e.Timestamp
		}
		b.add(e)
	}
	b.closeTurn()
	for _, e := range branch {
		if e.Type == "session_info" && e.Name != "" {
			c.Title = e.Name
		}
	}
	c.Finish()
	return c, nil
}

// activeBranch is the path from the root to the last entry, oldest first.
func activeBranch(all []*entry, byID map[string]*entry) []*entry {
	if len(all) == 0 {
		return nil
	}
	last := all[len(all)-1]
	if last.ID == "" {
		return all
	}
	var path []*entry
	seen := map[string]bool{}
	for e := last; e != nil && !seen[e.ID]; {
		seen[e.ID] = true
		path = append(path, e)
		if e.ParentID == nil {
			break
		}
		e = byID[*e.ParentID]
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

type builder struct {
	c     *conv.Conversation
	cur   int
	model string
	// Of the open turn: an assistant message seen, its last one asked for
	// tools, and the notice that ended it.
	answered, pending bool
	status, err       string
	tools             map[string][2]int
}

func (b *builder) add(e *entry) {
	switch e.Type {
	case "model_change":
		b.model = e.Model
	case "compaction":
		b.event("Compacted", e.Summary, e.Timestamp)
	case "branch_summary":
		b.event("Branch summary", e.Summary, e.Timestamp)
	case "custom_message":
		if e.Display {
			title := strings.ReplaceAll(e.CustomType, "-", " ")
			if title == "" {
				title = "Notification"
			}
			b.event(title, text(e.Content), e.Timestamp)
		}
	case "message":
		if e.Message == nil {
			return
		}
		if e.Message.Role == "assistant" {
			b.assistant(e)
		} else if e.Message.Role == "user" {
			b.user(e.Message, e.Timestamp)
		}
	}
}

func (b *builder) event(title, detail, at string) {
	b.closeTurn()
	b.c.Entries = append(b.c.Entries, conv.Entry{Kind: conv.EntryEvent, Text: title, Detail: detail, At: at})
}

func (b *builder) user(m *message, at string) {
	var blocks []block
	if json.Unmarshal(m.Content, &blocks) != nil {
		var s string
		if json.Unmarshal(m.Content, &s) != nil {
			return
		}
		blocks = []block{{Type: "text", Text: s}}
	}
	results := false
	for _, bl := range blocks {
		if bl.Type == "tool_result" {
			results = true
			b.result(bl, at)
		}
	}
	if results || m.Meta.IsMeta || m.Meta.IsAutomated {
		return
	}
	prompt := strings.TrimSpace(text(m.Content))
	if prompt == "" {
		return
	}
	// A message nobody wrote is Command Code's notice about the open turn.
	if m.Meta.Source == "" && b.cur >= 0 {
		if strings.Contains(prompt, interrupted) {
			b.status = conv.StatusAborted
		} else {
			b.status, b.err = conv.StatusError, prompt
		}
		b.c.Entries[b.cur].EndedAt = at
		return
	}
	b.closeTurn()
	b.c.Entries = append(b.c.Entries, conv.Entry{Kind: conv.EntryTurn, Prompt: prompt, PromptAt: at})
	b.cur = len(b.c.Entries) - 1
}

func (b *builder) result(bl block, at string) {
	idx, ok := b.tools[bl.ToolUseID]
	if !ok {
		return
	}
	s := &b.c.Entries[idx[0]].Steps[idx[1]]
	out, cut := conv.CapText(text(bl.Content))
	s.Output, s.OutputAt, s.IsError = out, at, bl.IsError
	s.Truncated = s.Truncated || cut
}

func (b *builder) assistant(en *entry) {
	at := en.Timestamp
	if b.cur < 0 {
		b.c.Entries = append(b.c.Entries, conv.Entry{Kind: conv.EntryTurn, PromptAt: at})
		b.cur = len(b.c.Entries) - 1
	}
	e := &b.c.Entries[b.cur]
	if en.Model != "" {
		b.model = en.Model
	}
	e.Model = b.model
	e.EndedAt = at
	b.answered, b.pending = true, false
	var blocks []block
	if json.Unmarshal(en.Message.Content, &blocks) != nil {
		return
	}
	for _, bl := range blocks {
		switch bl.Type {
		case "thinking":
			if strings.TrimSpace(bl.Thinking) != "" {
				e.Steps = append(e.Steps, conv.Step{Kind: conv.StepThinking, At: at, Text: bl.Thinking})
			}
		case "text":
			if strings.TrimSpace(bl.Text) != "" {
				e.Steps = append(e.Steps, conv.Step{Kind: conv.StepText, At: at, Text: bl.Text})
			}
		case "tool_use":
			input, cut := conv.CapInput(bl.Input)
			e.Steps = append(e.Steps, conv.Step{Kind: conv.StepTool, At: at, Tool: bl.Name, Input: input, Truncated: cut})
			b.tools[bl.ID] = [2]int{b.cur, len(e.Steps) - 1}
			b.pending = true
		}
	}
}

// closeTurn sets the open turn's status: from a notice (error, aborted), else
// done when the last assistant message answered without asking for tools,
// else unfinished (still running, or the CLI was killed).
func (b *builder) closeTurn() {
	if b.cur < 0 {
		return
	}
	e := &b.c.Entries[b.cur]
	e.SplitFinal()
	switch {
	case b.status != "":
		e.Status, e.Error = b.status, b.err
	case b.answered && !b.pending:
		e.Status = conv.StatusDone
	default:
		e.Status = conv.StatusIncomplete
	}
	b.cur, b.answered, b.pending, b.status, b.err = -1, false, false, "", ""
}

// text is a string content, or the text blocks of a content array.
func text(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []block
	if json.Unmarshal(raw, &blocks) != nil {
		return ""
	}
	parts := make([]string, 0, len(blocks))
	for _, bl := range blocks {
		switch bl.Type {
		case "text":
			parts = append(parts, bl.Text)
		case "image":
			parts = append(parts, "[image]")
		}
	}
	return strings.Join(parts, "\n\n")
}
