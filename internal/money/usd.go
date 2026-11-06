package money

import (
	"errors"
	"fmt"
	"strings"
)

// USDRate is a rate in rupees per US dollar, held as a whole number of 1/10,000 rupee
// (SPEC-019): 83.2500 rupees is USDRate(832500). It is never a float. The zero value is
// "no rate", and a rate is valid only when it is above zero.
type USDRate int64

// MaxUSDRateDecimals is the most decimal places a rate is written with.
const MaxUSDRateDecimals = 4

// ErrNoRate is returned by ConvertUSD when the rate is not above zero.
var ErrNoRate = errors.New("money: no valid dollar rate")

// ParseUSDRate reads a rate such as "83.2500" or "83": digits, up to four decimal places,
// above zero, no grouping and no sign. It never goes through floating point.
func ParseUSDRate(text string) (USDRate, error) {
	s := strings.TrimSpace(text)
	fail := func(reason string) (USDRate, error) { return 0, &ParseError{Input: text, Reason: reason} }
	if s == "" {
		return fail("empty")
	}
	intPart, frac, hasPoint := strings.Cut(s, ".")
	if intPart == "" || !allDigits(intPart) || !allDigits(frac) || (hasPoint && frac == "") || strings.Contains(frac, ".") {
		return fail("a rate is digits with an optional decimal point")
	}
	if len(frac) > MaxUSDRateDecimals {
		return fail(fmt.Sprintf("more than %d decimal places", MaxUSDRateDecimals))
	}
	frac += strings.Repeat("0", MaxUSDRateDecimals-len(frac))
	p, err := fromDigits(text, intPart+frac, false)
	if err != nil {
		return 0, err
	}
	if p <= 0 {
		return fail("a rate must be above zero")
	}
	return USDRate(p), nil
}

// String writes the rate with four decimal places ("83.2500").
func (r USDRate) String() string {
	return fmt.Sprintf("%d.%04d", int64(r)/10000, int64(r)%10000)
}

// IsUSDText reports whether a written amount is marked as US dollars: a leading "$" or a
// leading or trailing "USD", with a minus sign or brackets around it allowed. It does not say
// whether the amount is well formed; ParseUSD does.
func IsUSDText(text string) bool {
	s := strings.TrimSpace(text)
	s = strings.TrimPrefix(s, "-")
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = strings.TrimSpace(s[1 : len(s)-1])
		s = strings.TrimPrefix(s, "-")
	}
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "$") || strings.HasPrefix(s, "USD") || strings.HasSuffix(s, "USD")
}

// ParseUSD reads an amount written in US dollars and returns it in cents (hundredths of a
// dollar), exactly. It accepts a leading "$", or "USD" before or after the number; Western
// digit grouping ("1,250.50"); up to two decimal places; and a leading minus sign or accounting
// brackets for a negative. A "$" together with "USD" or "₹", a "$" inside the number, more than
// two decimal places, Indian grouping and anything else ambiguous is refused with a
// *ParseError. A text with no dollar mark is refused too: only an amount marked as dollars is
// read as dollars. An amount too large to hold returns an error wrapping ErrOverflow.
func ParseUSD(text string) (cents int64, err error) {
	fail := func(reason string) (int64, error) { return 0, &ParseError{Input: text, Reason: reason} }
	s := strings.TrimSpace(text)
	if s == "" {
		return fail("empty")
	}
	negative := false
	if rest, ok := strings.CutPrefix(s, "-"); ok {
		negative, s = true, strings.TrimSpace(rest)
	}
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		if negative {
			return fail("both a minus sign and brackets")
		}
		negative, s = true, strings.TrimSpace(s[1:len(s)-1])
		if strings.HasPrefix(s, "-") {
			return fail("both a minus sign and brackets")
		}
	}
	if strings.Contains(s, "₹") {
		return fail("a dollar amount cannot also carry a rupee sign")
	}
	marks := 0
	if rest, ok := strings.CutPrefix(s, "$"); ok {
		marks++
		s = strings.TrimSpace(rest)
	}
	if rest, ok := strings.CutPrefix(s, "USD"); ok {
		marks++
		s = strings.TrimSpace(rest)
	}
	if rest, ok := strings.CutSuffix(s, "USD"); ok {
		marks++
		s = strings.TrimSpace(rest)
	}
	switch {
	case marks == 0:
		return fail("not marked as US dollars")
	case marks > 1:
		return fail("more than one dollar mark")
	}
	if strings.ContainsAny(s, "$") || strings.Contains(s, "USD") {
		return fail("a dollar mark inside the number")
	}
	if strings.HasPrefix(s, "-") || strings.HasPrefix(s, "+") || strings.HasPrefix(s, "(") {
		return fail("a sign or bracket after the dollar mark")
	}
	if strings.Contains(s, ",") {
		intPart, _, _ := strings.Cut(s, ".")
		if !westernGrouped(intPart) {
			return fail("commas are not in valid Western positions")
		}
	}
	p, err := Parse(s, UnitRupees) // the digits are cents: two decimal places, no more
	if err != nil {
		var pe *ParseError
		if errors.As(err, &pe) {
			return fail(pe.Reason)
		}
		return 0, err
	}
	if negative {
		if p, err = Neg(p); err != nil {
			return 0, fmt.Errorf("money: %q: %w", text, err)
		}
	}
	return int64(p), nil
}

// westernGrouped reports whether the integer part uses commas as Western groups of three:
// "1,250" and "12,345,678", but not "1,25,000" (Indian) or "12,34".
func westernGrouped(intPart string) bool {
	groups := strings.Split(intPart, ",")
	if len(groups[0]) < 1 || len(groups[0]) > 3 {
		return false
	}
	for _, g := range groups[1:] {
		if len(g) != 3 {
			return false
		}
	}
	return true
}

// ConvertUSD converts an amount in cents to paise at rate: cents × rate / 10,000, rounded to
// the nearest paisa with a half going away from zero. It uses integer arithmetic only. A result
// too large to hold returns ErrOverflow; a rate that is not above zero returns ErrNoRate.
func ConvertUSD(cents int64, rate USDRate) (Paise, error) {
	if rate <= 0 {
		return 0, ErrNoRate
	}
	return MulRatio(Paise(cents), int64(rate), BasisPointsPerUnit, RoundHalfUpPaisa)
}

// ParseUSDAsPaise reads a dollar-marked amount and converts it to paise at rate.
func ParseUSDAsPaise(text string, rate USDRate) (Paise, error) {
	cents, err := ParseUSD(text)
	if err != nil {
		return 0, err
	}
	p, err := ConvertUSD(cents, rate)
	if err != nil {
		return 0, fmt.Errorf("money: %q: %w", text, err)
	}
	return p, nil
}
