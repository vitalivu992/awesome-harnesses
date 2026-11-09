package git

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// gitRepo makes a repository with one commit and a tracked file "a.txt".
func gitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	write(t, dir, "a.txt", "one\n")
	run("add", "a.txt")
	run("commit", "-qm", "first")
	return dir
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(dir+"/"+name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestStatusStagedAndChanges(t *testing.T) {
	dir := gitRepo(t)
	write(t, dir, "a.txt", "one and two\n") // modified
	write(t, dir, "b.txt", "new\n")         // untracked
	if _, err := git(context.Background(), dir, "add", "a.txt"); err != nil {
		t.Fatal(err)
	}
	staged, changes := parseStatus(mustStatus(t, dir))
	if len(staged) != 1 || staged[0].Path != "a.txt" || staged[0].X != "M" {
		t.Errorf("staged = %+v", staged)
	}
	if len(changes) != 1 || changes[0].Path != "b.txt" || changes[0].X != "?" {
		t.Errorf("changes = %+v", changes)
	}
}

func TestStatusRename(t *testing.T) {
	dir := gitRepo(t)
	if _, err := git(context.Background(), dir, "mv", "a.txt", "z.txt"); err != nil {
		t.Fatal(err)
	}
	staged, changes := parseStatus(mustStatus(t, dir))
	if len(staged) != 1 || staged[0].Path != "z.txt" || staged[0].Orig != "a.txt" || staged[0].X != "R" {
		t.Errorf("staged = %+v", staged)
	}
	if len(changes) != 0 {
		t.Errorf("changes = %+v", changes)
	}
}

func TestStatusConflict(t *testing.T) {
	staged, changes := parseStatus("UU both.txt\x00")
	if len(staged) != 1 || staged[0].X != "!" || len(changes) != 1 || changes[0].Y != "!" {
		t.Errorf("staged = %+v changes = %+v", staged, changes)
	}
}

func mustStatus(t *testing.T, dir string) string {
	t.Helper()
	out, err := git(context.Background(), dir, "status", "--porcelain=v1", "-z")
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestEndpoints(t *testing.T) {
	dir := gitRepo(t)
	mux := http.NewServeMux()
	(&Handler{}).Register(mux)
	post := func(url, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, url, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}

	// Stage the untracked file, commit it, then unstage and discard a change.
	write(t, dir, "b.txt", "staged\n")
	if rec := post("/api/git/stage", `{"cwd":"`+dir+`","paths":["b.txt"]}`); rec.Code != http.StatusNoContent {
		t.Fatalf("stage = %d (%s)", rec.Code, rec.Body)
	}
	if rec := post("/api/git/commit", `{"cwd":"`+dir+`","message":"add b"}`); rec.Code != http.StatusOK {
		t.Fatalf("commit = %d (%s)", rec.Code, rec.Body)
	}
	write(t, dir, "b.txt", "changed\n")
	if rec := post("/api/git/discard", `{"cwd":"`+dir+`","paths":["b.txt"]}`); rec.Code != http.StatusNoContent {
		t.Fatalf("discard = %d (%s)", rec.Code, rec.Body)
	}
	if rec := post("/api/git/unstage", `{"cwd":"`+dir+`","paths":["b.txt"]}`); rec.Code != http.StatusNoContent {
		t.Errorf("unstage unchanged file = %d (%s)", rec.Code, rec.Body)
	}

	// The log has both commits.
	req := httptest.NewRequest(http.MethodGet, "/api/git/log?cwd="+dir, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "add b") || !strings.Contains(rec.Body.String(), "first") {
		t.Errorf("log = %d (%s)", rec.Code, rec.Body)
	}

	// Bad paths are refused without running git.
	for _, body := range []string{
		`{"cwd":"` + dir + `","paths":["../outside"]}`,
		`{"cwd":"` + dir + `","paths":["/etc/passwd"]}`,
		`{"cwd":"` + dir + `","paths":[":(top)"]}`,
		`{"cwd":"relative","paths":["a.txt"]}`,
	} {
		if rec := post("/api/git/stage", body); rec.Code != http.StatusBadRequest {
			t.Errorf("stage %s = %d, want 400", body, rec.Code)
		}
	}

	// A folder without a repository answers a friendly 400.
	empty := t.TempDir()
	req = httptest.NewRequest(http.MethodGet, "/api/git/status?cwd="+empty, nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "not a git repository") {
		t.Errorf("status outside a repository = %d (%s)", rec.Code, rec.Body)
	}
}

func TestUnstageWithoutCommits(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	// A repository with no commits at all.
	fresh := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", append([]string{"-C", fresh}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write(t, fresh, "new.txt", "hello\n")
	h := &Handler{}
	ctx := context.Background()
	if err := h.stage(ctx, fresh, []string{"new.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := h.unstage(ctx, fresh, []string{"new.txt"}); err != nil {
		t.Fatalf("unstage on an unborn branch: %v", err)
	}
	if _, changes := parseStatus(mustStatus(t, fresh)); len(changes) != 1 {
		t.Errorf("changes after unstage = %+v", changes)
	}
}

func TestValidPath(t *testing.T) {
	for p, ok := range map[string]bool{
		"a.txt": true, "web/dist/x.js": true, "dir/": true,
		"": false, "/abs": false, "../x": false, "a/../b": false, "a//b": false, ":(top)": false,
	} {
		if validPath(p) != ok {
			t.Errorf("validPath(%q) = %v, want %v", p, !ok, ok)
		}
	}
}
