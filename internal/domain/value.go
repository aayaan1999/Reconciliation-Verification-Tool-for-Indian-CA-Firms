package domain

import "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"

// ValueKind says which field of a Value holds the figure. The zero value is invalid,
// so every fact and every rule input must name its kind.
type ValueKind uint8

const (
	// KindMoney is an amount in paise (Value.Money).
	KindMoney ValueKind = iota + 1
	// KindInt is a whole number that is not money, such as a count (Value.Int).
	KindInt
	// KindText is text, such as a code or a name (Value.Text).
	KindText
	// KindDate is an ISO date "2025-07-31" (Value.Date).
	KindDate
	// KindBool is yes or no (Value.Bool).
	KindBool
	// KindRate is a rate in basis points, 1% = 100 (Value.RateBP).
	KindRate
)

var valueKindNames = [...]string{
	KindMoney: "money",
	KindInt:   "int",
	KindText:  "text",
	KindDate:  "date",
	KindBool:  "bool",
	KindRate:  "rate",
}

// Valid reports whether k is one of the defined kinds.
func (k ValueKind) Valid() bool { return k >= KindMoney && k <= KindRate }

// String returns the canonical name used in JSON and rule files ("money", "int", ...),
// or "invalid" for an undefined kind.
func (k ValueKind) String() string {
	if !k.Valid() {
		return "invalid"
	}
	return valueKindNames[k]
}

// ParseValueKind returns the kind named s ("money", "int", ...), as written in vocabulary
// and rule files. It reports false for any other text, including "invalid".
func ParseValueKind(s string) (ValueKind, bool) {
	for k := KindMoney; k <= KindRate; k++ {
		if valueKindNames[k] == s {
			return k, true
		}
	}
	return 0, false
}

// Value is one figure of a known kind. Only the field named by Kind is meaningful; the
// others are ignored by every comparison, encoding and fingerprint.
type Value struct {
	Kind   ValueKind
	Money  money.Paise
	Int    int64
	Text   string
	Date   string // ISO "2025-07-31"
	Bool   bool
	RateBP int64 // basis points, 1% = 100
}

// MoneyValue returns a money Value.
func MoneyValue(p money.Paise) Value { return Value{Kind: KindMoney, Money: p} }

// IntValue returns a whole-number Value.
func IntValue(n int64) Value { return Value{Kind: KindInt, Int: n} }

// TextValue returns a text Value.
func TextValue(s string) Value { return Value{Kind: KindText, Text: s} }

// DateValue returns a date Value from an ISO date string.
func DateValue(iso string) Value { return Value{Kind: KindDate, Date: iso} }

// BoolValue returns a yes/no Value.
func BoolValue(b bool) Value { return Value{Kind: KindBool, Bool: b} }

// RateValue returns a rate Value in basis points.
func RateValue(bp int64) Value { return Value{Kind: KindRate, RateBP: bp} }
