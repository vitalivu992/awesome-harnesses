// Package claude reads Claude Code session transcripts:
// ~/.claude/projects/<slug>/<sessionId>.jsonl, one JSON object per line
// (user / assistant / system messages plus metadata lines such as ai-title).
// An assistant message is split over several lines, one per content block;
// tool results come back in the next user line.
package claude

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"awesome-harness/internal/conv"
)

type line struct {
	Type              string          `json:"type"`
	Timestamp         string          `json:"timestamp"`
	Cwd               string          `json:"cwd"`
	GitBranch         string          `json:"gitBranch"`
	IsSidechain       bool            `json:"isSidechain"`
	IsMeta            bool            `json:"isMeta"`
	IsCompactSummary  bool            `json:"isCompactSummary"`
	IsAPIErrorMessage bool            `json:"isApiErrorMessage"`
	Subtype           string          `json:"subtype"`
	Content           json.RawMessage `json:"content"`
	Message           *message        `json:"message"`
	ThinkingMs        int64           `json:"thinkingDurationMs"`
	AiTitle           string          `json:"aiTitle"`
	CustomTitle       string          `json:"customTitle"`
	AgentName         string          `json:"agentName"`
	Summary           string          `json:"summary"`
}

type message struct {
	Model      string          `json:"model"`
	Content    json.RawMessage `json:"content"`
	StopReason string          `json:"stop_reason"`
}

type block struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	Name      string          `json:"name"`
	ID        string          `json:"id"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

var (
	reminderRe   = regexp.MustCompile(`(?s)<system-reminder>.*?</system-reminder>`)
	commandRe    = regexp.MustCompile(`(?s)<command-name>(.*?)</command-name>`)
	commandArgRe = regexp.MustCompile(`(?s)<command-args>(.*?)</command-args>`)
	stdoutRe     = regexp.MustCompile(`(?s)<(?:local-command-stdout|bash-stdout|bash-stderr)>(.*?)</(?:local-command-stdout|bash-stdout|bash-stderr)>`)
	bashInputRe  = regexp.MustCompile(`(?s)<bash-input>(.*?)</bash-input>`)
	taskNoteRe   = regexp.MustCompile(`(?s)^\s*<task-notification>(.*)</task-notification>\s*$`)
)

// parser accumulates entries while reading one session file.
type parser struct {
	c         conv.Conversation
	cur       int // index of the open turn in c.Entries, -1 when none
	acted     bool
	stop      string
	tools     map[string][2]int // tool_use id → (entry, step)
	localOut  []string
	aiTitle   string
	custom    string
	agentName string
	summary   string
	lastAt    string
}

// ParseFile reads a session transcript.
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
	c.ID = strings.TrimSuffix(filepath.Base(path), ".jsonl")
	if st, err := f.Stat(); err == nil {
		c.SizeBytes = st.Size()
	}
	return c, nil
}

// Parse reads a session transcript from r. Lines that aren't valid JSON are skipped.
func Parse(r io.Reader) (*conv.Conversation, error) {
	p := &parser{cur: -1, tools: map[string][2]int{}}
	p.c.Harness = conv.Claude
	br := bufio.NewReaderSize(r, 1<<20)
	for {
		raw, err := br.ReadBytes('\n')
		if len(strings.TrimSpace(string(raw))) > 0 {
			var l line
			if json.Unmarshal(raw, &l) == nil {
				p.add(&l)
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	p.closeTurn()
	switch {
	case p.custom != "":
		p.c.Title = p.custom
	case p.agentName != "":
		p.c.Title = p.agentName
	case p.aiTitle != "":
		p.c.Title = p.aiTitle
	case p.summary != "":
		p.c.Title = p.summary
	}
	p.c.UpdatedAt = p.lastAt
	p.c.Resumable = true
	p.c.Finish()
	return &p.c, nil
}

func (p *parser) add(l *line) {
	switch l.Type {
	case "ai-title":
		p.aiTitle = l.AiTitle
		return
	case "custom-title":
		p.custom = l.CustomTitle
		return
	case "agent-name":
		p.agentName = l.AgentName
		return
	case "summary":
		p.summary = l.Summary
		return
	case "user", "assistant", "system":
	default:
		return
	}
	if l.IsSidechain {
		return
	}
	if l.Timestamp != "" {
		if p.c.StartedAt == "" {
			p.c.StartedAt = l.Timestamp
		}
		p.lastAt = l.Timestamp
	}
	if p.c.Cwd == "" && l.Cwd != "" {
		p.c.Cwd = l.Cwd
	}
	if l.GitBranch != "" && l.GitBranch != "HEAD" {
		p.c.GitBranch = l.GitBranch
	}
	switch l.Type {
	case "user":
		p.user(l)
	case "assistant":
		p.assistant(l)
	case "system":
		if l.Subtype == "local_command" {
			p.localOutput(rawString(l.Content))
		}
	}
}

func (p *parser) user(l *line) {
	if l.Message == nil {
		return
	}
	if l.IsCompactSummary {
		p.closeTurn()
		p.c.Entries = append(p.c.Entries, conv.Entry{
			Kind: conv.EntryEvent, Text: "Conversation compacted", Detail: rawString(l.Message.Content), At: l.Timestamp,
		})
		return
	}
	if l.IsMeta {
		return
	}
	var text string
	var blocks []block
	if json.Unmarshal(l.Message.Content, &text) != nil {
		if json.Unmarshal(l.Message.Content, &blocks) != nil {
			return
		}
	}
	var prompt []string
	if text != "" {
		prompt = append(prompt, text)
	}
	for _, b := range blocks {
		switch b.Type {
		case "tool_result":
			p.toolResult(&b, l.Timestamp)
		case "text":
			if strings.HasPrefix(strings.TrimSpace(b.Text), "[Request interrupted by user") {
				if p.cur >= 0 {
					p.c.Entries[p.cur].Status = conv.StatusAborted
				}
				continue
			}
			prompt = append(prompt, b.Text)
		case "image":
			prompt = append(prompt, "[image]")
		}
	}
	if len(prompt) == 0 {
		return
	}
	p.prompt(strings.Join(prompt, "\n\n"), l.Timestamp)
}

// prompt handles the text of a user line: a slash command, a `!` shell
// command, local command output, or a plain prompt that opens a new turn.
func (p *parser) prompt(text, at string) {
	text = strings.TrimSpace(reminderRe.ReplaceAllString(text, ""))
	if text == "" {
		return
	}
	if strings.HasPrefix(text, "[Request interrupted by user") {
		if p.cur >= 0 {
			p.c.Entries[p.cur].Status = conv.StatusAborted
		}
		return
	}
	if m := taskNoteRe.FindStringSubmatch(text); m != nil {
		p.closeTurn()
		p.c.Entries = append(p.c.Entries, conv.Entry{Kind: conv.EntryEvent, Text: "Background task notification", Detail: strings.TrimSpace(m[1]), At: at})
		return
	}
	if m := commandRe.FindStringSubmatch(text); m != nil {
		cmd := strings.TrimSpace(m[1])
		if !strings.HasPrefix(cmd, "/") {
			cmd = "/" + cmd
		}
		if a := commandArgRe.FindStringSubmatch(text); a != nil && strings.TrimSpace(a[1]) != "" {
			cmd += " " + strings.TrimSpace(a[1])
		}
		p.openTurn(cmd, at)
		return
	}
	if m := bashInputRe.FindStringSubmatch(text); m != nil {
		p.openTurn("! "+strings.TrimSpace(m[1]), at)
		return
	}
	if outs := stdoutRe.FindAllStringSubmatch(text, -1); outs != nil {
		for _, o := range outs {
			p.localOutput(o[0])
		}
		return
	}
	p.openTurn(text, at)
}

func (p *parser) localOutput(s string) {
	for _, m := range stdoutRe.FindAllStringSubmatch(s, -1) {
		if t := strings.TrimSpace(stripANSI(m[1])); t != "" {
			p.localOut = append(p.localOut, t)
		}
	}
}

func (p *parser) openTurn(prompt, at string) {
	p.closeTurn()
	p.c.Entries = append(p.c.Entries, conv.Entry{Kind: conv.EntryTurn, Prompt: prompt, PromptAt: at})
	p.cur = len(p.c.Entries) - 1
	p.acted = false
	p.stop = ""
	p.localOut = nil
}

func (p *parser) ensureTurn(at string) *conv.Entry {
	if p.cur < 0 {
		p.openTurn("", at)
	}
	return &p.c.Entries[p.cur]
}

func (p *parser) assistant(l *line) {
	if l.Message == nil {
		return
	}
	var blocks []block
	if json.Unmarshal(l.Message.Content, &blocks) != nil {
		return
	}
	e := p.ensureTurn(l.Timestamp)
	p.acted = true
	e.EndedAt = l.Timestamp
	if l.IsAPIErrorMessage || l.Message.Model == "<synthetic>" {
		var msg []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				msg = append(msg, b.Text)
			}
		}
		if l.IsAPIErrorMessage {
			e.Status = conv.StatusError
			e.Error = strings.Join(msg, "\n")
		}
		return
	}
	if l.Message.Model != "" {
		e.Model = l.Message.Model
	}
	if l.Message.StopReason != "" {
		p.stop = l.Message.StopReason
	}
	for _, b := range blocks {
		switch b.Type {
		case "thinking":
			if strings.TrimSpace(b.Thinking) == "" {
				continue
			}
			e.Steps = append(e.Steps, conv.Step{Kind: conv.StepThinking, At: l.Timestamp, Text: b.Thinking, DurationMs: l.ThinkingMs})
		case "text":
			if strings.TrimSpace(b.Text) == "" {
				continue
			}
			e.Steps = append(e.Steps, conv.Step{Kind: conv.StepText, At: l.Timestamp, Text: b.Text})
		case "tool_use":
			input, cut := conv.CapInput(b.Input)
			e.Steps = append(e.Steps, conv.Step{Kind: conv.StepTool, At: l.Timestamp, Tool: b.Name, Input: input, Truncated: cut})
			if b.ID != "" {
				p.tools[b.ID] = [2]int{p.cur, len(e.Steps) - 1}
			}
		}
	}
}

func (p *parser) toolResult(b *block, at string) {
	idx, ok := p.tools[b.ToolUseID]
	if !ok {
		return
	}
	s := &p.c.Entries[idx[0]].Steps[idx[1]]
	out, cut := conv.CapText(resultText(b.Content))
	s.Output = out
	s.OutputAt = at
	s.IsError = b.IsError
	s.Truncated = s.Truncated || cut
}

// closeTurn settles the open turn: a slash/shell command the agent never
// answered becomes an event; otherwise the final answer is split off and the
// status set from how the last assistant message stopped.
func (p *parser) closeTurn() {
	if p.cur < 0 {
		if len(p.localOut) > 0 {
			p.c.Entries = append(p.c.Entries, conv.Entry{Kind: conv.EntryEvent, Text: "Command output", Detail: strings.Join(p.localOut, "\n\n"), At: p.lastAt})
			p.localOut = nil
		}
		return
	}
	e := &p.c.Entries[p.cur]
	p.cur = -1
	if !p.acted && (strings.HasPrefix(e.Prompt, "/") || strings.HasPrefix(e.Prompt, "!")) {
		*e = conv.Entry{Kind: conv.EntryEvent, Text: e.Prompt, Detail: strings.Join(p.localOut, "\n\n"), At: e.PromptAt}
		p.localOut = nil
		return
	}
	p.localOut = nil
	e.SplitFinal()
	if e.Status == "" {
		switch {
		case p.stop == "end_turn" || p.stop == "stop_sequence":
			e.Status = conv.StatusDone
		default:
			e.Status = conv.StatusIncomplete
		}
	}
}

// resultText flattens a tool_result content (a string, or text/image blocks).
func resultText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []block
	if json.Unmarshal(raw, &blocks) != nil {
		return string(raw)
	}
	parts := make([]string, 0, len(blocks))
	for _, b := range blocks {
		switch b.Type {
		case "text":
			parts = append(parts, b.Text)
		case "image":
			parts = append(parts, "[image]")
		}
	}
	return strings.Join(parts, "\n")
}

// rawString returns a JSON string's value, or the text blocks of a content array.
func rawString(raw json.RawMessage) string {
	return resultText(raw)
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }

// ReadCwd returns the working directory recorded in the first lines of a
// transcript (Claude Code writes it on every message line), or "".
func ReadCwd(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	br := bufio.NewReaderSize(f, 1<<20)
	for i := 0; i < 200; i++ {
		raw, err := br.ReadBytes('\n')
		var l struct {
			Cwd string `json:"cwd"`
		}
		if json.Unmarshal(raw, &l) == nil && l.Cwd != "" {
			return l.Cwd
		}
		if err != nil {
			return ""
		}
	}
	return ""
}
