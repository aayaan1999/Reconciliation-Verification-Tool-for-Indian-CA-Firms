package rules_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// Unit tests of the Rule Builder's form (SPEC-010): the problems a form has, the rule file
// written from it, the form read back from a rule, and the categories list. Figures are
// illustrative.

const (
	builtinRules = "../../content/rules"
	vocabFolder  = "../../content/vocabulary"
)

func loadVocab(t *testing.T) *vocab.Vocab {
	t.Helper()
	v, err := vocab.LoadDir(vocabFolder)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func context(t *testing.T) rules.Context {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(builtinRules, "categories.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cats, err := rules.LoadCategories("categories.yaml", data)
	if err != nil {
		t.Fatal(err)
	}
	return rules.Context{Vocab: loadVocab(t), Checks: checks.All(), Categories: rules.CategoryCodes(cats)}
}

// validForm is a new rule that passes every check of the window.
func validForm() rules.Form {
	return rules.Form{
		Mode: "create", ID: "RECON-OFF-001", Title: "Depreciation check", RuleText: "Compares two depreciation figures.",
		Module: "tieout", Category: "R1", TaxYears: []string{"2025-26"}, ValidFrom: "2025-04-01",
		Inputs: []rules.FormInput{
			{Name: "a", Kind: "fact", Ref: "computation.allowed_depreciation", Label: "Computation"},
			{Name: "b", Kind: "fact", Ref: "computation.fixed_asset_schedule.total", Label: "Schedule"},
		},
		Check:          rules.FormCheck{Type: "equal", Tolerance: "1.00"},
		Severity:       "high",
		ReviewDocument: "computation",
		Law:            []rules.FormLaw{{Act: "Income-tax Act, 1961", Section: "32"}},
		Fixtures: []rules.FormFixture{{Kind: "pass", Name: "agree", Description: "They agree",
			Facts: []rules.FormFact{{Fact: "computation.allowed_depreciation", Value: "1.00", Period: "FY2025-26", File: "c.json", Locator: "$.a"}}}},
	}
}

func problemCodes(ps []rules.Problem) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Code + "@" + p.Field
	}
	return out
}

func hasProblem(ps []rules.Problem, code, field string) bool {
	return slices.ContainsFunc(ps, func(p rules.Problem) bool { return p.Code == code && p.Field == field })
}

// A valid form has no problems; each field of the window is checked in its own words.
func TestFormProblems(t *testing.T) {
	c := context(t)
	f := validForm()
	if ps := f.Problems(c); len(ps) != 0 {
		t.Fatalf("a valid form has problems: %v", problemCodes(ps))
	}
	if ps := f.FixtureProblems(c); len(ps) != 0 {
		t.Fatalf("a valid fixture has problems: %v", problemCodes(ps))
	}
	cases := []struct {
		name   string
		change func(f *rules.Form)
		code   string
		field  string
	}{
		{"a bad ID", func(f *rules.Form) { f.ID = "recon-x" }, "BAD_ID", "id"},
		{"a banned word in the title", func(f *rules.Form) { f.Title = "The total should match" }, "BANNED_WORD", "title"},
		{"no rule text", func(f *rules.Form) { f.RuleText = " " }, "RULE_TEXT_MISSING", "rule_text"},
		{"an unknown module", func(f *rules.Form) { f.Module = "payroll-x" }, "BAD_MODULE", "module"},
		{"an unknown category", func(f *rules.Form) { f.Category = "R99" }, "BAD_CATEGORY", "category"},
		{"an unknown severity", func(f *rules.Form) { f.Severity = "urgent" }, "BAD_SEVERITY", "severity"},
		{"an unknown review document", func(f *rules.Form) { f.ReviewDocument = "no_such_doc" }, "BAD_REVIEW_DOCUMENT", "review_document"},
		{"a malformed tax year", func(f *rules.Form) { f.TaxYears = []string{"2025/26"} }, "BAD_TAX_YEAR", "tax_years[0]"},
		{"no tax year", func(f *rules.Form) { f.TaxYears = nil }, "TAX_YEAR_MISSING", "tax_years"},
		{"a to-date before the from-date", func(f *rules.Form) { f.ValidTo = "2025-01-01" }, "BAD_DATE", "valid_to"},
		{"a retired figure", func(f *rules.Form) { f.Inputs[0].Ref = "tar.retired.example" }, "UNKNOWN_FACT", "inputs[0].ref"},
		{"an input with no label", func(f *rules.Form) { f.Inputs[1].Label = "" }, "LABEL_MISSING", "inputs[1].label"},
		{"a duplicate input name", func(f *rules.Form) { f.Inputs[1].Name = "a" }, "INPUT_TWICE", "inputs[1]"},
		{"a default on a figure that is not optional", func(f *rules.Form) { f.Inputs[0].Default = "1.00" }, "DEFAULT_NEEDS_OPTIONAL", "inputs[0].default"},
		{"an unknown check type", func(f *rules.Form) { f.Check.Type = "fuzzy" }, "UNKNOWN_CHECK_TYPE", "check.type"},
		{"a negative tolerance", func(f *rules.Form) { f.Check.Tolerance = "-1.00" }, "BAD_AMOUNT", "check.tolerance"},
		{"a rate that is not a percentage", func(f *rules.Form) { f.Check.Rate = "15.005" }, "BAD_RATE", "check.rate"},
		{"a legal basis with no section", func(f *rules.Form) { f.Law[0].Section = "" }, "LAW_INCOMPLETE", "law[0]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := validForm()
			tc.change(&f)
			ps := f.Problems(c)
			if !hasProblem(ps, tc.code, tc.field) {
				t.Errorf("problems %v: want %s at %s", problemCodes(ps), tc.code, tc.field)
			}
		})
	}
}

// Fixtures are checked on the window too: a value that is not an amount, a period that does not
// parse, an unknown figure, and a figure with no source (invariant 1) are each refused.
func TestFixtureProblems(t *testing.T) {
	c := context(t)
	cases := []struct {
		name   string
		change func(fx *rules.FormFixture)
		code   string
	}{
		{"a value with a half paisa", func(fx *rules.FormFixture) { fx.Facts[0].Value = "1.005" }, "BAD_VALUE"},
		{"a period that does not parse", func(fx *rules.FormFixture) { fx.Facts[0].Period = "April 2025" }, "BAD_PERIOD"},
		{"a figure that is not an input", func(fx *rules.FormFixture) { fx.Facts[0].Fact = "computation.allowed_depreciation.x" }, "UNKNOWN_FACT"},
		{"a figure with no source file", func(fx *rules.FormFixture) { fx.Facts[0].File = "" }, "SOURCE_MISSING"},
		{"a fixture with no figure", func(fx *rules.FormFixture) { fx.Facts = nil }, "FIXTURE_EMPTY"},
		{"a fixture name with capitals", func(fx *rules.FormFixture) { fx.Name = "Agree" }, "BAD_FIXTURE_NAME"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := validForm()
			tc.change(&f.Fixtures[0])
			if ps := f.FixtureProblems(c); !slices.ContainsFunc(ps, func(p rules.Problem) bool { return p.Code == tc.code }) {
				t.Errorf("problems %v: want %s", problemCodes(ps), tc.code)
			}
		})
	}
	if ps := rules.FixtureCountProblems([]string{"pass"}); !hasProblem(ps, "NO_FAIL_FIXTURE", "fixtures") || len(ps) != 1 {
		t.Errorf("count problems with one pass fixture = %v", problemCodes(ps))
	}
}

// Every built-in rule, read into a form and written back, loads again with the same check,
// inputs and tolerance: the window can show any rule without changing what it does.
func TestEveryBuiltInRuleRoundTripsThroughTheForm(t *testing.T) {
	v := loadVocab(t)
	cts := checks.All()
	n := 0
	err := filepath.WalkDir(builtinRules, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != rules.FileName {
			return err
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		orig, err := rules.Load(p, body, v, cts)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		f, err := rules.FormOf(p, body)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		out, err := rules.RuleYAML(&f, orig.Version, orig.Source)
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		again, err := rules.Load(p+" (written)", out, v, cts)
		if err != nil {
			t.Fatalf("%s written from its form does not load: %v\n%s", p, err, out)
		}
		if again.Check.Type != orig.Check.Type || again.Check.Tolerance != orig.Check.Tolerance ||
			len(again.Inputs) != len(orig.Inputs) || again.ID != orig.ID || !slices.Equal(again.TaxYears, orig.TaxYears) {
			t.Errorf("%s: the written rule differs from the original", p)
		}
		n++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Error("no built-in rule was read")
	}
}

// A rule may have more than one reconciliation type: the first is the main one (Category), the
// others are also_categories. A rule with one type is written exactly as before.
func TestMoreThanOneReconciliationType(t *testing.T) {
	c := context(t)
	v, cts := c.Vocab, c.Checks

	one := validForm()
	oneYAML, err := rules.RuleYAML(&one, "1-office.1", "office")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(oneYAML), "also_categories") {
		t.Errorf("a rule with one type must not write also_categories:\n%s", oneYAML)
	}

	many := validForm()
	many.AlsoCategories = []string{"R2", "GST"}
	if ps := many.Problems(c); len(ps) != 0 {
		t.Fatalf("a rule with three types has problems: %v", problemCodes(ps))
	}
	body, err := rules.RuleYAML(&many, "1-office.1", "office")
	if err != nil {
		t.Fatal(err)
	}
	r, err := rules.Load("many", body, v, cts)
	if err != nil {
		t.Fatalf("the rule does not load: %v\n%s", err, body)
	}
	if r.Category != "R1" || !slices.Equal(r.AlsoCategories, []string{"R2", "GST"}) {
		t.Errorf("category %q also %v", r.Category, r.AlsoCategories)
	}
	back, err := rules.FormOf("many", body)
	if err != nil {
		t.Fatal(err)
	}
	if back.Category != "R1" || !slices.Equal(back.AlsoCategories, []string{"R2", "GST"}) {
		t.Errorf("the form read back has category %q also %v", back.Category, back.AlsoCategories)
	}

	for name, also := range map[string][]string{
		"an unknown type":     {"R99"},
		"the main type again": {"R1"},
		"a type chosen twice": {"R2", "R2"},
	} {
		f := validForm()
		f.AlsoCategories = also
		if ps := f.Problems(c); !hasProblem(ps, "BAD_CATEGORY", "category") {
			t.Errorf("%s: not refused: %v", name, problemCodes(ps))
		}
	}

	// The loader refuses the same slips in a rule file.
	bad := strings.Replace(string(body), `also_categories: [ "R2", "GST" ]`, `also_categories: [ "R1" ]`, 1)
	if _, err := rules.Load("bad", []byte(bad), v, cts); err == nil || !strings.Contains(err.Error(), "also_categories") {
		t.Errorf("the loader accepted the main type as an extra one: %v", err)
	}
}

// A rule file written from a form carries the version and source given, and a version with an
// office edit is only for an office rule (RU-12).
func TestOfficeVersionOnlyForOfficeRules(t *testing.T) {
	v := loadVocab(t)
	cts := checks.All()
	body, err := os.ReadFile(filepath.Join(builtinRules, "2025-26", "tieout", "RECON-DEP-001", "rule.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	office := strings.Replace(strings.Replace(string(body), `version: "1"`, `version: "1-office.1"`, 1), `source: "builtin"`, `source: "office"`, 1)
	if _, err := rules.Load("office.yaml", []byte(office), v, cts); err != nil {
		t.Errorf("an office version: %v", err)
	}
	builtin := strings.Replace(string(body), `version: "1"`, `version: "1-office.1"`, 1)
	if _, err := rules.Load("builtin.yaml", []byte(builtin), v, cts); err == nil || !strings.Contains(err.Error(), "only be in an office rule") {
		t.Errorf("an office version in a built-in rule: %v", err)
	}
	for _, bad := range []string{"0-office.1", "1-office.0", "1.1"} {
		if _, err := rules.Load("bad.yaml", []byte(strings.Replace(office, `version: "1-office.1"`, `version: "`+bad+`"`, 1)), v, cts); err == nil {
			t.Errorf("version %q was accepted", bad)
		}
	}
}

// The fixture written back from a loaded fixture reads back the same figures and results.
func TestFixtureWrittenBackReadsTheSame(t *testing.T) {
	v := loadVocab(t)
	cts := checks.All()
	body, err := os.ReadFile(filepath.Join(builtinRules, "2025-26", "tieout", "RECON-DEP-001", "rule.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(builtinRules, "2025-26", "tieout", "RECON-DEP-001", "rule.yaml")
	r, err := rules.Load(path, body, v, cts)
	if err != nil {
		t.Fatal(err)
	}
	fxs, err := rules.LoadFixtures(r, v)
	if err != nil || len(fxs.Pass) == 0 {
		t.Fatalf("fixtures: %v", err)
	}
	f := fxs.Pass[0]
	out := rules.FixtureYAML(f, f.Expected)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "again.yaml"), out, 0o644); err != nil {
		t.Fatal(err)
	}
	again, err := rules.LoadFixture(filepath.Join(dir, "again.yaml"), rules.StatusPass, out, r, v)
	if err != nil {
		t.Fatalf("written fixture does not load: %v\n%s", err, out)
	}
	if len(again.Facts) != len(f.Facts) || again.Facts[0].Value != f.Facts[0].Value || again.Facts[0].Locator.Path != f.Facts[0].Locator.Path {
		t.Errorf("figures changed: %+v vs %+v", again.Facts, f.Facts)
	}
	if len(again.Expected) != len(f.Expected) || again.Expected[0].Status != f.Expected[0].Status {
		t.Errorf("expected results changed: %+v vs %+v", again.Expected, f.Expected)
	}
}

// The categories file loads, lists every category the built-in rules use, and refuses a wrong
// schema.
func TestCategoriesListEveryBuiltInCategory(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(builtinRules, "categories.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	cats, err := rules.LoadCategories("categories.yaml", data)
	if err != nil {
		t.Fatal(err)
	}
	codes := rules.CategoryCodes(cats)
	v := loadVocab(t)
	err = filepath.WalkDir(builtinRules, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != rules.FileName {
			return err
		}
		body, _ := os.ReadFile(p)
		r, err := rules.Load(p, body, v, checks.All())
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(codes, r.Category) {
			t.Errorf("%s: category %q is not in categories.yaml", p, r.Category)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rules.LoadCategories("bad.yaml", []byte(`schema: "wrong/1"
categories:
  - code: "R1"
    name: "x"
`)); err == nil || !strings.Contains(err.Error(), "schema") {
		t.Errorf("a wrong schema was accepted: %v", err)
	}
}
