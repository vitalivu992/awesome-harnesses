package web

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// Handler serves the single-page app from dist: files that exist (assets/,
// agent-icons/, …) as-is, and index.html for every client-side route.
// /health and /api/* never fall through to the app: an API path no other mux
// pattern matched (unknown path or wrong method) gets a plain 404.
func Handler(dist fs.FS) http.Handler {
	files := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		if isAppRoute(dist, r.URL.Path) {
			// "/" makes the file server serve index.html (asking for
			// /index.html itself would redirect to "/").
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		files.ServeHTTP(w, r)
	})
}

// isAppRoute reports whether urlPath is a router path rather than a file: it is
// not in dist and has no extension. A missing
// asset such as /assets/x.js stays a 404 instead of returning HTML.
func isAppRoute(dist fs.FS, urlPath string) bool {
	name := strings.TrimPrefix(path.Clean("/"+urlPath), "/")
	if name == "" {
		return false
	}
	if _, err := fs.Stat(dist, name); err == nil {
		return false
	}
	return path.Ext(name) == ""
}
