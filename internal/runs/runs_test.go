//go:build !windows

package runs

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"awesome-harness/internal/catalog"
	"awesome-harness/internal/launch"
)

// fakeCLI writes an executable shell script named `name` into dir.
func fakeCLI(t *testing.T, dir, name, script string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
}

// newManager: CLIs from a temp bin folder, a catalog over a temp Claude dir
// with one conversation (claude:conv-1) in `work`.
func newManager(t *testing.T) (*Manager, string, string) {
	t.Helper()
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	work := filepath.Join(root, "work")
	claudeDir := filepath.Join(root, "claude")
	for _, d := range []string{bin, work, filepath.Join(claudeDir, "projects", "-work")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	line := `{"type":"user","cwd":"` + work + `","timestamp":"2026-10-20T10:00:00Z","message":{"role":"user","content":"hi"}}` + "\n"
	if err := os.WriteFile(filepath.Join(claudeDir, "projects", "-work", "conv-1.jsonl"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := catalog.Config{ClaudeDir: claudeDir, ActiveWindow: time.Minute}
	m := &Manager{Catalog: catalog.New(cfg), Dirs: launch.Dirs{Claude: claudeDir}, Path: bin + ":/usr/bin:/bin"}
	return m, bin, work
}

func wait(t *testing.T, m *Manager, id string) Info {
	t.Helper()
	m.mu.Lock()
	ru := m.runs[id]
	m.mu.Unlock()
	select {
	case <-ru.done:
	case <-time.After(10 * time.Second):
		t.Fatal("the run didn't end")
	}
	return ru.snapshot()
}

func TestStartNewClaude(t *testing.T) {
	m, bin, work := newManager(t)
	// The fake writes its argv and folder, so the test sees what ran where.
	fakeCLI(t, bin, "claude", `pwd > "$0.cwd"; printf '%s\n' "$@" > "$0.args"`)
	info, err := m.Start(launch.Request{Harness: "claude", Cwd: work, Prompt: "hello there", Model: "opus"})
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f-]{36}$`).MatchString(info.Session) || !info.Running {
		t.Fatalf("started: %+v", info)
	}
	end := wait(t, m, info.ID)
	if end.Running || end.ExitCode != 0 || end.Error != "" || end.EndedAt == nil {
		t.Errorf("ended: %+v", end)
	}
	args, _ := os.ReadFile(filepath.Join(bin, "claude.args"))
	want := "-p\n--session-id\n" + info.Session + "\n--model\nopus\n--\nhello there\n"
	if string(args) != want {
		t.Errorf("args %q, want %q", args, want)
	}
	if cwd, _ := os.ReadFile(filepath.Join(bin, "claude.cwd")); strings.TrimSpace(string(cwd)) != work {
		t.Errorf("ran in %q", cwd)
	}
}

func TestSessionFromJSONEvents(t *testing.T) {
	m, bin, work := newManager(t)
	fakeCLI(t, bin, "codex", `echo 'not json'; echo '{"type":"thread.started","thread_id":"t-9"}'; echo '{"type":"turn.completed"}'`)
	info, err := m.Start(launch.Request{Harness: "codex", Cwd: work, Prompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if info.Session != "" {
		t.Errorf("session known before the CLI said: %q", info.Session)
	}
	if end := wait(t, m, info.ID); end.Session != "t-9" || end.Error != "" {
		t.Errorf("ended: %+v", end)
	}
}

func TestFailures(t *testing.T) {
	m, bin, work := newManager(t)
	fakeCLI(t, bin, "codex", `echo '{"type":"turn.failed","error":{"message":"usage limit reached"}}'; exit 1`)
	fakeCLI(t, bin, "cursor-agent", `echo "line 1" >&2; echo "Error: not logged in" >&2; exit 3`)
	fakeCLI(t, bin, "kilo", `exit 0`)

	for harness, want := range map[string]string{
		"codex":  "usage limit reached",
		"cursor": "line 1\nError: not logged in",
		"kilo":   "the CLI ended without starting a conversation",
	} {
		info, err := m.Start(launch.Request{Harness: harness, Cwd: work, Prompt: "x"})
		if err != nil {
			t.Fatal(err)
		}
		if end := wait(t, m, info.ID); end.Error != want {
			t.Errorf("%s: error %q, want %q", harness, end.Error, want)
		}
	}

	_, err := m.Start(launch.Request{Harness: "pi", Cwd: work, Prompt: "x"})
	var re requestError
	if !errors.As(err, &re) || !strings.Contains(err.Error(), "pi isn't installed") {
		t.Errorf("missing CLI: %v", err)
	}
	if _, err := m.Start(launch.Request{Harness: "claude", Cwd: filepath.Join(work, "gone"), Prompt: "x"}); !errors.As(err, &re) {
		t.Errorf("missing folder: %v", err)
	}
	if _, err := m.Start(launch.Request{Harness: "claude", Session: "nope", Prompt: "x"}); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("unknown conversation: %v", err)
	}
}

func TestResumeStopAndConflict(t *testing.T) {
	m, bin, work := newManager(t)
	fakeCLI(t, bin, "claude", `pwd > "$0.cwd"; exec sleep 30`)
	// The folder comes from the conversation, not the request.
	info, err := m.Start(launch.Request{Harness: "claude", Session: "conv-1", Cwd: "/elsewhere", Prompt: "more"})
	if err != nil {
		t.Fatal(err)
	}
	if info.Session != "conv-1" || info.Cwd != work {
		t.Errorf("started: %+v", info)
	}
	_, err = m.Start(launch.Request{Harness: "claude", Session: "conv-1", Prompt: "again"})
	var ce conflictError
	if !errors.As(err, &ce) {
		t.Errorf("second prompt while running: %v", err)
	}
	start := time.Now()
	if !m.Stop(info.ID) {
		t.Fatal("Stop: no such run")
	}
	end := wait(t, m, info.ID)
	if !end.Stopped || end.Error != "" || time.Since(start) > 3*time.Second {
		t.Errorf("stopped: %+v after %s", end, time.Since(start))
	}
	if m.Stop("nope") {
		t.Error("Stop of an unknown run")
	}
}

func TestHTTP(t *testing.T) {
	m, bin, work := newManager(t)
	fakeCLI(t, bin, "claude", `exit 0`)
	mux := http.NewServeMux()
	m.Register(mux)
	do := func(method, url string, body any) *httptest.ResponseRecorder {
		var b bytes.Buffer
		if body != nil {
			json.NewEncoder(&b).Encode(body)
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, url, &b)
		req.Header.Set("Content-Type", "application/json")
		mux.ServeHTTP(rec, req)
		return rec
	}

	// A form or text/plain post (what another site can send without a preflight) is refused.
	plain := httptest.NewRequest("POST", "/api/runs", strings.NewReader(`{"harness":"claude","cwd":"`+work+`","prompt":"x"}`))
	plain.Header.Set("Content-Type", "text/plain")
	if rec := httptest.NewRecorder(); func() int { mux.ServeHTTP(rec, plain); return rec.Code }() != 415 || len(m.List()) != 0 {
		t.Errorf("text/plain post started a run")
	}

	rec := do("POST", "/api/runs/preview", launch.Request{Harness: "claude", Cwd: work, Prompt: "hi", Effort: "max"})
	var plan launch.Plan
	json.Unmarshal(rec.Body.Bytes(), &plan)
	if rec.Code != 200 || plan.Command != "cd "+work+" && claude -p --session-id new-session-uuid --effort max -- hi" {
		t.Errorf("preview %d %s", rec.Code, rec.Body)
	}
	if rec := do("POST", "/api/runs/preview", launch.Request{Harness: "claude", Cwd: work, Prompt: "hi", Effort: "x y"}); rec.Code != 400 {
		t.Errorf("bad preview %d", rec.Code)
	}

	rec = do("GET", "/api/harnesses", nil)
	var hs struct {
		Harnesses []harnessInfo `json:"harnesses"`
	}
	json.Unmarshal(rec.Body.Bytes(), &hs)
	if len(hs.Harnesses) != 6 || hs.Harnesses[0].Found != filepath.Join(bin, "claude") || hs.Harnesses[1].Found != "" {
		t.Errorf("harnesses %s", rec.Body)
	}
	if rec := do("GET", "/api/harnesses/claude/models", nil); !strings.Contains(rec.Body.String(), `"opus"`) {
		t.Errorf("models %s", rec.Body)
	}
	if rec := do("GET", "/api/harnesses/nope/commands", nil); rec.Code != 404 {
		t.Errorf("commands of an unknown harness: %d", rec.Code)
	}

	rec = do("POST", "/api/runs", launch.Request{Harness: "claude", Cwd: work, Prompt: "hi"})
	var info Info
	json.Unmarshal(rec.Body.Bytes(), &info)
	if rec.Code != 201 || info.ID == "" {
		t.Fatalf("start %d %s", rec.Code, rec.Body)
	}
	wait(t, m, info.ID)
	if rec := do("GET", "/api/runs/"+info.ID, nil); !strings.Contains(rec.Body.String(), `"running":false`) {
		t.Errorf("get %s", rec.Body)
	}
	if rec := do("GET", "/api/runs", nil); !strings.Contains(rec.Body.String(), info.ID) {
		t.Errorf("list %s", rec.Body)
	}
	if rec := do("DELETE", "/api/runs/"+info.ID, nil); rec.Code != 204 {
		t.Errorf("stop %d", rec.Code)
	}
	if rec := do("DELETE", "/api/runs/nope", nil); rec.Code != 404 {
		t.Errorf("stop unknown %d", rec.Code)
	}
}
