package money

import (
	"math/rand"
	"strings"
	"testing"
)

func TestFormatIndianGroupingTable(t *testing.T) {
	cases := []struct {
		p    Paise
		show bool
		want string
	}{
		{0, false, "0"},
		{0, true, "0.00"},
		{1, true, "0.01"},
		{-1, true, "-0.01"},
		{99, true, "0.99"},
		{-50, true, "-0.50"},
		{100, false, "1"},
		{99900, false, "999"},
		{100000, false, "1,000"},
		{999900, false, "9,999"},
		{1000000, false, "10,000"},
		{9999900, false, "99,999"},
		{10000000, false, "1,00,000"},
		{99999900, false, "9,99,999"},
		{100000000, false, "10,00,000"},
		{1000000000, false, "1,00,00,000"},
		{123456789000, false, "1,23,45,67,890"},
		{1234567890, true, "1,23,45,678.90"},
		{-1234567890, true, "-1,23,45,678.90"},
		{-100000, false, "-1,000"},
		// 92233720368547758 rupees, 07 and 08 paise, at the two extremes.
		{MaxPaise, true, "92,23,37,20,36,85,47,758.07"},
		{MinPaise, true, "-92,23,37,20,36,85,47,758.08"},
	}
	for _, c := range cases {
		got, err := FormatIndian(c.p, c.show)
		if err != nil || got != c.want {
			t.Errorf("FormatIndian(%d,%v) = %q,%v; want %q", c.p, c.show, got, err, c.want)
		}
	}
	for _, p := range []Paise{MaxPaise, MinPaise, -1, 1, 101, MinPaise + 1} {
		if got, err := FormatIndian(p, false); err == nil || got != "" {
			t.Errorf("FormatIndian(%d,false) = %q,%v; want empty string and an error because paise would be hidden", p, got, err)
		}
	}
}

func TestFormatParseRoundTripBoundaryAndSeeded(t *testing.T) {
	check := func(p Paise) {
		s, err := FormatIndian(p, true)
		if err != nil {
			t.Fatalf("FormatIndian(%d): %v", p, err)
		}
		got, err := Parse(s, UnitRupees)
		if err != nil || got != p {
			t.Fatalf("round trip %d -> %q -> %d,%v", p, s, got, err)
		}
		if p%PaisePerRupee == 0 {
			w, err := FormatIndian(p, false)
			if err != nil {
				t.Fatalf("FormatIndian(%d,false): %v", p, err)
			}
			got, err := Parse(w, UnitRupees)
			if err != nil || got != p {
				t.Fatalf("whole-rupee round trip %d -> %q -> %d,%v", p, w, got, err)
			}
		}
	}
	for _, p := range boundaryValues {
		check(p)
	}
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 30000; i++ {
		check(Paise(rng.Uint64()))                                      // full int64 range
		check(Paise(rng.Int63n(2_000_000_000_000) - 1_000_000_000_000)) // realistic magnitudes
		check(Paise(rng.Int63n(2000) - 1000))                           // around zero
		check(Paise(rng.Int63n(1_000_000)-500_000) * PaisePerRupee)     // whole rupees, either sign
	}
}

// FormatIndian output is digits with commas only in Indian positions.
func TestFormatIndianShape(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for n := 0; n < 5000; n++ {
		p := Paise(rng.Uint64())
		s, err := FormatIndian(p, true)
		if err != nil {
			t.Fatal(err)
		}
		if (p < 0) != strings.HasPrefix(s, "-") {
			t.Fatalf("%q: sign mismatch for %d", s, p)
		}
		intPart, frac, _ := strings.Cut(strings.TrimPrefix(s, "-"), ".")
		if len(frac) != 2 {
			t.Fatalf("%q: want two paise digits", s)
		}
		groups := strings.Split(intPart, ",")
		for i, g := range groups {
			switch {
			case i == len(groups)-1 && len(groups) > 1 && len(g) != 3:
				t.Fatalf("%q: last group must have 3 digits", s)
			case i > 0 && i < len(groups)-1 && len(g) != 2:
				t.Fatalf("%q: middle groups must have 2 digits", s)
			case i == 0 && (len(g) < 1 || len(g) > 3):
				t.Fatalf("%q: first group size", s)
			}
		}
	}
}

func FuzzFormatRoundTrip(f *testing.F) {
	for _, p := range boundaryValues {
		f.Add(int64(p))
	}
	f.Fuzz(func(t *testing.T, v int64) {
		p := Paise(v)
		s, err := FormatIndian(p, true)
		if err != nil {
			t.Fatal(err)
		}
		back, err := Parse(s, UnitRupees)
		if err != nil || back != p {
			t.Fatalf("%d -> %q -> %d,%v", v, s, back, err)
		}
		w, err := FormatIndian(p, false)
		if (err == nil) != (v%100 == 0) {
			t.Fatalf("FormatIndian(%d,false) = %q,%v", v, w, err)
		}
	})
}
