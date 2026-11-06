package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SPEC-019: amounts written in US dollars, converted at the fixed rate in content/currency/usd-rate.yaml.
// The sample is testdata/dollar_amounts (invented names and figures). The Excel workbook is written
// from the CSV with: go test ./cmd/recon-engine -run TestDollar -update

var dollarDir = filepath.Join(repo, "testdata", "dollar_amounts")

const rateFileHead = "schema: \"recon.currency.usd/1\"\n"

// rateDir writes a currency folder holding a usd-rate.yaml with the given body ("" writes no file).
func rateDir(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if body != "" {
		if err := os.WriteFile(filepath.Join(dir, "usd-rate.yaml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// dollarInput puts the named sample files into a new folder.
func dollarInput(t *testing.T, names ...string) string {
	t.Helper()
	in := t.TempDir()
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(dollarDir, n))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(in, n), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return in
}

// runDollar runs the bank comparison of the sample over in, with the currency folder given, and
// returns the report.
func runDollar(t *testing.T, db, in, currencyDir string) compReport {
	t.Helper()
	jsonFile := filepath.Join(t.TempDir(), "report.json")
	args := []string{"run", "--db", db, "--input", in, "--tax-year", "2026-27", "--vocab", vocabDir, "--profiles", profDir,
		"--include-testing", "--comparison", "bank_book_vs_statement", "--comparisons", filepath.Join(repo, "content", "comparisons", "comparisons.yaml"),
		"--json", jsonFile}
	if currencyDir != "" {
		args = append(args, "--currency", currencyDir)
	}
	r := cmd(t, args...)
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
	return rep
}

// bnk010 gives the results of RECON-BNK-010 by key: "pass", "fail:<code>" or "cannot".
func bnk010(rep compReport) map[string]string {
	out := map[string]string{}
	for _, p := range rep.Passed {
		if p.RuleID == "RECON-BNK-010" {
			out[p.Subject.MatchKey] = "pass"
		}
	}
	for _, f := range rep.failed() {
		if f.RuleID == "RECON-BNK-010" {
			out[f.Subject.MatchKey] = "fail:" + f.Code
		}
	}
	for _, c := range rep.CannotCheck {
		if c.RuleID == "RECON-BNK-010" {
			out[c.Subject.MatchKey] = "cannot"
		}
	}
	return out
}

func eq(got, want map[string]string) bool {
	if len(got) != len(want) {
		return false
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}

// SPEC-019 AC-9 (with AC-2): an Excel cell and a CSV field holding "$1,250.50" give the same paise.
func TestDollarExcelAndCSVAgree(t *testing.T) {
	book := filepath.Join(dollarDir, "bank_book.xlsx")
	if *update {
		writeSheetBook(t, filepath.Join(dollarDir, "bank_book.csv"), book, "Bank Book", []string{"payment"}, false)
	}
	rates := rateDir(t, rateFileHead+"usd_rate: \"83.2500\"\neffective_from: \"2026-04-01\"\nnote: \"\"\n")
	want := map[string]string{"R1": "pass", "R2": "pass", "R3": "pass"}
	for _, bookFile := range []string{"bank_book.csv", "bank_book.xlsx"} {
		rep := runDollar(t, newDB(t), dollarInput(t, bookFile, "bank_statement.csv"), rates)
		if got := bnk010(rep); !eq(got, want) {
			t.Errorf("%s at 83.2500: %v, want %v (dollar amounts converted to the paise of the statement)", bookFile, got, want)
		}
		for _, f := range rep.Files {
			if len(f.Errors) != 0 {
				t.Errorf("%s: errors %v", f.File, f.Errors)
			}
		}
	}
}

// SPEC-019 AC-7, AC-8: changing the rate file is the only change needed; the same file read at the
// new rate gives new paise (a new draft), and the run keeps the rate and its date.
func TestDollarRateChangeAndStoredRate(t *testing.T) {
	db := newDB(t)
	in := dollarInput(t, "bank_book.csv", "bank_statement.csv")
	r8325 := rateDir(t, rateFileHead+"usd_rate: \"83.2500\"\neffective_from: \"2026-04-01\"\nnote: \"\"\n")
	r84 := rateDir(t, rateFileHead+"usd_rate: \"84\"\neffective_from: \"2026-05-01\"\nnote: \"\"\n")

	if got := bnk010(runDollar(t, db, in, r8325)); !eq(got, map[string]string{"R1": "pass", "R2": "pass", "R3": "pass"}) {
		t.Fatalf("at 83.2500: %v", got)
	}
	// The same files, the same database, only the rate file changed: the two dollar lines now differ.
	got := bnk010(runDollar(t, db, in, r84))
	if !eq(got, map[string]string{"R1": "fail:AMOUNT_DIFF", "R2": "pass", "R3": "fail:AMOUNT_DIFF"}) {
		t.Errorf("at 84.0000: %v", got)
	}
	// And back again: the first rate gives the first result.
	if got := bnk010(runDollar(t, db, in, r8325)); !eq(got, map[string]string{"R1": "pass", "R2": "pass", "R3": "pass"}) {
		t.Errorf("back at 83.2500: %v", got)
	}
	// AC-8: each run records the rate and its date.
	rows := query(t, db, `SELECT usd_rate, usd_rate_effective, usd_rate_problem FROM runs ORDER BY id`)
	want := [][]string{{"83.2500", "2026-04-01", ""}, {"84.0000", "2026-05-01", ""}, {"83.2500", "2026-04-01", ""}}
	if len(rows) != len(want) {
		t.Fatalf("runs: %v", rows)
	}
	for i := range want {
		if strings.Join(rows[i], "|") != strings.Join(want[i], "|") {
			t.Errorf("run %d stored %v, want %v", i+1, rows[i], want[i])
		}
	}
	// A run of the same file at the same rate is not a new draft; at another rate it is.
	drafts := query(t, db, `SELECT draft_no, read_with FROM document_versions WHERE read_with <> '' ORDER BY id`)
	if len(drafts) == 0 {
		t.Errorf("no draft recorded the rate it was read with")
	}
}

// SPEC-019 AC-6: with no usable rate a dollar amount is "cannot check" with the reason, never zero
// and never a mismatch; rupee amounts and a run with rupees only need no file.
func TestDollarWithoutUsableRate(t *testing.T) {
	in := dollarInput(t, "bank_book.csv", "bank_statement.csv")
	cases := []struct {
		name, body, reason string
	}{
		{"file missing", "", "dollar rate not set"},
		{"file empty", " \n", "dollar rate not set"},
		{"rate empty", rateFileHead + "usd_rate: \"\"\n", "dollar rate not set"},
		{"not a number", rateFileHead + "usd_rate: \"eighty\"\n", "dollar rate not valid"},
		{"zero", rateFileHead + "usd_rate: \"0\"\n", "dollar rate not valid"},
		{"negative", rateFileHead + "usd_rate: \"-83\"\n", "dollar rate not valid"},
	}
	for _, c := range cases {
		rep := runDollar(t, newDB(t), in, rateDir(t, c.body))
		got := bnk010(rep)
		// R2 is in rupees in both documents and still passes; the two dollar lines cannot be checked.
		if !eq(got, map[string]string{"R1": "cannot", "R2": "pass", "R3": "cannot"}) {
			t.Errorf("%s: %v, want the dollar lines cannot-check and the rupee line passing", c.name, got)
		}
		reasons := 0
		for _, f := range rep.Files {
			if len(f.Errors) != 0 {
				t.Errorf("%s: %s read with errors %v", c.name, f.File, f.Errors)
			}
		}
		raw, _ := json.Marshal(rep)
		if n := strings.Count(string(raw), c.reason); n == 0 {
			t.Errorf("%s: the report never says %q", c.name, c.reason)
		} else {
			reasons += n
		}
		if len(rep.failed()) != 0 {
			for _, f := range rep.failed() {
				if f.RuleID == "RECON-BNK-010" {
					t.Errorf("%s: a mismatch was reported for %s with no rate", c.name, f.Subject.MatchKey)
				}
			}
		}
		_ = reasons
	}
	// A run with rupee amounts only works without the rate file (no --currency given, no file there).
	rupees := t.TempDir()
	for _, n := range []string{"bank_book.csv", "bank_statement.csv"} {
		b, _ := os.ReadFile(filepath.Join(dollarDir, n))
		s := string(b)
		s = strings.NewReplacer("\"$1,250.50\"", "1250.50", "$80.25", "80.25").Replace(s)
		if err := os.WriteFile(filepath.Join(rupees, n), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rep := runDollar(t, newDB(t), rupees, "")
	if got := bnk010(rep); len(got) != 3 {
		t.Errorf("rupees only: %v", got)
	}
	for _, f := range rep.Files {
		if len(f.Errors) != 0 {
			t.Errorf("rupees only: %s %v", f.File, f.Errors)
		}
	}
}
