package client

import (
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

// The client form offers its own copy of the kinds of client, GST registration types and
// state codes (web/src/clientLists.ts). This keeps that copy equal to the lists the
// engine checks against, so the form never offers a value the engine refuses.
func TestScreensUseTheSameClientLists(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "web", "src", "clientLists.ts"))
	if err != nil {
		t.Fatal(err)
	}
	pair := regexp.MustCompile(`\['([^']*)', '([^']*)'\]`)
	list := func(name string) (codes []string, labels map[string]string) {
		t.Helper()
		m := regexp.MustCompile(`(?s)export const ` + name + `: \[string, string\]\[\] = \[(.*?)\n\]`).FindSubmatch(src)
		if m == nil {
			t.Fatalf("web/src/clientLists.ts has no %s list", name)
		}
		labels = map[string]string{}
		for _, p := range pair.FindAllSubmatch(m[1], -1) {
			codes = append(codes, string(p[1]))
			labels[string(p[1])] = string(p[2])
		}
		return codes, labels
	}
	if got, _ := list("constitutions"); !slices.Equal(got, Constitutions) {
		t.Errorf("constitutions = %q, want %q (internal/client)", got, Constitutions)
	}
	if got, _ := list("registrationTypes"); !slices.Equal(got, RegistrationTypes) {
		t.Errorf("registrationTypes = %q, want %q (internal/client)", got, RegistrationTypes)
	}
	codes, names := list("states")
	if want := slices.Sorted(maps.Keys(States)); !slices.Equal(codes, want) {
		t.Errorf("state codes = %q, want %q (internal/client)", codes, want)
	}
	for code, name := range States {
		if names[code] != name {
			t.Errorf("state %s: the screen says %q, the engine %q", code, names[code], name)
		}
	}
}
