package launch

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixedID() string { return "new-id" }

func TestBuild(t *testing.T) {
	cases := []struct {
		name    string
		req     Request
		argv    []string
		session string
	}{
		{"claude new", Request{Harness: "claude", Cwd: "/w", Prompt: "hi", Model: "opus", Effort: "high", Fast: true, Bypass: true},
			[]string{"claude", "-p", "--session-id", "new-id", "--model", "opus", "--effort", "high", "--settings", `{"fastMode":true}`,
				"--dangerously-skip-permissions", "--", "hi"}, "new-id"},
		{"claude resume, defaults", Request{Harness: "claude", Session: "abc", Cwd: "/w", Prompt: "-x"},
			[]string{"claude", "-p", "--resume", "abc", "--", "-x"}, "abc"},
		{"codex new", Request{Harness: "codex", Cwd: "/w", Prompt: "hi", Model: "gpt-6", Effort: "xhigh", Fast: true},
			[]string{"codex", "exec", "--json", "--skip-git-repo-check", "--model", "gpt-6", "-c", `model_reasoning_effort="xhigh"`,
				"-c", `service_tier="fast"`, "--", "hi"}, ""},
		{"codex resume", Request{Harness: "codex", Session: "t-1", Cwd: "/w", Prompt: "hi", Bypass: true},
			[]string{"codex", "exec", "resume", "--json", "--skip-git-repo-check", "--dangerously-bypass-approvals-and-sandbox",
				"--", "t-1", "hi"}, "t-1"},
		{"cursor ignores fast", Request{Harness: "cursor", Session: "c-1", Cwd: "/w", Prompt: "hi", Model: "auto", Fast: true, Bypass: true},
			[]string{"cursor-agent", "-p", "--output-format", "stream-json", "--trust", "--resume", "c-1", "--model", "auto", "--force",
				"--", "hi"}, "c-1"},
		{"pi new, no bypass flag", Request{Harness: "pi", Cwd: "/w", Prompt: "hi", Effort: "off", Bypass: true},
			[]string{"pi", "-p", "--session-id", "new-id", "--thinking", "off", "--", "hi"}, "new-id"},
		{"opencode", Request{Harness: "opencode", Session: "ses_1", Cwd: "/w", Prompt: "hi", Model: "openai/gpt-6", Effort: "max", Bypass: true},
			[]string{"opencode", "run", "--format", "json", "--session", "ses_1", "--model", "openai/gpt-6", "--variant", "max", "--auto",
				"--", "hi"}, "ses_1"},
		{"kilo", Request{Harness: "kilo", Cwd: "/w", Prompt: "hi"},
			[]string{"kilo", "run", "--format", "json", "--", "hi"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := Build(tc.req, nil, fixedID)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(p.Argv, tc.argv) {
				t.Errorf("argv\n got %q\nwant %q", p.Argv, tc.argv)
			}
			if p.Session != tc.session {
				t.Errorf("session %q, want %q", p.Session, tc.session)
			}
		})
	}
}

func TestBuildNotesAndCommandLine(t *testing.T) {
	p, err := Build(Request{Harness: "cursor", Cwd: "/my work", Prompt: "it's", Fast: true}, nil, fixedID)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Notes) != 1 || !strings.Contains(p.Notes[0], "-fast") {
		t.Errorf("notes %q", p.Notes)
	}
	want := `cd '/my work' && cursor-agent -p --output-format stream-json --trust -- 'it'\''s'`
	if p.Command != want {
		t.Errorf("command\n got %s\nwant %s", p.Command, want)
	}
}

func TestBuildRejects(t *testing.T) {
	for name, req := range map[string]Request{
		"unknown harness":    {Harness: "nope", Prompt: "x"},
		"empty prompt":       {Harness: "claude", Prompt: "  \n"},
		"flag as model":      {Harness: "claude", Prompt: "x", Model: "--dangerously-skip-permissions"},
		"spaces in model":    {Harness: "claude", Prompt: "x", Model: "a b"},
		"unknown effort":     {Harness: "claude", Prompt: "x", Effort: "ludicrous"},
		"effort for cursor":  {Harness: "cursor", Prompt: "x", Effort: "high"},
		"flag as effort":     {Harness: "opencode", Prompt: "x", Effort: "-x"},
		"path as session":    {Harness: "claude", Prompt: "x", Session: "../etc"},
		"flag as session":    {Harness: "claude", Prompt: "x", Session: "-r"},
		"NUL in the prompt":  {Harness: "claude", Prompt: "a\x00b"},
		"prompt over limits": {Harness: "claude", Prompt: strings.Repeat("x", MaxPrompt+1)},
	} {
		if _, err := Build(req, nil, fixedID); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func inserts(cmds []Command) []string {
	var out []string
	for _, c := range cmds {
		out = append(out, c.Insert+"|"+c.Source)
	}
	return out
}

func TestCommandsClaude(t *testing.T) {
	root := t.TempDir()
	d := Dirs{Claude: filepath.Join(root, "claude")}
	cwd := filepath.Join(root, "proj")
	write(t, filepath.Join(d.Claude, "commands", "commit.md"), "---\ndescription: Create commits\n---\nbody")
	write(t, filepath.Join(d.Claude, "commands", "frontend", "component.md"), "body")
	write(t, filepath.Join(d.Claude, "skills", "fix", "SKILL.md"), "---\nname: fix-backlog\ndescription: >\n  Clear the\n  backlog\n---\n")
	write(t, filepath.Join(d.Claude, "skills", "hidden", "SKILL.md"), "---\nname: hidden\nuser-invocable: false\n---\n")
	write(t, filepath.Join(d.Claude, "skills", "nofile", "README.md"), "")
	write(t, filepath.Join(cwd, ".claude", "commands", "commit.md"), "---\ndescription: Project commit\n---\n")
	plugin := filepath.Join(root, "plugin")
	write(t, filepath.Join(plugin, "commands", "help.md"), "---\ndescription: Help\n---\n")
	write(t, filepath.Join(plugin, "skills", "web", "SKILL.md"), "---\nname: ralph-web\n---\n")
	write(t, filepath.Join(d.Claude, "skills", "synced", "bucket-1", "docx", "SKILL.md"), "---\nname: docx\n---\n")
	write(t, filepath.Join(d.Claude, "skills", "synced", "bucket-1", "manifest.json"), "{}")
	write(t, filepath.Join(d.Claude, "plugins", "installed_plugins.json"),
		`{"plugins":{"ralph@m":[{"installPath":"`+plugin+`"}],"off@m":[{"installPath":"`+plugin+`"}]}}`)
	write(t, filepath.Join(d.Claude, "settings.json"), `{"enabledPlugins":{"ralph@m":true,"off@m":false}}`)

	cmds := Commands("claude", cwd, d)
	// Plugin skills are named after their folder.
	want := []string{"/anthropic-skills:docx|synced", "/commit|project", "/fix-backlog|user", "/frontend:component|user", "/ralph:help|ralph", "/ralph:web|ralph"}
	if got := inserts(cmds); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q\nwant %q", got, want)
	}
	if cmds[1].Description != "Project commit" || cmds[2].Description != "Clear the backlog" || cmds[2].Kind != "skill" {
		t.Errorf("descriptions/kinds: %+v", cmds[1:3])
	}
}

func TestCommandsOtherHarnesses(t *testing.T) {
	root := t.TempDir()
	d := Dirs{Home: root, Codex: filepath.Join(root, "codex"), Pi: filepath.Join(root, "pi"), OpenCode: filepath.Join(root, "oc"),
		Claude: filepath.Join(root, "claude")}
	write(t, filepath.Join(d.Codex, "skills", "pw", "SKILL.md"), "---\nname: playwright\n---\n")
	write(t, filepath.Join(d.Codex, "prompts", "review.md"), "Review $1 carefully")
	write(t, filepath.Join(d.Pi, "agent", "prompts", "commit.md"), "x")
	write(t, filepath.Join(d.Pi, "agent", "skills", "wiki", "SKILL.md"), "---\nname: llm-wiki\n---\n")
	write(t, filepath.Join(d.OpenCode, "commands", "test.md"), "Run tests")
	write(t, filepath.Join(d.Claude, "skills", "fix", "SKILL.md"), "---\nname: fix\n---\n")
	// A symlinked commands folder is followed.
	write(t, filepath.Join(root, "shared", "lint.md"), "Lint")
	if err := os.Symlink(filepath.Join(root, "shared"), filepath.Join(d.OpenCode, "command")); err != nil {
		t.Fatal(err)
	}

	for harness, want := range map[string][]string{
		"codex":    {"$playwright|user", "/prompts:review|user"},
		"pi":       {"/commit|user", "/skill:llm-wiki|user"},
		"opencode": {"/fix|user", "/lint|user", "/test|user"},
	} {
		if got := inserts(Commands(harness, "", d)); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %q, want %q", harness, got, want)
		}
	}
}

func TestBuildExpandsCommands(t *testing.T) {
	root := t.TempDir()
	d := Dirs{Codex: filepath.Join(root, "codex"), OpenCode: filepath.Join(root, "oc"), Cursor: filepath.Join(root, "cursor"),
		Claude: filepath.Join(root, "claude")}
	write(t, filepath.Join(d.Codex, "prompts", "review.md"), "---\ndescription: d\n---\nReview $1 then $ARGUMENTS")
	write(t, filepath.Join(d.OpenCode, "command", "test.md"), "Run tests")
	write(t, filepath.Join(d.Cursor, "skills", "fix", "SKILL.md"), "---\nname: fix\n---\n")
	write(t, filepath.Join(d.Claude, "skills", "fix", "SKILL.md"), "---\nname: fix\n---\n")

	build := func(harness, prompt string) Plan {
		t.Helper()
		p, err := Build(Request{Harness: harness, Cwd: "/w", Prompt: prompt}, Commands(harness, "", d), fixedID)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	if p := build("codex", "/prompts:review main.go now"); p.Argv[len(p.Argv)-1] != "Review main.go then main.go now" || len(p.Notes) != 1 {
		t.Errorf("codex template: %q %q", p.Argv, p.Notes)
	}
	if p := build("codex", "/prompts:reviewer x"); p.Argv[len(p.Argv)-1] != "/prompts:reviewer x" {
		t.Errorf("a longer name must not match: %q", p.Argv)
	}
	if p := build("opencode", "/test unit only"); !reflect.DeepEqual(p.Argv[4:], []string{"--command", "test", "--", "unit only"}) {
		t.Errorf("opencode command: %q", p.Argv)
	}
	if p := build("opencode", "/test"); !reflect.DeepEqual(p.Argv[4:], []string{"--command", "test"}) {
		t.Errorf("opencode command without args: %q", p.Argv)
	}
	if p := build("cursor", "/fix the bug"); !strings.HasPrefix(p.Argv[len(p.Argv)-1], `Use your "fix" skill (`) ||
		!strings.HasSuffix(p.Argv[len(p.Argv)-1], "\n\nthe bug") {
		t.Errorf("cursor skill: %q", p.Argv[len(p.Argv)-1])
	}
	if p := build("claude", "/fix the bug"); p.Argv[len(p.Argv)-1] != "/fix the bug" || len(p.Notes) != 0 {
		t.Errorf("claude expands skills itself: %q", p.Argv)
	}
}

func TestFillTemplate(t *testing.T) {
	if got := fillTemplate("Do it", "now"); got != "Do it\n\nnow" {
		t.Errorf("no placeholder: %q", got)
	}
	if got := fillTemplate("Do $2 $1", "a"); got != "Do  a" {
		t.Errorf("missing positional: %q", got)
	}
}

func TestModelParsers(t *testing.T) {
	cursor := parseCursorModels([]byte("Available models\n\nauto - Auto (default)\ngpt-5.3-codex-fast - Codex 5.3 Fast\n\nTip: x\n"))
	if len(cursor) != 2 || cursor[0].ID != "auto" || cursor[0].Name != "Auto" || cursor[1].Name != "Codex 5.3 Fast" {
		t.Errorf("cursor: %+v", cursor)
	}
	pi := parsePiModels([]byte("provider  model  context  max-out  thinking  images\nanthropic claude-x 200K 64K yes yes\nlocal tiny 8K 4K no no\n"))
	if len(pi) != 2 || pi[0].ID != "anthropic/claude-x" || pi[0].Efforts != nil || pi[1].Efforts == nil {
		t.Errorf("pi: %+v", pi)
	}
	oc := parseOpenCodeModels([]byte("opencode/a\n{\n  \"name\": \"A\",\n  \"variants\": {\"high\": {}, \"low\": {}}\n}\nopenai/b\n{\n  \"name\": \"B\"\n}\n"))
	if len(oc) != 2 || oc[0].ID != "opencode/a" || !reflect.DeepEqual(oc[0].Efforts, []string{"low", "high"}) || oc[1].Name != "B" || oc[1].Efforts != nil {
		t.Errorf("opencode: %+v", oc)
	}
}

func TestCodexModels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models_cache.json")
	write(t, path, `{"models":[{"slug":"gpt-6","display_name":"GPT-6","visibility":"list","supported_reasoning_levels":[{"effort":"low"},{"effort":"high"}],"additional_speed_tiers":["fast"]},{"slug":"hidden","visibility":"hide"}]}`)
	models, err := codexModels(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].ID != "gpt-6" || !models[0].Fast || !reflect.DeepEqual(models[0].Efforts, []string{"low", "high"}) {
		t.Errorf("%+v", models)
	}
}
