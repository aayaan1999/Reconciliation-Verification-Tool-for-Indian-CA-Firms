package archtest

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestIsNetworkPackage(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"net", true},
		{"net/http", true},
		{"net/netip", true}, // conservative: whole net/ tree is forbidden
		{"net/http/httptrace", true},
		{"crypto/tls", true},
		{"crypto/tls/fipsonly", true},
		{"vendor/golang.org/x/net/http2", true},
		{"golang.org/x/net/context", true},
		{"network", false},
		{"netstuff", false},
		{"github.com/x/net", false},
		{"github.com/x/net/http", false},
		{"crypto/tlsx", false},
		{"crypto/x509", false},
		{"", false},
		{"Net", false},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			if got := isNetworkPackage(c.path); got != c.want {
				t.Errorf("isNetworkPackage(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}

func g(edges map[string][]string) map[string]pkg {
	m := map[string]pkg{}
	for k, v := range edges {
		m[k] = pkg{ImportPath: k, Imports: v}
	}
	return m
}

func isNet(p string) bool { return p == "net" }

func TestFindChain(t *testing.T) {
	t.Run("direct import", func(t *testing.T) {
		got := findChain(g(map[string][]string{"a": {"net"}}), "a", isNet)
		if want := []string{"a", "net"}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %v want %v", got, want)
		}
	})
	t.Run("shortest of several paths", func(t *testing.T) {
		graph := g(map[string][]string{
			"a": {"b", "x"}, "b": {"c"}, "c": {"d"}, "d": {"net"}, "x": {"net"},
		})
		got := findChain(graph, "a", isNet)
		if want := []string{"a", "x", "net"}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %v want %v", got, want)
		}
	})
	t.Run("multi hop", func(t *testing.T) {
		graph := g(map[string][]string{"a": {"b"}, "b": {"c"}, "c": {"net"}})
		got := findChain(graph, "a", isNet)
		if want := []string{"a", "b", "c", "net"}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %v want %v", got, want)
		}
	})
	t.Run("no match returns nil", func(t *testing.T) {
		if got := findChain(g(map[string][]string{"a": {"b"}, "b": nil}), "a", isNet); got != nil {
			t.Errorf("got %v want nil", got)
		}
	})
	t.Run("start missing from graph", func(t *testing.T) {
		if got := findChain(g(nil), "a", isNet); got != nil {
			t.Errorf("got %v want nil", got)
		}
	})
	t.Run("import missing from graph terminates", func(t *testing.T) {
		if got := findChain(g(map[string][]string{"a": {"ghost"}}), "a", isNet); got != nil {
			t.Errorf("got %v want nil", got)
		}
	})
	t.Run("cycle without match terminates", func(t *testing.T) {
		graph := g(map[string][]string{"a": {"b"}, "b": {"c"}, "c": {"a", "b"}})
		if got := findChain(graph, "a", isNet); got != nil {
			t.Errorf("got %v want nil", got)
		}
	})
	t.Run("cycle with match beyond it", func(t *testing.T) {
		graph := g(map[string][]string{"a": {"b"}, "b": {"a", "net"}})
		got := findChain(graph, "a", isNet)
		if want := []string{"a", "b", "net"}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %v want %v", got, want)
		}
	})
	t.Run("self import cycle", func(t *testing.T) {
		if got := findChain(g(map[string][]string{"a": {"a"}}), "a", isNet); got != nil {
			t.Errorf("got %v want nil", got)
		}
	})
	t.Run("start itself matching is not reported", func(t *testing.T) {
		if got := findChain(g(map[string][]string{"net": {"b"}}), "net", isNet); got != nil {
			t.Errorf("got %v want nil", got)
		}
	})
	t.Run("start matching but imports a match", func(t *testing.T) {
		graph := g(map[string][]string{"net": {"net/x"}})
		got := findChain(graph, "net", func(p string) bool { return strings.HasPrefix(p, "net") })
		if want := []string{"net", "net/x"}; !reflect.DeepEqual(got, want) {
			t.Errorf("got %v want %v", got, want)
		}
	})
	t.Run("deterministic with equal length chains", func(t *testing.T) {
		graph := g(map[string][]string{
			"a": {"z", "m", "b"}, "z": {"net"}, "m": {"net"}, "b": {"net"},
		})
		want := []string{"a", "b", "net"}
		for i := 0; i < 50; i++ {
			if got := findChain(graph, "a", isNet); !reflect.DeepEqual(got, want) {
				t.Fatalf("iteration %d: got %v want %v", i, got, want)
			}
		}
	})
	t.Run("does not mutate graph imports order", func(t *testing.T) {
		graph := g(map[string][]string{"a": {"z", "b"}, "b": nil, "z": nil})
		findChain(graph, "a", isNet)
		if !reflect.DeepEqual(graph["a"].Imports, []string{"z", "b"}) {
			t.Errorf("imports were reordered: %v", graph["a"].Imports)
		}
	})
}

func writeSrc(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "x.go")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestClockOrRandomUses(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int    // expected number of problems
		gap  string // non-empty: real violation not caught today
	}{
		{"plain time.Now", "package x\nimport \"time\"\nvar _ = time.Now()\n", 1, ""},
		{"time.Now as value", "package x\nimport \"time\"\nvar f = time.Now\n", 1, ""},
		{"aliased time.Now", "package x\nimport tm \"time\"\nvar _ = tm.Now()\n", 1, ""},
		{"math/rand", "package x\nimport \"math/rand\"\nvar _ = rand.Int()\n", 1, ""},
		{"math/rand/v2", "package x\nimport \"math/rand/v2\"\nvar _ = rand.Int()\n", 1, ""},
		{"crypto/rand", "package x\nimport \"crypto/rand\"\nvar _ = rand.Reader\n", 1, ""},
		{"blank math/rand", "package x\nimport _ \"math/rand\"\n", 1, ""},
		{"time only for Duration", "package x\nimport \"time\"\nvar _ = time.Second\nvar _ time.Duration\n", 0, ""},
		{"blank time import", "package x\nimport _ \"time\"\n", 0, ""},
		{"clean file", "package x\nvar _ = 1\n", 0, ""},
		{"Now on other identifier", "package x\ntype c struct{}\nfunc (c) Now() int { return 0 }\nvar _ = c{}.Now()\n", 0, ""},
		{"dot import time", "package x\nimport . \"time\"\nvar _ = Now()\n", 1, ""},
		{"time.Since", "package x\nimport \"time\"\nvar _ = time.Since(time.Time{})\n", 1, ""},
		{"time.Until", "package x\nimport \"time\"\nvar _ = time.Until(time.Time{})\n", 1, ""},
		{"time.Sleep", "package x\nimport \"time\"\nfunc f() { time.Sleep(1) }\n", 1, ""},
		{"time.Tick", "package x\nimport \"time\"\nvar _ = time.Tick(1)\n", 1, ""},
		{"time.After", "package x\nimport \"time\"\nvar _ = time.After(1)\n", 1, ""},
		{"time.AfterFunc", "package x\nimport \"time\"\nvar _ = time.AfterFunc(1, nil)\n", 1, ""},
		{"time.NewTimer", "package x\nimport \"time\"\nvar _ = time.NewTimer(1)\n", 1, ""},
		{"time.NewTicker", "package x\nimport \"time\"\nvar _ = time.NewTicker(1)\n", 1, ""},
		{"aliased time.Since", "package x\nimport tm \"time\"\nvar _ = tm.Since(tm.Time{})\n", 1, ""},
		{"time.Date is allowed", "package x\nimport \"time\"\nvar _ = time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)\n", 0, ""},
		{"hash/maphash", "package x\nimport \"hash/maphash\"\nvar _ maphash.Seed\n", 1, ""},
		{"cgo", "package x\nimport \"C\"\n", 1, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := clockOrRandomUses(t, writeSrc(t, c.src))
			if c.gap != "" {
				if len(got) != c.want {
					t.Skip("known gap: " + c.gap)
				}
				return
			}
			if len(got) != c.want {
				t.Errorf("got %d problems %v, want %d", len(got), got, c.want)
			}
		})
	}

	t.Run("problem text names the cause", func(t *testing.T) {
		src := "package x\nimport (\n\"math/rand\"\n\"time\"\n)\nvar _, _ = rand.Int(), time.Now()\n"
		got := clockOrRandomUses(t, writeSrc(t, src))
		want := []string{"imports math/rand", "calls time.Now"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v want %v", got, want)
		}
	})
	t.Run("two calls give two problems", func(t *testing.T) {
		got := clockOrRandomUses(t, writeSrc(t, "package x\nimport \"time\"\nvar _, _ = time.Now(), time.Now()\n"))
		if len(got) != 2 {
			t.Errorf("got %v", got)
		}
	})
}

func TestGoSourceFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(rel string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("package x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"b.go", "a.go", "a_test.go", "notes.txt", "sub/z.go", "sub/z_test.go", "sub/deep/c.go", "go.go.bak"} {
		write(f)
	}
	var rel []string
	for _, f := range goSourceFiles(t, dir) {
		r, _ := filepath.Rel(dir, f)
		rel = append(rel, filepath.ToSlash(r))
	}
	want := []string{"a.go", "b.go", "sub/deep/c.go", "sub/z.go"}
	if !reflect.DeepEqual(rel, want) {
		t.Errorf("got %v want %v", rel, want)
	}

	t.Run("missing directory", func(t *testing.T) {
		if got := goSourceFiles(t, filepath.Join(dir, "nope")); len(got) != 0 {
			t.Errorf("got %v", got)
		}
	})
	t.Run("empty directory", func(t *testing.T) {
		if got := goSourceFiles(t, t.TempDir()); len(got) != 0 {
			t.Errorf("got %v", got)
		}
	})
}

func TestSortedKeysAndDescribeAllowed(t *testing.T) {
	if got := sortedKeys(map[string]int{"b": 1, "a": 2, "c": 3}); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("sortedKeys = %v", got)
	}
	if got := sortedKeys(map[string]int{}); len(got) != 0 {
		t.Errorf("sortedKeys(empty) = %v", got)
	}
	if got := describeAllowed(nil); got != "" {
		t.Errorf("describeAllowed(nil) = %q", got)
	}
	if got := describeAllowed([]string{"p", "q"}); got != " and p, q" {
		t.Errorf("describeAllowed = %q", got)
	}
}

func TestMatchPattern(t *testing.T) {
	m := func(p string) string { return module + p }
	cases := []struct {
		name, path, pattern string
		want                bool
	}{
		{"exact match", m("/internal/money"), "./internal/money", true},
		{"exact does not match child", m("/internal/money/sub"), "./internal/money", false},
		{"exact does not match shared-prefix sibling", m("/internal/moneyx"), "./internal/money", false},
		{"exact does not match parent", m("/internal"), "./internal/money", false},
		{"exact does not match other module", "example.com/internal/money", "./internal/money", false},
		{"dots matches base", m("/internal/modules"), "./internal/modules/...", true},
		{"dots matches child", m("/internal/modules/gst"), "./internal/modules/...", true},
		{"dots matches grandchild", m("/internal/modules/gst/x"), "./internal/modules/...", true},
		{"dots does not match shared-prefix sibling", m("/internal/modulesx"), "./internal/modules/...", false},
		{"dots does not match child of sibling", m("/internal/modulesx/y"), "./internal/modules/...", false},
		{"dots does not match parent", m("/internal"), "./internal/modules/...", false},
		{"module root dots matches everything", m("/internal/x"), "./...", true},
		{"module root dots matches root", module, "./...", true},
		{"module root dots rejects foreign path", "example.com/x", "./...", false},
		{"empty import path", "", "./internal/money", false},
		// Documented behaviour: a pattern lacking the leading "./" becomes module+"internal/money"
		// (no slash) so it can never match a real package. Patterns must use "./".
		{"pattern without dot-slash never matches", m("/internal/money"), "internal/money", false},
		// Documented quirk: only a leading "." is trimmed, so "/internal/money" behaves like "./internal/money".
		{"pattern with leading slash behaves like dot-slash", m("/internal/money"), "/internal/money", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := matchPattern(c.path, c.pattern); got != c.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", c.path, c.pattern, got, c.want)
			}
		})
	}
}

func TestMatchesAnyAndCountMatches(t *testing.T) {
	m := func(p string) string { return module + p }
	pats := []string{"./internal/money", "./internal/modules/..."}
	t.Run("matches first", func(t *testing.T) {
		if !matchesAny(m("/internal/money"), pats) {
			t.Error("want true")
		}
	})
	t.Run("matches last via dots", func(t *testing.T) {
		if !matchesAny(m("/internal/modules/gst"), pats) {
			t.Error("want true")
		}
	})
	t.Run("matches none", func(t *testing.T) {
		if matchesAny(m("/internal/moneyx"), pats) {
			t.Error("want false")
		}
	})
	t.Run("empty list", func(t *testing.T) {
		if matchesAny(m("/internal/money"), nil) || matchesAny(m("/internal/money"), []string{}) {
			t.Error("want false for empty list")
		}
	})
	t.Run("countMatches", func(t *testing.T) {
		pkgs := []pkg{
			{ImportPath: m("/internal/modules")},
			{ImportPath: m("/internal/modules/gst")},
			{ImportPath: m("/internal/modules/tds")},
			{ImportPath: m("/internal/modulesx")},
			{ImportPath: m("/internal/money")},
			{ImportPath: m("/internal/money/sub")},
		}
		for pattern, want := range map[string]int{
			"./internal/modules/...": 3,
			"./internal/money":       1,
			"./internal/money/...":   2,
			"./internal/nothing":     0,
			"./...":                  6,
			"internal/money":         0,
		} {
			if got := countMatches(pkgs, pattern); got != want {
				t.Errorf("countMatches(%q) = %d, want %d", pattern, got, want)
			}
		}
		if got := countMatches(nil, "./..."); got != 0 {
			t.Errorf("countMatches(nil) = %d", got)
		}
	})
}

func TestTargetStringAndEnv(t *testing.T) {
	tg := target{"linux", "arm64"}
	if got := tg.String(); got != "linux/arm64" {
		t.Errorf("String() = %q", got)
	}
	t.Run("targets list", func(t *testing.T) {
		var names []string
		for _, x := range targets {
			names = append(names, x.String())
		}
		if want := []string{"windows/amd64", "linux/amd64"}; !reflect.DeepEqual(names, want) {
			t.Errorf("targets = %v want %v", names, want)
		}
	})
	t.Run("env contains overrides", func(t *testing.T) {
		env := tg.env()
		for _, want := range []string{"GOOS=linux", "GOARCH=arm64", "CGO_ENABLED=0"} {
			found := false
			for _, e := range env {
				if e == want {
					found = true
				}
			}
			if !found {
				t.Errorf("env missing %s", want)
			}
		}
	})
	t.Run("overrides win over inherited values", func(t *testing.T) {
		t.Setenv("GOOS", "plan9")
		t.Setenv("GOARCH", "386")
		t.Setenv("CGO_ENABLED", "1")
		env := tg.env()
		last := map[string]string{}
		for _, e := range env {
			if k, v, ok := strings.Cut(e, "="); ok {
				switch k {
				case "GOOS", "GOARCH", "CGO_ENABLED":
					last[k] = v // the last entry wins for exec.Cmd.Env
				}
			}
		}
		want := map[string]string{"GOOS": "linux", "GOARCH": "arm64", "CGO_ENABLED": "0"}
		if !reflect.DeepEqual(last, want) {
			t.Errorf("effective env = %v want %v", last, want)
		}
		tail := env[len(env)-3:]
		if !reflect.DeepEqual(tail, []string{"GOOS=linux", "GOARCH=arm64", "CGO_ENABLED=0"}) {
			t.Errorf("tail = %v", tail)
		}
	})
	t.Run("does not modify process environment", func(t *testing.T) {
		t.Setenv("GOOS", "plan9")
		_ = tg.env()
		if os.Getenv("GOOS") != "plan9" {
			t.Error("env() changed the process environment")
		}
	})
}

func TestTouchModuleInputs(t *testing.T) {
	put := func(t *testing.T, root, rel, body string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("go.mod only, no go.sum", func(t *testing.T) {
		root := t.TempDir()
		put(t, root, "go.mod", "module x\n")
		if err := touchModuleInputs(root); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("go.mod and go.sum", func(t *testing.T) {
		root := t.TempDir()
		put(t, root, "go.mod", "module x\n")
		put(t, root, "go.sum", "")
		if err := touchModuleInputs(root); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("missing go.mod is tolerated", func(t *testing.T) {
		root := t.TempDir()
		put(t, root, "a.go", "package x\n")
		// Documented behaviour: IsNotExist is ignored for go.mod as well as go.sum.
		if err := touchModuleInputs(root); err != nil {
			t.Errorf("behaviour changed, error: %v", err)
		}
	})
	t.Run("nested dirs and skipped dirs with files", func(t *testing.T) {
		root := t.TempDir()
		put(t, root, "go.mod", "module x\n")
		put(t, root, "a.go", "package x\n")
		put(t, root, "internal/a/b/c.go", "package c\n")
		put(t, root, ".git/hooks/x.go", "package x\n")
		put(t, root, "build/gen.go", "package x\n")
		put(t, root, "web/node_modules/p/i.go", "package x\n")
		put(t, root, "notes.txt", "x")
		if err := touchModuleInputs(root); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("non-existent root", func(t *testing.T) {
		if err := touchModuleInputs(filepath.Join(t.TempDir(), "nope")); err == nil {
			t.Error("want error for non-existent root")
		}
	})
	t.Run("broken go symlink errors in normal dirs and is ignored in skipped dirs", func(t *testing.T) {
		root := t.TempDir()
		put(t, root, "go.mod", "module x\n")
		for _, d := range []string{".git", "build", "node_modules", "sub/.hidden", "sub/node_modules"} {
			if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(root, "missing-target"), filepath.Join(root, filepath.FromSlash(d), "broken.go")); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
		}
		if err := touchModuleInputs(root); err != nil {
			t.Fatalf("skipped dirs were read: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, "missing-target"), filepath.Join(root, "pkg", "broken.go")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		if err := touchModuleInputs(root); err == nil {
			t.Error("want error: broken symlink in a normal dir should be read")
		}
	})
}

func TestClockOrRandomUsesMore(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{"Sleep inside function body",
			"package x\nimport \"time\"\nfunc f() {\n\tfor i := 0; i < 3; i++ {\n\t\ttime.Sleep(time.Second)\n\t}\n}\n",
			[]string{"calls time.Sleep"}},
		{"mixed clock calls counted in source order",
			"package x\nimport \"time\"\nfunc f() {\n\t_ = time.Now()\n\ttime.Sleep(1)\n\t_ = time.Since(time.Time{})\n\t_ = time.NewTicker(1)\n}\n",
			[]string{"calls time.Now", "calls time.Sleep", "calls time.Since", "calls time.NewTicker"}},
		{"same call three times",
			"package x\nimport \"time\"\nfunc f() {\n\t_ = time.Now()\n\t_ = time.Now()\n\t_ = time.Now()\n}\n",
			[]string{"calls time.Now", "calls time.Now", "calls time.Now"}},
		{"time not imported: field Now on variable named time",
			"package x\ntype t struct{ Now int }\nfunc f() int {\n\ttime := t{}\n\treturn time.Now\n}\n",
			nil},
		{"time not imported: method Sleep on variable named time",
			"package x\ntype t struct{}\nfunc (t) Sleep() {}\nfunc f() {\n\ttime := t{}\n\ttime.Sleep()\n}\n",
			nil},
		{"aliased import: unaliased name is not flagged",
			"package x\nimport tm \"time\"\ntype t struct{ Now int }\nfunc f() int {\n\ttime := t{}\n\t_ = tm.Second\n\treturn time.Now\n}\n",
			nil},
		{"cgo import with preamble",
			"package x\n\n/*\n#include <stdlib.h>\n*/\nimport \"C\"\n",
			[]string{"imports C"}},
		{"cgo plus rand plus clock",
			"package x\nimport (\n\t\"C\"\n\t\"crypto/rand\"\n\t\"time\"\n)\nvar _, _ = rand.Reader, time.Now()\n",
			[]string{"imports C", "imports crypto/rand", "calls time.Now"}},
		{"dot import reports once and no calls",
			"package x\nimport . \"time\"\nvar _ = Now()\nvar _ = Sleep\n",
			[]string{`dot-imports "time"`}},
		{"time.Tick inside closure",
			"package x\nimport \"time\"\nvar f = func() { _ = time.Tick(1) }\n",
			[]string{"calls time.Tick"}},
		{"time.Time methods are allowed",
			"package x\nimport \"time\"\nfunc f(a, b time.Time) bool { return a.Before(b) || a.Equal(b) }\n",
			nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := clockOrRandomUses(t, writeSrc(t, c.src))
			if len(got) == 0 && len(c.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestFloatUses(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{"clean integer code", "package x\nvar a int64 = 100\nvar b = a * 18 / 100\n", 0},
		{"float64 type", "package x\nvar a float64\n", 1},
		{"float32 conversion", "package x\nvar a = float32(1)\n", 1},
		{"complex128 type", "package x\nvar a complex128\n", 1},
		{"decimal literal", "package x\nvar a = 0.18\n", 1},
		{"exponent literal", "package x\nvar a = 1e5\n", 1},
		{"imaginary literal", "package x\nvar a = 2i\n", 1},
		{"strconv.ParseFloat", "package x\nimport \"strconv\"\nvar a, _ = strconv.ParseFloat(\"1\", 64)\n", 1},
		{"aliased strconv.FormatFloat", "package x\nimport sc \"strconv\"\nvar a = sc.FormatFloat\n", 1},
		{"math.Float64frombits", "package x\nimport \"math\"\nvar a = math.Float64frombits\n", 1},
		{"math.Pow with integer arguments", "package x\nimport \"math\"\nvar a = int64(math.Pow(10, 2))\n", 1},
		{"math.Round", "package x\nimport \"math\"\nfunc f(n int64) int64 { return int64(math.Round(float64(n))) }\n", 2},
		{"aliased math.Floor", "package x\nimport m \"math\"\nvar a = m.Floor\n", 1},
		{"math.MaxFloat64", "package x\nimport \"math\"\nvar a = math.MaxFloat64\n", 1},
		{"big.NewFloat", "package x\nimport \"math/big\"\nvar a = big.NewFloat\n", 1},
		{"big.Int is fine", "package x\nimport \"math/big\"\nvar a = big.NewInt(1)\n", 0},
		{"json.Number.Float64", "package x\nimport \"encoding/json\"\nvar n json.Number\nvar a, _ = n.Float64()\n", 1},
		{"Rat.Float64 method value", "package x\nimport \"math/big\"\nvar a = new(big.Rat).Float64\n", 1},
		{"fmt %.2f", "package x\nimport \"fmt\"\nvar a = fmt.Sprintf(\"%.2f\", 1)\n", 1},
		{"fmt %g and %e", "package x\nimport \"fmt\"\nvar a = fmt.Sprintf(\"%g %e\", 1, 2)\n", 1},
		{"fmt.Sscanf %f", "package x\nimport \"fmt\"\nvar n int\nvar _, _ = fmt.Sscanf(\"1\", \"%f\", &n)\n", 1},
		{"aliased fmt %8.3F", "package x\nimport f \"fmt\"\nvar a = f.Sprintf(\"%8.3F\", 1)\n", 1},
		{"fmt %d %s %v %q and %%f are fine", "package x\nimport \"fmt\"\nvar a = fmt.Sprintf(\"%d %s %v %q %%f\", 1, \"x\", 2, \"y\")\n", 0},
		{"%f outside a fmt call is fine", "package x\nvar a = \"rate %f\"\n", 0},
		{"strconv.Itoa is fine", "package x\nimport \"strconv\"\nvar a = strconv.Itoa(5)\n", 0},
		{"math.MaxInt64 is fine", "package x\nimport \"math\"\nvar a = math.MaxInt64\n", 0},
		{"word float in a comment is fine", "package x\n// never use float64 here\nvar a = 1\n", 0},
		{"float64 in a string is fine", "package x\nvar a = \"float64\"\n", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := floatUses(t, writeSrc(t, c.src)); len(got) != c.want {
				t.Errorf("got %d problems %v, want %d", len(got), got, c.want)
			}
		})
	}
	t.Run("problem names the line", func(t *testing.T) {
		got := floatUses(t, writeSrc(t, "package x\n\nvar a = 0.5\n"))
		if len(got) != 1 || got[0] != "line 3: decimal number 0.5" {
			t.Errorf("got %v", got)
		}
	})
}
