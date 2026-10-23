// Package web embeds the built Vite bundle so the server is a single binary
// with no STATIC_ROOT or dist/ on disk. Run `make web` (pnpm build) before
// `go build` to (re)generate web/dist; the committed web/dist/.gitkeep keeps
// the embed pattern valid on a fresh clone.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist is the built web bundle (index.html, assets/, agent-icons/) rooted
// at the bundle top level; serve it with Handler.
var Dist fs.FS

func init() {
	var err error
	Dist, err = fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
}
