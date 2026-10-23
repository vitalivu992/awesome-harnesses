package api

import (
	"net/http"
	"net/http/httptest"
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
