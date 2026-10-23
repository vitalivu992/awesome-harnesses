// Command server serves the awesome-harness UI and its read-only API over
// the local Claude Code and Cursor transcripts.
package main

import (
	"compress/gzip"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"awesome-harness/internal/api"
	"awesome-harness/internal/catalog"
	"awesome-harness/web"
)

func env(name, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	return fallback
}

func main() {
	cfg := catalog.DefaultConfig()
	cfg.ClaudeDir = env("CLAUDE_DIR", cfg.ClaudeDir)
	cfg.CursorDir = env("CURSOR_DIR", cfg.CursorDir)

	mux := http.NewServeMux()
	(&api.Handler{Catalog: catalog.New(cfg)}).Register(mux)
	// The single-page app: bundle files, else index.html for client routes.
	// It 404s /health and /api/* paths no pattern above matched.
	mux.Handle("/", web.Handler(web.Dist))

	// Transcripts hold source code, prompts and secrets: listen on loopback
	// unless HOST says otherwise.
	addr := net.JoinHostPort(env("HOST", "127.0.0.1"), env("PORT", "8767"))
	if _, err := web.Dist.Open("index.html"); err != nil {
		log.Printf("warning: the embedded web bundle has no index.html — run `make web` and rebuild")
	}
	log.Printf("listening on http://%s (Claude Code: %s, Cursor: %s)", addr, cfg.ClaudeDir, cfg.CursorDir)
	srv := &http.Server{
		Addr:              addr,
		Handler:           logRequests(gzipJSON(mux)),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

// gzipJSON compresses API responses (a long conversation is megabytes of JSON).
func gzipJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		next.ServeHTTP(gzipWriter{w, gz}, r)
	})
}

type gzipWriter struct {
	http.ResponseWriter
	gz *gzip.Writer
}

func (g gzipWriter) Write(b []byte) (int, error) { return g.gz.Write(b) }

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.RequestURI(), sw.status, time.Since(start).Round(time.Millisecond))
	})
}
