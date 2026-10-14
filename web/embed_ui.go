//go:build ui

package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Files returns the built UI, rooted at web/dist (so "index.html" is at the top).
func Files() (fs.FS, error) {
	return fs.Sub(dist, "dist")
}
