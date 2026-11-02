package archtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestEveryLinkedModuleHasALicenceRecord fails when a module linked into any program under
// cmd/ is not listed, with its version, in docs/licences.md (SPEC-002 AC-6), so no library
// reaches the firm without its licence being recorded and reviewed.
func TestEveryLinkedModuleHasALicenceRecord(t *testing.T) {
	root := moduleRoot(t)
	record, err := os.ReadFile(filepath.Join(root, "docs", "licences.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tg := range targets {
		cmd := exec.Command(goTool(t), "list", "-deps", "-f", "{{if .Module}}{{.Module.Path}} {{.Module.Version}}{{end}}", "./cmd/...")
		cmd.Dir = root
		cmd.Env = tg.env()
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("[%s] go list: %v", tg, err)
		}
		seen := map[string]bool{}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			path, version, _ := strings.Cut(strings.TrimSpace(line), " ")
			if path == "" || path == module || seen[path] {
				continue
			}
			seen[path] = true
			if !strings.Contains(string(record), "| `"+path+"` | "+version+" |") {
				t.Errorf("[%s] %s %s is linked into a program but not recorded in docs/licences.md", tg, path, version)
			}
		}
	}
}
