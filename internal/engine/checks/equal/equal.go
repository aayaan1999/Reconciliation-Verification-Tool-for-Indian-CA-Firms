// Package equal is the "equal" check type (§B4.3): it compares inputs a and b, within an
// optional tolerance for money, and fails when they differ by more than the tolerance.
package equal

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
const Name = "equal"

// Check is the equal check type.
type Check struct{}

// Name implements rules.CheckType.
func (Check) Name() string { return Name }

// comparable lists the kinds equal can compare.
var comparable = []domain.ValueKind{domain.KindMoney, domain.KindInt, domain.KindText, domain.KindDate}

// ValidateParams implements rules.CheckType: exactly the inputs a and b, both facts of the
// same comparable kind; a tolerance only for money.
func (Check) ValidateParams(r *rules.Rule, _ *vocab.Vocab) error {
	// Every problem is reported in one load, as the other check types do: an unknown
	// parameter is added to the rest, not returned on its own.
	var errs []error
	if err := rules.OtherParams(r.Check, "tolerance"); err != nil {
		errs = append(errs, err)
	}
	names := make([]string, len(r.Inputs))
	for i, in := range r.Inputs {
		names[i] = in.Name
	}
	if !slices.Equal(names, []string{"a", "b"}) {
		errs = append(errs, fmt.Errorf("needs exactly the inputs a and b, not %s", strings.Join(names, ", ")))
		return errors.Join(errs...)
	}
	a, _ := r.Input("a")
	b, _ := r.Input("b")
	notFact := false
	for _, in := range []rules.Input{a, b} {
		if in.Fact == "" {
			notFact = true
			errs = append(errs, fmt.Errorf("input %s must be a fact, not a record type", in.Name))
		}
	}
	if notFact { // the kinds below are only known for facts
		return errors.Join(errs...)
	}
	switch {
	case a.Kind != b.Kind:
		errs = append(errs, fmt.Errorf("a is %s but b is %s; equal compares figures of the same kind", a.Kind, b.Kind))
	case !slices.Contains(comparable, a.Kind):
		errs = append(errs, fmt.Errorf("equal cannot compare %s figures (it compares money, int, text and date)", a.Kind))
	case r.Check.Tolerance.Kind != rules.ToleranceNone && a.Kind != domain.KindMoney:
		errs = append(errs, fmt.Errorf("a tolerance applies only to money; a and b are %s", a.Kind))
	}
	return errors.Join(errs...)
}

// Evaluate implements rules.Evaluator. Money differs when |a − b| is more than the
// tolerance; other kinds differ when they are not identical. The difference reported is
// |a − b| for money and 0 for other kinds.
func (Check) Evaluate(r *rules.Rule, found map[string]domain.Fact) (rules.Outcome, error) {
	var out rules.Outcome
	for _, in := range r.Inputs { // sorted by name: a, then b
		f, ok := found[in.Name]
		if !ok {
			return out, fmt.Errorf("rule %s: input %s was not supplied", r.ID, in.Name)
		}
		out.Values = append(out.Values, domain.LabeledValue{Input: in.Name, Label: in.Label, Value: f.Value, Locator: f.Locator})
	}
	a, b := out.Values[0].Value, out.Values[1].Value
	if a.Kind != b.Kind {
		return out, fmt.Errorf("rule %s: a is %s but b is %s", r.ID, a.Kind, b.Kind)
	}
	agree := a == b
	if a.Kind == domain.KindMoney {
		d, err := money.Sub(a.Money, b.Money)
		if err == nil {
			d, err = money.Abs(d)
		}
		if err != nil {
			return out, fmt.Errorf("rule %s: difference of %d and %d paise: %w", r.ID, a.Money, b.Money, err)
		}
		out.Difference = d
		agree = rules.WithinTolerance(d, a.Money, b.Money, r.Check.Tolerance)
	}
	out.Status = domain.StatusFail
	if agree {
		out.Status = domain.StatusPass
	}
	return out, nil
}
