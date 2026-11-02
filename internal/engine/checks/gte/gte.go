// Package gte is the "gte" check type (§B4.3; SPEC-006): input a against input b. It fails
// when a is lower than b by more than the tolerance; the difference shown is how much lower.
package gte

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// Name is the check type's name in rule files.
const Name = "gte"

// Check is the gte check type.
type Check struct{}

// Name implements rules.CheckType.
func (Check) Name() string { return Name }

// ValidateParams implements rules.CheckType: exactly the inputs a and b, both money facts.
func (Check) ValidateParams(r *rules.Rule, _ *vocab.Vocab) error {
	if err := rules.OtherParams(r.Check, "tolerance"); err != nil {
		return err
	}
	names := make([]string, len(r.Inputs))
	for i, in := range r.Inputs {
		names[i] = in.Name
	}
	if !slices.Equal(names, []string{"a", "b"}) {
		return fmt.Errorf("needs exactly the inputs a and b, not %s", strings.Join(names, ", "))
	}
	var errs []error
	for _, in := range r.Inputs {
		if in.Fact == "" || in.Kind != domain.KindMoney {
			errs = append(errs, fmt.Errorf("input %s must be a money figure", in.Name))
		}
	}
	return errors.Join(errs...)
}

// Evaluate implements rules.Evaluator.
func (Check) Evaluate(r *rules.Rule, found map[string]domain.Fact) (rules.Outcome, error) {
	var out rules.Outcome
	for _, in := range r.Inputs { // a, then b
		f, ok := found[in.Name]
		if !ok || f.Value.Kind != domain.KindMoney {
			return out, fmt.Errorf("rule %s: input %s is missing or not money", r.ID, in.Name)
		}
		out.Values = append(out.Values, domain.LabeledValue{Input: in.Name, Label: in.Label, Value: f.Value, Locator: f.Locator})
	}
	a, b := out.Values[0].Money, out.Values[1].Money
	out.Status = domain.StatusPass
	if a >= b {
		return out, nil
	}
	short, err := money.Sub(b, a)
	if err != nil {
		return out, fmt.Errorf("rule %s: difference: %w", r.ID, err)
	}
	out.Difference = short
	if !rules.WithinTolerance(short, a, b, r.Check.Tolerance) {
		out.Status = domain.StatusFail
	}
	return out, nil
}
