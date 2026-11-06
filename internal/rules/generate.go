package rules

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
)

// Generate (technical spec §B9): the YAML files the Rule Builder writes. The output has a
// stable key order and every text value in double quotes, so the strict loader reads it
// exactly as written. Both writers expect a form that Problems and FixtureProblems accept.

// RuleYAML writes the rule file of form f, with the version and source it is saved under.
func RuleYAML(f *Form, version, source string) ([]byte, error) {
	var b strings.Builder
	b.WriteString("# Rule file (schema recon.rule/1, technical spec §B4.2), written by the Rule Builder from the\n")
	b.WriteString("# form. Every text value is in double quotes. Its status (testing, disabled) is kept in the\n")
	b.WriteString("# database, not in this file. Check with: go run ./cmd/recon-dev rule-lint <office rules folder>\n")
	fmt.Fprintf(&b, "schema: %s\n", quote(Schema))
	fmt.Fprintf(&b, "id: %s\n", quote(f.ID))
	fmt.Fprintf(&b, "version: %s\n", quote(version))
	fmt.Fprintf(&b, "source: %s\n", quote(source))
	fmt.Fprintf(&b, "module: %s\n", quote(f.Module))
	fmt.Fprintf(&b, "category: %s\n", quote(f.Category))
	if len(f.AlsoCategories) > 0 { // written only when used, so a rule with one type reads exactly as before
		fmt.Fprintf(&b, "also_categories: %s\n", list(quoteAll(f.AlsoCategories)))
	}
	fmt.Fprintf(&b, "title: %s\n", quote(f.Title))
	fmt.Fprintf(&b, "rule_text_en: %s\n", quote(f.RuleText))
	fmt.Fprintf(&b, "notes: %s\n", quote(f.Notes))

	b.WriteString("\nlaw:\n")
	for _, l := range f.Law {
		fmt.Fprintf(&b, "  - act: %s\n    section: %s\n    reference: %s\n", quote(l.Act), quote(l.Section), quote(l.Reference))
	}

	var years []string
	for _, s := range f.TaxYears {
		ty, err := period.ParseTaxYear(s)
		if err != nil {
			return nil, fmt.Errorf("tax year %q: %w", s, err)
		}
		years = append(years, quote(ty.String()))
	}
	from, err := period.ParseDate(f.ValidFrom)
	if err != nil {
		return nil, fmt.Errorf("valid_from %q: %w", f.ValidFrom, err)
	}
	to := ""
	if f.ValidTo != "" {
		d, err := period.ParseDate(f.ValidTo)
		if err != nil {
			return nil, fmt.Errorf("valid_to %q: %w", f.ValidTo, err)
		}
		to = d.String()
	}
	fmt.Fprintf(&b, "\napplies:\n  tax_years: %s\n  valid_from: %s\n  valid_to: %s\n",
		list(years), quote(from.String()), quote(to))

	b.WriteString("\ninputs:\n")
	ins := slices.Clone(f.Inputs)
	slices.SortFunc(ins, func(a, b FormInput) int { return strings.Compare(a.Name, b.Name) })
	for _, in := range ins {
		key := "fact"
		if in.Kind == "record_type" {
			key = "record_type"
		}
		optional := ""
		if in.Optional {
			optional = "true"
		}
		fmt.Fprintf(&b, "  %s: %s\n", in.Name, flow(key, quote(in.Ref), "label", quote(in.Label), "optional", optional, "default", quoteOpt(in.Default)))
	}

	c := f.Check
	b.WriteString("\ncheck:\n")
	fmt.Fprintf(&b, "  type: %s\n", quote(c.Type))
	if c.Tolerance != "" {
		p, err := money.Parse(c.Tolerance, money.UnitRupees)
		if err != nil {
			return nil, fmt.Errorf("tolerance %q: %w", c.Tolerance, err)
		}
		fmt.Fprintf(&b, "  tolerance: { paise: %d }\n", int64(p))
	}
	for _, l := range []struct {
		name string
		xs   []string
	}{{"compare_amounts", c.CompareAmounts}, {"compare_fields", c.CompareFields}, {"duplicates_in", c.DuplicatesIn},
		{"invoice_amounts", c.InvoiceAmounts}, {"pair_unmatched_on", c.PairUnmatchedOn}, {"keys", c.Keys}} {
		if len(l.xs) > 0 {
			fmt.Fprintf(&b, "  %s: %s\n", l.name, list(quoteAll(l.xs)))
		}
	}
	if len(c.Parts) > 0 {
		var ps []string
		for _, p := range c.Parts {
			rows := ""
			if len(p.RowsWhere) > 0 {
				rows = "[ " + strings.Join(conditionsFlow(p.RowsWhere), ", ") + " ]"
			}
			ps = append(ps, flow("input", quote(p.Input), "field", quoteOpt(p.Field), "sign", quote(p.Sign), "rows_where", rows))
		}
		fmt.Fprintf(&b, "  parts: [ %s ]\n", strings.Join(ps, ", "))
	}
	for _, kv := range [][2]string{{"total", c.Total}, {"total_field", c.TotalField}, {"total_key", c.TotalKey}, {"base", c.Base}, {"value", c.Value}} {
		if kv[1] != "" {
			fmt.Fprintf(&b, "  %s: %s\n", kv[0], quote(kv[1]))
		}
	}
	if c.Rate != "" {
		rate, err := domain.ParseValue(domain.KindRate, c.Rate)
		if err != nil {
			return nil, fmt.Errorf("rate %q: %w", c.Rate, err)
		}
		fmt.Fprintf(&b, "  rate: %s\n", quote(domain.FormatValue(domain.RateValue(rate.RateBP))))
	}
	if c.ReportMissingSet {
		fmt.Fprintf(&b, "  report_missing: %s\n", list(quoteAll(c.ReportMissing)))
	}
	if len(c.ReportFields) > 0 {
		fmt.Fprintf(&b, "  report_fields: [ %s ]\n", strings.Join(conditionsFlow(c.ReportFields), ", "))
	}
	if len(c.When) > 0 {
		fmt.Fprintf(&b, "  when: [ %s ]\n", strings.Join(conditionsFlow(c.When), ", "))
	}
	if len(c.Table) > 0 {
		var ts []string
		for _, t := range c.Table {
			ts = append(ts, flow("keys", list(quoteAll(t.Keys)), "value", quote(t.Value), "source", quote(t.Source)))
		}
		fmt.Fprintf(&b, "  table: [ %s ]\n", strings.Join(ts, ", "))
	}

	fmt.Fprintf(&b, "\nseverity: %s\n", quote(f.Severity))
	fmt.Fprintf(&b, "review_document: %s\n", quote(f.ReviewDocument))
	return []byte(b.String()), nil
}

// A fixtureDoc is one fixture file in the order the file holds it.
type fixtureDoc struct {
	description string
	taxYear     string
	facts       []fixtureFactDoc
	records     []fixtureRecordDoc
}

type fixtureFactDoc struct{ fact, value, period, scope, file, locator string }

type fixtureRecordDoc struct {
	recordType, period, scope, file, locator string
	fields, amounts                          []FormNamed
}

// formFixtureDoc is a fixture as typed on the window, for tax year taxYear.
func formFixtureDoc(fx FormFixture, taxYear string) fixtureDoc {
	d := fixtureDoc{description: fx.Description, taxYear: taxYear}
	for _, f := range fx.Facts {
		d.facts = append(d.facts, fixtureFactDoc{fact: f.Fact, value: f.Value, period: f.Period, scope: f.Scope, file: f.File, locator: f.Locator})
	}
	for _, r := range fx.Records {
		d.records = append(d.records, fixtureRecordDoc{recordType: r.RecordType, period: r.Period, scope: r.Scope, file: r.File,
			locator: r.Locator, fields: r.Fields, amounts: r.Amounts})
	}
	return d
}

// fixtureDocOf is a fixture read from a file (a fixture carried into a new version of a rule),
// written back in the same form: its figures are formatted as FormatValue writes them, and its
// columns in name order.
func fixtureDocOf(fx *Fixture) fixtureDoc {
	d := fixtureDoc{description: fx.Description, taxYear: fx.TaxYear.String()}
	for _, f := range fx.Facts {
		d.facts = append(d.facts, fixtureFactDoc{fact: f.Name, value: domain.FormatValue(f.Value), period: f.Period, scope: f.Scope,
			file: f.Locator.FileName, locator: f.Locator.Path})
	}
	for _, r := range fx.Records {
		fd := fixtureRecordDoc{recordType: r.RecordType, period: r.Period, scope: r.Scope, file: r.Locator.FileName, locator: r.Locator.Path}
		for _, name := range sortedKeysOf(r.Fields) {
			fd.fields = append(fd.fields, FormNamed{Name: name, Value: r.Fields[name]})
		}
		for _, name := range sortedKeysOf(r.Amounts) {
			fd.amounts = append(fd.amounts, FormNamed{Name: name, Value: domain.FormatValue(domain.MoneyValue(r.Amounts[name]))})
		}
		d.records = append(d.records, fd)
	}
	return d
}

// YAML writes the fixture file with the results expected of it.
func (d fixtureDoc) YAML(expected []Expected) []byte {
	var b strings.Builder
	b.WriteString("# Rule fixture (schema recon.rule.fixture/1), written by the Rule Builder. The expected results\n")
	b.WriteString("# are the ones the engine gave when the rule was saved; the file's folder says pass or fail.\n")
	fmt.Fprintf(&b, "schema: %s\n", quote(FixtureSchema))
	fmt.Fprintf(&b, "description: %s\n", quote(d.description))
	fmt.Fprintf(&b, "tax_year: %s\n", quote(d.taxYear))
	b.WriteString(listKey("facts", len(d.facts)))
	for _, f := range d.facts {
		fmt.Fprintf(&b, "  - %s\n", flow("fact", quote(f.fact), "value", quote(f.value), "period", quote(f.period),
			"scope", quote(f.scope), "file", quote(f.file), "locator", quote(f.locator)))
	}
	b.WriteString(listKey("records", len(d.records)))
	for _, r := range d.records {
		fields, amounts := "", ""
		if len(r.fields) > 0 {
			fields = list(namedList(r.fields))
		}
		if len(r.amounts) > 0 {
			amounts = list(namedList(r.amounts))
		}
		fmt.Fprintf(&b, "  - %s\n", flow("record_type", quote(r.recordType), "period", quote(r.period), "scope", quote(r.scope),
			"file", quote(r.file), "locator", quote(r.locator), "fields", fields, "amounts", amounts))
	}
	b.WriteString(listKey("expected", len(expected)))
	for _, e := range expected {
		missing := ""
		if len(e.Missing) > 0 {
			missing = list(quoteAll(e.Missing))
		}
		fmt.Fprintf(&b, "  - %s\n", flow("status", quote(e.Status), "period", quote(e.Period), "scope", quote(e.Scope),
			"match_key", quoteOpt(e.MatchKey), "code", quoteOpt(e.Code), "missing", missing))
	}
	return []byte(b.String())
}

// FormFixtureYAML writes a fixture typed on the window with the results expected of it.
func FormFixtureYAML(fx FormFixture, taxYear string, expected []Expected) []byte {
	return formFixtureDoc(fx, taxYear).YAML(expected)
}

// FixtureYAML writes a fixture read from a file, with the results expected of it.
func FixtureYAML(fx *Fixture, expected []Expected) []byte { return fixtureDocOf(fx).YAML(expected) }

// PlaceholderExpected is the expected result a fixture is first written with, so it loads
// before the engine has run it: one line of the fixture's own kind, on its first figure's
// period. The engine's results replace it.
func PlaceholderExpected(kind string, period, scope string) []Expected {
	return []Expected{{Status: kind, Period: period, Scope: scope}}
}

// quote writes text as a YAML double-quoted scalar. JSON string syntax is a subset of it.
func quote(s string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // a string always encodes
	return strings.TrimSuffix(b.String(), "\n")
}

// quoteOpt writes text, or nothing when it is empty (so flow skips the key).
func quoteOpt(s string) string {
	if s == "" {
		return ""
	}
	return quote(s)
}

func quoteAll(xs []string) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = quote(x)
	}
	return out
}

// listKey starts a block sequence: "key:" when it has items (the items follow), and "key: []"
// when it has none, since a bare key reads as null and the loader wants a list.
func listKey(key string, n int) string {
	if n == 0 {
		return key + ": []\n"
	}
	return key + ":\n"
}

// list writes a flow sequence of already-written items.
func list(items []string) string { return "[ " + strings.Join(items, ", ") + " ]" }

// flow writes a flow mapping from alternating keys and already-written values; a value that
// is "" is left out.
func flow(kv ...string) string {
	var parts []string
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] != "" {
			parts = append(parts, kv[i]+": "+kv[i+1])
		}
	}
	if len(parts) == 0 {
		return "{}"
	}
	return "{ " + strings.Join(parts, ", ") + " }"
}

func namedList(xs []FormNamed) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = flow("name", quote(x.Name), "value", quote(x.Value))
	}
	return out
}

func sortedKeysOf[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// conditionsFlow writes conditions as flow mappings, one each.
func conditionsFlow(cs []FormCondition) []string {
	var ws []string
	for _, w := range cs {
		values := ""
		if len(w.Values) > 0 {
			values = list(quoteAll(w.Values))
		}
		ws = append(ws, flow("input", quote(w.Input), "field", quoteOpt(w.Field), "is", quote(w.Is), "value", quoteOpt(w.Value), "values", values))
	}
	return ws
}
