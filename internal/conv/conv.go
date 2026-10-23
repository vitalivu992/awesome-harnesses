// Package conv is the harness-neutral conversation model the API serves: the
// Claude Code and Cursor parsers both turn their transcript files into a
// Conversation, so the UI renders one shape.
package conv

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// Harness ids.
const (
	Claude   = "claude"
	Cursor   = "cursor"
	Codex    = "codex"
	Pi       = "pi"
	OpenCode = "opencode"
	Kilo     = "kilo"
)

// Turn and conversation statuses.
const (
	StatusDone       = "done"       // the agent finished its answer
	StatusError      = "error"      // the turn ended with an API/agent error
	StatusAborted    = "aborted"    // the user interrupted the turn
	StatusIncomplete = "incomplete" // the transcript stops mid-turn (crash, kill, or still running)
)

// Step kinds.
const (
	StepThinking = "thinking"
	StepText     = "text"
	StepTool     = "tool"
)

// Entry kinds.
const (
	EntryTurn  = "turn"  // a user prompt and the agent's work on it
	EntryEvent = "event" // a note between turns (compaction, local command output)
)

// MaxText caps one text value (tool output, a long tool input field) so a
// huge Read or Bash result can't bloat the response.
const MaxText = 40000

// Step is one piece of the agent's work inside a turn.
type Step struct {
	Kind      string          `json:"kind"`
	At        string          `json:"at,omitempty"`
	Text      string          `json:"text,omitempty"`
	Tool      string          `json:"tool,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	Output    string          `json:"output,omitempty"`
	OutputAt  string          `json:"outputAt,omitempty"`
	IsError   bool            `json:"isError,omitempty"`
	Truncated bool            `json:"truncated,omitempty"`
	// DurationMs is the thinking time Claude Code records for a thinking block.
	DurationMs int64 `json:"durationMs,omitempty"`
}

// Entry is a turn (prompt → work → final answer) or an event between turns.
type Entry struct {
	Kind string `json:"kind"`
	// Turn fields.
	Prompt   string `json:"prompt,omitempty"`
	PromptAt string `json:"promptAt,omitempty"`
	Model    string `json:"model,omitempty"`
	Steps    []Step `json:"steps,omitempty"`
	Final    string `json:"final,omitempty"`
	FinalAt  string `json:"finalAt,omitempty"`
	EndedAt  string `json:"endedAt,omitempty"`
	Status   string `json:"status,omitempty"`
	Error    string `json:"error,omitempty"`
	// Event fields.
	Text   string `json:"text,omitempty"`
	Detail string `json:"detail,omitempty"`
	At     string `json:"at,omitempty"`
}

// Summary is what the conversation list shows.
type Summary struct {
	ID          string   `json:"id"`
	Harness     string   `json:"harness"`
	Title       string   `json:"title"`
	ProjectPath string   `json:"projectPath"`
	Cwd         string   `json:"cwd,omitempty"`
	GitBranch   string   `json:"gitBranch,omitempty"`
	StartedAt   string   `json:"startedAt,omitempty"`
	UpdatedAt   string   `json:"updatedAt,omitempty"`
	Models      []string `json:"models,omitempty"`
	Status      string   `json:"status"`
	Turns       int      `json:"turns"`
	ToolCalls   int      `json:"toolCalls"`
	SizeBytes   int64    `json:"sizeBytes"`
	// Active: the file changed in the last few minutes (the session may still be open).
	Active bool `json:"active,omitempty"`
	// Resumable: the harness CLI can resume it (Cursor IDE-only chats can't).
	Resumable bool `json:"resumable"`
}

// Conversation is one transcript, summary plus its entries.
type Conversation struct {
	Summary
	Entries []Entry `json:"entries"`
}

// Finish fills the summary fields derived from the entries: turn and tool
// counts, models in order of first use, the last turn's status, and the
// first prompt as the title when none was set.
func (c *Conversation) Finish() {
	c.Turns, c.ToolCalls, c.Models = 0, 0, nil
	seen := map[string]bool{}
	status := ""
	for _, e := range c.Entries {
		if e.Kind != EntryTurn {
			continue
		}
		c.Turns++
		status = e.Status
		for _, s := range e.Steps {
			if s.Kind == StepTool {
				c.ToolCalls++
			}
		}
		if e.Model != "" && !seen[e.Model] {
			seen[e.Model] = true
			c.Models = append(c.Models, e.Model)
		}
		if c.Title == "" {
			c.Title = TitleFrom(e.Prompt)
		}
	}
	if status == "" {
		status = StatusDone
	}
	c.Status = status
	if c.Title == "" {
		c.Title = "(empty conversation)"
	}
	if c.Entries == nil {
		c.Entries = []Entry{}
	}
}

// TitleFrom shortens a prompt to a one-line title.
func TitleFrom(prompt string) string {
	line := strings.Join(strings.Fields(prompt), " ")
	return Truncate(line, 120)
}

// Truncate shortens s to at most n runes, adding "…" when it cut.
func Truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}

// CapText caps s at MaxText bytes (on a rune boundary) and reports whether it cut.
func CapText(s string) (string, bool) {
	if len(s) <= MaxText {
		return s, false
	}
	cut := MaxText
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut], true
}

// CapInput caps every long string inside a tool input (a Write's content, a
// big Edit) and re-encodes it; the bool reports whether anything was cut.
func CapInput(raw json.RawMessage) (json.RawMessage, bool) {
	if len(raw) <= MaxText {
		return raw, false
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		s, _ := CapText(string(raw))
		b, _ := json.Marshal(s)
		return b, true
	}
	cut := false
	var walk func(any) any
	walk = func(x any) any {
		switch t := x.(type) {
		case string:
			s, c := CapText(t)
			cut = cut || c
			return s
		case map[string]any:
			for k, val := range t {
				t[k] = walk(val)
			}
		case []any:
			for i, val := range t {
				t[i] = walk(val)
			}
		}
		return x
	}
	v = walk(v)
	b, err := json.Marshal(v)
	if err != nil {
		return raw, false
	}
	return b, cut
}

// SplitFinal moves the trailing text steps of a turn (the agent's answer
// after its last tool call or thought) into Final.
func (e *Entry) SplitFinal() {
	end := len(e.Steps)
	start := end
	for start > 0 && e.Steps[start-1].Kind == StepText {
		start--
	}
	if start == end {
		return
	}
	parts := make([]string, 0, end-start)
	for _, s := range e.Steps[start:end] {
		if t := strings.TrimSpace(s.Text); t != "" {
			parts = append(parts, t)
		}
	}
	e.Final = strings.Join(parts, "\n\n")
	e.FinalAt = e.Steps[end-1].At
	e.Steps = e.Steps[:start]
}
