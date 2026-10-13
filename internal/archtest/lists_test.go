package archtest

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGuardListsAreConsistent checks that the guard's own lists agree with each other, so a
// typo or a rename cannot silently weaken a rule.
func TestGuardListsAreConsistent(t *testing.T) {
	root := moduleRoot(t)
	asImport := func(dir string) string { return module + "/" + filepath.ToSlash(dir) }

	t.Run("server-only packages are all server-side", func(t *testing.T) {
		for _, p := range serverOnlyPatterns {
			if !matchesAny(module+p[1:], serverSidePatterns) {
				t.Errorf("%s is server-only but not in serverSidePatterns", p)
			}
		}
	})
	t.Run("foundation packages are engine-side", func(t *testing.T) {
		for _, p := range sortedKeys(foundationPackages) {
			if !matchesAny(module+p[1:], engineSidePatterns) {
				t.Errorf("%s is a foundation package but not in engineSidePatterns", p)
			}
		}
	})
	t.Run("deterministic and money dirs exist and are engine-side", func(t *testing.T) {
		for name, dirs := range map[string][]string{"deterministicDirs": deterministicDirs, "moneyDirs": moneyDirs} {
			for _, d := range dirs {
				if st, err := os.Stat(filepath.Join(root, d)); err != nil || !st.IsDir() {
					t.Errorf("%s entry %q is not a directory in the module", name, d)
				}
				if !matchesAny(asImport(d), engineSidePatterns) {
					t.Errorf("%s entry %q is not covered by engineSidePatterns", name, d)
				}
			}
		}
	})
	t.Run("money dirs are deterministic dirs", func(t *testing.T) {
		in := map[string]bool{}
		for _, d := range deterministicDirs {
			in[d] = true
		}
		for _, d := range moneyDirs {
			if !in[d] {
				t.Errorf("moneyDirs entry %q is not in deterministicDirs", d)
			}
		}
	})
	t.Run("network-free commands are not in the server list", func(t *testing.T) {
		for _, c := range networkFreeCommands {
			if matchesAny(module+c[1:], serverSidePatterns) {
				t.Errorf("%s must not be server-side", c)
			}
		}
	})
}
