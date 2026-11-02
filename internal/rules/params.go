package rules

import (
	"fmt"
	"math/big"
	"regexp"
	"slices"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
)

// Parameters of the check types added in SPEC-006. Each check type validates its own and
// refuses, with OtherParams, those of every other check type.

// A Part is one figure of a sum_equals, or of the base of a percent_of: added (Sign +1)
// or taken away (Sign −1). On rows (SPEC-007) Field names the amount of each row.
type Part struct {
	Input string
	Field string
	Sign  int
}

// Conditions a present_together input can meet.
const (
	IsBlank    = "blank"     // text "" (or only spaces)
	IsNotBlank = "not_blank" // text with something in it
	IsZero     = "zero"      // money or int 0
	IsNonzero  = "nonzero"   // money or int other than 0
	IsEquals   = "equals"    // text equal to Value, ignoring case and surrounding spaces
	IsMatches  = "matches"   // text in which the regular expression Value finds a match
	IsIn       = "in"        // text equal to one of Values (ignoring case and surrounding spaces)
	IsNotIn    = "not_in"    // text equal to none of Values
	IsTrue     = "true"      // bool true
	IsFalse    = "false"     // bool false
	IsSameAs   = "same_as"   // a figure equal to the figure of the input named in Value (SPEC-006 part 2)
)

// ConditionNames are the conditions, in the order they are documented.
var ConditionNames = []string{IsBlank, IsNotBlank, IsZero, IsNonzero, IsEquals, IsMatches, IsIn, IsNotIn, IsTrue, IsFalse, IsSameAs}

// A Condition is one test of present_together: on a fact input, or on a column (Field) of
// each row of a record-type input.
type Condition struct {
	Input  string
	Field  string // the column, for a record-type input
	Is     string
	Value  string
	Values []string
	Re     *regexp.Regexp // for IsMatches
}

// Holds reports whether v meets c. It is false for a value of a kind c does not test.
func (c Condition) Holds(v domain.Value) bool {
	text := strings.TrimSpace(v.Text)
	switch c.Is {
	case IsBlank:
		return v.Kind == domain.KindText && text == ""
	case IsNotBlank:
		return v.Kind == domain.KindText && text != ""
	case IsZero:
		return (v.Kind == domain.KindMoney && v.Money == 0) || (v.Kind == domain.KindInt && v.Int == 0)
	case IsNonzero:
		return (v.Kind == domain.KindMoney && v.Money != 0) || (v.Kind == domain.KindInt && v.Int != 0)
	case IsEquals:
		return v.Kind == domain.KindText && strings.EqualFold(text, strings.TrimSpace(c.Value))
	case IsMatches:
		return v.Kind == domain.KindText && c.Re != nil && c.Re.MatchString(v.Text)
	case IsIn, IsNotIn:
		if v.Kind != domain.KindText {
			return false
		}
		in := slices.ContainsFunc(c.Values, func(s string) bool { return strings.EqualFold(text, strings.TrimSpace(s)) })
		return in == (c.Is == IsIn)
	case IsTrue:
		return v.Kind == domain.KindBool && v.Bool
	case IsFalse:
		return v.Kind == domain.KindBool && !v.Bool
	}
	return false
}

// Text describes the condition for finding text: "is blank", "is one of Loan, Deposit".
func (c Condition) Text() string {
	switch c.Is {
	case IsBlank:
		return "is blank"
	case IsNotBlank:
		return "is filled in"
	case IsZero:
		return "is nil"
	case IsNonzero:
		return "is not nil"
	case IsEquals:
		return fmt.Sprintf("is %q", c.Value)
	case IsMatches:
		return fmt.Sprintf("contains %q", strings.TrimPrefix(c.Value, "(?i)"))
	case IsIn:
		return "is one of " + strings.Join(c.Values, ", ")
	case IsNotIn:
		return "is none of " + strings.Join(c.Values, ", ")
	case IsTrue:
		return "is yes"
	case IsFalse:
		return "is no"
	case IsSameAs:
		return "the same as the next figure"
	}
	return c.Is
}

// A TableRow is one row of a table_lookup's own table: the key values, the value, and
// where the figure comes from (a treaty article, a schedule of the Act).
type TableRow struct {
	Keys   []string
	Value  domain.Value
	Source string
}

// paramNames lists the check parameters set in c, by their name in rule files.
func paramNames(c Check) []string {
	var set []string
	add := func(name string, on bool) {
		if on {
			set = append(set, name)
		}
	}
	add("tolerance", c.Tolerance.Kind != ToleranceNone)
	add("compare_amounts", len(c.CompareAmounts) > 0)
	add("compare_fields", len(c.CompareFields) > 0)
	add("duplicates_in", len(c.DuplicatesIn) > 0)
	add("invoice_amounts", len(c.InvoiceAmounts) > 0)
	add("pair_unmatched_on", len(c.PairUnmatchedOn) > 0)
	add("parts", len(c.Parts) > 0)
	add("total", c.Total != "")
	add("total_field", c.TotalField != "")
	add("base", c.Base != "")
	add("rate", c.RateSet)
	add("value", c.Value != "")
	add("when", len(c.When) > 0)
	add("keys", len(c.Keys) > 0)
	add("table", len(c.Table) > 0)
	return set
}

// OtherParams returns an error naming every parameter set in c that is not in own: each
// check type uses only its own.
func OtherParams(c Check, own ...string) error {
	var extra []string
	for _, n := range paramNames(c) {
		if !slices.Contains(own, n) {
			extra = append(extra, n)
		}
	}
	if len(extra) == 0 {
		return nil
	}
	return fmt.Errorf("%s does not use %s", c.Type, strings.Join(extra, ", "))
}

// WithinTolerance reports whether the absolute difference d of a and b is within t,
// computed in integers: { paise: N } allows d <= N; { bp_of_larger: N } allows
// d x 10000 <= N x the larger of |a| and |b|; no tolerance allows only 0.
func WithinTolerance(d, a, b money.Paise, t Tolerance) bool {
	switch t.Kind {
	case TolerancePaise:
		return int64(d) <= t.Value
	case ToleranceBPOfLarger:
		larger := new(big.Int).Abs(big.NewInt(int64(a)))
		if bb := new(big.Int).Abs(big.NewInt(int64(b))); bb.Cmp(larger) > 0 {
			larger = bb
		}
		lhs := new(big.Int).Mul(big.NewInt(int64(d)), big.NewInt(MaxBasisPoints))
		return lhs.Cmp(new(big.Int).Mul(big.NewInt(t.Value), larger)) <= 0
	}
	return d == 0
}
