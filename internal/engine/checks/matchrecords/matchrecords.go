// Package matchrecords is the "match_records" check type (§B4.3; SPEC-004 part 8): it
// compares two kinds of row key by key, for example the B2B invoices of the sales register
// with those reported in GSTR-1. For every key it reports one result: matched; only in one
// input; duplicated in an input whose rows are whole invoices; amounts that differ beyond
// the tolerance; other columns that differ; or, for a row only in a and a row only in b
// that agree on everything else, an invoice number written differently. It never guesses:
// a row with more than one possible partner is left unmatched.
package matchrecords

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// Name is the check type's name in rule files.
const Name = "match_records"

// Result codes.
const (
	Matched       = "MATCHED"
	OnlyInA       = "ONLY_IN_A"
	OnlyInB       = "ONLY_IN_B"
	DuplicateA    = "DUPLICATE_IN_A"
	DuplicateB    = "DUPLICATE_IN_B"
	AmountDiff    = "AMOUNT_DIFF"
	FieldDiff     = "FIELD_DIFF"
	KeyDiff       = "KEY_DIFF"
	LinesDiffer   = "LINES_DIFFER"
	FieldReported = "FIELD_REPORTED" // SPEC-018: a matched pair whose listed column holds a listed value
	inputA        = "a"
	inputB        = "b"
	keyAmount     = "invoice_value" // shown for a matched key, when compared
	duplicateOf   = "row %d of %d"
)

// Check is the match_records check type.
type Check struct{}

// Name implements rules.CheckType.
func (Check) Name() string { return Name }

// ValidateParams implements rules.CheckType: exactly the inputs a and b, both record types
// with the same match key; the compared amounts and fields, and the pairing fields, exist
// in both; duplicates_in names a or b.
func (Check) ValidateParams(r *rules.Rule, v *vocab.Vocab) error {
	names := make([]string, len(r.Inputs))
	for i, in := range r.Inputs {
		names[i] = in.Name
	}
	if !slices.Equal(names, []string{inputA, inputB}) {
		return fmt.Errorf("needs exactly the inputs a and b, not %s", strings.Join(names, ", "))
	}
	a, _ := r.Input(inputA)
	b, _ := r.Input(inputB)
	if a.RecordType == "" || b.RecordType == "" {
		return errors.New("inputs a and b must be record types (rows), not facts")
	}
	ra, _ := v.RecordType(a.RecordType)
	rb, _ := v.RecordType(b.RecordType)
	var errs []error
	if !slices.Equal(ra.MatchKey, rb.MatchKey) {
		errs = append(errs, fmt.Errorf("%s is keyed on %s but %s on %s; rows can be matched only on the same key",
			ra.Name, strings.Join(ra.MatchKey, "+"), rb.Name, strings.Join(rb.MatchKey, "+")))
	}
	c := r.Check
	if err := rules.OtherParams(c, "tolerance", "compare_amounts", "compare_fields", "duplicates_in", "invoice_amounts", "pair_unmatched_on", "report_missing", "report_fields"); err != nil {
		errs = append(errs, err)
	}
	if len(c.CompareAmounts) == 0 && c.Tolerance.Kind != rules.ToleranceNone {
		errs = append(errs, errors.New("a tolerance needs compare_amounts: with no amounts the keys alone are matched"))
	}
	for _, n := range c.CompareAmounts {
		if !slices.Contains(ra.Amounts, n) || !slices.Contains(rb.Amounts, n) {
			errs = append(errs, fmt.Errorf("compare_amounts: %q is not an amount of both %s and %s", n, ra.Name, rb.Name))
		}
	}
	for _, list := range []struct {
		key   string
		names []string
	}{{"compare_fields", c.CompareFields}, {"pair_unmatched_on", c.PairUnmatchedOn}} {
		for _, n := range list.names {
			switch {
			case !slices.Contains(ra.Fields, n) || !slices.Contains(rb.Fields, n):
				errs = append(errs, fmt.Errorf("%s: %q is not a field of both %s and %s", list.key, n, ra.Name, rb.Name))
			case list.key == "compare_fields" && slices.Contains(ra.MatchKey, n):
				errs = append(errs, fmt.Errorf("%s: %q is part of the match key, so it is always equal for a matched key", list.key, n))
			}
		}
	}
	for _, n := range c.InvoiceAmounts {
		if !slices.Contains(ra.Amounts, n) || !slices.Contains(rb.Amounts, n) {
			errs = append(errs, fmt.Errorf("invoice_amounts: %q is not an amount of both %s and %s", n, ra.Name, rb.Name))
		}
	}
	for _, d := range c.DuplicatesIn {
		if d != inputA && d != inputB {
			errs = append(errs, fmt.Errorf("duplicates_in: %q must be a or b", d))
		}
	}
	errs = append(errs, validateReporting(r, ra, rb)...)
	return errors.Join(errs...)
}

// side is the rows of one input for one key, added together.
type side struct {
	key      string
	rows     []domain.Record
	amounts  map[string]money.Paise
	has      map[string]bool // the amounts some row of the whole document holds (shared by its sides)
	fields   map[string]string
	disagree []string // invoice-level amounts whose lines show different values
}

func group(rows []domain.Record) map[string]*side {
	out := map[string]*side{}
	for _, r := range rows {
		s := out[r.MatchKey]
		if s == nil {
			s = &side{key: r.MatchKey, amounts: map[string]money.Paise{}, fields: map[string]string{}}
			out[r.MatchKey] = s
		}
		s.rows = append(s.rows, r)
	}
	// File order, with row numbers read as numbers ("$[2]" before "$[10]"), so "line N" and
	// the key's locator follow the file.
	for _, s := range out {
		slices.SortStableFunc(s.rows, func(x, y domain.Record) int { return domain.CompareLocators(x.Locator, y.Locator) })
	}
	return out
}

// total adds the rows' amounts together and joins their distinct field values. An
// invoice-level amount (repeated on every line of the invoice) is taken once, from the
// first line that holds it; lines that leave it out are not counted against it. If the
// lines that hold it disagree, the amount is noted in disagree and not used.
func (s *side) total(invoiceAmounts []string) error {
	values := map[string][]string{}
	seen := map[string]bool{} // invoice-level amounts already taken from a line
	for _, r := range s.rows {
		// Sorted, so the order of disagree (and the amount a LINES_DIFFER finding shows) is the
		// same on every run.
		for _, k := range slices.Sorted(maps.Keys(r.Amounts)) {
			v := r.Amounts[k]
			if slices.Contains(invoiceAmounts, k) {
				switch {
				case !seen[k]:
					s.amounts[k], seen[k] = v, true
				case s.amounts[k] != v && !slices.Contains(s.disagree, k):
					s.disagree = append(s.disagree, k)
				}
				continue
			}
			sum, err := money.Add(s.amounts[k], v)
			if err != nil {
				return fmt.Errorf("key %s: %s: %w", s.key, k, err)
			}
			s.amounts[k] = sum
		}
		for k, v := range r.Fields {
			if !slices.Contains(values[k], v) {
				values[k] = append(values[k], v)
			}
		}
	}
	for k, vs := range values {
		slices.Sort(vs)
		s.fields[k] = strings.Join(vs, ",")
	}
	return nil
}

func (s *side) locator() domain.Locator { return s.rows[0].Locator }

// EvaluateRecords implements rules.RecordEvaluator.
func (Check) EvaluateRecords(r *rules.Rule, a, b []domain.Record) ([]rules.RecordOutcome, error) {
	inA, _ := r.Input(inputA)
	inB, _ := r.Input(inputB)
	ga, gb := group(a), group(b)
	// An amount no row of a document holds is a column its layout does not read: it is missing
	// there, never zero. An amount some rows lack is zero in those rows, as GST portal files
	// leave out zero amounts.
	for _, g := range []struct {
		sides map[string]*side
		rows  []domain.Record
	}{{ga, a}, {gb, b}} {
		has := map[string]bool{}
		for _, row := range g.rows {
			for k := range row.Amounts {
				has[k] = true
			}
		}
		for _, s := range g.sides {
			s.has = has
		}
	}
	keys := map[string]bool{}
	for k := range ga {
		keys[k] = true
	}
	for k := range gb {
		keys[k] = true
	}
	tol := r.Check.Tolerance
	var out []rules.RecordOutcome
	var onlyA, onlyB []*side
	for _, k := range slices.Sorted(maps.Keys(keys)) {
		sa, sb := ga[k], gb[k]
		for _, s := range []*side{sa, sb} {
			if s != nil {
				if err := s.total(r.Check.InvoiceAmounts); err != nil {
					return nil, fmt.Errorf("rule %s: %w", r.ID, err)
				}
			}
		}
		switch {
		case sb == nil:
			onlyA = append(onlyA, sa)
			continue
		case sa == nil:
			onlyB = append(onlyB, sb)
			continue
		}
		o, dup, err := duplicate(r, inA, inB, sa, sb)
		if err != nil {
			return nil, fmt.Errorf("rule %s: key %s: %w", r.ID, k, err)
		}
		if dup {
			out = append(out, o)
			continue
		}
		if o, bad := linesDiffer(inA, inB, sa, sb); bad {
			out = append(out, o)
			continue
		}
		o, err = compare(r, inA, inB, sa, sb, tol)
		if err != nil {
			return nil, fmt.Errorf("rule %s: key %s: %w", r.ID, k, err)
		}
		o = reportListed(r, inA, inB, sa, sb, o)
		out = append(out, o)
	}
	paired, onlyA, onlyB, err := pair(r, inA, inB, onlyA, onlyB, tol)
	if err != nil {
		return nil, fmt.Errorf("rule %s: %w", r.ID, err)
	}
	out = append(out, paired...)
	// A side whose rows with no partner are not reported (report_missing) is left out here, after
	// the pairing, so a pair of numbers written differently is still found.
	if !reportsSide(r, inputA) {
		onlyA = nil
	}
	if !reportsSide(r, inputB) {
		onlyB = nil
	}
	for _, x := range []struct {
		in    rules.Input
		sides []*side
		code  string
	}{{inA, onlyA, OnlyInA}, {inB, onlyB, OnlyInB}} {
		for _, s := range x.sides {
			o, err := only(r, x.in, s, x.code)
			if err != nil {
				return nil, fmt.Errorf("rule %s: key %s: %w", r.ID, s.key, err)
			}
			if x.code == OnlyInA {
				o.Values = append(o.Values, shown(r, inA, inB, s, nil)...)
			} else {
				o.Values = append(o.Values, shown(r, inA, inB, nil, s)...)
			}
			out = append(out, o)
		}
	}
	slices.SortStableFunc(out, func(x, y rules.RecordOutcome) int { return cmp.Compare(x.MatchKey, y.MatchKey) })
	return out, nil
}

// duplicate reports a key with more than one row in an input whose rows are whole invoices.
// The rows are shown with the first compared amount that both documents hold; an amount a
// document's layout never reads is never shown as zero.
func duplicate(r *rules.Rule, inA, inB rules.Input, sa, sb *side) (rules.RecordOutcome, bool, error) {
	for _, d := range []struct {
		name string
		in   rules.Input
		s    *side
		code string
	}{{inputA, inA, sa, DuplicateA}, {inputB, inB, sb, DuplicateB}} {
		if len(d.s.rows) < 2 || !slices.Contains(r.Check.DuplicatesIn, d.name) {
			continue
		}
		o := rules.RecordOutcome{MatchKey: sa.key, Status: domain.StatusFail, Code: d.code}
		amount, ok := heldByBoth(r.Check.CompareAmounts, sa, sb)
		if !ok { // keys only, or no compared amount in both documents: the duplicate is the result
			return o, true, nil
		}
		other := sa
		if d.name == inputA {
			other = sb
		}
		otherIn := inA
		if d.name == inputA {
			otherIn = inB
		}
		o.Values = append(o.Values, labeled(otherIn, amount, domain.MoneyValue(other.amounts[amount]), other.locator()))
		for i, row := range d.s.rows {
			lv := labeled(d.in, amount, domain.MoneyValue(row.Amounts[amount]), row.Locator)
			lv.Input += fmt.Sprintf(".%d", i+1)
			lv.Label += ", " + fmt.Sprintf(duplicateOf, i+1, len(d.s.rows))
			o.Values = append(o.Values, lv)
		}
		diff, err := absDiff(d.s.amounts[amount], other.amounts[amount])
		if err != nil {
			return rules.RecordOutcome{}, false, err
		}
		o.Difference = diff
		return o, true, nil
	}
	return rules.RecordOutcome{}, false, nil
}

// heldByBoth returns the first of amounts that both documents hold (some row of each
// document has it), or false if there is none.
func heldByBoth(amounts []string, sa, sb *side) (string, bool) {
	for _, n := range amounts {
		if sa.has[n] && sb.has[n] {
			return n, true
		}
	}
	return "", false
}

// linesDiffer reports a key whose invoice-level amount is not the same on every line of
// the invoice: the total cannot be known, so the key is not compared.
func linesDiffer(inA, inB rules.Input, sa, sb *side) (rules.RecordOutcome, bool) {
	for _, d := range []struct {
		in rules.Input
		s  *side
	}{{inA, sa}, {inB, sb}} {
		if len(d.s.disagree) == 0 {
			continue
		}
		o := rules.RecordOutcome{MatchKey: sa.key, Status: domain.StatusFail, Code: LinesDiffer}
		n := d.s.disagree[0]
		for i, row := range d.s.rows {
			lv := labeled(d.in, n, domain.MoneyValue(row.Amounts[n]), row.Locator)
			lv.Input += fmt.Sprintf(".%d", i+1)
			lv.Label += ", line " + fmt.Sprint(i+1)
			o.Values = append(o.Values, lv)
		}
		return o, true
	}
	return rules.RecordOutcome{}, false
}

// compare compares one key's totals: fields exactly (ignoring case and surrounding
// spaces), amounts within the tolerance.
func compare(r *rules.Rule, inA, inB rules.Input, sa, sb *side, tol rules.Tolerance) (rules.RecordOutcome, error) {
	o := rules.RecordOutcome{MatchKey: sa.key, Status: domain.StatusPass, Code: Matched}
	fieldDiff := false
	for _, f := range r.Check.CompareFields {
		va, vb := sa.fields[f], sb.fields[f]
		if strings.EqualFold(strings.TrimSpace(va), strings.TrimSpace(vb)) {
			continue
		}
		fieldDiff = true
		o.Values = append(o.Values, labeled(inA, f, domain.TextValue(va), sa.locator()), labeled(inB, f, domain.TextValue(vb), sb.locator()))
	}
	amountDiff := false
	var missing []string // amounts one document does not hold at all: not compared, never zero
	for _, n := range r.Check.CompareAmounts {
		if !sa.has[n] || !sb.has[n] {
			for _, x := range []struct {
				in rules.Input
				s  *side
			}{{inA, sa}, {inB, sb}} {
				if !x.s.has[n] {
					missing = append(missing, rules.MissingAmountPrefix+x.in.RecordType+"."+n)
				}
			}
			continue
		}
		va, vb := sa.amounts[n], sb.amounts[n]
		d, err := absDiff(va, vb)
		if err != nil {
			return rules.RecordOutcome{}, fmt.Errorf("%s: %w", n, err)
		}
		if rules.WithinTolerance(d, va, vb, tol) {
			continue
		}
		amountDiff = true
		o.Values = append(o.Values, labeled(inA, n, domain.MoneyValue(va), sa.locator()), labeled(inB, n, domain.MoneyValue(vb), sb.locator()))
		o.Difference = max(o.Difference, d)
	}
	switch {
	case fieldDiff:
		o.Status, o.Code = domain.StatusFail, FieldDiff
	case amountDiff:
		o.Status, o.Code = domain.StatusFail, AmountDiff
	case len(missing) > 0:
		// Everything that could be compared agrees, but an amount is not in a document:
		// the row cannot be checked in full, so it is not reported as matched.
		o.Status, o.Code, o.Missing = domain.StatusCannotCheck, "", missing
	case len(r.Check.CompareAmounts) > 0:
		n := keyAmount
		if !slices.Contains(r.Check.CompareAmounts, n) {
			n = r.Check.CompareAmounts[0]
		}
		o.Values = append(o.Values, labeled(inA, n, domain.MoneyValue(sa.amounts[n]), sa.locator()), labeled(inB, n, domain.MoneyValue(sb.amounts[n]), sb.locator()))
	}
	return o, nil
}

// pair reports, once, a row only in a and a row only in b that agree on every pairing
// field and every compared amount: the same invoice with its number written differently.
// Only a pair that is the single candidate both ways is reported; anything else stays
// unmatched, so nothing is guessed. An amount one document's layout never reads is not
// compared (it is not known, so it is neither agreement nor disagreement); if compared
// amounts are set but none is held by both documents, nothing is paired.
func pair(r *rules.Rule, inA, inB rules.Input, onlyA, onlyB []*side, tol rules.Tolerance) ([]rules.RecordOutcome, []*side, []*side, error) {
	if len(r.Check.PairUnmatchedOn) == 0 {
		return nil, onlyA, onlyB, nil
	}
	agree := func(sa, sb *side) (bool, error) {
		for _, f := range r.Check.PairUnmatchedOn {
			if sa.fields[f] == "" || !strings.EqualFold(strings.TrimSpace(sa.fields[f]), strings.TrimSpace(sb.fields[f])) {
				return false, nil
			}
		}
		compared := 0
		for _, n := range r.Check.CompareAmounts {
			if !sa.has[n] || !sb.has[n] {
				continue
			}
			compared++
			d, err := absDiff(sa.amounts[n], sb.amounts[n])
			if err != nil {
				return false, fmt.Errorf("keys %s and %s: %s: %w", sa.key, sb.key, n, err)
			}
			if !rules.WithinTolerance(d, sa.amounts[n], sb.amounts[n], tol) {
				return false, nil
			}
		}
		return len(r.Check.CompareAmounts) == 0 || compared > 0, nil
	}
	candA := map[*side][]*side{}
	candB := map[*side][]*side{}
	for _, sa := range onlyA {
		for _, sb := range onlyB {
			ok, err := agree(sa, sb)
			if err != nil {
				return nil, nil, nil, err
			}
			if ok {
				candA[sa] = append(candA[sa], sb)
				candB[sb] = append(candB[sb], sa)
			}
		}
	}
	var out []rules.RecordOutcome
	used := map[*side]bool{}
	for _, sa := range onlyA {
		if len(candA[sa]) != 1 || len(candB[candA[sa][0]]) != 1 {
			continue
		}
		sb := candA[sa][0]
		used[sa], used[sb] = true, true
		o := rules.RecordOutcome{MatchKey: sa.key, Status: domain.StatusFail, Code: KeyDiff}
		a0, b0 := sa.rows[0], sb.rows[0]
		for _, k := range inA.MatchKey {
			if a0.Fields[k] != b0.Fields[k] {
				o.Values = append(o.Values, labeled(inA, k, domain.TextValue(a0.Fields[k]), sa.locator()), labeled(inB, k, domain.TextValue(b0.Fields[k]), sb.locator()))
			}
		}
		out = append(out, o)
	}
	keep := func(list []*side) []*side {
		var rest []*side
		for _, s := range list {
			if !used[s] {
				rest = append(rest, s)
			}
		}
		return rest
	}
	return out, keep(onlyA), keep(onlyB), nil
}

// only reports a key found in one input only, with that input's compared amounts that its
// document holds (an amount the layout never reads is left out, never shown as zero). With
// no such amount, the key itself is shown, so the finding still shows where the row is.
func only(r *rules.Rule, in rules.Input, s *side, code string) (rules.RecordOutcome, error) {
	o := rules.RecordOutcome{MatchKey: s.key, Status: domain.StatusFail, Code: code}
	for _, n := range r.Check.CompareAmounts {
		if !s.has[n] {
			continue
		}
		v := s.amounts[n]
		o.Values = append(o.Values, labeled(in, n, domain.MoneyValue(v), s.locator()))
		a, err := absDiff(v, 0)
		if err != nil {
			return rules.RecordOutcome{}, fmt.Errorf("%s: %w", n, err)
		}
		o.Difference = max(o.Difference, a)
	}
	if len(o.Values) == 0 {
		for _, k := range in.MatchKey {
			o.Values = append(o.Values, labeled(in, k, domain.TextValue(s.rows[0].Fields[k]), s.locator()))
		}
	}
	return o, nil
}

func labeled(in rules.Input, column string, v domain.Value, loc domain.Locator) domain.LabeledValue {
	return domain.LabeledValue{Input: in.Name + "." + column, Label: in.Label + ", " + ColumnLabel(column), Value: v, Locator: loc}
}

// ColumnLabel names a record column for finding text: "taxable_value" → "taxable value".
func ColumnLabel(column string) string {
	switch column {
	case "igst", "cgst", "cess":
		return strings.ToUpper(column)
	case "sgst":
		return "SGST/UTGST"
	case "counterparty_gstin", "supplier_gstin":
		return strings.ReplaceAll(strings.Replace(column, "gstin", "GSTIN", 1), "_", " ")
	}
	return strings.ReplaceAll(column, "_", " ")
}

// absDiff returns |a − b|, or money.ErrOverflow when it does not fit in paise, so an extreme
// or corrupt amount gives an error instead of a wrapped-around difference.
func absDiff(a, b money.Paise) (money.Paise, error) {
	d, err := money.Sub(a, b)
	if err != nil {
		return 0, err
	}
	return money.Abs(d)
}

// reportingConditions are the conditions report_fields can use on a column's text.
var reportingConditions = []string{rules.IsBlank, rules.IsNotBlank, rules.IsEquals, rules.IsMatches, rules.IsIn, rules.IsNotIn}

// validateReporting checks report_missing (the inputs a and b, each at most once) and
// report_fields (a condition on a column of one of the two record types).
func validateReporting(r *rules.Rule, ra, rb vocab.RecordType) []error {
	var errs []error
	seen := map[string]bool{}
	for _, s := range r.Check.ReportMissing {
		switch {
		case s != inputA && s != inputB:
			errs = append(errs, fmt.Errorf("report_missing: %q must be a or b", s))
		case seen[s]:
			errs = append(errs, fmt.Errorf("report_missing: %q is listed twice", s))
		}
		seen[s] = true
	}
	for i, w := range r.Check.ReportFields {
		var rt vocab.RecordType
		switch w.Input {
		case inputA:
			rt = ra
		case inputB:
			rt = rb
		default:
			errs = append(errs, fmt.Errorf("report_fields[%d]: input %q must be a or b", i, w.Input))
			continue
		}
		if !slices.Contains(rt.Fields, w.Field) {
			errs = append(errs, fmt.Errorf("report_fields[%d]: %q is not a column of %s", i, w.Field, rt.Name))
		}
		if !slices.Contains(reportingConditions, w.Is) {
			errs = append(errs, fmt.Errorf("report_fields[%d]: %q cannot test a column (use one of: %s)", i, w.Is, strings.Join(reportingConditions, ", ")))
		}
	}
	return errs
}

// reportsSide reports whether the rows only in the named input are reported: both sides when
// report_missing is not written, otherwise those it lists (none for an empty list).
func reportsSide(r *rules.Rule, name string) bool {
	return !r.Check.ReportMissingSet || slices.Contains(r.Check.ReportMissing, name)
}

// listed reports whether a listed column (report_fields) of the side holds a listed value in
// some row of it.
func listed(r *rules.Rule, name string, s *side) bool {
	if s == nil {
		return false
	}
	for _, w := range r.Check.ReportFields {
		if w.Input != name {
			continue
		}
		for _, row := range s.rows {
			if w.Holds(domain.TextValue(row.Fields[w.Field])) {
				return true
			}
		}
	}
	return false
}

// shown gives the columns named by report_fields as written in the file, for the sides that
// have a row, so the reader sees them next to the figures (the status of an e-way bill, the
// action taken in IMS).
func shown(r *rules.Rule, inA, inB rules.Input, sa, sb *side) []domain.LabeledValue {
	var out []domain.LabeledValue
	seen := map[string]bool{}
	for _, w := range r.Check.ReportFields {
		in, s := inA, sa
		if w.Input == inputB {
			in, s = inB, sb
		}
		if s == nil || seen[w.Input+"."+w.Field] {
			continue
		}
		seen[w.Input+"."+w.Field] = true
		out = append(out, labeled(in, w.Field, domain.TextValue(s.fields[w.Field]), s.locator()))
	}
	return out
}

// reportListed applies report_fields to the result of a key found in both inputs: a match
// whose listed column holds a listed value is reported (FIELD_REPORTED) with the column shown
// as written; a result that is already reported shows the columns too.
func reportListed(r *rules.Rule, inA, inB rules.Input, sa, sb *side, o rules.RecordOutcome) rules.RecordOutcome {
	if len(r.Check.ReportFields) == 0 {
		return o
	}
	if o.Status == domain.StatusPass && o.Code == Matched {
		if !listed(r, inputA, sa) && !listed(r, inputB, sb) {
			return o
		}
		o.Status, o.Code = domain.StatusFail, FieldReported
	}
	if o.Status == domain.StatusFail {
		o.Values = append(o.Values, shown(r, inA, inB, sa, sb)...)
	}
	return o
}
