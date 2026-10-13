package money

import (
	"errors"
	"testing"
)

func TestAdd(t *testing.T) {
	cases := []struct {
		name    string
		a, b    Paise
		want    Paise
		wantErr bool
	}{
		{"zero", 0, 0, 0, false},
		{"depreciation example", 11245000, 2750 * 100, 11520000, false},
		{"positive and negative", 500, -750, -250, false},
		{"max plus zero", MaxPaise, 0, MaxPaise, false},
		{"max plus one overflows", MaxPaise, 1, 0, true},
		{"max plus max overflows", MaxPaise, MaxPaise, 0, true},
		{"min plus zero", MinPaise, 0, MinPaise, false},
		{"min minus one overflows", MinPaise, -1, 0, true},
		{"min plus min overflows", MinPaise, MinPaise, 0, true},
		{"max plus min is -1", MaxPaise, MinPaise, -1, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Add(c.a, c.b)
			checkResult(t, got, err, c.want, c.wantErr)
		})
	}
}

func TestSub(t *testing.T) {
	cases := []struct {
		name    string
		a, b    Paise
		want    Paise
		wantErr bool
	}{
		{"depreciation difference", 11520000, 11245000, 275000, false},
		{"negative result", 100, 250, -150, false},
		{"zero minus min overflows", 0, MinPaise, 0, true},
		{"max minus -1 overflows", MaxPaise, -1, 0, true},
		{"min minus 1 overflows", MinPaise, 1, 0, true},
		{"min minus -1", MinPaise, -1, MinPaise + 1, false},
		{"-1 minus min is max", -1, MinPaise, MaxPaise, false},
		{"max minus max", MaxPaise, MaxPaise, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Sub(c.a, c.b)
			checkResult(t, got, err, c.want, c.wantErr)
		})
	}
}

func TestNegAbs(t *testing.T) {
	if got, err := Neg(2750); err != nil || got != -2750 {
		t.Errorf("Neg(2750) = %d, %v", got, err)
	}
	if got, err := Neg(MaxPaise); err != nil || got != -MaxPaise {
		t.Errorf("Neg(MaxPaise) = %d, %v", got, err)
	}
	if _, err := Neg(MinPaise); !errors.Is(err, ErrOverflow) {
		t.Errorf("Neg(MinPaise) error = %v, want ErrOverflow", err)
	}
	for _, c := range []struct{ in, want Paise }{{-2750, 2750}, {2750, 2750}, {0, 0}, {MinPaise + 1, MaxPaise}} {
		if got, err := Abs(c.in); err != nil || got != c.want {
			t.Errorf("Abs(%d) = %d, %v; want %d", c.in, got, err, c.want)
		}
	}
	if _, err := Abs(MinPaise); !errors.Is(err, ErrOverflow) {
		t.Errorf("Abs(MinPaise) error = %v, want ErrOverflow", err)
	}
}

func TestSum(t *testing.T) {
	cases := []struct {
		name    string
		in      []Paise
		want    Paise
		wantErr bool
	}{
		{"none", nil, 0, false},
		{"one", []Paise{500}, 500, false},
		{"MAT book profit parts", []Paise{24000000000, 4800000000, -2500000}, 28797500000, false},
		{"mixed signs reaching max", []Paise{MaxPaise, -5, 5}, MaxPaise, false},
		{"running total overflows", []Paise{MaxPaise, 1, -1}, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Sum(c.in...)
			checkResult(t, got, err, c.want, c.wantErr)
		})
	}
}

func checkResult(t *testing.T, got Paise, err error, want Paise, wantErr bool) {
	t.Helper()
	if wantErr {
		if !errors.Is(err, ErrOverflow) {
			t.Fatalf("got %d, %v; want ErrOverflow", got, err)
		}
		return
	}
	if err != nil || got != want {
		t.Fatalf("got %d, %v; want %d", got, err, want)
	}
}
