package web

import (
	"errors"
	"io/fs"
	"regexp"
	"testing"
)

// externalRef matches an HTML src or href that loads from the internet.
var externalRef = regexp.MustCompile(`(?i)(src|href)\s*=\s*["']?\s*(https?:)?//`)

// TestFiles checks both builds. Without the "ui" tag Files reports ErrNotBuilt. With it,
// index.html is present and loads nothing from the internet, because the tool runs offline.
func TestFiles(t *testing.T) {
	files, err := Files()
	if errors.Is(err, ErrNotBuilt) {
		if files != nil {
			t.Error("Files returned a file system together with ErrNotBuilt")
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	index, err := fs.ReadFile(files, "index.html")
	if err != nil {
		t.Fatalf("the embedded UI has no index.html: %v", err)
	}
	if m := externalRef.Find(index); m != nil {
		t.Errorf("index.html loads from the internet: %s", m)
	}
}
