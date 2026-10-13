package archtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
)

const module = "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms"

// target is a GOOS/GOARCH pair the guard evaluates. Windows comes first because it is
// what ships to the firm; Linux is checked too so OS-specific files cannot hide anywhere.
// Cgo is disabled to match the shipped build and to exclude cgo network resolvers.
type target struct{ goos, goarch string }

var targets = []target{{"windows", "amd64"}, {"linux", "amd64"}}

func (tg target) String() string { return tg.goos + "/" + tg.goarch }

func (tg target) env() []string {
	return append(os.Environ(), "GOOS="+tg.goos, "GOARCH="+tg.goarch, "CGO_ENABLED=0")
}

// networkFreeCommands must never be able to open a network connection
// (CLAUDE.md invariant 5; technical spec §A3, §A4.1).
var networkFreeCommands = []string{
	"./cmd/recon-engine",
	"./cmd/recon-updater",
}

// isNetworkPackage reports whether an import path gives network capability.
func isNetworkPackage(path string) bool {
	return path == "net" || strings.HasPrefix(path, "net/") ||
		path == "crypto/tls" || strings.HasPrefix(path, "crypto/tls/") ||
		strings.HasPrefix(path, "vendor/golang.org/x/net/") ||
		strings.HasPrefix(path, "golang.org/x/net/")
}

// Every package in the module must match exactly one of the three lists below
// (TestEveryPackageIsClassified), so a new package can never escape the guard.

// engineSidePatterns are the packages that run inside recon-engine.
var engineSidePatterns = []string{
	"./cmd/recon-engine",
	"./internal/money", "./internal/period", "./internal/domain", "./internal/strictyaml", "./internal/vocab",
	"./internal/ingest/...", "./internal/profile", "./internal/rules", "./internal/lawtable",
	"./internal/engine/...", "./internal/starhost", "./internal/findings", "./internal/modules/...",
	"./internal/store", "./internal/vault", "./internal/jobs", "./internal/ipc",
}

// serverSidePatterns may be used by recon-server; engine-side code must never import
// the server-only ones.
var serverSidePatterns = []string{
	"./cmd/recon-server",
	"./internal/api", "./internal/authz", "./internal/export",
	"./internal/pack", "./internal/audit",
}

// serverOnlyPatterns must never be imported by engine-side packages.
var serverOnlyPatterns = []string{"./internal/api", "./internal/authz", "./internal/export"}

// otherPatterns are tools and tests that are neither engine nor server.
var otherPatterns = []string{
	"./cmd/recon-updater", // network-free, checked by networkFreeCommands
	"./cmd/recon-dev",     // developer tools, never shipped
	"./internal/archtest", // this guard
}

// deterministicDirs must not read the clock or use randomness (CLAUDE.md invariant 4).
var deterministicDirs = []string{
	"internal/money", "internal/period", "internal/domain", "internal/strictyaml", "internal/vocab",
	"internal/ingest", "internal/profile", "internal/rules", "internal/lawtable",
	"internal/engine", "internal/starhost", "internal/findings", "internal/modules",
}

// moneyDirs handle or will calculate with money and must never use floating point
// (CLAUDE.md invariant 3; technical spec §A8).
var moneyDirs = []string{
	"internal/money", "internal/domain", "internal/engine", "internal/findings", "internal/profile", "internal/lawtable",
}

// foundationPackages may import only the standard library, plus the listed module packages.
var foundationPackages = map[string][]string{
	"./internal/money":  nil,
	"./internal/period": nil,
	"./internal/domain": {module + "/internal/money", module + "/internal/period"},
}

// pkg is the subset of `go list -json` output these tests use.
type pkg struct {
	ImportPath string
	Standard   bool
	Imports    []string
}

// TestModulePathMatchesGoMod fails if go.mod is renamed without updating this guard,
// which would otherwise make the module-path checks silently match nothing.
func TestModulePathMatchesGoMod(t *testing.T) {
	root := moduleRoot(t)
	cmd := exec.Command(goTool(t), "list", "-m")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -m: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != module {
		t.Fatalf("go.mod module is %q but archtest expects %q; update the module constant", got, module)
	}
}

// TestEveryPackageIsClassified fails if a package belongs to no list, or to more than one,
// and if any pattern in the lists matches no package (a typo or a rename).
func TestEveryPackageIsClassified(t *testing.T) {
	root := moduleRoot(t)
	lists := map[string][]string{"engine-side": engineSidePatterns, "server-side": serverSidePatterns, "other": otherPatterns}
	for _, tg := range targets {
		all := listJSON(t, root, tg, "./...")
		for _, name := range sortedKeys(lists) {
			for _, pattern := range lists[name] {
				if countMatches(all, pattern) == 0 {
					t.Errorf("[%s] %s pattern %q matches no package", tg, name, pattern)
				}
			}
		}
		for _, p := range all {
			var in []string
			for _, name := range sortedKeys(lists) {
				if matchesAny(p.ImportPath, lists[name]) {
					in = append(in, name)
				}
			}
			switch len(in) {
			case 0:
				t.Errorf("[%s] package %s is not classified; add it to engineSidePatterns, serverSidePatterns or otherPatterns in archtest_test.go", tg, p.ImportPath)
			case 1:
			default:
				t.Errorf("[%s] package %s is in more than one list: %s", tg, p.ImportPath, strings.Join(in, ", "))
			}
		}
	}
}

// TestEngineHasNoNetwork fails if a network-free command or any engine-side package links a
// network package, on every target. It writes the full dependency list of recon-engine per
// target to build/engine-deps-<goos>-<goarch>.txt.
func TestEngineHasNoNetwork(t *testing.T) {
	root := moduleRoot(t)
	for _, tg := range targets {
		graph, _ := loadGraph(t, root, tg, "./cmd/recon-engine")
		writeDepsFile(t, root, tg, graph)

		patterns := append(append([]string(nil), networkFreeCommands...), engineSidePatterns...)
		graph, roots := loadGraph(t, root, tg, patterns...)
		for _, r := range roots {
			if chain := findChain(graph, r, isNetworkPackage); chain != nil {
				t.Errorf("[%s] %s must not have network capability, but it imports %s via:\n  %s",
					tg, r, chain[len(chain)-1], strings.Join(chain, "\n  -> "))
			}
		}
	}
}

// TestEngineSideAvoidsServerPackages fails if engine-side code imports a server-only package.
func TestEngineSideAvoidsServerPackages(t *testing.T) {
	root := moduleRoot(t)
	isServerOnly := func(p string) bool { return matchesAny(p, serverOnlyPatterns) }
	for _, tg := range targets {
		graph, roots := loadGraph(t, root, tg, engineSidePatterns...)
		for _, r := range roots {
			if chain := findChain(graph, r, isServerOnly); chain != nil {
				t.Errorf("[%s] engine-side package %s must not import server-only package %s; chain:\n  %s",
					tg, r, chain[len(chain)-1], strings.Join(chain, "\n  -> "))
			}
		}
	}
}

// TestFoundationPackagesImportOnlyStandardLibrary keeps money, period and domain free of
// outside dependencies.
func TestFoundationPackagesImportOnlyStandardLibrary(t *testing.T) {
	root := moduleRoot(t)
	for _, tg := range targets {
		for _, pattern := range sortedKeys(foundationPackages) {
			allowed := map[string]bool{}
			for _, a := range foundationPackages[pattern] {
				allowed[a] = true
			}
			graph, roots := loadGraph(t, root, tg, pattern)
			for _, r := range roots {
				for _, imp := range graph[r].Imports {
					if p, ok := graph[imp]; ok && !p.Standard && !allowed[imp] {
						t.Errorf("[%s] %s may import only the standard library%s, but imports %s",
							tg, r, describeAllowed(foundationPackages[pattern]), imp)
					}
				}
			}
		}
	}
}

// TestDeterministicPackagesHaveNoClockOrRandomness scans engine-side source (all files,
// whatever their OS build tags) for randomness, cgo and clock reads.
func TestDeterministicPackagesHaveNoClockOrRandomness(t *testing.T) {
	root := moduleRoot(t)
	for _, dir := range deterministicDirs {
		for _, file := range goSourceFiles(t, filepath.Join(root, dir)) {
			for _, problem := range clockOrRandomUses(t, file) {
				rel, _ := filepath.Rel(root, file)
				t.Errorf("%s: %s (results must not depend on the clock or randomness)", filepath.ToSlash(rel), problem)
			}
		}
	}
}

// TestMoneyPackagesUseNoFloatingPoint scans money-handling source for float types,
// float literals and float conversions.
func TestMoneyPackagesUseNoFloatingPoint(t *testing.T) {
	root := moduleRoot(t)
	for _, dir := range moneyDirs {
		for _, file := range goSourceFiles(t, filepath.Join(root, dir)) {
			for _, problem := range floatUses(t, file) {
				rel, _ := filepath.Rel(root, file)
				t.Errorf("%s: %s (money must be integer paise, never floating point)", filepath.ToSlash(rel), problem)
			}
		}
	}
}

// --- helpers ---

func goTool(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("the go command is not on PATH: %v", err)
	}
	return p
}

var (
	rootOnce sync.Once
	rootDir  string
	rootErr  error
)

// moduleRoot returns the module directory. The first call also reads every module input
// (go.mod, go.sum, all .go files) so that `go test` caching re-runs this guard whenever
// any of them changes; `go list` runs in a child process, which caching cannot see.
func moduleRoot(t *testing.T) string {
	t.Helper()
	rootOnce.Do(func() {
		out, err := exec.Command(goTool(t), "env", "GOMOD").Output()
		if err != nil {
			rootErr = fmt.Errorf("go env GOMOD: %v", err)
			return
		}
		gomod := strings.TrimSpace(string(out))
		if gomod == "" || gomod == os.DevNull {
			rootErr = fmt.Errorf("not inside the Go module")
			return
		}
		rootDir = filepath.Dir(gomod)
		rootErr = touchModuleInputs(rootDir)
	})
	if rootErr != nil {
		t.Fatal(rootErr)
	}
	return rootDir
}

func touchModuleInputs(root string) error {
	for _, name := range []string{"go.mod", "go.sum"} {
		if _, err := os.ReadFile(filepath.Join(root, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "build" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			_, err = os.ReadFile(path)
		}
		return err
	})
}

// matchesAny reports whether importPath matches one of the ./-relative patterns.
func matchesAny(importPath string, patterns []string) bool {
	for _, p := range patterns {
		if matchPattern(importPath, p) {
			return true
		}
	}
	return false
}

func matchPattern(importPath, pattern string) bool {
	full := module + strings.TrimPrefix(pattern, ".")
	if base, ok := strings.CutSuffix(full, "/..."); ok {
		return importPath == base || strings.HasPrefix(importPath, base+"/")
	}
	return importPath == full
}

func countMatches(pkgs []pkg, pattern string) int {
	n := 0
	for _, p := range pkgs {
		if matchPattern(p.ImportPath, pattern) {
			n++
		}
	}
	return n
}

// loadGraph returns every package reachable from patterns on target tg, and the import
// paths of the packages the patterns themselves name.
func loadGraph(t *testing.T, root string, tg target, patterns ...string) (map[string]pkg, []string) {
	t.Helper()
	graph := map[string]pkg{}
	for _, p := range listJSON(t, root, tg, append([]string{"-deps"}, patterns...)...) {
		graph[p.ImportPath] = p
	}
	var roots []string
	for _, p := range listJSON(t, root, tg, patterns...) {
		roots = append(roots, p.ImportPath)
	}
	sort.Strings(roots)
	return graph, roots
}

var (
	listMu    sync.Mutex
	listCache = map[string][]pkg{}
)

// listJSON runs `go list -json` for target tg, caching results within one test run.
func listJSON(t *testing.T, root string, tg target, args ...string) []pkg {
	t.Helper()
	key := tg.String() + "\x00" + strings.Join(args, "\x00")
	listMu.Lock()
	defer listMu.Unlock()
	if cached, ok := listCache[key]; ok {
		return cached
	}
	cmd := exec.Command(goTool(t), append([]string{"list", "-json=ImportPath,Standard,Imports"}, args...)...)
	cmd.Dir = root
	cmd.Env = tg.env()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("[%s] go list %v: %v\n%s", tg, args, err, stderr.String())
	}
	var pkgs []pkg
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var p pkg
		if err := dec.Decode(&p); err != nil {
			t.Fatalf("decoding go list output: %v", err)
		}
		pkgs = append(pkgs, p)
	}
	listCache[key] = pkgs
	return pkgs
}

// findChain returns the shortest import chain from start to a package matching bad, or nil.
func findChain(graph map[string]pkg, start string, bad func(string) bool) []string {
	prev := map[string]string{start: ""}
	queue := []string{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur != start && bad(cur) {
			var chain []string
			for p := cur; p != ""; p = prev[p] {
				chain = append([]string{p}, chain...)
			}
			return chain
		}
		imports := append([]string(nil), graph[cur].Imports...)
		sort.Strings(imports)
		for _, imp := range imports {
			if _, seen := prev[imp]; !seen {
				prev[imp] = cur
				queue = append(queue, imp)
			}
		}
	}
	return nil
}

func writeDepsFile(t *testing.T, root string, tg target, graph map[string]pkg) {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "# Every package linked into recon-engine for %s (generated by internal/archtest).\n", tg)
	b.WriteString("# The test fails if any network package (net, net/*, crypto/tls) appears.\n")
	for _, p := range sortedKeys(graph) {
		b.WriteString(p)
		b.WriteByte('\n')
	}
	path := filepath.Join(root, "build", fmt.Sprintf("engine-deps-%s-%s.txt", tg.goos, tg.goarch))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("creating build directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func goSourceFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("walking %s: %v", dir, err)
	}
	sort.Strings(files)
	return files
}

// forbiddenImports bring randomness or uncontrolled C code into deterministic packages.
var forbiddenImports = map[string]bool{
	"math/rand": true, "math/rand/v2": true, "crypto/rand": true, "hash/maphash": true, "C": true,
}

// clockFuncs read the clock or depend on elapsed time.
var clockFuncs = map[string]bool{
	"Now": true, "Since": true, "Until": true, "Sleep": true,
	"Tick": true, "After": true, "AfterFunc": true, "NewTimer": true, "NewTicker": true,
}

// clockOrRandomUses lists forbidden imports and clock reads in one source file.
func clockOrRandomUses(t *testing.T, file string) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parsing %s: %v", file, err)
	}
	var problems []string
	timeName := ""
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		switch {
		case forbiddenImports[path]:
			problems = append(problems, "imports "+path)
		case path == "time":
			timeName = "time"
			if imp.Name != nil {
				timeName = imp.Name.Name
			}
			if timeName == "." {
				// A dot import hides which calls are clock reads, so it is rejected outright.
				problems = append(problems, `dot-imports "time"`)
			}
		}
	}
	if timeName != "" && timeName != "_" && timeName != "." {
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || !clockFuncs[sel.Sel.Name] {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == timeName {
				problems = append(problems, "calls time."+sel.Sel.Name)
			}
			return true
		})
	}
	return problems
}

// floatTypes are the built-in floating-point and complex types.
var floatTypes = map[string]bool{"float32": true, "float64": true, "complex64": true, "complex128": true}

// floatFuncs are standard-library functions that parse, format or hold floating point.
// Package math is handled separately by mathIntConsts.
var floatFuncs = map[string]map[string]bool{
	"strconv":  {"ParseFloat": true, "FormatFloat": true, "AppendFloat": true, "ParseComplex": true, "FormatComplex": true},
	"math/big": {"Float": true, "NewFloat": true, "ParseFloat": true},
	"math":     {},
	"fmt":      {},
}

// mathIntConsts are the only names money code may use from package math. Every math
// function works in floating point, and most accept untyped integer constants
// (math.Pow(10, 2)), so no float type ever appears in the source; an allow-list
// catches them all.
var mathIntConsts = map[string]bool{
	"MaxInt": true, "MinInt": true, "MaxInt8": true, "MinInt8": true, "MaxInt16": true, "MinInt16": true,
	"MaxInt32": true, "MinInt32": true, "MaxInt64": true, "MinInt64": true,
	"MaxUint": true, "MaxUint8": true, "MaxUint16": true, "MaxUint32": true, "MaxUint64": true,
}

// floatMethods are methods that convert a value to floating point, such as
// json.Number.Float64 and big.Rat.Float64. Any receiver is flagged.
var floatMethods = map[string]bool{"Float32": true, "Float64": true}

// fmtVerb matches a fmt verb or an escaped "%%"; floatVerbIn uses it to find verbs
// that read or write floating point: %f, %.2f, %e, %g.
var fmtVerb = regexp.MustCompile(`%%|%[-+# 0]*(\[\d+\])?(\d+|\*)?(\.(\d+|\*)?)?(\[\d+\])?[a-zA-Z]`)

// floatVerbIn returns the first floating-point verb in a format string, or "".
func floatVerbIn(format string) string {
	for _, verb := range fmtVerb.FindAllString(format, -1) {
		if strings.ContainsAny(verb[len(verb)-1:], "eEfFgG") {
			return verb
		}
	}
	return ""
}

// floatUses lists floating-point types, literals and conversions in one source file,
// with their line numbers.
func floatUses(t *testing.T, file string) []string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parsing %s: %v", file, err)
	}
	importNames := map[string]string{} // local name -> import path, for the floatFuncs packages
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		if _, watched := floatFuncs[path]; watched {
			name := path[strings.LastIndex(path, "/")+1:] // "math/big" is used as big
			if imp.Name != nil {
				name = imp.Name.Name
			}
			importNames[name] = path
		}
	}
	var problems []string
	at := func(n ast.Node, what string) {
		problems = append(problems, fmt.Sprintf("line %d: %s", fset.Position(n.Pos()).Line, what))
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			if floatTypes[x.Name] {
				at(x, "uses "+x.Name)
			}
		case *ast.BasicLit:
			if x.Kind == token.FLOAT || x.Kind == token.IMAG {
				at(x, "decimal number "+x.Value)
			}
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok {
				if path, ok := importNames[id.Name]; ok {
					if floatFuncs[path][x.Sel.Name] || (path == "math" && !mathIntConsts[x.Sel.Name]) {
						at(x, "uses "+path+"."+x.Sel.Name)
					}
					return true
				}
			}
			if floatMethods[x.Sel.Name] {
				at(x, "calls ."+x.Sel.Name)
			}
		case *ast.CallExpr:
			sel, ok := x.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); !ok || importNames[id.Name] != "fmt" {
				return true
			}
			for _, arg := range x.Args {
				if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if verb := floatVerbIn(lit.Value); verb != "" {
						at(lit, "formats floating point with "+verb)
					}
				}
			}
		}
		return true
	})
	return problems
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func describeAllowed(allowed []string) string {
	if len(allowed) == 0 {
		return ""
	}
	return fmt.Sprintf(" and %s", strings.Join(allowed, ", "))
}
