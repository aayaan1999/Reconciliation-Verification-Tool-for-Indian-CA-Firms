package money

import (
	"errors"
	"math"
	"math/big"
	"math/rand"
	"testing"
)

// refRound is an independent reference using exact rationals.
func refRound(p, num, den int64, mode Rounding) (Paise, error) {
	x := new(big.Rat).SetFrac(new(big.Int).Mul(big.NewInt(p), big.NewInt(num)), big.NewInt(den))
	step := int64(1)
	switch mode {
	case RoundHalfUpRupee:
		step = 100
	case RoundNearestTenRupees:
		step = 1000
	}
	x.Quo(x, new(big.Rat).SetInt64(step))
	neg := x.Sign() < 0
	x.Abs(x)
	fl := new(big.Int).Quo(x.Num(), x.Denom())
	frac := new(big.Rat).Sub(x, new(big.Rat).SetInt(fl))
	switch mode {
	case RoundNone:
		if frac.Sign() != 0 {
			return 0, ErrInexact
		}
	case RoundDownPaisa:
	case RoundUpPaisa:
		if frac.Sign() != 0 {
			fl.Add(fl, big.NewInt(1))
		}
	default:
		if frac.Cmp(big.NewRat(1, 2)) >= 0 {
			fl.Add(fl, big.NewInt(1))
		}
	}
	if neg {
		fl.Neg(fl)
	}
	fl.Mul(fl, big.NewInt(step))
	if !fl.IsInt64() {
		return 0, ErrOverflow
	}
	return Paise(fl.Int64()), nil
}

var allModes = []Rounding{RoundNone, RoundHalfUpPaisa, RoundHalfUpRupee, RoundNearestTenRupees, RoundDownPaisa, RoundUpPaisa}

func TestMulRatioMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(99))
	check := func(p Paise, num, den int64) {
		for _, m := range allModes {
			want, wantErr := refRound(int64(p), num, den, m)
			got, err := MulRatio(p, num, den, m)
			if wantErr != nil {
				if !errors.Is(err, wantErr) {
					t.Fatalf("MulRatio(%d,%d,%d,mode %d) err = %v, want %v", p, num, den, m, err, wantErr)
				}
				continue
			}
			if err != nil || got != want {
				t.Fatalf("MulRatio(%d,%d,%d,mode %d) = %d,%v; want %d", p, num, den, m, got, err, want)
			}
		}
	}
	for i := 0; i < 20000; i++ {
		check(Paise(rng.Int63n(2_000_001)-1_000_000), rng.Int63n(2001)-1000, rng.Int63n(40)+1)
		check(Paise(rng.Int63n(2_000_001)-1_000_000), rng.Int63n(2001)-1000, -(rng.Int63n(40) + 1))
		check(Paise(rng.Int63n(2_000_001)-1_000_000), rng.Int63n(100), 100)
	}
	for _, p := range boundaryValues {
		for _, num := range []int64{0, 1, -1, 2, 1800, math.MaxInt64, math.MinInt64} {
			for _, den := range []int64{1, -1, 2, 10000, math.MaxInt64, math.MinInt64} {
				check(p, num, den)
			}
		}
	}
}

func TestMulRatioExtremes(t *testing.T) {
	if got, err := MulRatio(MaxPaise, math.MaxInt64, math.MaxInt64, RoundNone); err != nil || got != MaxPaise {
		t.Errorf("Max*Max/Max = %d,%v", got, err)
	}
	if got, err := MulRatio(MinPaise, math.MinInt64, math.MinInt64, RoundNone); err != nil || got != MinPaise {
		t.Errorf("Min*Min/Min = %d,%v", got, err)
	}
	if _, err := MulRatio(MinPaise, -1, 1, RoundNone); !errors.Is(err, ErrOverflow) {
		t.Errorf("MinPaise * -1 err = %v, want ErrOverflow", err)
	}
	if _, err := MulRatio(MinPaise, 1, -1, RoundNone); !errors.Is(err, ErrOverflow) {
		t.Errorf("MinPaise / -1 err = %v, want ErrOverflow", err)
	}
	if _, err := MulRatio(MaxPaise, 2, 1, RoundNone); !errors.Is(err, ErrOverflow) {
		t.Errorf("Max*2 err = %v, want ErrOverflow", err)
	}
	if _, err := MulRate(MaxPaise, 10001, RoundHalfUpPaisa); !errors.Is(err, ErrOverflow) {
		t.Errorf("Max*100.01%% err = %v, want ErrOverflow", err)
	}
	if got, err := MulRate(MaxPaise, 10000, RoundNone); err != nil || got != MaxPaise {
		t.Errorf("Max*100%% = %d,%v", got, err)
	}
	if got, err := MulRate(MinPaise, 10000, RoundNone); err != nil || got != MinPaise {
		t.Errorf("Min*100%% = %d,%v", got, err)
	}
	// Max is ...807 paise, so the nearest rupee is 7 paise lower.
	if got, err := MulRatio(MaxPaise, 1, 1, RoundHalfUpRupee); err != nil || got != MaxPaise-7 {
		t.Errorf("Max to nearest rupee = %d,%v; want %d", got, err, MaxPaise-7)
	}
	// Max ends in 807 paise, so the nearest 10 rupees is above the limit.
	if _, err := MulRatio(MaxPaise, 1, 1, RoundNearestTenRupees); !errors.Is(err, ErrOverflow) {
		t.Errorf("rounding past the limit err = %v, want ErrOverflow", err)
	}
	if _, err := MulRatio(1, 1, 0, RoundNone); err == nil {
		t.Error("zero denominator accepted")
	}
	if _, err := MulRatio(0, 5, 0, RoundHalfUpPaisa); err == nil {
		t.Error("zero denominator with zero amount accepted")
	}
	if got, err := MulRate(0, 1800, RoundHalfUpPaisa); err != nil || got != 0 {
		t.Errorf("0*18%% = %d,%v", got, err)
	}
}

func TestMulRateHalfPaisaCases(t *testing.T) {
	cases := []struct {
		p    Paise
		bp   int64
		mode Rounding
		want Paise
	}{
		{5, 1000, RoundHalfUpPaisa, 1},
		{-5, 1000, RoundHalfUpPaisa, -1},
		{5, 1000, RoundDownPaisa, 0},
		{-5, 1000, RoundDownPaisa, 0},
		{5, 1000, RoundUpPaisa, 1},
		{-5, 1000, RoundUpPaisa, -1},
		{4, 1000, RoundHalfUpPaisa, 0},
		{4, 1000, RoundUpPaisa, 1},
		{-4, 1000, RoundUpPaisa, -1},
		{6, 1000, RoundHalfUpPaisa, 1},
		{1, 5000, RoundHalfUpPaisa, 1},
		{3, 5000, RoundHalfUpPaisa, 2},
		{-3, 5000, RoundHalfUpPaisa, -2},
		{499, 10000, RoundHalfUpRupee, 500},
		{450, 10000, RoundHalfUpRupee, 500},
		{449, 10000, RoundHalfUpRupee, 400},
		{-450, 10000, RoundHalfUpRupee, -500},
		{-449, 10000, RoundHalfUpRupee, -400},
		{500, 10000, RoundNearestTenRupees, 1000},
		{499, 10000, RoundNearestTenRupees, 0},
		{-500, 10000, RoundNearestTenRupees, -1000},
		{-499, 10000, RoundNearestTenRupees, 0},
		{1500, 10000, RoundNearestTenRupees, 2000},
		{1499, 10000, RoundNearestTenRupees, 1000},
		{999, 10000, RoundNearestTenRupees, 1000},
	}
	for _, c := range cases {
		got, err := MulRate(c.p, c.bp, c.mode)
		if err != nil || got != c.want {
			t.Errorf("MulRate(%d,%d,mode %d) = %d,%v; want %d", c.p, c.bp, c.mode, got, err, c.want)
		}
	}
}

func TestRoundNoneExactAndInexact(t *testing.T) {
	if got, err := MulRate(1000, 1800, RoundNone); err != nil || got != 180 {
		t.Errorf("10.00 * 18%% = %d,%v; want 180", got, err)
	}
	for _, p := range []Paise{1, -1} {
		if _, err := MulRate(p, 1800, RoundNone); !errors.Is(err, ErrInexact) {
			t.Errorf("%d paisa * 18%% err = %v, want ErrInexact", p, err)
		}
	}
	if got, err := MulRatio(0, 1, 3, RoundNone); err != nil || got != 0 {
		t.Errorf("0/3 = %d,%v", got, err)
	}
	if got, err := MulRatio(9, 1, 3, RoundNone); err != nil || got != 3 {
		t.Errorf("9/3 = %d,%v", got, err)
	}
}

func TestInvalidRoundingValues(t *testing.T) {
	for _, m := range []Rounding{0, -1, 7, 100} {
		if _, err := MulRatio(100, 1, 1, m); !errors.Is(err, ErrRounding) {
			t.Errorf("MulRatio mode %d err = %v, want ErrRounding", m, err)
		}
		if _, err := MulRate(100, 1, m); !errors.Is(err, ErrRounding) {
			t.Errorf("MulRate mode %d err = %v, want ErrRounding", m, err)
		}
	}
}

func FuzzMulRatio(f *testing.F) {
	f.Add(int64(12345), int64(1800), int64(10000), uint8(2))
	f.Add(int64(math.MinInt64), int64(-1), int64(1), uint8(1))
	f.Fuzz(func(t *testing.T, p, num, den int64, m uint8) {
		mode := Rounding(m % 8)
		got, err := MulRatio(Paise(p), num, den, mode)
		if den == 0 || mode == 0 || mode > RoundUpPaisa {
			if err == nil {
				t.Fatalf("MulRatio(%d,%d,%d,%d) accepted", p, num, den, mode)
			}
			return
		}
		want, wantErr := refRound(p, num, den, mode)
		if wantErr != nil {
			if !errors.Is(err, wantErr) {
				t.Fatalf("MulRatio(%d,%d,%d,%d) err = %v, want %v", p, num, den, mode, err, wantErr)
			}
			return
		}
		if err != nil || got != want {
			t.Fatalf("MulRatio(%d,%d,%d,%d) = %d,%v; want %d", p, num, den, mode, got, err, want)
		}
	})
}
