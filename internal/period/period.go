package period

import (
	"fmt"
	"strconv"
	"strings"
)

// Error explains why a text is not a valid tax year, period or date.
type Error struct {
	Input  string
	Reason string
}

func (e *Error) Error() string {
	return fmt.Sprintf("period: %q is not valid: %s", e.Input, e.Reason)
}

func invalid(input, reason string) *Error { return &Error{Input: input, Reason: reason} }

// --- dates ---

// Date is a calendar day. It never comes from the system clock. Its fields are
// exported, so a hand-built Date may be invalid; check it with Valid. The zero Date
// is not valid.
type Date struct{ Year, Month, Day int }

// MinYear and MaxYear bound the calendar years of a valid Date (four digits, and
// never year 0, so that the zero Date and zero TaxYear are never valid values).
// A valid TaxYear starts between MinYear and MaxYear-1, so it ends within MaxYear.
const (
	MinYear = 1
	MaxYear = 9999
)

// Valid reports whether d is a real calendar day between MinYear and MaxYear.
func (d Date) Valid() bool {
	return d.Year >= MinYear && d.Year <= MaxYear && d.Month >= 1 && d.Month <= 12 &&
		d.Day >= 1 && d.Day <= daysIn(d.Year, d.Month)
}

// IsZero reports whether d is the zero Date.
func (d Date) IsZero() bool { return d == Date{} }

// ParseDate reads an ISO date "2025-07-31".
func ParseDate(s string) (Date, error) {
	parts := strings.Split(s, "-")
	if len(parts) != 3 || len(parts[0]) != 4 || len(parts[1]) != 2 || len(parts[2]) != 2 {
		return Date{}, invalid(s, "a date is written YYYY-MM-DD")
	}
	y, errY := atoi(parts[0])
	m, errM := atoi(parts[1])
	d, errD := atoi(parts[2])
	if errY != nil || errM != nil || errD != nil {
		return Date{}, invalid(s, "a date is written YYYY-MM-DD")
	}
	if y < MinYear || y > MaxYear {
		return Date{}, invalid(s, fmt.Sprintf("year is outside %04d to %04d", MinYear, MaxYear))
	}
	if m < 1 || m > 12 {
		return Date{}, invalid(s, "month is outside 01 to 12")
	}
	if d < 1 || d > daysIn(y, m) {
		return Date{}, invalid(s, fmt.Sprintf("day is outside 01 to %02d for that month", daysIn(y, m)))
	}
	return Date{y, m, d}, nil
}

func (d Date) String() string { return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day) }

// Before reports whether d is earlier than e.
func (d Date) Before(e Date) bool {
	if d.Year != e.Year {
		return d.Year < e.Year
	}
	if d.Month != e.Month {
		return d.Month < e.Month
	}
	return d.Day < e.Day
}

func isLeap(y int) bool { return y%4 == 0 && (y%100 != 0 || y%400 == 0) }

func daysIn(y, m int) int {
	switch m {
	case 2:
		if isLeap(y) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	}
	return 31
}

// --- tax years ---

// TaxYear is an Indian financial year, 1 April to 31 March, written "2025-26".
// StartYear is the calendar year in which it starts (2025 for "2025-26").
// The zero TaxYear is not valid; check a hand-built one with Valid.
type TaxYear struct{ StartYear int }

// Valid reports whether ty starts between MinYear and MaxYear-1.
func (ty TaxYear) Valid() bool { return ty.StartYear >= MinYear && ty.StartYear < MaxYear }

// IsZero reports whether ty is the zero TaxYear.
func (ty TaxYear) IsZero() bool { return ty == TaxYear{} }

// ParseTaxYear reads "2025-26".
func ParseTaxYear(s string) (TaxYear, error) {
	first, second, ok := strings.Cut(s, "-")
	if !ok || len(first) != 4 || len(second) != 2 {
		return TaxYear{}, invalid(s, "a tax year is written YYYY-YY, for example 2025-26")
	}
	start, err1 := atoi(first)
	end, err2 := atoi(second)
	if err1 != nil || err2 != nil {
		return TaxYear{}, invalid(s, "a tax year is written YYYY-YY, for example 2025-26")
	}
	if end != (start+1)%100 {
		return TaxYear{}, invalid(s, "the second year must follow the first")
	}
	if !(TaxYear{start}).Valid() {
		return TaxYear{}, invalid(s, fmt.Sprintf("the first year is outside %04d to %04d", MinYear, MaxYear-1))
	}
	return TaxYear{start}, nil
}

func (ty TaxYear) String() string {
	return fmt.Sprintf("%04d-%02d", ty.StartYear, (ty.StartYear+1)%100)
}

// Prior returns the previous tax year.
func (ty TaxYear) Prior() TaxYear { return TaxYear{ty.StartYear - 1} }

// FirstDay is 1 April of the start year; LastDay is 31 March of the next year.
func (ty TaxYear) FirstDay() Date { return Date{ty.StartYear, 4, 1} }
func (ty TaxYear) LastDay() Date  { return Date{ty.StartYear + 1, 3, 31} }

// TaxYearOf returns the tax year a date falls in: 31 March 2026 is in 2025-26,
// 1 April 2026 is in 2026-27. An invalid date gives the zero TaxYear.
func TaxYearOf(d Date) TaxYear {
	if !d.Valid() {
		return TaxYear{}
	}
	ty := TaxYear{d.Year}
	if d.Month < 4 {
		ty = TaxYear{d.Year - 1}
	}
	if !ty.Valid() {
		return TaxYear{}
	}
	return ty
}

// --- periods ---

// Kind says whether a Period is a whole financial year, a quarter or a month.
type Kind int

const (
	// KindYear is a whole financial year, 1 April to 31 March.
	KindYear Kind = iota + 1
	// KindQuarter is a quarter of a financial year; Q1 is April to June.
	KindQuarter
	// KindMonth is a calendar month.
	KindMonth
)

// Period is a financial year ("FY2025-26"), a quarter ("FY2025-26:Q1", Q1 = April to
// June) or a calendar month ("2025-04"). Its String form is the canonical spelling
// used everywhere in Tool 1. The zero Period is not a period: IsZero reports it,
// its String is "", it has no first or last day, and it contains nothing.
type Period struct {
	kind    Kind
	taxYear TaxYear
	quarter int // 1-4 for KindQuarter
	year    int // calendar year, for KindMonth
	month   int // 1-12, for KindMonth
}

// IsZero reports whether p is the zero Period, which is not a valid period.
func (p Period) IsZero() bool { return p.kind == 0 }

// Year returns the period covering the whole tax year, or the zero Period if ty is
// not valid.
func Year(ty TaxYear) Period {
	if !ty.Valid() {
		return Period{}
	}
	return Period{kind: KindYear, taxYear: ty}
}

// Quarter returns quarter q (1-4) of the tax year.
func Quarter(ty TaxYear, q int) (Period, error) {
	if !ty.Valid() {
		return Period{}, invalid(ty.String(), "not a valid tax year")
	}
	if q < 1 || q > 4 {
		return Period{}, invalid(fmt.Sprintf("Q%d", q), "quarter is not Q1 to Q4")
	}
	return Period{kind: KindQuarter, taxYear: ty, quarter: q}, nil
}

// Month returns the calendar month year-month.
func Month(year, month int) (Period, error) {
	if month < 1 || month > 12 {
		return Period{}, invalid(fmt.Sprintf("%04d-%02d", year, month), "month is outside 01 to 12")
	}
	ty := TaxYearOf(Date{year, month, 1})
	if !ty.Valid() {
		return Period{}, invalid(fmt.Sprintf("%04d-%02d", year, month), "the month falls outside every valid tax year")
	}
	return Period{kind: KindMonth, taxYear: ty, year: year, month: month}, nil
}

// Parse reads a period in its canonical spelling.
func Parse(s string) (Period, error) {
	if rest, ok := strings.CutPrefix(s, "FY"); ok {
		tyText, q, hasQuarter := strings.Cut(rest, ":")
		ty, err := ParseTaxYear(tyText)
		if err != nil {
			return Period{}, invalid(s, "a year period is written FY2025-26")
		}
		if !hasQuarter {
			return Year(ty), nil
		}
		if len(q) != 2 || q[0] != 'Q' || q[1] < '1' || q[1] > '4' {
			return Period{}, invalid(s, "quarter is not Q1 to Q4")
		}
		return Period{kind: KindQuarter, taxYear: ty, quarter: int(q[1] - '0')}, nil
	}
	parts := strings.Split(s, "-")
	if len(parts) == 2 && len(parts[0]) == 4 && len(parts[1]) == 2 {
		y, err1 := atoi(parts[0])
		m, err2 := atoi(parts[1])
		if err1 == nil && err2 == nil {
			if m < 1 || m > 12 {
				if _, err := ParseTaxYear(s); err == nil {
					return Period{}, invalid(s, "a year period needs the FY prefix: FY"+s)
				}
				return Period{}, invalid(s, "month is outside 01 to 12")
			}
			return Month(y, m)
		}
	}
	return Period{}, invalid(s, "a period is written FY2025-26, FY2025-26:Q1 or 2025-04")
}

func (p Period) String() string {
	switch p.kind {
	case KindYear:
		return "FY" + p.taxYear.String()
	case KindQuarter:
		return fmt.Sprintf("FY%s:Q%d", p.taxYear, p.quarter)
	case KindMonth:
		return fmt.Sprintf("%04d-%02d", p.year, p.month)
	}
	return ""
}

// Kind reports whether p is a year, quarter or month.
func (p Period) Kind() Kind { return p.kind }

// TaxYear returns the tax year p belongs to.
func (p Period) TaxYear() TaxYear { return p.taxYear }

// FirstDay and LastDay return the first and last day of the period, or the zero
// Date for the zero Period.
func (p Period) FirstDay() Date {
	switch p.kind {
	case 0:
		return Date{}
	case KindQuarter:
		y, m := quarterStart(p.taxYear, p.quarter)
		return Date{y, m, 1}
	case KindMonth:
		return Date{p.year, p.month, 1}
	}
	return p.taxYear.FirstDay()
}

func (p Period) LastDay() Date {
	switch p.kind {
	case 0:
		return Date{}
	case KindQuarter:
		y, m := quarterStart(p.taxYear, p.quarter)
		m += 2
		return Date{y, m, daysIn(y, m)}
	case KindMonth:
		return Date{p.year, p.month, daysIn(p.year, p.month)}
	}
	return p.taxYear.LastDay()
}

// quarterStart returns the calendar year and month in which quarter q of ty starts.
func quarterStart(ty TaxYear, q int) (int, int) {
	if q == 4 {
		return ty.StartYear + 1, 1
	}
	return ty.StartYear, 4 + 3*(q-1)
}

// Contains reports whether inner lies entirely within outer. It is false if either
// is the zero Period.
func Contains(outer, inner Period) bool {
	if outer.IsZero() || inner.IsZero() {
		return false
	}
	return !inner.FirstDay().Before(outer.FirstDay()) && !outer.LastDay().Before(inner.LastDay())
}

// MonthsOf returns the 12 months of a tax year in order, April to March.
func MonthsOf(ty TaxYear) []Period {
	months := make([]Period, 0, 12)
	for i := 0; i < 12; i++ {
		y, m := ty.StartYear, 4+i
		if m > 12 {
			y, m = y+1, m-12
		}
		p, _ := Month(y, m)
		months = append(months, p)
	}
	return months
}

// QuartersOf returns Q1 to Q4 of a tax year.
func QuartersOf(ty TaxYear) []Period {
	quarters := make([]Period, 0, 4)
	for q := 1; q <= 4; q++ {
		p, _ := Quarter(ty, q)
		quarters = append(quarters, p)
	}
	return quarters
}

// QuarterOf returns the quarter containing a month period. It is an error for a
// period that is not a month.
func QuarterOf(month Period) (Period, error) {
	if month.kind != KindMonth {
		return Period{}, invalid(month.String(), "only a month has a single quarter")
	}
	offset := (month.month + 12 - 4) % 12 // April = 0 … March = 11
	return Quarter(month.taxYear, offset/3+1)
}

// MonthOf returns the month period containing date d, or the zero Period if d is
// not valid.
func MonthOf(d Date) Period {
	if !d.Valid() {
		return Period{}
	}
	p, _ := Month(d.Year, d.Month)
	return p
}

// atoi accepts only plain ASCII digits (no signs or spaces).
func atoi(s string) (int, error) {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, fmt.Errorf("not digits")
		}
	}
	return strconv.Atoi(s)
}
