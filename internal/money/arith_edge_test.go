package money

import (
	"errors"
	"math"
	"math/big"
	"math/rand"
	"testing"
)

// boundaryValues are amounts around the limits of int64 and around zero.
var boundaryValues = []Paise{
	MinPaise, MinPaise + 1, MinPaise + 100, -PaisePerRupee, -1, 0, 1, PaisePerRupee,
	MaxPaise - 100, MaxPaise - 1, MaxPaise, math.MaxInt64 / 2, math.MinInt64 / 2,
}

func TestAddSubMatchBigIntOracle(t *testing.T) {
	check := func(a, b Paise) {
		wantAdd := new(big.Int).Add(big.NewInt(int64(a)), big.NewInt(int64(b)))
		got, err := Add(a, b)
		if wantAdd.IsInt64() {
			if err != nil || int64(got) != wantAdd.Int64() {
				t.Fatalf("Add(%d,%d) = %d,%v; want %s", a, b, got, err, wantAdd)
			}
		} else if !errors.Is(err, ErrOverflow) {
			t.Fatalf("Add(%d,%d) = %d,%v; want ErrOverflow (true sum %s)", a, b, got, err, wantAdd)
		}
		wantSub := new(big.Int).Sub(big.NewInt(int64(a)), big.NewInt(int64(b)))
		got, err = Sub(a, b)
		if wantSub.IsInt64() {
			if err != nil || int64(got) != wantSub.Int64() {
				t.Fatalf("Sub(%d,%d) = %d,%v; want %s", a, b, got, err, wantSub)
			}
		} else if !errors.Is(err, ErrOverflow) {
			t.Fatalf("Sub(%d,%d) = %d,%v; want ErrOverflow (true difference %s)", a, b, got, err, wantSub)
		}
	}
	for _, a := range boundaryValues {
		for _, b := range boundaryValues {
			check(a, b)
		}
	}
	rng := rand.New(rand.NewSource(20260101))
	for i := 0; i < 20000; i++ {
		check(Paise(rng.Uint64()), Paise(rng.Uint64()))
		check(Paise(rng.Int63n(1_000_000)-500_000), Paise(rng.Uint64()))
	}
}

func TestSumEdgeCases(t *testing.T) {
	cases := []struct {
		name    string
		in      []Paise
		want    Paise
		wantErr error
	}{
		{"nil", nil, 0, nil},
		{"max alone", []Paise{MaxPaise}, MaxPaise, nil},
		{"min alone", []Paise{MinPaise}, MinPaise, nil},
		{"max plus min is -1", []Paise{MaxPaise, MinPaise}, -1, nil},
		{"min plus max is -1", []Paise{MinPaise, MaxPaise}, -1, nil},
		{"max plus one", []Paise{MaxPaise, 1}, 0, ErrOverflow},
		{"min minus one", []Paise{MinPaise, -1}, 0, ErrOverflow},
		{"running total overflows although final total fits", []Paise{MaxPaise, 1, -1}, 0, ErrOverflow},
		{"running total underflows although final total fits", []Paise{MinPaise, -1, 1}, 0, ErrOverflow},
		{"exactly max by parts", []Paise{MaxPaise - 5, 3, 2}, MaxPaise, nil},
		{"one past max by parts", []Paise{MaxPaise - 5, 3, 3}, 0, ErrOverflow},
		{"many small values", []Paise{1, 2, 3, 4, 5, -15}, 0, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Sum(c.in...)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("err = %v, want %v", err, c.wantErr)
			}
			if got != c.want {
				t.Fatalf("Sum = %d, want %d", got, c.want)
			}
		})
	}
}

func TestNegAbsExtremes(t *testing.T) {
	if _, err := Neg(MinPaise); !errors.Is(err, ErrOverflow) {
		t.Errorf("Neg(MinPaise) err = %v, want ErrOverflow", err)
	}
	if _, err := Abs(MinPaise); !errors.Is(err, ErrOverflow) {
		t.Errorf("Abs(MinPaise) err = %v, want ErrOverflow", err)
	}
	if got, err := Neg(MinPaise + 1); err != nil || got != MaxPaise {
		t.Errorf("Neg(MinPaise+1) = %d,%v", got, err)
	}
	if got, err := Neg(MaxPaise); err != nil || got != MinPaise+1 {
		t.Errorf("Neg(MaxPaise) = %d,%v", got, err)
	}
	if got, err := Neg(0); err != nil || got != 0 {
		t.Errorf("Neg(0) = %d,%v", got, err)
	}
	if got, err := Abs(MaxPaise); err != nil || got != MaxPaise {
		t.Errorf("Abs(MaxPaise) = %d,%v", got, err)
	}
	if got, err := Abs(MinPaise + 1); err != nil || got != MaxPaise {
		t.Errorf("Abs(MinPaise+1) = %d,%v", got, err)
	}
	if _, err := Sub(0, MinPaise); !errors.Is(err, ErrOverflow) {
		t.Errorf("Sub(0, MinPaise) err = %v, want ErrOverflow", err)
	}
}

func TestConstants(t *testing.T) {
	if MaxPaise != math.MaxInt64 || MinPaise != math.MinInt64 || PaisePerRupee != 100 {
		t.Fatal("constants changed")
	}
}
