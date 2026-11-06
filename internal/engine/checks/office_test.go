package checks_test

import (
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// SPEC-022 risk 1: the three row-reading check types are generic over column names. Two office
// documents whose columns no built-in uses (ref_no, customer, net_amount, tax_amount,
// gross_amount) are read by match_records, sum_equals and present_together through the merged
// vocabulary, with no change to any check type. Figures and names invented.

func officeVocabulary(t *testing.T) *vocab.Vocab {
	t.Helper()
	cols := []vocab.OfficeColumn{
		{Name: "ref_no", Label: "Reference", Type: vocab.ColumnText, Key: true, Required: true},
		{Name: "customer", Label: "Customer", Type: vocab.ColumnText},
		{Name: "net_amount", Label: "Net amount", Type: vocab.ColumnAmount},
		{Name: "tax_amount", Label: "Tax amount", Type: vocab.ColumnAmount},
		{Name: "gross_amount", Label: "Gross amount", Type: vocab.ColumnAmount},
	}
	doc := func(code, rt string) vocab.OfficeDocument {
		return vocab.OfficeDocument{Code: code, Name: code, Category: "Books", Description: "A document added for the test",
			Formats: []string{"csv"}, RecordType: rt, Active: true}
	}
	m, err := vocabulary(t).WithOffice(vocab.Office{
		Documents: []vocab.OfficeDocument{doc("sales_return", "sales_return_row"), doc("return_ledger", "return_ledger_row")},
		RecordTypes: []vocab.OfficeRecordType{
			{Name: "sales_return_row", Description: "Rows of the sales return register", Columns: cols},
			{Name: "return_ledger_row", Description: "Rows of the returns ledger", Columns: cols},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func officeRule(t *testing.T, inputs, check string) *rules.Rule {
	t.Helper()
	text := strings.Replace(rule(inputs, check), `tax_years: [ "2025-26" ], valid_from: "2025-04-01"`, `tax_years: [ "2026-27" ], valid_from: "2026-04-01"`, 1)
	text = strings.Replace(text, `review_document: "computation"`, `review_document: "sales_return"`, 1) // an office document is a valid review document
	text = strings.Replace(text, `module: "tieout"`, `module: "office"`, 1)                              // a rule over office rows lives in the office module
	r, err := rules.Load("rule.yaml", []byte(text), officeVocabulary(t), checks.All())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func ret(recordType, ref, customer string, net, tax, gross int64) domain.Record {
	p := func(r int64) money.Paise { return money.Paise(r * 100) }
	return domain.Record{RecordType: recordType, Period: "2026-08", MatchKey: ref,
		Fields:  map[string]string{"ref_no": ref, "customer": customer},
		Amounts: map[string]money.Paise{"net_amount": p(net), "tax_amount": p(tax), "gross_amount": p(gross)},
		Locator: domain.Locator{FileName: recordType + ".csv", Kind: domain.LocatorJSONPath, Path: "row." + ref}}
}

func evaluate(t *testing.T, r *rules.Rule, recs ...domain.Record) map[string]domain.Evaluation {
	t.Helper()
	ty, _ := period.ParseTaxYear("2026-27")
	evs, err := engine.Evaluate([]*rules.Rule{r}, nil, recs, ty, checks.All(), 1)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]domain.Evaluation{}
	for _, e := range evs {
		got[e.Subject.MatchKey] = e
	}
	return got
}

func TestMatchRecordsOverOfficeDocuments(t *testing.T) {
	r := officeRule(t, `  a: { record_type: "sales_return_row", label: "Sales returns" }
  b: { record_type: "return_ledger_row", label: "Returns ledger" }`, `  type: "match_records"
  compare_amounts: [ "net_amount", "tax_amount" ]
  compare_fields: [ "customer" ]
  duplicates_in: [ "b" ]
  tolerance: { paise: 100 }`)
	a, b := "sales_return_row", "return_ledger_row"
	got := evaluate(t, r,
		ret(a, "SR-1", "Orion Retail", 100000, 18000, 118000), ret(b, "SR-1", "Orion Retail", 100000, 18000, 118000), // the same in both
		ret(a, "SR-2", "Orion Retail", 50000, 9000, 59000), ret(b, "SR-2", "Orion Retail", 50000, 8000, 58000), // tax differs
		ret(a, "SR-3", "Vega Traders", 20000, 3600, 23600),                                                     // only in the returns register
		ret(a, "SR-4", "Orion Retail", 10000, 1800, 11800), ret(b, "SR-4", "Vega Traders", 10000, 1800, 11800), // customer differs
		ret(a, "SR-5", "Orion Retail", 10000, 1800, 11800), ret(b, "SR-5", "Orion Retail", 10000, 1800, 11800), ret(b, "SR-5", "Orion Retail", 10000, 1800, 11800), // twice in the ledger
	)
	if e := got["SR-1"]; e.Status != domain.StatusPass {
		t.Errorf("SR-1: status %v, values %v", e.Status, e.Values)
	}
	e := got["SR-2"]
	if e.Status != domain.StatusFail || e.Difference != 100000 {
		t.Errorf("SR-2: status %v, difference %d (want a failure of 1,000.00 in tax)", e.Status, e.Difference)
	}
	// a finding shows both values and both source locators, in the column's own name
	if len(e.Values) < 2 || e.Values[0].Locator.FileName == e.Values[1].Locator.FileName {
		t.Errorf("SR-2: the finding does not show both sources: %v", e.Values)
	}
	var text strings.Builder
	for _, v := range e.Values {
		text.WriteString(v.Label + "|")
	}
	if !strings.Contains(text.String(), "tax amount") {
		t.Errorf("SR-2: labels %q do not name the column", text.String())
	}
	if e := got["SR-3"]; e.Status != domain.StatusFail {
		t.Errorf("SR-3 (only in one document): status %v", e.Status)
	}
	if e := got["SR-4"]; e.Status != domain.StatusFail {
		t.Errorf("SR-4 (customer differs): status %v", e.Status)
	}
	if e := got["SR-5"]; e.Status != domain.StatusFail {
		t.Errorf("SR-5 (twice in the ledger): status %v", e.Status)
	}
}

func TestSumEqualsOverOfficeRows(t *testing.T) {
	r := officeRule(t, `  a: { record_type: "sales_return_row", label: "Sales returns" }`, `  type: "sum_equals"
  parts:
    - { input: "a", field: "net_amount", sign: "+" }
    - { input: "a", field: "tax_amount", sign: "+" }
  total: "a"
  total_field: "gross_amount"`)
	a := "sales_return_row"
	partial := ret(a, "SR-3", "Vega Traders", 100, 18, 118)
	delete(partial.Amounts, "tax_amount")
	got := evaluate(t, r, ret(a, "SR-1", "Orion Retail", 100000, 18000, 118000), ret(a, "SR-2", "Orion Retail", 100000, 18000, 117000), partial)
	if e := got["SR-1"]; e.Status != domain.StatusPass {
		t.Errorf("SR-1: status %v", e.Status)
	}
	if e := got["SR-2"]; e.Status != domain.StatusFail || e.Difference != 100000 {
		t.Errorf("SR-2: status %v difference %d", e.Status, e.Difference)
	}
	if e := got["SR-3"]; e.Status != domain.StatusCannotCheck || strings.Join(e.MissingFacts, ",") != "tax_amount" {
		t.Errorf("SR-3 (a figure not in the row): status %v missing %v, want cannot check naming tax_amount", e.Status, e.MissingFacts)
	}
}

func TestPresentTogetherOverOfficeRows(t *testing.T) {
	r := officeRule(t, `  a: { record_type: "sales_return_row", label: "Sales returns" }`, `  type: "present_together"
  when:
    - { input: "a", field: "tax_amount", is: "zero" }
    - { input: "a", field: "customer", is: "equals", value: "Orion Retail" }`)
	a := "sales_return_row"
	got := evaluate(t, r, ret(a, "SR-1", "Orion Retail", 100000, 0, 100000), ret(a, "SR-2", "Orion Retail", 100000, 18000, 118000), ret(a, "SR-3", "Vega Traders", 100000, 0, 100000))
	if e := got["SR-1"]; e.Status != domain.StatusFail {
		t.Errorf("SR-1 (both conditions hold): status %v", e.Status)
	}
	for _, key := range []string{"SR-2", "SR-3"} {
		if e := got[key]; e.Status != domain.StatusPass {
			t.Errorf("%s (one condition holds): status %v", key, e.Status)
		}
	}
}

// The merged vocabulary is a superset: a rule over built-in record types loads and runs the same
// against it and against the base vocabulary.
func TestBuiltinRuleSameAgainstMergedVocabulary(t *testing.T) {
	inputs := `  block: { record_type: "tar_depreciation_block", label: "Block" }`
	check := `  type: "sum_equals"
  parts:
    - { input: "block", field: "opening_wdv", sign: "+" }
    - { input: "block", field: "additions", sign: "+" }
  total: "block"
  total_field: "closing_wdv"`
	text := rule(inputs, check)
	base := mustLoad(t, text)
	merged, err := rules.Load("rule.yaml", []byte(text), officeVocabulary(t), checks.All())
	if err != nil {
		t.Fatalf("a built-in rule does not load against the merged vocabulary: %v", err)
	}
	ty, _ := period.ParseTaxYear("2025-26")
	recs := []domain.Record{block("Computers", 100, 50, 0, 0, 150), block("Plant", 100, 50, 0, 0, 140)}
	x, errX := engine.Evaluate([]*rules.Rule{base}, nil, recs, ty, checks.All(), 1)
	y, errY := engine.Evaluate([]*rules.Rule{merged}, nil, recs, ty, checks.All(), 1)
	if errX != nil || errY != nil || len(x) != len(y) {
		t.Fatalf("evaluations %d and %d, errors %v and %v", len(x), len(y), errX, errY)
	}
	for i := range x {
		if x[i].Status != y[i].Status || x[i].Difference != y[i].Difference || x[i].Subject.MatchKey != y[i].Subject.MatchKey {
			t.Errorf("%s differs between the base and the merged vocabulary: %v / %v", x[i].Subject.MatchKey, x[i].Status, y[i].Status)
		}
	}
}

// AC-18 and AC-24a at the loader: a rule over an office document's rows is refused outside the office
// module, and under a check that reads single figures; each message names the input's record_type.
func TestOfficeRulesRefusals(t *testing.T) {
	v := officeVocabulary(t)
	load := func(module, inputs, check string) error {
		text := strings.Replace(rule(inputs, check), `tax_years: [ "2025-26" ], valid_from: "2025-04-01"`, `tax_years: [ "2026-27" ], valid_from: "2026-04-01"`, 1)
		text = strings.Replace(text, `review_document: "computation"`, `review_document: "sales_return"`, 1)
		text = strings.Replace(text, `module: "tieout"`, `module: "`+module+`"`, 1)
		_, err := rules.Load("rule.yaml", []byte(text), v, checks.All())
		return err
	}
	rows := `  a: { record_type: "sales_return_row", label: "Sales returns" }`
	sum := `  type: "sum_equals"
  parts:
    - { input: "a", field: "net_amount", sign: "+" }
    - { input: "a", field: "tax_amount", sign: "+" }
  total: "a"
  total_field: "gross_amount"`

	if err := load("office", rows, sum); err != nil {
		t.Fatalf("an office rule over office rows: %v", err)
	}
	if err := load("tieout", rows, sum); err == nil || !strings.Contains(err.Error(), "must be in the office module") {
		t.Errorf("office rows in the tieout module: %v", err)
	}
	// every module but office is refused; the seven built-in modules are unchanged
	for _, m := range []string{"bank", "drafts", "gst", "newheads", "payroll", "tds", "tieout"} {
		if err := load(m, rows, sum); err == nil {
			t.Errorf("office rows in the %s module were accepted", m)
		}
	}
	// a built-in rule is refused in the office module only if it reads office rows: it does not
	builtin := `  block: { record_type: "tar_depreciation_block", label: "Block" }`
	builtinCheck := `  type: "sum_equals"
  parts:
    - { input: "block", field: "opening_wdv", sign: "+" }
    - { input: "block", field: "additions", sign: "+" }
  total: "block"
  total_field: "closing_wdv"`
	if err := load("tieout", builtin, builtinCheck); err != nil {
		t.Errorf("a built-in rule over built-in rows: %v", err)
	}
	if err := load("office", builtin, builtinCheck); err != nil {
		t.Errorf("an office rule may read built-in record types too: %v", err)
	}
	// a check type that reads single figures cannot take an office record type, whichever input it is
	fact := `computation.allowed_depreciation`
	for name, tc := range map[string]struct{ inputs, check string }{
		"equal": {`  a: { record_type: "sales_return_row", label: "Returns" }
  b: { fact: "` + fact + `", label: "A figure" }`, `  type: "equal"
  tolerance: { paise: 100 }`},
		"gte": {`  a: { record_type: "sales_return_row", label: "Returns" }
  b: { fact: "` + fact + `", label: "A figure" }`, `  type: "gte"`},
		"percent_of": {`  base: { record_type: "sales_return_row", label: "Returns" }
  value: { fact: "` + fact + `", label: "A figure" }`, `  type: "percent_of"
  base: "base"
  rate: "15.00"
  value: "value"`},
		"table_lookup": {`  country: { fact: "ftc.country", label: "Country" }
  rate: { record_type: "sales_return_row", label: "Returns" }`, `  type: "table_lookup"
  keys: ["country"]
  value: "rate"
  table:
    - { keys: ["PORTUGAL"], value: "10.00", source: "a table" }`},
	} {
		err := load("office", tc.inputs, tc.check)
		if err == nil || !strings.Contains(err.Error(), `check type "`+name+`" reads single figures`) || !strings.Contains(err.Error(), "match_records, present_together, sum_equals") {
			t.Errorf("%s over office rows: %v", name, err)
		}
	}
}

// refused with the existing message about an unknown record type.
func TestOfficeRuleOverADraftRecordType(t *testing.T) {
	_, err := rules.Load("rule.yaml", []byte(strings.Replace(rule(`  a: { record_type: "sales_return_row", label: "Sales returns" }`, `  type: "present_together"
  when:
    - { input: "a", field: "tax_amount", is: "zero" }`), `module: "tieout"`, `module: "office"`, 1)), vocabulary(t), checks.All())
	if err == nil || !strings.Contains(err.Error(), `record type "sales_return_row" is not in the vocabulary`) {
		t.Errorf("a record type that is not ready: %v", err)
	}
}
