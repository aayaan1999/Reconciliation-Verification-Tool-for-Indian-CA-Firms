package sumequals

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// Totals of rows (SPEC-017). A part is the sum of an amount over every row of a record type
// that meets conditions (rows_where), and the total is one row of another record type, named
// by its match key (total_key). Either one amount is compared (each part's field and
// total_field), or each of compare_amounts in turn, giving one result per amount, so a split
// between tax heads shows. A figure with no rows is Cannot check, never zero.

// rowConditions are the conditions that can select rows by their text columns.
var rowConditions = []string{rules.IsBlank, rules.IsNotBlank, rules.IsEquals, rules.IsMatches, rules.IsIn, rules.IsNotIn}

func validateTotals(r *rules.Rule, v *vocab.Vocab) error {
	c := r.Check
	var errs []error
	if err := rules.OtherParams(c, "tolerance", "parts", "total", "total_key", "total_field", "compare_amounts"); err != nil {
		errs = append(errs, err)
	}
	for _, in := range r.Inputs {
		if in.RecordType == "" {
			errs = append(errs, fmt.Errorf("input %s must be a record type: totals of rows add up rows", in.Name))
		}
	}
	totalIn, ok := r.Input(c.Total)
	if !ok || totalIn.RecordType == "" {
		return errors.Join(append(errs, errors.New("total must name the record-type input holding the row the sum is compared with"))...)
	}
	totalRT, _ := v.RecordType(totalIn.RecordType)
	if len(c.Parts) == 0 {
		errs = append(errs, errors.New("parts must list at least one sum of rows"))
	}
	byAmounts := len(c.CompareAmounts) > 0
	used := map[string]bool{}
	for i, p := range c.Parts {
		in, ok := r.Input(p.Input)
		switch {
		case !ok || in.RecordType == "":
			errs = append(errs, fmt.Errorf("parts[%d]: input %s is not a record-type input of the rule", i, p.Input))
			continue
		case p.Input == c.Total:
			errs = append(errs, fmt.Errorf("parts[%d]: the input %s holding the total cannot also be added up", i, p.Input))
			continue
		}
		used[p.Input] = true
		rt, _ := v.RecordType(in.RecordType)
		if len(p.Where) == 0 {
			errs = append(errs, fmt.Errorf("parts[%d]: rows_where must name the rows by at least one condition", i))
		}
		for j, w := range p.Where {
			switch {
			case !slices.Contains(rowConditions, w.Is):
				errs = append(errs, fmt.Errorf("parts[%d].rows_where[%d]: %q cannot select rows (use one of: %s)", i, j, w.Is, strings.Join(rowConditions, ", ")))
			case !slices.Contains(rt.Fields, w.Field):
				errs = append(errs, fmt.Errorf("parts[%d].rows_where[%d]: %q is not a column of %s", i, j, w.Field, rt.Name))
			}
		}
		switch {
		case byAmounts && p.Field != "":
			errs = append(errs, fmt.Errorf("parts[%d]: with compare_amounts a part has no field", i))
		case !byAmounts && !slices.Contains(rt.Amounts, p.Field):
			errs = append(errs, fmt.Errorf("parts[%d]: %q is not an amount of %s", i, p.Field, rt.Name))
		}
		if byAmounts {
			for _, a := range c.CompareAmounts {
				if !slices.Contains(rt.Amounts, a) {
					errs = append(errs, fmt.Errorf("parts[%d]: compare_amounts names %q, which is not an amount of %s", i, a, rt.Name))
				}
			}
		}
	}
	if byAmounts {
		if c.TotalField != "" {
			errs = append(errs, errors.New("with compare_amounts the total is compared in the same amount: total_field is not used"))
		}
		seen := map[string]bool{}
		for _, a := range c.CompareAmounts {
			if seen[a] {
				errs = append(errs, fmt.Errorf("compare_amounts lists %s twice", a))
			}
			seen[a] = true
			if !slices.Contains(totalRT.Amounts, a) {
				errs = append(errs, fmt.Errorf("compare_amounts names %q, which is not an amount of %s", a, totalRT.Name))
			}
		}
	} else if !slices.Contains(totalRT.Amounts, c.TotalField) {
		errs = append(errs, fmt.Errorf("total_field %q is not an amount of %s", c.TotalField, totalRT.Name))
	}
	for _, in := range r.Inputs {
		if in.Name != c.Total && !used[in.Name] {
			errs = append(errs, fmt.Errorf("input %s is neither added up nor the total", in.Name))
		}
	}
	return errors.Join(errs...)
}

// EvaluateTotals implements rules.TotalsEvaluator.
func (Check) EvaluateTotals(r *rules.Rule, rows map[string][]domain.Record) ([]rules.RecordOutcome, error) {
	c := r.Check
	totalIn, _ := r.Input(c.Total)
	// The total's row: the one row of its record type that has the key.
	var keyed []domain.Record
	for _, row := range rows[c.Total] {
		if row.MatchKey == c.TotalKey {
			keyed = append(keyed, row)
		}
	}
	amounts := c.CompareAmounts
	if len(amounts) == 0 {
		amounts = []string{c.TotalField}
	}
	var out []rules.RecordOutcome
	for _, amount := range amounts {
		o := rules.RecordOutcome{MatchKey: amount, Status: domain.StatusCannotCheck}
		var sum money.Paise
		var missing []string
		for _, p := range c.Parts {
			in, _ := r.Input(p.Input)
			field := p.Field
			if field == "" {
				field = amount
			}
			picked := selectRows(rows[p.Input], p.Where)
			if len(picked) == 0 {
				missing = appendOnce(missing, in.RecordType)
				continue
			}
			for _, row := range picked {
				a, ok := row.Amounts[field]
				if !ok {
					missing = appendOnce(missing, rules.MissingAmountPrefix+in.RecordType+"."+field)
					continue
				}
				label := fmt.Sprintf("%s, %s (%s)", in.Label, humanName(field), row.MatchKey)
				var err error
				if p.Sign < 0 {
					sum, err = money.Sub(sum, a)
					label = "less: " + label
				} else {
					sum, err = money.Add(sum, a)
					if len(o.Values) > 0 {
						label = "add: " + label
					}
				}
				if err != nil {
					return nil, fmt.Errorf("rule %s: sum of the rows of %s: %w", r.ID, in.RecordType, err)
				}
				o.Values = append(o.Values, domain.LabeledValue{Input: p.Input + "." + field, Label: label, Value: domain.MoneyValue(a), Locator: row.Locator})
			}
		}
		totalField := c.TotalField
		if len(c.CompareAmounts) > 0 {
			totalField = amount
		}
		var total money.Paise
		switch len(keyed) {
		case 0:
			missing = appendOnce(missing, rules.MissingRowPrefix+totalIn.RecordType+"|"+c.TotalKey)
		case 1:
			a, ok := keyed[0].Amounts[totalField]
			if !ok {
				missing = appendOnce(missing, rules.MissingAmountPrefix+totalIn.RecordType+"."+totalField)
			}
			total = a
		default:
			missing = appendOnce(missing, rules.MissingRowPrefix+totalIn.RecordType+"|"+c.TotalKey+rules.MissingRowRepeated)
		}
		if len(missing) > 0 {
			o.Missing = missing
			out = append(out, o)
			continue
		}
		o.Values = append(o.Values,
			domain.LabeledValue{Input: "sum", Label: SumLabel, Value: domain.MoneyValue(sum)},
			domain.LabeledValue{Input: c.Total + "." + totalField, Label: fmt.Sprintf("%s, %s (%s)", totalIn.Label, humanName(totalField), c.TotalKey),
				Value: domain.MoneyValue(total), Locator: keyed[0].Locator})
		d, err := money.Sub(sum, total)
		if err == nil {
			d, err = money.Abs(d)
		}
		if err != nil {
			return nil, fmt.Errorf("rule %s: difference of %s: %w", r.ID, amount, err)
		}
		o.Difference = d
		o.Status = domain.StatusFail
		if rules.WithinTolerance(d, sum, total, c.Tolerance) {
			o.Status = domain.StatusPass
		}
		out = append(out, o)
	}
	return out, nil
}

// selectRows returns the rows that meet every condition, in a fixed order: by match key, then
// by place in the file with row numbers read as numbers.
func selectRows(rows []domain.Record, where []rules.Condition) []domain.Record {
	var picked []domain.Record
	for _, row := range rows {
		if slices.IndexFunc(where, func(w rules.Condition) bool { return !w.Holds(domain.TextValue(row.Fields[w.Field])) }) < 0 {
			picked = append(picked, row)
		}
	}
	slices.SortStableFunc(picked, func(a, b domain.Record) int {
		return cmp.Or(cmp.Compare(a.MatchKey, b.MatchKey), domain.CompareLocators(a.Locator, b.Locator))
	})
	return picked
}

func appendOnce(list []string, s string) []string {
	if slices.Contains(list, s) {
		return list
	}
	return append(list, s)
}

// humanName writes an amount name as words: "taxable_value" is "taxable value".
func humanName(s string) string { return strings.ReplaceAll(s, "_", " ") }
