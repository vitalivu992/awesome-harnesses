package catalog

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"awesome-harness/internal/conv"
	"awesome-harness/internal/cursor"
	"awesome-harness/internal/opencode/opencodetest"

	_ "modernc.org/sqlite"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sqliteExec(t *testing.T, path string, stmts ...string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
}

const claudeLine = `{"type":"user","cwd":%q,"timestamp":"2026-10-20T10:00:00Z","message":{"role":"user","content":"hello"}}
{"type":"assistant","cwd":%q,"timestamp":"2026-10-20T10:00:05Z","message":{"model":"claude-opus-5-5","content":[{"type":"text","text":"hi"}],"stop_reason":"end_turn"}}
`

const cursorChat = `{"role":"user","message":{"content":[{"type":"text","text":"<user_query>fix it</user_query>"}]}}
{"role":"assistant","message":{"content":[{"type":"text","text":"Fixed."}]}}
{"type":"turn_ended","status":"success"}
`

// A home with one project (my_app) used from both harnesses, a Cursor chat
// whose folder was deleted, and an empty Claude session.
func setup(t *testing.T) (*Catalog, string) {
	t.Helper()
	root := t.TempDir()
	project := filepath.Join(root, "work", "my_app")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	claudeDir := filepath.Join(root, ".claude")
	cursorDir := filepath.Join(root, ".cursor")
	claudeSlug := looseSlug(project)

	write(t, filepath.Join(claudeDir, "projects", "-"+claudeSlug, "c1.jsonl"), fmt.Sprintf(claudeLine, project, project))
	write(t, filepath.Join(claudeDir, "projects", "-"+claudeSlug, "empty.jsonl"), `{"type":"permission-mode","permissionMode":"default"}`+"\n")
	write(t, filepath.Join(cursorDir, "projects", claudeSlug, "agent-transcripts", "k1", "k1.jsonl"), cursorChat)
	write(t, filepath.Join(cursorDir, "projects", claudeSlug, "agent-transcripts", "k2", "k2.jsonl"), cursorChat)
	gone := looseSlug(filepath.Join(root, "work", "old-repo"))
	write(t, filepath.Join(cursorDir, "projects", gone, "agent-transcripts", "k3", "k3.jsonl"), cursorChat)

	// k1 is a cursor-agent CLI chat (resumable, model in its store); k2 an
	// IDE chat whose model comes from the AI code tracking db.
	meta := hex.EncodeToString([]byte(`{"name":"New Agent","createdAt":1792744159805}`))
	sqliteExec(t, cursor.ChatStorePath(filepath.Join(cursorDir, "chats"), project, "k1"),
		`CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT)`,
		`CREATE TABLE blobs (id TEXT PRIMARY KEY, data BLOB)`,
		`INSERT INTO meta VALUES ('0', '`+meta+`')`,
		`INSERT INTO blobs VALUES ('a', CAST('{"modelName":"composer-2"}' AS BLOB))`,
		`INSERT INTO blobs VALUES ('b', CAST('{"modelName":"composer-2.5"}' AS BLOB))`,
	)
	sqliteExec(t, filepath.Join(cursorDir, "ai-tracking", "ai-code-tracking.db"),
		`CREATE TABLE ai_code_hashes (hash TEXT PRIMARY KEY, source TEXT, conversationId TEXT, timestamp INTEGER, model TEXT)`,
		`INSERT INTO ai_code_hashes VALUES ('h1', 'composer', 'k2', 1, 'gpt-5.3-codex')`,
		`INSERT INTO ai_code_hashes VALUES ('h2', 'composer', 'k2', 2, 'claude-4.6-opus-high-thinking')`,
	)

	// Codex (one session in the project, one subagent), pi, OpenCode and Kilo.
	codexDir := filepath.Join(root, ".codex")
	write(t, filepath.Join(codexDir, "sessions", "2026", "10", "20", "rollout-x-cx1.jsonl"), fmt.Sprintf(codexRollout, "cx1", project, `"cli"`))
	write(t, filepath.Join(codexDir, "sessions", "2026", "10", "20", "rollout-x-cx2.jsonl"), fmt.Sprintf(codexRollout, "cx2", project, `{"subagent":{}}`))
	write(t, filepath.Join(codexDir, "session_index.jsonl"), `{"id":"cx1","thread_name":"Renamed in Codex"}`+"\n")
	piDir := filepath.Join(root, ".pi")
	write(t, filepath.Join(piDir, "agent", "sessions", "--x--", "2026_p1.jsonl"), fmt.Sprintf(piSession, project))
	dbs := map[string]string{}
	for _, h := range []string{conv.OpenCode, conv.Kilo} {
		dbs[h] = filepath.Join(root, h, h+".db")
		if err := os.MkdirAll(filepath.Dir(dbs[h]), 0o755); err != nil {
			t.Fatal(err)
		}
		db, err := sql.Open("sqlite", dbs[h])
		if err != nil {
			t.Fatal(err)
		}
		if err := opencodetest.Seed(db, project); err != nil {
			t.Fatal(err)
		}
		db.Close()
	}
	return New(Config{
		ClaudeDir: claudeDir, CursorDir: cursorDir, CodexDir: codexDir, PiDir: piDir,
		OpenCodeDB: dbs[conv.OpenCode], KiloDB: dbs[conv.Kilo], ActiveWindow: time.Minute,
	}), project
}

const codexRollout = `{"timestamp":"2026-10-20T10:00:00Z","type":"session_meta","payload":{"id":%q,"cwd":%q,"source":%s}}
{"timestamp":"2026-10-20T10:00:01Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello codex"}]}}
{"timestamp":"2026-10-20T10:00:02Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hi"}]}}
`

const piSession = `{"type":"session","id":"p1","timestamp":"2026-10-20T10:00:00Z","cwd":%q}
{"type":"message","id":"u1","parentId":null,"timestamp":"2026-10-20T10:00:01Z","message":{"role":"user","content":[{"type":"text","text":"hello pi"}]}}
{"type":"message","id":"a1","parentId":"u1","timestamp":"2026-10-20T10:00:02Z","message":{"role":"assistant","content":[{"type":"text","text":"hi"}],"stopReason":"stop"}}
`

func TestProjectsMergeHarnesses(t *testing.T) {
	c, project := setup(t)
	projects := c.Projects()
	if len(projects) != 2 {
		t.Fatalf("projects = %+v", projects)
	}
	var mine, gone *Project
	for i := range projects {
		if projects[i].Key == project {
			mine = &projects[i]
		} else {
			gone = &projects[i]
		}
	}
	want := map[string]int{"claude": 2, "cursor": 2, "codex": 1, "pi": 1, "opencode": 1, "kilo": 1}
	if mine == nil || mine.Name != "my_app" || !mine.Exists || fmt.Sprint(mine.Counts) != fmt.Sprint(want) {
		t.Fatalf("my_app = %+v, want counts %v (the Codex subagent session is left out)", mine, want)
	}
	if gone == nil || gone.Exists || gone.Name != "old-repo" || gone.Counts["cursor"] != 1 {
		t.Fatalf("deleted project = %+v (guessed from the slug)", gone)
	}
}

func TestConversationsEnrichCursor(t *testing.T) {
	c, project := setup(t)
	sums, err := c.Conversations(project)
	if err != nil {
		t.Fatal(err)
	}
	if len(sums) != 7 {
		t.Fatalf("conversations = %d, want 7 (the empty Claude session is hidden)", len(sums))
	}
	byID := map[string]conv.Summary{}
	for _, s := range sums {
		byID[s.ID] = s
		if s.Cwd != project {
			t.Errorf("%s %s cwd = %q", s.Harness, s.ID, s.Cwd)
		}
		if s.ProjectPath != project {
			t.Errorf("%s projectPath = %q", s.ID, s.ProjectPath)
		}
	}
	if s := byID["c1"]; s.Harness != conv.Claude || s.Cwd != project || !s.Resumable || s.Title != "hello" {
		t.Fatalf("c1 = %+v", s)
	}
	k1 := byID["k1"]
	if !k1.Resumable || len(k1.Models) != 1 || k1.Models[0] != "composer-2.5" || k1.Cwd != project {
		t.Fatalf("k1 = %+v (CLI store: resumable, newest modelName)", k1)
	}
	if k1.StartedAt != time.UnixMilli(1792744159805).UTC().Format(time.RFC3339Nano) {
		t.Fatalf("k1 startedAt = %q (store createdAt)", k1.StartedAt)
	}
	k2 := byID["k2"]
	if k2.Resumable || len(k2.Models) != 1 || k2.Models[0] != "claude-4.6-opus-high-thinking" {
		t.Fatalf("k2 = %+v (tracked model, newest edit wins)", k2)
	}
	if s := byID["cx1"]; s.Harness != conv.Codex || s.Title != "Renamed in Codex" {
		t.Fatalf("cx1 = %+v (title from session_index.jsonl)", s)
	}
	if s := byID["p1"]; s.Harness != conv.Pi || s.Title != "hello pi" {
		t.Fatalf("p1 = %+v", s)
	}
	if s := byID["ses_1"]; s.Title != "Fix the build" || s.Turns != 2 {
		t.Fatalf("ses_1 = %+v", s)
	}
	if _, err := c.Conversations("/nope"); err != ErrNotFound {
		t.Fatalf("unknown project err = %v", err)
	}
}

func TestConversation(t *testing.T) {
	c, project := setup(t)
	full, err := c.Conversation(conv.Cursor, "k2")
	if err != nil {
		t.Fatal(err)
	}
	if full.ProjectPath != project || full.Entries[0].Model != "claude-4.6-opus-high-thinking" || full.Entries[0].Final != "Fixed." {
		t.Fatalf("k2 = %+v", full)
	}
	if _, err := c.Conversation(conv.Claude, "c1"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"../c1", "..", "missing", ""} {
		if _, err := c.Conversation(conv.Claude, id); err != ErrNotFound {
			t.Errorf("Conversation(%q) err = %v", id, err)
		}
	}
	for _, h := range []string{conv.Codex, conv.Pi, conv.OpenCode, conv.Kilo} {
		id := map[string]string{conv.Codex: "cx1", conv.Pi: "p1", conv.OpenCode: "ses_1", conv.Kilo: "ses_1"}[h]
		full, err := c.Conversation(h, id)
		if err != nil || full.Harness != h || full.ProjectPath != project || len(full.Entries) == 0 {
			t.Errorf("Conversation(%s, %s) = %+v, %v", h, id, full, err)
		}
	}
	if _, err := c.Conversation(conv.Codex, "cx2"); err != ErrNotFound {
		t.Errorf("subagent session err = %v", err)
	}
	if _, err := c.Conversation("gemini", "c1"); err != ErrNotFound {
		t.Errorf("unknown harness err = %v", err)
	}
}

func TestSlugs(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "a.b", "my_app")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := resolveSlug("-" + looseSlug(dir)); got != dir {
		t.Fatalf("resolveSlug = %q, want %q", got, dir)
	}
	if got := guessSlug(looseSlug(filepath.Join(root, "a.b", "gone-repo"))); got != filepath.Join(root, "a.b", "gone-repo") {
		t.Fatalf("guessSlug = %q", got)
	}
}
