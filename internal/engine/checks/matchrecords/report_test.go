package matchrecords

import (
	"reflect"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
)

// SPEC-018 AC-3: report_missing says which side's rows with no partner are reported, and
// report_fields lists the values of a column that are reported, shown as written.

func reportRule(missing []string, set bool, fields ...rules.Condition) *rules.Rule {
	r := testRule()
	r.Check.ReportMissing, r.Check.ReportMissingSet = missing, set
	r.Check.ReportFields = fields
	return r
}

func reportCodes(t *testing.T, r *rules.Rule, a, b []domain.Record) map[string]string {
	t.Helper()
	out, err := Check{}.EvaluateRecords(r, a, b)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, o := range out {
		got[o.MatchKey] = o.Code
	}
	return got
}

func TestReportMissingEachSide(t *testing.T) {
	a := []domain.Record{
		sales("BOTH", g1, "01-08-2026", 1000, 180, 1180),
		sales("ONLY-A", g1, "02-08-2026", 500, 90, 590),
	}
	b := []domain.Record{
		gstr1("BOTH", g1, "01-08-2026", 1000, 180, 1180),
		gstr1("ONLY-B", g2, "03-08-2026", 700, 126, 826),
	}
	for _, c := range []struct {
		name    string
		missing []string
		set     bool
		want    map[string]string
	}{
		{"not written: both sides, as before", nil, false, map[string]string{"BOTH": Matched, "ONLY-A": OnlyInA, "ONLY-B": OnlyInB}},
		{"a only", []string{"a"}, true, map[string]string{"BOTH": Matched, "ONLY-A": OnlyInA}},
		{"b only", []string{"b"}, true, map[string]string{"BOTH": Matched, "ONLY-B": OnlyInB}},
		{"both written", []string{"a", "b"}, true, map[string]string{"BOTH": Matched, "ONLY-A": OnlyInA, "ONLY-B": OnlyInB}},
		{"neither (an empty list)", []string{}, true, map[string]string{"BOTH": Matched}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := reportCodes(t, reportRule(c.missing, c.set), a, b); !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

// A one-sided rule still finds an invoice number written differently: the pairing is made
// before the unreported side is left out.
func TestReportMissingKeepsPairing(t *testing.T) {
	a := []domain.Record{sales("KCD/0412", g1, "05-08-2026", 1000, 180, 1180)}
	b := []domain.Record{gstr1("KCD-412", g1, "05-08-2026", 1000, 180, 1180)}
	got := reportCodes(t, reportRule([]string{"a"}, true), a, b)
	if want := map[string]string{"KCD/0412": KeyDiff}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func withField(r domain.Record, name, value string) domain.Record {
	r.Fields[name] = value
	return r
}

func TestReportFieldsListsValuesAsWritten(t *testing.T) {
	cond := rules.Condition{Input: "b", Field: "irn_status", Is: rules.IsIn, Values: []string{"CNL", "Cancelled"}}
	a := []domain.Record{
		sales("ACTIVE", g1, "01-08-2026", 1000, 180, 1180),
		sales("CANCELLED", g1, "02-08-2026", 500, 90, 590),
		sales("DIFFERS", g1, "03-08-2026", 700, 126, 826),
	}
	b := []domain.Record{
		withField(gstr1("ACTIVE", g1, "01-08-2026", 1000, 180, 1180), "irn_status", "ACT"),
		withField(gstr1("CANCELLED", g1, "02-08-2026", 500, 90, 590), "irn_status", "cnl"),
		withField(gstr1("DIFFERS", g1, "03-08-2026", 800, 126, 926), "irn_status", "CNL"),
	}
	out, err := Check{}.EvaluateRecords(reportRule(nil, false, cond), a, b)
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]rules.RecordOutcome{}
	for _, o := range out {
		byKey[o.MatchKey] = o
	}
	if o := byKey["ACTIVE"]; o.Status != domain.StatusPass || o.Code != Matched {
		t.Errorf("an ACT row is not listed: %v %s", o.Status, o.Code)
	}
	for _, key := range []string{"CANCELLED", "DIFFERS"} {
		o := byKey[key]
		if o.Status != domain.StatusFail {
			t.Errorf("%s: %v %s, want a finding", key, o.Status, o.Code)
		}
		last := o.Values[len(o.Values)-1]
		if last.Input != "b.irn_status" || last.Locator.FileName != "GSTR1.json" || !strings.Contains(last.Label, "irn status") {
			t.Errorf("%s: last value %+v is not the status with its source", key, last)
		}
	}
	// As written: the file's own text, not the listed value.
	if v := byKey["CANCELLED"].Values; v[len(v)-1].Value.Text != "cnl" {
		t.Errorf("status shown as %q, want the file's text %q", v[len(v)-1].Value.Text, "cnl")
	}
	if o := byKey["CANCELLED"]; o.Code != FieldReported {
		t.Errorf("code %s, want %s", o.Code, FieldReported)
	}
	if o := byKey["DIFFERS"]; o.Code != AmountDiff {
		t.Errorf("an amount difference keeps its code, got %s", o.Code)
	}
}

// A row with no partner shows the listed column of the side that has it.
func TestReportFieldsOnOneSidedRow(t *testing.T) {
	cond := rules.Condition{Input: "b", Field: "ims_action", Is: rules.IsIn, Values: []string{"Rejected", "Pending"}}
	b := []domain.Record{withField(gstr1("ONLY-B", g2, "03-08-2026", 700, 126, 826), "ims_action", "Pending")}
	out, err := Check{}.EvaluateRecords(reportRule(nil, false, cond), nil, b)
	if err != nil || len(out) != 1 {
		t.Fatalf("%v %v", out, err)
	}
	last := out[0].Values[len(out[0].Values)-1]
	if out[0].Code != OnlyInB || last.Input != "b.ims_action" || last.Value.Text != "Pending" {
		t.Errorf("code %s, last value %+v", out[0].Code, last)
	}
}

func TestReportingParamsValidated(t *testing.T) {
	// ValidateParams needs the vocabulary; the rules package tests load rules through it. Here
	// only the pure helpers are checked.
	r := reportRule([]string{"a"}, true)
	if !reportsSide(r, "a") || reportsSide(r, "b") {
		t.Error("report_missing: [a] should report a only")
	}
	if !reportsSide(reportRule(nil, false), "b") {
		t.Error("report_missing not written should report both")
	}
}
