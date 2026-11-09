package git

// The file viewer's git side: one file's diff, and resolving a merge
// conflict by taking one side or both sides in a chosen order.
//
//	GET  /api/git/diff?cwd=&path=[&orig=][&staged=1][&untracked=1]   → {diff, truncated}
//	POST /api/git/resolve   {cwd, path, take}   take: mine | theirs | mine-theirs | theirs-mine

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// maxDiff caps one diff answer (the rest is cut, truncated: true).
const maxDiff = 1 << 20

// maxUntracked caps the files shown for an untracked folder.
const maxUntracked = 50

// ConflictInfo locates an unresolved conflict: the repository root and the
// file's path in it, and whether both sides still have the file (only then
// can the two sides be combined).
type ConflictInfo struct {
	Cwd  string `json:"cwd"`
	Path string `json:"path"`
	Both bool   `json:"both"`
}

// errNotConflicted: the file has no unmerged entries (already resolved).
var errNotConflicted = errors.New("the file is not in conflict")

// errOneSide: a side deleted the file, so the sides can't be combined.
var errOneSide = errors.New("one side deleted the file: take mine or theirs")

// Conflict reports whether the file at the absolute path is unmerged in its
// repository, nil when it is not (or it is in no repository at all).
func Conflict(ctx context.Context, path string) *ConflictInfo {
	dir, name := filepath.Split(path)
	out, err := git(ctx, dir, "ls-files", "-u", "-z", "--full-name", "--", name)
	if err != nil || out == "" {
		return nil
	}
	stages, rel := parseUnmerged(out)
	root, err := toplevel(ctx, dir)
	if err != nil || rel == "" {
		return nil
	}
	return &ConflictInfo{Cwd: root, Path: rel, Both: stages[2] && stages[3]}
}

// parseUnmerged reads `git ls-files -u -z` for one file: which stages it
// has (1 base, 2 ours, 3 theirs) and its path. Each record is
// "<mode> <object> <stage>\t<path>\0".
func parseUnmerged(z string) (map[int]bool, string) {
	stages := map[int]bool{}
	path := ""
	for _, rec := range strings.Split(z, "\x00") {
		meta, p, ok := strings.Cut(rec, "\t")
		if !ok {
			continue
		}
		fields := strings.Fields(meta)
		if len(fields) != 3 {
			continue
		}
		if n, err := strconv.Atoi(fields[2]); err == nil {
			stages[n] = true
			path = p
		}
	}
	return stages, path
}

// toplevel is the root of the repository cwd is in: status paths are
// relative to it, so path arguments are given there.
func toplevel(ctx context.Context, cwd string) (string, error) {
	out, err := git(ctx, cwd, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.FromSlash(strings.TrimSpace(out)), nil
}

func (h *Handler) handleDiff(w http.ResponseWriter, r *http.Request) {
	cwd, ok := cwdArg(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	path, orig := q.Get("path"), q.Get("orig")
	if !validPath(path) || (orig != "" && !validPath(orig)) {
		writeError(w, http.StatusBadRequest, "path is not a file of this repository")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	root, err := toplevel(ctx, cwd)
	if err != nil {
		writeGitError(w, err)
		return
	}
	diff := []string{"-c", "core.quotepath=off", "diff", "--no-color", "--no-ext-diff"}
	var out string
	switch {
	case q.Get("untracked") == "1":
		out, err = untrackedDiff(ctx, root, path)
	case q.Get("staged") == "1":
		args := append(diff, "--cached", "-M", "--", path)
		if orig != "" {
			args = append(args, orig)
		}
		out, err = git(ctx, root, args...)
	default:
		out, err = git(ctx, root, append(diff, "--", path)...)
	}
	if err != nil {
		writeGitError(w, err)
		return
	}
	truncated := len(out) > maxDiff
	if truncated {
		out = out[:maxDiff]
		if i := strings.LastIndexByte(out, '\n'); i > 0 {
			out = out[:i+1]
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"diff": out, "truncated": truncated})
}

// untrackedDiff shows an untracked file (or the files of an untracked
// folder, which git status reports with a trailing slash) as all added.
func untrackedDiff(ctx context.Context, root, path string) (string, error) {
	list, err := git(ctx, root, "ls-files", "-o", "--exclude-standard", "-z", "--", path)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	n := 0
	for _, f := range strings.Split(list, "\x00") {
		if f == "" {
			continue
		}
		if n++; n > maxUntracked || b.Len() > maxDiff {
			break
		}
		out, err := noIndexDiff(ctx, root, f)
		if err != nil {
			return "", err
		}
		b.WriteString(out)
	}
	return b.String(), nil
}

// noIndexDiff is `git diff --no-index /dev/null <file>`, which exits 1 when
// there is a difference (always, for a non-empty file).
func noIndexDiff(ctx context.Context, root, file string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", root, "-c", "core.quotepath=off",
		"diff", "--no-index", "--no-color", "--no-ext-diff", "--", "/dev/null", file)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
		return "", &gitError{stderr: stderr.String(), err: err}
	}
	return stdout.String(), nil
}

func (h *Handler) handleResolve(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Cwd  string `json:"cwd"`
		Path string `json:"path"`
		Take string `json:"take"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if !validCwd(w, req.Cwd) {
		return
	}
	if !validPath(req.Path) || strings.HasSuffix(req.Path, "/") {
		writeError(w, http.StatusBadRequest, "path is not a file of this repository")
		return
	}
	switch req.Take {
	case "mine", "theirs", "mine-theirs", "theirs-mine":
	default:
		writeError(w, http.StatusBadRequest, "take must be mine, theirs, mine-theirs or theirs-mine")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	root, err := toplevel(ctx, req.Cwd)
	if err != nil {
		writeGitError(w, err)
		return
	}
	switch err := resolve(ctx, root, req.Path, req.Take); {
	case errors.Is(err, errNotConflicted):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, errOneSide):
		writeError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		writeGitError(w, err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// resolve settles the conflict on path (relative to root) and stages the
// result. "mine" is HEAD's side (stage 2), "theirs" the side merged in
// (stage 3) — except during a rebase, where HEAD is the branch rebased onto
// and your own commit is the one being replayed, so the two swap.
// mine / theirs take that side's whole file (git checkout --ours/--theirs;
// a side that deleted the file removes it). mine-theirs / theirs-mine keep
// every change, each conflicting block as one side's lines then the other's
// (git merge-file --union).
func resolve(ctx context.Context, root, path, take string) error {
	out, err := git(ctx, root, "ls-files", "-u", "-z", "--", path)
	if err != nil {
		return err
	}
	stages, _ := parseUnmerged(out)
	if len(stages) == 0 {
		return errNotConflicted
	}
	mine, theirs := 2, 3
	if rebasing(ctx, root) {
		mine, theirs = 3, 2
	}
	switch take {
	case "mine":
		return takeSide(ctx, root, path, mine, stages[mine])
	case "theirs":
		return takeSide(ctx, root, path, theirs, stages[theirs])
	case "mine-theirs":
		return union(ctx, root, path, mine, theirs, stages)
	default:
		return union(ctx, root, path, theirs, mine, stages)
	}
}

func takeSide(ctx context.Context, root, path string, stage int, exists bool) error {
	if !exists {
		// That side deleted the file: taking it is the deletion.
		_, err := git(ctx, root, "rm", "-q", "--", path)
		return err
	}
	side := "--ours"
	if stage == 3 {
		side = "--theirs"
	}
	if _, err := git(ctx, root, "checkout", side, "--", path); err != nil {
		return err
	}
	_, err := git(ctx, root, "add", "--", path)
	return err
}

func union(ctx context.Context, root, path string, first, second int, stages map[int]bool) error {
	if !stages[first] || !stages[second] {
		return errOneSide
	}
	tmp, err := os.MkdirTemp("", "ah-merge-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	files := map[int]string{}
	for _, n := range []int{1, 2, 3} {
		content := ""
		if stages[n] {
			if content, err = git(ctx, root, "cat-file", "blob", ":"+strconv.Itoa(n)+":"+path); err != nil {
				return err
			}
		}
		files[n] = filepath.Join(tmp, strconv.Itoa(n))
		if err := os.WriteFile(files[n], []byte(content), 0o600); err != nil {
			return err
		}
	}
	merged, err := git(ctx, root, "merge-file", "-p", "--union", files[first], files[1], files[second])
	if err != nil {
		return err
	}
	target := filepath.Join(root, filepath.FromSlash(path))
	mode := os.FileMode(0o644)
	if info, err := os.Stat(target); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.WriteFile(target, []byte(merged), mode); err != nil {
		return err
	}
	_, err = git(ctx, root, "add", "--", path)
	return err
}

// rebasing reports whether a rebase is stopped in the repository.
func rebasing(ctx context.Context, root string) bool {
	for _, dir := range []string{"rebase-merge", "rebase-apply"} {
		out, err := git(ctx, root, "rev-parse", "--git-path", dir)
		if err != nil {
			continue
		}
		p := filepath.FromSlash(strings.TrimSpace(out))
		if !filepath.IsAbs(p) {
			p = filepath.Join(root, p)
		}
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}
