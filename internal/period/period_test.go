package period

import (
	"errors"
	"strings"
	"testing"
)

func mustDate(t *testing.T, s string) Date {
	t.Helper()
	d, err := ParseDate(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func mustPeriod(t *testing.T, s string) Period {
	t.Helper()
	p, err := Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTaxYearOfDateBoundary(t *testing.T) {
	cases := []struct{ date, want string }{
		{"2026-03-31", "2025-26"},
		{"2026-04-01", "2026-27"},
		{"2025-04-01", "2025-26"},
		{"2025-12-31", "2025-26"},
		{"2026-01-01", "2025-26"},
		{"1999-04-01", "1999-00"},
		{"2000-03-31", "1999-00"},
	}
	for _, c := range cases {
		if got := TaxYearOf(mustDate(t, c.date)).String(); got != c.want {
			t.Errorf("TaxYearOf(%s) = %s, want %s", c.date, got, c.want)
		}
	}
}

func TestParseTaxYear(t *testing.T) {
	for _, s := range []string{"2025-26", "2026-27", "1999-00", "2099-00"} {
		ty, err := ParseTaxYear(s)
		if err != nil || ty.String() != s {
			t.Errorf("ParseTaxYear(%q) = %v, %v", s, ty, err)
		}
	}
	for _, s := range []string{"2025-27", "2025-25", "2025-2026", "25-26", "2025/26", "", "2025-2", "abcd-26", "+025-26", "2025-26 "} {
		if _, err := ParseTaxYear(s); err == nil {
			t.Errorf("ParseTaxYear(%q) should fail", s)
		}
	}
	ty, _ := ParseTaxYear("2025-26")
	if ty.Prior().String() != "2024-25" {
		t.Errorf("Prior = %s", ty.Prior())
	}
	if ty.FirstDay().String() != "2025-04-01" || ty.LastDay().String() != "2026-03-31" {
		t.Errorf("FirstDay/LastDay = %s/%s", ty.FirstDay(), ty.LastDay())
	}
}

func TestParseDate(t *testing.T) {
	for _, s := range []string{"2025-07-31", "2028-02-29", "2000-02-29", "2026-02-28", "2025-12-31"} {
		if d, err := ParseDate(s); err != nil || d.String() != s {
			t.Errorf("ParseDate(%q) = %v, %v", s, d, err)
		}
	}
	bad := map[string]string{
		"2026-02-29": "day is outside 01 to 28",
		"1900-02-29": "day is outside 01 to 28",
		"2026-02-30": "day is outside 01 to 28",
		"2025-04-31": "day is outside 01 to 30",
		"2025-13-01": "month is outside 01 to 12",
		"2025-00-10": "month is outside 01 to 12",
		"2025-01-00": "day is outside 01 to 31",
		"2025-7-31":  "a date is written YYYY-MM-DD",
		"31-07-2025": "a date is written YYYY-MM-DD",
		"2025/07/31": "a date is written YYYY-MM-DD",
		"":           "a date is written YYYY-MM-DD",
	}
	for s, want := range bad {
		_, err := ParseDate(s)
		var pe *Error
		if !errors.As(err, &pe) || !strings.HasPrefix(pe.Reason, want) {
			t.Errorf("ParseDate(%q) error %v; want reason starting %q", s, err, want)
		}
	}
}

func TestParsePeriodRoundTrip(t *testing.T) {
	cases := []struct {
		s    string
		kind Kind
		ty   string
	}{
		{"FY2025-26", KindYear, "2025-26"},
		{"FY2025-26:Q1", KindQuarter, "2025-26"},
		{"FY2025-26:Q4", KindQuarter, "2025-26"},
		{"2025-04", KindMonth, "2025-26"},
		{"2026-03", KindMonth, "2025-26"},
		{"2026-04", KindMonth, "2026-27"},
	}
	for _, c := range cases {
		p := mustPeriod(t, c.s)
		if p.String() != c.s || p.Kind() != c.kind || p.TaxYear().String() != c.ty {
			t.Errorf("Parse(%q) = %s kind %d year %s", c.s, p, p.Kind(), p.TaxYear())
		}
	}
	for _, s := range []string{"FY2025-27", "FY2025-26:Q5", "FY2025-26:Q0", "FY2025-26:q1", "FY2025-26:", "2025-13", "2025-00", "2025-4", "fy2025-26", "2025-26", "Q1", "", "FY2025-26:Q1:Q2"} {
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) should fail", s)
		}
	}
}

func TestPeriodDays(t *testing.T) {
	cases := []struct{ p, first, last string }{
		{"FY2025-26", "2025-04-01", "2026-03-31"},
		{"FY2025-26:Q1", "2025-04-01", "2025-06-30"},
		{"FY2025-26:Q2", "2025-07-01", "2025-09-30"},
		{"FY2025-26:Q3", "2025-10-01", "2025-12-31"},
		{"FY2025-26:Q4", "2026-01-01", "2026-03-31"},
		{"2028-02", "2028-02-01", "2028-02-29"},
		{"2026-02", "2026-02-01", "2026-02-28"},
	}
	for _, c := range cases {
		p := mustPeriod(t, c.p)
		if p.FirstDay().String() != c.first || p.LastDay().String() != c.last {
			t.Errorf("%s: %s to %s; want %s to %s", c.p, p.FirstDay(), p.LastDay(), c.first, c.last)
		}
	}
}

func TestMonthsAndQuarters(t *testing.T) {
	ty, _ := ParseTaxYear("2025-26")
	months := MonthsOf(ty)
	var names []string
	for _, m := range months {
		names = append(names, m.String())
	}
	want := "2025-04 2025-05 2025-06 2025-07 2025-08 2025-09 2025-10 2025-11 2025-12 2026-01 2026-02 2026-03"
	if got := strings.Join(names, " "); got != want {
		t.Errorf("MonthsOf = %s", got)
	}
	quarters := QuartersOf(ty)
	if len(quarters) != 4 || quarters[0].String() != "FY2025-26:Q1" || quarters[3].String() != "FY2025-26:Q4" {
		t.Errorf("QuartersOf = %v", quarters)
	}
	wantQ := []int{1, 1, 1, 2, 2, 2, 3, 3, 3, 4, 4, 4}
	for i, m := range months {
		q, err := QuarterOf(m)
		if err != nil || q != quarters[wantQ[i]-1] {
			t.Errorf("QuarterOf(%s) = %s, %v; want Q%d", m, q, err, wantQ[i])
		}
	}
	if _, err := QuarterOf(Year(ty)); err == nil {
		t.Error("QuarterOf a year period should fail")
	}
	if _, err := Quarter(ty, 5); err == nil {
		t.Error("Quarter 5 should fail")
	}
	if _, err := Month(2025, 13); err == nil {
		t.Error("Month 13 should fail")
	}
}

func TestContains(t *testing.T) {
	cases := []struct {
		outer, inner string
		want         bool
	}{
		{"FY2025-26", "2025-04", true},
		{"FY2025-26", "2026-03", true},
		{"FY2025-26", "2026-04", false},
		{"FY2025-26", "2025-03", false},
		{"FY2025-26:Q2", "2025-08", true},
		{"FY2025-26:Q2", "2025-10", false},
		{"FY2025-26", "FY2025-26:Q4", true},
		{"FY2025-26:Q4", "FY2025-26", false},
		{"FY2025-26", "FY2025-26", true},
		{"2025-04", "2025-04", true},
		{"2025-04", "FY2025-26:Q1", false},
	}
	for _, c := range cases {
		if got := Contains(mustPeriod(t, c.outer), mustPeriod(t, c.inner)); got != c.want {
			t.Errorf("Contains(%s, %s) = %v, want %v", c.outer, c.inner, got, c.want)
		}
	}
}

func TestMonthOf(t *testing.T) {
	if got := MonthOf(mustDate(t, "2026-01-15")); got.String() != "2026-01" || got.TaxYear().String() != "2025-26" {
		t.Errorf("MonthOf = %s in %s", got, got.TaxYear())
	}
}

func TestDateBefore(t *testing.T) {
	a, b := mustDate(t, "2026-03-31"), mustDate(t, "2026-04-01")
	if !a.Before(b) || b.Before(a) || a.Before(a) {
		t.Error("Before ordering is wrong")
	}
}
