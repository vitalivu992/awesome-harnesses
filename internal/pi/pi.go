// Package pi reads pi coding agent sessions:
// ~/.pi/agent/sessions/--<cwd slug>--/<time>_<id>.jsonl. The first line is the
// session header ({type: "session", id, cwd, timestamp}); every later entry
// has an id and a parentId, so a session is a tree (rewinds and forks branch
// it). The conversation shown is the branch that ends at the last entry.
package pi

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
	ModelID    string          `json:"modelId"`
	Name       string          `json:"name"`
	CustomType string          `json:"customType"`
	Content    json.RawMessage `json:"content"`
}

type message struct {
	Role         string          `json:"role"`
	Content      json.RawMessage `json:"content"`
	Model        string          `json:"model"`
	StopReason   string          `json:"stopReason"`
	ErrorMessage string          `json:"errorMessage"`
	ToolCallID   string          `json:"toolCallId"`
	IsError      bool            `json:"isError"`
	Command      string          `json:"command"`
	Output       string          `json:"output"`
}

type block struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

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
		return nil, errors.New("pi: no session header")
	}
	return &Header{ID: e.ID, Cwd: e.Cwd, Timestamp: e.Timestamp}, nil
}

// ParseFile reads a session file.
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
	return c, nil
}

// Parse reads a session from r. Lines that aren't valid JSON are skipped.
func Parse(r io.Reader) (*conv.Conversation, error) {
	c := &conv.Conversation{}
	c.Harness = conv.Pi
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
	for _, e := range branch {
		if e.Timestamp != "" {
			if c.StartedAt == "" {
				c.StartedAt = e.Timestamp
			}
			c.UpdatedAt = e.Timestamp
		}
	}
	b := builder{c: c, cur: -1, tools: map[string][2]int{}}
	for _, e := range branch {
		b.add(e)
	}
	b.closeTurn()
	for _, e := range all {
		if e.Type == "session_info" && e.Name != "" {
			c.Title = e.Name
		}
	}
	c.Finish()
	return c, nil
}

// activeBranch is the path from the root to the last entry, oldest first.
// Entries without ids (older files) are kept in file order.
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
	stop  string
	tools map[string][2]int
}

func (b *builder) add(e *entry) {
	switch e.Type {
	case "model_change":
		b.model = e.ModelID
	case "custom_message":
		b.closeTurn()
		b.c.Entries = append(b.c.Entries, conv.Entry{Kind: conv.EntryEvent, Text: eventTitle(e.CustomType), Detail: rawText(e.Content), At: e.Timestamp})
	case "message":
		if e.Message != nil {
			b.message(e.Message, e.Timestamp)
		}
	}
}

func eventTitle(customType string) string {
	if customType == "" {
		return "Notification"
	}
	return strings.ReplaceAll(customType, "-", " ")
}

func (b *builder) message(m *message, at string) {
	switch m.Role {
	case "user":
		text := strings.TrimSpace(rawText(m.Content))
		if text == "" {
			return
		}
		b.closeTurn()
		b.c.Entries = append(b.c.Entries, conv.Entry{Kind: conv.EntryTurn, Prompt: text, PromptAt: at})
		b.cur = len(b.c.Entries) - 1
		b.stop = ""
	case "bashExecution":
		b.closeTurn()
		b.c.Entries = append(b.c.Entries, conv.Entry{Kind: conv.EntryEvent, Text: "! " + m.Command, Detail: m.Output, At: at})
	case "assistant":
		e := b.turn(at)
		if m.Model != "" {
			b.model = m.Model
		}
		e.Model = b.model
		e.EndedAt = at
		b.stop = m.StopReason
		if m.StopReason == "error" || m.StopReason == "aborted" {
			e.Error = m.ErrorMessage
		}
		var blocks []block
		if json.Unmarshal(m.Content, &blocks) != nil {
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
			case "toolCall":
				input, cut := conv.CapInput(bl.Arguments)
				e.Steps = append(e.Steps, conv.Step{Kind: conv.StepTool, At: at, Tool: bl.Name, Input: input, Truncated: cut})
				b.tools[bl.ID] = [2]int{b.cur, len(e.Steps) - 1}
			}
		}
	case "toolResult":
		idx, ok := b.tools[m.ToolCallID]
		if !ok {
			return
		}
		s := &b.c.Entries[idx[0]].Steps[idx[1]]
		out, cut := conv.CapText(rawText(m.Content))
		s.Output, s.OutputAt, s.IsError = out, at, m.IsError
		s.Truncated = s.Truncated || cut
	}
}

func (b *builder) turn(at string) *conv.Entry {
	if b.cur < 0 {
		b.c.Entries = append(b.c.Entries, conv.Entry{Kind: conv.EntryTurn, PromptAt: at})
		b.cur = len(b.c.Entries) - 1
	}
	return &b.c.Entries[b.cur]
}

// closeTurn sets the turn status from the last assistant stopReason
// ("stop" done, "error", "aborted", else unfinished).
func (b *builder) closeTurn() {
	if b.cur < 0 {
		return
	}
	e := &b.c.Entries[b.cur]
	b.cur = -1
	e.SplitFinal()
	switch b.stop {
	case "stop", "length":
		e.Status = conv.StatusDone
	case "error":
		e.Status = conv.StatusError
	case "aborted":
		e.Status = conv.StatusAborted
	default:
		e.Status = conv.StatusIncomplete
	}
}

// rawText is a string content, or the text blocks of a content array.
func rawText(raw json.RawMessage) string {
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
