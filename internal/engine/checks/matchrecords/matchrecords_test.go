package matchrecords

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
)

// testRule compares sales register rows (a) with GSTR-1 rows (b) keyed on invoice number,
// as RECON-GST-001 does, with a Rs 1 tolerance.
func testRule() *rules.Rule {
	key := []string{"invoice_number"}
	return &rules.Rule{
		ID: "TEST-MATCH",
		Inputs: []rules.Input{
			{Name: "a", RecordType: "sales_b2b_invoice", MatchKey: key, Label: "Sales register"},
			{Name: "b", RecordType: "gstr1_b2b_invoice", MatchKey: key, Label: "GSTR-1"},
		},
		Check: rules.Check{
			Type:            Name,
			Tolerance:       rules.Tolerance{Kind: rules.TolerancePaise, Value: 100},
			CompareAmounts:  []string{"taxable_value", "igst"},
			CompareFields:   []string{"counterparty_gstin"},
			DuplicatesIn:    []string{"b"},
			InvoiceAmounts:  []string{"invoice_value"},
			PairUnmatchedOn: []string{"counterparty_gstin", "invoice_date"},
		},
	}
}

var rowNo int

// row is one register line; amounts are in rupees for readability.
func row(file, key, gstin, date string, taxable, igst, invoice int64) domain.Record {
	rowNo++
	return domain.Record{
		MatchKey: key,
		Amounts: map[string]money.Paise{
			"taxable_value": money.Paise(taxable * 100), "igst": money.Paise(igst * 100), "invoice_value": money.Paise(invoice * 100),
		},
		Fields:  map[string]string{"invoice_number": key, "counterparty_gstin": gstin, "invoice_date": date},
		Locator: domain.Locator{FileName: file, Kind: "json_path", Path: fmt.Sprintf("$.rows[%d]", rowNo)},
	}
}

func sales(key, gstin, date string, taxable, igst, invoice int64) domain.Record {
	return row("SALES.json", key, gstin, date, taxable, igst, invoice)
}

func gstr1(key, gstin, date string, taxable, igst, invoice int64) domain.Record {
	return row("GSTR1.json", key, gstin, date, taxable, igst, invoice)
}

const g1, g2 = "27AAAAA0000A1Z5", "29BBBBB1111B1Z6"

func codes(t *testing.T, a, b []domain.Record) map[string]string {
	t.Helper()
	out, err := Check{}.EvaluateRecords(testRule(), a, b)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, o := range out {
		if _, seen := got[o.MatchKey]; seen {
			t.Errorf("key %s reported twice", o.MatchKey)
		}
		got[o.MatchKey] = o.Code
	}
	return got
}

func TestEveryResultCode(t *testing.T) {
	for _, c := range []struct {
		name string
		a, b []domain.Record
		want map[string]string
	}{
		{"matched", []domain.Record{sales("I1", g1, "01-08-2026", 1000, 180, 1180)},
			[]domain.Record{gstr1("I1", g1, "01-08-2026", 1000, 180, 1180)}, map[string]string{"I1": Matched}},
		{"within the Rs 1 tolerance", []domain.Record{sales("I1", g1, "01-08-2026", 1000, 180, 1180)},
			[]domain.Record{gstr1("I1", g1, "01-08-2026", 1001, 180, 1181)}, map[string]string{"I1": Matched}},
		{"amount beyond the tolerance", []domain.Record{sales("I1", g1, "01-08-2026", 1000, 180, 1180)},
			[]domain.Record{gstr1("I1", g1, "01-08-2026", 1002, 180, 1182)}, map[string]string{"I1": AmountDiff}},
		{"GSTIN differs (case and spaces ignored when equal)", []domain.Record{sales("I1", g1, "01-08-2026", 1000, 180, 1180), sales("I2", g1, "", 5, 0, 5)},
			[]domain.Record{gstr1("I1", g2, "01-08-2026", 1000, 180, 1180), gstr1("I2", " "+toLower(g1)+" ", "", 5, 0, 5)},
			map[string]string{"I1": FieldDiff, "I2": Matched}},
		{"only in one input", []domain.Record{sales("I1", g1, "01-08-2026", 1000, 180, 1180)},
			[]domain.Record{gstr1("I2", g2, "02-08-2026", 500, 90, 590)}, map[string]string{"I1": OnlyInA, "I2": OnlyInB}},
		{"duplicated in GSTR-1", []domain.Record{sales("I1", g1, "01-08-2026", 1000, 180, 1180)},
			[]domain.Record{gstr1("I1", g1, "01-08-2026", 1000, 180, 1180), gstr1("I1", g1, "01-08-2026", 1000, 180, 1180)},
			map[string]string{"I1": DuplicateB}},
		{"lines of one invoice in the register are added together", []domain.Record{
			sales("I1", g1, "01-08-2026", 600, 108, 1180), sales("I1", g1, "01-08-2026", 400, 72, 1180)},
			[]domain.Record{gstr1("I1", g1, "01-08-2026", 1000, 180, 1180)}, map[string]string{"I1": Matched}},
		{"an invoice value that differs between lines is not totalled", []domain.Record{
			sales("I1", g1, "01-08-2026", 600, 108, 1180), sales("I1", g1, "01-08-2026", 400, 72, 1190)},
			[]domain.Record{gstr1("I1", g1, "01-08-2026", 1000, 180, 1180)}, map[string]string{"I1": LinesDiffer}},
		{"number written differently, single candidate both ways", []domain.Record{sales("KCD/0412", g1, "05-08-2026", 1000, 180, 1180)},
			[]domain.Record{gstr1("KCD-412", g1, "05-08-2026", 1000, 180, 1180)}, map[string]string{"KCD/0412": KeyDiff}},
		{"different date: not paired, both stay unmatched", []domain.Record{sales("KCD/0412", g1, "05-08-2026", 1000, 180, 1180)},
			[]domain.Record{gstr1("KCD-412", g1, "06-08-2026", 1000, 180, 1180)}, map[string]string{"KCD/0412": OnlyInA, "KCD-412": OnlyInB}},
		{"two possible partners: nothing is guessed", []domain.Record{sales("X1", g1, "05-08-2026", 1000, 180, 1180)},
			[]domain.Record{gstr1("Y1", g1, "05-08-2026", 1000, 180, 1180), gstr1("Y2", g1, "05-08-2026", 1000, 180, 1180)},
			map[string]string{"X1": OnlyInA, "Y1": OnlyInB, "Y2": OnlyInB}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := codes(t, c.a, c.b); !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// A finding shows both values, the difference and both sources.
func TestAmountDiffShowsBothSides(t *testing.T) {
	a := sales("I1", g1, "01-08-2026", 450000, 81000, 531000)
	b := gstr1("I1", g1, "01-08-2026", 405000, 72900, 477900)
	out, err := Check{}.EvaluateRecords(testRule(), []domain.Record{a}, []domain.Record{b})
	if err != nil || len(out) != 1 {
		t.Fatalf("%v %v", out, err)
	}
	o := out[0]
	if o.Status != domain.StatusFail || o.Code != AmountDiff || o.Difference != money.Paise(45000*100) {
		t.Errorf("status %v code %s difference %d", o.Status, o.Code, o.Difference)
	}
	var inputs []string
	for _, v := range o.Values {
		inputs = append(inputs, v.Input)
		want := a.Locator
		if v.Input[0] == 'b' {
			want = b.Locator
		}
		if v.Locator != want {
			t.Errorf("%s: locator %v, want %v", v.Input, v.Locator, want)
		}
	}
	if want := []string{"a.taxable_value", "b.taxable_value", "a.igst", "b.igst"}; !slices.Equal(inputs, want) {
		t.Errorf("values %v, want %v", inputs, want)
	}
}

// The same rows in any order give the same outcomes (C4).
func TestOrderDoesNotMatter(t *testing.T) {
	a := []domain.Record{sales("I3", g1, "", 1, 0, 1), sales("I1", g1, "", 2, 0, 2), sales("I2", g2, "", 3, 0, 3), sales("I1", g1, "", 4, 0, 2)}
	b := []domain.Record{gstr1("I2", g2, "", 3, 0, 3), gstr1("I4", g1, "", 9, 0, 9), gstr1("I1", g1, "", 6, 0, 2)}
	first, err := Check{}.EvaluateRecords(testRule(), a, b)
	if err != nil {
		t.Fatal(err)
	}
	slices.Reverse(a)
	slices.Reverse(b)
	second, err := Check{}.EvaluateRecords(testRule(), a, b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("order changed the outcome:\n%v\n%v", first, second)
	}
}

// An amount no row of a document holds (its layout does not read that column) is missing
// there, never zero: a row that agrees on everything else cannot be checked in full. An
// amount only some rows lack is zero in those rows, as GST portal files leave zeros out
// (SPEC-008 part 2).
func TestAmountNoRowHoldsIsMissing(t *testing.T) {
	a1, a2 := sales("INV-1", g1, "2026-08-03", 1000, 180, 1180), sales("INV-2", g1, "2026-08-04", 500, 0, 500)
	b1, b2 := gstr1("INV-1", g1, "2026-08-03", 1000, 180, 1180), gstr1("INV-2", g1, "2026-08-04", 500, 0, 500)
	delete(a1.Amounts, "igst")
	delete(a2.Amounts, "igst")
	out, err := Check{}.EvaluateRecords(testRule(), []domain.Record{a1, a2}, []domain.Record{b1, b2})
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range out {
		if o.Status != domain.StatusCannotCheck || !slices.Equal(o.Missing, []string{rules.MissingAmountPrefix + "sales_b2b_invoice.igst"}) {
			t.Errorf("%s: %s %v, want cannot check, igst missing from the sales register", o.MatchKey, o.Status, o.Missing)
		}
	}

	// Only INV-2 lacks igst on the GSTR-1 side: it is zero there, and both rows match.
	a1, a2 = sales("INV-1", g1, "2026-08-03", 1000, 180, 1180), sales("INV-2", g1, "2026-08-04", 500, 0, 500)
	b1, b2 = gstr1("INV-1", g1, "2026-08-03", 1000, 180, 1180), gstr1("INV-2", g1, "2026-08-04", 500, 0, 500)
	delete(b2.Amounts, "igst")
	got := codes(t, []domain.Record{a1, a2}, []domain.Record{b1, b2})
	if got["INV-1"] != Matched || got["INV-2"] != Matched {
		t.Errorf("codes %v, want both matched", got)
	}
}

// An amount a document's layout never reads is never shown as zero or compared as zero: a
// row only in that document leaves it out, and pairing does not use it (code review
// finding 2).
func TestMissingAmountNeverZeroOutsideCompare(t *testing.T) {
	a := sales("KCD/0412", g1, "05-08-2026", 1000, 180, 1180)
	b := gstr1("KCD-412", g1, "05-08-2026", 1000, 180, 1180)
	other := gstr1("ZZ-9", g2, "09-08-2026", 700, 0, 700)
	for _, r := range []*domain.Record{&b, &other} {
		delete(r.Amounts, "igst") // GSTR-1's layout holds no igst column at all
	}
	out, err := Check{}.EvaluateRecords(testRule(), []domain.Record{a}, []domain.Record{b, other})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]rules.RecordOutcome{}
	for _, o := range out {
		got[o.MatchKey] = o
	}
	// The pair still agrees on GSTIN, date and taxable value (the amount both hold).
	if got["KCD/0412"].Code != KeyDiff {
		t.Errorf("KCD/0412: code %s, want %s", got["KCD/0412"].Code, KeyDiff)
	}
	o := got["ZZ-9"]
	if o.Code != OnlyInB {
		t.Fatalf("ZZ-9: code %s, want %s", o.Code, OnlyInB)
	}
	for _, v := range o.Values {
		if v.Input == "b.igst" {
			t.Errorf("ZZ-9 shows %s = %v, an amount GSTR-1 never holds", v.Label, v.Value)
		}
	}
	if o.Difference != money.Paise(700*100) {
		t.Errorf("ZZ-9 difference %d, want the taxable value", o.Difference)
	}
}

// With no compared amount held by both documents, nothing is paired on fields alone.
func TestNoPairWithoutAnAmountBothHold(t *testing.T) {
	a := sales("KCD/0412", g1, "05-08-2026", 1000, 180, 1180)
	b := gstr1("KCD-412", g1, "05-08-2026", 1000, 180, 1180)
	delete(b.Amounts, "igst")
	delete(b.Amounts, "taxable_value")
	got := codes(t, []domain.Record{a}, []domain.Record{b})
	if got["KCD/0412"] != OnlyInA || got["KCD-412"] != OnlyInB {
		t.Errorf("codes %v, want both unmatched", got)
	}
}

// An invoice-level amount is taken from the first line that holds it, so a first line that
// leaves it out does not make the other lines disagree (code review finding 3).
func TestInvoiceAmountFromFirstLineHoldingIt(t *testing.T) {
	l1 := sales("I7", g1, "01-08-2026", 600, 108, 0)
	delete(l1.Amounts, "invoice_value")
	l2 := sales("I7", g1, "01-08-2026", 400, 72, 1180)
	l3 := sales("I7", g1, "01-08-2026", 0, 0, 1180)
	got := codes(t, []domain.Record{l1, l2, l3}, []domain.Record{gstr1("I7", g1, "01-08-2026", 1000, 180, 1180)})
	if got["I7"] != Matched {
		t.Errorf("code %s, want %s", got["I7"], Matched)
	}
}

// Lines are labelled in file order with row numbers read as numbers: $.rows[2] is line 1
// and $.rows[10] is line 2 (code review finding 4).
func TestLinesFollowFileOrder(t *testing.T) {
	at := func(path string, invoice int64) domain.Record {
		r := sales("I1", g1, "01-08-2026", 500, 90, invoice)
		r.Locator.Path = path
		return r
	}
	out, err := Check{}.EvaluateRecords(testRule(), []domain.Record{at("$.rows[10]", 1190), at("$.rows[2]", 1180)},
		[]domain.Record{gstr1("I1", g1, "01-08-2026", 1000, 180, 1180)})
	if err != nil || len(out) != 1 || out[0].Code != LinesDiffer {
		t.Fatalf("%v %v, want one %s", out, err, LinesDiffer)
	}
	var paths []string
	for _, v := range out[0].Values {
		paths = append(paths, v.Locator.Path)
	}
	if want := []string{"$.rows[2]", "$.rows[10]"}; !slices.Equal(paths, want) {
		t.Errorf("line order %v, want %v", paths, want)
	}
}

// A difference that does not fit in paise is an error, never a wrapped-around figure (code
// review finding 6).
func TestDifferenceOverflowIsAnError(t *testing.T) {
	a := sales("I1", g1, "01-08-2026", 0, 0, 0)
	b := gstr1("I1", g1, "01-08-2026", 0, 0, 0)
	a.Amounts["taxable_value"] = money.MaxPaise
	b.Amounts["taxable_value"] = -1
	if _, err := (Check{}).EvaluateRecords(testRule(), []domain.Record{a}, []domain.Record{b}); err == nil {
		t.Error("no error for a difference beyond the paise range")
	}
	if _, err := absDiff(money.MinPaise, 0); err == nil {
		t.Error("absDiff(MinPaise, 0) gave no error")
	}
}
