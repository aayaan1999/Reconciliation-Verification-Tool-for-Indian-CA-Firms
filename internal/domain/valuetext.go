package domain

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
)

// FormatValue writes v as the text used in hand-written files (profile expected facts,
// rule fixtures): money as rupees with two decimals and no grouping ("112450.50",
// "-0.05"), whole numbers in digits, text as it is, dates as YYYY-MM-DD, yes/no as
// "true"/"false", and rates as a percentage with two decimals ("12.50").
// ParseValue reads it back.
func FormatValue(v Value) string {
	switch v.Kind {
	case KindMoney:
		return hundredths(int64(v.Money))
	case KindInt:
		return strconv.FormatInt(v.Int, 10)
	case KindText:
		return v.Text
	case KindDate:
		return v.Date
	case KindBool:
		return strconv.FormatBool(v.Bool)
	case KindRate:
		return hundredths(v.RateBP)
	}
	return ""
}

// hundredths writes n/100 with two decimals, safely for every int64.
func hundredths(n int64) string {
	sign, u := "", uint64(n)
	if n < 0 {
		sign, u = "-", uint64(-(n+1))+1
	}
	return fmt.Sprintf("%s%d.%02d", sign, u/100, u%100)
}

// ParseValue reads text written as FormatValue writes it, for a value of kind k. It is
// strict: money takes at most two decimal places and no grouping commas, whole numbers
// take no leading zeros or "+", dates must be real dates, and rates are percentages
// from 0 with at most two decimal places.
func ParseValue(k ValueKind, s string) (Value, error) {
	switch k {
	case KindMoney:
		p, err := money.ParseJSONNumber(json.Number(s), money.UnitRupees)
		if err != nil {
			return Value{}, err
		}
		return MoneyValue(p), nil
	case KindInt:
		digits := strings.TrimPrefix(s, "-")
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil || digits == "" || digits[0] == '+' || (len(digits) > 1 && digits[0] == '0') || s == "-0" {
			return Value{}, fmt.Errorf("%q is not a whole number", s)
		}
		return IntValue(n), nil
	case KindText:
		return TextValue(s), nil
	case KindDate:
		d, err := period.ParseDate(s)
		if err != nil {
			return Value{}, err
		}
		return DateValue(d.String()), nil
	case KindBool:
		switch s {
		case "true":
			return BoolValue(true), nil
		case "false":
			return BoolValue(false), nil
		}
		return Value{}, fmt.Errorf("%q is not true or false", s)
	case KindRate:
		bp, err := parseRate(s)
		if err != nil {
			return Value{}, err
		}
		return RateValue(bp), nil
	}
	return Value{}, fmt.Errorf("unknown value kind %d", k)
}

// parseRate reads a percentage with at most two decimal places ("10", "12.5", "0.75")
// as basis points (1000, 1250, 75).
func parseRate(s string) (int64, error) {
	bad := fmt.Errorf("%q is not a percentage with at most two decimal places", s)
	whole, frac, hasPoint := strings.Cut(s, ".")
	if whole == "" || len(whole) > 12 || len(frac) > 2 || (hasPoint && frac == "") || (len(whole) > 1 && whole[0] == '0') {
		return 0, bad
	}
	for _, r := range whole + frac {
		if r < '0' || r > '9' {
			return 0, bad
		}
	}
	n, err := strconv.ParseInt(whole+frac+strings.Repeat("0", 2-len(frac)), 10, 64)
	if err != nil {
		return 0, bad
	}
	return n, nil
}
