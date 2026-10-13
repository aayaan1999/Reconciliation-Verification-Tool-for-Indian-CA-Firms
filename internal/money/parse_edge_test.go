package money

import (
	"encoding/json"
	"errors"
	"math/big"
	"math/rand"
	"strconv"
	"strings"
	"testing"
)

func TestParseEdgeAccepted(t *testing.T) {
	cases := []struct {
		in   string
		unit Unit
		want Paise
	}{
		{"0.50", UnitRupees, 50},
		{"0.5", UnitRupees, 50},
		{"1,23,45,678.90", UnitRupees, 1234567890}, // 12345678 rupees 90 paise
		{"1,00,00,00,000", UnitRupees, 1_000_000_000 * 100},
		{"999", UnitRupees, 99900},
		{"1,000", UnitRupees, 100000},
		{"10,000", UnitRupees, 1000000},
		{"1,00,000", UnitRupees, 10000000},
		{"1,000,000", UnitRupees, 100000000},
		{"-1,23,456.78", UnitRupees, -12345678},
		{"(1,23,456.78)", UnitRupees, -12345678},
		{"  (2,750.00)  ", UnitRupees, -275000},
		{"\t₹5\n", UnitRupees, 500},
		{"₹0.00", UnitRupees, 0},
		{"(0)", UnitRupees, 0},
		{"00.10", UnitRupees, 10},
	}
	for _, c := range cases {
		got, err := Parse(c.in, c.unit)
		if err != nil || got != c.want {
			t.Errorf("Parse(%q) = %d,%v; want %d", c.in, got, err, c.want)
		}
	}
}

func TestParseEdgeRefused(t *testing.T) {
	cases := []struct {
		name string
		in   string
		unit Unit
	}{
		{"leading point", ".5", UnitRupees},
		{"trailing point", "5.", UnitRupees},
		{"only point", ".", UnitRupees},
		{"three decimals", "1.005", UnitRupees},
		{"three decimals zeros", "1.000", UnitRupees},
		{"many decimals", "1.123456789", UnitRupees},
		{"two points", "1.2.3", UnitRupees},
		{"empty", "", UnitRupees},
		{"spaces only", "   ", UnitRupees},
		{"newline only", "\n", UnitRupees},
		{"minus only", "-", UnitRupees},
		{"rupee only", "₹", UnitRupees},
		{"empty brackets", "()", UnitRupees},
		{"plus sign", "+5", UnitRupees},
		{"double minus", "--5", UnitRupees},
		{"minus then brackets", "-(5)", UnitRupees},
		{"brackets then minus", "(-5)", UnitRupees},
		{"unclosed bracket", "(5", UnitRupees},
		{"unopened bracket", "5)", UnitRupees},
		{"nested brackets", "((5))", UnitRupees},
		{"inner space", "1 000", UnitRupees},
		{"space after minus", "- 5", UnitRupees},
		{"space inside brackets", "( 5 )", UnitRupees},
		{"rupee after digits", "5₹", UnitRupees},
		{"double rupee", "₹₹5", UnitRupees},
		{"rupee then minus", "₹-5", UnitRupees},
		{"Rs prefix", "Rs. 5", UnitRupees},
		{"INR prefix", "INR 5", UnitRupees},
		{"Dr suffix", "1,000.00 Dr", UnitRupees},
		{"Cr suffix", "1,000.00Cr", UnitRupees},
		{"percent", "5%", UnitRupees},
		{"exponent lower", "1e5", UnitRupees},
		{"exponent upper", "1E5", UnitRupees},
		{"exponent signed", "1e+5", UnitRupees},
		{"hex", "0x10", UnitRupees},
		{"underscore", "1_000", UnitRupees},
		{"infinity", "Inf", UnitRupees},
		{"nan", "NaN", UnitRupees},
		{"arabic-indic digit", "٥", UnitRupees},
		{"fullwidth digit", "５", UnitRupees},
		{"NUL byte", "5\x00", UnitRupees},
		{"leading comma", ",100", UnitRupees},
		{"trailing comma", "100,", UnitRupees},
		{"double comma", "1,,000", UnitRupees},
		{"comma in fraction", "1.5,00", UnitRupees},
		{"two digit group only", "1,00", UnitRupees},
		{"two digit last group", "12,34", UnitRupees},
		{"four digit last group", "1,2345", UnitRupees},
		{"mixed grouping", "1,23,456,789", UnitRupees},
		{"mixed grouping 2", "12,345,67,890", UnitRupees},
		{"western four digit lead", "1234,567", UnitRupees},
		{"indian three digit lead", "123,45,678", UnitRupees},
		{"leading zero group", "01,000", UnitRupees},
		{"zero lead group", "0,100", UnitRupees},
		{"single digit last", "1,2", UnitRupees},
		{"group of one in middle", "1,2,345", UnitRupees},
		{"group of four in middle", "1,2345,678", UnitRupees},
		{"decimals in paise", "100.0", UnitPaise},
		{"fraction paise with commas", "1,000.5", UnitPaise},
		{"unknown unit zero", "5", Unit(0)},
		{"unknown unit big", "5", Unit(99)},
		{"overflow rupees", "92233720368547758.08", UnitRupees},
		{"overflow paise", "9223372036854775808", UnitPaise},
		{"underflow paise", "-9223372036854775809", UnitPaise},
		{"huge digits paise", strings.Repeat("9", 400), UnitPaise},
		{"huge digits rupees", strings.Repeat("9", 400), UnitRupees},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Parse(c.in, c.unit)
			if err == nil {
				t.Fatalf("Parse(%q) = %d, want error", c.in, got)
			}
			if got != 0 {
				t.Errorf("value on error = %d, want 0", got)
			}
			var pe *ParseError
			if !errors.As(err, &pe) && !errors.Is(err, ErrOverflow) {
				t.Errorf("error %T %v is neither *ParseError nor ErrOverflow", err, err)
			}
		})
	}
}

func TestParseExtremeLimits(t *testing.T) {
	cases := []struct {
		in   string
		unit Unit
		want Paise
	}{
		{"9223372036854775807", UnitPaise, MaxPaise},
		{"-9223372036854775807", UnitPaise, MinPaise + 1},
		{"-9223372036854775808", UnitPaise, MinPaise},
		{"(9223372036854775808)", UnitPaise, MinPaise},
		{"92233720368547758.07", UnitRupees, MaxPaise},
		{"-92233720368547758.08", UnitRupees, MinPaise},
		{"-0000000000000000000000009223372036854775808", UnitPaise, MinPaise},
	}
	for _, c := range cases {
		got, err := Parse(c.in, c.unit)
		if err != nil || got != c.want {
			t.Errorf("Parse(%q) = %d,%v; want %d", c.in, got, err, c.want)
		}
	}
	if _, err := Parse("(92233720368547758.09)", UnitRupees); !errors.Is(err, ErrOverflow) {
		t.Errorf("one past MinPaise err = %v, want ErrOverflow", err)
	}
	if _, err := Parse("92233720368547758.08", UnitRupees); !errors.Is(err, ErrOverflow) {
		t.Errorf("one past MaxPaise err = %v, want ErrOverflow", err)
	}
}

func TestParseJSONNumberEdges(t *testing.T) {
	good := []struct {
		in   string
		unit Unit
		want Paise
	}{
		{"0", UnitRupees, 0},
		{"-0.5", UnitRupees, -50},
		{"112450.50", UnitRupees, 11245050},
		{"11245050", UnitPaise, 11245050},
	}
	for _, c := range good {
		got, err := ParseJSONNumber(json.Number(c.in), c.unit)
		if err != nil || got != c.want {
			t.Errorf("ParseJSONNumber(%q) = %d,%v; want %d", c.in, got, err, c.want)
		}
	}
	bad := []string{"", "1,000", "(5)", "₹5", "1e5", "1E-2", "0.001", ".5", "5.", "--1", "1.5.2", "NaN"}
	for _, in := range bad {
		if got, err := ParseJSONNumber(json.Number(in), UnitRupees); err == nil {
			t.Errorf("ParseJSONNumber(%q) = %d, want error", in, got)
		}
	}
	if _, err := ParseJSONNumber("1.5", UnitPaise); err == nil {
		t.Error("fractional paise accepted")
	}
}

func TestParseMatchesBigIntOracle(t *testing.T) {
	rng := rand.New(rand.NewSource(1234))
	for i := 0; i < 20000; i++ {
		v := new(big.Int).SetUint64(rng.Uint64())
		if rng.Intn(2) == 0 {
			v.Neg(v)
		}
		abs := new(big.Int).Abs(v)
		q, r := new(big.Int).QuoRem(abs, big.NewInt(100), new(big.Int))
		sign := ""
		if v.Sign() < 0 {
			sign = "-"
		}
		paise := strconv.FormatInt(r.Int64(), 10)
		if len(paise) < 2 {
			paise = "0" + paise
		}
		for _, c := range []struct {
			in   string
			unit Unit
		}{{sign + q.String() + "." + paise, UnitRupees}, {v.String(), UnitPaise}} {
			got, err := Parse(c.in, c.unit)
			if v.IsInt64() {
				if err != nil || int64(got) != v.Int64() {
					t.Fatalf("Parse(%q) = %d,%v; want %s", c.in, got, err, v)
				}
			} else if !errors.Is(err, ErrOverflow) {
				t.Fatalf("Parse(%q) = %d,%v; want ErrOverflow", c.in, got, err)
			}
		}
	}
}

func TestParseErrorMessage(t *testing.T) {
	_, err := Parse("12.345", UnitRupees)
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("err %T not *ParseError", err)
	}
	if pe.Input != "12.345" || pe.Reason == "" {
		t.Errorf("ParseError = %+v", pe)
	}
	for _, bad := range []string{"should", "incorrect", "must be"} {
		if strings.Contains(strings.ToLower(err.Error()), bad) {
			t.Errorf("message %q contains forbidden wording %q", err.Error(), bad)
		}
	}
}

func TestParseRoundedEdges(t *testing.T) {
	cases := []struct {
		in   string
		unit Unit
		mode Rounding
		want Paise
		err  error
	}{
		{"12.345", UnitRupees, RoundHalfUpPaisa, 1235, nil},
		{"12.344", UnitRupees, RoundHalfUpPaisa, 1234, nil},
		{"-12.345", UnitRupees, RoundHalfUpPaisa, -1235, nil},
		{"(12.345)", UnitRupees, RoundHalfUpPaisa, -1235, nil},
		{"12.349", UnitRupees, RoundDownPaisa, 1234, nil},
		{"-12.341", UnitRupees, RoundDownPaisa, -1234, nil},
		{"12.341", UnitRupees, RoundUpPaisa, 1235, nil},
		{"-12.341", UnitRupees, RoundUpPaisa, -1235, nil},
		{"12.340", UnitRupees, RoundNone, 1234, nil},
		{"12.3400000000000000000000", UnitRupees, RoundNone, 1234, nil},
		{"12.341", UnitRupees, RoundNone, 0, ErrInexact},
		{"12.5", UnitRupees, RoundHalfUpRupee, 1300, nil},
		{"12.49", UnitRupees, RoundHalfUpRupee, 1200, nil},
		{"-12.5", UnitRupees, RoundHalfUpRupee, -1300, nil},
		{"15", UnitRupees, RoundNearestTenRupees, 2000, nil},
		{"14.99", UnitRupees, RoundNearestTenRupees, 1000, nil},
		{"-15", UnitRupees, RoundNearestTenRupees, -2000, nil},
		{"1,12,450.555", UnitRupees, RoundHalfUpPaisa, 11245056, nil},
		{"0.005", UnitRupees, RoundHalfUpPaisa, 1, nil},
		{"0.004", UnitRupees, RoundHalfUpPaisa, 0, nil},
		{"-0.004", UnitRupees, RoundHalfUpPaisa, 0, nil},
		{"-0.004", UnitRupees, RoundUpPaisa, -1, nil},
		{"100.5", UnitPaise, RoundHalfUpPaisa, 101, nil},
		{"100.4", UnitPaise, RoundHalfUpPaisa, 100, nil},
		{"100", UnitPaise, RoundNone, 100, nil},
		{"5", UnitRupees, RoundHalfUpPaisa, 500, nil},
		{"5.1", UnitRupees, RoundHalfUpPaisa, 510, nil},
		{"92233720368547758.074", UnitRupees, RoundDownPaisa, MaxPaise, nil},
		{"92233720368547758.075", UnitRupees, RoundHalfUpPaisa, 0, ErrOverflow},
		{"92233720368547758.071", UnitRupees, RoundUpPaisa, 0, ErrOverflow},
		{"-92233720368547758.085", UnitRupees, RoundHalfUpPaisa, 0, ErrOverflow},
		{"12.345", UnitRupees, Rounding(0), 0, ErrRounding},
		{"12.345", UnitRupees, Rounding(77), 0, ErrRounding},
		{"12.345", UnitRupees, Rounding(-1), 0, ErrRounding},
	}
	for _, c := range cases {
		got, err := ParseRounded(c.in, c.unit, c.mode)
		if c.err != nil {
			if !errors.Is(err, c.err) {
				t.Errorf("ParseRounded(%q, mode %d) err = %v, want %v", c.in, c.mode, err, c.err)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("ParseRounded(%q, mode %d) = %d,%v; want %d", c.in, c.mode, got, err, c.want)
		}
	}
	for _, bad := range []string{"", "x", "1.", ".5", "1e5", "1,2", "1.2.3", "-", "(1)5"} {
		if got, err := ParseRounded(bad, UnitRupees, RoundHalfUpPaisa); err == nil {
			t.Errorf("ParseRounded(%q) = %d, want error", bad, got)
		}
	}
	if _, err := ParseRounded("5", Unit(0), RoundHalfUpPaisa); err == nil {
		t.Error("unknown unit accepted")
	}
}

func TestParseRoundedAgreesWithParse(t *testing.T) {
	rng := rand.New(rand.NewSource(5))
	for i := 0; i < 5000; i++ {
		p := Paise(rng.Int63n(2_000_000_000_000) - 1_000_000_000_000)
		s, _ := FormatIndian(p, true)
		a, errA := Parse(s, UnitRupees)
		b, errB := ParseRounded(s, UnitRupees, RoundNone)
		if errA != nil || errB != nil || a != b || a != p {
			t.Fatalf("%q: Parse=%d,%v ParseRounded=%d,%v want %d", s, a, errA, b, errB, p)
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, s := range []string{"1,12,450.50", "(2,750.00)", "-0.50", "12.345", "", "1e5", "₹5", ".5", "0.50",
		"92233720368547758.07", "-92233720368547758.08", "1,23,45,678.90", "((", "1,,0", "\x00"} {
		f.Add(s, true)
		f.Add(s, false)
	}
	f.Fuzz(func(t *testing.T, s string, rupees bool) {
		unit := UnitPaise
		if rupees {
			unit = UnitRupees
		}
		p, err := Parse(s, unit)
		if err != nil {
			if p != 0 {
				t.Fatalf("Parse(%q) returned %d with error %v", s, p, err)
			}
			var pe *ParseError
			if !errors.As(err, &pe) && !errors.Is(err, ErrOverflow) {
				t.Fatalf("Parse(%q) error %T is not *ParseError or ErrOverflow", s, err)
			}
			return
		}
		out, ferr := FormatIndian(p, true)
		if ferr != nil {
			t.Fatalf("FormatIndian(%d): %v", p, ferr)
		}
		back, perr := Parse(out, UnitRupees)
		if perr != nil || back != p {
			t.Fatalf("Parse(%q)=%d -> %q -> %d,%v", s, p, out, back, perr)
		}
		pr, rerr := ParseRounded(s, unit, RoundNone)
		if rerr != nil || pr != p {
			t.Fatalf("ParseRounded(%q, RoundNone) = %d,%v; Parse gave %d", s, pr, rerr, p)
		}
	})
}

func FuzzParseRounded(f *testing.F) {
	for _, s := range []string{"12.345", "-0.005", "(1,12,450.555)", "99999999999999999999.999", "0.0000000001", "5"} {
		f.Add(s, uint8(2))
	}
	f.Fuzz(func(t *testing.T, s string, m uint8) {
		mode := Rounding(m % 8) // includes the invalid values 0 and 7
		p, err := ParseRounded(s, UnitRupees, mode)
		if err != nil && p != 0 {
			t.Fatalf("ParseRounded(%q) returned %d with error %v", s, p, err)
		}
		if mode == 0 || mode > RoundUpPaisa {
			if !errors.Is(err, ErrRounding) {
				t.Fatalf("mode %d: err = %v, want ErrRounding", mode, err)
			}
		}
	})
}
