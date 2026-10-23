// Package codex reads OpenAI Codex CLI session rollouts:
// ~/.codex/sessions/YYYY/MM/DD/rollout-<time>-<id>.jsonl, one
// {timestamp, type, payload} object per line. The first line is the
// session_meta (id, cwd, git); `response_item` lines are the model's input
// and output items (messages, reasoning, tool calls and their outputs, paired
// by call_id); `event_msg` lines carry turn ends, aborts and errors; and
// `turn_context` lines name the model.
package codex

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"awesome-harness/internal/conv"
)

type line struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type payload struct {
	Type         string          `json:"type"`
	Role         string          `json:"role"`
	Content      json.RawMessage `json:"content"`
	Summary      json.RawMessage `json:"summary"` // reasoning: []textPart; turn_context: a string
	Name         string          `json:"name"`
	Arguments    json.RawMessage `json:"arguments"`
	Input        json.RawMessage `json:"input"`
	Action       json.RawMessage `json:"action"`
	CallID       string          `json:"call_id"`
	Output       json.RawMessage `json:"output"`
	Model        string          `json:"model"`
	Message      string          `json:"message"`
	Reason       string          `json:"reason"`
	ThreadConfig *struct {
		Model string `json:"model"`
	} `json:"thread_settings"`
}

type textPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Meta is the session_meta line.
type Meta struct {
	ID           string          `json:"id"`
	Timestamp    string          `json:"timestamp"`
	Cwd          string          `json:"cwd"`
	Source       json.RawMessage `json:"source"`
	ThreadSource string          `json:"thread_source"`
	Git          *struct {
		Branch string `json:"branch"`
	} `json:"git"`
}

// Subagent reports whether the session was spawned by another session
// (those are shown through the parent's tool calls, not listed).
func (m *Meta) Subagent() bool {
	return m.ThreadSource == "subagent" || strings.HasPrefix(strings.TrimSpace(string(m.Source)), "{")
}

// Context Codex injects as user messages; not typed by the user.
var injected = []string{"<environment_context>", "# AGENTS.md instructions", "<user_instructions>", "<permissions instructions>", "<codex_internal_context"}

// ReadMeta returns a rollout's session_meta (its first line).
func ReadMeta(path string) (*Meta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	raw, err := bufio.NewReaderSize(f, 1<<20).ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	var l line
	if err := json.Unmarshal(raw, &l); err != nil || l.Type != "session_meta" {
		return nil, errors.New("codex: no session_meta")
	}
	var m Meta
	if err := json.Unmarshal(l.Payload, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

type parser struct {
	c     conv.Conversation
	cur   int
	model string
	tools map[string][2]int
	ended bool // the open turn got task_complete / turn_aborted / error
}

// ParseFile reads a rollout.
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

// Parse reads a rollout from r. Lines that aren't valid JSON are skipped.
func Parse(r io.Reader) (*conv.Conversation, error) {
	p := &parser{cur: -1, tools: map[string][2]int{}}
	p.c.Harness = conv.Codex
	p.c.Resumable = true
	br := bufio.NewReaderSize(r, 1<<20)
	for {
		raw, err := br.ReadBytes('\n')
		var l line
		if len(strings.TrimSpace(string(raw))) > 0 && json.Unmarshal(raw, &l) == nil {
			p.add(&l)
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	p.closeTurn()
	p.c.Finish()
	return &p.c, nil
}

func (p *parser) add(l *line) {
	if l.Timestamp != "" {
		if p.c.StartedAt == "" {
			p.c.StartedAt = l.Timestamp
		}
		p.c.UpdatedAt = l.Timestamp
	}
	if l.Type == "session_meta" {
		var m Meta
		if json.Unmarshal(l.Payload, &m) == nil {
			p.c.ID, p.c.Cwd = m.ID, m.Cwd
			if m.Git != nil {
				p.c.GitBranch = m.Git.Branch
			}
		}
		return
	}
	if l.Type == "compacted" {
		p.closeTurn()
		p.c.Entries = append(p.c.Entries, conv.Entry{Kind: conv.EntryEvent, Text: "Conversation compacted", At: l.Timestamp})
		return
	}
	var pl payload
	if json.Unmarshal(l.Payload, &pl) != nil {
		return
	}
	switch l.Type {
	case "turn_context":
		if pl.Model != "" {
			p.model = pl.Model
		}
	case "event_msg":
		p.event(&pl, l.Timestamp)
	case "response_item":
		p.item(&pl, l.Timestamp)
	}
}

func (p *parser) event(pl *payload, at string) {
	switch pl.Type {
	case "thread_settings_applied":
		if pl.ThreadConfig != nil && pl.ThreadConfig.Model != "" {
			p.model = pl.ThreadConfig.Model
		}
	case "task_complete":
		p.end(conv.StatusDone, "", at)
	case "turn_aborted":
		p.end(conv.StatusAborted, "", at)
	case "error":
		p.end(conv.StatusError, pl.Message, at)
	}
}

func (p *parser) end(status, errText, at string) {
	if p.cur < 0 {
		return
	}
	e := &p.c.Entries[p.cur]
	// An abort or error after the turn completed doesn't undo it.
	if p.ended && e.Status == conv.StatusDone && status != conv.StatusDone {
		return
	}
	e.Status, e.Error, e.EndedAt = status, errText, at
	p.ended = true
}

func (p *parser) item(pl *payload, at string) {
	switch pl.Type {
	case "message":
		text := messageText(pl.Content)
		switch pl.Role {
		case "user":
			p.userText(text, at)
		case "assistant":
			if strings.TrimSpace(text) != "" {
				p.step(conv.Step{Kind: conv.StepText, At: at, Text: text})
			}
		}
	case "reasoning":
		var parts []string
		var summary []textPart
		_ = json.Unmarshal(pl.Summary, &summary)
		for _, s := range summary {
			if t := strings.TrimSpace(s.Text); t != "" {
				parts = append(parts, t)
			}
		}
		if t := messageText(pl.Content); strings.TrimSpace(t) != "" {
			parts = append(parts, t)
		}
		if len(parts) > 0 {
			p.step(conv.Step{Kind: conv.StepThinking, At: at, Text: strings.Join(parts, "\n\n")})
		}
	case "function_call":
		p.tool(pl.Name, arguments(pl.Arguments), pl.CallID, at)
	case "custom_tool_call":
		p.tool(pl.Name, pl.Input, pl.CallID, at)
	case "web_search_call":
		p.tool("web_search", pl.Action, pl.CallID, at)
	case "tool_search_call":
		p.tool("tool_search", arguments(pl.Arguments), pl.CallID, at)
	case "function_call_output", "custom_tool_call_output":
		p.output(pl.CallID, pl.Output, at)
	}
}

func (p *parser) userText(text, at string) {
	t := strings.TrimSpace(text)
	if t == "" {
		return
	}
	if strings.HasPrefix(t, "<turn_aborted>") {
		p.end(conv.StatusAborted, "", at)
		return
	}
	for _, prefix := range injected {
		if strings.HasPrefix(t, prefix) {
			return
		}
	}
	p.closeTurn()
	p.c.Entries = append(p.c.Entries, conv.Entry{Kind: conv.EntryTurn, Prompt: t, PromptAt: at})
	p.cur = len(p.c.Entries) - 1
	p.ended = false
}

func (p *parser) turn(at string) *conv.Entry {
	if p.cur < 0 {
		p.c.Entries = append(p.c.Entries, conv.Entry{Kind: conv.EntryTurn, PromptAt: at})
		p.cur = len(p.c.Entries) - 1
		p.ended = false
	}
	e := &p.c.Entries[p.cur]
	if p.model != "" {
		e.Model = p.model
	}
	if !p.ended {
		e.EndedAt = at
	}
	return e
}

func (p *parser) step(s conv.Step) {
	e := p.turn(s.At)
	e.Steps = append(e.Steps, s)
}

func (p *parser) tool(name string, input json.RawMessage, callID, at string) {
	if len(input) == 0 || !json.Valid(input) {
		input = nil
	}
	in, cut := conv.CapInput(input)
	e := p.turn(at)
	e.Steps = append(e.Steps, conv.Step{Kind: conv.StepTool, At: at, Tool: name, Input: in, Truncated: cut})
	if callID != "" {
		p.tools[callID] = [2]int{p.cur, len(e.Steps) - 1}
	}
}

// output attaches a tool result. Older Codex wraps it as
// {"output": "...", "metadata": {"exit_code": N}}; newer writes plain text.
func (p *parser) output(callID string, raw json.RawMessage, at string) {
	idx, ok := p.tools[callID]
	if !ok {
		return
	}
	text := ""
	isErr := false
	var s string
	if json.Unmarshal(raw, &s) == nil {
		text = s
		var wrapped struct {
			Output   string `json:"output"`
			Metadata struct {
				ExitCode *int `json:"exit_code"`
			} `json:"metadata"`
		}
		if json.Unmarshal([]byte(s), &wrapped) == nil && wrapped.Output != "" {
			text = wrapped.Output
			isErr = wrapped.Metadata.ExitCode != nil && *wrapped.Metadata.ExitCode != 0
		}
	} else {
		text = messageText(raw)
		if text == "" {
			text = string(raw)
		}
	}
	st := &p.c.Entries[idx[0]].Steps[idx[1]]
	out, cut := conv.CapText(text)
	st.Output, st.OutputAt, st.IsError = out, at, isErr
	st.Truncated = st.Truncated || cut
}

func (p *parser) closeTurn() {
	if p.cur < 0 {
		return
	}
	e := &p.c.Entries[p.cur]
	p.cur = -1
	e.SplitFinal()
	if e.Status == "" {
		// Older rollouts have no task_complete: a turn that reached an answer finished.
		if e.Final != "" {
			e.Status = conv.StatusDone
		} else {
			e.Status = conv.StatusIncomplete
		}
	}
}

// arguments decodes a function call's arguments: a JSON string holding a
// JSON object (kept as the object), or already an object.
func arguments(raw json.RawMessage) json.RawMessage {
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return raw
	}
	if json.Valid([]byte(s)) {
		return json.RawMessage(s)
	}
	b, _ := json.Marshal(s)
	return b
}

// messageText joins the text parts of a message content array.
func messageText(raw json.RawMessage) string {
	var parts []textPart
	if json.Unmarshal(raw, &parts) != nil {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			return s
		}
		return ""
	}
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case "input_text", "output_text", "text", "reasoning_text":
			texts = append(texts, part.Text)
		case "input_image":
			texts = append(texts, "[image]")
		}
	}
	return strings.Join(texts, "\n\n")
}

// ReadIndex maps session id → thread name from ~/.codex/session_index.jsonl
// (names set with /rename or generated by Codex); the last entry wins.
func ReadIndex(path string) map[string]string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	names := map[string]string{}
	br := bufio.NewReader(f)
	for {
		raw, err := br.ReadBytes('\n')
		var e struct {
			ID   string `json:"id"`
			Name string `json:"thread_name"`
		}
		if json.Unmarshal(raw, &e) == nil && e.ID != "" && e.Name != "" {
			names[e.ID] = e.Name
		}
		if err != nil {
			return names
		}
	}
}
