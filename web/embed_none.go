//go:build !ui

package web

import "io/fs"

// Files reports ErrNotBuilt: this program was built without the "ui" tag.
func Files() (fs.FS, error) {
	return nil, ErrNotBuilt
}
