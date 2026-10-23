// Package catalog finds the conversations of every supported harness (Claude
// Code, Cursor, Codex, pi, OpenCode, Kilo), groups them by project (the
// directory the agent ran in, so one project lists every harness's
// conversations) and caches parsed summaries by file size + modification time
// (update time for database-backed harnesses). Everything is read-only.
package catalog

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"awesome-harness/internal/conv"
	"awesome-harness/internal/cursor"
)

// Config says where the harnesses keep their data. An empty or missing
// location just means that harness has no conversations.
type Config struct {
	ClaudeDir string // ~/.claude
	CursorDir string // ~/.cursor
	// CursorLogsDir: the Cursor server's logs (~/.cursor-server/data/logs), which
	// hold the model and turn times of Cursor app chats.
	CursorLogsDir string
	CodexDir      string // ~/.codex ($CODEX_HOME)
	PiDir         string // ~/.pi
	OpenCodeDB    string // ~/.local/share/opencode/opencode.db
	KiloDB        string // ~/.local/share/kilo/kilo.db
	// ActiveWindow: a conversation updated this recently is marked active.
	ActiveWindow time.Duration
}

// DefaultConfig uses the harnesses' standard locations.
func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	codex := os.Getenv("CODEX_HOME")
	if codex == "" {
		codex = filepath.Join(home, ".codex")
	}
	data := os.Getenv("XDG_DATA_HOME")
	if data == "" {
		data = filepath.Join(home, ".local", "share")
	}
	return Config{
		ClaudeDir:     filepath.Join(home, ".claude"),
		CursorDir:     filepath.Join(home, ".cursor"),
		CursorLogsDir: filepath.Join(home, ".cursor-server", "data", "logs"),
		CodexDir:      codex,
		PiDir:         filepath.Join(home, ".pi"),
		OpenCodeDB:    filepath.Join(data, "opencode", "opencode.db"),
		KiloDB:        filepath.Join(data, "kilo", "kilo.db"),
		ActiveWindow:  2 * time.Minute,
	}
}

// Project is one working directory with conversations in any harness.
type Project struct {
	// Key identifies the project in URLs: its path, or NoWorkspaceKey.
	Key    string `json:"key"`
	Path   string `json:"path"`
	Name   string `json:"name"`
	Exists bool   `json:"exists"`
	// Counts: conversations per harness id.
	Counts    map[string]int `json:"counts"`
	UpdatedAt string         `json:"updatedAt,omitempty"`
}

// ErrNotFound is returned for an unknown project or conversation.
var ErrNotFound = errors.New("not found")

// NoWorkspaceKey is the project of conversations started without a folder.
const NoWorkspaceKey = "no-workspace"

// item is one conversation as found on disk, before parsing.
type item struct {
	harness string
	id      string
	path    string // the transcript file, or the database (OpenCode, Kilo)
	key     string // project key
	size    int64
	mtime   time.Time
}

func (it item) cacheKey() string { return it.harness + "\x00" + it.path + "\x00" + it.id }

type cached struct {
	size  int64
	mtime time.Time
	sum   conv.Summary
}

type stamped[T any] struct {
	mtime time.Time
	value T
}

// Catalog is safe for concurrent use.
type Catalog struct {
	cfg Config

	mu        sync.Mutex
	summaries map[string]cached                    // item.cacheKey() → summary
	dirPaths  map[string]string                    // transcript folder → project path
	heads     map[string]header                    // Codex / pi file → id + cwd from its first line
	dbs       map[string]*sql.DB                   // OpenCode / Kilo database path → handle
	chatInfo  map[string]stamped[*cursor.ChatInfo] // Cursor store.db path → info (nil = none)
	tracked   stamped[map[string]string]           // Cursor conversation id → model
	logTurns  stamped[map[string][]cursor.LogTurn] // Cursor conversation id → logged app turns
	logsSig   string                               // Cursor log files' count + sizes, with logTurns.mtime
	codexIdx  stamped[map[string]string]           // Codex session id → thread name
	now       func() time.Time
}

// New returns a catalog over cfg's locations.
func New(cfg Config) *Catalog {
	if cfg.ActiveWindow == 0 {
		cfg.ActiveWindow = 2 * time.Minute
	}
	return &Catalog{
		cfg:       cfg,
		summaries: map[string]cached{},
		dirPaths:  map[string]string{},
		heads:     map[string]header{},
		dbs:       map[string]*sql.DB{},
		chatInfo:  map[string]stamped[*cursor.ChatInfo]{},
		now:       time.Now,
	}
}

// items lists every conversation of every harness.
func (c *Catalog) items() []item {
	claudePaths := map[string]string{} // loose slug → path, to place unresolvable Cursor folders
	var out []item
	out = append(out, c.claudeItems(claudePaths)...)
	out = append(out, c.cursorItems(claudePaths)...)
	out = append(out, c.codexItems()...)
	out = append(out, c.piItems()...)
	out = append(out, c.dbItems(conv.OpenCode, c.cfg.OpenCodeDB)...)
	out = append(out, c.dbItems(conv.Kilo, c.cfg.KiloDB)...)
	return out
}

func keyForPath(path string) string {
	if path == "" {
		return NoWorkspaceKey
	}
	return filepath.Clean(path)
}

func projectPathOf(key string) string {
	if key == NoWorkspaceKey {
		return ""
	}
	return key
}

// Projects lists every project, most recently active first.
func (c *Catalog) Projects() []Project {
	byKey := map[string]*Project{}
	latest := map[string]time.Time{}
	for _, it := range c.items() {
		p := byKey[it.key]
		if p == nil {
			p = &Project{Key: it.key, Counts: map[string]int{}}
			if it.key == NoWorkspaceKey {
				p.Name = "No workspace"
			} else {
				p.Path = it.key
				p.Name = filepath.Base(it.key)
				if st, err := os.Stat(it.key); err == nil && st.IsDir() {
					p.Exists = true
				}
			}
			byKey[it.key] = p
		}
		p.Counts[it.harness]++
		if it.mtime.After(latest[it.key]) {
			latest[it.key] = it.mtime
		}
	}
	projects := make([]Project, 0, len(byKey))
	for key, p := range byKey {
		if t := latest[key]; !t.IsZero() {
			p.UpdatedAt = t.UTC().Format(time.RFC3339)
		}
		projects = append(projects, *p)
	}
	sort.Slice(projects, func(i, j int) bool {
		if projects[i].UpdatedAt != projects[j].UpdatedAt {
			return projects[i].UpdatedAt > projects[j].UpdatedAt
		}
		return projects[i].Key < projects[j].Key
	})
	return projects
}

// Conversations lists a project's conversations, most recently updated first.
func (c *Catalog) Conversations(key string) ([]conv.Summary, error) {
	var items []item
	for _, it := range c.items() {
		if it.key == key {
			items = append(items, it)
		}
	}
	if len(items) == 0 {
		return nil, ErrNotFound
	}
	out := make([]conv.Summary, len(items))
	ok := make([]bool, len(items))
	var wg sync.WaitGroup
	jobs := make(chan int)
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if sum, err := c.summary(items[i]); err == nil {
					out[i], ok[i] = sum, true
				}
			}
		}()
	}
	for i := range items {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	sums := make([]conv.Summary, 0, len(items))
	for i, s := range out {
		// Sessions with no prompt (only a /clear, or opened and closed) are noise.
		if !ok[i] || s.Turns == 0 {
			continue
		}
		s.ProjectPath = projectPathOf(key)
		c.enrich(&s, nil, items[i])
		sums = append(sums, s)
	}
	sort.Slice(sums, func(i, j int) bool { return sums[i].UpdatedAt > sums[j].UpdatedAt })
	return sums, nil
}

func (c *Catalog) summary(it item) (conv.Summary, error) {
	c.mu.Lock()
	hit, ok := c.summaries[it.cacheKey()]
	c.mu.Unlock()
	if ok && hit.size == it.size && hit.mtime.Equal(it.mtime) {
		return hit.sum, nil
	}
	full, err := c.parse(it)
	if err != nil {
		return conv.Summary{}, err
	}
	c.mu.Lock()
	c.summaries[it.cacheKey()] = cached{it.size, it.mtime, full.Summary}
	c.mu.Unlock()
	return full.Summary, nil
}

var idRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

// Conversation reads one conversation in full.
func (c *Catalog) Conversation(harness, id string) (*conv.Conversation, error) {
	if !idRe.MatchString(id) || strings.Contains(id, "..") {
		return nil, ErrNotFound
	}
	for _, it := range c.items() {
		if it.harness != harness || it.id != id {
			continue
		}
		full, err := c.parse(it)
		if err != nil {
			return nil, err
		}
		full.ProjectPath = projectPathOf(it.key)
		c.enrich(&full.Summary, full, it)
		return full, nil
	}
	return nil, ErrNotFound
}

// enrich adds what isn't in the conversation itself: the active flag, the
// project directory as cwd when the harness doesn't record one, and the
// harness-specific extras (Cursor model / resumability, Codex thread names).
func (c *Catalog) enrich(s *conv.Summary, full *conv.Conversation, it item) {
	s.Active = c.now().Sub(it.mtime) < c.cfg.ActiveWindow
	if s.Cwd == "" {
		s.Cwd = s.ProjectPath
	}
	switch s.Harness {
	case conv.Cursor:
		c.enrichCursor(s, full)
	case conv.Codex:
		if name := c.codexName(s.ID); name != "" {
			s.Title = name
		}
	}
}
