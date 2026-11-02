package rules

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// A Rule is a loaded, validated rule (§B4.2). It is read-only and safe to share.
// Lifecycle status, approvals and office versions are not in the file (SPEC-027).
type Rule struct {
	ID             string // "RECON-DEP-002"
	Version        string // "1"
	Source         string // "builtin" | "office"
	Module         string // "tieout" | "newheads" | "gst" | "tds" | "payroll" | "drafts"
	Category       string // "R1"
	Title          string
	RuleText       string // rule_text_en: plain English, shown on screen (RU-03)
	Notes          string
	Law            []domain.LawRef  // at least one (RU-30)
	TaxYears       []period.TaxYear // in file order, no repeats
	ValidFrom      period.Date
	ValidTo        period.Date // zero: no end date
	Inputs         []Input     // sorted by name
	Check          Check
	Severity       string // "high" | "medium" | "low"
	ReviewDocument string // a vocabulary document type: UI grouping only (FD-06)
	File           string // the file it was loaded from, for messages
}

// An Input is one figure or kind of row the rule reads.
type Input struct {
	Name       string           // "a", "b", "parts" ...
	Fact       string           // a vocabulary fact, or ""
	RecordType string           // a vocabulary record type, or ""
	Kind       domain.ValueKind // the fact's kind; 0 for a record type
	MatchKey   []string         // the record type's match-key columns; nil for a fact
	Label      string           // shown in finding text next to the value
	Optional   bool
	Default    *domain.Value // the value used when an optional fact is absent; nil: none
}

// Check is the rule's check block.
type Check struct {
	Type      string
	Tolerance Tolerance
	// match_records only.
	CompareAmounts  []string
	CompareFields   []string
	DuplicatesIn    []string
	InvoiceAmounts  []string
	PairUnmatchedOn []string
	// sum_equals (SPEC-006): the parts, each added or taken away, and the input they are
	// compared with; on rows (SPEC-007) TotalField is the amount of each row compared.
	// percent_of may take its base as Parts instead of Base.
	Parts      []Part
	Total      string
	TotalField string
	// percent_of: RateBP per cent (basis points) of Base, against Value. table_lookup:
	// the input looked up (Value) by the Keys inputs in Table.
	Base    string
	RateBP  int64
	RateSet bool
	Value   string
	Keys    []string
	Table   []TableRow
	// present_together: the conditions, all of which hold for a finding.
	When []Condition
}

// ToleranceKind says how a tolerance is measured.
type ToleranceKind int

const (
	ToleranceNone       ToleranceKind = iota // exact comparison
	TolerancePaise                           // a fixed number of paise
	ToleranceBPOfLarger                      // basis points of the larger absolute value
)

// Tolerance is how far two amounts may differ and still agree, computed in integers.
type Tolerance struct {
	Kind  ToleranceKind
	Value int64 // paise, or basis points (0..10000)
}

// Input returns the input called name.
func (r *Rule) Input(name string) (Input, bool) {
	i, ok := slices.BinarySearchFunc(r.Inputs, name, func(in Input, n string) int { return strings.Compare(in.Name, n) })
	if !ok {
		return Input{}, false
	}
	return r.Inputs[i], true
}

// AppliesTo reports whether the rule is written for tax year ty.
func (r *Rule) AppliesTo(ty period.TaxYear) bool { return slices.Contains(r.TaxYears, ty) }

// Key is "<id> v<version>", the way a rule is named in messages.
func (r *Rule) Key() string { return r.ID + " v" + r.Version }

var (
	idPattern       = regexp.MustCompile(`^[A-Z][A-Z0-9]*(-[A-Z0-9]+)+$`)
	versionPattern  = regexp.MustCompile(`^[1-9][0-9]*$`)
	categoryPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	inputPattern    = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
)

// officeVersionPattern is an office edit of a version: "1-office.1" (SPEC-010, RU-12).
var officeVersionPattern = regexp.MustCompile(`^[1-9][0-9]*-office\.[1-9][0-9]*$`)

// Modules are the reconciliation areas a rule can belong to (§A6.4).
var Modules = []string{"drafts", "gst", "newheads", "payroll", "tds", "tieout"}

var (
	sources    = []string{"builtin", "office"}
	severities = []string{"high", "medium", "low"}
)

// MaxBasisPoints is 100%: a bp_of_larger tolerance cannot be larger.
const MaxBasisPoints = 10000

// Load parses and validates one rule file against the vocabulary and the check types.
// Every problem is reported, each with file and line, as strictyaml.Errors.
func Load(name string, data []byte, v *vocab.Vocab, cts *CheckTypes) (*Rule, error) {
	var f ruleYAML
	pos, err := strictyaml.Decode(name, data, &f)
	if err != nil {
		return nil, err
	}
	c := &checker{file: name, pos: pos}
	r := &Rule{
		ID: f.ID, Version: f.Version, Source: f.Source, Module: f.Module, Category: f.Category,
		Title: f.Title, RuleText: f.RuleTextEn, Notes: f.Notes, Severity: f.Severity,
		ReviewDocument: f.ReviewDocument, File: name, Check: Check{Type: f.Check.Type,
			CompareAmounts: f.Check.CompareAmounts, CompareFields: f.Check.CompareFields,
			DuplicatesIn: f.Check.DuplicatesIn, InvoiceAmounts: f.Check.InvoiceAmounts, PairUnmatchedOn: f.Check.PairUnmatchedOn},
	}

	if f.Schema != Schema {
		c.fail("schema", "schema must be %q, not %q", Schema, f.Schema)
	}
	if !idPattern.MatchString(f.ID) {
		c.fail("id", "id %q must be capital letters and digits in parts joined by -, such as \"RECON-DEP-002\"", f.ID)
	}
	c.oneOf("source", f.Source, sources)
	switch {
	case versionPattern.MatchString(f.Version):
	case officeVersionPattern.MatchString(f.Version) && f.Source == "office":
		// An office version of a built-in rule: "1-office.1" is office edit 1 of version 1.
	case officeVersionPattern.MatchString(f.Version):
		c.fail("version", "version %q with an office edit can only be in an office rule", f.Version)
	default:
		c.fail("version", "version %q must be a whole number from 1, in quotes", f.Version)
	}
	c.oneOf("module", f.Module, Modules)
	if !categoryPattern.MatchString(f.Category) {
		c.fail("category", "category %q must be capital letters and digits, such as \"R1\"", f.Category)
	}
	c.words("title", "title", f.Title)
	if strings.TrimSpace(f.RuleTextEn) == "" {
		c.fail("rule_text_en", "rule_text_en must say in plain English what the rule compares")
	}
	c.oneOf("severity", f.Severity, severities)
	if _, ok := v.DocumentType(f.ReviewDocument); !ok {
		c.fail("review_document", "review_document %q is not a document type in the vocabulary", f.ReviewDocument)
	}

	if len(f.Law) == 0 {
		c.fail("law", "law must list at least one legal basis, with act and section (RU-30)")
	}
	for i, l := range f.Law {
		at := fmt.Sprintf("law[%d]", i)
		if strings.TrimSpace(l.Act) == "" || strings.TrimSpace(l.Section) == "" {
			c.fail(at, "each law entry needs an act and a section")
		}
		r.Law = append(r.Law, domain.LawRef{Act: l.Act, Section: l.Section, Reference: l.Reference})
	}

	c.applies(r, f.Applies)
	c.inputs(r, f.Inputs, v)
	c.tolerance(r, f.Check.Tolerance)
	c.params(r, f.Check)

	// The check type sees the rest of the rule, so it runs only once everything else is valid.
	ct, ok := cts.Lookup(f.Check.Type)
	switch {
	case f.Check.Type == "":
		c.fail("check", "check.type is missing")
	case !ok:
		c.fail("check.type", "check type %q is not known (known: %s)", f.Check.Type, strings.Join(cts.Names(), ", "))
	case len(c.errs) == 0:
		if err := ct.ValidateParams(r, v); err != nil {
			for _, e := range unjoin(err) {
				c.fail("check", "check %q: %v", f.Check.Type, e)
			}
		}
	}

	if len(c.errs) > 0 {
		c.errs.Sort()
		return nil, c.errs
	}
	return r, nil
}

// unjoin splits an errors.Join error into its parts.
func unjoin(err error) []error {
	if j, ok := err.(interface{ Unwrap() []error }); ok {
		return j.Unwrap()
	}
	return []error{err}
}

type checker struct {
	file string
	pos  strictyaml.Positions
	errs strictyaml.Errors
}

// fail records a problem at the line of path, or of the nearest enclosing path known.
func (c *checker) fail(path, format string, args ...any) {
	line := 0
	for p := path; ; {
		if line = c.pos.Line(p); line > 0 || p == "" {
			break
		}
		if i := strings.LastIndexAny(p, ".["); i >= 0 {
			p = p[:i]
		} else {
			p = ""
		}
	}
	c.errs = append(c.errs, &strictyaml.Error{File: c.file, Line: line, Msg: fmt.Sprintf(format, args...)})
}

func (c *checker) oneOf(key, value string, allowed []string) {
	if !slices.Contains(allowed, value) {
		c.fail(key, "%s %q must be one of: %s", key, value, strings.Join(allowed, ", "))
	}
}

// words checks text that reaches finding text: present, and free of banned phrases.
func (c *checker) words(at, what, text string) {
	if strings.TrimSpace(text) == "" {
		c.fail(at, "%s is missing", what)
		return
	}
	for _, b := range BannedIn(text) {
		c.fail(at, "%s uses %q; finding text states what was compared, never what ought to be", what, b)
	}
}

func (c *checker) applies(r *Rule, a appliesYAML) {
	if len(a.TaxYears) == 0 {
		c.fail("applies", "applies.tax_years must list at least one tax year, such as [\"2025-26\"]")
	}
	for i, s := range a.TaxYears {
		ty, err := period.ParseTaxYear(s)
		switch {
		case err != nil:
			c.fail(fmt.Sprintf("applies.tax_years[%d]", i), "%v", err)
		case slices.Contains(r.TaxYears, ty):
			c.fail(fmt.Sprintf("applies.tax_years[%d]", i), "tax year %s is listed twice", ty)
		default:
			r.TaxYears = append(r.TaxYears, ty)
		}
	}
	from, err := period.ParseDate(a.ValidFrom)
	if err != nil {
		c.fail("applies.valid_from", "applies.valid_from must be a date such as \"2025-04-01\": %v", err)
		return
	}
	r.ValidFrom = from
	if a.ValidTo == "" {
		return
	}
	to, err := period.ParseDate(a.ValidTo)
	switch {
	case err != nil:
		c.fail("applies.valid_to", "applies.valid_to must be a date such as \"2026-03-31\", or \"\" for no end: %v", err)
	case to.Before(from):
		c.fail("applies.valid_to", "applies.valid_to %s is before valid_from %s", to, from)
	default:
		r.ValidTo = to
	}
}

func (c *checker) inputs(r *Rule, in map[string]inputYAML, v *vocab.Vocab) {
	if len(in) == 0 {
		c.fail("inputs", "inputs must name at least one figure or record type")
	}
	names := make([]string, 0, len(in))
	for n := range in {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		iy, at := in[n], "inputs."+n
		if !inputPattern.MatchString(n) {
			c.fail(at, "input name %q must be lowercase letters, digits and _, starting with a letter", n)
			continue
		}
		input := Input{Name: n, Fact: iy.Fact, RecordType: iy.RecordType, Label: iy.Label, Optional: iy.Optional}
		c.words(at+".label", "the label of input "+n, iy.Label)
		switch {
		case iy.Fact != "" && iy.RecordType != "":
			c.fail(at, "input %s must name a fact or a record_type, not both", n)
		case iy.Fact != "":
			f, ok := v.Lookup(iy.Fact)
			switch {
			case !ok:
				c.fail(at+".fact", "fact %q is not in the vocabulary", iy.Fact)
			case f.Deprecated():
				c.fail(at+".fact", "fact %q was retired in vocabulary version %d", iy.Fact, f.DeprecatedIn)
			default:
				input.Kind = f.Kind
			}
		case iy.RecordType != "":
			rt, ok := v.RecordType(iy.RecordType)
			switch {
			case !ok:
				c.fail(at+".record_type", "record type %q is not in the vocabulary", iy.RecordType)
			case rt.Deprecated():
				c.fail(at+".record_type", "record type %q was retired in vocabulary version %d", iy.RecordType, rt.DeprecatedIn)
			default:
				input.MatchKey = slices.Clone(rt.MatchKey)
			}
		default:
			c.fail(at, "input %s must name a fact or a record_type", n)
		}
		switch {
		case iy.Default != "" && !iy.Optional:
			c.fail(at+".default", "input %s has a default but is not optional", n)
		case iy.Default != "" && iy.RecordType != "":
			c.fail(at+".default", "a record type input cannot have a default")
		case iy.Default != "" && input.Kind != 0:
			d, err := domain.ParseValue(input.Kind, iy.Default)
			if err != nil {
				c.fail(at+".default", "default of input %s: %v", n, err)
			} else {
				input.Default = &d
			}
		}
		r.Inputs = append(r.Inputs, input)
	}
}

// params reads the parameters of the SPEC-006 check types; whether they suit the rule's
// check type is for its ValidateParams.
func (c *checker) params(r *Rule, cy checkYAML) {
	for i, p := range cy.Parts {
		at := fmt.Sprintf("check.parts[%d]", i)
		sign := 0
		switch p.Sign {
		case "+":
			sign = 1
		case "-":
			sign = -1
		default:
			c.fail(at+".sign", "sign %q must be \"+\" (added) or \"-\" (taken away)", p.Sign)
		}
		r.Check.Parts = append(r.Check.Parts, Part{Input: p.Input, Field: p.Field, Sign: sign})
	}
	r.Check.TotalField = cy.TotalField
	r.Check.Total, r.Check.Base, r.Check.Value, r.Check.Keys = cy.Total, cy.Base, cy.Value, cy.Keys
	if cy.Rate != "" {
		rate, err := domain.ParseValue(domain.KindRate, cy.Rate)
		if err != nil || rate.RateBP <= 0 || rate.RateBP > MaxBasisPoints {
			c.fail("check.rate", "rate %q must be a percentage from 0.01 to 100, such as \"15.00\"", cy.Rate)
		} else {
			r.Check.RateBP, r.Check.RateSet = rate.RateBP, true
		}
	}
	for i, w := range cy.When {
		at := fmt.Sprintf("check.when[%d]", i)
		cond := Condition{Input: w.Input, Field: w.Field, Is: w.Is, Value: w.Value, Values: w.Values}
		switch {
		case !slices.Contains(ConditionNames, w.Is):
			c.fail(at+".is", "is %q must be one of: %s", w.Is, strings.Join(ConditionNames, ", "))
		case (w.Is == IsEquals || w.Is == IsMatches || w.Is == IsSameAs) && w.Value == "":
			c.fail(at+".value", "%s needs a value", w.Is)
		case (w.Is == IsIn || w.Is == IsNotIn) && len(w.Values) == 0:
			c.fail(at+".values", "%s needs a list of values", w.Is)
		case w.Is == IsMatches:
			re, err := regexp.Compile(w.Value)
			if err != nil {
				c.fail(at+".value", "%q is not a regular expression: %v", w.Value, err)
			}
			cond.Re = re
		}
		r.Check.When = append(r.Check.When, cond)
	}
	// Table values are of the kind of the input they are compared with.
	kind := domain.ValueKind(0)
	if in, ok := r.Input(cy.Value); ok {
		kind = in.Kind
	}
	for i, row := range cy.Table {
		at := fmt.Sprintf("check.table[%d]", i)
		if strings.TrimSpace(row.Source) == "" {
			c.fail(at+".source", "each table row names its source, such as a treaty article")
		}
		if kind == 0 {
			c.fail(at+".value", "the table's values need value to name a figure input")
			continue
		}
		val, err := domain.ParseValue(kind, row.Value)
		if err != nil {
			c.fail(at+".value", "%v", err)
			continue
		}
		r.Check.Table = append(r.Check.Table, TableRow{Keys: row.Keys, Value: val, Source: row.Source})
	}
}

func (c *checker) tolerance(r *Rule, t *toleranceYAML) {
	if t == nil {
		return
	}
	switch {
	case (t.Paise == nil) == (t.BPOfLarger == nil):
		c.fail("check.tolerance", "check.tolerance must be either { paise: N } or { bp_of_larger: N }")
	case t.Paise != nil && *t.Paise < 0:
		c.fail("check.tolerance.paise", "tolerance cannot be negative")
	case t.Paise != nil:
		r.Check.Tolerance = Tolerance{Kind: TolerancePaise, Value: *t.Paise}
	case *t.BPOfLarger < 0 || *t.BPOfLarger > MaxBasisPoints:
		c.fail("check.tolerance.bp_of_larger", "bp_of_larger must be from 0 to %d (100%%)", MaxBasisPoints)
	default:
		r.Check.Tolerance = Tolerance{Kind: ToleranceBPOfLarger, Value: *t.BPOfLarger}
	}
}
