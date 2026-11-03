package rules

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// The Rule Builder (SPEC-010): the window's fields as a Form, checked with plain words that
// name the field, then written as a rule file by RuleYAML. The CA never types YAML (RU-21).
// Every text is a string as typed; amounts are read with internal/money, never as floats.

// A Form is what the Rule Builder window sends. Field names are the JSON keys.
type Form struct {
	Mode           string        `json:"mode"` // "create" for a new rule, "edit" for an existing one
	ID             string        `json:"id"`   // RECON-<PREFIX>-<NNN>
	Title          string        `json:"title"`
	RuleText       string        `json:"rule_text"`
	Notes          string        `json:"notes"`
	Module         string        `json:"module"`
	Category       string        `json:"category"`
	TaxYears       []string      `json:"tax_years"`
	ValidFrom      string        `json:"valid_from"`
	ValidTo        string        `json:"valid_to"`
	Inputs         []FormInput   `json:"inputs"`
	Check          FormCheck     `json:"check"`
	Severity       string        `json:"severity"`
	ReviewDocument string        `json:"review_document"`
	Law            []FormLaw     `json:"law"`
	Fixtures       []FormFixture `json:"fixtures"` // create: at least one pass and one fail; edit: optional, added to the rule's own
}

// A FormInput is one figure or kind of row the rule reads, picked from the vocabulary.
type FormInput struct {
	Name     string `json:"name"`  // "a", "b", "parts" ...
	Kind     string `json:"kind"`  // "fact" or "record_type"
	Ref      string `json:"ref"`   // the fact or record type name, from the vocabulary
	Label    string `json:"label"` // shown in finding text next to the value
	Optional bool   `json:"optional"`
	Default  string `json:"default"` // the value of an optional figure when it is absent
}

// A FormCheck holds the parameters of the check type. Only the ones the chosen type uses are
// written (the window shows only those fields).
type FormCheck struct {
	Type            string   `json:"type"`
	Tolerance       string   `json:"tolerance"` // rupees as typed, such as "1.00"; "" for none
	CompareAmounts  []string `json:"compare_amounts"`
	CompareFields   []string `json:"compare_fields"`
	DuplicatesIn    []string `json:"duplicates_in"`
	InvoiceAmounts  []string `json:"invoice_amounts"`
	PairUnmatchedOn []string `json:"pair_unmatched_on"`
	// SPEC-018 (match_records): the sides whose rows with no partner are reported (written only when
	// ReportMissingSet), and the columns whose listed values are reported.
	ReportMissing    []string        `json:"report_missing"`
	ReportMissingSet bool            `json:"report_missing_set"`
	ReportFields     []FormCondition `json:"report_fields"`
	Parts            []FormPart      `json:"parts"`
	Total            string          `json:"total"`
	TotalField       string          `json:"total_field"`
	TotalKey         string          `json:"total_key"` // sum_equals on totals of rows (SPEC-017)
	Base             string          `json:"base"`
	Rate             string          `json:"rate"`
	Value            string          `json:"value"`
	Keys             []string        `json:"keys"`
	When             []FormCondition `json:"when"`
	Table            []FormTableRow  `json:"table"`
}

// A FormPart is one figure added or taken away in sum_equals or percent_of.
type FormPart struct {
	Input     string          `json:"input"`
	Field     string          `json:"field"`
	Sign      string          `json:"sign"`       // "+" or "-"
	RowsWhere []FormCondition `json:"rows_where"` // the part is the sum over the rows that meet these (SPEC-017)
}

// A FormCondition is one test in present_together.
type FormCondition struct {
	Input  string   `json:"input"`
	Field  string   `json:"field"`
	Is     string   `json:"is"`
	Value  string   `json:"value"`
	Values []string `json:"values"`
}

// A FormTableRow is one row of a table_lookup table, with the source it comes from.
type FormTableRow struct {
	Keys   []string `json:"keys"`
	Value  string   `json:"value"`
	Source string   `json:"source"`
}

// A FormLaw is one legal basis: the Act and the section, and an optional reference (RU-30).
type FormLaw struct {
	Act       string `json:"act"`
	Section   string `json:"section"`
	Reference string `json:"reference"`
}

// A FormFixture is one pass or fail fixture: figures as typed, each with its source.
type FormFixture struct {
	Kind        string       `json:"kind"` // "pass" or "fail"
	Name        string       `json:"name"` // the file name, such as "values_agree"
	Description string       `json:"description"`
	Facts       []FormFact   `json:"facts"`
	Records     []FormRecord `json:"records"`
}

// A FormFact is one figure of a fixture.
type FormFact struct {
	Fact    string `json:"fact"`
	Value   string `json:"value"`
	Period  string `json:"period"`
	Scope   string `json:"scope"`
	File    string `json:"file"`    // the source file name, for finding text
	Locator string `json:"locator"` // where the figure is in that file, such as a JSON path
}

// A FormRecord is one row of a record type in a fixture.
type FormRecord struct {
	RecordType string      `json:"record_type"`
	Period     string      `json:"period"`
	Scope      string      `json:"scope"`
	File       string      `json:"file"`
	Locator    string      `json:"locator"`
	Fields     []FormNamed `json:"fields"`
	Amounts    []FormNamed `json:"amounts"`
}

// A FormNamed is one named column of a row: a text field or an amount.
type FormNamed struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// A Problem is one thing the window refuses, with the field it is about (SPEC-010 AC-3).
type Problem struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// A Context is what a form is checked against: the vocabulary, the check types the engine
// registers, and the category codes of content/rules/categories.yaml.
type Context struct {
	Vocab      *vocab.Vocab
	Checks     *CheckTypes
	Categories []string
}

var (
	formIDPattern      = regexp.MustCompile(`^RECON-[A-Z][A-Z0-9]*-[0-9]{3}$`)
	fixtureNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// Problems returns everything wrong with the rule's own fields, in the order they appear on
// the window. An empty list means the form can be written; the loader still has the last word.
func (f *Form) Problems(c Context) []Problem {
	var ps []Problem
	add := func(field, code, format string, args ...any) {
		ps = append(ps, Problem{Field: field, Code: code, Message: fmt.Sprintf(format, args...)})
	}
	if !formIDPattern.MatchString(f.ID) {
		add("id", "BAD_ID", "the ID %q must look like RECON-DEP-002: RECON, a prefix of capital letters, and three digits", f.ID)
	}
	if strings.TrimSpace(f.Title) == "" {
		add("title", "TITLE_MISSING", "give the rule a title, in a few words")
	} else if b := BannedIn(f.Title); len(b) > 0 {
		add("title", "BANNED_WORD", "the title uses %q; a title says what is compared, never what ought to be", b[0])
	}
	if strings.TrimSpace(f.RuleText) == "" {
		add("rule_text", "RULE_TEXT_MISSING", "say in plain English what the rule compares")
	} else if b := BannedIn(f.RuleText); len(b) > 0 {
		add("rule_text", "BANNED_WORD", "the rule text uses %q; say what is compared, never what ought to be", b[0])
	}
	if !slices.Contains(Modules, f.Module) {
		add("module", "BAD_MODULE", "choose a module from the list (%s)", strings.Join(Modules, ", "))
	}
	if !slices.Contains(c.Categories, f.Category) {
		add("category", "BAD_CATEGORY", "choose a category from the list (%s)", strings.Join(c.Categories, ", "))
	}
	if !slices.Contains(severities, f.Severity) {
		add("severity", "BAD_SEVERITY", "choose a severity from the list (%s)", strings.Join(severities, ", "))
	}
	if _, ok := c.Vocab.DocumentType(f.ReviewDocument); !ok {
		add("review_document", "BAD_REVIEW_DOCUMENT", "choose the document that needs fixing from the list; it groups the findings")
	}
	ps = append(ps, f.yearProblems()...)
	ps = append(ps, f.inputProblems(c)...)
	if strings.TrimSpace(f.Check.Type) == "" {
		add("check.type", "CHECK_TYPE_MISSING", "choose a check type")
	} else if _, ok := c.Checks.Lookup(f.Check.Type); !ok {
		add("check.type", "UNKNOWN_CHECK_TYPE", "the check %q is not one the engine runs (choose from: %s)", f.Check.Type, strings.Join(c.Checks.Names(), ", "))
	}
	if f.Check.Tolerance != "" {
		switch p, err := money.Parse(f.Check.Tolerance, money.UnitRupees); {
		case err != nil:
			add("check.tolerance", "BAD_AMOUNT", "the tolerance %q is not an amount in rupees; write it with at most two decimals, such as 1.00 (%s)", f.Check.Tolerance, bareMessage(err))
		case p < 0:
			add("check.tolerance", "BAD_AMOUNT", "the tolerance cannot be negative")
		}
	}
	if f.Check.Rate != "" {
		if _, err := domain.ParseValue(domain.KindRate, f.Check.Rate); err != nil {
			add("check.rate", "BAD_RATE", "the rate %q must be a percentage with at most two decimals, such as 15.00", f.Check.Rate)
		}
	}
	if len(f.Law) == 0 {
		add("law", "NO_LAW", "give at least one legal basis: the Act and the section (RU-30)")
	}
	for i, l := range f.Law {
		if strings.TrimSpace(l.Act) == "" || strings.TrimSpace(l.Section) == "" {
			add(fmt.Sprintf("law[%d]", i), "LAW_INCOMPLETE", "each legal basis needs the Act and the section")
		}
	}
	return ps
}

func (f *Form) yearProblems() []Problem {
	var ps []Problem
	if len(f.TaxYears) == 0 {
		ps = append(ps, Problem{Field: "tax_years", Code: "TAX_YEAR_MISSING", Message: "choose at least one tax year, such as 2026-27"})
	}
	seen := map[period.TaxYear]bool{}
	for i, s := range f.TaxYears {
		at := fmt.Sprintf("tax_years[%d]", i)
		ty, err := period.ParseTaxYear(s)
		switch {
		case err != nil:
			ps = append(ps, Problem{Field: at, Code: "BAD_TAX_YEAR", Message: fmt.Sprintf("the tax year %q must look like 2026-27", s)})
		case seen[ty]:
			ps = append(ps, Problem{Field: at, Code: "TAX_YEAR_TWICE", Message: fmt.Sprintf("the tax year %s is listed twice", ty)})
		default:
			seen[ty] = true
		}
	}
	from, err := period.ParseDate(f.ValidFrom)
	if err != nil {
		ps = append(ps, Problem{Field: "valid_from", Code: "BAD_DATE", Message: "the from-date must be a date such as 2026-04-01"})
	}
	if f.ValidTo != "" {
		to, err := period.ParseDate(f.ValidTo)
		switch {
		case err != nil:
			ps = append(ps, Problem{Field: "valid_to", Code: "BAD_DATE", Message: "the to-date must be a date such as 2027-03-31, or left empty for no end"})
		case err == nil && from.Valid() && to.Before(from):
			ps = append(ps, Problem{Field: "valid_to", Code: "BAD_DATE", Message: "the to-date is before the from-date"})
		}
	}
	return ps
}

func (f *Form) inputProblems(c Context) []Problem {
	var ps []Problem
	if len(f.Inputs) == 0 {
		return append(ps, Problem{Field: "inputs", Code: "NO_INPUTS", Message: "pick at least one figure or record type the rule reads"})
	}
	names := map[string]bool{}
	for i, in := range f.Inputs {
		at := fmt.Sprintf("inputs[%d]", i)
		if !inputPattern.MatchString(in.Name) {
			ps = append(ps, Problem{Field: at, Code: "BAD_INPUT_NAME", Message: fmt.Sprintf("the name %q must be lowercase letters, digits and _, starting with a letter", in.Name)})
		} else if names[in.Name] {
			ps = append(ps, Problem{Field: at, Code: "INPUT_TWICE", Message: fmt.Sprintf("the input name %q is used twice", in.Name)})
		}
		names[in.Name] = true
		if strings.TrimSpace(in.Label) == "" {
			ps = append(ps, Problem{Field: at + ".label", Code: "LABEL_MISSING", Message: "give the input a label, shown next to its value in finding text"})
		}
		switch in.Kind {
		case "fact":
			fact, ok := c.Vocab.Lookup(in.Ref)
			switch {
			case !ok:
				ps = append(ps, Problem{Field: at + ".ref", Code: "UNKNOWN_FACT", Message: fmt.Sprintf("the figure %q is not in the fact vocabulary; pick a figure from the list", in.Ref)})
			case fact.Deprecated():
				ps = append(ps, Problem{Field: at + ".ref", Code: "RETIRED_FACT", Message: fmt.Sprintf("the figure %q was retired in vocabulary version %d", in.Ref, fact.DeprecatedIn)})
			}
			if in.Default != "" {
				if !in.Optional {
					ps = append(ps, Problem{Field: at + ".default", Code: "DEFAULT_NEEDS_OPTIONAL", Message: "a default is only for an optional figure"})
				} else if ok {
					if _, err := domain.ParseValue(fact.Kind, in.Default); err != nil {
						ps = append(ps, Problem{Field: at + ".default", Code: "BAD_VALUE", Message: fmt.Sprintf("the default %q is not a valid value for %s", in.Default, in.Ref)})
					}
				}
			}
		case "record_type":
			rt, ok := c.Vocab.RecordType(in.Ref)
			switch {
			case !ok:
				ps = append(ps, Problem{Field: at + ".ref", Code: "UNKNOWN_RECORD_TYPE", Message: fmt.Sprintf("the record type %q is not in the vocabulary; pick one from the list", in.Ref)})
			case rt.Deprecated():
				ps = append(ps, Problem{Field: at + ".ref", Code: "RETIRED_RECORD_TYPE", Message: fmt.Sprintf("the record type %q was retired in vocabulary version %d", in.Ref, rt.DeprecatedIn)})
			}
			if in.Default != "" {
				ps = append(ps, Problem{Field: at + ".default", Code: "DEFAULT_ON_ROWS", Message: "a record type cannot have a default"})
			}
		default:
			ps = append(ps, Problem{Field: at + ".kind", Code: "BAD_INPUT_KIND", Message: "an input is a figure (fact) or a record type"})
		}
	}
	return ps
}

// FixtureProblems checks the fixtures typed on the window: their names, kinds, figures and
// sources. A pass or fail fixture must hold at least one figure or row, each with its source
// file and locator (invariant 1: a finding shows where both values come from).
func (f *Form) FixtureProblems(c Context) []Problem {
	var ps []Problem
	add := func(field, code, format string, args ...any) {
		ps = append(ps, Problem{Field: field, Code: code, Message: fmt.Sprintf(format, args...)})
	}
	type key struct{ kind, name string }
	seen := map[key]bool{}
	for i, fx := range f.Fixtures {
		at := fmt.Sprintf("fixtures[%d]", i)
		if fx.Kind != "pass" && fx.Kind != "fail" {
			add(at+".kind", "BAD_FIXTURE_KIND", "a fixture is a pass or a fail")
		}
		switch {
		case !fixtureNamePattern.MatchString(fx.Name):
			add(at+".name", "BAD_FIXTURE_NAME", "the fixture name %q must be lowercase letters, digits and _, such as values_agree", fx.Name)
		case seen[key{fx.Kind, fx.Name}]:
			add(at+".name", "FIXTURE_TWICE", "a %s fixture is named %q twice", fx.Kind, fx.Name)
		}
		seen[key{fx.Kind, fx.Name}] = true
		if strings.TrimSpace(fx.Description) == "" {
			add(at+".description", "FIXTURE_DESCRIPTION_MISSING", "say in a sentence what the fixture shows")
		}
		if len(fx.Facts)+len(fx.Records) == 0 {
			add(at, "FIXTURE_EMPTY", "a fixture needs at least one figure or row")
		}
		for j, fa := range fx.Facts {
			fat := fmt.Sprintf("%s.facts[%d]", at, j)
			ps = append(ps, f.factFixtureProblems(c, fa, fat)...)
		}
		for j, r := range fx.Records {
			rat := fmt.Sprintf("%s.records[%d]", at, j)
			ps = append(ps, f.recordFixtureProblems(c, r, rat)...)
		}
	}
	return ps
}

func (f *Form) factFixtureProblems(c Context, fa FormFact, at string) []Problem {
	var ps []Problem
	add := func(field, code, format string, args ...any) {
		ps = append(ps, Problem{Field: field, Code: code, Message: fmt.Sprintf(format, args...)})
	}
	vf, ok := c.Vocab.Lookup(fa.Fact)
	switch {
	case !ok:
		add(at+".fact", "UNKNOWN_FACT", "the figure %q is not in the fact vocabulary", fa.Fact)
	case !f.readsFact(fa.Fact):
		add(at+".fact", "FIXTURE_NOT_AN_INPUT", "the figure %q is not an input of this rule", fa.Fact)
	default:
		if _, err := domain.ParseValue(vf.Kind, fa.Value); err != nil {
			add(at+".value", "BAD_VALUE", "the value %q of %s is not valid: %s", fa.Value, fa.Fact, bareMessage(err))
		}
	}
	if _, err := period.Parse(fa.Period); err != nil {
		add(at+".period", "BAD_PERIOD", "the period %q must be written such as FY2026-27 or 2026-27 Q1, the way the figure is read", fa.Period)
	}
	if strings.TrimSpace(fa.File) == "" || strings.TrimSpace(fa.Locator) == "" {
		add(at+".source", "SOURCE_MISSING", "each figure names its source file and where it is in that file")
	}
	return ps
}

func (f *Form) recordFixtureProblems(c Context, r FormRecord, at string) []Problem {
	var ps []Problem
	add := func(field, code, format string, args ...any) {
		ps = append(ps, Problem{Field: field, Code: code, Message: fmt.Sprintf(format, args...)})
	}
	rt, ok := c.Vocab.RecordType(r.RecordType)
	switch {
	case !ok:
		add(at+".record_type", "UNKNOWN_RECORD_TYPE", "the record type %q is not in the vocabulary", r.RecordType)
	case !f.readsRecords(r.RecordType):
		add(at+".record_type", "FIXTURE_NOT_AN_INPUT", "the record type %q is not an input of this rule", r.RecordType)
	default:
		for _, fv := range r.Fields {
			if !slices.Contains(rt.Fields, fv.Name) {
				add(at+".fields", "BAD_FIELD", "%q is not a field of %s", fv.Name, r.RecordType)
			}
		}
		for _, av := range r.Amounts {
			if !slices.Contains(rt.Amounts, av.Name) {
				add(at+".amounts", "BAD_AMOUNT_NAME", "%q is not an amount of %s", av.Name, r.RecordType)
				continue
			}
			if _, err := money.Parse(av.Value, money.UnitRupees); err != nil {
				add(at+".amounts", "BAD_AMOUNT", "the amount %q of %s is not an amount in rupees with at most two decimals", av.Value, av.Name)
			}
		}
	}
	if _, err := period.Parse(r.Period); err != nil {
		add(at+".period", "BAD_PERIOD", "the period %q must be written such as FY2026-27 or 2026-27 Q1", r.Period)
	}
	if strings.TrimSpace(r.File) == "" || strings.TrimSpace(r.Locator) == "" {
		add(at+".source", "SOURCE_MISSING", "each row names its source file and where it is in that file")
	}
	return ps
}

// FixtureCountProblems says what a rule's fixtures lack: at least one pass and one fail
// (RU-43). kinds holds the kind of every fixture the rule has, the new ones and any kept.
func FixtureCountProblems(kinds []string) []Problem {
	var ps []Problem
	if !slices.Contains(kinds, "pass") {
		ps = append(ps, Problem{Field: "fixtures", Code: "NO_PASS_FIXTURE", Message: "add a pass fixture: figures that agree, which the rule must pass"})
	}
	if !slices.Contains(kinds, "fail") {
		ps = append(ps, Problem{Field: "fixtures", Code: "NO_FAIL_FIXTURE", Message: "add a fail fixture: figures that disagree, which the rule must fail"})
	}
	return ps
}

func (f *Form) readsFact(name string) bool {
	return slices.ContainsFunc(f.Inputs, func(in FormInput) bool { return in.Kind == "fact" && in.Ref == name })
}

func (f *Form) readsRecords(name string) bool {
	return slices.ContainsFunc(f.Inputs, func(in FormInput) bool { return in.Kind == "record_type" && in.Ref == name })
}

// bareMessage drops the package prefix of an error, so a message reads in plain words.
func bareMessage(err error) string {
	var pe *money.ParseError
	if errors.As(err, &pe) {
		return pe.Reason
	}
	return err.Error()
}

// FormOf reads a stored rule file back into the form it was written from, so the window can
// show a rule to edit. A rule whose tolerance the window cannot express is refused rather
// than silently changed on the next save.
func FormOf(name string, body []byte) (Form, error) {
	var y ruleYAML
	if _, err := strictyaml.Decode(name, body, &y); err != nil {
		return Form{}, err
	}
	f := Form{Mode: "edit", ID: y.ID, Title: y.Title, RuleText: y.RuleTextEn, Notes: y.Notes, Module: y.Module,
		Category: y.Category, TaxYears: y.Applies.TaxYears, ValidFrom: y.Applies.ValidFrom, ValidTo: y.Applies.ValidTo,
		Severity: y.Severity, ReviewDocument: y.ReviewDocument}
	for _, l := range y.Law {
		f.Law = append(f.Law, FormLaw{Act: l.Act, Section: l.Section, Reference: l.Reference})
	}
	names := make([]string, 0, len(y.Inputs))
	for n := range y.Inputs {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		in := y.Inputs[n]
		kind, ref := "fact", in.Fact
		if in.RecordType != "" {
			kind, ref = "record_type", in.RecordType
		}
		f.Inputs = append(f.Inputs, FormInput{Name: n, Kind: kind, Ref: ref, Label: in.Label, Optional: in.Optional, Default: in.Default})
	}
	c := y.Check
	f.Check = FormCheck{Type: c.Type, CompareAmounts: c.CompareAmounts, CompareFields: c.CompareFields, DuplicatesIn: c.DuplicatesIn,
		InvoiceAmounts: c.InvoiceAmounts, PairUnmatchedOn: c.PairUnmatchedOn, Total: c.Total, TotalField: c.TotalField, TotalKey: c.TotalKey,
		Base: c.Base, Rate: c.Rate, Value: c.Value, Keys: c.Keys}
	if c.Tolerance != nil {
		if c.Tolerance.BPOfLarger != nil {
			return Form{}, fmt.Errorf("%s: the tolerance is a percentage of the larger amount, which the Rule Builder cannot edit yet; edit the file by hand", name)
		}
		if c.Tolerance.Paise != nil {
			f.Check.Tolerance = domain.FormatValue(domain.MoneyValue(money.Paise(*c.Tolerance.Paise)))
		}
	}
	for _, p := range c.Parts {
		fp := FormPart{Input: p.Input, Field: p.Field, Sign: p.Sign}
		for _, w := range p.RowsWhere {
			fp.RowsWhere = append(fp.RowsWhere, FormCondition{Input: w.Input, Field: w.Field, Is: w.Is, Value: w.Value, Values: w.Values})
		}
		f.Check.Parts = append(f.Check.Parts, fp)
	}
	f.Check.ReportMissing, f.Check.ReportMissingSet = c.ReportMissing, c.ReportMissing != nil
	for _, w := range c.ReportFields {
		f.Check.ReportFields = append(f.Check.ReportFields, FormCondition{Input: w.Input, Field: w.Field, Is: w.Is, Value: w.Value, Values: w.Values})
	}
	for _, w := range c.When {
		f.Check.When = append(f.Check.When, FormCondition{Input: w.Input, Field: w.Field, Is: w.Is, Value: w.Value, Values: w.Values})
	}
	for _, t := range c.Table {
		f.Check.Table = append(f.Check.Table, FormTableRow{Keys: t.Keys, Value: t.Value, Source: t.Source})
	}
	return f, nil
}
