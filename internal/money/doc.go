// Package money handles every rupee amount as an exact whole number of paise (Paise, int64).
// Floating point is never used for money. Arithmetic reports overflow instead of wrapping.
package money
