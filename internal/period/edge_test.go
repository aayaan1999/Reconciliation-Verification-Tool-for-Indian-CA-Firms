package period

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestParseTaxYearEdges(t *testing.T) {
	good := map[string]int{"2025-26": 2025, "1999-00": 1999, "2099-00": 2099, "2000-01": 2000, "0001-02": 1, "9998-99": 9998}
	for in, start := range good {
		ty, err := ParseTaxYear(in)
		if err != nil || ty.StartYear != start || ty.String() != in {
			t.Errorf("ParseTaxYear(%q) = %+v,%v (String %q)", in, ty, err, ty.String())
		}
	}
	bad := []string{"", "2025", "2025-", "-26", "2025-27", "2025-25", "2025-2026", "25-26", "2025/26", "2025_26",
		" 2025-26", "2025-26 ", "2025-2", "2025--26", "2025-26-27", "+025-26", "2025-+6", "2O25-26", "2025–26",
		"FY2025-26", "2025-26:Q1", "２０２５-26", "2025-00", "2025-1a", "0000-01", "9999-00"}
	for _, in := range bad {
		ty, err := ParseTaxYear(in)
		var pe *Error
		if err == nil || !errors.As(err, &pe) || pe.Reason == "" {
			t.Errorf("ParseTaxYear(%q) = %+v,%v; want *Error", in, ty, err)
		}
	}
}

func TestParseDateEdges(t *testing.T) {
	good := []string{"2024-02-29", "2000-02-29", "2025-12-31", "2025-01-01", "2026-03-31", "2026-04-01", "0001-01-01", "9999-12-31"}
	for _, in := range good {
		d, err := ParseDate(in)
		if err != nil || d.String() != in {
			t.Errorf("ParseDate(%q) = %v,%v", in, d, err)
		}
	}
	bad := []string{"", "2025-02-29", "2100-02-29", "1900-02-29", "2023-02-29", "2025-04-31", "2025-06-31", "2025-09-31", "2025-11-31",
		"2025-00-10", "2025-13-01", "2025-01-00", "2025-01-32", "2025-1-01", "2025-01-1", "25-01-01", "20250101",
		"2025/01/01", "2025-01-01T00:00:00", "2025-01-01 ", " 2025-01-01", "2025-01", "2025-01-01-01", "+025-01-01",
		"2025-0a-01", "01-01-2025", "1-1-2025", "2025-01-01\n", "２０２５-01-01"}
	for _, in := range bad {
		d, err := ParseDate(in)
		var pe *Error
		if err == nil || !errors.As(err, &pe) {
			t.Errorf("ParseDate(%q) = %v,%v; want *Error", in, d, err)
		}
	}
}

func TestLeapYearRules(t *testing.T) {
	cases := map[int]bool{1900: false, 2000: true, 2023: false, 2024: true, 2100: false, 2400: true, 2025: false, 2028: true}
	for y, want := range cases {
		if got := isLeap(y); got != want {
			t.Errorf("isLeap(%d) = %v, want %v", y, got, want)
		}
	}
	// Leap day sits in Q4 of the tax year that started the previous April.
	d := mustDate(t, "2024-02-29")
	if got := TaxYearOf(d).String(); got != "2023-24" {
		t.Errorf("TaxYearOf(2024-02-29) = %s, want 2023-24", got)
	}
	m := MonthOf(d)
	if m.String() != "2024-02" || m.LastDay().String() != "2024-02-29" {
		t.Errorf("Feb 2024 = %s last day %s", m, m.LastDay())
	}
	if got := MonthOf(mustDate(t, "2025-02-28")).LastDay().String(); got != "2025-02-28" {
		t.Errorf("Feb 2025 last day = %s", got)
	}
	q, err := QuarterOf(m)
	if err != nil || q.String() != "FY2023-24:Q4" || q.LastDay().String() != "2024-03-31" {
		t.Errorf("QuarterOf(2024-02) = %v,%v", q, err)
	}
	ends := []string{"2023-06-30", "2023-09-30", "2023-12-31", "2024-03-31"}
	starts := []string{"2023-04-01", "2023-07-01", "2023-10-01", "2024-01-01"}
	for i, qq := range QuartersOf(TaxYear{2023}) {
		if qq.FirstDay().String() != starts[i] || qq.LastDay().String() != ends[i] {
			t.Errorf("%s = %s..%s, want %s..%s", qq, qq.FirstDay(), qq.LastDay(), starts[i], ends[i])
		}
	}
}

func nextDay(d Date) Date {
	if d.Day < daysIn(d.Year, d.Month) {
		return Date{d.Year, d.Month, d.Day + 1}
	}
	if d.Month < 12 {
		return Date{d.Year, d.Month + 1, 1}
	}
	return Date{d.Year + 1, 1, 1}
}

// Every day of eleven years maps to one tax year, month and quarter, consistently.
func TestEveryDayConsistent(t *testing.T) {
	d, end, count := mustDate(t, "2019-01-01"), mustDate(t, "2030-01-01"), 0
	var prev Date
	for d.Before(end) {
		ty := TaxYearOf(d)
		if d.Before(ty.FirstDay()) || ty.LastDay().Before(d) {
			t.Fatalf("%s not within %s (%s..%s)", d, ty, ty.FirstDay(), ty.LastDay())
		}
		mp := MonthOf(d)
		if mp.TaxYear() != ty {
			t.Fatalf("%s: month %s tax year %s != %s", d, mp, mp.TaxYear(), ty)
		}
		if !Contains(Year(ty), mp) || !Contains(mp, mp) {
			t.Fatalf("%s not inside %s", mp, Year(ty))
		}
		q, err := QuarterOf(mp)
		if err != nil || !Contains(q, mp) || !Contains(Year(ty), q) {
			t.Fatalf("%s: quarter %v err %v", d, q, err)
		}
		if d.Before(mp.FirstDay()) || mp.LastDay().Before(d) {
			t.Fatalf("%s outside its month %s", d, mp)
		}
		if count > 0 {
			if !prev.Before(d) {
				t.Fatalf("dates not increasing at %s", d)
			}
			if d.Day == 1 && MonthOf(prev).LastDay() != prev {
				t.Fatalf("day before %s is %s but last day of its month is %s", d, prev, MonthOf(prev).LastDay())
			}
		}
		prev = d
		count++
		d = nextDay(d)
	}
	if want := 11*365 + 3; count != want { // leap years 2020, 2024, 2028
		t.Errorf("walked %d days, want %d", count, want)
	}
}

func TestTaxYearBoundariesAndPrior(t *testing.T) {
	for _, c := range []struct{ date, ty string }{
		{"2026-03-31", "2025-26"}, {"2026-04-01", "2026-27"}, {"2026-01-01", "2025-26"}, {"2025-12-31", "2025-26"},
		{"2025-04-01", "2025-26"}, {"2025-03-31", "2024-25"}, {"2000-03-31", "1999-00"}, {"2000-04-01", "2000-01"},
		{"2099-12-31", "2099-00"}, {"2100-01-01", "2099-00"},
	} {
		if got := TaxYearOf(mustDate(t, c.date)).String(); got != c.ty {
			t.Errorf("TaxYearOf(%s) = %s, want %s", c.date, got, c.ty)
		}
	}
	ty, _ := ParseTaxYear("2000-01")
	if ty.Prior().String() != "1999-00" || ty.Prior().Prior().String() != "1998-99" {
		t.Errorf("Prior chain wrong: %s %s", ty.Prior(), ty.Prior().Prior())
	}
	if ty.FirstDay().String() != "2000-04-01" || ty.LastDay().String() != "2001-03-31" {
		t.Errorf("2000-01 spans %s..%s", ty.FirstDay(), ty.LastDay())
	}
	// 2099-00 ends in 2100, which is not a leap year.
	ty2, _ := ParseTaxYear("2099-00")
	if ty2.LastDay().String() != "2100-03-31" {
		t.Errorf("2099-00 last day = %s", ty2.LastDay())
	}
	if p := MonthsOf(ty2); p[10].LastDay().String() != "2100-02-28" {
		t.Errorf("Feb 2100 last day = %s, want 2100-02-28", p[10].LastDay())
	}
}

func TestParsePeriodEdges(t *testing.T) {
	good := []string{"FY2025-26", "FY2025-26:Q1", "FY2025-26:Q4", "2025-04", "2026-03", "2025-12", "2026-01", "FY1999-00:Q2", "0001-04", "9999-03", "2011-12"}
	for _, in := range good {
		p, err := Parse(in)
		if err != nil || p.String() != in {
			t.Errorf("Parse(%q) = %v,%v", in, p, err)
		}
	}
	bad := []string{"", "FY", "FY2025", "FY2025-27", "FY2025-26:", "FY2025-26:Q", "FY2025-26:Q0", "FY2025-26:Q5", "FY2025-26:q1",
		"FY2025-26:Q1:x", "FY2025-26:Q12", "FY2025-26: Q1", "fy2025-26", "FY 2025-26", " FY2025-26", "FY2025-26 ", "2025-26",
		"2025-00", "2025-13", "2025-4", "25-04", "2025-04-01", "2025/04", "Q1", "FY2025-26:Q1\n", "FYFY2025-26", "2025-0x",
		"FY2025-26;Q1", "FY2025-26:１"}
	for _, in := range bad {
		p, err := Parse(in)
		var pe *Error
		if err == nil || !errors.As(err, &pe) || pe.Reason == "" {
			t.Errorf("Parse(%q) = %v,%v; want *Error", in, p, err)
		}
	}
}

func TestConstructorsRejectInvalid(t *testing.T) {
	for _, q := range []int{-1, 0, 5, 100} {
		if _, err := Quarter(TaxYear{2025}, q); err == nil {
			t.Errorf("Quarter(%d) accepted", q)
		}
	}
	for _, m := range []int{-1, 0, 13, 99} {
		if _, err := Month(2025, m); err == nil {
			t.Errorf("Month(2025,%d) accepted", m)
		}
	}
	if _, err := QuarterOf(Year(TaxYear{2025})); err == nil {
		t.Error("QuarterOf(year) accepted")
	}
	q, _ := Quarter(TaxYear{2025}, 1)
	if _, err := QuarterOf(q); err == nil {
		t.Error("QuarterOf(quarter) accepted")
	}
}

func TestMonthToQuarterMapping(t *testing.T) {
	want := map[int]int{4: 1, 5: 1, 6: 1, 7: 2, 8: 2, 9: 2, 10: 3, 11: 3, 12: 3, 1: 4, 2: 4, 3: 4}
	for _, m := range MonthsOf(TaxYear{2025}) {
		q, err := QuarterOf(m)
		if err != nil {
			t.Fatal(err)
		}
		wantQ := want[m.FirstDay().Month]
		if !strings.HasSuffix(q.String(), ":Q"+strconv.Itoa(wantQ)) || q.TaxYear().StartYear != 2025 {
			t.Errorf("%s -> %s, want Q%d of 2025-26", m, q, wantQ)
		}
	}
}

func TestContainsEdges(t *testing.T) {
	for _, c := range []struct {
		outer, inner string
		want         bool
	}{
		{"FY2025-26", "FY2025-26", true},
		{"FY2025-26", "FY2025-26:Q1", true},
		{"FY2025-26", "2025-04", true},
		{"FY2025-26", "2026-03", true},
		{"FY2025-26", "2026-04", false},
		{"FY2025-26", "2025-03", false},
		{"FY2025-26", "FY2026-27", false},
		{"FY2026-27", "FY2025-26", false},
		{"FY2025-26:Q1", "2025-06", true},
		{"FY2025-26:Q1", "2025-07", false},
		{"FY2025-26:Q4", "2026-01", true},
		{"FY2025-26:Q4", "2025-12", false},
		{"2025-04", "FY2025-26", false},
		{"2025-04", "2025-04", true},
		{"FY2025-26:Q1", "FY2025-26", false},
		{"FY2025-26:Q1", "FY2025-26:Q2", false},
	} {
		if got := Contains(mustPeriod(t, c.outer), mustPeriod(t, c.inner)); got != c.want {
			t.Errorf("Contains(%s,%s) = %v, want %v", c.outer, c.inner, got, c.want)
		}
	}
}

func TestDateOrdering(t *testing.T) {
	dates := []string{"2025-03-31", "2025-04-01", "2025-04-02", "2025-12-31", "2026-01-01", "2026-03-31", "2026-04-01"}
	for i := range dates {
		for j := range dates {
			a, b := mustDate(t, dates[i]), mustDate(t, dates[j])
			if a.Before(b) != (i < j) {
				t.Errorf("%s.Before(%s) = %v", a, b, a.Before(b))
			}
		}
	}
}

func TestPeriodsAreComparableAndCanonical(t *testing.T) {
	a, b := mustPeriod(t, "FY2025-26:Q1"), mustPeriod(t, "FY2025-26:Q1")
	q, _ := Quarter(TaxYear{2025}, 1)
	if a != b || a != q {
		t.Error("equal periods compare unequal")
	}
	m1, _ := Month(2025, 4)
	if m1 != mustPeriod(t, "2025-04") {
		t.Error("Month and Parse disagree")
	}
	if a == Year(TaxYear{2025}) {
		t.Error("quarter equals year")
	}
	if got := (Period{}).String(); got != "" {
		t.Errorf("zero Period String = %q", got)
	}
}

func TestMonthsAndQuartersContiguous(t *testing.T) {
	ms := MonthsOf(TaxYear{2025})
	if len(ms) != 12 || ms[0].String() != "2025-04" || ms[8].String() != "2025-12" || ms[9].String() != "2026-01" || ms[11].String() != "2026-03" {
		t.Fatalf("months = %v", ms)
	}
	for i := 1; i < len(ms); i++ {
		if nextDay(ms[i-1].LastDay()) != ms[i].FirstDay() {
			t.Errorf("%s and %s are not contiguous", ms[i-1], ms[i])
		}
	}
	qs := QuartersOf(TaxYear{2025})
	for i := 1; i < len(qs); i++ {
		if nextDay(qs[i-1].LastDay()) != qs[i].FirstDay() {
			t.Errorf("%s and %s are not contiguous", qs[i-1], qs[i])
		}
	}
	a, b := MonthsOf(TaxYear{2025}), MonthsOf(TaxYear{2025})
	a[0] = ms[11]
	if b[0] != ms[0] {
		t.Error("MonthsOf shares storage between calls")
	}
}

func TestErrorWording(t *testing.T) {
	for _, in := range []string{"", "x", "2025-27", "FY2025-26:Q9", "2025-02-30"} {
		var msgs []string
		if _, err := ParseTaxYear(in); err != nil {
			msgs = append(msgs, err.Error())
		}
		if _, err := Parse(in); err != nil {
			msgs = append(msgs, err.Error())
		}
		if _, err := ParseDate(in); err != nil {
			msgs = append(msgs, err.Error())
		}
		for _, m := range msgs {
			for _, bad := range []string{"should", "incorrect", "must be"} {
				if strings.Contains(strings.ToLower(m), bad) {
					t.Errorf("message %q contains %q", m, bad)
				}
			}
		}
	}
}
