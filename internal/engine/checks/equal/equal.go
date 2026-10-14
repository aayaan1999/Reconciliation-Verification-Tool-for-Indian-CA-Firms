// Package equal is the "equal" check type (§B4.3): inputs a and b must agree, within an
// optional tolerance for money. SPEC-007 adds its parameter validation; its evaluation
// comes with the rule engine in SPEC-008.
package equal

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
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
	var errs []error
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
	for _, in := range []rules.Input{a, b} {
		if in.Fact == "" {
			errs = append(errs, fmt.Errorf("input %s must be a fact, not a record type", in.Name))
		}
	}
	if len(errs) > 0 {
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
