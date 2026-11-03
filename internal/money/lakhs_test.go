package money

import "testing"

// Amounts written in lakhs or crores are read exactly, as whole paise, without floating point.
func TestParseLakhsAndCrores(t *testing.T) {
	cases := []struct {
		in   string
		unit Unit
		want Paise
	}{
		{"1", UnitLakhs, 10_000_000},
		{"12.5", UnitLakhs, 125_000_000},
		{"0.0000001", UnitLakhs, 1},        // the smallest amount: one paisa
		{"(2.25)", UnitLakhs, -22_500_000}, // brackets still mean negative
		{"-0.5", UnitLakhs, -5_000_000},
		{"1,234.5", UnitLakhs, 12_345_000_000},
		{"1", UnitCrores, 1_000_000_000},
		{"1.25", UnitCrores, 1_250_000_000},
		{"0.000000001", UnitCrores, 1},
	}
	for _, c := range cases {
		got, err := Parse(c.in, c.unit)
		if err != nil || got != c.want {
			t.Errorf("Parse(%q, %d) = %d, %v; want %d", c.in, c.unit, got, err, c.want)
		}
	}
}

func TestParseLakhsRefusesAPartOfAPaisa(t *testing.T) {
	for _, c := range []struct {
		in   string
		unit Unit
	}{{"0.00000001", UnitLakhs}, {"1.0000000001", UnitCrores}} {
		if got, err := Parse(c.in, c.unit); err == nil {
			t.Errorf("Parse(%q, %d) = %d, want a refusal: it is a part of a paisa", c.in, c.unit, got)
		}
	}
	if _, err := Parse("1", Unit(99)); err == nil {
		t.Error("an unknown unit was accepted")
	}
}

func TestParseRoundedLakhs(t *testing.T) {
	// Eight decimal places of a lakh is a tenth of a paisa: rounded half up it is zero or one paisa.
	if got, err := ParseRounded("0.00000005", UnitLakhs, RoundHalfUpPaisa); err != nil || got != 1 {
		t.Errorf("ParseRounded half up = %d, %v; want 1", got, err)
	}
	if got, err := ParseRounded("0.00000004", UnitLakhs, RoundHalfUpPaisa); err != nil || got != 0 {
		t.Errorf("ParseRounded below half = %d, %v; want 0", got, err)
	}
	if got, err := ParseRounded("12.5", UnitLakhs, RoundNone); err != nil || got != 125_000_000 {
		t.Errorf("ParseRounded exact = %d, %v", got, err)
	}
}
