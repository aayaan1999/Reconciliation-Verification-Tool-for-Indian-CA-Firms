// Package sumequals is the "sum_equals" check type (§B4.3; SPEC-006): it adds up the
// parts, each added or taken away, and compares the sum with the total, within an optional
// tolerance. The finding shows every part, the sum it computed, the total and the
// difference.
//
// On the rows of one record type (SPEC-007) it adds up amounts of each row and compares
// the sum with another amount of the same row, giving one result per row: each block of
// assets in a depreciation schedule, for example.
package sumequals

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// Name is the check type's name in rule files.
const Name = "sum_equals"

// SumLabel labels the sum the check computed; it has no source of its own.
const SumLabel = "Sum of the figures above"

// Check is the sum_equals check type.
type Check struct{}

// Name implements rules.CheckType.
func (Check) Name() string { return Name }

// ValidateParams implements rules.CheckType: at least two parts and a total, every one a
// money fact input, each input used once, and no other input. On rows: exactly one
// record-type input, each part and the total_field an amount of it, each used once.
func (Check) ValidateParams(r *rules.Rule, v *vocab.Vocab) error {
	if len(r.Inputs) == 1 && r.Inputs[0].RecordType != "" {
		return validateRows(r, v)
	}
	c := r.Check
	var errs []error
	if err := rules.OtherParams(c, "tolerance", "parts", "total"); err != nil {
		errs = append(errs, err)
	}
	if len(c.Parts) < 2 {
		errs = append(errs, errors.New("parts must list at least two figures"))
	}
	if c.Total == "" {
		errs = append(errs, errors.New("total must name the input the sum is compared with"))
	}
	for i, p := range c.Parts {
		if p.Field != "" {
			errs = append(errs, fmt.Errorf("parts[%d]: field applies only to rows", i))
		}
	}
	used := map[string]bool{}
	for _, name := range append(partInputs(c), c.Total) {
		if name == "" {
			continue
		}
		if used[name] {
			errs = append(errs, fmt.Errorf("input %s is used twice", name))
		}
		used[name] = true
		in, ok := r.Input(name)
		switch {
		case !ok:
			errs = append(errs, fmt.Errorf("input %s is not one of the rule's inputs", name))
		case in.Fact == "" || in.Kind != domain.KindMoney:
			errs = append(errs, fmt.Errorf("input %s must be a money figure", name))
		}
	}
	for _, in := range r.Inputs {
		if !used[in.Name] {
			errs = append(errs, fmt.Errorf("input %s is neither a part nor the total", in.Name))
		}
	}
	return errors.Join(errs...)
}

func validateRows(r *rules.Rule, v *vocab.Vocab) error {
	c := r.Check
	in := r.Inputs[0]
	var errs []error
	if err := rules.OtherParams(c, "tolerance", "parts", "total", "total_field"); err != nil {
		errs = append(errs, err)
	}
	if len(c.Parts) < 2 {
		errs = append(errs, errors.New("parts must list at least two amounts"))
	}
	if c.Total != in.Name || c.TotalField == "" {
		errs = append(errs, fmt.Errorf("on rows, total names the input %s and total_field the amount the sum is compared with", in.Name))
	}
	rt, _ := v.RecordType(in.RecordType)
	used := map[string]bool{}
	fields := make([]string, 0, len(c.Parts)+1)
	for i, p := range c.Parts {
		if p.Input != in.Name || p.Field == "" {
			errs = append(errs, fmt.Errorf("parts[%d]: on rows, each part names the input %s and a field, one of its amounts", i, in.Name))
			continue
		}
		fields = append(fields, p.Field)
	}
	if c.TotalField != "" {
		fields = append(fields, c.TotalField)
	}
	for _, f := range fields {
		switch {
		case !slices.Contains(rt.Amounts, f):
			errs = append(errs, fmt.Errorf("%q is not an amount of %s", f, rt.Name))
		case used[f]:
			errs = append(errs, fmt.Errorf("amount %s is used twice", f))
		}
		used[f] = true
	}
	return errors.Join(errs...)
}

func partInputs(c rules.Check) []string {
	out := make([]string, len(c.Parts))
	for i, p := range c.Parts {
		out[i] = p.Input
	}
	return out
}

// Evaluate implements rules.Evaluator. The parts are listed in the rule's order, each
// label saying whether it is added or taken away, then the sum, then the total.
func (Check) Evaluate(r *rules.Rule, found map[string]domain.Fact) (rules.Outcome, error) {
	var out rules.Outcome
	var sum money.Paise
	for i, p := range r.Check.Parts {
		f, in, err := input(r, found, p.Input)
		if err != nil {
			return out, err
		}
		if p.Sign > 0 {
			sum, err = money.Add(sum, f.Value.Money)
		} else {
			sum, err = money.Sub(sum, f.Value.Money)
		}
		if err != nil {
			return out, fmt.Errorf("rule %s: sum of the parts: %w", r.ID, err)
		}
		label := in.Label
		switch {
		case p.Sign < 0:
			label = "less: " + label
		case i > 0:
			label = "add: " + label
		}
		out.Values = append(out.Values, domain.LabeledValue{Input: p.Input, Label: label, Value: f.Value, Locator: f.Locator})
	}
	out.Values = append(out.Values, domain.LabeledValue{Input: "sum", Label: SumLabel, Value: domain.MoneyValue(sum)})
	t, in, err := input(r, found, r.Check.Total)
	if err != nil {
		return out, err
	}
	out.Values = append(out.Values, domain.LabeledValue{Input: r.Check.Total, Label: in.Label, Value: t.Value, Locator: t.Locator})
	d, err := money.Sub(sum, t.Value.Money)
	if err == nil {
		d, err = money.Abs(d)
	}
	if err != nil {
		return out, fmt.Errorf("rule %s: difference: %w", r.ID, err)
	}
	out.Difference = d
	out.Status = domain.StatusFail
	if rules.WithinTolerance(d, sum, t.Value.Money, r.Check.Tolerance) {
		out.Status = domain.StatusPass
	}
	return out, nil
}

func input(r *rules.Rule, found map[string]domain.Fact, name string) (domain.Fact, rules.Input, error) {
	in, ok := r.Input(name)
	f, got := found[name]
	if !ok || !got {
		return domain.Fact{}, in, fmt.Errorf("rule %s: input %s was not supplied", r.ID, name)
	}
	if f.Value.Kind != domain.KindMoney {
		return domain.Fact{}, in, fmt.Errorf("rule %s: input %s is %s, not money", r.ID, name, f.Value.Kind)
	}
	return f, in, nil
}

// EvaluateRows implements rules.RowEvaluator: one result for each row, in key order. A
// row missing one of the amounts is Cannot check, naming it; never a finding.
func (Check) EvaluateRows(r *rules.Rule, rows []domain.Record) ([]rules.RecordOutcome, error) {
	in := r.Inputs[0]
	sorted := slices.Clone(rows)
	slices.SortStableFunc(sorted, func(a, b domain.Record) int {
		// Within a key, file order with row numbers read as numbers, so rows sharing a key are
		// told apart the same way on every run ("$[2]" before "$[10]").
		return cmp.Or(cmp.Compare(a.MatchKey, b.MatchKey), domain.CompareLocators(a.Locator, b.Locator))
	})
	var out []rules.RecordOutcome
	for _, row := range sorted {
		o := rules.RecordOutcome{MatchKey: row.MatchKey}
		show := func(field, label string) (money.Paise, bool) {
			a, ok := row.Amounts[field]
			if ok {
				o.Values = append(o.Values, domain.LabeledValue{Input: in.Name + "." + field, Label: label, Value: domain.MoneyValue(a), Locator: row.Locator})
			}
			return a, ok
		}
		var sum money.Paise
		var missing []string
		var err error
		for i, p := range r.Check.Parts {
			label := in.Label + ", " + p.Field
			switch {
			case p.Sign < 0:
				label = "less: " + label
			case i > 0:
				label = "add: " + label
			}
			a, ok := show(p.Field, label)
			if !ok {
				missing = append(missing, p.Field)
				continue
			}
			if p.Sign > 0 {
				sum, err = money.Add(sum, a)
			} else {
				sum, err = money.Sub(sum, a)
			}
			if err != nil {
				return nil, fmt.Errorf("rule %s: row %s: sum of the parts: %w", r.ID, row.MatchKey, err)
			}
		}
		if len(missing) == 0 {
			o.Values = append(o.Values, domain.LabeledValue{Input: "sum", Label: SumLabel, Value: domain.MoneyValue(sum)})
		}
		total, ok := show(r.Check.TotalField, in.Label+", "+r.Check.TotalField)
		if !ok {
			missing = append(missing, r.Check.TotalField)
		}
		if len(missing) > 0 {
			o.Status, o.Missing = domain.StatusCannotCheck, missing
			out = append(out, o)
			continue
		}
		d, err := money.Sub(sum, total)
		if err == nil {
			d, err = money.Abs(d)
		}
		if err != nil {
			return nil, fmt.Errorf("rule %s: row %s: difference: %w", r.ID, row.MatchKey, err)
		}
		o.Difference = d
		o.Status = domain.StatusFail
		if rules.WithinTolerance(d, sum, total, r.Check.Tolerance) {
			o.Status = domain.StatusPass
		}
		out = append(out, o)
	}
	return out, nil
}
