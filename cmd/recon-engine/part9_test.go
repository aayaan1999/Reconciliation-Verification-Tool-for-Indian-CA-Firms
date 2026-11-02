package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The acceptance tests of SPEC-004 part 9: choosing the rules of a type of check, and
// keeping a client's latest result per type.

var comparisonsYAML = filepath.Join(repo, "content", "comparisons", "comparisons.yaml")

// AC-31: only the chosen rules of the type run; the others are listed as not selected and
// are not counted as checks.
func TestRunSelectedRules(t *testing.T) {
	jsonFile := filepath.Join(t.TempDir(), "report.json")
	r := runCmd(t, newDB(t), filepath.Join(runData, "taxsuite"), "--include-testing",
		"--comparison", "taxsuite", "--comparisons", comparisonsYAML, "--rules", "RECON-DEP-001", "--json", jsonFile)
	if r.code != exitFindings {
		t.Fatalf("exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	b, err := os.ReadFile(jsonFile)
	if err != nil {
		t.Fatal(err)
	}
	var rep struct {
		NotSelected []struct {
			RuleID string `json:"rule_id"`
			Title  string `json:"title"`
		} `json:"not_selected"`
	}
	if err := json.Unmarshal(b, &rep); err != nil {
		t.Fatal(err)
	}
	if len(rep.NotSelected) != 1 || rep.NotSelected[0].RuleID != "RECON-DEP-002" || rep.NotSelected[0].Title == "" {
		t.Errorf("not_selected = %+v, want RECON-DEP-002 with its title", rep.NotSelected)
	}
	if strings.Contains(string(b), `"rule_id":"RECON-DEP-002","rule_version"`) {
		t.Error("RECON-DEP-002 was evaluated although it was not selected")
	}
	for _, want := range []string{"1 rule of this comparison not run: not selected", "== Not run: not selected ==", "[RECON-DEP-002]"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("text report lacks %q:\n%s", want, r.stdout)
		}
	}

	// Without --rules every rule of the type runs and nothing is listed as not selected.
	all := runCmd(t, newDB(t), filepath.Join(runData, "taxsuite"), "--include-testing", "--comparison", "taxsuite", "--comparisons", comparisonsYAML)
	if strings.Contains(all.stdout, "not selected") {
		t.Errorf("a run of every rule mentions rules not selected:\n%s", all.stdout)
	}
}

// AC-32: a rule outside the type, --rules without a type and an empty choice are refused,
// and nothing is stored.
func TestRunSelectedRulesRefused(t *testing.T) {
	for _, c := range []struct {
		name, code string
		extra      []string
	}{
		{"rule of another type", "RULE_NOT_IN_COMPARISON", []string{"--comparison", "taxsuite", "--rules", "RECON-DEP-001,RECON-GST-001"}},
		{"no type", "RULES_NEED_COMPARISON", []string{"--rules", "RECON-DEP-001"}},
		{"nothing chosen", "NO_RULES_SELECTED", []string{"--comparison", "taxsuite", "--rules", " , "}},
	} {
		t.Run(c.name, func(t *testing.T) {
			db := newDB(t)
			args := append([]string{"--include-testing", "--comparisons", comparisonsYAML}, c.extra...)
			r := runCmd(t, db, filepath.Join(runData, "taxsuite"), args...)
			if r.code != exitInput || !strings.Contains(r.stderr, c.code) {
				t.Errorf("exit %d, want %d with %s\n%s", r.code, exitInput, c.code, r.stderr)
			}
			if n := one(t, db, `SELECT COUNT(*) FROM runs`); n != "0" {
				t.Errorf("%s runs stored after a refusal", n)
			}
		})
	}
}

// AC-33 and AC-34: one client compared on two types keeps the latest result of each; a
// later run of one type leaves the other's alone; the board's counts are their sum.
func TestClientResultPerType(t *testing.T) {
	db := newDB(t)
	if r := withStdin(t, deccanJSON, "client-add", "--db", db, "--tax-year", "2026-27"); r.code != exitOK {
		t.Fatalf("client-add: exit %d\n%s", r.code, r.stderr)
	}
	runType := func(folder, comparison string) {
		t.Helper()
		r := cmd(t, "run", "--db", db, "--input", filepath.Join(runData, folder), "--tax-year", "2026-27", "--vocab", vocabDir,
			"--profiles", profDir, "--include-testing", "--client", "1", "--comparison", comparison, "--comparisons", comparisonsYAML)
		if r.code != exitFindings {
			t.Fatalf("%s: exit %d\n%s%s", comparison, r.code, r.stdout, r.stderr)
		}
	}
	type result struct {
		Client struct {
			Counts map[string]int `json:"counts"`
		} `json:"client"`
		Results []struct {
			ComparisonID string `json:"comparison_id"`
			LastRunOn    string `json:"last_run_on"`
			Report       struct {
				Counts map[string]int `json:"counts"`
			} `json:"report"`
		} `json:"results"`
	}
	read := func() result {
		t.Helper()
		out := filepath.Join(t.TempDir(), "result.json")
		if r := cmd(t, "result", "--db", db, "--client", "1", "--tax-year", "2026-27", "--json", out); r.code != exitOK {
			t.Fatalf("result: exit %d\n%s", r.code, r.stderr)
		}
		b, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		var res result
		if err := json.Unmarshal(b, &res); err != nil {
			t.Fatal(err)
		}
		return res
	}

	runType("gst-sales", "gst_sales_vs_gstr1")
	runType("gst-purchase", "gst_purchase_vs_gstr2b")
	res := read()
	if len(res.Results) != 2 || res.Results[0].ComparisonID != "gst_purchase_vs_gstr2b" || res.Results[1].ComparisonID != "gst_sales_vs_gstr1" {
		t.Fatalf("results = %+v, want one per type, sorted by type", res.Results)
	}
	purchase, sales := res.Results[0].Report.Counts, res.Results[1].Report.Counts
	if sales["checks"] != 23 || sales["fail"] != 6 || purchase["fail"] != 5 || res.Results[0].LastRunOn == "" {
		t.Errorf("sales %v, purchase %v, last run %q", sales, purchase, res.Results[0].LastRunOn)
	}
	for _, k := range []string{"checks", "pass", "fail", "cannot_check"} {
		if got, want := res.Client.Counts[k], sales[k]+purchase[k]; got != want {
			t.Errorf("board %s = %d, want %d (the sum over the types)", k, got, want)
		}
	}

	// Running the sales type again keeps the purchase result as it was.
	runType("gst-sales", "gst_sales_vs_gstr1")
	again := read()
	if len(again.Results) != 2 || again.Results[0].Report.Counts["fail"] != purchase["fail"] {
		t.Errorf("after a second sales run: %+v", again.Results)
	}
	if n := one(t, db, `SELECT COUNT(*) FROM runs WHERE comparison_id = 'gst_sales_vs_gstr1'`); n != "2" {
		t.Errorf("%s sales runs stored, want 2", n)
	}
}

// The Client file opens on a tax year with a run and suggests the files' own tax year:
// inspect says which tax year a recognised file is for, and result lists the years in which
// the client has runs.
func TestTaxYearHints(t *testing.T) {
	out := filepath.Join(t.TempDir(), "inspect.json")
	if r := cmd(t, "inspect", "--input", filepath.Join(runData, "gst-sales", "GSTR1_Aug2026.json"), "--profiles", profDir, "--vocab", vocabDir, "--json", out); r.code != exitOK {
		t.Fatalf("inspect: exit %d\n%s", r.code, r.stderr)
	}
	var in struct {
		TaxYear string `json:"tax_year"`
	}
	if b, err := os.ReadFile(out); err != nil || json.Unmarshal(b, &in) != nil || in.TaxYear != "2026-27" {
		t.Errorf("inspect tax_year = %q, want 2026-27 (%v)", in.TaxYear, err)
	}

	db := newDB(t)
	if r := withStdin(t, deccanJSON, "client-add", "--db", db, "--tax-year", "2026-27"); r.code != exitOK {
		t.Fatalf("client-add: exit %d\n%s", r.code, r.stderr)
	}
	r := cmd(t, "run", "--db", db, "--input", filepath.Join(runData, "gst-sales"), "--tax-year", "2026-27", "--vocab", vocabDir,
		"--profiles", profDir, "--include-testing", "--client", "1", "--comparison", "gst_sales_vs_gstr1", "--comparisons", comparisonsYAML)
	if r.code != exitFindings {
		t.Fatalf("run: exit %d\n%s", r.code, r.stderr)
	}
	res := filepath.Join(t.TempDir(), "result.json")
	if r := cmd(t, "result", "--db", db, "--client", "1", "--tax-year", "2025-26", "--json", res); r.code != exitOK {
		t.Fatalf("result: exit %d\n%s", r.code, r.stderr)
	}
	var got struct {
		TaxYear     string   `json:"tax_year"`
		RunTaxYears []string `json:"run_tax_years"`
		Results     []any    `json:"results"`
	}
	if b, err := os.ReadFile(res); err != nil || json.Unmarshal(b, &got) != nil {
		t.Fatal(err)
	}
	if got.TaxYear != "2025-26" || len(got.Results) != 0 || len(got.RunTaxYears) != 1 || got.RunTaxYears[0] != "2026-27" {
		t.Errorf("result for 2025-26: %+v, want no results there and the run listed under 2026-27", got)
	}
}

// Every part of a file that was not read says why: a row the format leaves out (with the
// condition it fails), a column of a row that was read, or another part of the file.
func TestUnreadReasons(t *testing.T) {
	jsonFile := filepath.Join(t.TempDir(), "report.json")
	r := cmd(t, "run", "--db", newDB(t), "--input", filepath.Join(runData, "gst-sales"), "--tax-year", "2026-27",
		"--vocab", vocabDir, "--profiles", profDir, "--json", jsonFile)
	if r.code > exitFindings {
		t.Fatalf("exit %d\n%s", r.code, r.stderr)
	}
	b, err := os.ReadFile(jsonFile)
	if err != nil {
		t.Fatal(err)
	}
	var rep struct {
		Files []struct {
			File   string `json:"file"`
			Unread []struct {
				Path, Kind, Row, Reason string
			} `json:"unread"`
		} `json:"files"`
	}
	if err := json.Unmarshal(b, &rep); err != nil {
		t.Fatal(err)
	}
	for _, f := range rep.Files {
		if f.File != "SALES_REG_Aug2026.json" {
			continue
		}
		left := map[string]string{} // row → reason
		kinds := map[string]int{}
		for _, u := range f.Unread {
			kinds[u.Kind]++
			if u.Kind == "row_left_out" {
				left[u.Row] = u.Reason
			}
			if u.Kind == "" || u.Reason == "" {
				t.Errorf("%s: no reason", u.Path)
			}
		}
		// 36 lines: 26 B2B invoices read; 5 B2C small, 1 B2C large, 1 export, 2 credit notes and 1 cancelled left out.
		if len(left) != 10 {
			t.Errorf("%d rows left out, want 10: %v", len(left), left)
		}
		// The row's own value is client data, so the reason names the column, not the value.
		want := `row left out: .supply_type is another value; this format reads only rows where .supply_type is "B2B"`
		found := false
		for _, why := range left {
			found = found || why == want
		}
		if !found || kinds["column_not_used"] == 0 {
			t.Errorf("reasons %v, kinds %v; want a credit note left out with %q and columns not used", left, kinds, want)
		}
		return
	}
	t.Fatal("no sales register in the report")
}
