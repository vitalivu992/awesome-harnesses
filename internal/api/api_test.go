package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"awesome-harness/internal/catalog"
)

func TestRoutes(t *testing.T) {
	dir := t.TempDir()
	mux := http.NewServeMux()
	(&Handler{Catalog: catalog.New(catalog.Config{ClaudeDir: dir, CursorDir: dir})}).Register(mux)
	cases := map[string]int{
		"/health":                            http.StatusOK,
		"/api/projects":                      http.StatusOK,
		"/api/conversations":                 http.StatusBadRequest,
		"/api/conversations?project=/nope":   http.StatusNotFound,
		"/api/conversations/claude/missing":  http.StatusNotFound,
		"/api/conversations/claude/..%2Fetc": http.StatusNotFound,
	}
	for url, want := range cases {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
		if rec.Code != want {
			t.Errorf("GET %s = %d, want %d (%s)", url, rec.Code, want, rec.Body)
		}
	}
}

func TestConversationNotModified(t *testing.T) {
	dir := t.TempDir()
	line := `{"type":"user","cwd":"/w","timestamp":"2026-10-20T10:00:00Z","message":{"role":"user","content":"hello"}}`
	if err := os.MkdirAll(filepath.Join(dir, "projects", "-w"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "projects", "-w", "c1.jsonl"), []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	(&Handler{Catalog: catalog.New(catalog.Config{ClaudeDir: dir})}).Register(mux)
	get := func(etag string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/conversations/claude/c1", nil)
		if etag != "" {
			req.Header.Set("If-None-Match", etag)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	first := get("")
	etag := first.Header().Get("ETag")
	if first.Code != http.StatusOK || etag == "" {
		t.Fatalf("first GET = %d, ETag %q", first.Code, etag)
	}
	if rec := get(etag); rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
		t.Errorf("same ETag = %d, body %q", rec.Code, rec.Body)
	}
	if rec := get(fmt.Sprintf(`W/%s`, etag)); rec.Code != http.StatusNotModified {
		t.Errorf("weak ETag = %d", rec.Code)
	}
	if rec := get(`"stale"`); rec.Code != http.StatusOK {
		t.Errorf("stale ETag = %d", rec.Code)
	}
}
