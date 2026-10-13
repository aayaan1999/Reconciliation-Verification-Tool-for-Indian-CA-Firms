package money

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// Unit says whether a written amount is in rupees or in paise.
type Unit int

const (
	// UnitRupees reads "1,12,450.50" as ₹1,12,450.50. Up to two decimal places.
	UnitRupees Unit = iota + 1
	// UnitPaise reads "11245050" as 11245050 paise. No decimal places.
	UnitPaise
)

// MaxRoundedDecimals is the most decimal places ParseRounded accepts. It bounds the
// work done on hostile input; no real document carries more.
const MaxRoundedDecimals = 40

// ParseError explains why a text could not be read as an amount.
type ParseError struct {
	Input  string
	Reason string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("money: cannot read %q as an amount: %s", e.Input, e.Reason)
}

// Parse reads an amount as written in a document. It accepts plain digits, Indian
// ("1,12,450.50") or Western ("112,450.50") digit grouping, a leading minus sign or
// accounting brackets for negatives ("(2,750.00)"), and a leading "₹". It never goes
// through floating point. Anything ambiguous is refused with a *ParseError, and an
// amount too large to hold returns an error wrapping ErrOverflow.
func Parse(text string, unit Unit) (Paise, error) {
	return parse(text, unit, true, 0)
}

// ParseJSONNumber reads a JSON number (decoded with json.Decoder.UseNumber) exactly.
// It applies the same rules as Parse but allows no grouping, brackets, "₹",
// surrounding spaces or leading zeros ("007"), and refuses exponents such as "1e5".
func ParseJSONNumber(n json.Number, unit Unit) (Paise, error) {
	return parse(string(n), unit, false, 0)
}

// parse implements Parse, ParseJSONNumber and ParseRounded. documentForms enables the
// forms only found in documents (grouping commas, brackets, "₹"). mode 0 means strict:
// extra decimal places are refused; otherwise they are rounded by mode.
func parse(input string, unit Unit, documentForms bool, mode Rounding) (Paise, error) {
	fail := func(reason string) (Paise, error) {
		return 0, &ParseError{Input: input, Reason: reason}
	}
	if unit != UnitRupees && unit != UnitPaise {
		return fail("unknown unit")
	}

	s := strings.TrimSpace(input)
	if s == "" {
		return fail("empty")
	}
	if !documentForms && s != input {
		return fail("spaces around the number")
	}
	negative := false
	if rest, ok := strings.CutPrefix(s, "-"); ok {
		negative, s = true, rest
	}
	if documentForms && strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		if negative {
			return fail("both a minus sign and brackets")
		}
		negative, s = true, s[1:len(s)-1]
	}
	if documentForms {
		s = strings.TrimPrefix(s, "₹")
	}

	intPart, fracPart, hasPoint := strings.Cut(s, ".")
	if strings.Contains(fracPart, ".") {
		return fail("more than one decimal point")
	}
	if intPart == "" {
		return fail("no digits before the decimal point")
	}
	if hasPoint && fracPart == "" {
		return fail("no digits after the decimal point")
	}
	if strings.Contains(intPart, ",") {
		if !documentForms {
			return fail("grouping commas are not allowed here")
		}
		var ok bool
		if intPart, ok = ungroup(intPart); !ok {
			return fail("commas are not in valid Indian or Western positions")
		}
	}
	if !allDigits(intPart) || !allDigits(fracPart) {
		return fail("contains characters that are not digits")
	}
	if !documentForms && len(intPart) > 1 && intPart[0] == '0' {
		return fail("leading zeros are not allowed in a JSON number")
	}

	if mode != 0 {
		if len(fracPart) > MaxRoundedDecimals {
			return fail(fmt.Sprintf("more than %d decimal places", MaxRoundedDecimals))
		}
		scale := len(fracPart) // decimal places of the digits, measured in paise
		if unit == UnitRupees {
			scale -= 2
		}
		digits := intPart + fracPart
		if scale < 0 {
			digits += strings.Repeat("0", -scale)
			scale = 0
		}
		return roundedFromDigits(input, digits, scale, negative, mode)
	}

	switch unit {
	case UnitPaise:
		if hasPoint {
			return fail("a paise amount cannot have decimal places")
		}
	case UnitRupees:
		if len(fracPart) > 2 {
			return fail("more than two decimal places")
		}
		fracPart += strings.Repeat("0", 2-len(fracPart))
	}
	return fromDigits(input, intPart+fracPart, negative)
}

// ungroup removes grouping commas if they sit in valid positions: Western groups of
// three ("1,234,567") or Indian groups of two before the last three ("12,34,567").
func ungroup(s string) (string, bool) {
	groups := strings.Split(s, ",")
	first, last, middle := groups[0], groups[len(groups)-1], groups[1:len(groups)-1]
	if len(last) != 3 || first == "" || len(first) > 3 || first[0] == '0' {
		return "", false
	}
	if len(middle) > 0 {
		size := len(middle[0])
		if size != 2 && size != 3 {
			return "", false
		}
		for _, g := range middle {
			if len(g) != size {
				return "", false
			}
		}
		if size == 2 && len(first) > 2 {
			return "", false // Indian grouping: the leading group has one or two digits
		}
	}
	return strings.Join(groups, ""), true
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// fromDigits converts a string of decimal digits (already in paise) to Paise,
// checking for overflow at every digit.
func fromDigits(input, digits string, negative bool) (Paise, error) {
	limit := uint64(math.MaxInt64)
	if negative {
		limit++ // MinPaise has one more unit of magnitude than MaxPaise
	}
	var mag uint64
	for i := 0; i < len(digits); i++ {
		d := uint64(digits[i] - '0')
		if mag > (limit-d)/10 {
			return 0, fmt.Errorf("money: %q: %w", input, ErrOverflow)
		}
		mag = mag*10 + d
	}
	if !negative {
		return Paise(mag), nil
	}
	if mag == uint64(math.MaxInt64)+1 {
		return MinPaise, nil
	}
	return -Paise(mag), nil
}
