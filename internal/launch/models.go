package launch

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"awesome-harness/internal/conv"
)

// Model is a model the harness offers, as a suggestion for the settings.
type Model struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	// Efforts: what this model supports; nil: the harness's list.
	Efforts []string `json:"efforts"`
	// Fast: the model has a fast mode (Codex's speed tiers).
	Fast bool `json:"fast,omitempty"`
}

// Runner runs a CLI and returns its standard output.
type Runner func(ctx context.Context, argv ...string) ([]byte, error)

// Claude Code takes these aliases for the latest of each family (or a full id).
var claudeModels = []Model{
	{ID: "fable", Name: "Fable (latest)"},
	{ID: "opus", Name: "Opus (latest)"},
	{ID: "sonnet", Name: "Sonnet (latest)"},
	{ID: "haiku", Name: "Haiku (latest)"},
}

// Models lists the models a harness offers: Claude's aliases, Codex's model
// cache, or what the CLI lists (cursor-agent models, pi --list-models,
// opencode / kilo models --verbose).
func Models(ctx context.Context, harness string, d Dirs, run Runner) ([]Model, error) {
	switch harness {
	case conv.Claude:
		return slices.Clone(claudeModels), nil
	case conv.Codex:
		return codexModels(filepath.Join(d.Codex, "models_cache.json"))
	case conv.Cursor:
		out, err := run(ctx, "cursor-agent", "models")
		if err != nil {
			return nil, err
		}
		return parseCursorModels(out), nil
	case conv.Pi:
		out, err := run(ctx, "pi", "--list-models")
		if err != nil {
			return nil, err
		}
		return parsePiModels(out), nil
	case conv.OpenCode, conv.Kilo:
		out, err := run(ctx, harness, "models", "--verbose")
		if err != nil {
			return nil, err
		}
		return parseOpenCodeModels(out), nil
	}
	return nil, nil
}

func codexModels(path string) ([]Model, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cache struct {
		Models []struct {
			Slug       string `json:"slug"`
			Name       string `json:"display_name"`
			Visibility string `json:"visibility"`
			Levels     []struct {
				Effort string `json:"effort"`
			} `json:"supported_reasoning_levels"`
			Speeds []string `json:"additional_speed_tiers"`
		} `json:"models"`
	}
	if err := json.Unmarshal(b, &cache); err != nil {
		return nil, err
	}
	var out []Model
	for _, m := range cache.Models {
		if m.Slug == "" || (m.Visibility != "" && m.Visibility != "list") {
			continue
		}
		efforts := []string{}
		for _, l := range m.Levels {
			efforts = append(efforts, l.Effort)
		}
		out = append(out, Model{ID: m.Slug, Name: m.Name, Efforts: efforts, Fast: slices.Contains(m.Speeds, "fast")})
	}
	return out, nil
}

// "gpt-5.3-codex-high - Codex 5.3 High" (the "(current)" / "(default)" marks dropped).
var cursorModelRe = regexp.MustCompile(`^(\S+) - (.+?)(?:\s+\((?:current|default)\))*$`)

func parseCursorModels(out []byte) []Model {
	var models []Model
	for _, line := range strings.Split(string(out), "\n") {
		m := cursorModelRe.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		// Cursor's names carry the effort and speed; no separate setting.
		models = append(models, Model{ID: m[1], Name: strings.Trim(m[2], "​ "), Efforts: []string{}})
	}
	return models
}

// pi --list-models: a table "provider model context max-out thinking images".
func parsePiModels(out []byte) []Model {
	var models []Model
	sc := bufio.NewScanner(bytes.NewReader(out))
	header := true
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if header {
			header = len(f) == 0 || f[0] != "provider"
			continue
		}
		if len(f) < 5 {
			continue
		}
		m := Model{ID: f[0] + "/" + f[1]}
		if f[4] != "yes" {
			m.Efforts = []string{}
		}
		models = append(models, m)
	}
	return models
}

// opencode models --verbose: "provider/model" then the model as indented JSON, repeated.
func parseOpenCodeModels(out []byte) []Model {
	var models []Model
	rest := out
	for len(rest) > 0 {
		line, after, _ := bytes.Cut(rest, []byte("\n"))
		id := strings.TrimSpace(string(line))
		rest = after
		if id == "" || strings.HasPrefix(id, "{") || !strings.Contains(id, "/") {
			continue
		}
		var info struct {
			Name     string                     `json:"name"`
			Variants map[string]json.RawMessage `json:"variants"`
		}
		dec := json.NewDecoder(bytes.NewReader(rest))
		if bytes.HasPrefix(bytes.TrimSpace(rest), []byte("{")) && dec.Decode(&info) == nil {
			rest = rest[dec.InputOffset():]
		}
		m := Model{ID: id, Name: info.Name}
		if info.Variants != nil {
			m.Efforts = sortEfforts(info.Variants)
		}
		models = append(models, m)
	}
	return models
}

var effortOrder = []string{"off", "none", "minimal", "low", "medium", "high", "xhigh", "max", "ultra"}

func sortEfforts[V any](set map[string]V) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	rank := func(s string) int {
		if i := slices.Index(effortOrder, s); i >= 0 {
			return i
		}
		return len(effortOrder)
	}
	sort.Slice(out, func(i, j int) bool {
		if ri, rj := rank(out[i]), rank(out[j]); ri != rj {
			return ri < rj
		}
		return out[i] < out[j]
	})
	return out
}
