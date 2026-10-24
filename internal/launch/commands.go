package launch

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"awesome-harness/internal/conv"
)

// Dirs says where the harnesses keep their user-level commands and skills.
type Dirs struct {
	Home     string // ~ (for ~/.agents/skills)
	Claude   string // ~/.claude
	Cursor   string // ~/.cursor
	Codex    string // ~/.codex
	Pi       string // ~/.pi
	OpenCode string // ~/.config/opencode
	Kilo     string // ~/.config/kilo
}

// DefaultDirs uses the standard locations.
func DefaultDirs() Dirs {
	home, _ := os.UserHomeDir()
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		config = filepath.Join(home, ".config")
	}
	codex := os.Getenv("CODEX_HOME")
	if codex == "" {
		codex = filepath.Join(home, ".codex")
	}
	return Dirs{
		Home:     home,
		Claude:   filepath.Join(home, ".claude"),
		Cursor:   filepath.Join(home, ".cursor"),
		Codex:    codex,
		Pi:       filepath.Join(home, ".pi"),
		OpenCode: filepath.Join(config, "opencode"),
		Kilo:     filepath.Join(config, "kilo"),
	}
}

// How a command reaches the agent.
const (
	sendNative   = "native"   // the CLI expands it itself (claude -p, pi -p, codex $skill)
	sendTemplate = "template" // a markdown prompt template: expanded here
	sendCommand  = "command"  // opencode / kilo run --command <name>
	sendSkill    = "skill"    // a skill the CLI doesn't expand: an instruction to use it
)

// Command is a slash command or skill the prompt box offers.
type Command struct {
	Name string `json:"name"`
	// Insert is what the prompt starts with to use it: "/fix-backlog",
	// "/ralph-loop:help", "/skill:llm-wiki", "$playwright".
	Insert      string `json:"insert"`
	Kind        string `json:"kind"` // "skill" or "command"
	Description string `json:"description,omitempty"`
	Source      string `json:"source"` // "user", "project" or the plugin's name
	Path        string `json:"-"`
	send        string
}

// Commands lists the commands and skills of a harness for a folder, project
// ones first (they win over a user one with the same name), then by name.
func Commands(harness, cwd string, d Dirs) []Command {
	var out []Command
	add := func(cmds ...Command) { out = append(out, cmds...) }
	proj := func(rel string) string {
		if cwd == "" {
			return ""
		}
		return filepath.Join(cwd, rel)
	}
	switch harness {
	case conv.Claude:
		add(templates(proj(".claude/commands"), "/", "project", sendNative, true)...)
		add(skills(proj(".claude/skills"), "/", "project", sendNative)...)
		add(templates(filepath.Join(d.Claude, "commands"), "/", "user", sendNative, true)...)
		add(skills(filepath.Join(d.Claude, "skills"), "/", "user", sendNative)...)
		add(claudePlugins(d.Claude)...)
		add(claudeSynced(filepath.Join(d.Claude, "skills", "synced"))...)
	case conv.Codex:
		add(skills(proj(".agents/skills"), "$", "project", sendNative)...)
		add(skills(proj(".codex/skills"), "$", "project", sendNative)...)
		add(skills(filepath.Join(d.Codex, "skills"), "$", "user", sendNative)...)
		add(skills(filepath.Join(d.Codex, "skills", ".system"), "$", "user", sendNative)...)
		add(skills(filepath.Join(d.Home, ".agents", "skills"), "$", "user", sendNative)...)
		// Custom prompts: the TUI expands /prompts:<name>, codex exec doesn't.
		add(templates(filepath.Join(d.Codex, "prompts"), "/prompts:", "user", sendTemplate, false)...)
	case conv.Cursor:
		add(templates(proj(".cursor/commands"), "/", "project", sendTemplate, false)...)
		add(skills(proj(".cursor/skills"), "/", "project", sendSkill)...)
		add(templates(filepath.Join(d.Cursor, "commands"), "/", "user", sendTemplate, false)...)
		add(skills(filepath.Join(d.Cursor, "skills"), "/", "user", sendSkill)...)
	case conv.Pi:
		add(templates(proj(".pi/prompts"), "/", "project", sendNative, false)...)
		add(skills(proj(".pi/skills"), "/skill:", "project", sendNative)...)
		add(templates(filepath.Join(d.Pi, "agent", "prompts"), "/", "user", sendNative, false)...)
		add(skills(filepath.Join(d.Pi, "agent", "skills"), "/skill:", "user", sendNative)...)
	case conv.OpenCode, conv.Kilo:
		dot, cfg := ".opencode", d.OpenCode
		if harness == conv.Kilo {
			dot, cfg = ".kilo", d.Kilo
		}
		for _, sub := range []string{"command", "commands"} {
			add(templates(proj(filepath.Join(dot, sub)), "/", "project", sendCommand, false)...)
			add(templates(filepath.Join(cfg, sub), "/", "user", sendCommand, false)...)
		}
		for _, sub := range []string{"skill", "skills"} {
			add(skills(proj(filepath.Join(dot, sub)), "/", "project", sendSkill)...)
			add(skills(filepath.Join(cfg, sub), "/", "user", sendSkill)...)
		}
		add(skills(proj(".claude/skills"), "/", "project", sendSkill)...)
		add(skills(filepath.Join(d.Claude, "skills"), "/", "user", sendSkill)...)
	}

	seen := map[string]bool{}
	kept := out[:0]
	for _, c := range out {
		if !seen[c.Insert] {
			seen[c.Insert] = true
			kept = append(kept, c)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].Name < kept[j].Name })
	return kept
}

// templates: markdown prompt files in dir; with `nested`, sub-folders become
// "folder:name" (Claude). Names starting with "-" are skipped: they would
// read as flags.
func templates(dir, prefix, source, send string, nested bool) []Command {
	if dir == "" {
		return nil
	}
	// WalkDir doesn't follow a symlinked folder (~/.config/opencode/commands → ~/.claude/commands).
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	var out []Command
	filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if e.IsDir() {
			if path != dir && !nested {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(e.Name(), ".md") {
			return nil
		}
		rel, _ := filepath.Rel(dir, strings.TrimSuffix(path, ".md"))
		name := strings.ReplaceAll(filepath.ToSlash(rel), "/", ":")
		if !validName(name) {
			return nil
		}
		meta, _ := frontmatter(path)
		out = append(out, Command{Name: name, Insert: prefix + name, Kind: "command", Description: meta["description"],
			Source: source, Path: path, send: send})
		return nil
	})
	return out
}

// skills: <dir>/<name>/SKILL.md; the frontmatter name wins over the folder's.
// Skills marked `user-invocable: false` aren't offered.
func skills(dir, prefix, source, send string) []Command {
	return skillsNamed(dir, prefix, source, send, false)
}

// skillsNamed is skills; with `byFolder` the folder's name is the skill's (Claude plugins).
func skillsNamed(dir, prefix, source, send string, byFolder bool) []Command {
	if dir == "" {
		return nil
	}
	entries, _ := os.ReadDir(dir)
	var out []Command
	for _, e := range entries {
		path := filepath.Join(dir, e.Name(), "SKILL.md")
		meta, ok := frontmatter(path)
		if !ok || meta["user-invocable"] == "false" {
			continue
		}
		name := meta["name"]
		if name == "" || byFolder {
			name = e.Name()
		}
		if !validName(name) {
			continue
		}
		out = append(out, Command{Name: name, Insert: prefix + name, Kind: "skill", Description: meta["description"],
			Source: source, Path: path, send: send})
	}
	return out
}

func validName(name string) bool {
	return name != "" && len(name) <= 128 && !strings.HasPrefix(name, "-") &&
		!strings.ContainsAny(name, " \t\r\n\x00")
}

// claudePlugins: the commands and skills of the enabled plugins, as
// /<plugin>:<name>.
func claudePlugins(dir string) []Command {
	var installed struct {
		Plugins map[string][]struct {
			InstallPath string `json:"installPath"`
		} `json:"plugins"`
	}
	var settings struct {
		EnabledPlugins map[string]bool `json:"enabledPlugins"`
	}
	if b, err := os.ReadFile(filepath.Join(dir, "plugins", "installed_plugins.json")); err != nil || json.Unmarshal(b, &installed) != nil {
		return nil
	}
	if b, err := os.ReadFile(filepath.Join(dir, "settings.json")); err == nil {
		json.Unmarshal(b, &settings)
	}
	keys := make([]string, 0, len(installed.Plugins))
	for k := range installed.Plugins {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []Command
	for _, key := range keys {
		installs := installed.Plugins[key]
		if !settings.EnabledPlugins[key] || len(installs) == 0 {
			continue
		}
		plugin, _, _ := strings.Cut(key, "@")
		root := installs[len(installs)-1].InstallPath
		for _, c := range templates(filepath.Join(root, "commands"), "/"+plugin+":", plugin, sendNative, true) {
			c.Name = plugin + ":" + c.Name
			out = append(out, c)
		}
		for _, c := range skillsNamed(filepath.Join(root, "skills"), "/"+plugin+":", plugin, sendNative, true) {
			c.Name = plugin + ":" + c.Name
			out = append(out, c)
		}
	}
	return out
}

// claudeSynced: the skills synced from the Claude account (skills/synced/<bucket>/<name>), as
// /anthropic-skills:<name>.
func claudeSynced(dir string) []Command {
	buckets, _ := os.ReadDir(dir)
	var out []Command
	for _, b := range buckets {
		if !b.IsDir() {
			continue
		}
		for _, c := range skillsNamed(filepath.Join(dir, b.Name()), "/anthropic-skills:", "synced", sendNative, true) {
			c.Name = "anthropic-skills:" + c.Name
			out = append(out, c)
		}
	}
	return out
}

// frontmatter reads the `key: value` lines of a markdown file's leading
// --- block (folded `>` / `|` values joined into one line). ok: the file exists.
func frontmatter(path string) (map[string]string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	meta := map[string]string{}
	head, _, found := cutFrontmatter(string(b))
	if !found {
		return meta, true
	}
	lines := strings.Split(head, "\n")
	for i := 0; i < len(lines); i++ {
		key, value, ok := strings.Cut(lines[i], ":")
		if !ok || key == "" || key[0] == ' ' || key[0] == '\t' {
			continue
		}
		value = strings.TrimSpace(value)
		if value == ">" || value == "|" || value == ">-" || value == "|-" {
			var parts []string
			for i+1 < len(lines) && (strings.HasPrefix(lines[i+1], " ") || strings.HasPrefix(lines[i+1], "\t")) {
				i++
				parts = append(parts, strings.TrimSpace(lines[i]))
			}
			value = strings.Join(parts, " ")
		}
		meta[strings.TrimSpace(key)] = strings.Trim(value, `"'`)
	}
	return meta, true
}

// cutFrontmatter splits "---\n<head>\n---\n<body>".
func cutFrontmatter(s string) (head, body string, found bool) {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return "", s, false
	}
	rest := s[4:]
	if i := strings.Index(rest, "\n---"); i >= 0 {
		body = rest[i+4:]
		if j := strings.IndexByte(body, '\n'); j >= 0 {
			body = body[j+1:]
		} else {
			body = ""
		}
		return rest[:i], body, true
	}
	return "", s, false
}

// expand turns a prompt that starts with a command the CLI doesn't expand
// into what it should receive: the template's text, an instruction to use the
// skill, or (OpenCode, Kilo) the name for --command plus its arguments.
func expand(harness, prompt string, cmds []Command, notes *[]string) (string, string) {
	trimmed := strings.TrimLeft(prompt, " \t\r\n")
	for _, c := range cmds {
		if !strings.HasPrefix(trimmed, c.Insert) {
			continue
		}
		rest := trimmed[len(c.Insert):]
		if rest != "" && !strings.ContainsAny(rest[:1], " \t\r\n") {
			continue
		}
		args := strings.TrimSpace(rest)
		switch c.send {
		case sendTemplate:
			b, err := os.ReadFile(c.Path)
			if err != nil {
				return prompt, ""
			}
			*notes = append(*notes, fmt.Sprintf("%s is expanded from %s", c.Insert, c.Path))
			return fillTemplate(string(b), args), ""
		case sendCommand:
			*notes = append(*notes, fmt.Sprintf("%s is sent as --command %s", c.Insert, c.Name))
			return args, c.Name
		case sendSkill:
			*notes = append(*notes, fmt.Sprintf("%s is sent as an instruction to use the skill", c.Insert))
			text := fmt.Sprintf("Use your %q skill (%s): read it and follow it.", c.Name, c.Path)
			if args != "" {
				text += "\n\n" + args
			}
			return text, ""
		}
		return prompt, ""
	}
	return prompt, ""
}

// fillTemplate puts the arguments into a prompt template: $ARGUMENTS, $1…$9,
// or after the text when it has neither.
func fillTemplate(text, args string) string {
	_, body, _ := cutFrontmatter(text)
	body = strings.TrimSpace(body)
	words := strings.Fields(args)
	used := strings.Contains(body, "$ARGUMENTS")
	body = strings.ReplaceAll(body, "$ARGUMENTS", args)
	for i := 9; i >= 1; i-- {
		ph := fmt.Sprintf("$%d", i)
		if strings.Contains(body, ph) {
			used = true
			v := ""
			if i <= len(words) {
				v = words[i-1]
			}
			body = strings.ReplaceAll(body, ph, v)
		}
	}
	if !used && args != "" {
		body += "\n\n" + args
	}
	return body
}
