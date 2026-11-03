package profile

import (
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
)

// SPEC-011 part 2: what the one-workbook-per-form layout added to the profile language, shown
// on a JSON document of the same shape (labelled rows): the context read from the row that
// meets conditions, equals_year, not_equals and yes_no. All values are illustrative.

const labelledProfile = `schema: "recon.profile/1"
profile_id: "test.suite.labelled"
profile_version: "1"
source_system: "Test suite"
document_type: "computation"
file_type: "json"
regime: "ITA1961"
detect:
  - json_exists: { path: "$.assessee" }
context:
  tax_year:
    rows: "$.assessee[*]"
    where:
      - { path: ".k", equals: "Previous Year" }
    json_path: ".v"
  pan:
    rows: "$.assessee[*]"
    where:
      - { path: ".k", equals: "Pan" }
    json_path: ".v"
  period: { const: "FY" }
facts:
  - fact: "computation.allowed_depreciation"
    rows: "$.credit[*]"
    where:
      - { path: ".ay", equals_year: "assessment_year" }
    json_path: ".cf"
    combine: "one"
    unit: "rupees"
  - fact: "computation.employee_count"
    rows: "$.credit[*]"
    where:
      - { path: ".ay", equals_year: "tax_year" }
    json_path: ".cf"
    combine: "one"
  - fact: "computation.schedule_total"
    rows: "$.blocks[*]"
    where:
      - { path: ".b", not_equals: "Total" }
    json_path: ".amt"
    combine: "sum"
    unit: "rupees"
  - fact: "computation.is_audited"
    rows: "$.flags[*]"
    where:
      - { path: ".k", equals: "Audited" }
    json_path: ".v"
    combine: "one"
    yes_no: true
`

const labelledDoc = `{
  "assessee": [{"k": "Name", "v": "Illustrative Traders"}, {"k": "Pan", "v": "ABCDE1234F"}, {"k": "Previous Year", "v": "2025-26"}],
  "credit": [{"ay": "2024-25", "cf": 10}, {"ay": "2025-26", "cf": 100}, {"ay": "2026-27", "cf": 250}],
  "blocks": [{"b": "A", "amt": 1}, {"b": "B", "amt": 2}, {"b": "Total", "amt": 3}, {"amt": 50}],
  "flags": [{"k": "Audited", "v": "yes"}]
}`

func TestLabelledRows(t *testing.T) {
	p := load(t, labelledProfile)
	r := extract(t, p, labelledDoc, Source{FileName: "labelled.json"})
	if r.Report.HasErrors() {
		t.Fatalf("errors: %v", r.Report.Errors)
	}
	if r.Context.TaxYear.String() != "2025-26" || r.Context.PAN != "ABCDE1234F" {
		t.Errorf("context %+v, want the tax year and PAN of their rows", r.Context)
	}
	got := map[string]string{}
	for _, f := range r.Facts {
		got[f.Name] = domain.FormatValue(f.Value)
	}
	for name, want := range map[string]string{
		"computation.allowed_depreciation": "250.00", // the row of the assessment year, 2026-27
		"computation.employee_count":       "100",    // the row of the tax year, 2025-26
		"computation.schedule_total":       "3.00",   // A and B, not the Total row, and not the row with no b
		"computation.is_audited":           "true",
	} {
		if got[name] != want {
			t.Errorf("%s = %q, want %q", name, got[name], want)
		}
	}

	// Two rows meeting a context condition are not chosen between.
	two := strings.Replace(labelledDoc, `{"k": "Name", "v": "Illustrative Traders"}`, `{"k": "Previous Year", "v": "2024-25"}`, 1)
	if r := extract(t, p, two, Source{FileName: "two.json"}); !r.Report.HasErrors() || (len(r.Report.Errors) != 1 || !strings.Contains(r.Report.Errors[0].Msg, "2 entries meet the conditions")) {
		t.Errorf("two Previous Year rows: %v", r.Report.Errors)
	}
	// yes_no reads only Yes or No.
	maybe := strings.Replace(labelledDoc, `"v": "yes"`, `"v": "maybe"`, 1)
	if r := extract(t, p, maybe, Source{FileName: "maybe.json"}); !r.Report.HasErrors() || !strings.Contains(r.Report.Errors[0].Msg, "expected Yes or No") {
		t.Errorf("yes_no with other text: %v", r.Report.Errors)
	}
}

func TestLabelledRowsRefusals(t *testing.T) {
	v := testVocab(t)
	rep := func(old, new string) string {
		if !strings.Contains(labelledProfile, old) {
			t.Fatalf("test setup: %q not in labelledProfile", old)
		}
		return strings.Replace(labelledProfile, old, new, 1)
	}
	for _, c := range []struct{ name, text, msg string }{
		{"rows without where", rep("    where:\n      - { path: \".k\", equals: \"Previous Year\" }\n", ""), "rows needs where"},
		{"rows with const", rep(`    json_path: ".v"
  pan:`, `    json_path: ".v"
    const: "2025-26"
  pan:`), "not both"},
		{"bad equals_year", rep(`equals_year: "tax_year"`, `equals_year: "next_year"`), `equals_year is "tax_year" or "assessment_year"`},
		{"two conditions in one", rep(`{ path: ".b", not_equals: "Total" }`, `{ path: ".b", not_equals: "Total", equals: "A" }`), "a condition has one of"},
		{"yes_no on a number", rep("    unit: \"rupees\"\n  - fact: \"computation.employee_count\"", "    unit: \"rupees\"\n    yes_no: true\n  - fact: \"computation.employee_count\""), "yes_no applies only to yes or no facts"},
	} {
		if _, err := Load("profile.yaml", []byte(c.text), v); err == nil || !strings.Contains(err.Error(), c.msg) {
			t.Errorf("%s: %v, want %q", c.name, err, c.msg)
		}
	}
}
