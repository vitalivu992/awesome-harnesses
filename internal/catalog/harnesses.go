package catalog

import (
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"awesome-harness/internal/claude"
	"awesome-harness/internal/codex"
	"awesome-harness/internal/conv"
	"awesome-harness/internal/cursor"
	"awesome-harness/internal/opencode"
	"awesome-harness/internal/pi"
)

// header is what a Codex or pi file's first line says: its id and cwd.
type header struct {
	id   string
	cwd  string
	skip bool // not a listable conversation (subagent, unreadable)
}

func statFile(path string) (os.FileInfo, bool) {
	st, err := os.Stat(path)
	if err != nil || st.Size() == 0 {
		return nil, false
	}
	return st, true
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

// headerOf memoizes a file's first-line header (it never changes).
func (c *Catalog) headerOf(path string, read func() header) header {
	c.mu.Lock()
	h, ok := c.heads[path]
	c.mu.Unlock()
	if ok {
		return h
	}
	h = read()
	c.mu.Lock()
	c.heads[path] = h
	c.mu.Unlock()
	return h
}

// Claude Code: ~/.claude/projects/<slug>/<session>.jsonl; the project path is
// the cwd its transcripts record.
func (c *Catalog) claudeItems(claudePaths map[string]string) []item {
	if c.cfg.ClaudeDir == "" {
		return nil
	}
	root := filepath.Join(c.cfg.ClaudeDir, "projects")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []item
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		files, _ := os.ReadDir(dir)
		var items []item
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			path := filepath.Join(dir, f.Name())
			if st, ok := statFile(path); ok {
				items = append(items, item{harness: conv.Claude, id: strings.TrimSuffix(f.Name(), ".jsonl"), path: path, size: st.Size(), mtime: st.ModTime()})
			}
		}
		if len(items) == 0 {
			continue
		}
		path := c.dirPath(dir, func() string {
			sort.Slice(items, func(i, j int) bool { return items[i].mtime.After(items[j].mtime) })
			for _, it := range items {
				if cwd := claude.ReadCwd(it.path); cwd != "" {
					return cwd
				}
			}
			if p := resolveSlug(e.Name()); p != "" {
				return p
			}
			return guessSlug(e.Name())
		})
		claudePaths[looseSlug(path)] = path
		for i := range items {
			items[i].key = keyForPath(path)
		}
		out = append(out, items...)
	}
	return out
}

// Cursor: ~/.cursor/projects/<slug>/agent-transcripts/<id>/<id>.jsonl (and
// ~/.cursor/projects/agent-transcripts/… for chats without a workspace); the
// project path is recovered from the slug.
func (c *Catalog) cursorItems(claudePaths map[string]string) []item {
	if c.cfg.CursorDir == "" {
		return nil
	}
	root := filepath.Join(c.cfg.CursorDir, "projects")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []item
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name(), "agent-transcripts")
		key := ""
		if e.Name() == "agent-transcripts" {
			dir, key = filepath.Join(root, e.Name()), NoWorkspaceKey
		}
		chats, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		var items []item
		for _, chat := range chats {
			if !chat.IsDir() {
				continue
			}
			path := filepath.Join(dir, chat.Name(), chat.Name()+".jsonl")
			if st, ok := statFile(path); ok {
				items = append(items, item{harness: conv.Cursor, id: chat.Name(), path: path, size: st.Size(), mtime: st.ModTime()})
			}
		}
		if len(items) == 0 {
			continue
		}
		if key == "" {
			key = keyForPath(c.dirPath(dir, func() string {
				if p := resolveSlug(e.Name()); p != "" {
					return p
				}
				if p := claudePaths[looseSlug(e.Name())]; p != "" {
					return p
				}
				return guessSlug(e.Name())
			}))
		}
		for i := range items {
			items[i].key = key
		}
		out = append(out, items...)
	}
	return out
}

// Codex: ~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl, any project; the
// session_meta line names the id and cwd.
func (c *Catalog) codexItems() []item {
	if c.cfg.CodexDir == "" {
		return nil
	}
	root := filepath.Join(c.cfg.CodexDir, "sessions")
	var out []item
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		st, ok := statFile(path)
		if !ok {
			return nil
		}
		h := c.headerOf(path, func() header {
			m, err := codex.ReadMeta(path)
			if err != nil {
				return header{skip: true}
			}
			return header{id: m.ID, cwd: m.Cwd, skip: m.Subagent() || m.ID == ""}
		})
		if !h.skip {
			out = append(out, item{harness: conv.Codex, id: h.id, path: path, key: keyForPath(h.cwd), size: st.Size(), mtime: st.ModTime()})
		}
		return nil
	})
	return out
}

// pi: ~/.pi/agent/sessions/--<slug>--/<time>_<id>.jsonl; the header line names
// the id and cwd.
func (c *Catalog) piItems() []item {
	if c.cfg.PiDir == "" {
		return nil
	}
	root := filepath.Join(c.cfg.PiDir, "agent", "sessions")
	dirs, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []item
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		files, _ := os.ReadDir(filepath.Join(root, d.Name()))
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			path := filepath.Join(root, d.Name(), f.Name())
			st, ok := statFile(path)
			if !ok {
				continue
			}
			h := c.headerOf(path, func() header {
				hd, err := pi.ReadHeader(path)
				if err != nil {
					return header{skip: true}
				}
				return header{id: hd.ID, cwd: hd.Cwd}
			})
			if !h.skip {
				out = append(out, item{harness: conv.Pi, id: h.id, path: path, key: keyForPath(h.cwd), size: st.Size(), mtime: st.ModTime()})
			}
		}
	}
	return out
}

// db returns a shared read-only handle on an OpenCode/Kilo database, or nil
// when it doesn't exist.
func (c *Catalog) db(path string) *sql.DB {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if db, ok := c.dbs[path]; ok {
		return db
	}
	db, err := opencode.Open(path)
	if err != nil {
		return nil
	}
	c.dbs[path] = db
	return db
}

// OpenCode and Kilo: one row per session in their SQLite database.
func (c *Catalog) dbItems(harness, path string) []item {
	db := c.db(path)
	if db == nil {
		return nil
	}
	sessions, err := opencode.List(db)
	if err != nil {
		return nil
	}
	out := make([]item, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, item{harness: harness, id: s.ID, path: path, key: keyForPath(s.Directory), mtime: s.Updated})
	}
	return out
}

func (c *Catalog) parse(it item) (*conv.Conversation, error) {
	var full *conv.Conversation
	var err error
	switch it.harness {
	case conv.Claude:
		full, err = claude.ParseFile(it.path)
	case conv.Cursor:
		full, err = cursor.ParseFile(it.path)
	case conv.Codex:
		full, err = codex.ParseFile(it.path)
	case conv.Pi:
		full, err = pi.ParseFile(it.path)
	case conv.OpenCode, conv.Kilo:
		db := c.db(it.path)
		if db == nil {
			return nil, ErrNotFound
		}
		full, err = opencode.Read(db, it.harness, it.id)
	default:
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	full.ID = it.id
	return full, nil
}

// enrichCursor adds what Cursor transcripts lack: the CLI chat store
// (resumability, creation time, model), the app's per-turn models and times
// from the Cursor server logs, and the model from the AI code tracking
// database.
func (c *Catalog) enrichCursor(s *conv.Summary, full *conv.Conversation) {
	turns := c.cursorLogTurns(s.ID)
	if full != nil {
		cursor.AnnotateTurns(full.Entries, turns)
	}
	if len(s.Models) == 0 {
		s.Models = cursor.LogModels(turns)
	}
	if len(turns) > 0 {
		if first := turns[0].Start.UTC().Format(time.RFC3339Nano); s.StartedAt == "" || first < s.StartedAt {
			s.StartedAt = first
		}
	}
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
	if model == "" && len(s.Models) > 0 {
		model = s.Models[len(s.Models)-1]
	}
	if model == "" {
		return
	}
	if len(s.Models) == 0 {
		s.Models = []string{model}
	}
	if full != nil {
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

// cursorLogTurns reads the Cursor server logs again when a log file was
// added, removed or grew.
func (c *Catalog) cursorLogTurns(id string) []cursor.LogTurn {
	if c.cfg.CursorLogsDir == "" {
		return nil
	}
	files := cursor.LogFiles(c.cfg.CursorLogsDir)
	var newest time.Time
	var size int64
	for _, f := range files {
		if st, err := os.Stat(f); err == nil {
			size += st.Size()
			if st.ModTime().After(newest) {
				newest = st.ModTime()
			}
		}
	}
	sig := fmt.Sprintf("%d/%d", len(files), size)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.logTurns.value == nil || c.logsSig != sig || !c.logTurns.mtime.Equal(newest) {
		c.logTurns = stamped[map[string][]cursor.LogTurn]{newest, cursor.ReadLogTurns(files)}
		c.logsSig = sig
	}
	return c.logTurns.value[id]
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

// codexName is the session's thread name from ~/.codex/session_index.jsonl.
func (c *Catalog) codexName(id string) string {
	path := filepath.Join(c.cfg.CodexDir, "session_index.jsonl")
	st, err := os.Stat(path)
	if err != nil {
		return ""
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.codexIdx.value == nil || !c.codexIdx.mtime.Equal(st.ModTime()) {
		names := codex.ReadIndex(path)
		if names == nil {
			names = map[string]string{}
		}
		c.codexIdx = stamped[map[string]string]{st.ModTime(), names}
	}
	return c.codexIdx.value[id]
}
