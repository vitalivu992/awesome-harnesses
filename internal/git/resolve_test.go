package git

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// conflictRepo makes a merge conflict on c.txt: the base has "a\nbase\nz\n",
// main (HEAD, ours) changes the middle line to "mine", the branch "other"
// (theirs) to "theirs".
func conflictRepo(t *testing.T) string {
	t.Helper()
	dir := gitRepo(t)
	runGit(t, dir, "checkout", "-q", "-b", "main")
	write(t, dir, "c.txt", "a\nbase\nz\n")
	runGit(t, dir, "add", "c.txt")
	runGit(t, dir, "commit", "-qm", "base")
	runGit(t, dir, "checkout", "-q", "-b", "other")
	write(t, dir, "c.txt", "a\ntheirs\nz\n")
	runGit(t, dir, "commit", "-qam", "theirs")
	runGit(t, dir, "checkout", "-q", "main")
	write(t, dir, "c.txt", "a\nmine\nz\n")
	runGit(t, dir, "commit", "-qam", "mine")
	cmd := exec.Command("git", "-C", dir, "merge", "other")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("merge did not conflict:\n%s", out)
	}
	return dir
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	(&Handler{}).Register(mux)
	return mux
}

func postJSON(mux *http.ServeMux, url, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestResolveTakes(t *testing.T) {
	for take, want := range map[string]string{
		"mine":        "a\nmine\nz\n",
		"theirs":      "a\ntheirs\nz\n",
		"mine-theirs": "a\nmine\ntheirs\nz\n",
		"theirs-mine": "a\ntheirs\nmine\nz\n",
	} {
		t.Run(take, func(t *testing.T) {
			dir := conflictRepo(t)
			mux := newMux()
			rec := postJSON(mux, "/api/git/resolve", `{"cwd":"`+dir+`","path":"c.txt","take":"`+take+`"}`)
			if rec.Code != http.StatusNoContent {
				t.Fatalf("resolve = %d (%s)", rec.Code, rec.Body)
			}
			if got := read(t, dir, "c.txt"); got != want {
				t.Errorf("c.txt = %q, want %q", got, want)
			}
			// Resolved and staged: no conflict left ("mine" is HEAD's file,
			// so nothing is left to stage at all).
			staged, changes := parseStatus(mustStatus(t, dir))
			if len(changes) != 0 || len(staged) > 1 || (len(staged) == 1 && staged[0].X != "M") {
				t.Errorf("status after resolve: staged %+v changes %+v", staged, changes)
			}
			// Asking again: nothing to resolve.
			if rec := postJSON(mux, "/api/git/resolve", `{"cwd":"`+dir+`","path":"c.txt","take":"mine"}`); rec.Code != http.StatusConflict {
				t.Errorf("second resolve = %d, want 409", rec.Code)
			}
		})
	}
}

func TestResolveDuringRebaseSwapsSides(t *testing.T) {
	dir := conflictRepo(t)
	runGit(t, dir, "merge", "--abort")
	// Replay main's "mine" commit onto other: git's ours is other there.
	cmd := exec.Command("git", "-C", dir, "rebase", "other")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("rebase did not conflict:\n%s", out)
	}
	if !rebasing(context.Background(), dir) {
		t.Fatal("rebasing = false during a rebase")
	}
	if err := resolve(context.Background(), dir, "c.txt", "mine"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, dir, "c.txt"); got != "a\nmine\nz\n" {
		t.Errorf("mine during a rebase = %q, want the replayed commit's side", got)
	}
}

func TestResolveDeleteConflict(t *testing.T) {
	dir := gitRepo(t)
	runGit(t, dir, "checkout", "-q", "-b", "main")
	runGit(t, dir, "checkout", "-q", "-b", "other")
	runGit(t, dir, "rm", "-q", "a.txt")
	runGit(t, dir, "commit", "-qm", "delete a")
	runGit(t, dir, "checkout", "-q", "main")
	write(t, dir, "a.txt", "changed\n")
	runGit(t, dir, "commit", "-qam", "change a")
	if out, err := exec.Command("git", "-C", dir, "merge", "other").CombinedOutput(); err == nil {
		t.Fatalf("merge did not conflict:\n%s", out)
	}
	staged, _ := parseStatus(mustStatus(t, dir))
	if len(staged) != 1 || staged[0].Conflict != "UD" {
		t.Fatalf("staged = %+v, want a UD conflict", staged)
	}
	ctx := context.Background()
	if err := resolve(ctx, dir, "a.txt", "mine-theirs"); err != errOneSide {
		t.Errorf("combine with a deleted side = %v, want errOneSide", err)
	}
	// Theirs deleted it: taking theirs removes the file.
	if err := resolve(ctx, dir, "a.txt", "theirs"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.txt")); !os.IsNotExist(err) {
		t.Errorf("a.txt still there after taking the deleting side: %v", err)
	}
}

func TestConflictInfo(t *testing.T) {
	dir := conflictRepo(t)
	ctx := context.Background()
	info := Conflict(ctx, filepath.Join(dir, "c.txt"))
	if info == nil || info.Path != "c.txt" || !info.Both {
		t.Fatalf("Conflict = %+v", info)
	}
	if real, _ := filepath.EvalSymlinks(dir); info.Cwd != dir && info.Cwd != real {
		t.Errorf("Cwd = %q, want %q", info.Cwd, dir)
	}
	if info := Conflict(ctx, filepath.Join(dir, "a.txt")); info != nil {
		t.Errorf("a merged file has conflict %+v", info)
	}
	if info := Conflict(ctx, filepath.Join(t.TempDir(), "x.txt")); info != nil {
		t.Errorf("a file outside a repository has conflict %+v", info)
	}
}

func getDiff(t *testing.T, mux *http.ServeMux, query string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/git/diff?"+query, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("diff %s = %d (%s)", query, rec.Code, rec.Body)
	}
	var out struct {
		Diff string `json:"diff"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Diff
}

func TestDiff(t *testing.T) {
	dir := gitRepo(t)
	mux := newMux()
	write(t, dir, "a.txt", "one\ntwo\n")
	if d := getDiff(t, mux, "cwd="+dir+"&path=a.txt"); !strings.Contains(d, "+two") || !strings.Contains(d, " one") {
		t.Errorf("working tree diff = %q", d)
	}
	runGit(t, dir, "add", "a.txt")
	if d := getDiff(t, mux, "cwd="+dir+"&path=a.txt"); d != "" {
		t.Errorf("working tree diff after staging = %q", d)
	}
	if d := getDiff(t, mux, "cwd="+dir+"&path=a.txt&staged=1"); !strings.Contains(d, "+two") {
		t.Errorf("staged diff = %q", d)
	}
	// An untracked file, and an untracked folder (as git status lists it).
	write(t, dir, "new.txt", "hello\n")
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "sub/x.txt", "x\n")
	write(t, dir, "sub/y.txt", "y\n")
	if d := getDiff(t, mux, "cwd="+dir+"&path=new.txt&untracked=1"); !strings.Contains(d, "+hello") {
		t.Errorf("untracked diff = %q", d)
	}
	if d := getDiff(t, mux, "cwd="+dir+"&path=sub/&untracked=1"); !strings.Contains(d, "+x") || !strings.Contains(d, "+y") {
		t.Errorf("untracked folder diff = %q", d)
	}
	// A conflict: the combined diff with its markers.
	cdir := conflictRepo(t)
	if d := getDiff(t, mux, "cwd="+cdir+"&path=c.txt"); !strings.Contains(d, "diff --cc") || !strings.Contains(d, "<<<<<<<") {
		t.Errorf("conflict diff = %q", d)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/git/diff?cwd="+dir+"&path=../x", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("diff of ../x = %d, want 400", rec.Code)
	}
}

// A project folder inside a repository: status paths are relative to the
// root, and the actions take them as such.
func TestPathsFromSubfolder(t *testing.T) {
	dir := gitRepo(t)
	sub := filepath.Join(dir, "app")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "app/m.txt", "m\n")
	mux := newMux()
	if rec := postJSON(mux, "/api/git/stage", `{"cwd":"`+sub+`","paths":["app/m.txt"]}`); rec.Code != http.StatusNoContent {
		t.Fatalf("stage from a subfolder = %d (%s)", rec.Code, rec.Body)
	}
	if d := getDiff(t, mux, "cwd="+sub+"&path=app/m.txt&staged=1"); !strings.Contains(d, "+m") {
		t.Errorf("staged diff from a subfolder = %q", d)
	}
}
