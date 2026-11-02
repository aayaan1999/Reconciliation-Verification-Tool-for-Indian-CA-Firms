package xlsx

import (
	"fmt"
	"strings"
)

// maxExponent bounds the exponent of a stored number, so "1E999999" cannot become a
// million digits. Excel itself keeps numbers between 1E-308 and 1E308.
const maxExponent = 400

// plainDecimal returns the number written as plain decimal text, the way JSON and
// internal/money read it: "1.2345E5" is "123450", "-1.5e-3" is "-0.0015", "007" is "7".
// It works on the digits only, never through floating point. ok is false when raw is not
// a number.
func plainDecimal(raw string) (string, bool) {
	s := raw
	neg := false
	switch {
	case strings.HasPrefix(s, "-"):
		neg, s = true, s[1:]
	case strings.HasPrefix(s, "+"):
		s = s[1:]
	}
	mant, exp := s, 0
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		mant = s[:i]
		e := s[i+1:]
		eneg := false
		switch {
		case strings.HasPrefix(e, "-"):
			eneg, e = true, e[1:]
		case strings.HasPrefix(e, "+"):
			e = e[1:]
		}
		if e == "" || len(e) > 4 {
			return "", false
		}
		for _, c := range e {
			if c < '0' || c > '9' {
				return "", false
			}
			exp = exp*10 + int(c-'0')
		}
		if exp > maxExponent {
			return "", false
		}
		if eneg {
			exp = -exp
		}
	}
	intPart, frac, _ := strings.Cut(mant, ".")
	digits := intPart + frac
	if digits == "" {
		return "", false
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return "", false
		}
	}
	point := len(intPart) + exp // the decimal point sits after this many digits
	switch {
	case point < 0:
		digits = strings.Repeat("0", -point) + digits
		point = 0
	case point > len(digits):
		digits += strings.Repeat("0", point-len(digits))
	}
	whole := strings.TrimLeft(digits[:point], "0")
	if whole == "" {
		whole = "0"
	}
	fraction := strings.TrimRight(digits[point:], "0")
	out := whole
	if fraction != "" {
		out += "." + fraction
	}
	if neg && out != "0" {
		out = "-" + out
	}
	return out, true
}

// serialDate turns a whole-day date serial number into YYYY-MM-DD, in the workbook's date
// system: from 1900 (Excel's default, which counts a 29 February 1900 that never was) or
// from 1904. ok is false for a serial with a time of day, a negative one, or serial 60 of
// the 1900 system (the day that does not exist).
func serialDate(num string, date1904 bool) (string, bool) {
	if strings.ContainsAny(num, ".-") || len(num) > 7 {
		return "", false
	}
	n := 0
	for _, c := range num {
		n = n*10 + int(c-'0')
	}
	var base int64 // days from 1970-01-01 of the day before serial 1, or of serial 0 in 1904
	switch {
	case date1904:
		base = daysFromCivil(1904, 1, 1)
	case n == 0 || n == 60:
		return "", false
	case n < 60:
		base = daysFromCivil(1899, 12, 31)
	default:
		base = daysFromCivil(1899, 12, 30)
	}
	y, m, d := civilFromDays(base + int64(n))
	return fmt.Sprintf("%04d-%02d-%02d", y, m, d), true
}

// daysFromCivil and civilFromDays convert between a proleptic Gregorian date and days from
// 1970-01-01, in integers only (H. Hinnant's algorithms).
func daysFromCivil(y, m, d int64) int64 {
	if m <= 2 {
		y--
	}
	era := y / 400
	if y < 0 && y%400 != 0 {
		era--
	}
	yoe := y - era*400
	mp := (m + 9) % 12
	doy := (153*mp+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}

func civilFromDays(z int64) (y, m, d int64) {
	z += 719468
	era := z / 146097
	if z < 0 && z%146097 != 0 {
		era--
	}
	doe := z - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	y = yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	d = doy - (153*mp+2)/5 + 1
	m = mp + 3
	if m > 12 {
		m -= 12
	}
	if m <= 2 {
		y++
	}
	return y, m, d
}

// validISODate reports whether s is a real date written YYYY-MM-DD.
func validISODate(s string) bool {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return false
	}
	var parts [3]int64
	for i, f := range []string{s[:4], s[5:7], s[8:]} {
		for _, c := range f {
			if c < '0' || c > '9' {
				return false
			}
			parts[i] = parts[i]*10 + int64(c-'0')
		}
	}
	y, m, d := civilFromDays(daysFromCivil(parts[0], parts[1], parts[2]))
	return y == parts[0] && m == parts[1] && d == parts[2]
}

// builtInDateFormats are Excel's built-in number formats that show a date (with or
// without a time): 14 to 17 and 22, and the East Asian date formats 27 to 36 and 50 to 58.
// The time-only formats (18 to 21, 45 to 47) are not dates.
var builtInDateFormats = map[int]bool{14: true, 15: true, 16: true, 17: true, 22: true,
	27: true, 28: true, 29: true, 30: true, 31: true, 32: true, 33: true, 34: true, 35: true, 36: true,
	50: true, 51: true, 52: true, 53: true, 54: true, 55: true, 56: true, 57: true, 58: true}

// isDateFormat reports whether a number format shows a date: a built-in date format, or a
// custom format with a day or year code (d or y) outside quoted text, [brackets] and
// escaped characters. A format with only months or minutes (m) is not taken as a date,
// since m means both.
func isDateFormat(numFmt int, custom string) bool {
	if builtInDateFormats[numFmt] {
		return true
	}
	quoted, bracket := false, false
	for i := 0; i < len(custom); i++ {
		c := custom[i]
		switch {
		case quoted:
			quoted = c != '"'
		case bracket:
			bracket = c != ']'
		case c == '"':
			quoted = true
		case c == '[':
			bracket = true
		case c == '\\' || c == '_' || c == '*':
			i++ // the next character is shown as it is, or is padding
		case c == 'd' || c == 'D' || c == 'y' || c == 'Y':
			return true
		}
	}
	return false
}
