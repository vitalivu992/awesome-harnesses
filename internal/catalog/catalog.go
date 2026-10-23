// Package catalog finds the Claude Code and Cursor transcripts on disk,
// groups them by project (the directory the agent ran in, so one project
// lists both harnesses' conversations) and caches parsed summaries by file
// size + modification time. Everything is read-only.
package catalog

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"awesome-harness/internal/claude"
	"awesome-harness/internal/conv"
	"awesome-harness/internal/cursor"
)

// Config says where the harnesses keep their data.
type Config struct {
	ClaudeDir string // ~/.claude
	CursorDir string // ~/.cursor
	// ActiveWindow: a conversation whose file changed this recently is marked active.
	ActiveWindow time.Duration
}

// DefaultConfig uses the harnesses' standard locations under $HOME.
func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		ClaudeDir:    filepath.Join(home, ".claude"),
		CursorDir:    filepath.Join(home, ".cursor"),
		ActiveWindow: 2 * time.Minute,
	}
}

// Project is one working directory with conversations in either harness.
type Project struct {
	// Key identifies the project in URLs: its path, or "slug:<folder>" when
	// the folder's path couldn't be recovered (deleted project).
	Key       string `json:"key"`
	Path      string `json:"path"`
	Name      string `json:"name"`
	Exists    bool   `json:"exists"`
	Claude    int    `json:"claude"`
	Cursor    int    `json:"cursor"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// ErrNotFound is returned for an unknown project or conversation.
var ErrNotFound = errors.New("not found")

// NoWorkspaceKey is the project of Cursor chats opened without a folder.
const NoWorkspaceKey = "cursor:no-workspace"

type source struct {
	harness string
	dir     string // the folder holding the transcripts
}

type file struct {
	harness string
	path    string
	size    int64
	mtime   time.Time
}

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
	summaries map[string]cached                    // transcript path → summary
	dirPaths  map[string]string                    // transcript folder → project path ("" = unresolved)
	chatInfo  map[string]stamped[*cursor.ChatInfo] // store.db path → info (nil = none)
	tracked   stamped[map[string]string]           // Cursor conversation id → model
	now       func() time.Time
}

// New returns a catalog over cfg's directories.
func New(cfg Config) *Catalog {
	if cfg.ActiveWindow == 0 {
		cfg.ActiveWindow = 2 * time.Minute
	}
	return &Catalog{
		cfg:       cfg,
		summaries: map[string]cached{},
		dirPaths:  map[string]string{},
		chatInfo:  map[string]stamped[*cursor.ChatInfo]{},
		now:       time.Now,
	}
}

func (c *Catalog) claudeProjects() string { return filepath.Join(c.cfg.ClaudeDir, "projects") }
func (c *Catalog) cursorProjects() string { return filepath.Join(c.cfg.CursorDir, "projects") }

// sources lists every transcript folder with its project key.
func (c *Catalog) sources() map[string][]source {
	keys := map[string][]source{}
	claudePaths := map[string]string{} // loose slug → path, to place unresolvable Cursor folders
	if entries, err := os.ReadDir(c.claudeProjects()); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(c.claudeProjects(), e.Name())
			files := listFiles(conv.Claude, dir)
			if len(files) == 0 {
				continue
			}
			path := c.dirPath(dir, func() string {
				sort.Slice(files, func(i, j int) bool { return files[i].mtime.After(files[j].mtime) })
				for _, f := range files {
					if cwd := claude.ReadCwd(f.path); cwd != "" {
						return cwd
					}
				}
				return resolveSlug(e.Name())
			})
			key := keyFor(path, e.Name())
			if path != "" {
				claudePaths[looseSlug(path)] = path
			}
			keys[key] = append(keys[key], source{conv.Claude, dir})
		}
	}
	if entries, err := os.ReadDir(c.cursorProjects()); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if e.Name() == "agent-transcripts" {
				dir := filepath.Join(c.cursorProjects(), e.Name())
				if len(listFiles(conv.Cursor, dir)) > 0 {
					keys[NoWorkspaceKey] = append(keys[NoWorkspaceKey], source{conv.Cursor, dir})
				}
				continue
			}
			dir := filepath.Join(c.cursorProjects(), e.Name(), "agent-transcripts")
			if len(listFiles(conv.Cursor, dir)) == 0 {
				continue
			}
			path := c.dirPath(dir, func() string {
				if p := resolveSlug(e.Name()); p != "" {
					return p
				}
				if p := claudePaths[looseSlug(e.Name())]; p != "" {
					return p
				}
				return guessSlug(e.Name())
			})
			key := keyFor(path, e.Name())
			keys[key] = append(keys[key], source{conv.Cursor, dir})
		}
	}
	return keys
}

func keyFor(path, folder string) string {
	if path != "" {
		return path
	}
	return "slug:" + folder
}

// dirPath memoizes a transcript folder's project path.
func (c *Catalog) dirPath(dir string, resolve func() string) string {
	c.mu.Lock()
	path, ok := c.dirPaths[dir]
	c.mu.Unlock()
	if ok {
		return path
	}
	path = resolve()
	c.mu.Lock()
	c.dirPaths[dir] = path
	c.mu.Unlock()
	return path
}

// listFiles lists a folder's transcripts: <dir>/<id>.jsonl for Claude Code,
// <dir>/<id>/<id>.jsonl for Cursor (subagent transcripts are left out).
func listFiles(harness, dir string) []file {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []file
	for _, e := range entries {
		var path string
		switch harness {
		case conv.Claude:
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
				continue
			}
			path = filepath.Join(dir, e.Name())
		case conv.Cursor:
			if !e.IsDir() {
				continue
			}
			path = filepath.Join(dir, e.Name(), e.Name()+".jsonl")
		}
		st, err := os.Stat(path)
		if err != nil || st.Size() == 0 {
			continue
		}
		files = append(files, file{harness, path, st.Size(), st.ModTime()})
	}
	return files
}

// Projects lists every project, most recently active first.
func (c *Catalog) Projects() []Project {
	var projects []Project
	for key, srcs := range c.sources() {
		p := Project{Key: key}
		switch {
		case key == NoWorkspaceKey:
			p.Name = "No workspace"
		case strings.HasPrefix(key, "slug:"):
			p.Name = strings.TrimPrefix(key, "slug:")
		default:
			p.Path = key
			p.Name = filepath.Base(key)
			if st, err := os.Stat(key); err == nil && st.IsDir() {
				p.Exists = true
			}
		}
		var latest time.Time
		for _, s := range srcs {
			files := listFiles(s.harness, s.dir)
			if s.harness == conv.Claude {
				p.Claude += len(files)
			} else {
				p.Cursor += len(files)
			}
			for _, f := range files {
				if f.mtime.After(latest) {
					latest = f.mtime
				}
			}
		}
		if !latest.IsZero() {
			p.UpdatedAt = latest.UTC().Format(time.RFC3339)
		}
		projects = append(projects, p)
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
	srcs, ok := c.sources()[key]
	if !ok {
		return nil, ErrNotFound
	}
	var files []file
	for _, s := range srcs {
		files = append(files, listFiles(s.harness, s.dir)...)
	}
	out := make([]conv.Summary, len(files))
	ok2 := make([]bool, len(files))
	var wg sync.WaitGroup
	jobs := make(chan int)
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				if sum, err := c.summary(files[i]); err == nil {
					out[i], ok2[i] = sum, true
				}
			}
		}()
	}
	for i := range files {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	sums := make([]conv.Summary, 0, len(files))
	for i, s := range out {
		// Sessions with no prompt (only a /clear, or opened and closed) are noise.
		if !ok2[i] || s.Turns == 0 {
			continue
		}
		s.ProjectPath = projectPathOf(key)
		c.enrich(&s, nil, files[i])
		sums = append(sums, s)
	}
	sort.Slice(sums, func(i, j int) bool { return sums[i].UpdatedAt > sums[j].UpdatedAt })
	return sums, nil
}

func projectPathOf(key string) string {
	if key == NoWorkspaceKey || strings.HasPrefix(key, "slug:") {
		return ""
	}
	return key
}

func (c *Catalog) summary(f file) (conv.Summary, error) {
	c.mu.Lock()
	hit, ok := c.summaries[f.path]
	c.mu.Unlock()
	if ok && hit.size == f.size && hit.mtime.Equal(f.mtime) {
		return hit.sum, nil
	}
	full, err := parse(f)
	if err != nil {
		return conv.Summary{}, err
	}
	c.mu.Lock()
	c.summaries[f.path] = cached{f.size, f.mtime, full.Summary}
	c.mu.Unlock()
	return full.Summary, nil
}

func parse(f file) (*conv.Conversation, error) {
	if f.harness == conv.Claude {
		return claude.ParseFile(f.path)
	}
	return cursor.ParseFile(f.path)
}

var idRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

// Conversation reads one transcript in full.
func (c *Catalog) Conversation(harness, id string) (*conv.Conversation, error) {
	if !idRe.MatchString(id) || strings.Contains(id, "..") {
		return nil, ErrNotFound
	}
	var pattern string
	switch harness {
	case conv.Claude:
		pattern = filepath.Join(c.claudeProjects(), "*", id+".jsonl")
	case conv.Cursor:
		pattern = filepath.Join(c.cursorProjects(), "*", "agent-transcripts", id, id+".jsonl")
	default:
		return nil, ErrNotFound
	}
	matches, _ := filepath.Glob(pattern)
	if harness == conv.Cursor {
		more, _ := filepath.Glob(filepath.Join(c.cursorProjects(), "agent-transcripts", id, id+".jsonl"))
		matches = append(matches, more...)
	}
	if len(matches) == 0 {
		return nil, ErrNotFound
	}
	path := matches[0]
	st, err := os.Stat(path)
	if err != nil {
		return nil, ErrNotFound
	}
	f := file{harness, path, st.Size(), st.ModTime()}
	full, err := parse(f)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(path)
	if harness == conv.Cursor {
		dir = filepath.Dir(dir)
	}
	for key, srcs := range c.sources() {
		for _, s := range srcs {
			if s.dir == dir {
				full.ProjectPath = projectPathOf(key)
			}
		}
	}
	c.enrich(&full.Summary, full, f)
	return full, nil
}

// enrich adds what isn't in the transcript itself: the active flag, and for
// Cursor the workspace, the CLI chat store (resumability, creation time,
// model) and the model from the AI code tracking database.
func (c *Catalog) enrich(s *conv.Summary, full *conv.Conversation, f file) {
	s.Active = c.now().Sub(f.mtime) < c.cfg.ActiveWindow
	if s.Harness != conv.Cursor {
		return
	}
	s.Cwd = s.ProjectPath
	model := ""
	if s.ProjectPath != "" {
		if info := c.readChatInfo(cursor.ChatStorePath(filepath.Join(c.cfg.CursorDir, "chats"), s.ProjectPath, s.ID)); info != nil {
			s.Resumable = true
			model = info.Model
			if !info.CreatedAt.IsZero() && (s.StartedAt == "" || info.CreatedAt.UTC().Format(time.RFC3339Nano) < s.StartedAt) {
				s.StartedAt = info.CreatedAt.UTC().Format(time.RFC3339Nano)
			}
		}
	}
	if model == "" {
		model = c.trackedModel(s.ID)
	}
	if model == "" || len(s.Models) > 0 {
		return
	}
	s.Models = []string{model}
	if full != nil {
		full.Models = s.Models
		for i := range full.Entries {
			if full.Entries[i].Kind == conv.EntryTurn && full.Entries[i].Model == "" {
				full.Entries[i].Model = model
			}
		}
	}
}

func (c *Catalog) readChatInfo(path string) *cursor.ChatInfo {
	st, err := os.Stat(path)
	if err != nil {
		return nil
	}
	c.mu.Lock()
	hit, ok := c.chatInfo[path]
	c.mu.Unlock()
	if ok && hit.mtime.Equal(st.ModTime()) {
		return hit.value
	}
	info, err := cursor.ReadChatInfo(path)
	if err != nil {
		info = nil
	}
	c.mu.Lock()
	c.chatInfo[path] = stamped[*cursor.ChatInfo]{st.ModTime(), info}
	c.mu.Unlock()
	return info
}

func (c *Catalog) trackedModel(id string) string {
	path := filepath.Join(c.cfg.CursorDir, "ai-tracking", "ai-code-tracking.db")
	st, err := os.Stat(path)
	if err != nil {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tracked.value == nil || !c.tracked.mtime.Equal(st.ModTime()) {
		models, err := cursor.ReadTrackedModels(path)
		if err != nil {
			models = map[string]string{}
		}
		c.tracked = stamped[map[string]string]{st.ModTime(), models}
	}
	return c.tracked.value[id]
}
