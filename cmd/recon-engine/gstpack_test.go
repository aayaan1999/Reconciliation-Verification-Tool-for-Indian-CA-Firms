package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/profile"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// SPEC-004 AC-27: on the synthetic GST pack (testdata/gst_pack_aug2026, a fictional
// company whose books are correct and whose portal returns carry planted errors listed in
// ANSWER_KEY.xlsx), invoice matching reports exactly the planted errors it covers, and
// every other invoice matches.
func TestGSTPackMatchesTheAnswerKey(t *testing.T) {
	for _, c := range []struct {
		folder string
		rule   string
		want   map[string]string // match key → code, for every result that is not MATCHED
		total  int               // every result of the rule, MATCHED included
	}{
		{"gst-sales", "RECON-GST-001", map[string]string{
			"DBS/26-27/0458": "ONLY_IN_A",      // E01 not in GSTR-1
			"DBS/26-27/0462": "AMOUNT_DIFF",    // E02 taxable value keyed as 4,05,000
			"DBS/26-27/0466": "FIELD_DIFF",     // E03 another taxpayer's GSTIN
			"DBS/26-27/0470": "FIELD_DIFF",     // E04 place of supply and tax heads
			"DBS/26-27/0453": "DUPLICATE_IN_B", // E05 uploaded twice
			"DBS/26-27/0475": "FIELD_DIFF",     // E06 old rate 28%
		}, 23},
		{"gst-purchase", "RECON-GST-013", map[string]string{
			"24NQPCK9772R1Z1|KPS/UPS/0231":   "ONLY_IN_A",   // F01 not in GSTR-2B
			"33UVVCM8633H1Z8|MPPL/1189":      "AMOUNT_DIFF", // F02 lower value reported
			"29KLJCK1232C1ZY|KCD/26-27/0412": "KEY_DIFF",    // F03 number keyed differently
			"27BTUCB2264K1Z2|BCL/2627/0912":  "ONLY_IN_B",   // F04 not in the books
			"29KLJCK1232C1ZY|KCD/26-27/0431": "FIELD_DIFF",  // F06 date one day later
		}, 13}, // 12 invoices in the books and 12 in GSTR-2B; F03 pairs two of them, F04 is in GSTR-2B only
		// SPEC-015: rule-book rules RC-GST-003 and RC-GST-011 made runnable.
		{"gst-sales", "RECON-GST-003", map[string]string{
			"27|18": "AMOUNT_DIFF", // E08 cash sale DBS/26-27/0465 left out of the POS 27 line
			"33|18": "ONLY_IN_B",   // consequential to E09: the B2C large invoice put in Table 7
		}, 3}, // POS 30 at 18% matches
		{"gst-sales", "RECON-GST-011", map[string]string{
			"B2C|847130|18": "ONLY_IN_A", // E11 HSN cut to four digits in Table 12 ...
			"B2C|8471|18":   "ONLY_IN_B", // ... so each is on one side only
		}, 11}, // 6 B2B lines and 3 other B2C lines match, credit notes deducted
	} {
		t.Run(c.folder, func(t *testing.T) {
			jsonFile := filepath.Join(t.TempDir(), "report.json")
			r := cmd(t, "run", "--db", newDB(t), "--input", filepath.Join(runData, c.folder), "--tax-year", "2026-27",
				"--vocab", vocabDir, "--profiles", profDir, "--include-testing", "--json", jsonFile)
			if r.code != exitFindings {
				t.Fatalf("exit %d\n%s%s", r.code, r.stdout, r.stderr)
			}
			b, err := os.ReadFile(jsonFile)
			if err != nil {
				t.Fatal(err)
			}
			var rep struct {
				Findings []struct {
					Findings []struct {
						RuleID  string `json:"rule_id"`
						Code    string `json:"code"`
						Subject struct {
							MatchKey string `json:"match_key"`
						} `json:"subject"`
					} `json:"findings"`
				} `json:"findings"`
				Passed []struct {
					RuleID string `json:"rule_id"`
					Code   string `json:"code"`
				} `json:"passed"`
			}
			if err := json.Unmarshal(b, &rep); err != nil {
				t.Fatal(err)
			}
			got := map[string]string{}
			for _, g := range rep.Findings {
				for _, f := range g.Findings {
					if f.RuleID == c.rule {
						got[f.Subject.MatchKey] = f.Code
					}
				}
			}
			matched := 0
			for _, p := range rep.Passed {
				if p.RuleID == c.rule && p.Code == "MATCHED" {
					matched++
				}
			}
			for k, code := range c.want {
				if got[k] != code {
					t.Errorf("%s: got %q, want %q", k, got[k], code)
				}
			}
			for k, code := range got {
				if _, planted := c.want[k]; !planted {
					t.Errorf("%s: %s, but the answer key plants no error there", k, code)
				}
			}
			if matched+len(got) != c.total {
				t.Errorf("%d matched + %d findings, want %d results in all", matched, len(got), c.total)
			}
		})
	}
}

// comparisonReport is the part of the run JSON that a comparison type changes.
type comparisonReport struct {
	Counts struct {
		Checks int `json:"checks"`
	} `json:"counts"`
	Comparison struct {
		ID        string `json:"id"`
		Documents []struct {
			DocumentType string `json:"document_type"`
			Present      bool   `json:"present"`
		} `json:"documents"`
	} `json:"comparison"`
	Totals []struct {
		DocumentType string           `json:"document_type"`
		Rows         int              `json:"rows"`
		Amounts      map[string]int64 `json:"amounts"`
	} `json:"totals"`
}

// runComparison runs one comparison type on the named files of the GST pack, copied into
// one folder, and returns the exit code, the text report, the raw JSON and its parsed form.
func runComparison(t *testing.T, comparison string, files ...string) (result, string, comparisonReport) {
	t.Helper()
	in := t.TempDir()
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(runData, f))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(in, filepath.Base(f)), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	jsonFile := filepath.Join(t.TempDir(), "report.json")
	args := []string{"run", "--db", newDB(t), "--input", in, "--tax-year", "2026-27", "--vocab", vocabDir, "--profiles", profDir,
		"--include-testing", "--comparisons", filepath.Join(repo, "content", "comparisons", "comparisons.yaml"), "--json", jsonFile}
	if comparison != "" { // "" is a run of every rule, as the screens make
		args = append(args, "--comparison", comparison)
	}
	r := cmd(t, args...)
	b, err := os.ReadFile(jsonFile)
	if err != nil {
		t.Fatalf("exit %d, no JSON report: %v\n%s%s", r.code, err, r.stdout, r.stderr)
	}
	var rep comparisonReport
	if err := json.Unmarshal(b, &rep); err != nil {
		t.Fatal(err)
	}
	return r, string(b), rep
}

// SPEC-004 AC-27: the sales-vs-purchase summary shows both sides' totals and, having no
// rule, produces no finding. A GSTR-1 in the same engagement is not part of this summary,
// so its totals are left out.
func TestSalesVsPurchaseSummary(t *testing.T) {
	r, raw, rep := runComparison(t, "gst_sales_vs_purchase", "gst-sales/SALES_REG_Aug2026.json", "gst-purchase/PURCH_REG_Aug2026.json",
		"gst-sales/GSTR1_Aug2026.json")
	if r.code != exitOK {
		t.Fatalf("exit %d, want %d (no finding)\n%s%s", r.code, exitOK, r.stdout, r.stderr)
	}
	if rep.Counts.Checks != 0 || strings.Contains(raw, `"rule_id"`) {
		t.Errorf("a totals-only comparison evaluated %d checks: %.300s", rep.Counts.Checks, raw)
	}
	if !strings.Contains(r.stdout, "not compared") {
		t.Errorf("the text report does not say the totals are not compared:\n%s", r.stdout)
	}
	got := map[string]bool{}
	for _, tot := range rep.Totals {
		got[tot.DocumentType] = true
		if tot.Rows == 0 || tot.Amounts["taxable_value"] <= 0 {
			t.Errorf("%s: %d rows, taxable value %d paise", tot.DocumentType, tot.Rows, tot.Amounts["taxable_value"])
		}
	}
	if !got["sales_register"] || !got["purchase_register"] || len(rep.Totals) != 2 {
		t.Errorf("totals for %v, want the sales and purchase registers", got)
	}
}

// Types of check removed from the screens (30 October 2026): a run of every rule on the GST
// files evaluates the invoice matching rules and shows the row totals of every uploaded
// document a totals type reads, with no type of check named in the report.
func TestEveryRuleRunShowsGSTTotals(t *testing.T) {
	r, raw, rep := runComparison(t, "", "gst-sales/SALES_REG_Aug2026.json", "gst-purchase/PURCH_REG_Aug2026.json",
		"gst-sales/GSTR1_Aug2026.json")
	if r.code != exitFindings {
		t.Fatalf("exit %d, want %d (the sales register differs from GSTR-1)\n%s%s", r.code, exitFindings, r.stdout, r.stderr)
	}
	if rep.Comparison.ID != "" || strings.Contains(r.stdout, "Comparison: ") {
		t.Errorf("a run of every rule names a type of check: %q", rep.Comparison.ID)
	}
	if !strings.Contains(raw, `"RECON-GST-001"`) {
		t.Errorf("the invoice matching rule did not run: %.300s", raw)
	}
	got := map[string]bool{}
	for _, tot := range rep.Totals {
		got[tot.DocumentType] = true
	}
	if !got["sales_register"] || !got["purchase_register"] {
		t.Errorf("totals for %v, want at least the sales and purchase registers", got)
	}
	if !strings.Contains(r.stdout, "not compared") {
		t.Errorf("the text report does not show the totals:\n%s", r.stdout)
	}
}

// SPEC-004 AC-26: a run for a comparison type evaluates only its rules, and the report says
// which of its documents are present and which are missing.
func TestComparisonEvaluatesOnlyItsRules(t *testing.T) {
	_, raw, rep := runComparison(t, "gst_sales_vs_gstr1", "gst-sales/SALES_REG_Aug2026.json", "gst-purchase/PURCH_REG_Aug2026.json",
		"gst-purchase/GSTR2B_Aug2026.json")
	present := map[string]bool{}
	for _, d := range rep.Comparison.Documents {
		present[d.DocumentType] = d.Present
	}
	if rep.Comparison.ID != "gst_sales_vs_gstr1" || !present["sales_register"] || present["gstr1"] || len(present) != 2 {
		t.Errorf("comparison %q documents %v, want sales register present and GSTR-1 missing", rep.Comparison.ID, present)
	}
	// The type's rules: B2B invoices, and since SPEC-015 B2C large, B2C small and the HSN summary.
	its := []string{"RECON-GST-001", "RECON-GST-002", "RECON-GST-003", "RECON-GST-011"}
	for _, m := range regexp.MustCompile(`"rule_id":"([^"]+)"`).FindAllStringSubmatch(raw, -1) {
		if !slices.Contains(its, m[1]) {
			t.Errorf("rule %s evaluated in a sales-vs-GSTR-1 run", m[1])
		}
	}
}

// SPEC-015 AC-1: the GSTR-1 and sales register samples are read by version 2 of their formats,
// which supersedes version 1 (not refused as ambiguous). Version 2 reads every B2B invoice
// exactly as version 1 does (its expected file, unchanged), plus the B2C and HSN rows.
// -update writes version 2's expected files.
func TestGSTFormatsVersion2(t *testing.T) {
	v, err := vocab.LoadDir(vocabDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := profile.LoadDir(profDir, v)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		doc, b2b string
		extra    []string // the record types version 2 adds
	}{
		{"gstr1", "gstr1_b2b_invoice", []string{"gstr1_b2cs_line", "gstr1_hsn_line"}},
		{"sales_register", "sales_b2b_invoice", []string{"sales_b2cl_invoice", "sales_b2cs_line", "sales_hsn_line"}},
	} {
		dir := filepath.Join(profDir, "generic", c.doc)
		samples, err := filepath.Glob(filepath.Join(dir, "v2", "samples", "*.json"))
		if err != nil || len(samples) < 2 {
			t.Fatalf("%s v2 samples: %v %v", c.doc, samples, err)
		}
		for _, sample := range samples {
			name := strings.TrimSuffix(filepath.Base(sample), ".json")
			p, got := readSample(t, set, sample)
			// SPEC-018: GSTR-1 version 3 supersedes version 2 and reads these samples too (it adds the flat
			// outward rows); version 2 stays installed and is tested on its samples by profile-test.
			if p.Supersedes == "" || (p.Version != "2" && !(c.doc == "gstr1" && p.Version == "3")) {
				t.Fatalf("%s was read by %s", sample, p.Key())
			}
			body := profile.ExpectedFile(p, got)
			if _, rest, ok := bytes.Cut(body, []byte("\n")); ok {
				body = append([]byte("# What reading the sample gives (SPEC-015): written by go test ./cmd/recon-engine -run TestGSTFormatsVersion2 -update\n# and checked against version 1's expected file for the B2B invoices on every test run.\n"), rest...)
			}
			expected := filepath.Join(dir, "v2", "expected", name+".facts.yaml")
			if *update && p.Version == "2" {
				if err := os.WriteFile(expected, body, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			v1Data, err := os.ReadFile(filepath.Join(dir, "v1", "expected", name+".facts.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			v2, v1 := readSampleExpected(t, expected, body), readSampleExpected(t, "v1", v1Data)
			only := func(e sampleExpected, rt string) []string {
				var out []string
				for _, l := range e.lines() {
					if strings.HasPrefix(l, "row "+rt+" ") {
						out = append(out, l)
					}
				}
				return out
			}
			if g, w := only(v2, c.b2b), only(v1, c.b2b); !slices.Equal(g, w) || len(g) == 0 {
				t.Errorf("%s: version 2 reads %d B2B rows, version 1 %d; they differ", sample, len(g), len(w))
			}
			for _, rt := range c.extra {
				if name == strings.TrimSuffix(filepath.Base(samples[0]), ".json") && len(only(v2, rt)) == 0 {
					t.Errorf("%s: version 2 reads no %s rows", sample, rt)
				}
			}
		}
	}
}
