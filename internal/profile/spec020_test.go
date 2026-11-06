package profile

import (
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
)

// SPEC-020: a file that holds no tax year (the TDS files) takes the run's tax year with
// context.tax_year from_run, and is refused when the run gave none. Illustrative values only.

const fromRunProfile = `schema: "recon.profile/1"
profile_id: "test.portal.undated"
profile_version: "1"
source_system: "Test portal"
document_type: "gst_return"
file_type: "json"
regime: "CGST2017"
detect:
  - json_exists: { path: "$.rows" }
context:
  tax_year: { from_run: true }
  period: { const: "FY" }
records:
  - record_type: "gst_invoice"
    rows: "$.rows[*]"
    unit: "rupees"
    fields:
      - { name: "counterparty_gstin", const: "27AAAAA0000A1Z5", required: true }
      - { name: "invoice_no", path: ".no", required: true }
    amounts:
      - { name: "taxable_value", path: ".tx" }
`

const undatedDoc = `{"rows": [{"no": "T1", "tx": 10.5}]}`

func TestTaxYearFromRun(t *testing.T) {
	ty := period.TaxYear{StartYear: 2026}
	r := extract(t, load(t, fromRunProfile), undatedDoc, Source{FileName: "undated.json", TaxYear: ty})
	if r.Report.HasErrors() {
		t.Fatalf("errors: %v", r.Report.Errors)
	}
	if r.Context.TaxYear != ty || r.Context.Period != "FY2026-27" {
		t.Errorf("context %+v, want tax year 2026-27 and period FY2026-27", r.Context)
	}
	// No tax year given by the run: the file is refused, never read under a guessed year.
	r = extract(t, load(t, fromRunProfile), undatedDoc, Source{FileName: "undated.json"})
	if !r.Report.HasErrors() || len(r.Records) != 0 {
		t.Fatalf("a file with no tax year was read: %d records, errors %v", len(r.Records), r.Report.Errors)
	}
	if !strings.Contains(r.Report.Errors[0].Msg, "tax year") {
		t.Errorf("error %q does not name the tax year", r.Report.Errors[0].Msg)
	}
	// from_run takes no path, constant or from_period.
	bad := strings.Replace(fromRunProfile, "{ from_run: true }", `{ from_run: true, const: "2026-27" }`, 1)
	if _, err := Load("bad.yaml", []byte(bad), testVocab(t)); err == nil {
		t.Error("from_run with a const was accepted")
	}
}
