// Package percentof is the "percent_of" check type (§B4.3 formula; SPEC-006): rate per cent
// of the base input, rounded to the rupee (half away from zero), against the value input,
// within an optional tolerance. The finding shows the base, the figure computed, the value
// and the difference. All arithmetic is in integers.
//
// The base may instead be a sum of parts, each added or taken away (SPEC-007): cess on tax
// plus surcharge, for example. The finding then lists the parts and their sum.
package percentof

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// Name is the check type's name in rule files.
const Name = "percent_of"

// sumLabel labels the sum of the parts, as sum_equals labels its sum.
const sumLabel = "Sum of the figures above"

// Check is the percent_of check type.
type Check struct{}

// Name implements rules.CheckType.
func (Check) Name() string { return Name }

// ValidateParams implements rules.CheckType: rate and value set, and either base or at
// least two parts; every input a different money figure, and no other input.
func (Check) ValidateParams(r *rules.Rule, _ *vocab.Vocab) error {
	c := r.Check
	var errs []error
	if err := rules.OtherParams(c, "tolerance", "base", "parts", "rate", "value"); err != nil {
		errs = append(errs, err)
	}
	if !c.RateSet {
		errs = append(errs, errors.New("rate must give the percentage, such as \"15.00\""))
	}
	names := []string{c.Base}
	switch {
	case c.Base != "" && len(c.Parts) > 0:
		errs = append(errs, errors.New("the base is either one input (base) or a sum (parts), not both"))
	case c.Base == "" && len(c.Parts) < 2:
		errs = append(errs, errors.New("base must name an input, or parts must list at least two"))
	case c.Base == "":
		names = names[:0]
		for i, p := range c.Parts {
			if p.Field != "" {
				errs = append(errs, fmt.Errorf("parts[%d]: field applies only to rows", i))
			}
			names = append(names, p.Input)
		}
	}
	if c.Value == "" {
		errs = append(errs, errors.New("value must name the input compared"))
	}
	names = append(names, c.Value)
	seen := map[string]bool{}
	for _, name := range names {
		if name == "" {
			continue
		}
		if seen[name] {
			errs = append(errs, fmt.Errorf("input %s is used twice", name))
		}
		seen[name] = true
		if in, ok := r.Input(name); !ok || in.Fact == "" || in.Kind != domain.KindMoney {
			errs = append(errs, fmt.Errorf("input %s must be a money figure of the rule", name))
		}
	}
	if len(r.Inputs) != len(names) {
		errs = append(errs, errors.New("the inputs must be exactly the base (or its parts) and the value"))
	}
	return errors.Join(errs...)
}

// Percent returns rateBP basis points of base, rounded to the rupee, half away from zero.
func Percent(base money.Paise, rateBP int64) (money.Paise, error) {
	n := new(big.Int).Mul(big.NewInt(int64(base)), big.NewInt(rateBP))
	den := big.NewInt(rules.MaxBasisPoints * money.PaisePerRupee) // to rupees
	neg := n.Sign() < 0
	n.Abs(n)
	q, rem := new(big.Int).QuoRem(n, den, new(big.Int))
	if new(big.Int).Mul(rem, big.NewInt(2)).Cmp(den) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	q.Mul(q, big.NewInt(money.PaisePerRupee))
	if neg {
		q.Neg(q)
	}
	if !q.IsInt64() {
		return 0, errors.New("the result is too large")
	}
	return money.Paise(q.Int64()), nil
}

// Evaluate implements rules.Evaluator.
func (Check) Evaluate(r *rules.Rule, found map[string]domain.Fact) (rules.Outcome, error) {
	var out rules.Outcome
	fig := func(name string) (domain.Fact, rules.Input, error) {
		in, _ := r.Input(name)
		f, ok := found[name]
		if !ok || f.Value.Kind != domain.KindMoney {
			return f, in, fmt.Errorf("rule %s: input %s is missing or not money", r.ID, name)
		}
		return f, in, nil
	}
	parts := r.Check.Parts
	if r.Check.Base != "" {
		parts = []rules.Part{{Input: r.Check.Base, Sign: 1}}
	}
	var base money.Paise
	for i, p := range parts {
		f, in, err := fig(p.Input)
		if err != nil {
			return out, err
		}
		if p.Sign > 0 {
			base, err = money.Add(base, f.Value.Money)
		} else {
			base, err = money.Sub(base, f.Value.Money)
		}
		if err != nil {
			return out, fmt.Errorf("rule %s: sum of the parts: %w", r.ID, err)
		}
		label := in.Label
		switch {
		case len(parts) == 1:
		case p.Sign < 0:
			label = "less: " + label
		case i > 0:
			label = "add: " + label
		}
		out.Values = append(out.Values, domain.LabeledValue{Input: p.Input, Label: label, Value: f.Value, Locator: f.Locator})
	}
	if len(parts) > 1 {
		out.Values = append(out.Values, domain.LabeledValue{Input: "sum", Label: sumLabel, Value: domain.MoneyValue(base)})
	}
	v, vIn, err := fig(r.Check.Value)
	if err != nil {
		return out, err
	}
	computed, err := Percent(base, r.Check.RateBP)
	if err != nil {
		return out, fmt.Errorf("rule %s: %w", r.ID, err)
	}
	rate := domain.FormatValue(domain.Value{Kind: domain.KindRate, RateBP: r.Check.RateBP})
	out.Values = append(out.Values,
		domain.LabeledValue{Input: "computed", Label: fmt.Sprintf("%s%% of the figure above, to the rupee", rate), Value: domain.MoneyValue(computed)},
		domain.LabeledValue{Input: vIn.Name, Label: vIn.Label, Value: v.Value, Locator: v.Locator},
	)
	d, err := money.Sub(computed, v.Value.Money)
	if err == nil {
		d, err = money.Abs(d)
	}
	if err != nil {
		return out, fmt.Errorf("rule %s: difference: %w", r.ID, err)
	}
	out.Difference = d
	out.Status = domain.StatusFail
	if rules.WithinTolerance(d, computed, v.Value.Money, r.Check.Tolerance) {
		out.Status = domain.StatusPass
	}
	return out, nil
}
