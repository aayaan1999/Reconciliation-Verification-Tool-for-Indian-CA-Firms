package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
)

// SPEC-005: Test Pack A (fictional Nimbus Logic Systems Private Limited), Pass 1 — the
// structured JSON only — scored against the pack's answer key and negative controls.
// SPEC-007: each negative control is checked by rules that pass on it, and each defect is
// planted on its own in the corrected copy.

var (
	testPack      = filepath.Join(repo, "testdata", "testpack_nimbus")
	packFile      = filepath.Join(testPack, "09_filing_pack_structured.json")
	packCorrected = filepath.Join(profDir, "taxsuite_pack", "computation", "v1", "samples", "nimbus_pack_corrected.json")
)

type scoreFile struct {
	Schema   string `yaml:"schema"`
	Findings []struct {
		ID         string `yaml:"id"`
		Critical   bool   `yaml:"critical"`
		InJSON     *bool  `yaml:"in_json"`
		Planned    string `yaml:"planned"`
		DetectedBy []struct {
			Rule       string `yaml:"rule"`
			Difference string `yaml:"difference"`
		} `yaml:"detected_by"`
		// Worklist items the finding is (SPEC-006 part 3): checked by TestTestPackAWorklist.
		Worklist []struct {
			Kind    string `yaml:"kind"`
			Ledger  string `yaml:"ledger"`
			Missing string `yaml:"missing"`
			Times   string `yaml:"times"`
		} `yaml:"worklist"`
		// The finding's figures, put on their own into the corrected copy (SPEC-007).
		Plant []plantStep `yaml:"plant"`
	} `yaml:"findings"`
	NegativeControls []struct {
		ID    string   `yaml:"id"`
		Rules []string `yaml:"rules"`
		None  string   `yaml:"none"` // why no rule reads the control's figures
	} `yaml:"negative_controls"`
	// Rules that fail on the pack because its own figures disagree, although a control calls
	// them consistent (SPEC-007).
	PackErrors []struct {
		Control    string `yaml:"control"`
		Rule       string `yaml:"rule"`
		Difference string `yaml:"difference"`
		Note       string `yaml:"note"`
	} `yaml:"pack_errors"`
}

// A plantStep puts one value into the corrected copy: the pack's value at Copy (at the
// same path, at To, or added to the list at AppendTo), or Value (JSON) at Path.
type plantStep struct {
	Copy     string `yaml:"copy"`
	To       string `yaml:"to"`
	AppendTo string `yaml:"append_to"`
	Path     string `yaml:"path"`
	Value    string `yaml:"value"`
}

func readScore(t *testing.T) scoreFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testPack, "score.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var score scoreFile
	if _, err := strictyaml.Decode("score.yaml", data, &score); err != nil {
		t.Fatal(err)
	}
	return score
}

// packErrors returns the difference in paise of each rule score.yaml says fails on the
// pack because of the pack's own figures.
func (s scoreFile) packErrors(t *testing.T) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	for _, e := range s.PackErrors {
		d, err := money.Parse(e.Difference, money.UnitRupees)
		if err != nil {
			t.Fatalf("pack_errors %s: difference %q: %v", e.Rule, e.Difference, err)
		}
		out[e.Rule] = int64(d)
	}
	return out
}

// csvIDs returns the first column of a CSV file, without its header.
func csvIDs(t *testing.T, name string) []string {
	t.Helper()
	f, err := os.Open(filepath.Join(testPack, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, r := range rows[1:] {
		ids = append(ids, r[0])
	}
	return ids
}

// pass1 runs every rule (testing included) on one filing-pack file and returns the
// difference of each failed rule, and the rules that passed.
func pass1(t *testing.T, file string) (failed map[string]int64, passed []string) {
	t.Helper()
	jsonFile := filepath.Join(t.TempDir(), "report.json")
	r := cmd(t, "run", "--db", newDB(t), "--input", file, "--tax-year", "2025-26",
		"--vocab", vocabDir, "--profiles", profDir, "--include-testing", "--json", jsonFile)
	if r.code > exitFindings {
		t.Fatalf("exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	b, err := os.ReadFile(jsonFile)
	if err != nil {
		t.Fatal(err)
	}
	var rep struct {
		Files []struct {
			DocumentType string `json:"document_type"`
			Errors       []any  `json:"errors"`
		} `json:"files"`
		Findings []struct {
			Findings []struct {
				RuleID     string `json:"rule_id"`
				Difference int64  `json:"difference_paise"`
			} `json:"findings"`
		} `json:"findings"`
		Passed []struct {
			RuleID string `json:"rule_id"`
		} `json:"passed"`
	}
	if err := json.Unmarshal(b, &rep); err != nil {
		t.Fatal(err)
	}
	var docs []string
	for _, f := range rep.Files {
		docs = append(docs, f.DocumentType)
		if len(f.Errors) > 0 {
			t.Errorf("%s: read with errors: %v", f.DocumentType, f.Errors)
		}
	}
	if want := []string{"computation", "financial_statements", "form_26as", "form_67", "mat_report", "tax_audit_report"}; !slices.Equal(docs, want) {
		t.Errorf("documents read %v, want %v", docs, want)
	}
	failed = map[string]int64{}
	for _, g := range rep.Findings {
		for _, f := range g.Findings {
			failed[f.RuleID] = f.Difference
		}
	}
	for _, p := range rep.Passed {
		passed = append(passed, p.RuleID)
	}
	return failed, passed
}

func TestTestPackAPass1(t *testing.T) {
	score := readScore(t)
	packErrors := score.packErrors(t)

	// The score file covers the whole answer key and every negative control, in order.
	var ids, controls []string
	for _, f := range score.Findings {
		ids = append(ids, f.ID)
	}
	for _, n := range score.NegativeControls {
		controls = append(controls, n.ID)
	}
	if want := csvIDs(t, "10_EXPECTED_FINDINGS.csv"); !slices.Equal(ids, want) {
		t.Fatalf("score.yaml findings %v, answer key %v", ids, want)
	}
	if want := csvIDs(t, "11_NEGATIVE_CONTROLS.csv"); !slices.Equal(controls, want) {
		t.Fatalf("score.yaml negative controls %v, pack %v", controls, want)
	}

	failed, passed := pass1(t, packFile)

	// Every mapped finding is detected with its difference; every failed rule is mapped.
	mapped := map[string]string{} // rule → finding
	detected, critical, criticalHit, reachable := 0, 0, 0, 0
	for _, f := range score.Findings {
		if f.Critical {
			critical++
		}
		if f.InJSON == nil || *f.InJSON {
			reachable++
		}
		if len(f.DetectedBy) == 0 {
			if f.InJSON == nil || *f.InJSON {
				t.Logf("not yet covered: %s (%s)", f.ID, f.Planned)
			}
			continue
		}
		hit := true
		for _, d := range f.DetectedBy {
			mapped[d.Rule] = f.ID
			want, err := money.Parse(d.Difference, money.UnitRupees)
			if err != nil {
				t.Fatalf("%s %s: difference %q: %v", f.ID, d.Rule, d.Difference, err)
			}
			got, ok := failed[d.Rule]
			switch {
			case !ok:
				t.Errorf("%s: %s did not fire", f.ID, d.Rule)
				hit = false
			case got != int64(want):
				t.Errorf("%s: %s difference %d paise, want %d", f.ID, d.Rule, got, want)
			}
		}
		if hit {
			detected++
			if f.Critical {
				criticalHit++
			}
		}
	}
	// A rule that fails because of the pack's own figures fails with exactly the
	// difference score.yaml records; it is not a false positive.
	for rule, want := range packErrors {
		switch got, ok := failed[rule]; {
		case !ok:
			t.Errorf("pack_errors: %s did not fail on the pack", rule)
		case got != want:
			t.Errorf("pack_errors: %s difference %d paise, want %d", rule, got, want)
		}
	}
	falsePositives := 0
	for rule := range failed {
		_, finds := mapped[rule]
		_, known := packErrors[rule]
		if !finds && !known {
			t.Errorf("%s fired but finds no answer-key finding: a false positive", rule)
			falsePositives++
		}
	}

	// Every negative control is read by rules that run and pass on it, on the pack and on
	// its corrected copy (SPEC-007), or says why no rule reads it.
	corrected, correctedPassed := pass1(t, packCorrected)
	checked := 0
	for _, n := range score.NegativeControls {
		if (len(n.Rules) == 0) == (n.None == "") {
			t.Errorf("negative control %s: list its rules, or say in none why no rule reads it", n.ID)
		}
		if len(n.Rules) > 0 {
			checked++
		}
		for _, rule := range n.Rules {
			if f, ok := mapped[rule]; ok {
				t.Errorf("negative control %s: %s detects %s, so it cannot also check the control", n.ID, rule, f)
			}
			_, known := packErrors[rule]
			switch _, bad := failed[rule]; {
			case bad && !known:
				t.Errorf("negative control %s flagged by %s", n.ID, rule)
				falsePositives++
			case !bad && !slices.Contains(passed, rule):
				t.Errorf("negative control %s: %s did not run", n.ID, rule)
			}
			if !slices.Contains(correctedPassed, rule) {
				t.Errorf("negative control %s: %s did not pass on the corrected copy", n.ID, rule)
			}
		}
	}
	t.Logf("Test Pack A, Pass 1: %d of %d findings (%d reachable from the JSON), critical %d of %d, %d false positives; "+
		"%d of %d negative controls checked by rules, %d error(s) in the pack itself",
		detected, len(score.Findings), reachable, criticalHit, critical, falsePositives, checked, len(score.NegativeControls), len(packErrors))

	// The corrected copy of the pack gives no finding at all.
	if len(corrected) > 0 {
		t.Errorf("the corrected pack gives findings: %v", corrected)
	}
}

// SPEC-007 part 2: each defect the JSON can show, planted on its own in the corrected copy,
// fires exactly the rules mapped to it in score.yaml: no other rule, and none of the
// negative-control rules.
func TestTestPackAOneDefectAtATime(t *testing.T) {
	score := readScore(t)
	pack, base := readPack(t, packFile), readPack(t, packCorrected)
	for _, f := range score.Findings {
		if len(f.DetectedBy) == 0 {
			if len(f.Plant) > 0 {
				t.Errorf("%s: planted, but no rule detects it", f.ID)
			}
			continue
		}
		t.Run(f.ID, func(t *testing.T) {
			if len(f.Plant) == 0 {
				t.Fatal("not planted: every finding the JSON shows is planted on its own")
			}
			doc := readPack(t, packCorrected) // a fresh copy for each finding
			for i, st := range f.Plant {
				if err := st.apply(pack, doc); err != nil {
					t.Fatalf("plant[%d]: %v", i, err)
				}
			}
			if jsonEqual(t, doc, base) {
				t.Fatal("the plant changes nothing in the corrected copy")
			}
			b, err := json.MarshalIndent(doc, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			file := filepath.Join(t.TempDir(), "planted.json")
			if err := os.WriteFile(file, b, 0o600); err != nil {
				t.Fatal(err)
			}
			failed, _ := pass1(t, file)
			var want []string
			for _, d := range f.DetectedBy {
				want = append(want, d.Rule)
			}
			slices.Sort(want)
			if got := slices.Sorted(maps.Keys(failed)); !slices.Equal(got, want) {
				t.Errorf("rules that fail %v, want exactly %v", got, want)
			}
		})
	}
}

func readPack(t *testing.T, file string) any {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber() // figures stay exactly as written
	var v any
	if err := d.Decode(&v); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	return v
}

func jsonEqual(t *testing.T, a, b any) bool {
	t.Helper()
	ja, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	jb, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Equal(ja, jb)
}

func (st plantStep) apply(pack, doc any) error {
	if st.Copy == "" {
		if st.Path == "" || st.Value == "" || st.To != "" || st.AppendTo != "" {
			return fmt.Errorf("a step is { copy } (with to or append_to) or { path, value }: %+v", st)
		}
		d := json.NewDecoder(strings.NewReader(st.Value))
		d.UseNumber()
		var v any
		if err := d.Decode(&v); err != nil {
			return fmt.Errorf("value %q: %w", st.Value, err)
		}
		return jsonSet(doc, st.Path, v)
	}
	if st.Path != "" || st.Value != "" || (st.To != "" && st.AppendTo != "") {
		return fmt.Errorf("a step is { copy } (with to or append_to) or { path, value }: %+v", st)
	}
	v, err := jsonGet(pack, st.Copy)
	if err != nil {
		return fmt.Errorf("in the pack: %w", err)
	}
	switch {
	case st.AppendTo != "":
		list, err := jsonGet(doc, st.AppendTo)
		if err != nil {
			return err
		}
		l, ok := list.([]any)
		if !ok {
			return fmt.Errorf("%s is not a list", st.AppendTo)
		}
		return jsonSet(doc, st.AppendTo, append(l, v))
	case st.To != "":
		return jsonSet(doc, st.To, v)
	}
	return jsonSet(doc, st.Copy, v)
}

var jsonStep = regexp.MustCompile(`^\.([A-Za-z0-9_]+)|^\[([0-9]+)\]`)

// jsonSteps splits a path such as "$.form_67[0].dtaa_rate" into its keys and indexes.
func jsonSteps(path string) ([]any, error) {
	rest, ok := strings.CutPrefix(path, "$")
	if !ok || rest == "" {
		return nil, fmt.Errorf("path %q does not start with $.", path)
	}
	var steps []any
	for rest != "" {
		m := jsonStep.FindStringSubmatch(rest)
		if m == nil {
			return nil, fmt.Errorf("path %q: cannot read %q", path, rest)
		}
		if m[1] != "" {
			steps = append(steps, m[1])
		} else {
			i, _ := strconv.Atoi(m[2])
			steps = append(steps, i)
		}
		rest = rest[len(m[0]):]
	}
	return steps, nil
}

func jsonGet(doc any, path string) (any, error) {
	steps, err := jsonSteps(path)
	if err != nil {
		return nil, err
	}
	return walk(doc, path, steps)
}

func walk(v any, path string, steps []any) (any, error) {
	for _, s := range steps {
		switch k := s.(type) {
		case string:
			m, ok := v.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%s: %q is not inside an object", path, k)
			}
			if v, ok = m[k]; !ok {
				return nil, fmt.Errorf("%s: no %q", path, k)
			}
		case int:
			l, ok := v.([]any)
			if !ok || k >= len(l) {
				return nil, fmt.Errorf("%s: no entry %d", path, k)
			}
			v = l[k]
		}
	}
	return v, nil
}

// jsonSet replaces the value at path in doc; the place must already exist there.
func jsonSet(doc any, path string, v any) error {
	steps, err := jsonSteps(path)
	if err != nil {
		return err
	}
	parent, err := walk(doc, path, steps[:len(steps)-1])
	if err != nil {
		return err
	}
	switch k := steps[len(steps)-1].(type) {
	case string:
		m, ok := parent.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: not inside an object", path)
		}
		if _, ok := m[k]; !ok {
			return fmt.Errorf("%s: no %q in the corrected copy", path, k)
		}
		m[k] = v
	case int:
		l, ok := parent.([]any)
		if !ok || k >= len(l) {
			return fmt.Errorf("%s: no entry %d in the corrected copy", path, k)
		}
		l[k] = v
	}
	return nil
}

// One file holding several documents (a bundle) is read once per document; a file that two
// unrelated profiles match is still refused.
func TestBundleReadsEachDocument(t *testing.T) {
	r := cmd(t, "run", "--db", newDB(t), "--input", packFile, "--tax-year", "2025-26",
		"--vocab", vocabDir, "--profiles", profDir, "--include-testing", "--comparison", "taxsuite_filing_pack", "--comparisons", comparisonsYAML)
	// 28 fail: the 27 rules of the answer key, and RECON-MAT-010 on the pack's own Rs 10
	// (score.yaml, pack_errors). Clause 18 gives one check per block of assets.
	if r.code != exitFindings || !strings.Contains(r.stdout, "1 file (6 documents) read, 67 checks: 39 pass, 28 fail, 0 cannot check") {
		t.Fatalf("exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	// The assessee's name is outside every section and no profile reads it: listed once.
	if n := strings.Count(r.stdout, "$.assessee.name (text; not part of any document this file holds)"); n != 1 {
		t.Errorf("the assessee's name listed %d times, want once:\n%s", n, r.stdout)
	}
	// The assessee's status is outside every section but the audit report reads it: never listed.
	if strings.Contains(r.stdout, "$.assessee.status (") {
		t.Error("the assessee's status is read but listed as not read")
	}
	// Finding text states what was compared, never what ought to be (SPEC-006 AC-6).
	if banned := rules.BannedIn(r.stdout); len(banned) > 0 {
		t.Errorf("the report uses %q", banned)
	}
}
