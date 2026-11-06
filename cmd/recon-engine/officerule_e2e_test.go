package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
)

// SPEC-022 part B: a rule over the rows of a document the Partner added goes through the Rule
// Builder's own commands, with the existing lifecycle, versions, fixtures and audit. Names and
// figures are invented.

// officeReturns is a database with the sales return register added and one layout taught on a JSON
// file whose second row does not add up: net 50,000 and tax 9,000 against a gross of 58,000.
type officeReturns struct {
	db, office, input, profiles string
}

func setUpOfficeReturns(t *testing.T) officeReturns {
	t.Helper()
	dir := t.TempDir()
	o := officeReturns{db: newDB(t), office: officeRoot(t), input: filepath.Join(dir, "in"), profiles: filepath.Join(dir, "profiles")}
	if err := os.Mkdir(o.input, 0o700); err != nil {
		t.Fatal(err)
	}
	file := `{"Returns":[
 {"Ref":"SR-0001","Raised":"03-08-2026","Customer":"Orion Retail","Net":100000,"Tax":18000,"Gross":118000},
 {"Ref":"SR-0002","Raised":"04-08-2026","Customer":"Vega Traders","Net":50000,"Tax":9000,"Gross":58000},
 {"Ref":"SR-0003","Raised":"09-08-2026","Customer":"Orion Retail","Net":20000,"Tax":3600,"Gross":23600}]}`
	if err := os.WriteFile(filepath.Join(o.input, "returns.json"), []byte(file), 0o600); err != nil {
		t.Fatal(err)
	}
	if r, out := docCmd[docSaveOut](t, "document-save", o.db, sixColumnReturns); r.code != 0 || !out.Saved {
		t.Fatalf("document-save: %s", r.stderr)
	}
	mapping := `{"name":"Sales return register","document_type":"sales_return","rows":"$.Returns[*]","where":[],"unit":"rupees","columns":[
 {"name":"ref_no","path":".Ref","date_format":""},{"name":"doc_date","path":".Raised","date_format":"dd-mm-yyyy"},{"name":"customer","path":".Customer","date_format":""},
 {"name":"net_amount","path":".Net","date_format":""},{"name":"tax_amount","path":".Tax","date_format":""},{"name":"gross_amount","path":".Gross","date_format":""}],
 "period":{"path":"","format":"","const":"2026-08"},"tax_year":"","gstin_path":"","identify":[]}`
	r := withStdin(t, mapping, "profile-create", "--profiles", profDir, "--office-profiles", o.profiles, "--vocab", vocabDir, "--db", o.db, "--by", "A. Partner",
		"--rules", rulesDir, "--sample", filepath.Join(o.input, "returns.json"), "--json", filepath.Join(dir, "taught.json"))
	if r.code != exitOK {
		t.Fatalf("profile-create: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	return o
}

// runReturns runs the returns file, as client 1 when client is set, with the office rules.
func (o officeReturns) run(t *testing.T, extra ...string) result {
	t.Helper()
	args := append([]string{"run", "--db", o.db, "--input", o.input, "--tax-year", "2026-27", "--vocab", vocabDir, "--profiles", profDir,
		"--office-profiles", o.profiles, "--office-rules", o.office}, extra...)
	return cmd(t, args...)
}

func rowRule(id string) rules.Form {
	rec := func(file, ref, net, tax, gross string) rules.FormRecord {
		return rules.FormRecord{RecordType: "sales_return_row", Period: "2026-08", File: file, Locator: "$.Returns[" + ref + "]",
			Fields:  []rules.FormNamed{{Name: "ref_no", Value: "SR-" + ref}, {Name: "customer", Value: "Orion Retail"}},
			Amounts: []rules.FormNamed{{Name: "net_amount", Value: net}, {Name: "tax_amount", Value: tax}, {Name: "gross_amount", Value: gross}}}
	}
	return rules.Form{
		Mode: "create", ID: id, Module: "office", Category: "R1", TaxYears: []string{"2026-27"}, ValidFrom: "2026-04-01",
		Title:    "Sales returns: net plus tax equals gross (office test)",
		RuleText: "Checks, row by row, that the net amount plus the tax amount equals the gross amount in the sales return register.",
		Notes:    "Synthetic test rule over a document added in the Document Master.",
		Inputs:   []rules.FormInput{{Name: "a", Kind: "record_type", Ref: "sales_return_row", Label: "Sales returns"}},
		Check: rules.FormCheck{Type: "sum_equals", Tolerance: "1.00",
			Parts: []rules.FormPart{{Input: "a", Field: "net_amount", Sign: "+"}, {Input: "a", Field: "tax_amount", Sign: "+"}}, Total: "a", TotalField: "gross_amount"},
		Severity: "high", ReviewDocument: "sales_return",
		Law: []rules.FormLaw{{Act: "Central Goods and Services Tax Act, 2017", Section: "34"}},
		Fixtures: []rules.FormFixture{
			{Kind: "pass", Name: "adds_up", Description: "Net and tax add up to the gross", Records: []rules.FormRecord{rec("returns_ok.json", "1", "1000.00", "180.00", "1180.00")}},
			{Kind: "fail", Name: "short", Description: "The gross is Rs 10 short of net plus tax", Records: []rules.FormRecord{rec("returns_bad.json", "2", "1000.00", "180.00", "1170.00")}},
		},
	}
}

// AC-17, AC-19, AC-20, AC-23: a Staff user saves a rule over office rows; it is testing and runs only with
// testing rules included; it reports a finding with both values, the difference and both locators;
// the Partner approves it after a client run, then it runs without testing; editing keeps earlier
// versions; switching it off stops it.
func TestOfficeRuleLifecycle(t *testing.T) {
	o := setUpOfficeReturns(t)

	s := saveRule(t, o.db, o.office, "A. Staff", rowRule("RECON-OFF-101"))
	if s.code != 0 {
		t.Fatalf("rule-save: exit %d\n%s%s", s.code, s.stdout, s.stderr)
	}
	if got := query(t, o.db, `SELECT s.status, v.source FROM rule_state s JOIN rule_versions v ON v.id = s.active_version_id WHERE s.rule_id = 'RECON-OFF-101'`); got[0][0] != "testing" || got[0][1] != "office" {
		t.Fatalf("state after the save = %v", got)
	}
	file := filepath.Join(o.office, "2026-27", "office", "RECON-OFF-101", "rule.yaml")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("the rule file is not at <tax-year>/office/<ID>: %v", err)
	}
	for _, want := range []string{`module: "office"`, `record_type: "sales_return_row"`, `type: "sum_equals"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("the generated rule lacks %s:\n%s", want, data)
		}
	}
	if _, err := os.Stat(filepath.Join(o.office, "2026-27", "office", "RECON-OFF-101", "fixtures", "pass", "adds_up.yaml")); err != nil {
		t.Errorf("the pass fixture was not written: %v", err)
	}

	// without testing rules included, it does not run
	if r := o.run(t); r.code != exitOK {
		t.Fatalf("run without testing rules: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	// with them, it reports the row that does not add up, with both values and both locators
	out := filepath.Join(t.TempDir(), "report.json")
	r := o.run(t, "--include-testing", "--json", out)
	if r.code != exitFindings {
		t.Fatalf("run with testing rules: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	report, _ := os.ReadFile(out)
	for _, want := range []string{"RECON-OFF-101", "SR-0002", "returns.json", "Sales returns"} {
		if !strings.Contains(string(report), want) {
			t.Errorf("the report lacks %q", want)
		}
	}
	if strings.Contains(string(report), "SR-0001") && strings.Contains(string(report), `"status":"fail","subject":{"key":"SR-0001"`) {
		t.Error("a row that adds up was reported as a finding")
	}

	// the rule is not approvable before it has run on a client; then the Partner approves it
	if r := approveRule(t, o.db, o.office, "A. Partner", "checked", "RECON-OFF-101"); r.code != exitInput || !strings.Contains(r.stderr, "NOT_RUN_ON_CLIENT") {
		t.Fatalf("approve before a client run: exit %d %s", r.code, r.stderr)
	}
	if r := withStdin(t, deccanJSON, "client-add", "--db", o.db, "--tax-year", "2026-27"); r.code != exitOK {
		t.Fatalf("client-add: %s", r.stderr)
	}
	if r := o.run(t, "--include-testing", "--client", "1"); r.code > exitFindings {
		t.Fatalf("client run: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if r := approveRule(t, o.db, o.office, "A. Partner", "checked against the register", "RECON-OFF-101"); r.code != exitOK {
		t.Fatalf("approve: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if r := o.run(t); r.code != exitFindings {
		t.Fatalf("an approved rule runs without testing rules included: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}

	// editing saves a new version and keeps the earlier one; the record type stays locked by every version
	rc := getRule(t, o.db, o.office, "RECON-OFF-101")
	edit := editForm(t, rc)
	edit.Check.Tolerance = "2.00"
	if r := saveRule(t, o.db, o.office, "A. Staff", edit); r.code != 0 {
		t.Fatalf("edit: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if got := one(t, o.db, `SELECT COUNT(*) FROM rule_versions WHERE rule_id = 'RECON-OFF-101'`); got != "2" {
		t.Errorf("%s versions after an edit, want 2", got)
	}
	_, d := docCmd[docDetailOut](t, "document-get", o.db, "", "--code", "sales_return")
	if !slices.Equal(d.Usage.Rules, []string{"RECON-OFF-101"}) || !d.Structure.Locked {
		t.Errorf("usage %+v locked %v", d.Usage, d.Structure.Locked)
	}

	// switching it off stops it
	body := `{"by":"A. Partner","reason":"checking the register again"}`
	if r := withStdin(t, body, "rule-disable", "--db", o.db, "--rule", "RECON-OFF-101", "--json", filepath.Join(t.TempDir(), "d.json")); r.code != exitOK {
		t.Fatalf("rule-disable: exit %d %s", r.code, r.stderr)
	}
	if r := o.run(t, "--include-testing"); r.code != exitOK {
		t.Errorf("a disabled rule still ran: exit %d\n%s", r.code, r.stdout)
	}
	if got := auditActions(t, o.db); !slices.Contains(got, "rule.create") || !slices.Contains(got, "rule.approve") || !slices.Contains(got, "rule.disable") {
		t.Errorf("audit actions = %v", got)
	}
}

// AC-18: the Rule Builder refuses a rule over an office document that its checks cannot take, a column
// the record type does not have, an amount where text is needed, and a record type that is not ready;
// each problem names the field, and nothing is stored.
func TestOfficeRuleRefusals(t *testing.T) {
	o := setUpOfficeReturns(t)
	before := ruleCount(t, o.db)
	refused := func(name string, f rules.Form, field string) {
		t.Helper()
		s := saveRule(t, o.db, o.office, "A. Staff", f)
		if s.code != exitInput {
			t.Errorf("%s: exit %d, want a refusal\n%s", name, s.code, s.stderr)
			return
		}
		var named bool
		for _, p := range refusedProblems(t, s) {
			named = named || strings.HasPrefix(p.Field, field)
		}
		if !named {
			t.Errorf("%s: no problem names %s: %+v", name, field, refusedProblems(t, s))
		}
		if ruleCount(t, o.db) != before {
			t.Errorf("%s: a refused rule was stored", name)
		}
	}
	notOffice := rowRule("RECON-OFF-102")
	notOffice.Module = "tieout"
	refused("outside the office module", notOffice, "module")

	equal := rowRule("RECON-OFF-103")
	equal.Inputs = append(equal.Inputs, rules.FormInput{Name: "b", Kind: "fact", Ref: "computation.allowed_depreciation", Label: "A figure"})
	equal.Check = rules.FormCheck{Type: "equal", Tolerance: "1.00"}
	equal.Fixtures = nil
	refused("equal over office rows", equal, "check.type")

	wrongColumn := rowRule("RECON-OFF-104")
	wrongColumn.Check.Parts[1].Field = "vat_amount"
	refused("a column the record type does not have", wrongColumn, "check")

	textColumn := rowRule("RECON-OFF-105")
	textColumn.Check.Parts[1].Field = "customer"
	refused("a text column where an amount is needed", textColumn, "check")

	missing := rowRule("RECON-OFF-106")
	missing.Inputs[0].Ref = "no_such_row"
	refused("a record type that is not ready", missing, "inputs")
}

// AC-23: a row-reading rule over an office document is listed with the flag the Rules tab shows as a
// chip, and a built-in rule is not.
func TestRulesListFlagsOfficeDocumentRules(t *testing.T) {
	o := setUpOfficeReturns(t)
	if s := saveRule(t, o.db, o.office, "A. Staff", rowRule("RECON-OFF-101")); s.code != 0 {
		t.Fatalf("rule-save: %s", s.stderr)
	}
	out := filepath.Join(t.TempDir(), "rules.json")
	if r := cmd(t, "rules", "--db", o.db, "--vocab", vocabDir, "--rules", rulesDir, "--office-rules", o.office, "--json", out); r.code != exitOK {
		t.Fatalf("rules: exit %d\n%s", r.code, r.stderr)
	}
	var list struct {
		Rules []struct {
			ID                   string `json:"id"`
			ReadsOfficeDocuments bool   `json:"reads_office_documents"`
		} `json:"rules"`
	}
	readJSON(t, out, &list)
	var flagged []string
	for _, r := range list.Rules {
		if r.ReadsOfficeDocuments {
			flagged = append(flagged, r.ID)
		}
	}
	if !slices.Equal(flagged, []string{"RECON-OFF-101"}) {
		t.Errorf("rules flagged as reading office documents: %v", flagged)
	}
}

// rule-options offers the office record types with their labelled columns, grouped by origin, and says
// which check types read rows; an inactive document stays listed, marked inactive.
func TestRuleOptionsOfferOfficeRecordTypes(t *testing.T) {
	o := setUpOfficeReturns(t)
	read := func() (rec struct {
		Origin  string `json:"origin"`
		Active  bool   `json:"active"`
		Columns []struct {
			Name, Label, Type string
			Key, Required     bool
		} `json:"columns"`
	}, rows []string, modules []string) {
		out := filepath.Join(t.TempDir(), "options.json")
		if r := cmd(t, "rule-options", "--db", o.db, "--rules", rulesDir, "--vocab", vocabDir, "--json", out); r.code != exitOK {
			t.Fatalf("rule-options: exit %d\n%s", r.code, r.stderr)
		}
		var opts struct {
			Modules       []string `json:"modules"`
			RowCheckTypes []string `json:"row_check_types"`
			RecordTypes   []struct {
				Name    string `json:"name"`
				Origin  string `json:"origin"`
				Active  bool   `json:"active"`
				Columns []struct {
					Name, Label, Type string
					Key, Required     bool
				} `json:"columns"`
			} `json:"record_types"`
		}
		readJSON(t, out, &opts)
		for _, rt := range opts.RecordTypes {
			if rt.Name == "sales_return_row" {
				rec.Origin, rec.Active, rec.Columns = rt.Origin, rt.Active, rt.Columns
			} else if rt.Origin != "builtin" || !rt.Active || len(rt.Columns) != 0 {
				t.Errorf("built-in record type %s is %+v", rt.Name, rt)
			}
		}
		return rec, opts.RowCheckTypes, opts.Modules
	}
	rec, rowChecks, modules := read()
	if rec.Origin != "office" || !rec.Active || len(rec.Columns) != 6 || rec.Columns[0].Label != "Reference" || !rec.Columns[0].Key || !rec.Columns[0].Required || rec.Columns[3].Type != "amount" {
		t.Errorf("office record type = %+v", rec)
	}
	if !slices.Equal(rowChecks, []string{"match_records", "present_together", "sum_equals"}) {
		t.Errorf("row-reading check types = %v", rowChecks)
	}
	if !slices.Contains(modules, "office") {
		t.Errorf("modules = %v", modules)
	}
	if r, _ := docCmd[docDetailOut](t, "document-status", o.db, `{"by":"A. Partner","code":"sales_return","active":false}`); r.code != 0 {
		t.Fatalf("deactivate: %s", r.stderr)
	}
	if rec, _, _ := read(); rec.Active || rec.Origin != "office" {
		t.Errorf("an inactive document is %+v, want listed and marked inactive", rec)
	}
}

// AC-21: reproducibility. After a run, switching the document off, adding an optional column to its
// record type and editing the rule, the earlier comparison opens exactly as it was reported (the same
// bytes, with the office vocabulary kept with the run), and running its inputs again with the rule
// switched back to the version it ran with reproduces the same report byte for byte.
func TestEarlierComparisonOpensAsReported(t *testing.T) {
	o := setUpOfficeReturns(t)
	if s := saveRule(t, o.db, o.office, "A. Staff", rowRule("RECON-OFF-101")); s.code != 0 {
		t.Fatalf("rule-save: %s", s.stderr)
	}
	if r := withStdin(t, deccanJSON, "client-add", "--db", o.db, "--tax-year", "2026-27"); r.code != exitOK {
		t.Fatalf("client-add: %s", r.stderr)
	}
	if r := o.run(t, "--include-testing", "--client", "1"); r.code != exitFindings {
		t.Fatalf("first run: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	opened := func(no string) []byte {
		t.Helper()
		out := filepath.Join(t.TempDir(), "history.json")
		if r := cmd(t, "history", "--db", o.db, "--client", "1", "--tax-year", "2026-27", "--no", no, "--json", out); r.code != exitOK {
			t.Fatalf("history --no %s: exit %d\n%s", no, r.code, r.stderr)
		}
		b, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	before := opened("1")
	var view struct {
		Hash   string `json:"office_vocab_hash"`
		Office struct {
			Documents   []struct{ Code, Name string } `json:"Documents"`
			RecordTypes []struct {
				Name    string
				Columns []struct{ Name, Label string }
			} `json:"RecordTypes"`
		} `json:"office_vocab"`
	}
	if err := json.Unmarshal(before, &view); err != nil {
		t.Fatal(err)
	}
	if len(view.Hash) != 64 || len(view.Office.Documents) != 1 || view.Office.Documents[0].Code != "sales_return" ||
		len(view.Office.RecordTypes) != 1 || len(view.Office.RecordTypes[0].Columns) != 6 || view.Office.RecordTypes[0].Columns[0].Label != "Reference" {
		t.Fatalf("the comparison does not carry the vocabulary it ran with: hash %q %+v", view.Hash, view.Office)
	}

	// everything the Partner can change afterwards
	if r, _ := docCmd[docDetailOut](t, "document-status", o.db, `{"by":"A. Partner","code":"sales_return","active":false}`); r.code != 0 {
		t.Fatalf("deactivate: %s", r.stderr)
	}
	_, cur := docCmd[docDetailOut](t, "document-get", o.db, "", "--code", "sales_return")
	cols := append(slices.Clone(cur.Structure.Columns), docColumnJSON{Name: "memo", Label: "Memo", Type: "text"})
	cols[0].Label = "Reference number"
	body, _ := json.Marshal(map[string]any{"by": "A. Partner", "code": "sales_return", "name": "Sales return register", "category": "Books",
		"description": "Credit notes issued to customers", "applicable_for": []string{"json", "xlsx", "csv"}, "status": "inactive",
		"record_type": map[string]any{"name": "sales_return_row", "description": "Rows of the sales return register", "columns": cols}})
	if r, out := docCmd[docSaveOut](t, "document-save", o.db, string(body), "--edit"); r.code != 0 || !out.Saved {
		t.Fatalf("adding an optional column: exit %d %s", r.code, r.stderr)
	}
	edit := editForm(t, getRule(t, o.db, o.office, "RECON-OFF-101"))
	edit.Check.Tolerance = "5.00"
	if s := saveRule(t, o.db, o.office, "A. Staff", edit); s.code != 0 {
		t.Fatalf("edit of the rule: %s", s.stderr)
	}

	if after := opened("1"); string(after) != string(before) {
		t.Errorf("the earlier comparison changed after the document and the rule were edited")
	}

	// run the same inputs again with the rule as it was: the same report, byte for byte
	first := one(t, o.db, `SELECT id FROM rule_versions WHERE rule_id = 'RECON-OFF-101' ORDER BY revision LIMIT 1`)
	if r := withStdin(t, `{"by":"A. Staff"}`, "rule-versions", "--db", o.db, "--rule", "RECON-OFF-101", "--switch", first, "--json", filepath.Join(t.TempDir(), "s.json")); r.code != 0 {
		t.Fatalf("switch back: %s", r.stderr)
	}
	if r := o.run(t, "--include-testing", "--client", "1"); r.code != exitFindings {
		t.Fatalf("second run: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	var one1, two struct {
		Report json.RawMessage `json:"report"`
	}
	if err := json.Unmarshal(before, &one1); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(opened("2"), &two); err != nil {
		t.Fatal(err)
	}
	if len(one1.Report) == 0 || string(one1.Report) != string(two.Report) {
		t.Errorf("running the same inputs again with the same rule version gave a different report")
	}
	// the new comparison carries the vocabulary of its own day: the label that was edited
	var later struct {
		Hash   string `json:"office_vocab_hash"`
		Office struct {
			RecordTypes []struct{ Columns []struct{ Label string } } `json:"RecordTypes"`
		} `json:"office_vocab"`
	}
	if err := json.Unmarshal(opened("2"), &later); err != nil {
		t.Fatal(err)
	}
	if later.Hash == view.Hash || later.Office.RecordTypes[0].Columns[0].Label != "Reference number" || len(later.Office.RecordTypes[0].Columns) != 7 {
		t.Errorf("the second comparison's vocabulary: hash %q (was %q), %+v", later.Hash, view.Hash, later.Office)
	}
	if n := one(t, o.db, `SELECT COUNT(*) FROM office_vocab_snapshots`); n != "2" {
		t.Errorf("%s vocabulary snapshots, want 2 (one for each distinct office vocabulary)", n)
	}
}

// AC-22: with office documents and an office rule present, every built-in rule gives the same status,
// values and difference for each row of each sample case as it does with an empty Master.
func TestBuiltinRulesUnchangedWithOfficeRulesPresent(t *testing.T) {
	const evaluations = `SELECT rule_id, rule_version, subject_key, status, values_json, difference_paise, missing_json FROM evaluations
		WHERE rule_id NOT LIKE 'RECON-OFF-%' ORDER BY rule_id, subject_key, seq`
	for _, c := range webSamples {
		t.Run(c, func(t *testing.T) {
			input := filepath.Join(runData, c)
			args := func(db string, extra ...string) []string {
				return append(append([]string{"run", "--db", db, "--input", input, "--vocab", vocabDir, "--profiles", profDir, "--include-testing"}, extra...),
					func() []string {
						if a, ok := webSampleRun[c]; ok {
							return a
						}
						return []string{"--tax-year", "2025-26"}
					}()...)
			}
			empty := newDB(t)
			if r := cmd(t, args(empty)...); r.code > exitFindings {
				t.Fatalf("run with an empty Master: exit %d\n%s", r.code, r.stderr)
			}

			o := setUpOfficeReturns(t)
			if s := saveRule(t, o.db, o.office, "A. Staff", rowRule("RECON-OFF-101")); s.code != 0 {
				t.Fatalf("rule-save: %s", s.stderr)
			}
			if r := cmd(t, args(o.db, "--office-rules", o.office, "--office-profiles", o.profiles)...); r.code > exitFindings {
				t.Fatalf("run with office documents and rules: exit %d\n%s", r.code, r.stderr)
			}
			a, b := query(t, empty, evaluations), query(t, o.db, evaluations)
			if len(a) == 0 {
				t.Fatal("no built-in evaluations to compare")
			}
			if len(a) != len(b) {
				t.Fatalf("%d built-in evaluations with an empty Master, %d with office documents and rules", len(a), len(b))
			}
			for i := range a {
				if !slices.Equal(a[i], b[i]) {
					t.Fatalf("evaluation %d differs:\n  empty Master:   %v\n  office present: %v", i, a[i], b[i])
				}
			}
		})
	}
}

const salesSummaryDoc = `{"by":"A. Partner","code":"sales_summary","name":"Sales summary","category":"Books","description":"Invoice totals kept in the office",
 "applicable_for":["csv"],"status":"active",
 "record_type":{"name":"sales_summary_row","description":"Rows of the sales summary","columns":[
  {"name":"invoice_no","label":"Invoice number","type":"text","key":true,"required":true},
  {"name":"customer","label":"Customer","type":"text","key":false,"required":false},
  {"name":"taxable_value","label":"Taxable value","type":"amount","key":false,"required":false},
  {"name":"igst","label":"IGST","type":"amount","key":false,"required":false}]}}`

// AC-17: a Staff user's match_records rule between a document the Partner added and a built-in record type
// (the sales register) is saved as testing, with generated YAML the strict loader accepts and a pass and a
// fail fixture that run in the engine.
func TestMatchRecordsRuleBetweenOfficeAndBuiltInRows(t *testing.T) {
	db, office := newDB(t), officeRoot(t)
	if r, out := docCmd[docSaveOut](t, "document-save", db, salesSummaryDoc); r.code != 0 || !out.Saved {
		t.Fatalf("document-save: %s", r.stderr)
	}
	row := func(recordType, file, invoice string, taxable, igst string, withCustomer bool) rules.FormRecord {
		rec := rules.FormRecord{RecordType: recordType, Period: "2026-08", File: file, Locator: "$.rows[" + invoice + "]",
			Fields:  []rules.FormNamed{{Name: "invoice_no", Value: invoice}},
			Amounts: []rules.FormNamed{{Name: "taxable_value", Value: taxable}, {Name: "igst", Value: igst}}}
		if withCustomer {
			rec.Fields = append(rec.Fields, rules.FormNamed{Name: "customer", Value: "Orion Retail"})
		}
		return rec
	}
	form := rules.Form{
		Mode: "create", ID: "RECON-OFF-111", Module: "office", Category: "R1", TaxYears: []string{"2026-27"}, ValidFrom: "2026-04-01",
		Title:    "Sales summary against the sales register (office test)",
		RuleText: "Compares each invoice of the sales summary with the sales register, invoice by invoice: whether it is in both, and its taxable value and IGST.",
		Notes:    "Synthetic test rule between a document added in the Document Master and a built-in record type.",
		Inputs: []rules.FormInput{
			{Name: "a", Kind: "record_type", Ref: "sales_summary_row", Label: "Sales summary"},
			{Name: "b", Kind: "record_type", Ref: "sales_b2b_invoice", Label: "Sales register"},
		},
		Check:    rules.FormCheck{Type: "match_records", Tolerance: "1.00", CompareAmounts: []string{"taxable_value", "igst"}},
		Severity: "high", ReviewDocument: "sales_summary",
		Law: []rules.FormLaw{{Act: "Central Goods and Services Tax Act, 2017", Section: "37"}},
		Fixtures: []rules.FormFixture{
			{Kind: "pass", Name: "same_in_both", Description: "The invoice is the same in both documents",
				Records: []rules.FormRecord{row("sales_summary_row", "summary.csv", "INV-1", "1000.00", "180.00", true), row("sales_b2b_invoice", "register.json", "INV-1", "1000.00", "180.00", false)}},
			{Kind: "fail", Name: "igst_differs", Description: "The IGST of the invoice differs by Rs 20",
				Records: []rules.FormRecord{row("sales_summary_row", "summary_bad.csv", "INV-2", "1000.00", "180.00", true), row("sales_b2b_invoice", "register_bad.json", "INV-2", "1000.00", "160.00", false)}},
		},
	}
	s := saveRule(t, db, office, "A. Staff", form)
	if s.code != 0 {
		t.Fatalf("rule-save: exit %d\n%s%s", s.code, s.stdout, s.stderr)
	}
	if got := query(t, db, `SELECT s.status FROM rule_state s WHERE s.rule_id = 'RECON-OFF-111'`); got[0][0] != "testing" {
		t.Errorf("status after the save = %v", got)
	}
	data, err := os.ReadFile(filepath.Join(office, "2026-27", "office", "RECON-OFF-111", "rule.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`module: "office"`, `record_type: "sales_summary_row"`, `record_type: "sales_b2b_invoice"`, `type: "match_records"`, `compare_amounts`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("the generated rule lacks %s:\n%s", want, data)
		}
	}
	for _, kind := range []string{"pass/same_in_both", "fail/igst_differs"} {
		if _, err := os.Stat(filepath.Join(office, "2026-27", "office", "RECON-OFF-111", "fixtures", filepath.FromSlash(kind)+".yaml")); err != nil {
			t.Errorf("fixture %s was not written: %v", kind, err)
		}
	}
	// the fixtures run in the engine at the save: one that the rule gets wrong is refused, nothing stored
	wrong := form
	wrong.ID = "RECON-OFF-112"
	wrong.Fixtures = []rules.FormFixture{form.Fixtures[1], form.Fixtures[1]}
	wrong.Fixtures[0].Kind, wrong.Fixtures[0].Name = "pass", "should_pass"
	before := ruleCount(t, db)
	if s := saveRule(t, db, office, "A. Staff", wrong); s.code != exitInput || ruleCount(t, db) != before {
		t.Errorf("a pass fixture the rule fails on: exit %d, rules stored %s -> %s", s.code, before, ruleCount(t, db))
	}
}

// AC-18: an amount column used where text is needed, and a text column where an amount is needed, are
// each refused on the field of the form, naming the column.
func TestOfficeRuleAmountAndTextColumnsAreNotMixedUp(t *testing.T) {
	o := setUpOfficeReturns(t)
	problems := func(f rules.Form) []rules.Problem {
		t.Helper()
		before := ruleCount(t, o.db) // taken here: a rule saved earlier in the test is not a refusal that stored something
		s := saveRule(t, o.db, o.office, "A. Staff", f)
		if s.code != exitInput || ruleCount(t, o.db) != before {
			t.Fatalf("exit %d, rules stored %s -> %s; want a refusal that stores nothing\n%s", s.code, before, ruleCount(t, o.db), s.stderr)
		}
		return refusedProblems(t, s)
	}
	has := func(ps []rules.Problem, field, code, column string) bool {
		for _, p := range ps {
			if p.Field == field && p.Code == code && strings.Contains(p.Message, column) {
				return true
			}
		}
		return false
	}
	// a text test (equals) on an amount column
	text := rowRule("RECON-OFF-121")
	text.Check = rules.FormCheck{Type: "present_together", When: []rules.FormCondition{{Input: "a", Field: "net_amount", Is: "equals", Value: "100"}}}
	if ps := problems(text); !has(ps, "check.when[0].field", "NOT_TEXT", "net_amount") {
		t.Errorf("a text test on an amount column: %+v", ps)
	}
	// zero on an amount column is fine, and on a text column the loader's own message is the answer
	zero := rowRule("RECON-OFF-122")
	zero.Check = rules.FormCheck{Type: "present_together", When: []rules.FormCondition{{Input: "a", Field: "net_amount", Is: "zero"}}}
	netRow := func(file, ref, net string) rules.FormRecord {
		return rules.FormRecord{RecordType: "sales_return_row", Period: "2026-08", File: file, Locator: "$.Returns[" + ref + "]",
			Fields:  []rules.FormNamed{{Name: "ref_no", Value: ref}},
			Amounts: []rules.FormNamed{{Name: "net_amount", Value: net}}}
	}
	zero.Fixtures = []rules.FormFixture{
		{Kind: "pass", Name: "has_net", Description: "A return with a net amount", Records: []rules.FormRecord{netRow("ok.json", "SR-1", "1000.00")}},
		{Kind: "fail", Name: "zero_net", Description: "A return with no net amount", Records: []rules.FormRecord{netRow("zero.json", "SR-2", "0.00")}},
	}
	if s := saveRule(t, o.db, o.office, "A. Staff", zero); s.code != 0 {
		t.Errorf("zero on an amount column was refused: %s", s.stderr)
	}
	// match_records: an amount compared as text, and text compared as an amount
	two := rowRule("RECON-OFF-123")
	two.Inputs = append(two.Inputs, rules.FormInput{Name: "b", Kind: "record_type", Ref: "sales_return_row", Label: "Returns again"})
	two.Check = rules.FormCheck{Type: "match_records", CompareFields: []string{"net_amount"}, CompareAmounts: []string{"customer"}}
	ps := problems(two)
	if !has(ps, "check.compare_fields[0]", "NOT_TEXT", "net_amount") || !has(ps, "check.compare_amounts[0]", "NOT_AN_AMOUNT", "customer") {
		t.Errorf("amount and text columns swapped in match_records: %+v", ps)
	}
}
