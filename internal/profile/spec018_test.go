package profile

import (
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
)

// SPEC-018: a record block that reads one object as one row (one_row), a period taken from
// a date (format "yyyy-mm-dd"), and the chain of versions (3 supersedes 2, which supersedes 1).
// All values are illustrative.

const oneRowProfile = `schema: "recon.profile/1"
profile_id: "test.portal.summary"
profile_version: "1"
source_system: "Test portal"
document_type: "gst_return"
file_type: "json"
regime: "CGST2017"
detect:
  - json_exists: { path: "$.tables" }
context:
  tax_year: { const: "2026-27" }
  period: { json_path: "$.first_date", format: "yyyy-mm-dd" }
records:
  - record_type: "gst_invoice"
    rows: "$.tables.one"
    one_row: true
    unit: "rupees"
    fields:
      - { name: "counterparty_gstin", const: "27AAAAA0000A1Z5", required: true }
      - { name: "invoice_no", const: "T1", required: true }
    amounts:
      - { name: "taxable_value", path: ".tx" }
`

const oneRowDoc = `{"first_date": "2026-09-03", "tables": {"one": {"tx": 1234.5}}}`

func TestOneRowAndPeriodFromDate(t *testing.T) {
	r := extract(t, load(t, oneRowProfile), oneRowDoc, Source{FileName: "summary.json"})
	if r.Report.HasErrors() {
		t.Fatalf("errors: %v", r.Report.Errors)
	}
	if len(r.Records) != 1 || r.Records[0].Amounts["taxable_value"] != money.Paise(123450) || r.Records[0].Fields["invoice_no"] != "T1" {
		t.Errorf("records %+v, want one row of 1,234.50", r.Records)
	}
	if r.Context.Period != "2026-09" {
		t.Errorf("period %q, want the month of the date, 2026-09", r.Context.Period)
	}
	// An object that is not there gives no row, not an error.
	r = extract(t, load(t, oneRowProfile), `{"first_date": "2026-09-03", "tables": {}}`, Source{FileName: "summary.json"})
	if r.Report.HasErrors() || len(r.Records) != 0 {
		t.Errorf("an absent object: %d records, errors %v", len(r.Records), r.Report.Errors)
	}
	// A date that is not a date is refused, naming it.
	r = extract(t, load(t, oneRowProfile), `{"first_date": "2026-13-03", "tables": {"one": {"tx": 1}}}`, Source{FileName: "summary.json"})
	if !r.Report.HasErrors() {
		t.Errorf("a period from 2026-13-03 was accepted")
	}

	v := testVocab(t)
	rep := func(old, new string) string {
		if !strings.Contains(oneRowProfile, old) {
			t.Fatalf("test setup: %q not in oneRowProfile", old)
		}
		return strings.Replace(oneRowProfile, old, new, 1)
	}
	for _, c := range []struct{ name, text, msg string }{
		{"one_row with a wildcard", rep(`rows: "$.tables.one"`, `rows: "$.tables[*]"`), "must not have [*]"},
		{"a block without one_row and without a wildcard", rep("    one_row: true\n", ""), "must select every row"},
	} {
		if _, err := Load("profile.yaml", []byte(c.text), v); err == nil || !strings.Contains(err.Error(), c.msg) {
			t.Errorf("%s: %v, want %q", c.name, err, c.msg)
		}
	}
}
