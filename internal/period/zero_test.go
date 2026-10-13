package period

import (
	"strings"
	"testing"
)

func TestZeroValuesAreNotValid(t *testing.T) {
	var p Period
	if !p.IsZero() || p.String() != "" {
		t.Errorf("zero Period: IsZero %v, String %q", p.IsZero(), p.String())
	}
	if !p.FirstDay().IsZero() || !p.LastDay().IsZero() {
		t.Errorf("zero Period spans %s..%s; want zero dates", p.FirstDay(), p.LastDay())
	}
	fy, _ := Parse("FY2025-26")
	if Contains(p, p) || Contains(fy, p) || Contains(p, fy) {
		t.Error("the zero Period contains or is contained in something")
	}
	if (Date{}).Valid() || (TaxYear{}).Valid() {
		t.Error("zero Date or TaxYear reports Valid")
	}
	if !(Date{}).IsZero() || !(TaxYear{}).IsZero() {
		t.Error("zero Date or TaxYear does not report IsZero")
	}
	if !Year(TaxYear{}).IsZero() {
		t.Error("Year of the zero TaxYear is not the zero Period")
	}
	if _, err := Quarter(TaxYear{}, 1); err == nil {
		t.Error("Quarter of the zero TaxYear accepted")
	}
}

func TestHandBuiltDates(t *testing.T) {
	for _, d := range []Date{{}, {2025, 2, 30}, {2025, 13, 1}, {2025, 0, 1}, {2025, 4, 0}, {0, 4, 1}, {10000, 1, 1}, {-1, 4, 1}} {
		if d.Valid() {
			t.Errorf("%+v reports Valid", d)
		}
		if ty := TaxYearOf(d); !ty.IsZero() {
			t.Errorf("TaxYearOf(%+v) = %s; want the zero TaxYear", d, ty)
		}
		if p := MonthOf(d); !p.IsZero() {
			t.Errorf("MonthOf(%+v) = %s; want the zero Period", d, p)
		}
	}
	for _, d := range []Date{{2024, 2, 29}, {1, 4, 1}, {9999, 3, 31}} {
		if !d.Valid() || TaxYearOf(d).IsZero() || MonthOf(d).IsZero() {
			t.Errorf("%+v: Valid %v, TaxYearOf %s, MonthOf %s", d, d.Valid(), TaxYearOf(d), MonthOf(d))
		}
	}
	// Dates outside every valid tax year are valid days but belong to no tax year.
	for _, d := range []Date{{1, 3, 31}, {9999, 4, 1}} {
		if !d.Valid() || !TaxYearOf(d).IsZero() || !MonthOf(d).IsZero() {
			t.Errorf("%+v: TaxYearOf %s, MonthOf %s; want zero values", d, TaxYearOf(d), MonthOf(d))
		}
	}
}

func TestMonthRejectsOutOfRangeYears(t *testing.T) {
	for _, c := range [][2]int{{0, 4}, {-5, 6}, {1, 3}, {9999, 4}, {10000, 1}} {
		if p, err := Month(c[0], c[1]); err == nil {
			t.Errorf("Month(%d, %d) = %s; want an error", c[0], c[1], p)
		}
	}
}

func TestParseBareTaxYearSuggestsPrefix(t *testing.T) {
	_, err := Parse("2025-26")
	if err == nil || !strings.Contains(err.Error(), "FY2025-26") {
		t.Errorf("Parse(\"2025-26\") error = %v; want a hint to write FY2025-26", err)
	}
	// "2011-12" is also December 2011, so it must still parse as a month.
	if p, err := Parse("2011-12"); err != nil || p.Kind() != KindMonth {
		t.Errorf("Parse(\"2011-12\") = %v, %v; want the month", p, err)
	}
}
