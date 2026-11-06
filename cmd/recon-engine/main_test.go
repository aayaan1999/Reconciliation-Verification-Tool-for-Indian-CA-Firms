package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	_ "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store" // registers the SQLite driver
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// The acceptance tests of SPEC-003. They run the command in-process on the samples in
// testdata/run, each against a fresh database. All figures are illustrative.

var (
	repo     = filepath.Join("..", "..")
	storeDir = filepath.Join(repo, "docs", "Recon_Tool_YAML_Rule_Store", "recon_yaml")
	rulesDir = filepath.Join(repo, "content", "rules")
	vocabDir = filepath.Join(repo, "content", "vocabulary")
	profDir  = filepath.Join(repo, "content", "profiles")
	runData  = filepath.Join(repo, "testdata", "run")
)

type result struct {
	code           int
	stdout, stderr string
}

func cmd(t *testing.T, args ...string) result {
	t.Helper()
	var out, errOut bytes.Buffer
	code := realMain(args, &out, &errOut)
	return result{code, out.String(), errOut.String()}
}

// newDB returns a database file with every rule loaded.
func newDB(t *testing.T) string {
	t.Helper()
	db := filepath.Join(t.TempDir(), "recon.db")
	if r := loadRulesCmd(t, db, rulesDir); r.code != 0 {
		t.Fatalf("load-rules: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	return db
}

func loadRulesCmd(t *testing.T, db, rules string) result {
	return cmd(t, "load-rules", "--db", db, "--store", storeDir, "--rules", rules, "--vocab", vocabDir)
}

func runCmd(t *testing.T, db, input string, extra ...string) result {
	t.Helper()
	args := append([]string{"run", "--db", db, "--input", input, "--tax-year", "2025-26", "--vocab", vocabDir, "--profiles", profDir}, extra...)
	return cmd(t, args...)
}

func query(t *testing.T, db, q string, args ...any) [][]string {
	t.Helper()
	conn, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(db))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	rows, err := conn.Query(q, args...)
	if err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	var out [][]string
	for rows.Next() {
		vals := make([]sql.NullString, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		row := make([]string, len(cols))
		for i, v := range vals {
			row[i] = v.String
		}
		out = append(out, row)
	}
	return out
}

func one(t *testing.T, db, q string, args ...any) string {
	t.Helper()
	rows := query(t, db, q, args...)
	if len(rows) != 1 || len(rows[0]) != 1 {
		t.Fatalf("%s: got %v, want one value", q, rows)
	}
	return rows[0][0]
}

// AC-2: 304 rule-store rules load disabled and the engine rules testing; a second load
// with no changes leaves the database unchanged; a changed file adds a version.
func TestLoadRules(t *testing.T) {
	db := newDB(t)
	byStatus := query(t, db, `SELECT v.source, s.status, COUNT(*) FROM rule_state s JOIN rule_versions v ON v.id = s.active_version_id GROUP BY 1, 2 ORDER BY 1, 2`)
	n := len(engineRuleFiles(t))
	want := [][]string{{"builtin", "testing", strconv.Itoa(n)}, {"rulebook", "disabled", "304"}}
	if !slices.EqualFunc(byStatus, want, slices.Equal) {
		t.Errorf("rules by source and status = %v, want %v", byStatus, want)
	}
	snapshot := func() [][]string {
		return query(t, db, `SELECT v.rule_id, v.revision, v.sha256, s.status, s.active_version_id FROM rule_versions v JOIN rule_state s USING (rule_id) ORDER BY v.id`)
	}
	before := snapshot()
	if r := loadRulesCmd(t, db, rulesDir); r.code != 0 || !strings.Contains(r.stdout, fmt.Sprintf("0 rules added or changed, %d unchanged", n+304)) {
		t.Fatalf("second load: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if after := snapshot(); !slices.EqualFunc(before, after, slices.Equal) {
		t.Errorf("a second load with no changes changed the database")
	}

	// A changed engine rule adds a revision and keeps the old one.
	changed := filepath.Join(t.TempDir(), "rules")
	copyTree(t, rulesDir, changed)
	file := filepath.Join(changed, "2025-26", "tieout", "RECON-DEP-001", "rule.yaml")
	b, _ := os.ReadFile(file)
	os.WriteFile(file, bytes.Replace(b, []byte(`notes: "`), []byte(`notes: "Edited. `), 1), 0o644)
	if r := loadRulesCmd(t, db, changed); r.code != 0 || !strings.Contains(r.stdout, "1 rules added or changed") {
		t.Fatalf("changed load: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	revs := query(t, db, `SELECT revision FROM rule_versions WHERE rule_id = 'RECON-DEP-001' ORDER BY revision`)
	if len(revs) != 2 || one(t, db, `SELECT v.revision FROM rule_state s JOIN rule_versions v ON v.id = s.active_version_id WHERE s.rule_id = 'RECON-DEP-001'`) != "2" {
		t.Errorf("revisions of RECON-DEP-001 = %v; want 1 and 2, with 2 active", revs)
	}
}

// AC-3: a clean sample prints the validation report and no findings, and exits 0.
func TestRunClean(t *testing.T) {
	r := runCmd(t, newDB(t), filepath.Join(runData, "clean"), "--include-testing")
	if r.code != exitOK {
		t.Fatalf("exit %d, want 0\n%s%s", r.code, r.stdout, r.stderr)
	}
	for _, s := range []string{"== Files read ==", "comp_2025-26_agrees.json", "generic.taxsuite.computation v1", "Figures read  : 2", "No findings."} {
		if !strings.Contains(r.stdout, s) {
			t.Errorf("report lacks %q:\n%s", s, r.stdout)
		}
	}
	if first := strings.SplitN(r.stdout, "\n", 2)[0]; !strings.Contains(first, "not yet approved by the CA") {
		t.Errorf("first line %q does not say the run includes rules not approved by the CA", first)
	}
}

// AC-4: a planted difference prints the finding with both figures, the difference and
// both locators, and exits 1.
func TestRunDiffers(t *testing.T) {
	r := runCmd(t, newDB(t), filepath.Join(runData, "differs"), "--include-testing")
	if r.code != exitFindings {
		t.Fatalf("exit %d, want 1\n%s%s", r.code, r.stdout, r.stderr)
	}
	for _, s := range []string{
		"Computation of total income prepared in the tax suite (1)",
		"[RECON-DEP-001]",
		`Computation, "Allowed depreciation"     : 1,12,450.50`,
		"Computation, fixed-asset schedule total : 1,15,200.50",
		"Difference                              :    2,750.00",
		"Source: comp_2025-26_differs.json $.business_income.allowed_depreciation | comp_2025-26_differs.json $.fixed_assets.schedule_total",
		"Rule basis: Income-tax Act, 1961 – Sec 32",
	} {
		if !strings.Contains(r.stdout, s) {
			t.Errorf("report lacks %q:\n%s", s, r.stdout)
		}
	}
}

// A fuller tax-suite export (generic computation v3) is read and its planted Rs 90
// transposition between allowed depreciation and the schedule total is found.
func TestRunTaxSuiteExport(t *testing.T) {
	db := newDB(t)
	r := runCmd(t, db, filepath.Join(runData, "taxsuite"), "--include-testing")
	if r.code != exitFindings {
		t.Fatalf("exit %d, want 1\n%s%s", r.code, r.stdout, r.stderr)
	}
	for _, s := range []string{
		"generic.taxsuite.computation v3",
		"Figures read  : 10",
		`Computation, "Allowed depreciation"     : 98,760`,
		"Computation, fixed-asset schedule total : 98,670",
		"Difference                              :     90",
	} {
		if !strings.Contains(r.stdout, s) {
			t.Errorf("report lacks %q:\n%s", s, r.stdout)
		}
	}
}

// AC-5: a needed figure removed gives a Cannot check line naming it, not a finding.
func TestRunFigureMissing(t *testing.T) {
	r := runCmd(t, newDB(t), filepath.Join(runData, "missing"), "--include-testing")
	if r.code != exitOK {
		t.Fatalf("exit %d, want 0\n%s%s", r.code, r.stdout, r.stderr)
	}
	want := "Cannot check: figure not found in Tax audit report (Form 3CA/3CB with 3CD): Tax audit report, depreciation schedule total (tar.depreciation_schedule.total)"
	for _, s := range []string{want, "No findings.", "REQUIRED_FIELD_MISSING"} {
		if !strings.Contains(r.stdout, s) {
			t.Errorf("report lacks %q:\n%s", s, r.stdout)
		}
	}
}

// AC-6: an unknown file and a damaged file exit 2 with their code and file name, and
// nothing partial is written, even when other files of the input were fine.
func TestRunInputErrors(t *testing.T) {
	for _, c := range []struct{ folder, want string }{
		{"unknown", "PROFILE_NO_MATCH: balance_sheet.json"},
		{"damaged", "PARSE_ERROR: comp_damaged.json: line 8, column 38"},
	} {
		t.Run(c.folder, func(t *testing.T) {
			db := newDB(t)
			mixed := t.TempDir()
			copyTree(t, filepath.Join(runData, "clean"), mixed) // a good file, read first
			copyTree(t, filepath.Join(runData, c.folder), mixed)
			r := runCmd(t, db, mixed, "--include-testing")
			if r.code != exitInput || !strings.Contains(r.stderr, c.want) {
				t.Fatalf("exit %d, stderr %q; want 2 and %q", r.code, r.stderr, c.want)
			}
			for _, table := range []string{"documents", "document_versions", "facts", "runs", "evaluations", "engagements"} {
				if n := one(t, db, "SELECT COUNT(*) FROM "+table); n != "0" {
					t.Errorf("%s has %s rows after a failed run", table, n)
				}
			}
		})
	}
	if r := cmd(t, "run", "--db", "x.db", "--tax-year", "2025-26"); r.code != exitInput || !strings.Contains(r.stderr, "--input is required") {
		t.Errorf("missing flag: exit %d, stderr %q", r.code, r.stderr)
	}
}

// AC-7: a database that cannot be written exits 3.
func TestRunReadOnlyDatabase(t *testing.T) {
	db := newDB(t)
	if err := os.Chmod(db, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(db, 0o644) })
	r := runCmd(t, db, filepath.Join(runData, "differs"), "--include-testing")
	if r.code != exitInternal {
		t.Fatalf("exit %d, want 3\n%s%s", r.code, r.stdout, r.stderr)
	}
}

// AC-8: without --include-testing no testing or disabled rule is evaluated.
func TestRunWithoutTesting(t *testing.T) {
	db := newDB(t)
	r := runCmd(t, db, filepath.Join(runData, "differs"))
	if r.code != exitOK || !strings.Contains(r.stdout, "0 checks") || strings.Contains(r.stdout, "not yet approved") {
		t.Fatalf("exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if n := one(t, db, `SELECT COUNT(*) FROM evaluations`); n != "0" {
		t.Errorf("%s evaluations stored; want none", n)
	}
	if n := one(t, db, `SELECT COUNT(*) FROM run_rules`); n != "0" {
		t.Errorf("%s rules in the run snapshot; want none", n)
	}
}

// AC-9: each run stores its snapshot; evaluations are stored sorted by rule and subject.
func TestRunSnapshot(t *testing.T) {
	db := newDB(t)
	if r := runCmd(t, db, filepath.Join(runData, "missing"), "--include-testing"); r.code != exitOK {
		t.Fatalf("exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	run := query(t, db, `SELECT engine_version, vocab_version, include_testing, finished_at <> '' FROM runs`)
	v, err := vocab.LoadDir(vocabDir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.EqualFunc(run, [][]string{{EngineVersion, strconv.Itoa(v.Version()), "1", "1"}}, slices.Equal) {
		t.Errorf("runs = %v", run)
	}
	docs := query(t, db, `SELECT d.document_type, v.file_name, rd.profile_id, rd.profile_version FROM run_documents rd
		JOIN document_versions v ON v.id = rd.document_version_id JOIN documents d ON d.id = v.document_id ORDER BY 1`)
	wantDocs := [][]string{
		{"computation", "comp_2025-26_agrees.json", "generic.taxsuite.computation", "1"},
		{"tax_audit_report", "tar_2025-26_no_total.json", "generic.taxsuite.tax_audit_report", "1"},
	}
	if !slices.EqualFunc(docs, wantDocs, slices.Equal) {
		t.Errorf("run documents = %v", docs)
	}
	// The snapshot records every engine rule of the tax year with the status it had.
	rs := query(t, db, `SELECT rule_id, version, status FROM run_rules ORDER BY rule_id`)
	// (SPEC-020: a rule that applies to later years only, such as the TDS and bank rules, is not in this tax year's run.)
	n := 0
	for _, rf := range engineRuleFiles(t) {
		if body, err := os.ReadFile(rf); err == nil && strings.Contains(string(body), "tax_years: [") && strings.Contains(strings.SplitAfter(string(body), "tax_years: [")[1][:40], "\"2025-26\"") {
			n++
		}
	}
	if len(rs) != n {
		t.Errorf("%d run rules, want every engine rule of the year (%d): %v", len(rs), n, rs)
	}
	for _, r := range rs {
		if r[2] != "testing" {
			t.Errorf("run rule %v: want status testing", r)
		}
	}
	// One evaluation per rule: the computation's own depreciation agrees, the tax audit
	// report has no total, and every other rule lacks its documents.
	evs := query(t, db, `SELECT rule_id, subject_key, status, missing_json FROM evaluations ORDER BY seq`)
	if len(evs) != len(rs) {
		t.Errorf("%d evaluations, want one per rule", len(evs))
	}
	for _, e := range evs {
		want := "cannot_check"
		switch e[0] {
		case "RECON-DEP-001":
			want = "pass"
		case "RECON-DEP-002":
			if e[3] != `["tar.depreciation_schedule.total"]` {
				t.Errorf("RECON-DEP-002 missing %s, want the tax audit report's total", e[3])
			}
		}
		if e[1] != "FY2025-26||" || e[2] != want {
			t.Errorf("evaluation %v: want %s for FY2025-26", e, want)
		}
	}
	// The same file again is not a new draft.
	if r := runCmd(t, db, filepath.Join(runData, "missing"), "--include-testing"); r.code != exitOK {
		t.Fatalf("second run: exit %d", r.code)
	}
	if n := one(t, db, `SELECT MAX(draft_no) FROM document_versions`); n != "1" {
		t.Errorf("the same files again made draft %s", n)
	}
}

// AC-10 and AC-11: two runs with GOMAXPROCS 1 and 8 give byte-identical output with no
// timestamps, and no report contains a banned phrase.
func TestRunDeterministicAndWorded(t *testing.T) {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(0))
	timestamp := regexp.MustCompile(`\d{4}-\d{2}-\d{2}T|\d{2}:\d{2}:\d{2}`)
	for _, folder := range []string{"clean", "differs", "missing"} {
		var outs, jsons []string
		for _, procs := range []int{1, 8} {
			runtime.GOMAXPROCS(procs)
			jsonFile := filepath.Join(t.TempDir(), "report.json")
			r := runCmd(t, newDB(t), filepath.Join(runData, folder), "--include-testing", "--json", jsonFile)
			if r.code > exitFindings {
				t.Fatalf("%s: exit %d\n%s", folder, r.code, r.stderr)
			}
			js, err := os.ReadFile(jsonFile)
			if err != nil {
				t.Fatal(err)
			}
			outs, jsons = append(outs, r.stdout), append(jsons, string(js))
		}
		if outs[0] != outs[1] || jsons[0] != jsons[1] {
			t.Errorf("%s: GOMAXPROCS 1 and 8 gave different output", folder)
		}
		for _, text := range []string{outs[0], jsons[0]} {
			if m := timestamp.FindString(text); m != "" {
				t.Errorf("%s: report contains a timestamp %q", folder, m)
			}
			if b := rules.BannedIn(text); len(b) > 0 {
				t.Errorf("%s: report contains banned phrases %q", folder, b)
			}
		}
	}
}

// engineRuleFiles are the engine's rule files (content/rules/<tax year>/<module>/<id>/rule.yaml).
func engineRuleFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(rulesDir, "*", "*", "*", "rule.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no engine rules under %s: %v", rulesDir, err)
	}
	return files
}

func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(to, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(to, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
