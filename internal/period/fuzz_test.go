package period

import (
	"errors"
	"testing"
)

func FuzzParseDate(f *testing.F) {
	for _, s := range []string{"2025-07-31", "2024-02-29", "2025-02-29", "", "0000-00-00", "9999-99-99", "2025-1-1", "9999-04-01", "0001-03-31"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		d, err := ParseDate(s)
		if err != nil {
			var pe *Error
			if !errors.As(err, &pe) {
				t.Fatalf("error %T not *Error", err)
			}
			return
		}
		if d.String() != s {
			t.Fatalf("ParseDate(%q).String() = %q", s, d.String())
		}
		if d.Month < 1 || d.Month > 12 || d.Day < 1 || d.Day > daysIn(d.Year, d.Month) {
			t.Fatalf("accepted impossible date %+v", d)
		}
		ty := TaxYearOf(d)
		if ty.IsZero() {
			// Only dates before 0001-04-01 or after 9999-03-31 belong to no valid tax year.
			if d.Before(Date{MinYear, 4, 1}) || (d.Year == MaxYear && d.Month >= 4) {
				return
			}
			t.Fatalf("%s has no tax year", d)
		}
		if d.Before(ty.FirstDay()) || ty.LastDay().Before(d) {
			t.Fatalf("%s outside its tax year %s", d, ty)
		}
	})
}

func FuzzParseTaxYear(f *testing.F) {
	for _, s := range []string{"2025-26", "1999-00", "2025-27", "", "2025-2026", "9999-00"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		ty, err := ParseTaxYear(s)
		if err != nil {
			return
		}
		if ty.String() != s {
			t.Fatalf("ParseTaxYear(%q).String() = %q", s, ty.String())
		}
		if ty.Prior().StartYear != ty.StartYear-1 {
			t.Fatal("Prior wrong")
		}
		if len(MonthsOf(ty)) != 12 || len(QuartersOf(ty)) != 4 {
			t.Fatal("wrong month or quarter count")
		}
	})
}

func FuzzParsePeriod(f *testing.F) {
	for _, s := range []string{"FY2025-26", "FY2025-26:Q1", "2025-04", "FY2025-26:Q5", "", "2025-13", "FY2025-26:Q1:Q2"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		p, err := Parse(s)
		if err != nil {
			var pe *Error
			if !errors.As(err, &pe) {
				t.Fatalf("error %T not *Error", err)
			}
			return
		}
		if p.String() != s {
			t.Fatalf("Parse(%q).String() = %q", s, p.String())
		}
		again, err := Parse(p.String())
		if err != nil || again != p {
			t.Fatalf("not stable: %v %v", again, err)
		}
		if !Contains(p, p) || p.LastDay().Before(p.FirstDay()) {
			t.Fatalf("%s has an empty or inverted span", p)
		}
		if !Contains(Year(p.TaxYear()), p) {
			t.Fatalf("%s not inside its tax year", p)
		}
	})
}
