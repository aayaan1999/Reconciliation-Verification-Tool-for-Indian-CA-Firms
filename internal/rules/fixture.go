package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// Rule fixtures (RU-43). Next to each rule.yaml:
//
//	fixtures/pass/*.yaml   input facts on which every evaluation passes
//	fixtures/fail/*.yaml   input facts on which at least one evaluation fails
//
// SPEC-008's fixture runner evaluates them; this file defines and checks their format.

// FixtureSchema is the value of the "schema" key of every fixture file.
const FixtureSchema = "recon.rule.fixture/1"

// Evaluation statuses as written in fixture files.
const (
	StatusPass        = "pass"
	StatusFail        = "fail"
	StatusCannotCheck = "cannot_check"
)

type fixtureYAML struct {
	Schema      string             `yaml:"schema"`
	Description string             `yaml:"description"`
	TaxYear     string             `yaml:"tax_year"`
	Facts       []fixtureFactYAML  `yaml:"facts"`
	Expected    []expectedEvalYAML `yaml:"expected"`
}

// fixtureFactYAML is one input fact. Values are written as domain.FormatValue writes them
// (money as rupees with two decimals: "112450.50").
type fixtureFactYAML struct {
	Fact    string `yaml:"fact"`
	Value   string `yaml:"value"`
	Period  string `yaml:"period"`
	Scope   string `yaml:"scope"`
	File    string `yaml:"file"`    // for finding text: the source file name
	Locator string `yaml:"locator"` // for finding text: the JSON path in that file
}

type expectedEvalYAML struct {
	Status  string   `yaml:"status"`
	Period  string   `yaml:"period"`
	Scope   string   `yaml:"scope"`
	Missing []string `yaml:"missing"` // facts named in a cannot_check
}

// A Fixture is one validated fixture file.
type Fixture struct {
	File        string
	Kind        string // StatusPass or StatusFail: the folder it sits in
	Description string
	TaxYear     period.TaxYear
	Facts       []domain.Fact // in file order; EngagementID 0
	Expected    []Expected
}

// Expected is one evaluation a fixture expects.
type Expected struct {
	Status  string // StatusPass, StatusFail or StatusCannotCheck
	Period  string
	Scope   string
	Missing []string // sorted
}

// Fixtures are a rule's pass and fail fixtures, each in file-name order.
type Fixtures struct {
	Pass, Fail []*Fixture
}

// LoadFixtures reads and checks every fixture of r, which must have been loaded from a
// file (r.File). Missing folders give no fixtures; whether that is allowed is the
// caller's decision (rule-lint requires one of each, RU-43).
func LoadFixtures(r *Rule, v *vocab.Vocab) (*Fixtures, error) {
	out := &Fixtures{}
	var errs strictyaml.Errors
	for _, kind := range []string{StatusPass, StatusFail} {
		files, _ := filepath.Glob(filepath.Join(filepath.Dir(r.File), "fixtures", kind, "*.yaml"))
		slices.Sort(files)
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				return nil, err
			}
			fx, err := LoadFixture(filepath.ToSlash(file), kind, data, r, v)
			if err != nil {
				errs = append(errs, err.(strictyaml.Errors)...)
				continue
			}
			if kind == StatusPass {
				out.Pass = append(out.Pass, fx)
			} else {
				out.Fail = append(out.Fail, fx)
			}
		}
	}
	if len(errs) > 0 {
		errs.Sort()
		return nil, errs
	}
	return out, nil
}

// LoadFixture checks one fixture file of rule r. kind is the folder: StatusPass or
// StatusFail.
func LoadFixture(name, kind string, data []byte, r *Rule, v *vocab.Vocab) (*Fixture, error) {
	var f fixtureYAML
	pos, err := strictyaml.Decode(name, data, &f)
	if err != nil {
		return nil, err
	}
	c := &checker{file: name, pos: pos}
	fx := &Fixture{File: name, Kind: kind, Description: f.Description}

	if f.Schema != FixtureSchema {
		c.fail("schema", "schema must be %q, not %q", FixtureSchema, f.Schema)
	}
	if strings.TrimSpace(f.Description) == "" {
		c.fail("description", "description must say what the fixture shows")
	}
	ty, err := period.ParseTaxYear(f.TaxYear)
	switch {
	case err != nil:
		c.fail("tax_year", "%v", err)
	case !r.AppliesTo(ty):
		c.fail("tax_year", "rule %s does not apply to tax year %s", r.ID, ty)
	default:
		fx.TaxYear = ty
	}

	type key struct{ name, period, scope string }
	seen := map[key]bool{}
	for i, fy := range f.Facts {
		at := fmt.Sprintf("facts[%d]", i)
		if !ruleReadsFact(r, fy.Fact) {
			c.fail(at+".fact", "fact %q is not an input of rule %s", fy.Fact, r.ID)
			continue
		}
		vf, _ := v.Lookup(fy.Fact)
		val, err := domain.ParseValue(vf.Kind, fy.Value)
		if err != nil {
			c.fail(at+".value", "%s: %v", fy.Fact, err)
			continue
		}
		p, ok := c.period(at+".period", fy.Period, fx.TaxYear)
		if !ok {
			continue
		}
		if !periodicityFits(vf.Periodicity, p) {
			c.fail(at+".period", "%s is a %s figure; period %s does not fit", fy.Fact, vf.Periodicity, p)
			continue
		}
		k := key{fy.Fact, p.String(), fy.Scope}
		if seen[k] {
			c.fail(at, "%s for %s is given twice", fy.Fact, p)
			continue
		}
		seen[k] = true
		fx.Facts = append(fx.Facts, domain.Fact{
			Name: fy.Fact, Period: p.String(), Scope: fy.Scope, Value: val,
			Locator: domain.Locator{FileName: fy.File, Kind: domain.LocatorJSONPath, Path: fy.Locator},
		})
	}

	if len(f.Expected) == 0 {
		c.fail("expected", "expected must list at least one evaluation")
	}
	fails := 0
	for i, ey := range f.Expected {
		at := fmt.Sprintf("expected[%d]", i)
		e := Expected{Status: ey.Status, Scope: ey.Scope, Missing: slices.Sorted(slices.Values(ey.Missing))}
		switch ey.Status {
		case StatusFail:
			fails++
		case StatusPass, StatusCannotCheck:
		default:
			c.fail(at+".status", "status %q must be one of: pass, fail, cannot_check", ey.Status)
		}
		if kind == StatusPass && ey.Status != StatusPass {
			c.fail(at+".status", "a fixture in fixtures/pass expects only passes; move it to fixtures/fail")
		}
		if (ey.Status == StatusCannotCheck) != (len(ey.Missing) > 0) {
			c.fail(at+".missing", "missing lists the absent facts of a cannot_check, and only there")
		}
		for _, m := range ey.Missing {
			if !ruleReadsFact(r, m) {
				c.fail(at+".missing", "missing fact %q is not an input of rule %s", m, r.ID)
			}
		}
		if p, ok := c.period(at+".period", ey.Period, fx.TaxYear); ok {
			e.Period = p.String()
		}
		fx.Expected = append(fx.Expected, e)
	}
	if kind == StatusFail && len(f.Expected) > 0 && fails == 0 {
		c.fail("expected", "a fixture in fixtures/fail must expect at least one fail")
	}

	if len(c.errs) > 0 {
		c.errs.Sort()
		return nil, c.errs
	}
	return fx, nil
}

func ruleReadsFact(r *Rule, fact string) bool {
	for _, in := range r.Inputs {
		if in.Fact == fact && fact != "" {
			return true
		}
	}
	return false
}

// period reads a canonical period that lies inside tax year ty (when ty is known).
func (c *checker) period(at, text string, ty period.TaxYear) (period.Period, bool) {
	p, err := period.Parse(text)
	switch {
	case err != nil:
		c.fail(at, "%v", err)
		return period.Period{}, false
	case !ty.IsZero() && !period.Contains(period.Year(ty), p):
		c.fail(at, "period %s is outside tax year %s", p, ty)
		return period.Period{}, false
	}
	return p, true
}

func periodicityFits(p vocab.Periodicity, pp period.Period) bool {
	switch p {
	case vocab.PeriodicityFY:
		return pp.Kind() == period.KindYear
	case vocab.PeriodicityQuarter:
		return pp.Kind() == period.KindQuarter
	case vocab.PeriodicityMonth:
		return pp.Kind() == period.KindMonth
	}
	return false
}
