package money

import (
	"errors"
	"fmt"
	"math/big"
)

// Rounding names how a result that is not a whole number of the target unit is rounded.
// There is no default: the zero value is invalid, so every rule must choose one.
// "Half up" means a result exactly halfway goes away from zero (−₹5 to the nearest
// ₹10 becomes −₹10).
type Rounding int

const (
	// RoundNone requires an exact whole number of paise; any remainder is an error.
	RoundNone Rounding = iota + 1
	// RoundHalfUpPaisa rounds to the nearest paisa.
	RoundHalfUpPaisa
	// RoundHalfUpRupee rounds to the nearest rupee.
	RoundHalfUpRupee
	// RoundNearestTenRupees rounds to the nearest ₹10, ₹5 or more going up. For a
	// positive amount this equals the Income-tax Act rounding of income and tax
	// (ss.288A/288B: paise are first ignored, then a last digit of 5 or more rounds
	// up), because ignoring the paise cannot carry an amount across the ₹5 mark.
	// Negative amounts round half away from zero; that this matches the Act for
	// refunds is not yet confirmed.
	RoundNearestTenRupees
	// RoundDownPaisa drops fractions of a paisa (towards zero).
	RoundDownPaisa
	// RoundUpPaisa raises any fraction of a paisa to the next paisa (away from zero).
	RoundUpPaisa
)

// BasisPointsPerUnit is the number of basis points in 100%: a rate of 10% is 1000 bp.
const BasisPointsPerUnit = 10000

// ErrInexact is returned with RoundNone when a result has a fraction of a paisa.
var ErrInexact = errors.New("money: result is not a whole number of paise and rounding was not allowed")

// ErrRounding is returned for an unknown Rounding value.
var ErrRounding = errors.New("money: unknown rounding method")

// ErrZeroDenominator is returned by MulRatio when den is zero.
var ErrZeroDenominator = errors.New("money: ratio with a zero denominator")

// roundingNames are the spellings used in rules, profiles and law tables.
var roundingNames = map[Rounding]string{
	RoundNone:             "none",
	RoundHalfUpPaisa:      "half_up_paisa",
	RoundHalfUpRupee:      "half_up_rupee",
	RoundNearestTenRupees: "nearest_10_rupees",
	RoundDownPaisa:        "down_paisa",
	RoundUpPaisa:          "up_paisa",
}

// String returns the name a rule uses for mode, such as "half_up_rupee".
func (mode Rounding) String() string {
	if name, ok := roundingNames[mode]; ok {
		return name
	}
	return fmt.Sprintf("Rounding(%d)", int(mode))
}

// ParseRounding reads a rounding name as written in a rule, such as "nearest_10_rupees".
// An unknown name returns an error wrapping ErrRounding; it never falls back to a default.
func ParseRounding(name string) (Rounding, error) {
	for mode := RoundNone; mode <= RoundUpPaisa; mode++ {
		if roundingNames[mode] == name {
			return mode, nil
		}
	}
	return 0, fmt.Errorf("%w: %q", ErrRounding, name)
}

// MulRate returns p × rateBP / 10000, rounded by mode. A rate of 10% is 1000 basis points.
func MulRate(p Paise, rateBP int64, mode Rounding) (Paise, error) {
	return MulRatio(p, rateBP, BasisPointsPerUnit, mode)
}

// MulRatio returns p × num / den, rounded by mode. den must not be zero.
func MulRatio(p Paise, num, den int64, mode Rounding) (Paise, error) {
	if den == 0 {
		return 0, ErrZeroDenominator
	}
	n := new(big.Int).Mul(big.NewInt(int64(p)), big.NewInt(num))
	d := big.NewInt(den)
	if den < 0 {
		n.Neg(n)
		d.Neg(d)
	}
	return divRound(n, d, mode)
}

// ParseRounded is Parse for amounts that may carry more decimal places than the unit
// allows (for example "11,245.055" rupees); the extra precision is rounded by mode.
// At most MaxRoundedDecimals decimal places are accepted.
func ParseRounded(text string, unit Unit, mode Rounding) (Paise, error) {
	if _, ok := roundingStep(mode); !ok {
		return 0, ErrRounding
	}
	return parse(text, unit, true, mode)
}

// roundingStep returns the unit, in paise, that mode rounds to.
func roundingStep(mode Rounding) (int64, bool) {
	switch mode {
	case RoundNone, RoundHalfUpPaisa, RoundDownPaisa, RoundUpPaisa:
		return 1, true
	case RoundHalfUpRupee:
		return PaisePerRupee, true
	case RoundNearestTenRupees:
		return 10 * PaisePerRupee, true
	}
	return 0, false
}

// divRound returns num/den paise rounded by mode. den must be positive.
func divRound(num, den *big.Int, mode Rounding) (Paise, error) {
	step, ok := roundingStep(mode)
	if !ok {
		return 0, ErrRounding
	}
	d := new(big.Int).Mul(den, big.NewInt(step))
	q, r := new(big.Int).QuoRem(num, d, new(big.Int)) // q truncated towards zero; r has num's sign
	if r.Sign() != 0 {
		awayFromZero := big.NewInt(int64(num.Sign()))
		switch mode {
		case RoundNone:
			return 0, ErrInexact
		case RoundDownPaisa:
			// already truncated towards zero
		case RoundUpPaisa:
			q.Add(q, awayFromZero)
		default: // half up
			twice := new(big.Int).Abs(r)
			twice.Lsh(twice, 1)
			if twice.Cmp(d) >= 0 {
				q.Add(q, awayFromZero)
			}
		}
	}
	q.Mul(q, big.NewInt(step))
	if !q.IsInt64() {
		return 0, ErrOverflow
	}
	return Paise(q.Int64()), nil
}

// roundedFromDigits converts digits with scale decimal places (in paise) to Paise
// using mode; used by ParseRounded.
func roundedFromDigits(input, digits string, scale int, negative bool, mode Rounding) (Paise, error) {
	num, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		return 0, &ParseError{Input: input, Reason: "contains characters that are not digits"}
	}
	if negative {
		num.Neg(num)
	}
	den := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
	p, err := divRound(num, den, mode)
	if err != nil {
		return 0, fmt.Errorf("money: %q: %w", input, err)
	}
	return p, nil
}
