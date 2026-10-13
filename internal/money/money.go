package money

import (
	"errors"
	"math"
)

// Paise is an amount of Indian money in paise (₹1 = 100 paise).
// Every amount in Tool 1 is a Paise; floating point is never used for money.
type Paise int64

// PaisePerRupee is the number of paise in one rupee.
const PaisePerRupee = 100

// MaxPaise and MinPaise are the largest and smallest representable amounts
// (about ±₹9.2×10^16, roughly ₹922 crore crore), far beyond any real figure.
const (
	MaxPaise Paise = math.MaxInt64
	MinPaise Paise = math.MinInt64
)

// ErrOverflow is returned when a result does not fit in a Paise. Arithmetic never
// wraps around silently.
var ErrOverflow = errors.New("money: amount out of range")

// Add returns a + b, or ErrOverflow.
func Add(a, b Paise) (Paise, error) {
	s := a + b
	// Overflow happened if both operands have the same sign and the sum's sign differs.
	if (a >= 0) == (b >= 0) && (s >= 0) != (a >= 0) {
		return 0, ErrOverflow
	}
	return s, nil
}

// Sub returns a − b, or ErrOverflow.
func Sub(a, b Paise) (Paise, error) {
	d := a - b
	// Overflow happened if the operands have different signs and the result's sign differs from a.
	if (a >= 0) != (b >= 0) && (d >= 0) != (a >= 0) {
		return 0, ErrOverflow
	}
	return d, nil
}

// Neg returns −a, or ErrOverflow for MinPaise (which has no positive counterpart).
func Neg(a Paise) (Paise, error) {
	if a == MinPaise {
		return 0, ErrOverflow
	}
	return -a, nil
}

// Abs returns |a|, or ErrOverflow for MinPaise.
func Abs(a Paise) (Paise, error) {
	if a < 0 {
		return Neg(a)
	}
	return a, nil
}

// Sum returns the total of amounts (0 for none), or ErrOverflow if any running
// total goes out of range.
func Sum(amounts ...Paise) (Paise, error) {
	var total Paise
	for _, a := range amounts {
		var err error
		if total, err = Add(total, a); err != nil {
			return 0, err
		}
	}
	return total, nil
}
