package domain

import (
	"math"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
)

func TestFormatAndParseValueRoundTrip(t *testing.T) {
	for _, c := range []struct {
		v    Value
		text string
	}{
		{MoneyValue(11245050), "112450.50"},
		{MoneyValue(0), "0.00"},
		{MoneyValue(-5), "-0.05"},
		{MoneyValue(-100), "-1.00"},
		{MoneyValue(money.Paise(math.MaxInt64)), "92233720368547758.07"},
		{IntValue(42), "42"},
		{IntValue(-7), "-7"},
		{IntValue(0), "0"},
		{TextValue("  Rent ₹ | x "), "  Rent ₹ | x "},
		{TextValue(""), ""},
		{DateValue("2026-09-30"), "2026-09-30"},
		{BoolValue(true), "true"},
		{BoolValue(false), "false"},
		{RateValue(1250), "12.50"},
		{RateValue(0), "0.00"},
	} {
		if got := FormatValue(c.v); got != c.text {
			t.Errorf("FormatValue(%+v) = %q, want %q", c.v, got, c.text)
		}
		back, err := ParseValue(c.v.Kind, c.text)
		if err != nil || back != c.v {
			t.Errorf("ParseValue(%s, %q) = %+v, %v; want %+v", c.v.Kind, c.text, back, err, c.v)
		}
	}
	if got := FormatValue(MoneyValue(money.Paise(math.MinInt64))); got != "-92233720368547758.08" {
		t.Errorf("most negative money = %q", got)
	}
	if FormatValue(Value{}) != "" {
		t.Error("an invalid value formats as text")
	}
}

func TestParseValueAcceptsShortForms(t *testing.T) {
	for _, c := range []struct {
		k    ValueKind
		text string
		want Value
	}{
		{KindMoney, "112450.5", MoneyValue(11245050)},
		{KindMoney, "112450", MoneyValue(11245000)},
		{KindRate, "12.5", RateValue(1250)},
		{KindRate, "10", RateValue(1000)},
		{KindRate, "0.75", RateValue(75)},
	} {
		if got, err := ParseValue(c.k, c.text); err != nil || got != c.want {
			t.Errorf("ParseValue(%s, %q) = %+v, %v; want %+v", c.k, c.text, got, err, c.want)
		}
	}
}

func TestParseValueRefuses(t *testing.T) {
	for _, c := range []struct {
		k    ValueKind
		text string
		msg  string
	}{
		{KindMoney, "1,12,450.50", "grouping commas"},
		{KindMoney, "112450.505", "more than two decimal places"},
		{KindMoney, "1e5", ""},
		{KindMoney, "", "empty"},
		{KindMoney, " 1.00", "spaces"},
		{KindInt, "007", "not a whole number"},
		{KindInt, "+7", "not a whole number"},
		{KindInt, "-0", "not a whole number"},
		{KindInt, "4.2", "not a whole number"},
		{KindInt, "", "not a whole number"},
		{KindDate, "2026-02-30", ""},
		{KindDate, "30/09/2026", ""},
		{KindBool, "yes", "not true or false"},
		{KindBool, "True", "not true or false"},
		{KindRate, "-1", "not a percentage"},
		{KindRate, "12.555", "not a percentage"},
		{KindRate, "012", "not a percentage"},
		{KindRate, "12.", "not a percentage"},
		{KindRate, "", "not a percentage"},
		{ValueKind(0), "1", "unknown value kind"},
	} {
		_, err := ParseValue(c.k, c.text)
		if err == nil || !strings.Contains(err.Error(), c.msg) {
			t.Errorf("ParseValue(%s, %q) = %v, want an error containing %q", c.k, c.text, err, c.msg)
		}
	}
}
