package rules

import (
	"fmt"
	"slices"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// A CheckType is one comparison shape ("equal", "sum_equals", ...) as the rule loader sees
// it: it validates the check block and inputs of every rule that uses it, so a bad rule is
// refused when it loads, never during a run. The engine's check types (internal/engine/checks,
// SPEC-008 onwards) implement it together with their evaluation.
type CheckType interface {
	Name() string
	// ValidateParams returns nil if r's inputs and check parameters suit this check type.
	// Several problems may be returned together with errors.Join; each is reported.
	ValidateParams(r *Rule, v *vocab.Vocab) error
}

// An Evaluator is a check type that can evaluate a rule (SPEC-003). The engine calls it
// only when every input the rule needs is present (a missing input is Cannot check, which
// the engine decides, RU-52), so found holds one fact for every input, by input name.
// Evaluate must be a pure function of its arguments: no clock, no randomness, no I/O.
type Evaluator interface {
	CheckType
	Evaluate(r *Rule, found map[string]domain.Fact) (Outcome, error)
}

// An Outcome is what a check type computed for one subject.
type Outcome struct {
	Status     domain.EvalStatus     // StatusPass or StatusFail; StatusCannotCheck only with Missing
	Values     []domain.LabeledValue // every side, in the rule's input order, with labels and locators
	Difference money.Paise           // the absolute difference for money; 0 otherwise
	// Missing names what the check could not find although every input is present, such
	// as "table:PORTUGAL|12" (no row of the rule's table for those keys). Status is then
	// StatusCannotCheck.
	Missing []string
}

// MissingTablePrefix starts an Outcome.Missing entry for a row the rule's table lacks.
const MissingTablePrefix = "table:"

// MissingRowPrefix starts an Outcome.Missing or RecordOutcome.Missing entry for the one row of a
// record type that a rule names by its key (SPEC-017): "row:gstr3b_table_row|3.1(a)" when no
// row has the key, and the same with "|repeated" added when more than one has it.
const MissingRowPrefix = "row:"

// MissingRowRepeated ends a MissingRowPrefix entry whose key is on more than one row.
const MissingRowRepeated = "|repeated"

// MissingAmountPrefix starts a RecordOutcome.Missing entry for an amount that no row of one
// side holds: "amount:sales_b2b_invoice.invoice_value". The amount is not in that document,
// so it is not compared (never taken as zero).
const MissingAmountPrefix = "amount:"

// A RowEvaluator is a check type that tests each row of one record type on its own
// (SPEC-006), giving one result per row.
type RowEvaluator interface {
	CheckType
	EvaluateRows(r *Rule, rows []domain.Record) ([]RecordOutcome, error)
}

// A TotalsEvaluator is a check type that adds up the rows of one record type and compares the
// sum with a row of another, picked by its key (SPEC-017: sum_equals with rows_where and
// total_key). The engine calls it once per period and scope for which every input has rows,
// giving the rows of each input by input name; when one input has none, the engine reports
// Cannot check itself, naming its record type. It returns one result for each amount
// compared, in the rule's order.
type TotalsEvaluator interface {
	CheckType
	EvaluateTotals(r *Rule, rows map[string][]domain.Record) ([]RecordOutcome, error)
}

// A RecordEvaluator is a check type that compares two kinds of row key by key (SPEC-004
// part 8). The engine calls it once per period and scope for which both inputs have rows;
// when one has none, the engine reports Cannot check itself.
type RecordEvaluator interface {
	CheckType
	EvaluateRecords(r *Rule, a, b []domain.Record) ([]RecordOutcome, error)
}

// A RecordOutcome is the result for one match key.
type RecordOutcome struct {
	MatchKey   string
	Status     domain.EvalStatus
	Code       string                // "MATCHED", "ONLY_IN_A", "ONLY_IN_B", "DUPLICATE_IN_A", "DUPLICATE_IN_B", "AMOUNT_DIFF", "FIELD_DIFF", "KEY_DIFF", "FIELD_REPORTED"
	Values     []domain.LabeledValue // the figures behind the result, each with its source
	Difference money.Paise           // the largest absolute difference among the amounts compared
	Missing    []string              // the amounts a row lacks, with Status StatusCannotCheck (SPEC-007)
}

// CheckTypes is the registry of check types, filled at start-up and read-only afterwards.
type CheckTypes struct {
	byName map[string]CheckType
}

// NewCheckTypes returns a registry holding cts, refusing an empty or repeated name.
func NewCheckTypes(cts ...CheckType) (*CheckTypes, error) {
	r := &CheckTypes{byName: map[string]CheckType{}}
	for _, ct := range cts {
		name := ct.Name()
		if name == "" {
			return nil, fmt.Errorf("rules: a check type has no name")
		}
		if _, dup := r.byName[name]; dup {
			return nil, fmt.Errorf("rules: check type %q is registered twice", name)
		}
		r.byName[name] = ct
	}
	return r, nil
}

// Lookup returns the check type called name.
func (r *CheckTypes) Lookup(name string) (CheckType, bool) {
	ct, ok := r.byName[name]
	return ct, ok
}

// Names returns every registered name, sorted.
func (r *CheckTypes) Names() []string {
	names := make([]string, 0, len(r.byName))
	for n := range r.byName {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}
