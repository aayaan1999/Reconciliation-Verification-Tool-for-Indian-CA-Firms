package rules

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

// The screens check their text against their own copy of the banned phrases
// (web/src/wording.ts). This keeps that copy equal to BannedPhrases.
func TestScreensUseTheSameBannedPhrases(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "wording.ts"))
	if err != nil {
		t.Fatal(err)
	}
	list := regexp.MustCompile(`bannedPhrases = \[([^\]]*)\]`).FindSubmatch(src)
	if list == nil {
		t.Fatal("web/src/wording.ts has no bannedPhrases list")
	}
	var got []string
	for _, m := range regexp.MustCompile(`'([^']*)'`).FindAllSubmatch(list[1], -1) {
		got = append(got, string(m[1]))
	}
	if !slices.Equal(got, BannedPhrases) {
		t.Errorf("web/src/wording.ts bannedPhrases = %q, want %q (internal/rules/wording.go)", got, BannedPhrases)
	}
}

func TestBannedIn(t *testing.T) {
	for text, want := range map[string][]string{
		"The figure Should agree":         {"should"},
		"MUST  BE equal":                  {"must be"},
		"shoulder of the road":            nil,
		"Depreciation mismatch, 1,12,450": nil,
	} {
		if got := BannedIn(text); !slices.Equal(got, want) {
			t.Errorf("BannedIn(%q) = %q, want %q", text, got, want)
		}
	}
}
