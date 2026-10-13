package money

import (
	"math/rand/v2"
	"testing"
)

func TestFormatIndian(t *testing.T) {
	cases := []struct {
		p         Paise
		showPaise bool
		want      string
	}{
		// The requirements §6.9 example figures.
		{11245000, false, "1,12,450"},
		{11520000, false, "1,15,200"},
		{275000, false, "2,750"},
		{11245000, true, "1,12,450.00"},
		{11245050, true, "1,12,450.50"},
		{-275000, false, "-2,750"},
		{-275000, true, "-2,750.00"},
		{50, true, "0.50"},
		{-50, true, "-0.50"},
		{5, true, "0.05"},
		{0, false, "0"},
		{0, true, "0.00"},
		{100, false, "1"},
		{99900, false, "999"},
		{100000, false, "1,000"},
		{1000000, false, "10,000"},
		{10000000, false, "1,00,000"},
		{12345678900, false, "12,34,56,789"},
		{123456789000, false, "1,23,45,67,890"},
		{MaxPaise, true, "92,23,37,20,36,85,47,758.07"},
		{MinPaise, true, "-92,23,37,20,36,85,47,758.08"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			got, err := FormatIndian(c.p, c.showPaise)
			if err != nil || got != c.want {
				t.Fatalf("FormatIndian(%d, %v) = %q, %v; want %q", c.p, c.showPaise, got, err, c.want)
			}
		})
	}
}

func TestFormatIndianRefusesHiddenPaise(t *testing.T) {
	for _, p := range []Paise{11245050, -1, 1, MaxPaise, MinPaise} {
		if got, err := FormatIndian(p, false); err == nil {
			t.Errorf("FormatIndian(%d, false) = %q; want an error, the paise would be hidden", p, got)
		}
	}
}

// TestFormatParseRoundTrip writes thousands of amounts and reads them back; every one
// must come back exactly. The generator has a fixed seed, so the test is repeatable.
func TestFormatParseRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewPCG(2025, 2026))
	values := []Paise{0, 1, -1, 99, 100, -100, MaxPaise, MinPaise, MaxPaise - 1, MinPaise + 1}
	for i := 0; i < 20000; i++ {
		v := Paise(rng.Int64())
		if rng.IntN(2) == 0 {
			v = -v
		}
		// Mix in amounts of everyday size as well as huge ones.
		if rng.IntN(2) == 0 {
			v %= 10_000_000_000 // up to ₹10 crore
		}
		values = append(values, v)
	}
	for _, v := range values {
		text, err := FormatIndian(v, true)
		if err != nil {
			t.Fatalf("FormatIndian(%d): %v", v, err)
		}
		back, err := Parse(text, UnitRupees)
		if err != nil || back != v {
			t.Fatalf("round trip %d -> %q -> %d, %v", v, text, back, err)
		}
		if v%PaisePerRupee == 0 {
			whole, err := FormatIndian(v, false)
			if err != nil {
				t.Fatalf("FormatIndian(%d, false): %v", v, err)
			}
			if back, err := Parse(whole, UnitRupees); err != nil || back != v {
				t.Fatalf("whole-rupee round trip %d -> %q -> %d, %v", v, whole, back, err)
			}
		}
	}
}
