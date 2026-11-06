package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// SPEC-020 AC-5..AC-8: results of RECON-TDS-010..013 and RECON-BNK-010..011 on the sample pack
// (testdata/tds_bank_sep2026).

// tdsBankFiles are the pack files of this spec.
var tdsBankFiles = []string{
	"tally_tds_ledger_q2_ty2026-27.csv", "tds_statement_form140_q2_deductee_records.csv", "tally_tds_challans_q2.csv",
	"traces_oltas_challan_status.csv", "tally_tds_receivable_q2.csv", "form168_part1_extract.csv",
	"tally_bank_book_sep2026.csv", "bank_statement_sep2026.csv", "bank_meta.json",
}

func tdsBankRule(id string) bool {
	return strings.HasPrefix(id, "RECON-TDS-01") || strings.HasPrefix(id, "RECON-BNK-01")
}

// runTDSBank puts the named files of a folder into a new folder and runs every rule over them
// with --skip-unread; it returns the JSON report and what was printed.
func runTDSBank(t *testing.T, dir string, names []string) (compReport, string) {
	t.Helper()
	in := t.TempDir()
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(in, n), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	jsonFile := filepath.Join(t.TempDir(), "report.json")
	r := cmd(t, "run", "--db", newDB(t), "--input", in, "--tax-year", "2026-27", "--vocab", vocabDir, "--profiles", profDir,
		"--include-testing", "--skip-unread", "--json", jsonFile)
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

// What the tool computes on the pack. Each finding is a row of the pack's own outputs
// (answer_key/R11, R12, R14, R18) that is not MATCHED, except where a comment says the tool
// computes one more difference than the pack lists. The pack's remarks that are judgments
// (short deduction, late deposit and interest, income not booked, unpresented cheque,
// uncleared deposit, unidentified receipt, unbooked interest or charge) are not reproduced.
var tdsBankWant = []struct {
	rule, key, code string
	answerFile      string // the pack output that has the row ("" when the pack does not list it)
	answerText      string // text that identifies the row in that file
	answerStatus    string // the pack's status for it
}{
	{"RECON-TDS-010", "Beta Consultants|2026-08-05", "FIELD_DIFF", "R11_TDS_Ledger_vs_Form140.csv", "Beta Consultants", "MISMATCH"},
	{"RECON-TDS-010", "Epsilon Transport|2026-09-20", "ONLY_IN_A", "R11_TDS_Ledger_vs_Form140.csv", "Epsilon Transport", "MISSING_IN_B"},
	{"RECON-TDS-010", "Eta Printers|2026-09-25", "FIELD_DIFF", "R11_TDS_Ledger_vs_Form140.csv", "Eta Printers", "MISMATCH"},
	{"RECON-TDS-010", "Zeta Advisors|2026-09-28", "ONLY_IN_B", "R11_TDS_Ledger_vs_Form140.csv", "Zeta Advisors", "MISSING_IN_A"},
	{"RECON-TDS-011", "0510308|2026-09-07|00021", "AMOUNT_DIFF", "R12_TDS_Challan_vs_OLTAS.csv", "0510308-07092026-00021", "MISMATCH"},
	{"RECON-TDS-011", "0510308|2026-10-07|00041", "ONLY_IN_A", "R12_TDS_Challan_vs_OLTAS.csv", "0510308-07102026-00041", "MISSING_IN_B"},
	// R12 also lists challan 00041 as claimed in the statement at 7,000 against 6,000 in the books.
	{"RECON-TDS-012", "0510308-07102026-00041", "AMOUNT_DIFF", "R12_TDS_Challan_vs_OLTAS.csv", "0510308-07102026-00041", "MISSING_IN_B"},
	// Extra: R12 shows challan 00034 as matched (its books amount equals OLTAS). The tool also
	// compares the books' challan with the TDS the statement rows deposit with it (15,675 against
	// 15,000), which the pack shows only in its "claimed_in_statement" column.
	{"RECON-TDS-012", "0510308-12102026-00034", "AMOUNT_DIFF", "", "", ""},
	{"RECON-TDS-013", "AHMG33333C|393(1) [ex-194J technical]", "AMOUNT_DIFF", "R14_TDS_Receivable_vs_Form168.csv", "Gujarat Infotech", "MISMATCH"},
	{"RECON-TDS-013", "DELN44444D|393(1) [ex-194Q]", "ONLY_IN_B", "R14_TDS_Receivable_vs_Form168.csv", "NOIDA NETWORKS", "MISSING_IN_A"},
	{"RECON-TDS-013", "MUMM22222B|393(1) [ex-194Q]", "ONLY_IN_A", "R14_TDS_Receivable_vs_Form168.csv", "Mehta Retail LLP", "MISSING_IN_B"},
	{"RECON-BNK-010", "0510308-07092026-00021", "AMOUNT_DIFF", "R18_BankBook_vs_Statement.csv", "PYMT-202", "MISMATCH"},
	{"RECON-BNK-010", "ICICN26293001", "AMOUNT_DIFF", "R18_BankBook_vs_Statement.csv", "RCPT-102", "MISMATCH"},
	{"RECON-BNK-010", "004512", "ONLY_IN_A", "R18_BankBook_vs_Statement.csv", "PYMT-204", "TIMING_DIFFERENCE"},
	{"RECON-BNK-010", "778899", "ONLY_IN_A", "R18_BankBook_vs_Statement.csv", "RCPT-103", "TIMING_DIFFERENCE"},
	{"RECON-BNK-010", "CHG-OCT26", "ONLY_IN_B", "R18_BankBook_vs_Statement.csv", "CHG-OCT26", "MISSING_IN_A"},
	{"RECON-BNK-010", "INT-OCT26", "ONLY_IN_B", "R18_BankBook_vs_Statement.csv", "INT-OCT26", "MISSING_IN_A"},
	{"RECON-BNK-010", "UTIBN26301122", "ONLY_IN_B", "R18_BankBook_vs_Statement.csv", "UTIBN26301122", "MISSING_IN_A"},
}

// The results that pass: the figures agree (the pack's MATCHED rows, and its rows whose only
// remark is a judgment the tool does not make: Gamma's short deduction and Delta's late deposit).
var tdsBankPasses = []struct{ rule, key string }{
	{"RECON-TDS-010", "Alpha Contractors|2026-07-10"}, {"RECON-TDS-010", "Gamma Tech Services|2026-08-12"}, {"RECON-TDS-010", "Delta Estates|2026-08-31"},
	{"RECON-TDS-011", "0510308|2026-08-07|00011"}, {"RECON-TDS-011", "0510308|2026-10-12|00034"},
	{"RECON-TDS-012", "0510308-07082026-00011"}, {"RECON-TDS-012", "0510308-07092026-00021"},
	{"RECON-TDS-013", "JPRB11111A|393(1) [ex-194Q]"},
	{"RECON-BNK-010", "SBIN226276001"}, {"RECON-BNK-010", "HDFCN26280455"}, {"RECON-BNK-010", "CPIN26100800012345"},
	{"RECON-BNK-011", ""},
}

// tdsBankAnswerRow reports whether a pack output has a row with the status that names text.
func tdsBankAnswerRow(t *testing.T, file, text, status string) bool {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(tdsBankDir, "answer_key", file))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(bytes.NewReader(b)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	si := slices.Index(rows[0], "status")
	if si < 0 {
		t.Fatalf("%s has no status column", file)
	}
	for _, r := range rows[1:] {
		if r[si] == status && strings.Contains(strings.Join(r, "|"), text) {
			return true
		}
	}
	return false
}

// SPEC-020 AC-5, AC-6: on the pack each rule gives the results of the answer list and nothing
// else, each finding with its values and both sources; every finding the pack lists is there
// with the pack's status; and no wording judges, states a correct value or classifies a line.
func TestTDSBankPackResults(t *testing.T) {
	rep, _ := runTDSBank(t, filepath.Join(tdsBankDir, "inputs"), tdsBankFiles)
	type key struct{ rule, key string }
	got := map[key]compEval{}
	for _, f := range rep.failed() {
		if tdsBankRule(f.RuleID) {
			got[key{f.RuleID, f.Subject.MatchKey}] = f
		}
	}
	for _, w := range tdsBankWant {
		f, ok := got[key{w.rule, w.key}]
		if !ok {
			t.Errorf("%s %q: no finding", w.rule, w.key)
			continue
		}
		delete(got, key{w.rule, w.key})
		if f.Code != w.code {
			t.Errorf("%s %q: code %s, want %s", w.rule, w.key, f.Code, w.code)
		}
		if len(f.Values) == 0 {
			t.Errorf("%s %q: no values shown", w.rule, w.key)
		}
		for _, v := range f.Values {
			if v.Locator.FileName == "" || v.Locator.Path == "" {
				t.Errorf("%s %q: value %s has no source", w.rule, w.key, v.Label)
			}
		}
		if w.answerFile != "" && !tdsBankAnswerRow(t, w.answerFile, w.answerText, w.answerStatus) {
			t.Errorf("%s %q: %s has no %s row for %q", w.rule, w.key, w.answerFile, w.answerStatus, w.answerText)
		}
	}
	for k := range got {
		t.Errorf("unexpected finding %s %q", k.rule, k.key)
	}
	passed := map[key]bool{}
	for _, p := range rep.Passed {
		if tdsBankRule(p.RuleID) {
			passed[key{p.RuleID, p.Subject.MatchKey}] = true
		}
	}
	for _, w := range tdsBankPasses {
		if !passed[key{w.rule, w.key}] {
			t.Errorf("%s %q: not passed", w.rule, w.key)
		}
	}
	if len(passed) != len(tdsBankPasses) {
		t.Errorf("%d results pass, want %d", len(passed), len(tdsBankPasses))
	}
	banned := []string{"should", "correct", "unidentified", "unpresented", "uncleared", "timing", "short deduction", "late", "interest"}
	for _, f := range rep.failed() {
		if !tdsBankRule(f.RuleID) {
			continue
		}
		text := strings.ToLower(f.Title + " " + f.CodeText)
		for _, v := range f.Values {
			text += " " + strings.ToLower(v.Label)
		}
		for _, b := range banned {
			if strings.Contains(text, b) {
				t.Errorf("%s %q: the wording says %q", f.RuleID, f.Subject.MatchKey, b)
			}
		}
	}
}

// SPEC-020 AC-7: with one document of a rule missing, the rule is reported as cannot check
// and finds nothing wrong.
func TestTDSBankMissingDocument(t *testing.T) {
	rep, _ := runTDSBank(t, filepath.Join(tdsBankDir, "inputs"), []string{"tally_bank_book_sep2026.csv", "tds_statement_form140_q2_deductee_records.csv"})
	for _, f := range rep.failed() {
		if tdsBankRule(f.RuleID) {
			t.Errorf("%s %q: a finding with one document missing", f.RuleID, f.Subject.MatchKey)
		}
	}
	seen := map[string]bool{}
	for _, c := range rep.CannotCheck {
		seen[c.RuleID] = true
	}
	for _, id := range []string{"RECON-BNK-010", "RECON-BNK-011", "RECON-TDS-010", "RECON-TDS-012"} {
		if !seen[id] {
			t.Errorf("%s is not reported as cannot check", id)
		}
	}
}

// SPEC-020 AC-8: the whole pack folder with --skip-unread reads every GST, TDS and bank file
// and lists the payroll, EPF, ESIC and Tally XML files as not read.
func TestTDSBankWholePackFolder(t *testing.T) {
	dir := filepath.Join(completionDir, "inputs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	rep, out := runTDSBank(t, dir, names)
	notRead := []string{"epf_ecr_sep2026.txt", "esic_contribution_sep2026.csv", "payroll_register_sep2026.xlsx", "tally_sales_register_sep2026.xml"}
	for _, n := range notRead {
		if !strings.Contains(out, n) {
			t.Errorf("%s is not listed in the output", n)
		}
	}
	read := map[string]bool{}
	for _, f := range rep.Files {
		read[f.File] = true
		// The pack's March GSTR-2B belongs to another tax year; SPEC-018 reads it with errors too.
		if len(f.Errors) != 0 && f.File != "gstr2b_032026.json" {
			t.Errorf("%s was read with errors", f.File)
		}
	}
	for _, n := range append(slices.Clone(tdsBankFiles), "gstr3b_092026.json", "einvoice_irn_register_sep2026.csv", "ewaybill_register_sep2026.csv",
		"ims_actions_092026.csv", "tally_sales_register_sep2026.csv", "tally_purchase_register_sep2026.csv", "gstr1_092026.json", "gstr2b_092026.json") {
		if !read[n] {
			t.Errorf("%s was not read", n)
		}
	}
	for _, n := range notRead {
		if read[n] {
			t.Errorf("%s was read, but this spec has no format for it", n)
		}
	}
}
