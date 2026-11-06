package money

import (
	"errors"
	"math"
	"os"
	"strings"
	"testing"
)

// SPEC-019: dollar amounts read in cents and converted to paise at a fixed rate. All values
// are illustrative.

func TestParseUSDForms(t *testing.T) { // AC-1
	for _, in := range []string{"$1,250.50", "USD 1,250.50", "1,250.50 USD", "USD1,250.50", " $ 1,250.50 ", "$1250.50"} {
		got, err := ParseUSD(in)
		if err != nil || got != 125050 {
			t.Errorf("ParseUSD(%q) = %d, %v; want 125050 cents", in, got, err)
		}
	}
	for _, in := range []string{"($1,250.50)", "-$1,250.50", "(USD 1,250.50)", "-1,250.50 USD"} {
		got, err := ParseUSD(in)
		if err != nil || got != -125050 {
			t.Errorf("ParseUSD(%q) = %d, %v; want -125050 cents", in, got, err)
		}
	}
	for in, want := range map[string]int64{"$0.05": 5, "$7": 700, "$0": 0, "$1,000": 100000, "$12,345,678.90": 1234567890} {
		if got, err := ParseUSD(in); err != nil || got != want {
			t.Errorf("ParseUSD(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
}

func TestIsUSDText(t *testing.T) {
	for in, want := range map[string]bool{
		"$1,250.50": true, "USD 5": true, "5 USD": true, "($5)": true, "-$5": true, "1,250.50": false, "₹1,250.50": false,
		"": false, "(2,750.00)": false, "-12": false, "USDA": true, // marked: ParseUSD then refuses the rest
	} {
		if got := IsUSDText(in); got != want {
			t.Errorf("IsUSDText(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestConvertUSDRates(t *testing.T) { // AC-2
	rate, err := ParseUSDRate("83.2500")
	if err != nil || rate != 832500 {
		t.Fatalf("rate = %d, %v", rate, err)
	}
	// 125,050 cents × 832,500 / 10,000 = 10,410,412.5: the half goes away from zero.
	if p, err := ParseUSDAsPaise("$1,250.50", rate); err != nil || p != 10410413 {
		t.Errorf("$1,250.50 at 83.2500 = %d, %v; want 10,410,413 paise", p, err)
	}
	if p, err := ParseUSDAsPaise("($1,250.50)", rate); err != nil || p != -10410413 {
		t.Errorf("($1,250.50) at 83.2500 = %d, %v; want -10,410,413 paise (half away from zero)", p, err)
	}
	r83, _ := ParseUSDRate("83.0000")
	if p, err := ParseUSDAsPaise("$1,250.50", r83); err != nil || p != 10379150 {
		t.Errorf("$1,250.50 at 83.0000 = %d, %v; want 10,379,150 paise", p, err)
	}
	// Either side of the half: 1 cent at 83.2500 = 83.25 paise (down); 3 cents at 0.0050 = 0.15 paise (down);
	// 1 cent at 0.0050 = 0.05; 100 cents at 0.5050 = 50.5 paise (up).
	for _, c := range []struct {
		cents int64
		rate  string
		want  Paise
	}{{1, "83.2500", 83}, {3, "0.0050", 0}, {100, "0.5050", 51}, {-100, "0.5050", -51}, {100, "0.5049", 50}, {-100, "0.5049", -50}} {
		rt, err := ParseUSDRate(c.rate)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := ConvertUSD(c.cents, rt); err != nil || got != c.want {
			t.Errorf("%d cents at %s = %d, %v; want %d", c.cents, c.rate, got, err, c.want)
		}
	}
}

func TestParseUSDRate(t *testing.T) {
	good := map[string]USDRate{"83": 830000, "83.5": 835000, "83.2500": 832500, "0.0001": 1, " 83.25 ": 832500}
	for in, want := range good {
		if got, err := ParseUSDRate(in); err != nil || got != want {
			t.Errorf("ParseUSDRate(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "0", "0.0000", "-83", "+83", "83.12345", "8,3.25", "abc", "83.", ".5", "1e2", "83.2.5", "$83"} {
		var pe *ParseError
		if _, err := ParseUSDRate(in); !errors.As(err, &pe) {
			t.Errorf("ParseUSDRate(%q) = %v; want a *ParseError", in, err)
		}
	}
	if s := USDRate(832500).String(); s != "83.2500" {
		t.Errorf("String = %q", s)
	}
}

func TestConvertUSDOverflowAndIntegerOnly(t *testing.T) { // AC-3
	rate := USDRate(10000) // 1 rupee per dollar: paise = cents
	if p, err := ConvertUSD(math.MaxInt64, rate); err != nil || p != Paise(math.MaxInt64) {
		t.Errorf("the largest amount: %d, %v", p, err)
	}
	if _, err := ConvertUSD(math.MaxInt64, USDRate(10001)); !errors.Is(err, ErrOverflow) {
		t.Errorf("above the largest: %v, want an error wrapping ErrOverflow", err)
	}
	if _, err := ConvertUSD(math.MinInt64, USDRate(10001)); !errors.Is(err, ErrOverflow) {
		t.Errorf("below the smallest: %v, want an error wrapping ErrOverflow", err)
	}
	if _, err := ParseUSD("$92,233,720,368,547,758.08"); !errors.Is(err, ErrOverflow) {
		t.Errorf("one cent above the largest: %v, want ErrOverflow", err)
	}
	for _, bad := range []USDRate{0, -1} {
		if _, err := ConvertUSD(100, bad); !errors.Is(err, ErrNoRate) {
			t.Errorf("rate %d: %v, want ErrNoRate", bad, err)
		}
	}
	// No floating point in the conversion: the source of usd.go has no float type or float parser.
	src, err := os.ReadFile("usd.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"float32", "float64", "ParseFloat", "math.Round", "math.Floor"} {
		if strings.Contains(string(src), banned) {
			t.Errorf("usd.go uses %s", banned)
		}
	}
}

func TestParseUSDRefused(t *testing.T) { // AC-4
	for _, in := range []string{
		"$1,250.505",        // more than two decimal places
		"$12.345",           // the same, no grouping
		"12$5.00",           // a $ in the middle
		"1,250$.50",         // the same
		"$₹1,250.50",        // dollar and rupee together
		"₹$1,250.50",        // the same, the other way
		"$ USD 5",           // USD with a $
		"USD 5 USD",         // two marks
		"$1,25,000.00",      // Indian grouping
		"$1,2500.00",        // commas in the wrong places
		"$-5",               // a sign after the mark
		"$(5)",              // brackets after the mark
		"-($5)",             // a minus sign and brackets
		"($-5)",             // the same
		"$",                 // no digits
		"USD",               // no digits
		"$.50",              // no digits before the point
		"$5.",               // no digits after the point
		"$1e3",              // not digits
		"$ 1 250.50",        // a space inside the number
		"1,250.50",          // not marked as dollars
		"",                  // empty
		"$1,250.50 dollars", // trailing words
	} {
		var pe *ParseError
		if got, err := ParseUSD(in); !errors.As(err, &pe) {
			t.Errorf("ParseUSD(%q) = %d, %v; want a *ParseError", in, got, err)
		}
	}
}

func TestRupeeReadingUnchanged(t *testing.T) { // AC-5
	// A text with no dollar mark is read as rupees exactly as before; the dollar functions do not touch Parse.
	for in, want := range map[string]Paise{"1,12,450.50": 11245050, "(2,750.00)": -275000, "₹5": 500, "112,450.50": 11245050} {
		if got, err := Parse(in, UnitRupees); err != nil || got != want {
			t.Errorf("Parse(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"$5", "USD 5"} {
		if _, err := Parse(in, UnitRupees); err == nil {
			t.Errorf("Parse(%q) was accepted: a dollar amount is read only through the dollar functions", in)
		}
	}
}
