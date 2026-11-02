package newheads

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
)

var th = Thresholds{UpHundredths: 200, DownHundredths: 50, Minimum: 100000 * 100}

// rows makes one ledger row per month for a head, each of amount rupees.
func rows(name string, rupees int64, months ...string) []domain.Record {
	var out []domain.Record
	for _, m := range months {
		out = append(out, domain.Record{RecordType: RecordType,
			Fields:  map[string]string{"ledger_head": name, "group": "G", "month": m},
			Amounts: map[string]money.Paise{"amount": money.Paise(rupees * 100)},
			Locator: domain.Locator{FileName: "l.csv", Kind: domain.LocatorJSONPath, Path: "$[" + m + "]"}})
	}
	return out
}

func join(rs ...[]domain.Record) []domain.Record { return slices.Concat(rs...) }

var (
	py = []string{"2024-04", "2024-05", "2024-06"}
	cy = []string{"2025-04", "2025-05", "2025-06"}
)

func summary(items []Item) []string {
	var out []string
	for _, it := range items {
		s := it.Key()
		if len(it.MissingMonths) > 0 {
			s += " missing " + strings.Join(it.MissingMonths, ",")
		}
		if it.Times != "" {
			s += " x" + it.Times
		}
		out = append(out, s)
	}
	return out
}

func TestEachKind(t *testing.T) {
	prior := join(rows("Rent", 50000, py...), rows("Courier", 4000, py...), rows("Power", 9000, py...),
		rows("Travel", 100000, py...), rows("Fees", 200000, py...), rows("Odd", 10, py[:1]...))
	current := join(rows("Rent", 50000, cy...), rows("Ads", 1000, cy[:1]...), rows("Power", 9000, cy[0], cy[2]),
		rows("Travel", 400000, cy...), rows("Fees", 99000, cy...), rows("Odd", 10, cy...))
	items, err := Compare(prior, current, th)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"new_head|Ads", "vanished|Courier", "broken_recurrence|Power missing 2025-05",
		"step_change|Fees x0.50", "step_change|Travel x4.00"}
	// Odd goes from 10 rupees to 30, three times, but under the minimum: not listed.
	if got := summary(items); !slices.Equal(got, want) {
		t.Errorf("items\n got %q\nwant %q", got, want)
	}
	if it := items[0]; it.CurrentTotal != 100000 || it.CurrentMonths != 1 || it.PriorMonths != 0 || len(it.CurrentLocators) != 1 {
		t.Errorf("new head %+v", it)
	}
}

func TestStepChangeEdges(t *testing.T) {
	for _, c := range []struct {
		prior, current int64 // rupees
		want           string
	}{
		{100000, 199999, ""},      // just under twice
		{100000, 200000, "2.00"},  // exactly twice
		{100000, 50001, ""},       // just over half
		{100000, 50000, "0.50"},   // exactly half
		{40000, 90000, ""},        // more than twice, but the larger is under the minimum
		{983627, 3772925, "3.84"}, // F30 of Test Pack A
		{3, 1000000, "333333.33"},
	} {
		got, ok := stepChange(money.Paise(c.prior*100), money.Paise(c.current*100), th)
		if got != c.want || ok != (c.want != "") {
			t.Errorf("%d to %d: %q %v, want %q", c.prior, c.current, got, ok, c.want)
		}
	}
}

func TestOneYearOnly(t *testing.T) {
	// Without last year's ledger nothing is new; without this year's nothing has vanished.
	for _, c := range [][2][]domain.Record{{nil, rows("Rent", 1, cy...)}, {rows("Rent", 1, py...), nil}} {
		if items, err := Compare(c[0], c[1], th); err != nil || len(items) != 0 {
			t.Errorf("items %v, err %v", summary(items), err)
		}
	}
}

func TestDeterministic(t *testing.T) {
	prior := join(rows("A", 1, py...), rows("B", 1, py...), rows("C", 1, py...))
	current := join(rows("D", 1, cy...), rows("E", 1, cy...), rows("C", 1, cy[:2]...))
	first, _ := Compare(prior, current, th)
	for range 50 {
		slices.Reverse(current)
		got, _ := Compare(prior, current, th)
		if !slices.Equal(summary(got), summary(first)) || !slices.Equal(got[2].CurrentLocators, first[2].CurrentLocators) {
			t.Fatalf("order changed: %v vs %v", summary(got), summary(first))
		}
	}
}

func TestThresholdsFile(t *testing.T) {
	data, err := os.ReadFile("../../../content/worklist/newheads.yaml")
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseThresholds("newheads.yaml", data)
	if err != nil || got != th {
		t.Errorf("thresholds %+v, err %v; want %+v", got, err, th)
	}
	for _, bad := range []string{
		"schema: \"recon.newheads/1\"\nstep_change: { up_times: \"0.90\", down_times: \"0.50\", minimum_inr: \"1\" }\n",
		"schema: \"recon.newheads/1\"\nstep_change: { up_times: \"two\", down_times: \"0.50\", minimum_inr: \"1\" }\n",
		"schema: \"x\"\nstep_change: { up_times: \"2.00\", down_times: \"0.50\", minimum_inr: \"1\" }\n",
		"schema: \"recon.newheads/1\"\nstep_change: { up_times: \"2.00\", down_times: \"0.50\", minimum_inr: \"1\", extra: \"1\" }\n",
	} {
		if _, err := ParseThresholds("bad.yaml", []byte(bad)); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}
