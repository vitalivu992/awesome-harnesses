// Package git runs git in a project folder for the Explorer panel: the
// status split into staged and unstaged changes, staging, unstaging,
// discarding and committing, and a short log. Every request names the
// folder in `cwd` (the git commands run with -C there); paths are
// repository-relative, as `git status` reports them.
//
//	GET  /api/git/status?cwd=   branch + staged changes + changes
//	GET  /api/git/log?cwd=      the newest commits
//	POST /api/git/stage         {cwd, paths} → git add
//	POST /api/git/unstage       {cwd, paths} → git reset HEAD (unborn: git rm --cached)
//	POST /api/git/discard       {cwd, paths} → git checkout HEAD (tracked files)
//	POST /api/git/commit        {cwd, message} → git commit -m, → the new hash
//	GET  /api/git/diff, POST /api/git/resolve: see resolve.go
package git

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Handler serves the git endpoints.
type Handler struct{}

// Register adds the routes to mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/git/status", h.handleStatus)
	mux.HandleFunc("GET /api/git/log", h.handleLog)
	mux.HandleFunc("POST /api/git/stage", h.handlePaths(h.stage))
	mux.HandleFunc("POST /api/git/unstage", h.handlePaths(h.unstage))
	mux.HandleFunc("POST /api/git/discard", h.handlePaths(h.discard))
	mux.HandleFunc("POST /api/git/commit", h.handleCommit)
	mux.HandleFunc("GET /api/git/diff", h.handleDiff)
	mux.HandleFunc("POST /api/git/resolve", h.handleResolve)
}

// Entry is one file of the status: x/y are git's porcelain status letters
// ("M", "A", …; "?" untracked, "!" a conflict), orig the previous path of
// a rename, conflict a conflict's own two letters ("UU", "DU", …).
type Entry struct {
	Path     string `json:"path"`
	Orig     string `json:"orig,omitempty"`
	X        string `json:"x"`
	Y        string `json:"y"`
	Conflict string `json:"conflict,omitempty"`
}

// Status is the repository's state the Git tab shows. Root is the
// repository's folder: the entries' paths are relative to it.
type Status struct {
	Root   string  `json:"root"`
	Branch string  `json:"branch"`
	Staged []Entry `json:"staged"`
	Files  []Entry `json:"changes"`
}

// Commit is one entry of the log.
type Commit struct {
	Hash    string `json:"hash"`
	Date    string `json:"date"`
	Author  string `json:"author"`
	Subject string `json:"subject"`
}

// maxLogCommits is how many log entries are returned.
const maxLogCommits = 30

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	cwd, ok := cwdArg(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	out, err := git(ctx, cwd, "status", "--porcelain=v1", "-z")
	if err != nil {
		writeGitError(w, err)
		return
	}
	staged, changes := parseStatus(out)
	root, _ := toplevel(ctx, cwd)
	branch, _ := git(ctx, cwd, "branch", "--show-current")
	branch = strings.TrimSpace(branch)
	if branch == "" {
		if b, err := git(ctx, cwd, "rev-parse", "--short", "HEAD"); err == nil {
			branch = "(detached) " + strings.TrimSpace(b)
		} else {
			branch = "(no commits yet)"
		}
	}
	if staged == nil {
		staged = []Entry{}
	}
	if changes == nil {
		changes = []Entry{}
	}
	writeJSON(w, http.StatusOK, Status{Root: root, Branch: branch, Staged: staged, Files: changes})
}

func (h *Handler) handleLog(w http.ResponseWriter, r *http.Request) {
	cwd, ok := cwdArg(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	out, err := git(ctx, cwd, "log", "-n", strconv.Itoa(maxLogCommits),
		"--pretty=format:%h%x1f%aI%x1f%an%x1f%s%x1e")
	if err != nil {
		writeGitError(w, err)
		return
	}
	commits := []Commit{}
	for _, rec := range strings.Split(strings.TrimRight(out, "\x1e\n"), "\x1e") {
		fields := strings.Split(strings.TrimSpace(rec), "\x1f")
		if len(fields) != 4 {
			continue
		}
		commits = append(commits, Commit{Hash: fields[0], Date: fields[1], Author: fields[2], Subject: fields[3]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"commits": commits})
}

func (h *Handler) stage(ctx context.Context, cwd string, paths []string) error {
	_, err := git(ctx, cwd, append([]string{"add", "--"}, paths...)...)
	return err
}

func (h *Handler) unstage(ctx context.Context, cwd string, paths []string) error {
	_, err := git(ctx, cwd, append([]string{"reset", "-q", "HEAD", "--"}, paths...)...)
	if err == nil {
		return nil
	}
	// A repository without commits has no HEAD to reset to: removing from
	// the index is the same thing there.
	_, err = git(ctx, cwd, append([]string{"rm", "-r", "--cached", "-q", "--"}, paths...)...)
	return err
}

func (h *Handler) discard(ctx context.Context, cwd string, paths []string) error {
	_, err := git(ctx, cwd, append([]string{"checkout", "-q", "HEAD", "--"}, paths...)...)
	return err
}

// handlePaths runs fn for a {cwd, paths} body.
func (h *Handler) handlePaths(fn func(context.Context, string, []string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Cwd   string   `json:"cwd"`
			Paths []string `json:"paths"`
		}
		if !readJSON(w, r, &req) {
			return
		}
		if !validCwd(w, req.Cwd) {
			return
		}
		if len(req.Paths) == 0 {
			writeError(w, http.StatusBadRequest, "paths is required")
			return
		}
		for _, p := range req.Paths {
			if !validPath(p) {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("%q is not a file of this repository", p))
				return
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		// The paths are relative to the repository root (as git status
		// reports them), which is not cwd when the project is a subfolder.
		root, err := toplevel(ctx, req.Cwd)
		if err != nil {
			writeGitError(w, err)
			return
		}
		if err := fn(ctx, root, req.Paths); err != nil {
			writeGitError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) handleCommit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Cwd     string `json:"cwd"`
		Message string `json:"message"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if !validCwd(w, req.Cwd) {
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		writeError(w, http.StatusBadRequest, "the commit message is empty")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if _, err := git(ctx, req.Cwd, "commit", "-m", req.Message); err != nil {
		writeGitError(w, err)
		return
	}
	hash, _ := git(ctx, req.Cwd, "rev-parse", "--short", "HEAD")
	writeJSON(w, http.StatusOK, map[string]string{"commit": strings.TrimSpace(hash)})
}

// gitError is a failed git command; Unwrap keeps the cause (git missing).
type gitError struct {
	stderr string
	err    error
}

func (e *gitError) Error() string {
	msg := strings.TrimSpace(e.stderr)
	if msg == "" {
		msg = e.err.Error()
	}
	return msg
}

func (e *gitError) Unwrap() error { return e.err }

func git(ctx context.Context, cwd string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", cwd}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", &gitError{stderr: stderr.String(), err: err}
	}
	return stdout.String(), nil
}

// notRepo reports whether err is git refusing a folder outside a repository.
func notRepo(err error) bool {
	var ge *gitError
	return errors.As(err, &ge) && strings.Contains(ge.stderr, "not a git repository")
}

func writeGitError(w http.ResponseWriter, err error) {
	if errors.Is(err, exec.ErrNotFound) {
		writeError(w, http.StatusNotImplemented, "git is not installed on the server")
		return
	}
	if notRepo(err) {
		writeError(w, http.StatusBadRequest, "not a git repository")
		return
	}
	log.Printf("git: %v", err)
	writeError(w, http.StatusInternalServerError, err.Error())
}

// parseStatus reads `git status --porcelain=v1 -z` output into staged and
// unstaged entries. A file in both (e.g. "MM") appears in both lists; an
// unresolved conflict shows as "!". With -z the entries are NUL-terminated,
// and a rename or copy is two strings in a row: the new path, then the
// original.
func parseStatus(z string) (staged, changes []Entry) {
	s := z
	for len(s) >= 4 {
		x, y := string(s[0]), string(s[1])
		path := s[3:]
		if i := strings.IndexByte(path, 0); i >= 0 {
			s = path[i+1:]
			path = path[:i]
		} else {
			s = ""
		}
		if path == "" {
			continue
		}
		entry := Entry{Path: path, X: x, Y: y}
		if x == "R" || x == "C" {
			if i := strings.IndexByte(s, 0); i >= 0 {
				entry.Orig, s = s[:i], s[i+1:]
			} else {
				entry.Orig, s = s, ""
			}
		}
		if x == "U" || y == "U" || conflictCode(x+y) {
			entry.X, entry.Y, entry.Conflict = "!", "!", x+y
		}
		// "?" (untracked) is only a change; everything else staged is also
		// listed under Staged.
		if entry.X != " " && entry.X != "?" {
			staged = append(staged, entry)
		}
		if entry.Y != " " {
			changes = append(changes, entry)
		}
	}
	return staged, changes
}

// conflictCode: the porcelain codes of an unresolved merge conflict.
func conflictCode(xy string) bool {
	switch xy {
	case "DD", "AU", "UD", "UA", "DU", "AA", "UU":
		return true
	}
	return false
}

func cwdArg(w http.ResponseWriter, r *http.Request) (string, bool) {
	cwd := r.URL.Query().Get("cwd")
	if !validCwd(w, cwd) {
		return "", false
	}
	return cwd, true
}

func validCwd(w http.ResponseWriter, cwd string) bool {
	if !filepath.IsAbs(cwd) {
		writeError(w, http.StatusBadRequest, "cwd must be an absolute path")
		return false
	}
	return true
}

// validPath accepts repository-relative paths: no drive/absolute paths, no
// walking out with .., no git's own magic pathspecs. A trailing slash is
// fine: git status reports an untracked folder that way.
func validPath(p string) bool {
	if p == "" || filepath.IsAbs(p) || strings.HasPrefix(p, ":") {
		return false
	}
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == ".." || part == "" {
			return false
		}
	}
	return true
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if ct := r.Header.Get("Content-Type"); ct != "" {
		if ct != "application/json" {
			writeError(w, http.StatusUnsupportedMediaType, "send JSON")
			return false
		}
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return false
	}
	return true
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("git: write response: %v", err)
	}
}
