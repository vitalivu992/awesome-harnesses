// Command server serves the Awesome Harnesses UI and its read-only API over
// the local agent conversations (Claude Code, Cursor, Codex, pi, OpenCode, Kilo),
// and the terminal panel's shells.
package main

import (
	"compress/gzip"
	"log"
	"net"
	"net/http"
	"os"
	"os/user"
	"slices"
	"strconv"
	"strings"
	"time"

	"awesome-harness/internal/api"
	"awesome-harness/internal/auth"
	"awesome-harness/internal/catalog"
	"awesome-harness/internal/shell"
	"awesome-harness/web"
)

// serverVars are the settings this server reads (often from .env via the
// systemd unit); the shells it starts don't inherit them.
var serverVars = []string{"HOST", "PORT", "PAM_SERVICE", "AUTH_DISABLED", "CLAUDE_DIR", "CURSOR_DIR",
	"CURSOR_LOGS_DIR", "CODEX_DIR", "PI_DIR", "OPENCODE_DB", "KILO_DB"}

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
	cfg.CursorLogsDir = env("CURSOR_LOGS_DIR", cfg.CursorLogsDir)
	cfg.CodexDir = env("CODEX_DIR", cfg.CodexDir)
	cfg.PiDir = env("PI_DIR", cfg.PiDir)
	cfg.OpenCodeDB = env("OPENCODE_DB", cfg.OpenCodeDB)
	cfg.KiloDB = env("KILO_DB", cfg.KiloDB)

	// Every /api/* route but the login ones needs a session: the password is
	// checked by the OS, as for a desktop login (PAM_SERVICE: Linux and macOS).
	pamService := env("PAM_SERVICE", "login")
	authenticate, err := auth.System(pamService)
	if err != nil {
		log.Fatal(err)
	}
	login := &auth.Handler{Auth: authenticate}
	// AUTH_DISABLED=1 drops the login (local testing, agents driving a browser).
	if off, _ := strconv.ParseBool(env("AUTH_DISABLED", "false")); off {
		u, err := user.Current()
		if err != nil {
			log.Fatal(err)
		}
		login.NoLogin = u.Username
		log.Printf("warning: AUTH_DISABLED is set — no login, anyone who reaches the port reads the transcripts and gets a shell as %s", u.Username)
	}

	// Shells run as the server's user; logging out ends them.
	shells := &shell.Manager{Env: shellEnv()}
	login.OnLogout = shells.StopAll

	mux := http.NewServeMux()
	login.Register(mux)
	(&api.Handler{Catalog: catalog.New(cfg)}).Register(mux)
	shells.Register(mux)
	// The single-page app: bundle files, else index.html for client routes.
	// It 404s /health and /api/* paths no pattern above matched.
	mux.Handle("/", web.Handler(web.Dist))

	// Transcripts hold source code, prompts and secrets: listen on loopback
	// unless HOST says otherwise.
	addr := net.JoinHostPort(env("HOST", "127.0.0.1"), env("PORT", "8767"))
	if _, err := web.Dist.Open("index.html"); err != nil {
		log.Printf("warning: the embedded web bundle has no index.html — run `make web` and rebuild")
	}
	log.Printf("listening on http://%s", addr)
	log.Printf("reading Claude Code %s, Cursor %s, Codex %s, pi %s, OpenCode %s, Kilo %s",
		cfg.ClaudeDir, cfg.CursorDir, cfg.CodexDir, cfg.PiDir, cfg.OpenCodeDB, cfg.KiloDB)
	srv := &http.Server{
		Addr:              addr,
		Handler:           logRequests(gzipJSON(login.Guard(mux))),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

func shellEnv() []string {
	var out []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if !slices.Contains(serverVars, name) {
			out = append(out, kv)
		}
	}
	return out
}

// gzipJSON compresses API responses (a long conversation is megabytes of JSON),
// not WebSocket upgrades.
func gzipJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") || !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") || r.Header.Get("Upgrade") != "" {
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

// Unwrap lets the WebSocket upgrade reach the connection (http.Hijacker).
func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }

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
