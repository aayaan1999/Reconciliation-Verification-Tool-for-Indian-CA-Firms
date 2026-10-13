package money

import (
	"errors"
	"testing"
)

// ₹11,245.055 is 1124505.5 paise: half a paisa above a whole paisa.
const tdsBase Paise = 11245055 // ₹1,12,450.55; 10% of it is ₹11,245.055

func TestMulRateRoundingModes(t *testing.T) {
	cases := []struct {
		mode Rounding
		want Paise
	}{
		{RoundHalfUpPaisa, 1124506},      // ₹11,245.06
		{RoundHalfUpRupee, 1124500},      // ₹11,245
		{RoundNearestTenRupees, 1125000}, // ₹11,250
		{RoundDownPaisa, 1124505},        // ₹11,245.05
		{RoundUpPaisa, 1124506},          // ₹11,245.06
	}
	for _, c := range cases {
		got, err := MulRate(tdsBase, 1000, c.mode)
		if err != nil || got != c.want {
			t.Errorf("mode %d: got %d, %v; want %d", c.mode, got, err, c.want)
		}
		// The same amount negated must round symmetrically (away from zero for "up").
		got, err = MulRate(-tdsBase, 1000, c.mode)
		if err != nil || got != -c.want {
			t.Errorf("mode %d negative: got %d, %v; want %d", c.mode, got, err, -c.want)
		}
	}
	if _, err := MulRate(tdsBase, 1000, RoundNone); !errors.Is(err, ErrInexact) {
		t.Errorf("RoundNone with a remainder: err %v, want ErrInexact", err)
	}
	if got, err := MulRate(11245000, 1000, RoundNone); err != nil || got != 1124500 {
		t.Errorf("RoundNone exact: got %d, %v", got, err)
	}
}

func TestRoundingAtExactlyHalf(t *testing.T) {
	cases := []struct {
		name string
		p    Paise
		mode Rounding
		want Paise
	}{
		{"₹5 to nearest ₹10 goes up", 500, RoundNearestTenRupees, 1000},
		{"-₹5 to nearest ₹10 goes away from zero", -500, RoundNearestTenRupees, -1000},
		{"₹4.99 to nearest ₹10 goes down", 499, RoundNearestTenRupees, 0},
		{"₹15 to nearest ₹10", 1500, RoundNearestTenRupees, 2000},
		{"₹14.99 to nearest ₹10", 1499, RoundNearestTenRupees, 1000},
		{"50 paise to nearest rupee goes up", 50, RoundHalfUpRupee, 100},
		{"49 paise to nearest rupee goes down", 49, RoundHalfUpRupee, 0},
		{"-50 paise to nearest rupee", -50, RoundHalfUpRupee, -100},
		{"₹1,234,565 to nearest ₹10", 123456500, RoundNearestTenRupees, 123457000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, err := MulRatio(c.p, 1, 1, c.mode); err != nil || got != c.want {
				t.Fatalf("got %d, %v; want %d", got, err, c.want)
			}
		})
	}
}

func TestMulRatio(t *testing.T) {
	// 7/12 of ₹1,20,000 is ₹70,000 exactly.
	if got, err := MulRatio(12000000, 7, 12, RoundNone); err != nil || got != 7000000 {
		t.Errorf("7/12: got %d, %v", got, err)
	}
	// A negative denominator is normalised: 1/-3 of ₹3 is -₹1.
	if got, err := MulRatio(300, 1, -3, RoundNone); err != nil || got != -100 {
		t.Errorf("negative denominator: got %d, %v", got, err)
	}
	if _, err := MulRatio(300, 1, 0, RoundHalfUpPaisa); err == nil {
		t.Error("zero denominator: want an error")
	}
	// 1/3 of ₹1 = 33.33… paise.
	for _, c := range []struct {
		mode Rounding
		want Paise
	}{{RoundHalfUpPaisa, 33}, {RoundDownPaisa, 33}, {RoundUpPaisa, 34}} {
		if got, err := MulRatio(100, 1, 3, c.mode); err != nil || got != c.want {
			t.Errorf("1/3 mode %d: got %d, %v; want %d", c.mode, got, err, c.want)
		}
	}
}

func TestMulRateLargeValues(t *testing.T) {
	// The intermediate product exceeds int64, but the result fits.
	if got, err := MulRate(MaxPaise, BasisPointsPerUnit, RoundNone); err != nil || got != MaxPaise {
		t.Errorf("100%% of MaxPaise: got %d, %v", got, err)
	}
	if got, err := MulRate(MinPaise, BasisPointsPerUnit, RoundNone); err != nil || got != MinPaise {
		t.Errorf("100%% of MinPaise: got %d, %v", got, err)
	}
	if _, err := MulRate(MaxPaise, 2*BasisPointsPerUnit, RoundHalfUpPaisa); !errors.Is(err, ErrOverflow) {
		t.Errorf("200%% of MaxPaise: err %v, want ErrOverflow", err)
	}
	// Rounding up past the largest amount must overflow, not wrap.
	if _, err := MulRatio(MaxPaise, 1, 1, RoundNearestTenRupees); !errors.Is(err, ErrOverflow) {
		t.Errorf("rounding MaxPaise up: err %v, want ErrOverflow", err)
	}
}

func TestUnknownRounding(t *testing.T) {
	for _, mode := range []Rounding{0, 99, -1} {
		if _, err := MulRate(100, 100, mode); !errors.Is(err, ErrRounding) {
			t.Errorf("MulRate mode %d: err %v, want ErrRounding", mode, err)
		}
		if _, err := ParseRounded("1.005", UnitRupees, mode); !errors.Is(err, ErrRounding) {
			t.Errorf("ParseRounded mode %d: err %v, want ErrRounding", mode, err)
		}
	}
}

func TestParseRounded(t *testing.T) {
	cases := []struct {
		in   string
		unit Unit
		mode Rounding
		want Paise
	}{
		{"11,245.055", UnitRupees, RoundHalfUpPaisa, 1124506},
		{"11,245.055", UnitRupees, RoundDownPaisa, 1124505},
		{"(11,245.055)", UnitRupees, RoundHalfUpPaisa, -1124506},
		{"12.340", UnitRupees, RoundNone, 1234},
		{"12.3", UnitRupees, RoundNone, 1230},
		{"12", UnitRupees, RoundNone, 1200},
		{"100.5", UnitPaise, RoundHalfUpPaisa, 101},
		{"1,12,454.99", UnitRupees, RoundNearestTenRupees, 11245000},
		{"1,12,455.00", UnitRupees, RoundNearestTenRupees, 11246000},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			if got, err := ParseRounded(c.in, c.unit, c.mode); err != nil || got != c.want {
				t.Fatalf("got %d, %v; want %d", got, err, c.want)
			}
		})
	}
	if _, err := ParseRounded("12.345", UnitRupees, RoundNone); !errors.Is(err, ErrInexact) {
		t.Errorf("RoundNone with extra decimals: err %v, want ErrInexact", err)
	}
	// The strict reader still refuses extra decimals.
	if _, err := Parse("12.345", UnitRupees); err == nil {
		t.Error("Parse must still refuse 12.345")
	}
	// Formatting errors are still reported as ParseError.
	var pe *ParseError
	if _, err := ParseRounded("1,1,2.005", UnitRupees, RoundHalfUpPaisa); !errors.As(err, &pe) {
		t.Errorf("bad grouping: err %v, want ParseError", err)
	}
}
