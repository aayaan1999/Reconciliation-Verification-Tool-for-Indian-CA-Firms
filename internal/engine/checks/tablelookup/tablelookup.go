// Package tablelookup is the "table_lookup" check type (§B4.3 domain with a law table;
// SPEC-006): the key inputs select a row of the rule's own table (a small law table kept in
// the rule file, each row with its source, so it works offline), and the value input is
// compared with that row's value. Keys with no row are Cannot check, never a finding.
package tablelookup

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
const Name = "table_lookup"

// Check is the table_lookup check type.
type Check struct{}

// Name implements rules.CheckType.
func (Check) Name() string { return Name }

// ValidateParams implements rules.CheckType: keys are text figures, value is a figure, the
// inputs are exactly those, and every table row has one key per key input and is unique.
func (Check) ValidateParams(r *rules.Rule, _ *vocab.Vocab) error {
	c := r.Check
	var errs []error
	if err := rules.OtherParams(c, "value", "keys", "table"); err != nil {
		errs = append(errs, err)
	}
	if len(c.Keys) == 0 || c.Value == "" || len(c.Table) == 0 {
		return errors.Join(append(errs, errors.New("needs keys, value and a table of at least one row"))...)
	}
	names := append(slices.Clone(c.Keys), c.Value)
	for _, n := range names {
		in, ok := r.Input(n)
		switch {
		case !ok:
			errs = append(errs, fmt.Errorf("input %q is not one of the rule's inputs", n))
		case in.Fact == "":
			errs = append(errs, fmt.Errorf("input %s must be a figure", n))
		case n != c.Value && in.Kind != domain.KindText:
			errs = append(errs, fmt.Errorf("key input %s must be text", n))
		}
	}
	if len(r.Inputs) != len(names) {
		errs = append(errs, errors.New("the inputs must be exactly the keys and the value"))
	}
	seen := map[string]bool{}
	for i, row := range c.Table {
		if len(row.Keys) != len(c.Keys) {
			errs = append(errs, fmt.Errorf("table[%d] has %d keys, want %d", i, len(row.Keys), len(c.Keys)))
			continue
		}
		k := normal(row.Keys)
		if seen[k] {
			errs = append(errs, fmt.Errorf("table[%d] repeats the keys %s", i, strings.Join(row.Keys, ", ")))
		}
		seen[k] = true
	}
	return errors.Join(errs...)
}

func normal(keys []string) string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = strings.ToUpper(strings.TrimSpace(k))
	}
	return strings.Join(out, "|")
}

// Evaluate implements rules.Evaluator.
func (Check) Evaluate(r *rules.Rule, found map[string]domain.Fact) (rules.Outcome, error) {
	var out rules.Outcome
	var keys []string
	for _, n := range append(slices.Clone(r.Check.Keys), r.Check.Value) {
		in, _ := r.Input(n)
		f, ok := found[n]
		if !ok {
			return out, fmt.Errorf("rule %s: input %s was not supplied", r.ID, n)
		}
		if n != r.Check.Value {
			keys = append(keys, f.Value.Text)
		}
		out.Values = append(out.Values, domain.LabeledValue{Input: n, Label: in.Label, Value: f.Value, Locator: f.Locator})
	}
	want := normal(keys)
	i := slices.IndexFunc(r.Check.Table, func(row rules.TableRow) bool { return normal(row.Keys) == want })
	if i < 0 {
		out.Status = domain.StatusCannotCheck
		out.Missing = []string{rules.MissingTablePrefix + strings.Join(keys, "|")}
		return out, nil
	}
	row := r.Check.Table[i]
	out.Values = append(out.Values, domain.LabeledValue{Input: "table", Label: "Rule table: " + row.Source, Value: row.Value})
	out.Status = domain.StatusFail
	if row.Value == found[r.Check.Value].Value {
		out.Status = domain.StatusPass
	}
	return out, nil
}
