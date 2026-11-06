package checks_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks/percentof"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// The check types of SPEC-006 and SPEC-007, on cases the rules' fixtures do not cover.
// Figures invented.

func vocabulary(t *testing.T) *vocab.Vocab {
	t.Helper()
	v, err := vocab.LoadDir(filepath.Join("..", "..", "..", "content", "vocabulary"))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// rule builds a rule file around its inputs and check block.
func rule(inputs, check string) string {
	return `schema: "recon.rule/1"
id: "TEST-CHK-001"
version: "1"
source: "builtin"
module: "tieout"
category: "R1"
title: "Test rule"
rule_text_en: "A test."
notes: ""
law:
  - { act: "Income-tax Act, 1961", section: "1", reference: "" }
applies: { tax_years: [ "2025-26" ], valid_from: "2025-04-01", valid_to: "" }
inputs:
` + inputs + `
check:
` + check + `
severity: "low"
review_document: "computation"
`
}

func load(t *testing.T, text string) (*rules.Rule, error) {
	t.Helper()
	return rules.Load("rule.yaml", []byte(text), vocabulary(t), checks.All())
}

func mustLoad(t *testing.T, text string) *rules.Rule {
	t.Helper()
	r, err := load(t, text)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func money2(name string, rupees int64) domain.Fact {
	return domain.Fact{Name: name, Period: "FY2025-26", Value: domain.MoneyValue(money.Paise(rupees * 100)),
		Locator: domain.Locator{FileName: "f.json", Kind: domain.LocatorJSONPath, Path: "$." + name}}
}

func text(name, s string) domain.Fact {
	return domain.Fact{Name: name, Period: "FY2025-26", Value: domain.TextValue(s),
		Locator: domain.Locator{FileName: "f.json", Kind: domain.LocatorJSONPath, Path: "$." + name}}
}

func run(t *testing.T, r *rules.Rule, facts ...domain.Fact) domain.Evaluation {
	t.Helper()
	ty, _ := period.ParseTaxYear("2025-26")
	evs, err := engine.Evaluate([]*rules.Rule{r}, facts, nil, ty, checks.All(), 1)
	if err != nil || len(evs) != 1 {
		t.Fatalf("evaluations %v, err %v", evs, err)
	}
	return evs[0]
}

func TestPercentRounding(t *testing.T) {
	for _, c := range []struct {
		base money.Paise
		bp   int64
		want money.Paise
	}{
		{3809299000, 1500, 571394900}, // 5713948.50 rounds half up to 5713949
		{3712460000, 1500, 556869000}, // exact
		{-1000, 1500, -200},           // -1.50 rounds away from zero to -2
		{149, 1000, 0},                // 0.149 rupees to the rupee
	} {
		got, err := percentof.Percent(c.base, c.bp)
		if err != nil || got != c.want {
			t.Errorf("Percent(%d, %d) = %d, %v; want %d", c.base, c.bp, got, err, c.want)
		}
	}
}

func TestSumEqualsShowsPartsSumAndTotal(t *testing.T) {
	r := mustLoad(t, rule(`  a: { fact: "computation.net_profit_as_per_pl", label: "Profit" }
  b: { fact: "computation.depreciation_as_per_books", label: "Depreciation" }
  c: { fact: "computation.business_income", label: "Business income" }`, `  type: "sum_equals"
  parts: [ { input: "a", sign: "+" }, { input: "b", sign: "-" } ]
  total: "c"`))
	e := run(t, r, money2("computation.net_profit_as_per_pl", 1000), money2("computation.depreciation_as_per_books", 300), money2("computation.business_income", 650))
	var labels []string
	for _, v := range e.Values {
		labels = append(labels, v.Label)
	}
	if e.Status != domain.StatusFail || e.Difference != 5000 || strings.Join(labels, " / ") != "Profit / less: Depreciation / Sum of the figures above / Business income" {
		t.Errorf("status %v difference %d labels %q", e.Status, e.Difference, labels)
	}
	if e.Values[2].Locator.FileName != "" {
		t.Error("the sum the check computed has a source")
	}
}

// SPEC-007: percent_of on a sum of parts lists each part and the sum before the figure
// computed.
func TestPercentOfPartsShowsTheSum(t *testing.T) {
	r := mustLoad(t, rule(`  tax: { fact: "computation.mat.tax", label: "Tax" }
  sur: { fact: "computation.mat.surcharge", label: "Surcharge" }
  cess: { fact: "computation.mat.cess", label: "Cess" }`, `  type: "percent_of"
  parts: [ { input: "tax", sign: "+" }, { input: "sur", sign: "+" } ]
  rate: "4.00"
  value: "cess"`))
	e := run(t, r, money2("computation.mat.tax", 5713949), money2("computation.mat.surcharge", 399976), money2("computation.mat.cess", 228558))
	var labels []string
	for _, v := range e.Values {
		labels = append(labels, v.Label)
	}
	want := "Tax / add: Surcharge / Sum of the figures above / 4.00% of the figure above, to the rupee / Cess"
	if e.Status != domain.StatusFail || e.Difference != 1599900 || strings.Join(labels, " / ") != want {
		t.Errorf("status %v difference %d labels %q", e.Status, e.Difference, labels)
	}
}

func block(name string, opening, additions, deductions, depreciation, closing int64) domain.Record {
	amt := func(r int64) money.Paise { return money.Paise(r * 100) }
	return domain.Record{RecordType: "tar_depreciation_block", Period: "FY2025-26", MatchKey: name,
		Fields: map[string]string{"block": name},
		Amounts: map[string]money.Paise{"opening_wdv": amt(opening), "additions": amt(additions), "deductions": amt(deductions),
			"depreciation": amt(depreciation), "closing_wdv": amt(closing)},
		Locator: domain.Locator{FileName: "f.json", Kind: domain.LocatorJSONPath, Path: "$.blocks." + name}}
}

// SPEC-007: sum_equals on rows gives one result per row, in key order, each with its own
// parts, sum and difference; a row missing an amount is Cannot check, never a finding.
func TestSumEqualsOnRows(t *testing.T) {
	r := mustLoad(t, rule(`  block: { record_type: "tar_depreciation_block", label: "Block" }`, `  type: "sum_equals"
  parts:
    - { input: "block", field: "opening_wdv", sign: "+" }
    - { input: "block", field: "additions", sign: "+" }
    - { input: "block", field: "deductions", sign: "-" }
    - { input: "block", field: "depreciation", sign: "-" }
  total: "block"
  total_field: "closing_wdv"`))
	partial := block("Buildings", 100, 0, 0, 10, 90)
	delete(partial.Amounts, "deductions")
	ty, _ := period.ParseTaxYear("2025-26")
	evs, err := engine.Evaluate([]*rules.Rule{r}, nil, []domain.Record{block("Computers", 42760, 66600, 0, 33354, 78454), block("Plant", 296180, 142300, 0, 57653, 380827), partial},
		ty, checks.All(), 1)
	if err != nil || len(evs) != 3 {
		t.Fatalf("evaluations %v, err %v", evs, err)
	}
	got := map[string]domain.Evaluation{}
	for _, e := range evs {
		got[e.Subject.MatchKey] = e
	}
	if e := got["Computers"]; e.Status != domain.StatusFail || e.Difference != 244800 || len(e.Values) != 6 || e.Values[4].Label != "Sum of the figures above" {
		t.Errorf("Computers: status %v difference %d values %v", e.Status, e.Difference, e.Values)
	}
	if e := got["Plant"]; e.Status != domain.StatusPass {
		t.Errorf("Plant: status %v difference %d", e.Status, e.Difference)
	}
	if e := got["Buildings"]; e.Status != domain.StatusCannotCheck || strings.Join(e.MissingFacts, ",") != "deductions" {
		t.Errorf("Buildings: status %v missing %v", e.Status, e.MissingFacts)
	}
}

func TestTableLookupKeysNotInTable(t *testing.T) {
	r := mustLoad(t, rule(`  country: { fact: "ftc.country", label: "Country" }
  article: { fact: "ftc.dtaa_article", label: "Article" }
  rate: { fact: "ftc.dtaa_rate", label: "Rate" }`, `  type: "table_lookup"
  keys: ["country", "article"]
  value: "rate"
  table:
    - { keys: ["PORTUGAL", "12"], value: "10.00", source: "Treaty" }`))
	rate := domain.Fact{Name: "ftc.dtaa_rate", Period: "FY2025-26", Value: domain.Value{Kind: domain.KindRate, RateBP: 1000}}
	e := run(t, r, text("ftc.country", "FRANCE"), text("ftc.dtaa_article", "12"), rate)
	if e.Status != domain.StatusCannotCheck || len(e.MissingFacts) != 1 || e.MissingFacts[0] != "table:FRANCE|12" {
		t.Errorf("keys not in the table: status %v missing %v", e.Status, e.MissingFacts)
	}
}

func TestRefusedRules(t *testing.T) {
	for name, text := range map[string]string{
		"equal with parts": rule(`  a: { fact: "computation.business_income", label: "A" }
  b: { fact: "computation.total_income", label: "B" }`, `  type: "equal"
  parts: [ { input: "a", sign: "+" }, { input: "b", sign: "+" } ]`),
		"sum_equals with a text part": rule(`  a: { fact: "ftc.country", label: "A" }
  b: { fact: "computation.total_income", label: "B" }
  c: { fact: "computation.business_income", label: "C" }`, `  type: "sum_equals"
  parts: [ { input: "a", sign: "+" }, { input: "b", sign: "+" } ]
  total: "c"`),
		"sum_equals with an unused input": rule(`  a: { fact: "computation.gross_total_income", label: "A" }
  b: { fact: "computation.total_income", label: "B" }
  c: { fact: "computation.business_income", label: "C" }
  d: { fact: "computation.taxes_paid.tds", label: "D" }`, `  type: "sum_equals"
  parts: [ { input: "a", sign: "+" }, { input: "b", sign: "+" } ]
  total: "c"`),
		"sum_equals with a bad sign": rule(`  a: { fact: "computation.gross_total_income", label: "A" }
  b: { fact: "computation.total_income", label: "B" }
  c: { fact: "computation.business_income", label: "C" }`, `  type: "sum_equals"
  parts: [ { input: "a", sign: "+" }, { input: "b", sign: "plus" } ]
  total: "c"`),
		"present_together testing money as text": rule(`  a: { fact: "computation.total_income", label: "A" }`, `  type: "present_together"
  when: [ { input: "a", is: "blank" } ]`),
		"present_together with an untested input": rule(`  a: { fact: "tar.udin", label: "A" }
  b: { fact: "mat.udin", label: "B" }`, `  type: "present_together"
  when: [ { input: "a", is: "blank" } ]`),
		"present_together on an unknown column": rule(`  rows: { record_type: "tar_loan_taken", label: "Rows" }`, `  type: "present_together"
  when: [ { input: "rows", field: "colour", is: "blank" } ]`),
		"present_together with a bad pattern": rule(`  a: { fact: "tar.udin", label: "A" }`, `  type: "present_together"
  when: [ { input: "a", is: "matches", value: "(" } ]`),
		"percent_of without a rate": rule(`  base: { fact: "mat.book_profit", label: "A" }
  value: { fact: "mat.fifteen_percent", label: "B" }`, `  type: "percent_of"
  base: "base"
  value: "value"`),
		"table_lookup with a repeated row": rule(`  country: { fact: "ftc.country", label: "C" }
  rate: { fact: "ftc.dtaa_rate", label: "R" }`, `  type: "table_lookup"
  keys: ["country"]
  value: "rate"
  table:
    - { keys: ["PORTUGAL"], value: "10.00", source: "T" }
    - { keys: ["portugal"], value: "15.00", source: "T" }`),
		"sum_equals on rows with a field that is not an amount": rule(`  block: { record_type: "tar_depreciation_block", label: "B" }`, `  type: "sum_equals"
  parts: [ { input: "block", field: "block", sign: "+" }, { input: "block", field: "additions", sign: "+" } ]
  total: "block"
  total_field: "closing_wdv"`),
		"sum_equals on rows without total_field": rule(`  block: { record_type: "tar_depreciation_block", label: "B" }`, `  type: "sum_equals"
  parts: [ { input: "block", field: "opening_wdv", sign: "+" }, { input: "block", field: "additions", sign: "+" } ]
  total: "block"`),
		"sum_equals on rows using an amount twice": rule(`  block: { record_type: "tar_depreciation_block", label: "B" }`, `  type: "sum_equals"
  parts: [ { input: "block", field: "opening_wdv", sign: "+" }, { input: "block", field: "closing_wdv", sign: "+" } ]
  total: "block"
  total_field: "closing_wdv"`),
		"sum_equals with a field on a figure": rule(`  a: { fact: "computation.gross_total_income", label: "A" }
  b: { fact: "computation.total_income", label: "B" }
  c: { fact: "computation.business_income", label: "C" }`, `  type: "sum_equals"
  parts: [ { input: "a", field: "x", sign: "+" }, { input: "b", sign: "+" } ]
  total: "c"`),
		"total_field outside sum_equals": rule(`  a: { fact: "computation.business_income", label: "A" }
  b: { fact: "computation.total_income", label: "B" }`, `  type: "equal"
  total_field: "closing_wdv"`),
		"percent_of with base and parts": rule(`  tax: { fact: "computation.mat.tax", label: "T" }
  sur: { fact: "computation.mat.surcharge", label: "S" }
  cess: { fact: "computation.mat.cess", label: "C" }`, `  type: "percent_of"
  base: "tax"
  parts: [ { input: "tax", sign: "+" }, { input: "sur", sign: "+" } ]
  rate: "4.00"
  value: "cess"`),
		"percent_of with one part": rule(`  tax: { fact: "computation.mat.tax", label: "T" }
  cess: { fact: "computation.mat.cess", label: "C" }`, `  type: "percent_of"
  parts: [ { input: "tax", sign: "+" } ]
  rate: "4.00"
  value: "cess"`),
		"percent_of with the value among the parts": rule(`  tax: { fact: "computation.mat.tax", label: "T" }
  cess: { fact: "computation.mat.cess", label: "C" }`, `  type: "percent_of"
  parts: [ { input: "tax", sign: "+" }, { input: "cess", sign: "+" } ]
  rate: "4.00"
  value: "cess"`),
		"gte on text": rule(`  a: { fact: "ftc.country", label: "A" }
  b: { fact: "computation.total_income", label: "B" }`, `  type: "gte"`),
	} {
		if _, err := load(t, text); err == nil {
			t.Errorf("%s: loaded", name)
		}
	}
}

// bankEntry is a bank book line; unread names the amounts left unread for want of a dollar rate.
func bankEntry(ref string, receipt int64, unread string) domain.Record {
	rec := domain.Record{RecordType: "bank_book_entry", Period: "FY2026-27", MatchKey: ref,
		Fields:  map[string]string{"reference": ref},
		Amounts: map[string]money.Paise{"receipt": money.Paise(receipt * 100)},
		Locator: domain.Locator{FileName: "f.json", Kind: domain.LocatorJSONPath, Path: "$.entries." + ref}}
	if unread != "" {
		rec.Amounts = map[string]money.Paise{}
		rec.Fields[domain.UnreadField] = "dollar rate not set|" + unread
	}
	return rec
}

// SPEC-019: an amount left unread (a dollar amount with no rate) is "cannot check" with its
// reason in a row sum, never "not in the document" and never a sum over the remaining amounts.
func TestSumEqualsOnRowsUnreadAmount(t *testing.T) {
	r := mustLoad(t, rule(`  block: { record_type: "tar_depreciation_block", label: "Block" }`, `  type: "sum_equals"
  parts:
    - { input: "block", field: "opening_wdv", sign: "+" }
    - { input: "block", field: "additions", sign: "+" }
    - { input: "block", field: "depreciation", sign: "-" }
  total: "block"
  total_field: "closing_wdv"`))
	unread := block("Computers", 100, 50, 0, 10, 140)
	unread.Amounts = map[string]money.Paise{}
	unread.Fields[domain.UnreadField] = "dollar rate not set|opening_wdv,additions,deductions,depreciation,closing_wdv"
	ty, _ := period.ParseTaxYear("2025-26")
	evs, err := engine.Evaluate([]*rules.Rule{r}, nil, []domain.Record{unread}, ty, checks.All(), 1)
	if err != nil || len(evs) != 1 {
		t.Fatalf("evaluations %v, err %v", evs, err)
	}
	e := evs[0]
	if e.Status != domain.StatusCannotCheck || len(e.MissingFacts) == 0 {
		t.Fatalf("status %v missing %q", e.Status, e.MissingFacts)
	}
	for _, m := range e.MissingFacts {
		if !strings.HasPrefix(m, "unread:tar_depreciation_block.") || !strings.HasSuffix(m, "~dollar rate not set") {
			t.Errorf("missing entry %q is not an unread amount with its reason", m)
		}
	}
}

// SPEC-019: present_together on rows does not test an unread amount as zero or as text.
func TestPresentTogetherOnRowsUnreadAmount(t *testing.T) {
	r := mustLoad(t, rule(`  e: { record_type: "bank_book_entry", label: "Bank book" }`, `  type: "present_together"
  when:
    - { input: "e", field: "receipt", is: "zero" }`))
	ty, _ := period.ParseTaxYear("2026-27")
	evs, err := engine.Evaluate([]*rules.Rule{r}, nil, []domain.Record{bankEntry("U1", 0, "receipt"), bankEntry("R1", 0, "")}, ty, checks.All(), 1)
	if err != nil || len(evs) != 2 {
		t.Fatalf("evaluations %v, err %v", evs, err)
	}
	got := map[string]domain.Evaluation{}
	for _, e := range evs {
		got[e.Subject.MatchKey] = e
	}
	if e := got["U1"]; e.Status != domain.StatusCannotCheck || strings.Join(e.MissingFacts, ",") != "unread:bank_book_entry.receipt~dollar rate not set" {
		t.Errorf("unread row: status %v missing %q", e.Status, e.MissingFacts)
	}
	if e := got["R1"]; e.Status != domain.StatusFail {
		t.Errorf("read row with a zero receipt: status %v", e.Status)
	}
}
