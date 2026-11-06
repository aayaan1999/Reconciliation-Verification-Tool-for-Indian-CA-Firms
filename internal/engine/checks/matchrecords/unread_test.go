package matchrecords

import (
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
)

// An amount that was in a document but was not read (SPEC-019: an amount in US dollars with no
// usable rate) is never shown as zero, never counted as agreeing, and is named with its reason
// wherever a result would otherwise leave it out. Figures invented.

const noRate = "dollar rate not set"

// unread marks the named amounts of a row as left unread, as the reader does: the amounts are
// gone from the row and the reserved field names them with the reason.
func unread(r domain.Record, names ...string) domain.Record {
	for _, n := range names {
		delete(r.Amounts, n)
	}
	r.Fields[domain.UnreadField] = noRate + "|" + strings.Join(names, ",")
	return r
}

// A line of an invoice that left the invoice-level amount unread is not one of the lines that
// disagree, and it has no figure to show: it is not listed as 0.00.
func TestLinesDifferListsOnlyLinesThatHoldTheAmount(t *testing.T) {
	l1 := sales("I9", g1, "01-08-2026", 500, 90, 1180)
	l2 := sales("I9", g1, "01-08-2026", 500, 90, 1190)
	l3 := unread(sales("I9", g1, "01-08-2026", 0, 0, 0), "invoice_value")
	out, err := Check{}.EvaluateRecords(testRule(), []domain.Record{l1, l2, l3}, []domain.Record{gstr1("I9", g1, "01-08-2026", 1000, 180, 1180)})
	if err != nil || len(out) != 1 || out[0].Code != LinesDiffer {
		t.Fatalf("%v %v, want one %s", out, err, LinesDiffer)
	}
	var inputs []string
	for _, v := range out[0].Values {
		inputs = append(inputs, v.Input)
		if v.Value.Kind != domain.KindMoney || v.Value.Money == 0 {
			t.Errorf("%s shows %+v, want a read, non-zero amount", v.Input, v.Value)
		}
	}
	if want := "a.invoice_value.1,a.invoice_value.2"; strings.Join(inputs, ",") != want {
		t.Errorf("lines shown %v, want %s (the line that left the amount out is not listed)", inputs, want)
	}
}

// Two rows are not claimed to be the same invoice written with different numbers when a compared
// amount of either was unread: that amount is not known to agree. An amount the layout never
// reads (absent, not unread) still does not stop the pairing, as before.
func TestNoPairWhenAComparedAmountIsUnread(t *testing.T) {
	a := sales("KCD/0412", g1, "05-08-2026", 1000, 180, 1180)
	for name, tc := range map[string]struct {
		b    domain.Record
		want [2]string
	}{
		"igst unread on the other side": {unread(gstr1("KCD-412", g1, "05-08-2026", 1000, 180, 1180), "igst"), [2]string{OnlyInA, OnlyInB}},
		"igst absent from its layout": {func() domain.Record {
			r := gstr1("KCD-412", g1, "05-08-2026", 1000, 180, 1180)
			delete(r.Amounts, "igst")
			return r
		}(), [2]string{KeyDiff, ""}},
	} {
		got := codes(t, []domain.Record{a}, []domain.Record{tc.b})
		if tc.want[1] == "" {
			if got["KCD/0412"] != tc.want[0] || len(got) != 1 {
				t.Errorf("%s: codes %v, want the pair reported once as %s", name, got, tc.want[0])
			}
			continue
		}
		if got["KCD/0412"] != tc.want[0] || got["KCD-412"] != tc.want[1] {
			t.Errorf("%s: codes %v, want %s and %s", name, got, tc.want[0], tc.want[1])
		}
	}
	// the same when the unread amount is on the first side
	b := gstr1("KCD-412", g1, "05-08-2026", 1000, 180, 1180)
	got := codes(t, []domain.Record{unread(sales("KCD/0412", g1, "05-08-2026", 1000, 180, 1180), "igst")}, []domain.Record{b})
	if got["KCD/0412"] != OnlyInA || got["KCD-412"] != OnlyInB {
		t.Errorf("unread on the first side: codes %v, want both unmatched", got)
	}
}

// A row found in one document only says which of its compared amounts was not read, with the reason,
// in place of leaving it out; the amounts that were read are shown as before and the unread one is
// never a money value.
func TestOneSidedRowNamesAnUnreadAmount(t *testing.T) {
	a := unread(sales("ONLY-1", g1, "05-08-2026", 1000, 180, 1180), "igst")
	out, err := Check{}.EvaluateRecords(testRule(), []domain.Record{a}, nil)
	if err != nil || len(out) != 1 || out[0].Code != OnlyInA {
		t.Fatalf("%v %v, want one %s", out, err, OnlyInA)
	}
	var shown = map[string]domain.Value{}
	for _, v := range out[0].Values {
		shown[v.Input] = v.Value
	}
	if v := shown["a.taxable_value"]; v.Kind != domain.KindMoney || v.Money != 100000 {
		t.Errorf("the amount that was read: %+v", v)
	}
	v := shown["a.igst"]
	if v.Kind != domain.KindText || !strings.Contains(v.Text, noRate) || !strings.HasPrefix(v.Text, "not read") {
		t.Errorf("the unread amount is shown as %+v, want text saying it was not read and why", v)
	}
	if out[0].Difference != 100000 {
		t.Errorf("difference %d, want 100000 (from the amount that was read only)", out[0].Difference)
	}

	// with every compared amount unread the key is still shown, so the finding says where the row is
	all := unread(sales("ONLY-2", g1, "05-08-2026", 1000, 180, 1180), "taxable_value", "igst")
	out, err = Check{}.EvaluateRecords(testRule(), nil, []domain.Record{all})
	if err != nil || len(out) != 1 || out[0].Code != OnlyInB {
		t.Fatalf("%v %v, want one %s", out, err, OnlyInB)
	}
	var key bool
	for _, v := range out[0].Values {
		key = key || (v.Input == "b.invoice_number" && v.Value.Text == "ONLY-2")
		if v.Value.Kind == domain.KindMoney {
			t.Errorf("%s is a money value, want none for an unread row", v.Input)
		}
	}
	if !key {
		t.Errorf("the key is not shown: %+v", out[0].Values)
	}
}
