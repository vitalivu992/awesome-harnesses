// Package files lists a project folder for the Explorer panel's file tree.
// Hidden entries (dotfiles, so also .git) are skipped; each directory is
// listed when it is expanded, so large folders cost nothing until opened.
//
//	GET /api/files?cwd=           the folder's entries: folders first, then files
//	GET /api/files/content?path=  one file's text for the viewer (+ its merge conflict)
package files

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"awesome-harness/internal/git"
)

// maxEntries caps one listing (a folder with more is huge either way).
const maxEntries = 5000

// maxFile caps the text a file view returns (the rest is cut, truncated: true).
const maxFile = 2 << 20

// sniffLen: a NUL byte in this many first bytes makes a file binary.
const sniffLen = 8000

// Content is one file for the viewer. Binary files come without text.
// Conflict is set when the file is unmerged in its git repository.
type Content struct {
	Path      string            `json:"path"`
	Size      int64             `json:"size"`
	Text      string            `json:"text"`
	Binary    bool              `json:"binary"`
	Truncated bool              `json:"truncated"`
	Conflict  *git.ConflictInfo `json:"conflict,omitempty"`
}

// Entry is one item of a folder.
type Entry struct {
	Name string `json:"name"`
	Dir  bool   `json:"dir"`
	Size int64  `json:"size"`
}

// Handler serves the files endpoints.
type Handler struct{}

// Register adds the routes to mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/files", h.listing)
	mux.HandleFunc("GET /api/files/content", h.content)
}

func (h *Handler) content(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if !filepath.IsAbs(path) {
		writeError(w, http.StatusBadRequest, "path must be an absolute path")
		return
	}
	path = filepath.Clean(path)
	f, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if info.IsDir() {
		writeError(w, http.StatusBadRequest, "a folder, not a file")
		return
	}
	data, err := io.ReadAll(io.LimitReader(f, maxFile+1))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := Content{Path: path, Size: info.Size(), Truncated: len(data) > maxFile}
	if out.Truncated {
		data = data[:maxFile]
	}
	if bytes.IndexByte(data[:min(len(data), sniffLen)], 0) >= 0 {
		out.Binary = true
	} else {
		out.Text = string(data)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	out.Conflict = git.Conflict(ctx, path)
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) listing(w http.ResponseWriter, r *http.Request) {
	cwd := r.URL.Query().Get("cwd")
	if !filepath.IsAbs(cwd) {
		writeError(w, http.StatusBadRequest, "cwd must be an absolute path")
		return
	}
	dir, err := os.ReadDir(cwd)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	entries := []Entry{}
	for _, e := range dir {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		entry := Entry{Name: e.Name(), Dir: e.IsDir()}
		if !entry.Dir {
			if info, err := e.Info(); err == nil {
				entry.Size = info.Size()
			}
		}
		entries = append(entries, entry)
		if len(entries) >= maxEntries {
			break
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Dir != entries[j].Dir {
			return entries[i].Dir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("files: write response: %v", err)
	}
}
