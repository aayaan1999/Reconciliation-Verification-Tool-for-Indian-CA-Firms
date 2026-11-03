package checks_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
)

// SPEC-017 AC-2: sum_equals on totals of rows (rows_where, a keyed total row). Figures invented.

const totalsInputs = `  a: { record_type: "gstr1_outward_line", label: "GSTR-1" }
  b: { record_type: "gstr3b_table_row", label: "GSTR-3B" }`

// totalsByAmounts compares each tax head of the B2B and B2CS rows of a with row 3.1(a) of b.
const totalsByAmounts = `  type: "sum_equals"
  compare_amounts: [ "taxable_value", "igst", "cgst" ]
  parts:
    - { input: "a", sign: "+", rows_where: [ { field: "category", is: "in", values: [ "B2B", "B2CS" ] } ] }
  total: "b"
  total_key: "3.1(a)"
  tolerance: { paise: 100 }`

func outwardLine(key, category, loc string, tv, igst, cgst int64) domain.Record {
	return domain.Record{RecordType: "gstr1_outward_line", Period: "2025-04", MatchKey: key,
		Fields:  map[string]string{"invoice_no": key, "category": category},
		Amounts: map[string]money.Paise{"taxable_value": money.Paise(tv), "igst": money.Paise(igst), "cgst": money.Paise(cgst)},
		Locator: domain.Locator{FileName: "GSTR-1.csv", Kind: domain.LocatorJSONPath, Path: loc}}
}

func summaryRow(table string, tv, igst, cgst int64) domain.Record {
	return domain.Record{RecordType: "gstr3b_table_row", Period: "2025-04", MatchKey: table,
		Fields:  map[string]string{"table": table},
		Amounts: map[string]money.Paise{"taxable_value": money.Paise(tv), "igst": money.Paise(igst), "cgst": money.Paise(cgst)},
		Locator: domain.Locator{FileName: "GSTR-3B.csv", Kind: domain.LocatorJSONPath, Path: "$[0]"}}
}

func evalTotals(t *testing.T, check string, records ...domain.Record) []domain.Evaluation {
	t.Helper()
	r := mustLoad(t, rule(totalsInputs, check))
	ty, _ := period.ParseTaxYear("2025-26")
	evs, err := engine.Evaluate([]*rules.Rule{r}, nil, records, ty, checks.All(), 1)
	if err != nil {
		t.Fatal(err)
	}
	return evs
}

func byKey(evs []domain.Evaluation) map[string]domain.Evaluation {
	m := map[string]domain.Evaluation{}
	for _, e := range evs {
		m[e.Subject.MatchKey] = e
	}
	return m
}

func TestSumEqualsTotalsAddsRows(t *testing.T) {
	evs := evalTotals(t, totalsByAmounts,
		outwardLine("INV-1", "B2B", "$[0]", 10000000, 0, 900000),
		outwardLine("INV-2", "B2B", "$[1]", 5000000, 900000, 0),
		outwardLine("INV-3", "B2CS", "$[2]", 2000000, 0, 180000),
		outwardLine("EXP-1", "EXP", "$[3]", 25000000, 0, 0), // not one of the categories: left out
		summaryRow("3.1(a)", 17000000, 900000, 1080000),
		summaryRow("3.1(b)", 25000000, 0, 0))
	if len(evs) != 3 {
		t.Fatalf("%d evaluations, want one for each of the 3 amounts", len(evs))
	}
	got := byKey(evs)
	for _, k := range []string{"taxable_value", "igst", "cgst"} {
		if got[k].Status != domain.StatusPass || got[k].Difference != 0 {
			t.Errorf("%s: status %v difference %d, want pass 0", k, got[k].Status, got[k].Difference)
		}
	}
	// Three rows, their sum and the total row, each with its place.
	v := got["taxable_value"].Values
	if len(v) != 5 || v[3].Label != "Sum of the figures above" || v[3].Value.Money != 17000000 || v[4].Value.Money != 17000000 {
		t.Fatalf("values %+v", v)
	}
	if v[0].Locator.Path != "$[0]" || v[2].Locator.Path != "$[2]" || v[4].Locator.FileName != "GSTR-3B.csv" {
		t.Errorf("sources %+v", v)
	}
	if !strings.HasPrefix(v[1].Label, "add: ") || strings.HasPrefix(v[0].Label, "add: ") {
		t.Errorf("labels %q, %q", v[0].Label, v[1].Label)
	}
}

func TestSumEqualsTotalsTolerance(t *testing.T) {
	// 100 paise is within the Rs 1 tolerance; 101 paise is one paise over it.
	for _, c := range []struct {
		total int64
		want  domain.EvalStatus
	}{{1080000 + 100, domain.StatusPass}, {1080000 - 100, domain.StatusPass}, {1080000 + 101, domain.StatusFail}, {1080000 - 101, domain.StatusFail}} {
		evs := evalTotals(t, totalsByAmounts,
			outwardLine("INV-1", "B2B", "$[0]", 100, 0, 1080000),
			summaryRow("3.1(a)", 100, 0, c.total))
		e := byKey(evs)["cgst"]
		if e.Status != c.want {
			t.Errorf("total %d: status %v, want %v (difference %d)", c.total, e.Status, c.want, e.Difference)
		}
		if other := byKey(evs)["igst"]; other.Status != domain.StatusPass {
			t.Errorf("an agreeing amount is %v", other.Status)
		}
	}
}

func TestSumEqualsTotalsCannotCheck(t *testing.T) {
	cases := map[string]struct {
		records []domain.Record
		missing string
	}{
		"no row meets the conditions": {[]domain.Record{
			outwardLine("EXP-1", "EXP", "$[0]", 100, 0, 0), summaryRow("3.1(a)", 100, 0, 0)}, "gstr1_outward_line"},
		"no row has the key": {[]domain.Record{
			outwardLine("INV-1", "B2B", "$[0]", 100, 0, 0), summaryRow("3.1(b)", 100, 0, 0)}, "row:gstr3b_table_row|3.1(a)"},
		"the key is on two rows": {[]domain.Record{
			outwardLine("INV-1", "B2B", "$[0]", 100, 0, 0), summaryRow("3.1(a)", 100, 0, 0), summaryRow("3.1(a)", 100, 0, 0)},
			"row:gstr3b_table_row|3.1(a)|repeated"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			evs := evalTotals(t, totalsByAmounts, c.records...)
			if len(evs) != 3 {
				t.Fatalf("%d evaluations", len(evs))
			}
			for _, e := range evs {
				if e.Status != domain.StatusCannotCheck || !slices.Equal(e.MissingFacts, []string{c.missing}) {
					t.Errorf("%s: status %v missing %v, want Cannot check naming %s", e.Subject.MatchKey, e.Status, e.MissingFacts, c.missing)
				}
			}
		})
	}
	t.Run("an amount a row lacks", func(t *testing.T) {
		row := outwardLine("INV-1", "B2B", "$[0]", 100, 0, 0)
		delete(row.Amounts, "cgst")
		got := byKey(evalTotals(t, totalsByAmounts, row, summaryRow("3.1(a)", 100, 0, 0)))
		if e := got["cgst"]; e.Status != domain.StatusCannotCheck || !slices.Equal(e.MissingFacts, []string{"amount:gstr1_outward_line.cgst"}) {
			t.Errorf("cgst: status %v missing %v", e.Status, e.MissingFacts)
		}
		if got["igst"].Status != domain.StatusPass {
			t.Errorf("igst: %v; the other amounts are still compared", got["igst"].Status)
		}
	})
	t.Run("no rows of one input at all", func(t *testing.T) {
		evs := evalTotals(t, totalsByAmounts, summaryRow("3.1(a)", 100, 0, 0))
		if len(evs) != 1 || evs[0].Status != domain.StatusCannotCheck || !slices.Equal(evs[0].MissingFacts, []string{"gstr1_outward_line"}) {
			t.Errorf("%+v", evs)
		}
		evs = evalTotals(t, totalsByAmounts)
		if len(evs) != 1 || !slices.Equal(evs[0].MissingFacts, []string{"gstr1_outward_line", "gstr3b_table_row"}) {
			t.Errorf("no rows at all: %+v", evs)
		}
	})
}

// One amount compared, with a part taken away (a credit note row) and the total's amount named.
func TestSumEqualsTotalsOneAmountAndLess(t *testing.T) {
	check := `  type: "sum_equals"
  parts:
    - { input: "a", field: "taxable_value", sign: "+", rows_where: [ { field: "category", is: "equals", value: "B2B" } ] }
    - { input: "a", field: "taxable_value", sign: "-", rows_where: [ { field: "category", is: "equals", value: "CDN" } ] }
  total: "b"
  total_key: "3.1(a)"
  total_field: "taxable_value"`
	evs := evalTotals(t, check,
		outwardLine("INV-1", "B2B", "$[0]", 10000, 0, 0),
		outwardLine("CN-1", "CDN", "$[1]", 2500, 0, 0),
		summaryRow("3.1(a)", 7500, 0, 0))
	if len(evs) != 1 || evs[0].Status != domain.StatusPass || evs[0].Subject.MatchKey != "taxable_value" {
		t.Fatalf("%+v", evs)
	}
	if v := evs[0].Values; len(v) != 4 || !strings.HasPrefix(v[1].Label, "less: ") {
		t.Errorf("values %+v", v)
	}
}

// The same rows in any order give the same evaluations, byte for byte.
func TestSumEqualsTotalsDeterministicOrder(t *testing.T) {
	rows := []domain.Record{
		outwardLine("INV-10", "B2B", "$[10]", 100, 7, 3),
		outwardLine("INV-2", "B2B", "$[2]", 200, 5, 1),
		outwardLine("INV-2", "B2B", "$[9]", 300, 1, 1), // a second line of INV-2
		outwardLine("INV-3", "B2CS", "$[3]", 400, 2, 2),
		summaryRow("3.1(a)", 1000, 15, 7),
	}
	want := evalTotals(t, totalsByAmounts, rows...)
	for _, order := range [][]int{{4, 3, 2, 1, 0}, {2, 0, 4, 1, 3}, {1, 2, 3, 4, 0}} {
		var shuffled []domain.Record
		for _, i := range order {
			shuffled = append(shuffled, rows[i])
		}
		if got := evalTotals(t, totalsByAmounts, shuffled...); !reflect.DeepEqual(got, want) {
			t.Errorf("order %v gives different evaluations:\n%+v\n%+v", order, got, want)
		}
	}
	// Rows are added in key order, then by place with row numbers read as numbers.
	var places []string
	for _, v := range byKey(want)["taxable_value"].Values[:4] {
		places = append(places, v.Locator.Path)
	}
	if !slices.Equal(places, []string{"$[10]", "$[2]", "$[9]", "$[3]"}) {
		t.Errorf("rows added in the order %v", places)
	}
}

func TestSumEqualsTotalsRefused(t *testing.T) {
	for name, c := range map[string]struct{ inputs, check, want string }{
		"no conditions": {totalsInputs, `  type: "sum_equals"
  compare_amounts: [ "igst" ]
  parts:
    - { input: "a", sign: "+" }
  total: "b"
  total_key: "3.1(a)"`, "rows_where must name the rows"},
		"a column that is not there": {totalsInputs, `  type: "sum_equals"
  compare_amounts: [ "igst" ]
  parts:
    - { input: "a", sign: "+", rows_where: [ { field: "nope", is: "equals", value: "x" } ] }
  total: "b"
  total_key: "3.1(a)"`, `"nope" is not a column of gstr1_outward_line`},
		"a condition on a figure": {totalsInputs, `  type: "sum_equals"
  compare_amounts: [ "igst" ]
  parts:
    - { input: "a", sign: "+", rows_where: [ { field: "category", is: "zero" } ] }
  total: "b"
  total_key: "3.1(a)"`, `"zero" cannot select rows`},
		"an amount the rows do not have": {totalsInputs, `  type: "sum_equals"
  compare_amounts: [ "invoice_value" ]
  parts:
    - { input: "a", sign: "+", rows_where: [ { field: "category", is: "equals", value: "B2B" } ] }
  total: "b"
  total_key: "3.1(a)"`, `"invoice_value", which is not an amount of gstr3b_table_row`},
		"a field with compare_amounts": {totalsInputs, `  type: "sum_equals"
  compare_amounts: [ "igst" ]
  parts:
    - { input: "a", field: "igst", sign: "+", rows_where: [ { field: "category", is: "equals", value: "B2B" } ] }
  total: "b"
  total_key: "3.1(a)"`, "with compare_amounts a part has no field"},
		"total_field with compare_amounts": {totalsInputs, `  type: "sum_equals"
  compare_amounts: [ "igst" ]
  parts:
    - { input: "a", sign: "+", rows_where: [ { field: "category", is: "equals", value: "B2B" } ] }
  total: "b"
  total_key: "3.1(a)"
  total_field: "igst"`, "total_field is not used"},
		"the total added up as well": {totalsInputs, `  type: "sum_equals"
  compare_amounts: [ "igst" ]
  parts:
    - { input: "a", sign: "+", rows_where: [ { field: "category", is: "equals", value: "B2B" } ] }
    - { input: "b", sign: "+", rows_where: [ { field: "table", is: "equals", value: "3.1(a)" } ] }
  total: "b"
  total_key: "3.1(a)"`, "cannot also be added up"},
		"no field and no compare_amounts": {totalsInputs, `  type: "sum_equals"
  parts:
    - { input: "a", sign: "+", rows_where: [ { field: "category", is: "equals", value: "B2B" } ] }
  total: "b"
  total_key: "3.1(a)"`, `total_field ""`},
		"a fact input": {`  a: { fact: "computation.business_income", label: "A" }
  b: { record_type: "gstr3b_table_row", label: "GSTR-3B" }`, `  type: "sum_equals"
  compare_amounts: [ "igst" ]
  parts:
    - { input: "a", sign: "+", rows_where: [ { field: "category", is: "equals", value: "B2B" } ] }
  total: "b"
  total_key: "3.1(a)"`, "must be a record type"},
		"total_key on another check type": {totalsInputs, `  type: "equal"
  total_key: "3.1(a)"`, "does not use total_key"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := load(t, rule(c.inputs, c.check))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("err = %v, want one saying %q", err, c.want)
			}
		})
	}
}
