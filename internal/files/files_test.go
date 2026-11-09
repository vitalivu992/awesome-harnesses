package files

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListing(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"z.txt", "a.txt", ".hidden", ".git"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "Beta"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "alpha"), 0o755); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	(&Handler{}).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/files?cwd="+dir, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d (%s)", rec.Code, rec.Body)
	}
	var out struct {
		Entries []Entry `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range out.Entries {
		names = append(names, e.Name)
	}
	// Hidden entries skipped, folders first and case-insensitive.
	if got := strings.Join(names, ","); got != "alpha,Beta,a.txt,z.txt" {
		t.Errorf("entries = %s", got)
	}

	// A relative path is refused.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/files?cwd=relative", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("relative cwd = %d", rec.Code)
	}
}

func TestContent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.bin"), []byte("PK\x00\x01"), 0o644); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	(&Handler{}).Register(mux)
	get := func(path string) (int, Content) {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/files/content?path="+path, nil))
		var c Content
		_ = json.Unmarshal(rec.Body.Bytes(), &c)
		return rec.Code, c
	}
	if code, c := get(filepath.Join(dir, "a.go")); code != http.StatusOK || c.Text != "package a\n" || c.Binary || c.Conflict != nil {
		t.Errorf("text file = %d %+v", code, c)
	}
	if code, c := get(filepath.Join(dir, "b.bin")); code != http.StatusOK || !c.Binary || c.Text != "" || c.Size != 4 {
		t.Errorf("binary file = %d %+v", code, c)
	}
	for _, p := range []string{"relative.txt", dir, filepath.Join(dir, "missing")} {
		if code, _ := get(p); code != http.StatusBadRequest {
			t.Errorf("content of %s = %d, want 400", p, code)
		}
	}
}
