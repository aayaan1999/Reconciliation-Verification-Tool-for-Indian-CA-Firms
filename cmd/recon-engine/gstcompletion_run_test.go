package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
)

// SPEC-018: a run on the September 2026 sample pack (testdata/gst_completion_sep2026).

// completionClient is the pack company, with the GSTIN written with a valid check character
// (the pack's own GSTIN fails it; see testdata/gst_completion_sep2026/README.md).
const completionClient = `{"legal_name": "Shree Sample Traders Pvt Ltd", "pan": "aapcs9876k", "constitution": "company",
 "gstins": [{"gstin": "08AAPCS9876K1Z6", "registration_type": "regular", "primary": true}],
 "address": {"city": "Jaipur", "state": "08", "pin": "302001"}}`

// completionGST are the pack GST files this spec reads for the exact comparison (the pack also
// holds a draft GSTR-1 of October and a GSTR-2B of March, read in the whole-folder run).
var completionGST = []string{
	"gstr3b_092026.json", "einvoice_irn_register_sep2026.csv", "ewaybill_register_sep2026.csv", "ims_actions_092026.csv",
	"tally_sales_register_sep2026.csv", "tally_purchase_register_sep2026.csv", "gstr1_092026.json", "gstr2b_092026.json",
}

// compEval is one result of a rule, as the JSON report gives it.
type compEval struct {
	RuleID     string `json:"rule_id"`
	Title      string `json:"title"`
	Code       string `json:"code"`
	CodeText   string `json:"code_text"`
	Difference int64  `json:"difference_paise"`
	Subject    struct {
		Period   string `json:"period"`
		MatchKey string `json:"match_key"`
	} `json:"subject"`
	Values []struct {
		Input   string `json:"input"`
		Label   string `json:"label"`
		Locator struct {
			FileName string `json:"file_name"`
			Path     string `json:"path"`
		} `json:"locator"`
		Value struct {
			Kind  string `json:"kind"`
			Paise int64  `json:"paise"`
			Text  string `json:"text"`
		} `json:"value"`
	} `json:"values"`
}

type compReport struct {
	Findings []struct {
		Findings []compEval `json:"findings"`
	} `json:"findings"`
	Passed      []compEval `json:"passed"`
	CannotCheck []struct {
		RuleID  string `json:"rule_id"`
		Subject struct {
			Period   string `json:"period"`
			MatchKey string `json:"match_key"`
		} `json:"subject"`
		Missing []struct {
			DocumentType string `json:"document_type"`
			Fact         string `json:"fact"`
		} `json:"missing"`
	} `json:"cannot_check"`
	Files []struct {
		File         string `json:"file"`
		DocumentType string `json:"document_type"`
		RecordCount  int    `json:"record_count"`
		Errors       []any  `json:"errors"`
	} `json:"files"`
}

func (r compReport) failed() []compEval {
	var out []compEval
	for _, g := range r.Findings {
		out = append(out, g.Findings...)
	}
	return out
}

// runCompletion puts the named files of the pack (all of them when names is empty) into a
// folder, adds the pack company as a client, runs every rule with --skip-unread and returns
// the report and what was printed.
func runCompletion(t *testing.T, names []string) (compReport, string) {
	t.Helper()
	in := t.TempDir()
	if len(names) == 0 {
		entries, err := os.ReadDir(filepath.Join(completionDir, "inputs"))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			names = append(names, e.Name())
		}
	}
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(completionDir, "inputs", n))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(in, n), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	db := newDB(t)
	if r := withStdin(t, completionClient, "client-add", "--db", db, "--tax-year", "2026-27"); r.code != exitOK {
		t.Fatalf("client-add: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	jsonFile := filepath.Join(t.TempDir(), "report.json")
	r := cmd(t, "run", "--db", db, "--input", in, "--tax-year", "2026-27", "--vocab", vocabDir, "--profiles", profDir,
		"--include-testing", "--client", "1", "--skip-unread", "--json", jsonFile)
	if r.code != exitOK && r.code != exitFindings {
		t.Fatalf("run: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	b, err := os.ReadFile(jsonFile)
	if err != nil {
		t.Fatal(err)
	}
	var rep compReport
	if err := json.Unmarshal(b, &rep); err != nil {
		t.Fatal(err)
	}
	return rep, r.stdout
}

// newRule reports whether id is one of the rules of this spec.
func newRule(id string) bool {
	return id >= "RECON-GST-030" && id <= "RECON-GST-035"
}

type expectedAnswerKey struct {
	File        string `yaml:"file"`
	Doc         string `yaml:"doc"`
	Status      string `yaml:"status"`
	Reason      string `yaml:"reason"`
	AmountPaise int64  `yaml:"amount_paise"`
}

type expectedResults struct {
	Period   string `yaml:"period"`
	Findings []struct {
		Rule            string             `yaml:"rule"`
		Key             string             `yaml:"key"`
		Code            string             `yaml:"code"`
		DifferencePaise int64              `yaml:"difference_paise"`
		AnswerKey       *expectedAnswerKey `yaml:"answer_key"`
	} `yaml:"findings"`
	Passes []struct {
		Rule string `yaml:"rule"`
		Key  string `yaml:"key"`
	} `yaml:"passes"`
	Exclusions []struct {
		AnswerKey string `yaml:"answer_key"`
		Reason    string `yaml:"reason"`
	} `yaml:"exclusions"`
}

func loadExpectedResults(t *testing.T) expectedResults {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(completionDir, "expected_results.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var e expectedResults
	if err := yaml.Unmarshal(b, &e); err != nil {
		t.Fatal(err)
	}
	return e
}

// answerKeyRows reads one CSV of the pack outputs as rows of column name to value.
func answerKeyRows(t *testing.T, file string) []map[string]string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(completionDir, "answer_key", file))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(bytes.NewReader(b)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]string
	for _, r := range rows[1:] {
		m := map[string]string{}
		for i, h := range rows[0] {
			if i < len(r) {
				m[h] = r[i]
			}
		}
		out = append(out, m)
	}
	return out
}

// answerKeyHolds reports whether the pack output has the row an expected result names: the
// status, the reason code and, where given, the difference.
func answerKeyHolds(t *testing.T, k expectedAnswerKey) bool {
	t.Helper()
	for _, row := range answerKeyRows(t, k.File) {
		var status, amount string
		switch {
		case strings.HasPrefix(k.File, "R02"):
			if row["head"] != k.Doc {
				continue
			}
			status, amount = row["check"], row["books_minus_3b"]
		case strings.HasPrefix(k.File, "R05"):
			if row["head"] != k.Doc {
				continue
			}
			status, amount = row["status"], row["excess_claimed"]
		default:
			if row["left_doc_no"] != k.Doc && row["right_doc_no"] != k.Doc {
				continue
			}
			status, amount = row["status"], row["diff_value"]
			if k.Reason != "" && !strings.Contains(row["reason_codes"], k.Reason) {
				continue
			}
		}
		if status != k.Status {
			continue
		}
		if k.AmountPaise != 0 {
			p, err := money.Parse(strings.TrimPrefix(amount, "-"), money.UnitRupees)
			if err != nil || int64(p) != k.AmountPaise {
				continue
			}
		}
		return true
	}
	return false
}

// SPEC-018 AC-5, AC-9: on the pack each rule gives exactly the results of the checked-in
// answer list, each finding with both values, the difference and both sources, and nothing
// else; every result named in the pack outputs is there with that status and reason; and no
// text of a finding says "should" or states a correct value.
func TestGSTCompletionPackResults(t *testing.T) {
	want := loadExpectedResults(t)
	rep, _ := runCompletion(t, completionGST)

	type key struct{ rule, key string }
	got := map[key]compEval{}
	for _, f := range rep.failed() {
		if newRule(f.RuleID) {
			got[key{f.RuleID, f.Subject.MatchKey}] = f
			if f.Subject.Period != want.Period {
				t.Errorf("%s %s: period %s", f.RuleID, f.Subject.MatchKey, f.Subject.Period)
			}
		}
	}
	for _, w := range want.Findings {
		f, ok := got[key{w.Rule, w.Key}]
		if !ok {
			t.Errorf("no finding of %s for %s", w.Rule, w.Key)
			continue
		}
		delete(got, key{w.Rule, w.Key})
		if f.Code != w.Code || f.Difference != w.DifferencePaise {
			t.Errorf("%s %s: code %q difference %d, want %q %d", w.Rule, w.Key, f.Code, f.Difference, w.Code, w.DifferencePaise)
		}
		// Both values, the difference and the sources.
		if len(f.Values) == 0 {
			t.Errorf("%s %s: no values", w.Rule, w.Key)
		}
		var a, b int
		for _, v := range f.Values {
			if v.Input != "sum" && (v.Locator.FileName == "" || v.Locator.Path == "") {
				t.Errorf("%s %s: the value %q has no source", w.Rule, w.Key, v.Label)
			}
			switch {
			case strings.HasPrefix(v.Input, "a."):
				a++
			case strings.HasPrefix(v.Input, "b."):
				b++
			}
		}
		switch w.Code {
		case "", "AMOUNT_DIFF":
			if a == 0 || b == 0 {
				t.Errorf("%s %s: shows %d values of a and %d of b, want both", w.Rule, w.Key, a, b)
			}
		case "ONLY_IN_A", "DUPLICATE_IN_A":
			if a == 0 {
				t.Errorf("%s %s: no value of a", w.Rule, w.Key)
			}
		case "ONLY_IN_B":
			if b == 0 {
				t.Errorf("%s %s: no value of b", w.Rule, w.Key)
			}
		}
	}
	for k := range got {
		t.Errorf("an unexpected finding: %s %s", k.rule, k.key)
	}

	wantPass := map[key]bool{}
	for _, p := range want.Passes {
		wantPass[key{p.Rule, p.Key}] = true
	}
	for _, p := range rep.Passed {
		if !newRule(p.RuleID) {
			continue
		}
		k := key{p.RuleID, p.Subject.MatchKey}
		if !wantPass[k] {
			t.Errorf("an unexpected pass: %s %s", k.rule, k.key)
		}
		delete(wantPass, k)
	}
	for k := range wantPass {
		t.Errorf("expected a pass of %s for %s", k.rule, k.key)
	}
	// All six rules ran with every document present: nothing is left unchecked.
	for _, c := range rep.CannotCheck {
		if newRule(c.RuleID) {
			t.Errorf("%s cannot check %s: %+v", c.RuleID, c.Subject.MatchKey, c.Missing)
		}
	}

	// The pack outputs are the answer key: each named row is there with that status and reason.
	for _, w := range want.Findings {
		if w.AnswerKey != nil && !answerKeyHolds(t, *w.AnswerKey) {
			t.Errorf("%s %s: the pack output %s has no row %+v", w.Rule, w.Key, w.AnswerKey.File, *w.AnswerKey)
		}
	}
	if len(want.Exclusions) < 10 {
		t.Errorf("only %d exclusions are named", len(want.Exclusions))
	}

	// AC-9: wording. Rule files, and every finding title, result text and label.
	for _, id := range []string{"RECON-GST-030", "RECON-GST-031", "RECON-GST-032", "RECON-GST-033", "RECON-GST-034", "RECON-GST-035"} {
		b, err := os.ReadFile(filepath.Join(repo, "content", "rules", "2026-27", "gst", id, "rule.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if bad := rules.BannedIn(string(b)); len(bad) > 0 {
			t.Errorf("%s says %v", id, bad)
		}
	}
	for _, f := range rep.failed() {
		if !newRule(f.RuleID) {
			continue
		}
		text := f.Title + " " + f.CodeText
		for _, v := range f.Values {
			text += " " + v.Label + " " + v.Value.Text
		}
		if bad := rules.BannedIn(text); len(bad) > 0 {
			t.Errorf("%s %s says %v", f.RuleID, f.Subject.MatchKey, bad)
		}
		if strings.Contains(strings.ToLower(text), "correct") {
			t.Errorf("%s %s names a correct value: %s", f.RuleID, f.Subject.MatchKey, text)
		}
	}
	// The status and the IMS action are shown as written in the file.
	shown := map[string]string{
		"RECON-GST-033|INV/26-27/0112": "CNL", "RECON-GST-034|INV/26-27/0112": "Cancelled",
		"RECON-GST-035|08AABCK1006F1Z2|KT-901": "Rejected", "RECON-GST-035|08AABCS1010K1Z4|SL-3301": "Pending",
		"RECON-GST-035|08AABCR1001A1Z2|RS-CN-12": "Accepted",
	}
	for _, f := range rep.failed() {
		k := f.RuleID + "|" + f.Subject.MatchKey
		w, ok := shown[k]
		if !ok {
			continue
		}
		found := false
		for _, v := range f.Values {
			found = found || v.Value.Text == w
		}
		if !found {
			t.Errorf("%s does not show %q as written: %+v", k, w, f.Values)
		}
		delete(shown, k)
	}
	if len(shown) > 0 {
		t.Errorf("not found: %v", shown)
	}
}

// SPEC-018 AC-6: the whole pack folder is uploaded with --skip-unread: every GST file is read,
// the TDS, payroll, bank and XML files are listed as not read, and the rules that need only
// the read files give results.
func TestGSTCompletionWholePackFolder(t *testing.T) {
	rep, out := runCompletion(t, nil)
	read := map[string]string{}
	for _, f := range rep.Files {
		read[f.File] = f.DocumentType
		// The pack's March GSTR-2B is of tax year 2025-26: it is read, and reported as of another year.
		if len(f.Errors) > 0 && f.File != "gstr2b_032026.json" {
			t.Errorf("%s: %v", f.File, f.Errors)
		}
		if f.File == "gstr2b_032026.json" && (len(f.Errors) != 1 || !strings.Contains(fmt.Sprint(f.Errors[0]), "tax year 2025-26")) {
			t.Errorf("%s: errors %v, want one naming tax year 2025-26", f.File, f.Errors)
		}
	}
	wantRead := map[string]string{
		"gstr3b_092026.json": "gstr3b", "einvoice_irn_register_sep2026.csv": "einvoice", "ewaybill_register_sep2026.csv": "ewaybill",
		"ims_actions_092026.csv": "ims", "tally_sales_register_sep2026.csv": "sales_register", "tally_purchase_register_sep2026.csv": "purchase_register",
		"gstr1_092026.json": "gstr1", "gstr1_102026_draft.json": "gstr1", "gstr2b_092026.json": "gstr2b", "gstr2b_032026.json": "gstr2b",
	}
	if !maps.Equal(read, wantRead) {
		t.Errorf("files read %v, want %v", read, wantRead)
	}
	notRead := []string{
		"bank_meta.json", "bank_statement_sep2026.csv", "epf_ecr_sep2026.txt", "esic_contribution_sep2026.csv", "form168_part1_extract.csv",
		"payroll_register_sep2026.xlsx", "tally_bank_book_sep2026.csv", "tally_sales_register_sep2026.xml", "tally_tds_challans_q2.csv",
		"tally_tds_ledger_q2_ty2026-27.csv", "tally_tds_receivable_q2.csv", "tds_statement_form140_q2_deductee_records.csv", "traces_oltas_challan_status.csv",
	}
	_, section, ok := strings.Cut(out, "== Files NOT read (13) ==")
	if !ok {
		t.Fatalf("no list of 13 files not read:\n%s", out)
	}
	for _, n := range notRead {
		if !strings.Contains(section, n+":") {
			t.Errorf("%s is not listed as not read", n)
		}
	}
	if n := strings.Count(section, "\n  "); n != len(notRead) {
		t.Errorf("%d lines in the list of files not read, want %d", n, len(notRead))
	}
	// The rules that need only the read files give the results they give on the GST files alone.
	want := loadExpectedResults(t)
	failed := map[string]bool{}
	for _, f := range rep.failed() {
		if newRule(f.RuleID) && f.Subject.Period == want.Period {
			failed[f.RuleID+"|"+f.Subject.MatchKey] = true
		}
	}
	for _, w := range want.Findings {
		if !failed[w.Rule+"|"+w.Key] {
			t.Errorf("with the whole folder, no finding of %s for %s", w.Rule, w.Key)
		}
		delete(failed, w.Rule+"|"+w.Key)
	}
	for k := range failed {
		t.Errorf("with the whole folder, an unexpected finding %s", k)
	}
}

// SPEC-018 AC-7: with the GSTR-3B JSON missing, 030, 031 and 032 name gstr3b as the missing
// document and find nothing wrong; the rules that need no 3B are not affected.
func TestGSTCompletionMissingGSTR3B(t *testing.T) {
	var names []string
	for _, n := range completionGST {
		if n != "gstr3b_092026.json" {
			names = append(names, n)
		}
	}
	rep, _ := runCompletion(t, names)
	needs3B := []string{"RECON-GST-030", "RECON-GST-031", "RECON-GST-032"}
	for _, f := range rep.failed() {
		if slices.Contains(needs3B, f.RuleID) || f.RuleID == "RECON-GST-020" {
			t.Errorf("%s found %s with no GSTR-3B", f.RuleID, f.Subject.MatchKey)
		}
	}
	for _, p := range rep.Passed {
		if slices.Contains(needs3B, p.RuleID) {
			t.Errorf("%s passed %s with no GSTR-3B", p.RuleID, p.Subject.MatchKey)
		}
	}
	named := map[string][]string{}
	for _, c := range rep.CannotCheck {
		if slices.Contains(needs3B, c.RuleID) {
			for _, m := range c.Missing {
				named[c.RuleID] = append(named[c.RuleID], m.DocumentType)
			}
		}
	}
	for _, id := range needs3B {
		if !slices.Contains(named[id], "gstr3b") {
			t.Errorf("%s does not name gstr3b as missing: %v", id, named[id])
		}
		for _, d := range named[id] {
			if d != "gstr3b" {
				t.Errorf("%s names %s as missing", id, d)
			}
		}
	}
	var n int
	for _, f := range rep.failed() {
		if f.RuleID == "RECON-GST-033" || f.RuleID == "RECON-GST-034" || f.RuleID == "RECON-GST-035" {
			n++
		}
	}
	if n != 9+4+9 {
		t.Errorf("%d findings of 033, 034 and 035, want 22", n)
	}
}

// Determinism: two runs on the same files give the same results, in the same order.
func TestGSTCompletionDeterministic(t *testing.T) {
	a, _ := runCompletion(t, completionGST)
	b, _ := runCompletion(t, completionGST)
	ja, _ := json.Marshal(a.Findings)
	jb, _ := json.Marshal(b.Findings)
	if !bytes.Equal(ja, jb) || len(ja) < 1000 {
		t.Errorf("two runs differ (or are empty): %d and %d bytes", len(ja), len(jb))
	}
}
