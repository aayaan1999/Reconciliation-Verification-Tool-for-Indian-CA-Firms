package checks_test

import (
	"slices"
	"strings"
	"testing"
)

// SPEC-018 AC-3: report_missing and report_fields of match_records are read from a rule file,
// "not written" and "an empty list" are different, and a wrong value is refused naming it.

const reportInputs = `  a: { record_type: "ewaybill_row", label: "E-way bill register" }
  b: { record_type: "sales_b2b_invoice", label: "Sales register" }`

func reportCheck(extra string) string {
	return `  type: "match_records"
  compare_amounts: [ "invoice_value" ]
` + extra
}

func TestReportMissingAndFieldsAreRead(t *testing.T) {
	r := mustLoad(t, rule(reportInputs, reportCheck("")))
	if r.Check.ReportMissingSet || len(r.Check.ReportFields) != 0 {
		t.Errorf("not written: set %v, fields %v", r.Check.ReportMissingSet, r.Check.ReportFields)
	}
	r = mustLoad(t, rule(reportInputs, reportCheck("  report_missing: []\n")))
	if !r.Check.ReportMissingSet || len(r.Check.ReportMissing) != 0 {
		t.Errorf("an empty list: set %v, sides %v", r.Check.ReportMissingSet, r.Check.ReportMissing)
	}
	r = mustLoad(t, rule(reportInputs, reportCheck(`  report_missing: [ "a" ]
  report_fields: [ { input: "a", field: "ewb_status", is: "in", values: [ "Cancelled", "CNL" ] } ]
`)))
	if !r.Check.ReportMissingSet || !slices.Equal(r.Check.ReportMissing, []string{"a"}) {
		t.Errorf("report_missing: set %v, sides %v", r.Check.ReportMissingSet, r.Check.ReportMissing)
	}
	if len(r.Check.ReportFields) != 1 || r.Check.ReportFields[0].Field != "ewb_status" || !slices.Equal(r.Check.ReportFields[0].Values, []string{"Cancelled", "CNL"}) {
		t.Errorf("report_fields: %+v", r.Check.ReportFields)
	}
}

func TestReportingParamsRefused(t *testing.T) {
	for _, c := range []struct{ name, extra, want string }{
		{"a side that is not a or b", "  report_missing: [ \"c\" ]\n", `"c" must be a or b`},
		{"a side twice", "  report_missing: [ \"a\", \"a\" ]\n", `"a" is listed twice`},
		{"a column the record type lacks", "  report_fields: [ { input: \"a\", field: \"irn_status\", is: \"equals\", value: \"CNL\" } ]\n", `"irn_status" is not a column of ewaybill_row`},
		{"an input that is not a or b", "  report_fields: [ { input: \"c\", field: \"ewb_status\", is: \"equals\", value: \"CNL\" } ]\n", `input "c" must be a or b`},
		{"a condition on a number", "  report_fields: [ { input: \"a\", field: \"ewb_status\", is: \"zero\" } ]\n", `"zero" cannot test a column`},
		{"a list with no values", "  report_fields: [ { input: \"a\", field: \"ewb_status\", is: \"in\" } ]\n", "needs a list of values"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := load(t, rule(reportInputs, reportCheck(c.extra)))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("got %v, want an error with %q", err, c.want)
			}
		})
	}
	// Another check type refuses them.
	_, err := load(t, rule(`  a: { fact: "computation.business_income", label: "A" }
  b: { fact: "computation.total_income", label: "B" }`, `  type: "equal"
  report_missing: [ "a" ]`))
	if err == nil || !strings.Contains(err.Error(), "report_missing") {
		t.Errorf("equal with report_missing: %v", err)
	}
}
