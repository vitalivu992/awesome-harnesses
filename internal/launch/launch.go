// Package launch builds the command lines that send a prompt to a harness
// CLI without its interactive UI (claude -p, codex exec, cursor-agent -p,
// pi -p, opencode run, kilo run): in a new conversation or continuing one.
// The browser only sends structured settings (model, effort, fast mode,
// bypass); the argv is always built here, so the preview the settings show is
// exactly what runs.
package launch

import (
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"awesome-harness/internal/conv"
)

// Harness is what the settings need to know about a harness CLI.
type Harness struct {
	ID  string `json:"id"`
	CLI string `json:"cli"`
	// Efforts: the thinking effort values the CLI accepts (a model may support
	// fewer; see Model.Efforts). Empty: the CLI has no effort setting.
	Efforts    []string `json:"efforts"`
	EffortNote string   `json:"effortNote,omitempty"`
	// Fast: the CLI has a fast mode (faster output, higher usage).
	Fast     bool   `json:"fast"`
	FastNote string `json:"fastNote,omitempty"`
	// Bypass: the flag that skips permission prompts. A headless run can't
	// answer them, so without it the agent is denied what needs approval.
	Bypass string `json:"bypass"`
}

// Harnesses in display order (the same ids as conv and the frontend).
var Harnesses = []Harness{
	{ID: conv.Claude, CLI: "claude", Efforts: []string{"low", "medium", "high", "xhigh", "max"}, Fast: true,
		Bypass: "--dangerously-skip-permissions"},
	{ID: conv.Cursor, CLI: "cursor-agent", EffortNote: "Cursor's model names carry the effort (…-high, …-xhigh).",
		FastNote: "Cursor's model names carry the speed: pick a …-fast model.", Bypass: "--force"},
	{ID: conv.Codex, CLI: "codex", Efforts: []string{"minimal", "low", "medium", "high", "xhigh", "max", "ultra"}, Fast: true,
		Bypass: "--dangerously-bypass-approvals-and-sandbox"},
	{ID: conv.Pi, CLI: "pi", Efforts: []string{"off", "minimal", "low", "medium", "high", "xhigh", "max"},
		FastNote: "pi has no fast mode."},
	{ID: conv.OpenCode, CLI: "opencode", Efforts: []string{"minimal", "low", "medium", "high", "xhigh", "max"},
		EffortNote: "Sent as the model variant; which ones exist depends on the provider.", FastNote: "OpenCode has no fast mode.",
		Bypass: "--auto"},
	{ID: conv.Kilo, CLI: "kilo", Efforts: []string{"minimal", "low", "medium", "high", "xhigh", "max"},
		EffortNote: "Sent as the model variant; which ones exist depends on the provider.", FastNote: "Kilo has no fast mode.",
		Bypass: "--auto"},
}

// Find returns the harness with that id.
func Find(id string) (Harness, bool) {
	i := slices.IndexFunc(Harnesses, func(h Harness) bool { return h.ID == id })
	if i < 0 {
		return Harness{}, false
	}
	return Harnesses[i], true
}

// Request is one prompt to send.
type Request struct {
	Harness string `json:"harness"`
	// Session: the conversation to continue; "" starts a new one.
	Session string `json:"session"`
	Cwd     string `json:"cwd"`
	Prompt  string `json:"prompt"`
	// Model, Effort: "" leaves the harness's default.
	Model  string `json:"model"`
	Effort string `json:"effort"`
	Fast   bool   `json:"fast"`
	Bypass bool   `json:"bypass"`
}

// Plan is the command for a Request.
type Plan struct {
	Argv []string `json:"argv"`
	Cwd  string   `json:"cwd"`
	// Session: the conversation id when it is known before the CLI runs (a
	// resumed one, or a new one whose id the CLI takes as a flag); else ""
	// until the CLI reports it (SessionFrom).
	Session string `json:"session"`
	// Command: the argv as one POSIX shell line (cd included), for display.
	Command string `json:"command"`
	// Notes: settings this harness ignores, and how a command was sent.
	Notes []string `json:"notes,omitempty"`
}

var (
	// Model ids: "opus", "claude-opus-5-5[1m]", "openai/gpt-6", "sonnet:high".
	modelRe  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@+\[\]-]{0,127}$`)
	effortRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,23}$`)
	idRe     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
)

// MaxPrompt is the longest prompt accepted (it goes on the command line).
const MaxPrompt = 100_000

// Build checks the request and returns its command. `cmds` are the commands
// and skills the harness has for the request's folder (Commands): a prompt
// starting with one the CLI doesn't expand by itself is expanded here.
// `newID` makes the id of a new conversation where the CLI takes one
// (nil: random UUIDs).
func Build(req Request, cmds []Command, newID func() string) (Plan, error) {
	h, ok := Find(req.Harness)
	if !ok {
		return Plan{}, fmt.Errorf("unknown harness %q", req.Harness)
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return Plan{}, errors.New("the prompt is empty")
	}
	if len(req.Prompt) > MaxPrompt {
		return Plan{}, fmt.Errorf("the prompt is longer than %d bytes", MaxPrompt)
	}
	if strings.ContainsRune(req.Prompt, 0) {
		return Plan{}, errors.New("the prompt contains a NUL byte")
	}
	if req.Model != "" && !modelRe.MatchString(req.Model) {
		return Plan{}, fmt.Errorf("invalid model %q", req.Model)
	}
	if req.Effort != "" && (!effortRe.MatchString(req.Effort) || len(h.Efforts) == 0) {
		return Plan{}, fmt.Errorf("invalid effort %q for %s", req.Effort, h.CLI)
	}
	if req.Session != "" && (!idRe.MatchString(req.Session) || strings.Contains(req.Session, "..")) {
		return Plan{}, fmt.Errorf("invalid session id %q", req.Session)
	}
	if newID == nil {
		newID = NewUUID
	}

	p := Plan{Cwd: req.Cwd, Session: req.Session}
	prompt, command := expand(h.ID, req.Prompt, cmds, &p.Notes)
	if req.Fast && !h.Fast {
		p.Notes = append(p.Notes, h.FastNote)
	}
	// Claude Code and pi have a fixed list; Codex's and OpenCode's depend on the model.
	if req.Effort != "" && (h.ID == conv.Claude || h.ID == conv.Pi) && !slices.Contains(h.Efforts, req.Effort) {
		return Plan{}, fmt.Errorf("%s has no effort %q", h.CLI, req.Effort)
	}
	bypass := req.Bypass && h.Bypass != ""

	var a []string
	switch h.ID {
	case conv.Claude:
		a = []string{"claude", "-p"}
		if req.Session != "" {
			a = append(a, "--resume", req.Session)
		} else {
			p.Session = newID()
			a = append(a, "--session-id", p.Session)
		}
		a = appendIf(a, "--model", req.Model)
		a = appendIf(a, "--effort", req.Effort)
		if req.Fast {
			a = append(a, "--settings", `{"fastMode":true}`)
		}
		if bypass {
			a = append(a, h.Bypass)
		}
		a = append(a, "--", prompt)
	case conv.Codex:
		a = []string{"codex", "exec"}
		if req.Session != "" {
			a = append(a, "resume")
		}
		a = append(a, "--json", "--skip-git-repo-check")
		a = appendIf(a, "--model", req.Model)
		if req.Effort != "" {
			a = append(a, "-c", fmt.Sprintf("model_reasoning_effort=%q", req.Effort))
		}
		if req.Fast {
			a = append(a, "-c", `service_tier="fast"`)
		}
		if bypass {
			a = append(a, h.Bypass)
		}
		a = append(a, "--")
		if req.Session != "" {
			a = append(a, req.Session)
		}
		a = append(a, prompt)
	case conv.Cursor:
		// --trust: a headless run can't answer the "trust this folder?" prompt.
		a = []string{"cursor-agent", "-p", "--output-format", "stream-json", "--trust"}
		a = appendIf(a, "--resume", req.Session)
		a = appendIf(a, "--model", req.Model)
		if bypass {
			a = append(a, h.Bypass)
		}
		a = append(a, "--", prompt)
	case conv.Pi:
		a = []string{"pi", "-p"}
		if req.Session != "" {
			a = append(a, "--session", req.Session)
		} else {
			p.Session = newID()
			a = append(a, "--session-id", p.Session)
		}
		a = appendIf(a, "--model", req.Model)
		a = appendIf(a, "--thinking", req.Effort)
		a = append(a, "--", prompt)
	case conv.OpenCode, conv.Kilo:
		a = []string{h.CLI, "run", "--format", "json"}
		a = appendIf(a, "--session", req.Session)
		a = appendIf(a, "--model", req.Model)
		a = appendIf(a, "--variant", req.Effort)
		a = appendIf(a, "--command", command)
		if bypass {
			a = append(a, h.Bypass)
		}
		// A --command without arguments has no message.
		if prompt != "" {
			a = append(a, "--", prompt)
		}
	}
	p.Argv = a
	p.Command = ShellLine(p.Cwd, a)
	return p, nil
}

func appendIf(a []string, flag, value string) []string {
	if value == "" {
		return a
	}
	return append(a, flag, value)
}

// NewUUID is a random (version 4) UUID.
func NewUUID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

var safeArg = regexp.MustCompile(`^[A-Za-z0-9_./:@%+=,-]+$`)

// Quote is POSIX shell quoting: bare when safe, else single quotes.
func Quote(s string) string {
	if s != "" && safeArg.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ShellLine is `cd <cwd> && <argv>` with every word quoted.
func ShellLine(cwd string, argv []string) string {
	words := make([]string, len(argv))
	for i, a := range argv {
		words[i] = Quote(a)
	}
	line := strings.Join(words, " ")
	if cwd != "" {
		line = "cd " + Quote(cwd) + " && " + line
	}
	return line
}
