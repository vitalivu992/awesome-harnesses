// Package cursor reads Cursor agent transcripts:
// ~/.cursor/projects/<slug>/agent-transcripts/<id>/<id>.jsonl, written by both
// the Cursor IDE agent and the cursor-agent CLI. Each line is a message
// ({role, message: {content: [text | tool_use]}}) or a turn end marker
// ({type: "turn_ended", status, error}). Transcripts carry no model, no tool
// results and no per-message timestamps; prompts sometimes start with a
// <timestamp> tag, which is the only time recorded inside the file.
package cursor

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"awesome-harness/internal/conv"
)

type line struct {
	Role    string   `json:"role"`
	Message *message `json:"message"`
	Type    string   `json:"type"`
	Status  string   `json:"status"`
	Error   string   `json:"error"`
}

type message struct {
	Content json.RawMessage `json:"content"`
}

type block struct {
	Type  string          `json:"type"`
	Text  string          `json:"text"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

var (
	timestampRe = regexp.MustCompile(`(?s)<timestamp>(.*?)</timestamp>`)
	queryRe     = regexp.MustCompile(`(?s)<user_query>(.*?)</user_query>`)
	// A context block Cursor prepends to a prompt (attached skills, links, tool catalogs…).
	contextRe  = regexp.MustCompile(`(?m)^<([a-z_]+)>`)
	redactedRe = regexp.MustCompile(`(?:\s*\[REDACTED\])+\s*$`)
	tzRe       = regexp.MustCompile(`\(UTC(?:([+-])(\d{1,2})(?::(\d{2}))?)?\)\s*$`)
)

// ParseFile reads a transcript. The id is the file name; the start time is
// the first prompt <timestamp> or, failing that, the file's birth time.
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
		c.UpdatedAt = st.ModTime().UTC().Format(time.RFC3339Nano)
		if c.StartedAt == "" {
			c.StartedAt = birthTime(path, st).UTC().Format(time.RFC3339Nano)
		}
	}
	return c, nil
}

// Parse reads a transcript from r. Lines that aren't valid JSON are skipped.
func Parse(r io.Reader) (*conv.Conversation, error) {
	c := &conv.Conversation{}
	c.Harness = conv.Cursor
	cur := -1
	closeTurn := func(status, errText string) {
		if cur < 0 {
			return
		}
		e := &c.Entries[cur]
		e.SplitFinal()
		// Transcripts from before mid-2026 have no turn_ended lines: a turn
		// that got to a final answer finished.
		if status == conv.StatusIncomplete && e.Final != "" {
			status = conv.StatusDone
		}
		e.Status = status
		e.Error = errText
		cur = -1
	}
	br := bufio.NewReaderSize(r, 1<<20)
	for {
		raw, err := br.ReadBytes('\n')
		var l line
		if len(strings.TrimSpace(string(raw))) > 0 && json.Unmarshal(raw, &l) == nil {
			switch {
			case l.Type == "turn_ended":
				status, errText := turnStatus(l.Status, l.Error)
				closeTurn(status, errText)
			case l.Role == "user" && l.Message != nil:
				prompt, at := userPrompt(l.Message.Content)
				closeTurn(conv.StatusIncomplete, "")
				if c.StartedAt == "" && at != "" {
					c.StartedAt = at
				}
				c.Entries = append(c.Entries, conv.Entry{Kind: conv.EntryTurn, Prompt: prompt, PromptAt: at})
				cur = len(c.Entries) - 1
			case l.Role == "assistant" && l.Message != nil:
				if cur < 0 {
					c.Entries = append(c.Entries, conv.Entry{Kind: conv.EntryTurn})
					cur = len(c.Entries) - 1
				}
				addAssistant(&c.Entries[cur], l.Message.Content)
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
	}
	closeTurn(conv.StatusIncomplete, "")
	c.Finish()
	return c, nil
}

func turnStatus(status, errText string) (string, string) {
	switch {
	case status == "success":
		return conv.StatusDone, ""
	case status == "aborted" || strings.Contains(strings.ToLower(errText), "abort"):
		return conv.StatusAborted, ""
	default:
		return conv.StatusError, errText
	}
}

func addAssistant(e *conv.Entry, raw json.RawMessage) {
	var blocks []block
	if json.Unmarshal(raw, &blocks) != nil {
		return
	}
	for _, b := range blocks {
		switch b.Type {
		case "text":
			text := strings.TrimSpace(redactedRe.ReplaceAllString(b.Text, ""))
			if text != "" {
				e.Steps = append(e.Steps, conv.Step{Kind: conv.StepText, Text: text})
			}
		case "tool_use":
			input, cut := conv.CapInput(b.Input)
			e.Steps = append(e.Steps, conv.Step{Kind: conv.StepTool, Tool: b.Name, Input: input, Truncated: cut})
		}
	}
}

// userPrompt returns what the user typed (the <user_query>, else the text
// without Cursor's context blocks) and the prompt's <timestamp> as RFC 3339.
func userPrompt(raw json.RawMessage) (string, string) {
	var blocks []block
	if json.Unmarshal(raw, &blocks) != nil {
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return "", ""
		}
		blocks = []block{{Type: "text", Text: s}}
	}
	var texts []string
	at := ""
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if m := timestampRe.FindStringSubmatch(b.Text); m != nil && at == "" {
				at = ParseTimestamp(m[1])
			}
			if m := queryRe.FindStringSubmatch(b.Text); m != nil {
				texts = append(texts, strings.TrimSpace(m[1]))
				continue
			}
			if t := stripContext(b.Text); t != "" {
				texts = append(texts, t)
			}
		case "image":
			texts = append(texts, "[image]")
		}
	}
	return strings.Join(texts, "\n\n"), at
}

// stripContext removes the <tag>…</tag> blocks that start a line.
func stripContext(text string) string {
	for from := 0; ; {
		loc := contextRe.FindStringSubmatchIndex(text[from:])
		if loc == nil {
			break
		}
		start := from + loc[0]
		closing := "</" + text[from+loc[2]:from+loc[3]] + ">"
		end := strings.Index(text[start:], closing)
		if end < 0 {
			from = start + loc[1] - loc[0]
			continue
		}
		text = text[:start] + text[start+end+len(closing):]
		from = start
	}
	return strings.TrimSpace(text)
}

// ParseTimestamp turns Cursor's "Friday, Oct 23, 2026, 1:46 PM (UTC+7)" into
// RFC 3339, or "" when it doesn't parse.
func ParseTimestamp(s string) string {
	s = strings.TrimSpace(s)
	loc := time.UTC
	if m := tzRe.FindStringSubmatch(s); m != nil {
		s = strings.TrimSpace(s[:len(s)-len(m[0])])
		if m[2] != "" {
			h, _ := strconv.Atoi(m[2])
			mins, _ := strconv.Atoi(m[3])
			off := h*3600 + mins*60
			if m[1] == "-" {
				off = -off
			}
			loc = time.FixedZone("", off)
		}
	}
	for _, layout := range []string{"Monday, Jan 2, 2006, 3:04 PM", "Monday, January 2, 2006, 3:04 PM", "Jan 2, 2006, 3:04 PM"} {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return ""
}
