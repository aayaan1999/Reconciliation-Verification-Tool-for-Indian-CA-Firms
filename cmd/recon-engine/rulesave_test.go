package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/audit"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// The Rule Builder's acceptance tests (SPEC-010): saving an office rule, refusing one with the
// field named, the fixtures a save must pass, editing a built-in rule as an office version,
// versions, switching a rule off and on, the audit log, and office rules read by load-rules and
// run. Every figure is illustrative and every test works in its own temporary folders; the
// built-in rules are only read, and a test checks that they are unchanged.

const testBuiltinRule = "RECON-DEP-001" // a built-in equal check on two computation figures

// testForm is a valid new rule: two computation figures that must agree within Rs 1.00.
func testForm() rules.Form {
	return rules.Form{
		Mode:      "create",
		ID:        "RECON-OFF-001",
		Title:     "Depreciation: computation against its schedule (office test)",
		RuleText:  "Compares allowed depreciation in the computation with the total of its fixed-asset schedule.",
		Notes:     "Synthetic test rule written by the Rule Builder tests.",
		Module:    "tieout",
		Category:  "R1",
		TaxYears:  []string{"2025-26"},
		ValidFrom: "2025-04-01",
		Inputs: []rules.FormInput{
			{Name: "a", Kind: "fact", Ref: "computation.allowed_depreciation", Label: "Computation, allowed depreciation"},
			{Name: "b", Kind: "fact", Ref: "computation.fixed_asset_schedule.total", Label: "Computation, schedule total"},
		},
		Check:          rules.FormCheck{Type: "equal", Tolerance: "1.00"},
		Severity:       "high",
		ReviewDocument: "computation",
		Law:            []rules.FormLaw{{Act: "Income-tax Act, 1961", Section: "32"}},
		Fixtures: []rules.FormFixture{
			{Kind: "pass", Name: "values_agree", Description: "The computation and its schedule show the same depreciation",
				Facts: []rules.FormFact{
					{Fact: "computation.allowed_depreciation", Value: "112450.50", Period: "FY2025-26", File: "comp_agrees.json", Locator: "$.business_income.allowed_depreciation"},
					{Fact: "computation.fixed_asset_schedule.total", Value: "112450.50", Period: "FY2025-26", File: "comp_agrees.json", Locator: "$.fixed_assets.schedule_total"},
				}},
			{Kind: "fail", Name: "differs", Description: "The schedule total is Rs 2,750 higher than the allowed depreciation",
				Facts: []rules.FormFact{
					{Fact: "computation.allowed_depreciation", Value: "112450.50", Period: "FY2025-26", File: "comp_differs.json", Locator: "$.business_income.allowed_depreciation"},
					{Fact: "computation.fixed_asset_schedule.total", Value: "115200.50", Period: "FY2025-26", File: "comp_differs.json", Locator: "$.fixed_assets.schedule_total"},
				}},
		},
	}
}

// officeRoot is a new, empty office rules folder for one test.
func officeRoot(t *testing.T) string { return filepath.Join(t.TempDir(), "office-rules") }

// saveResult is what a rule-save or a rule command returned.
type saveResult struct {
	result
	jsonPath string
}

// saveRule runs rule-save with form f, as the window sends it.
func saveRule(t *testing.T, db, office, by string, f rules.Form) saveResult {
	t.Helper()
	body, err := json.Marshal(map[string]any{"by": by, "rule": f})
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "saved.json")
	r := withStdin(t, string(body), "rule-save", "--db", db, "--rules", rulesDir, "--office-rules", office, "--vocab", vocabDir, "--json", out)
	return saveResult{r, out}
}

// refusedProblems reads the problems a refused rule-save wrote to its --json file.
func refusedProblems(t *testing.T, s saveResult) []rules.Problem {
	t.Helper()
	b, err := os.ReadFile(s.jsonPath)
	if err != nil {
		t.Fatalf("no problems were written (exit %d): %v\n%s", s.code, err, s.stderr)
	}
	var out struct {
		Problems []rules.Problem `json:"problems"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out.Problems
}

// ruleCount is the number of stored rule versions, to show that a refusal wrote nothing.
func ruleCount(t *testing.T, db string) string {
	return one(t, db, `SELECT COUNT(*) FROM rule_versions`)
}

// getRule reads a rule as the Rule Builder window does (rule-get).
func getRule(t *testing.T, db, office, id string) ruleCurrent {
	t.Helper()
	out := filepath.Join(t.TempDir(), "rule.json")
	r := cmd(t, "rule-get", "--db", db, "--rules", rulesDir, "--office-rules", office, "--vocab", vocabDir, "--rule", id, "--json", out)
	if r.code != 0 {
		t.Fatalf("rule-get %s: exit %d\n%s%s", id, r.code, r.stdout, r.stderr)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var rc ruleCurrent
	if err := json.Unmarshal(b, &rc); err != nil {
		t.Fatal(err)
	}
	return rc
}

// editForm turns a rule read by rule-get into the form of an edit.
func editForm(t *testing.T, rc ruleCurrent) rules.Form {
	t.Helper()
	if !rc.Editable || rc.Form == nil {
		t.Fatalf("rule %s is not editable: %s", rc.RuleID, rc.Reason)
	}
	f := *rc.Form
	f.Mode = "edit"
	f.Fixtures = nil
	return f
}

// The rule file of a new office rule holds the version, the source and the tolerance written.
// AC-2: the file passes the strict loader and the fixtures, and it is testing in the database.
func TestRuleSaveWritesOfficeRule(t *testing.T) {
	db, office := newDB(t), officeRoot(t)
	r := saveRule(t, db, office, "A. Staff", testForm())
	if r.code != 0 {
		t.Fatalf("rule-save: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	dir := filepath.Join(office, "2025-26", "tieout", "RECON-OFF-001")
	for _, p := range []string{"rule.yaml", "fixtures/pass/values_agree.yaml", "fixtures/fail/differs.yaml"} {
		if _, err := os.Stat(filepath.Join(dir, p)); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
	body, err := os.ReadFile(filepath.Join(dir, "rule.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`source: "office"`, `version: "1"`, `tolerance: { paise: 100 }`} {
		if !strings.Contains(string(body), want) {
			t.Errorf("rule.yaml lacks %s:\n%s", want, body)
		}
	}
	// The strict loader and rule-lint's checks read the file back without change.
	v, err := vocab.LoadDir(vocabDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := rules.LoadDir(office, v, checks.All())
	if err != nil {
		t.Fatalf("the office rule does not load: %v", err)
	}
	rs := set.Rules()
	if len(rs) != 1 {
		t.Fatalf("%d rules in the office folder, want 1", len(rs))
	}
	fx, err := rules.LoadFixtures(rs[0], v)
	if err != nil || len(fx.Pass) != 1 || len(fx.Fail) != 1 {
		t.Fatalf("fixtures: %v, pass %d, fail %d", err, len(fx.Pass), len(fx.Fail))
	}
	got := query(t, db, `SELECT v.source, v.version, s.status FROM rule_state s JOIN rule_versions v ON v.id = s.active_version_id WHERE s.rule_id = 'RECON-OFF-001'`)
	if len(got) != 1 || !slices.Equal(got[0], []string{"office", "1", "testing"}) {
		t.Errorf("stored rule = %v, want office, version 1, testing", got)
	}
}

// AC-3: each refusal names the field and says in plain words what is wrong; nothing is written.
func TestRuleSaveRefusesWithTheFieldNamed(t *testing.T) {
	cases := []struct {
		name   string
		change func(f *rules.Form)
		field  string
		code   string
	}{
		{"an unknown fact", func(f *rules.Form) { f.Inputs[0].Ref = "computation.made_up_figure" }, "inputs[0].ref", "UNKNOWN_FACT"},
		{"an unknown record type", func(f *rules.Form) { f.Inputs[0].Kind, f.Inputs[0].Ref = "record_type", "no_such_rows" }, "inputs[0].ref", "UNKNOWN_RECORD_TYPE"},
		{"no law reference", func(f *rules.Form) { f.Law = nil }, "law", "NO_LAW"},
		{"no pass fixture", func(f *rules.Form) { f.Fixtures = f.Fixtures[1:] }, "fixtures", "NO_PASS_FIXTURE"},
		{"no fail fixture", func(f *rules.Form) { f.Fixtures = f.Fixtures[:1] }, "fixtures", "NO_FAIL_FIXTURE"},
		{"a tolerance that is not a number", func(f *rules.Form) { f.Check.Tolerance = "one rupee" }, "check.tolerance", "BAD_AMOUNT"},
		{"a tolerance with more than two decimals", func(f *rules.Form) { f.Check.Tolerance = "1.005" }, "check.tolerance", "BAD_AMOUNT"},
		{"an ID that already exists for the year", func(f *rules.Form) { f.ID = testBuiltinRule }, "id", "ID_EXISTS"},
		{"an ID of the wrong shape", func(f *rules.Form) { f.ID = "DEP-1" }, "id", "BAD_ID"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db, office := newDB(t), officeRoot(t)
			before := ruleCount(t, db)
			f := testForm()
			c.change(&f)
			r := saveRule(t, db, office, "A. Staff", f)
			if r.code != exitInput {
				t.Fatalf("exit %d, want %d\n%s%s", r.code, exitInput, r.stdout, r.stderr)
			}
			ps := refusedProblems(t, r)
			if !slices.ContainsFunc(ps, func(p rules.Problem) bool { return p.Field == c.field && p.Code == c.code }) {
				t.Errorf("problems %+v: want %s at %s", ps, c.code, c.field)
			}
			for _, p := range ps {
				if strings.TrimSpace(p.Message) == "" {
					t.Errorf("a problem with no message: %+v", p)
				}
			}
			if _, err := os.Stat(office); err == nil {
				t.Errorf("a refused rule wrote the office folder")
			}
			if after := ruleCount(t, db); after != before {
				t.Errorf("a refused rule changed the database: %s -> %s rule versions", before, after)
			}
		})
	}
}

// AC-4: a pass fixture that fails, or a fail fixture that passes, is refused and nothing is
// written.
func TestRuleSaveRefusesAFixtureTheRuleGetsWrong(t *testing.T) {
	cases := []struct {
		name   string
		change func(f *rules.Form)
		code   string
	}{
		{"a pass fixture whose figures differ", func(f *rules.Form) { f.Fixtures[0].Facts[1].Value = "115200.50" }, "PASS_FIXTURE_FAILS"},
		{"a fail fixture whose figures agree", func(f *rules.Form) { f.Fixtures[1].Facts[1].Value = "112450.50" }, "FAIL_FIXTURE_PASSES"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			db, office := newDB(t), officeRoot(t)
			f := testForm()
			c.change(&f)
			r := saveRule(t, db, office, "A. Staff", f)
			if r.code != exitInput {
				t.Fatalf("exit %d, want %d\n%s%s", r.code, exitInput, r.stdout, r.stderr)
			}
			if ps := refusedProblems(t, r); !slices.ContainsFunc(ps, func(p rules.Problem) bool { return p.Code == c.code }) {
				t.Errorf("problems %+v: want %s", ps, c.code)
			}
			if _, err := os.Stat(office); err == nil {
				t.Errorf("a refused rule wrote the office folder")
			}
			if got := query(t, db, `SELECT COUNT(*) FROM rule_versions WHERE rule_id = 'RECON-OFF-001'`); got[0][0] != "0" {
				t.Errorf("a refused rule was stored: %v", got)
			}
		})
	}
}

// AC-10: an amount is stored as integer paise. "0.10" becomes 10 paise; "1.005" is refused
// (it has a half paisa), never rounded.
func TestRuleSaveStoresAmountsAsPaise(t *testing.T) {
	db, office := newDB(t), officeRoot(t)
	f := testForm()
	f.Check.Tolerance = "0.10"
	if r := saveRule(t, db, office, "A. Staff", f); r.code != 0 {
		t.Fatalf("rule-save with 0.10: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	stored := one(t, db, `SELECT body_yaml FROM rule_versions WHERE rule_id = 'RECON-OFF-001'`)
	if !strings.Contains(stored, "tolerance: { paise: 10 }") {
		t.Errorf("stored tolerance is not 10 paise:\n%s", stored)
	}
	f2 := testForm()
	f2.ID = "RECON-OFF-002"
	f2.Check.Tolerance = "1.005"
	r := saveRule(t, db, office, "A. Staff", f2)
	if r.code != exitInput {
		t.Fatalf("tolerance 1.005: exit %d, want refusal", r.code)
	}
	if ps := refusedProblems(t, r); !slices.ContainsFunc(ps, func(p rules.Problem) bool { return p.Field == "check.tolerance" }) {
		t.Errorf("1.005 refused without naming the tolerance: %+v", ps)
	}
}

// AC-5: editing a built-in rule writes "<ID> v1-office.1" to the office folder and leaves the
// built-in file byte-identical; "Reset to standard rule" makes the built-in version in use again.
func TestRuleSaveEditsBuiltInRuleAsOfficeVersion(t *testing.T) {
	before := hashTree(t, rulesDir)
	db, office := newDB(t), officeRoot(t)
	rc := getRule(t, db, office, testBuiltinRule)
	if rc.Source != "builtin" || rc.Version != "1" || len(rc.Fixtures) != 4 {
		t.Fatalf("rule-get: source %s version %s, %d fixtures", rc.Source, rc.Version, len(rc.Fixtures))
	}
	f := editForm(t, rc)
	f.Title += " (office edit)"
	r := saveRule(t, db, office, "A. Staff", f)
	if r.code != 0 {
		t.Fatalf("edit of %s: exit %d\n%s%s", testBuiltinRule, r.code, r.stdout, r.stderr)
	}
	body, err := os.ReadFile(filepath.Join(office, "2025-26", "tieout", testBuiltinRule, "rule.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `version: "1-office.1"`) || !strings.Contains(string(body), `source: "office"`) {
		t.Errorf("office version not written as 1-office.1:\n%s", body)
	}
	if _, err := os.Stat(filepath.Join(office, "2025-26", "tieout", testBuiltinRule, "fixtures", "pass")); err != nil {
		t.Errorf("the built-in fixtures were not carried into the office version: %v", err)
	}
	if got := query(t, db, `SELECT v.source, v.version FROM rule_state s JOIN rule_versions v ON v.id = s.active_version_id WHERE s.rule_id = ?`, testBuiltinRule); !slices.Equal(got[0], []string{"office", "1-office.1"}) {
		t.Errorf("version in use = %v, want the office edit", got)
	}
	if after := hashTree(t, rulesDir); !maps.Equal(before, after) {
		t.Errorf("the built-in rules folder changed")
	}

	// Reset to standard rule: the built-in version is in use again, and the office file stays.
	reset := withStdin(t, `{"by":"A. Staff"}`, "rule-versions", "--db", db, "--rule", testBuiltinRule, "--reset", "--json", filepath.Join(t.TempDir(), "v.json"))
	if reset.code != 0 {
		t.Fatalf("reset: exit %d\n%s%s", reset.code, reset.stdout, reset.stderr)
	}
	if got := query(t, db, `SELECT v.source, v.version FROM rule_state s JOIN rule_versions v ON v.id = s.active_version_id WHERE s.rule_id = ?`, testBuiltinRule); !slices.Equal(got[0], []string{"builtin", "1"}) {
		t.Errorf("after reset the version in use is %v, want the standard rule", got)
	}
	if _, err := os.Stat(filepath.Join(office, "2025-26", "tieout", testBuiltinRule, "rule.yaml")); err != nil {
		t.Errorf("reset removed the office file: %v", err)
	}
}

// AC-6: each edit of an office rule is a new version; earlier versions are listed, and
// switching back to one is recorded.
func TestRuleVersionsKeepEachEdit(t *testing.T) {
	db, office := newDB(t), officeRoot(t)
	if r := saveRule(t, db, office, "A. Staff", testForm()); r.code != 0 {
		t.Fatalf("create: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	rc := getRule(t, db, office, "RECON-OFF-001")
	f := editForm(t, rc)
	f.Title = "Depreciation: computation against its schedule (second office text)"
	if r := saveRule(t, db, office, "A. Staff", f); r.code != 0 {
		t.Fatalf("edit: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	list := filepath.Join(t.TempDir(), "versions.json")
	if r := cmd(t, "rule-versions", "--db", db, "--rule", "RECON-OFF-001", "--json", list); r.code != 0 {
		t.Fatalf("rule-versions: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	var got struct {
		Versions []versionOut `json:"versions"`
	}
	readJSON(t, list, &got)
	if len(got.Versions) != 2 || got.Versions[0].Version != "1" || got.Versions[1].Version != "1-office.1" || !got.Versions[1].Active {
		t.Fatalf("versions = %+v, want 1 then 1-office.1 (in use)", got.Versions)
	}
	if _, err := os.Stat(filepath.Join(office, "2025-26", "tieout", "RECON-OFF-001", "history", "1", "rule.yaml.txt")); err != nil {
		t.Errorf("the earlier text was not kept: %v", err)
	}

	// Switching back to version 1 is recorded in the audit log.
	first := got.Versions[0].ID
	sw := withStdin(t, `{"by":"A. Staff"}`, "rule-versions", "--db", db, "--rule", "RECON-OFF-001", "--switch", strconv.FormatInt(first, 10), "--json", filepath.Join(t.TempDir(), "s.json"))
	if sw.code != 0 {
		t.Fatalf("switch: exit %d\n%s%s", sw.code, sw.stdout, sw.stderr)
	}
	if got := query(t, db, `SELECT v.version FROM rule_state s JOIN rule_versions v ON v.id = s.active_version_id WHERE s.rule_id = 'RECON-OFF-001'`); got[0][0] != "1" {
		t.Errorf("version in use after switch = %v, want 1", got)
	}
	if got := query(t, db, `SELECT COUNT(*) FROM audit_log WHERE action = 'rule.switch_version' AND object_id = 'RECON-OFF-001 v1'`); got[0][0] != "1" {
		t.Errorf("switch not in the audit log: %v", got)
	}
}

// AC-7: disabling and enabling need a reason; a disabled rule stops running, and its files and
// versions stay.
func TestRuleDisableAndEnableNeedAReason(t *testing.T) {
	db, office := newDB(t), officeRoot(t)
	if r := saveRule(t, db, office, "A. Staff", testForm()); r.code != 0 {
		t.Fatalf("create: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	input := filepath.Join(runData, "differs")
	ranOffice := func() string {
		return query(t, db, `SELECT COUNT(*) FROM run_rules WHERE rule_id = 'RECON-OFF-001'`)[0][0]
	}
	if r := runCmd(t, db, input, "--include-testing", "--office-rules", office); r.code > 1 {
		t.Fatalf("run before disable: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if ranOffice() != "1" {
		t.Fatalf("the testing rule was not used before it was disabled")
	}
	disable := func(reason string) result {
		return withStdin(t, `{"by":"A. Staff","reason":"`+reason+`"}`, "rule-disable", "--db", db, "--rule", "RECON-OFF-001", "--json", filepath.Join(t.TempDir(), "d.json"))
	}
	if r := disable("  "); r.code != exitInput || !strings.Contains(r.stderr, "REASON_REQUIRED") {
		t.Fatalf("disable without a reason: exit %d, stderr %q", r.code, r.stderr)
	}
	if r := disable("fires on test data"); r.code != 0 {
		t.Fatalf("disable: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if got := query(t, db, `SELECT status, disabled_reason FROM rule_state WHERE rule_id = 'RECON-OFF-001'`); !slices.Equal(got[0], []string{"disabled", "fires on test data"}) {
		t.Errorf("state after disable = %v", got)
	}
	if r := runCmd(t, db, input, "--include-testing", "--office-rules", office); r.code > 1 {
		t.Fatalf("run after disable: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	// The run that followed did not use the rule: the latest run's rules hold no RECON-OFF-001.
	if got := query(t, db, `SELECT COUNT(*) FROM run_rules rr JOIN runs r ON r.id = rr.run_id WHERE rr.rule_id = 'RECON-OFF-001' AND r.id = (SELECT MAX(id) FROM runs)`); got[0][0] != "0" {
		t.Errorf("a disabled rule ran: %v", got)
	}
	if _, err := os.Stat(filepath.Join(office, "2025-26", "tieout", "RECON-OFF-001", "rule.yaml")); err != nil {
		t.Errorf("disable removed the rule file: %v", err)
	}
	enable := func(reason string) result {
		return withStdin(t, `{"by":"A. Staff","reason":"`+reason+`"}`, "rule-enable", "--db", db, "--rule", "RECON-OFF-001", "--json", filepath.Join(t.TempDir(), "e.json"))
	}
	if r := enable(""); r.code != exitInput || !strings.Contains(r.stderr, "REASON_REQUIRED") {
		t.Fatalf("enable without a reason: exit %d, stderr %q", r.code, r.stderr)
	}
	if r := enable("checked with the CA"); r.code != 0 {
		t.Fatalf("enable: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if got := query(t, db, `SELECT status, disabled_reason FROM rule_state WHERE rule_id = 'RECON-OFF-001'`); !slices.Equal(got[0], []string{"testing", ""}) {
		t.Errorf("state after enable = %v, want testing with no reason", got)
	}
}

// AC-8: every create, edit, disable, enable and switch writes one audit entry with the reason
// where one is needed; the table refuses updates and deletes; the chain verifies.
func TestAuditLogRecordsEachChange(t *testing.T) {
	db, office := newDB(t), officeRoot(t)
	if r := saveRule(t, db, office, "A. Staff", testForm()); r.code != 0 {
		t.Fatalf("create: exit %d", r.code)
	}
	f := editForm(t, getRule(t, db, office, "RECON-OFF-001"))
	f.Title += " (edited)"
	if r := saveRule(t, db, office, "A. Staff", f); r.code != 0 {
		t.Fatalf("edit: exit %d", r.code)
	}
	if r := withStdin(t, `{"by":"A. Staff","reason":"noisy"}`, "rule-disable", "--db", db, "--rule", "RECON-OFF-001", "--json", filepath.Join(t.TempDir(), "d.json")); r.code != 0 {
		t.Fatalf("disable: exit %d", r.code)
	}
	if r := withStdin(t, `{"by":"A. Staff","reason":"resolved"}`, "rule-enable", "--db", db, "--rule", "RECON-OFF-001", "--json", filepath.Join(t.TempDir(), "e.json")); r.code != 0 {
		t.Fatalf("enable: exit %d", r.code)
	}
	got := query(t, db, `SELECT action, actor, object_id, details_json FROM audit_log ORDER BY seq`)
	wantActions := []string{"rule.create", "rule.edit", "rule.disable", "rule.enable"}
	if len(got) != len(wantActions) {
		t.Fatalf("audit rows = %d, want %d: %v", len(got), len(wantActions), got)
	}
	for i, w := range wantActions {
		if got[i][0] != w || got[i][1] != "A. Staff" {
			t.Errorf("audit row %d = %v, want action %s by A. Staff", i, got[i], w)
		}
	}
	if !strings.Contains(got[2][3], `"reason":"noisy"`) || !strings.Contains(got[3][3], `"reason":"resolved"`) {
		t.Errorf("the reasons are not in the audit details: %v", got)
	}
	if got[0][2] != "RECON-OFF-001 v1" {
		t.Errorf("object of the create = %q, want the rule and version", got[0][2])
	}

	// The table refuses updates and deletes.
	conn, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(db))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Exec(`UPDATE audit_log SET actor = 'someone else'`); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Errorf("UPDATE on audit_log: %v, want the append-only refusal", err)
	}
	if _, err := conn.Exec(`DELETE FROM audit_log`); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Errorf("DELETE on audit_log: %v, want the append-only refusal", err)
	}

	// The chain verifies.
	tx := openStore(t, db)
	entries, err := tx.AuditEntries()
	if err != nil {
		t.Fatal(err)
	}
	if err := audit.Verify(entries); err != nil {
		t.Errorf("the audit chain does not verify: %v", err)
	}
}

// AC-9: a run after an office rule is added or edited records that rule's version and source,
// and two runs of the same files give identical results.
func TestRuleSaveRunRecordsTheOfficeVersion(t *testing.T) {
	db, office := newDB(t), officeRoot(t)
	if r := saveRule(t, db, office, "A. Staff", testForm()); r.code != 0 {
		t.Fatalf("create: exit %d", r.code)
	}
	f := editForm(t, getRule(t, db, office, "RECON-OFF-001"))
	f.Title += " (edited)"
	if r := saveRule(t, db, office, "A. Staff", f); r.code != 0 {
		t.Fatalf("edit: exit %d", r.code)
	}
	input := filepath.Join(runData, "differs")
	out1, out2 := filepath.Join(t.TempDir(), "r1.json"), filepath.Join(t.TempDir(), "r2.json")
	if r := runCmd(t, db, input, "--include-testing", "--office-rules", office, "--json", out1); r.code > 1 {
		t.Fatalf("run 1: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if r := runCmd(t, db, input, "--include-testing", "--office-rules", office, "--json", out2); r.code > 1 {
		t.Fatalf("run 2: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	a, _ := os.ReadFile(out1)
	b, _ := os.ReadFile(out2)
	if len(a) == 0 || string(a) != string(b) {
		t.Errorf("two runs of the same files differ")
	}
	got := query(t, db, `SELECT rr.version, v.source FROM run_rules rr JOIN rule_versions v ON v.id = rr.rule_version_id
		JOIN runs r ON r.id = rr.run_id WHERE rr.rule_id = 'RECON-OFF-001' AND r.id = (SELECT MAX(id) FROM runs)`)
	if len(got) != 1 || !slices.Equal(got[0], []string{"1-office.1", "office"}) {
		t.Errorf("the run recorded %v, want version 1-office.1 from the office", got)
	}
}

// load-rules reads the office folder too, and a run given --office-rules uses its rules.
func TestOfficeRulesLoadWithLoadRules(t *testing.T) {
	first, office := newDB(t), officeRoot(t)
	if r := saveRule(t, first, office, "A. Staff", testForm()); r.code != 0 {
		t.Fatalf("create: exit %d", r.code)
	}
	fresh := newDB(t)
	r := cmd(t, "load-rules", "--db", fresh, "--store", storeDir, "--rules", rulesDir, "--office-rules", office, "--vocab", vocabDir)
	if r.code != 0 || !strings.Contains(r.stdout, "1 office rules added or changed") {
		t.Fatalf("load-rules with the office folder: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if got := query(t, fresh, `SELECT v.source, s.status FROM rule_state s JOIN rule_versions v ON v.id = s.active_version_id WHERE s.rule_id = 'RECON-OFF-001'`); !slices.Equal(got[0], []string{"office", "testing"}) {
		t.Errorf("office rule loaded as %v", got)
	}
	// Loading the same folder again changes nothing.
	if r := cmd(t, "load-rules", "--db", fresh, "--store", storeDir, "--rules", rulesDir, "--office-rules", office, "--vocab", vocabDir); !strings.Contains(r.stdout, "0 office rules added or changed") {
		t.Errorf("a second load changed the office rules: %s", r.stdout)
	}
}

// hashTree maps every file under dir to its SHA-256, so a test can show nothing changed.
func hashTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(b)
		out[filepath.ToSlash(p)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// openStore opens the database for reading its audit log.
func openStore(t *testing.T, path string) *store.Tx {
	t.Helper()
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tx.Rollback)
	return tx
}

// clientRunWithTesting adds a sample client and runs its documents with testing rules included,
// which is the run the approval gate asks for (SPEC-010 §4).
func clientRunWithTesting(t *testing.T, db, office string) {
	t.Helper()
	if r := withStdin(t, "ABCDE1234F", "client-add", "--db", db, "--name", "Sample Firm", "--tax-year", "2025-26"); r.code != exitOK {
		t.Fatalf("client-add: exit %d\n%s", r.code, r.stderr)
	}
	clientRunAgain(t, db, office)
}

// clientRunAgain runs the client's documents again with testing rules included (after the client
// was added by clientRunWithTesting), so a changed version of a rule has a run for the gate.
func clientRunAgain(t *testing.T, db, office string) {
	t.Helper()
	if r := runCmd(t, db, filepath.Join(runData, "differs"), "--include-testing", "--client", "1", "--office-rules", office); r.code > exitFindings {
		t.Fatalf("client run: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
}

// approveRule sends the Partner's name and reason to rule-approve, as the window does.
func approveRule(t *testing.T, db, office, by, reason, id string) result {
	t.Helper()
	body, err := json.Marshal(map[string]string{"by": by, "reason": reason})
	if err != nil {
		t.Fatal(err)
	}
	return withStdin(t, string(body), "rule-approve", "--db", db, "--rules", rulesDir, "--office-rules", office, "--vocab", vocabDir,
		"--rule", id, "--json", filepath.Join(t.TempDir(), "approved.json"))
}

// lastRunRules lists the rules the latest run used, with the status each had (RU-53).
func lastRunRules(t *testing.T, db string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, row := range query(t, db, `SELECT rr.rule_id, rr.status FROM run_rules rr WHERE rr.run_id = (SELECT MAX(id) FROM runs)`) {
		out[row[0]] = row[1]
	}
	return out
}

// auditActions lists the audit log's actions, oldest first.
func auditActions(t *testing.T, db string) []string {
	t.Helper()
	var out []string
	for _, row := range query(t, db, `SELECT action FROM audit_log ORDER BY seq`) {
		out = append(out, row[0])
	}
	return out
}

// listedApproval reads the rule list the Rules tab gets (recon-engine rules with the rule folders)
// and returns whether the rule may be approved now, and the note when it may not.
func listedApproval(t *testing.T, db, office, id string) (bool, string) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "rules.json")
	if r := cmd(t, "rules", "--db", db, "--vocab", vocabDir, "--rules", rulesDir, "--office-rules", office, "--json", out); r.code != exitOK {
		t.Fatalf("rules: exit %d\n%s", r.code, r.stderr)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var list struct {
		Rules []ruleInfo `json:"rules"`
	}
	if err := json.Unmarshal(b, &list); err != nil {
		t.Fatal(err)
	}
	for _, ri := range list.Rules {
		if ri.ID == id {
			return ri.Approvable, ri.ApprovalNote
		}
	}
	t.Fatalf("%s is not listed", id)
	return false, ""
}

// AC-18: a testing rule is approved only when its fixtures pass and a client's run included it;
// the approval needs the Partner's name and a reason, sets the rule active and writes one audit
// entry with both. A refusal names the reason and changes nothing.
func TestRuleApprovalGate(t *testing.T) {
	db, office := newDB(t), officeRoot(t)
	if r := saveRule(t, db, office, "A. Staff", testForm()); r.code != 0 {
		t.Fatalf("create: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}

	// Run without a client: the rule has not been run on a client, so it is refused with the reason.
	if r := runCmd(t, db, filepath.Join(runData, "differs"), "--include-testing", "--office-rules", office); r.code > exitFindings {
		t.Fatalf("run without a client: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if ok, note := listedApproval(t, db, office, "RECON-OFF-001"); ok || !strings.Contains(note, "not been run on a client") {
		t.Errorf("before a client run the list says approvable=%v note=%q", ok, note)
	}
	if r := approveRule(t, db, office, "A. Partner", "checked", "RECON-OFF-001"); r.code != exitInput || !strings.Contains(r.stderr, "NOT_RUN_ON_CLIENT") {
		t.Fatalf("approve before a client run: exit %d, stderr %q", r.code, r.stderr)
	}
	if got := query(t, db, `SELECT status FROM rule_state WHERE rule_id = 'RECON-OFF-001'`); got[0][0] != "testing" {
		t.Fatalf("a refused approval changed the status to %v", got)
	}
	clientRunWithTesting(t, db, office)
	if ok, note := listedApproval(t, db, office, "RECON-OFF-001"); !ok || note != "" {
		t.Errorf("after a client run the list says approvable=%v note=%q", ok, note)
	}

	if r := approveRule(t, db, office, "A. Partner", "  ", "RECON-OFF-001"); r.code != exitInput || !strings.Contains(r.stderr, "REASON_REQUIRED") {
		t.Fatalf("approve without a reason: exit %d, stderr %q", r.code, r.stderr)
	}
	if r := approveRule(t, db, office, "  ", "checked", "RECON-OFF-001"); r.code != exitInput || !strings.Contains(r.stderr, "BAD_NAME") {
		t.Fatalf("approve without a name: exit %d, stderr %q", r.code, r.stderr)
	}
	if r := approveRule(t, db, office, "A. Partner", "checked against the computation", "RECON-OFF-001"); r.code != exitOK {
		t.Fatalf("approve: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if got := query(t, db, `SELECT status, approved_by, approved_at FROM rule_state WHERE rule_id = 'RECON-OFF-001'`); got[0][0] != "active" || got[0][1] != "A. Partner" || got[0][2] == "" {
		t.Errorf("state after approval = %v", got)
	}
	if got := auditActions(t, db); !slices.Equal(got, []string{"rule.create", "rule.approve"}) {
		t.Errorf("audit actions = %v, want one rule.approve after the create", got)
	}
	row := query(t, db, `SELECT actor, object_id, details_json FROM audit_log WHERE action = 'rule.approve'`)
	if len(row) != 1 || row[0][0] != "A. Partner" || row[0][1] != "RECON-OFF-001 v1" || !strings.Contains(row[0][2], "checked against the computation") {
		t.Errorf("approval audit entry = %v", row)
	}
	if r := approveRule(t, db, office, "A. Partner", "again", "RECON-OFF-001"); r.code != exitInput || !strings.Contains(r.stderr, "ALREADY_ACTIVE") {
		t.Errorf("approve twice: exit %d, stderr %q", r.code, r.stderr)
	}
}

// AC-18: the fixtures must pass in the engine before approval. A pass fixture that does not pass
// refuses the approval, even after a client run, and the refusal names the fixture problem.
func TestRuleApprovalNeedsFixturesToPass(t *testing.T) {
	db, office := newDB(t), officeRoot(t)
	if r := saveRule(t, db, office, "A. Staff", testForm()); r.code != 0 {
		t.Fatalf("create: exit %d", r.code)
	}
	clientRunWithTesting(t, db, office)
	// The fail fixture's figures go into the pass fixture's file, as if the file had been edited.
	ruleDir := filepath.Join(office, "2025-26", "tieout", "RECON-OFF-001", "fixtures")
	failFile, err := os.ReadFile(filepath.Join(ruleDir, "fail", "differs.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ruleDir, "pass", "values_agree.yaml"), failFile, 0o644); err != nil {
		t.Fatal(err)
	}
	if r := approveRule(t, db, office, "A. Partner", "checked", "RECON-OFF-001"); r.code != exitInput || !strings.Contains(r.stderr, "FIXTURES_NOT_PASSING") {
		t.Fatalf("approve with a failing pass fixture: exit %d, stderr %q", r.code, r.stderr)
	}
	if got := query(t, db, `SELECT status FROM rule_state WHERE rule_id = 'RECON-OFF-001'`); got[0][0] != "testing" {
		t.Errorf("status after a refused approval = %v", got)
	}
}

// AC-19: an active rule runs in a normal run, without --include-testing, and the run records its
// version and its status as active (RU-53). A testing rule in the same folder still does not run.
func TestActiveRuleRunsWithoutIncludeTesting(t *testing.T) {
	db, office := newDB(t), officeRoot(t)
	if r := saveRule(t, db, office, "A. Staff", testForm()); r.code != 0 {
		t.Fatalf("create: exit %d", r.code)
	}
	second := testForm()
	second.ID = "RECON-OFF-002"
	if r := saveRule(t, db, office, "A. Staff", second); r.code != 0 {
		t.Fatalf("create second: exit %d", r.code)
	}
	clientRunWithTesting(t, db, office)
	if r := approveRule(t, db, office, "A. Partner", "checked", "RECON-OFF-001"); r.code != 0 {
		t.Fatalf("approve: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}

	if r := runCmd(t, db, filepath.Join(runData, "differs"), "--office-rules", office); r.code > exitFindings {
		t.Fatalf("normal run: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	used := lastRunRules(t, db)
	if used["RECON-OFF-001"] != "active" {
		t.Errorf("the active rule in a normal run has status %q, want active (rules used: %v)", used["RECON-OFF-001"], used)
	}
	if _, ran := used["RECON-OFF-002"]; ran {
		t.Errorf("a testing rule ran without --include-testing: %v", used)
	}
	if n := one(t, db, `SELECT COUNT(*) FROM evaluations WHERE run_id = (SELECT MAX(id) FROM runs) AND rule_id = 'RECON-OFF-001'`); n == "0" {
		t.Errorf("the active rule was not evaluated")
	}
	if got := query(t, db, `SELECT version FROM run_rules WHERE run_id = (SELECT MAX(id) FROM runs) AND rule_id = 'RECON-OFF-001'`); len(got) != 1 || got[0][0] != "1" {
		t.Errorf("the run recorded version %v, want 1", got)
	}
}

// AC-20: disabling an active rule needs a reason and returns it to disabled; enabling it again
// returns it to testing, and it must be approved again. Each step writes an audit entry.
func TestDisabledActiveRuleMustBeApprovedAgain(t *testing.T) {
	db, office := newDB(t), officeRoot(t)
	if r := saveRule(t, db, office, "A. Staff", testForm()); r.code != 0 {
		t.Fatalf("create: exit %d", r.code)
	}
	clientRunWithTesting(t, db, office)
	if r := approveRule(t, db, office, "A. Partner", "checked", "RECON-OFF-001"); r.code != 0 {
		t.Fatalf("approve: exit %d", r.code)
	}
	disable := func(reason string) result {
		return withStdin(t, `{"by":"A. Partner","reason":"`+reason+`"}`, "rule-disable", "--db", db, "--rule", "RECON-OFF-001", "--json", filepath.Join(t.TempDir(), "d.json"))
	}
	if r := disable(""); r.code != exitInput || !strings.Contains(r.stderr, "REASON_REQUIRED") {
		t.Fatalf("disable an active rule without a reason: exit %d, stderr %q", r.code, r.stderr)
	}
	if r := disable("fires on the client's data"); r.code != 0 {
		t.Fatalf("disable an active rule: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if got := query(t, db, `SELECT status, disabled_reason, approved_by FROM rule_state WHERE rule_id = 'RECON-OFF-001'`); !slices.Equal(got[0], []string{"disabled", "fires on the client's data", ""}) {
		t.Errorf("state after disabling an active rule = %v", got)
	}
	if r := withStdin(t, `{"by":"A. Partner","reason":"checked again"}`, "rule-enable", "--db", db, "--rule", "RECON-OFF-001", "--json", filepath.Join(t.TempDir(), "e.json")); r.code != 0 {
		t.Fatalf("enable: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if got := query(t, db, `SELECT status, approved_by, approved_at FROM rule_state WHERE rule_id = 'RECON-OFF-001'`); !slices.Equal(got[0], []string{"testing", "", ""}) {
		t.Errorf("state after enabling = %v, want testing with no approval", got)
	}
	// Back as testing, the rule does not run without --include-testing until it is approved again.
	if r := runCmd(t, db, filepath.Join(runData, "differs"), "--office-rules", office); r.code > exitFindings {
		t.Fatalf("run: exit %d", r.code)
	}
	if _, ran := lastRunRules(t, db)["RECON-OFF-001"]; ran {
		t.Errorf("an enabled but unapproved rule ran without --include-testing")
	}
	if r := approveRule(t, db, office, "A. Partner", "checked again", "RECON-OFF-001"); r.code != 0 {
		t.Fatalf("approve again: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	want := []string{"rule.create", "rule.approve", "rule.disable", "rule.enable", "rule.approve"}
	if got := auditActions(t, db); !slices.Equal(got, want) {
		t.Errorf("audit actions = %v, want %v", got, want)
	}
	if got := query(t, db, `SELECT details_json FROM audit_log WHERE action = 'rule.disable'`); len(got) != 1 || !strings.Contains(got[0][0], `"from_status":"active"`) {
		t.Errorf("the disable entry does not record that the rule was active: %v", got)
	}
}

// AC-20 (version change): an active rule that is given another version is testing again, because
// the Partner approved the version it had, not the one switched in.
func TestSwitchingVersionEndsApproval(t *testing.T) {
	db, office := newDB(t), officeRoot(t)
	if r := saveRule(t, db, office, "A. Staff", testForm()); r.code != 0 {
		t.Fatalf("create: exit %d", r.code)
	}
	clientRunWithTesting(t, db, office)
	if r := approveRule(t, db, office, "A. Partner", "checked", "RECON-OFF-001"); r.code != 0 {
		t.Fatalf("approve: exit %d", r.code)
	}
	f := editForm(t, getRule(t, db, office, "RECON-OFF-001"))
	f.Title += " (edited)"
	if r := saveRule(t, db, office, "A. Staff", f); r.code != 0 {
		t.Fatalf("edit: exit %d", r.code)
	}
	if got := query(t, db, `SELECT status FROM rule_state WHERE rule_id = 'RECON-OFF-001'`); got[0][0] != "testing" {
		t.Fatalf("an edited active rule is %v, want testing", got)
	}
	clientRunAgain(t, db, office)
	if r := approveRule(t, db, office, "A. Partner", "checked the edit", "RECON-OFF-001"); r.code != 0 {
		t.Fatalf("approve the edit: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	firstVersion := query(t, db, `SELECT id FROM rule_versions WHERE rule_id = 'RECON-OFF-001' ORDER BY revision LIMIT 1`)[0][0]
	if r := withStdin(t, `{"by":"A. Staff"}`, "rule-versions", "--db", db, "--rule", "RECON-OFF-001", "--switch", firstVersion, "--json", filepath.Join(t.TempDir(), "v.json")); r.code != 0 {
		t.Fatalf("switch version: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if got := query(t, db, `SELECT status, approved_by FROM rule_state WHERE rule_id = 'RECON-OFF-001'`); !slices.Equal(got[0], []string{"testing", ""}) {
		t.Errorf("after switching to an earlier version: %v, want testing with no approval", got)
	}
}
