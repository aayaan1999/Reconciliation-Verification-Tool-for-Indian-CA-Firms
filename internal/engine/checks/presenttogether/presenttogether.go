// Package presenttogether is the "present_together" check type (§B4.3 field_present and
// domain; SPEC-006). Each condition tests one figure, or one column of each row of a
// record type, against a fixed test: blank, nil, a text, a pattern, a list of codes, yes or
// no. The check fails (a finding) when every condition holds at once.
//
// It never judges what a value means. The finding only states which values were found
// together, each with its source (CLAUDE.md invariant 1); what follows from them is for the
// reviewer and for the wording the CA approves.
package presenttogether

import (
	"errors"
	"fmt"
	"slices"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// Name is the check type's name in rule files.
const Name = "present_together"

// Check is the present_together check type.
type Check struct{}

// Name implements rules.CheckType.
func (Check) Name() string { return Name }

// kinds lists the value kinds each condition tests.
var kinds = map[string][]domain.ValueKind{
	rules.IsBlank: {domain.KindText}, rules.IsNotBlank: {domain.KindText},
	rules.IsEquals: {domain.KindText}, rules.IsMatches: {domain.KindText},
	rules.IsIn: {domain.KindText}, rules.IsNotIn: {domain.KindText},
	rules.IsZero: {domain.KindMoney, domain.KindInt}, rules.IsNonzero: {domain.KindMoney, domain.KindInt},
	rules.IsTrue: {domain.KindBool}, rules.IsFalse: {domain.KindBool},
	rules.IsSameAs: {domain.KindMoney, domain.KindInt, domain.KindText, domain.KindDate},
}

// ValidateParams implements rules.CheckType. On figures: every input is a fact tested by at
// least one condition that suits its kind. On rows: exactly one record-type input, every
// condition names one of its columns (a text column, or an amount for zero and nonzero).
func (Check) ValidateParams(r *rules.Rule, v *vocab.Vocab) error {
	c := r.Check
	var errs []error
	if err := rules.OtherParams(c, "when"); err != nil {
		errs = append(errs, err)
	}
	if len(c.When) == 0 {
		return errors.Join(append(errs, errors.New("when must list at least one condition"))...)
	}
	rows := len(r.Inputs) == 1 && r.Inputs[0].RecordType != ""
	tested := map[string]bool{}
	for i, w := range c.When {
		in, ok := r.Input(w.Input)
		if !ok {
			errs = append(errs, fmt.Errorf("when[%d]: input %q is not one of the rule's inputs", i, w.Input))
			continue
		}
		tested[w.Input] = true
		if rows && w.Is == rules.IsSameAs {
			errs = append(errs, fmt.Errorf("when[%d]: same_as compares figures, not columns of rows", i))
			continue
		}
		if rows {
			rt, _ := v.RecordType(in.RecordType)
			switch {
			case w.Field == "":
				errs = append(errs, fmt.Errorf("when[%d]: a condition on rows names a field, the column it tests", i))
			case slices.Contains(rt.Fields, w.Field):
				if !slices.Contains(kinds[w.Is], domain.KindText) {
					errs = append(errs, fmt.Errorf("when[%d]: %q tests text, not %s", i, w.Field, w.Is))
				}
			case slices.Contains(rt.Amounts, w.Field):
				if w.Is != rules.IsZero && w.Is != rules.IsNonzero {
					errs = append(errs, fmt.Errorf("when[%d]: %q is an amount; only zero and nonzero test amounts", i, w.Field))
				}
			default:
				errs = append(errs, fmt.Errorf("when[%d]: %q is not a column of %s", i, w.Field, rt.Name))
			}
			continue
		}
		if w.Is == rules.IsSameAs {
			other, ok := r.Input(w.Value)
			switch {
			case !ok || other.Name == in.Name:
				errs = append(errs, fmt.Errorf("when[%d]: same_as names another input of the rule, not %q", i, w.Value))
			case other.Fact == "" || other.Kind != in.Kind:
				errs = append(errs, fmt.Errorf("when[%d]: %s and %s must be figures of the same kind", i, in.Name, other.Name))
			default:
				tested[other.Name] = true
			}
		}
		switch {
		case in.Fact == "":
			errs = append(errs, fmt.Errorf("when[%d]: input %s must be a figure; rows are tested with a single record-type input", i, in.Name))
		case w.Field != "":
			errs = append(errs, fmt.Errorf("when[%d]: field applies only to rows", i))
		case !slices.Contains(kinds[w.Is], in.Kind):
			errs = append(errs, fmt.Errorf("when[%d]: %s cannot test the %s figure %s", i, w.Is, in.Kind, in.Name))
		}
	}
	for _, in := range r.Inputs {
		if !tested[in.Name] {
			errs = append(errs, fmt.Errorf("input %s is not tested by any condition", in.Name))
		}
	}
	return errors.Join(errs...)
}

func label(in rules.Input, w rules.Condition) string {
	return in.Label + " (tested: " + w.Text() + ")"
}

// Evaluate implements rules.Evaluator: one result for the figures. Each figure is shown
// once, with the conditions tested on it; a figure named by same_as follows the one compared.
func (Check) Evaluate(r *rules.Rule, found map[string]domain.Fact) (rules.Outcome, error) {
	out := rules.Outcome{Status: domain.StatusFail}
	shown := map[string]bool{}
	show := func(name, lbl string) error {
		if shown[name] {
			return nil
		}
		f, ok := found[name]
		if !ok {
			return fmt.Errorf("rule %s: input %s was not supplied", r.ID, name)
		}
		shown[name] = true
		out.Values = append(out.Values, domain.LabeledValue{Input: name, Label: lbl, Value: f.Value, Locator: f.Locator})
		return nil
	}
	for _, w := range r.Check.When {
		in, _ := r.Input(w.Input)
		if err := show(w.Input, label(in, w)); err != nil {
			return out, err
		}
		holds := w.Holds(found[w.Input].Value)
		if w.Is == rules.IsSameAs {
			other, _ := r.Input(w.Value)
			if err := show(w.Value, other.Label); err != nil {
				return out, err
			}
			holds = found[w.Input].Value == found[w.Value].Value
		}
		if !holds {
			out.Status = domain.StatusPass
		}
	}
	return out, nil
}

// EvaluateRows implements rules.RowEvaluator: one result for each row, in key order.
func (Check) EvaluateRows(r *rules.Rule, rows []domain.Record) ([]rules.RecordOutcome, error) {
	in := r.Inputs[0]
	sorted := slices.Clone(rows)
	slices.SortStableFunc(sorted, func(a, b domain.Record) int {
		if a.MatchKey != b.MatchKey {
			if a.MatchKey < b.MatchKey {
				return -1
			}
			return 1
		}
		if a.Locator.Path < b.Locator.Path {
			return -1
		}
		if a.Locator.Path > b.Locator.Path {
			return 1
		}
		return 0
	})
	var out []rules.RecordOutcome
	for _, row := range sorted {
		o := rules.RecordOutcome{MatchKey: row.MatchKey, Status: domain.StatusFail}
		for _, w := range r.Check.When {
			var val domain.Value
			if a, isAmount := row.Amounts[w.Field]; isAmount {
				val = domain.MoneyValue(a)
			} else {
				val = domain.TextValue(row.Fields[w.Field])
			}
			o.Values = append(o.Values, domain.LabeledValue{Input: in.Name + "." + w.Field, Label: in.Label + ", " + w.Field + " (tested: " + w.Text() + ")",
				Value: val, Locator: row.Locator})
			if !w.Holds(val) {
				o.Status = domain.StatusPass
			}
		}
		out = append(out, o)
	}
	return out, nil
}
