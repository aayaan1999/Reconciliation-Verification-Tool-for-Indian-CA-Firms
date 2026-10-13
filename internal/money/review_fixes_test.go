package money

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRoundingNamesRoundTrip(t *testing.T) {
	for mode := RoundNone; mode <= RoundUpPaisa; mode++ {
		got, err := ParseRounding(mode.String())
		if err != nil || got != mode {
			t.Errorf("ParseRounding(%q) = %v, %v; want %v", mode.String(), got, err, mode)
		}
	}
	for _, name := range []string{"", "half_up", "Half_Up_Rupee", "nearest_10_rupees ", "Rounding(1)"} {
		if got, err := ParseRounding(name); !errors.Is(err, ErrRounding) {
			t.Errorf("ParseRounding(%q) = %v, %v; want ErrRounding", name, got, err)
		}
	}
	if got := Rounding(0).String(); got != "Rounding(0)" {
		t.Errorf("Rounding(0).String() = %q", got)
	}
}

func TestMulRatioZeroDenominatorIsNamed(t *testing.T) {
	if _, err := MulRatio(100, 1, 0, RoundNone); !errors.Is(err, ErrZeroDenominator) {
		t.Errorf("err = %v; want ErrZeroDenominator", err)
	}
}

// TestNearestTenRupeesMatchesIgnorePaiseFirst pins RoundNearestTenRupees to the
// statutory two-step reading for positive amounts: drop the paise, then round the
// whole rupees to the nearest ₹10 with 5 or more going up.
func TestNearestTenRupeesMatchesIgnorePaiseFirst(t *testing.T) {
	statutory := func(p Paise) Paise {
		rupees := int64(p) / PaisePerRupee
		tens := rupees / 10
		if rupees%10 >= 5 {
			tens++
		}
		return Paise(tens * 10 * PaisePerRupee)
	}
	for p := Paise(0); p <= 3_000_00; p++ {
		got, err := MulRatio(p, 1, 1, RoundNearestTenRupees)
		if err != nil || got != statutory(p) {
			t.Fatalf("%d paise: got %d, %v; want %d", p, got, err, statutory(p))
		}
	}
}

func TestParseRoundedCapsDecimalPlaces(t *testing.T) {
	ok := "1." + strings.Repeat("0", MaxRoundedDecimals)
	if got, err := ParseRounded(ok, UnitRupees, RoundHalfUpPaisa); err != nil || got != 100 {
		t.Errorf("ParseRounded(%d places) = %d, %v; want 100", MaxRoundedDecimals, got, err)
	}
	long := "1." + strings.Repeat("0", MaxRoundedDecimals+1)
	var pe *ParseError
	if _, err := ParseRounded(long, UnitRupees, RoundHalfUpPaisa); !errors.As(err, &pe) {
		t.Errorf("ParseRounded(%d places): err = %v; want *ParseError", MaxRoundedDecimals+1, err)
	}
}

func TestParseRoundedOverflow(t *testing.T) {
	if _, err := ParseRounded("92233720368547758.08", UnitRupees, RoundHalfUpPaisa); !errors.Is(err, ErrOverflow) {
		t.Errorf("err = %v; want ErrOverflow", err)
	}
	if _, err := ParseRounded("92233720368547758.075", UnitRupees, RoundHalfUpPaisa); !errors.Is(err, ErrOverflow) {
		t.Errorf("rounding up past MaxPaise: err = %v; want ErrOverflow", err)
	}
}

func TestParseJSONNumberIsStrict(t *testing.T) {
	for _, in := range []string{" 12", "12 ", "\t12", "007", "-01", "00.5"} {
		if got, err := ParseJSONNumber(json.Number(in), UnitRupees); err == nil {
			t.Errorf("ParseJSONNumber(%q) = %d; want an error", in, got)
		}
	}
	for in, want := range map[string]Paise{"0": 0, "-0": 0, "0.5": 50, "-0.05": -5, "10": 1000} {
		if got, err := ParseJSONNumber(json.Number(in), UnitRupees); err != nil || got != want {
			t.Errorf("ParseJSONNumber(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
}

// TestParseRefusesLedgerSideMarkers pins that Dr/Cr and other Tally-style negative
// forms are refused: their sign depends on the ledger, so an adapter must decide.
func TestParseRefusesLedgerSideMarkers(t *testing.T) {
	for _, in := range []string{
		"2,750.00 Dr", "2,750.00 Cr", "2750Dr", "Dr 2750", "(-)2,750", "2,750-", "−2,750",
	} {
		var pe *ParseError
		if got, err := Parse(in, UnitRupees); !errors.As(err, &pe) {
			t.Errorf("Parse(%q) = %d, %v; want *ParseError", in, got, err)
		}
	}
}
