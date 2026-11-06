package profile

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
)

// Profile tests (IN-22, technical spec §B2.3). Next to each profile.yaml:
//
//	samples/<name>.<ext>          a sample file of the format (synthetic until real ones arrive)
//	expected/<name>.facts.yaml    the hand-verified result of reading it
//
// A sample passes when it is recognised as this profile and nothing else, reads without
// errors, and gives exactly the expected context, facts and records: same names, values,
// periods and locators.

// ExpectedSchema is the value of the "schema" key of every expected-facts file.
const ExpectedSchema = "recon.expected/1"

const (
	samplesDir     = "samples"
	expectedDir    = "expected"
	expectedSuffix = ".facts.yaml"
)

type expectedYAML struct {
	Schema         string               `yaml:"schema"`
	ProfileID      string               `yaml:"profile_id"`
	ProfileVersion string               `yaml:"profile_version"`
	Context        expectedContextYAML  `yaml:"context"`
	Facts          []expectedFactYAML   `yaml:"facts"`
	Records        []expectedRecordYAML `yaml:"records"`
}

type expectedContextYAML struct {
	TaxYear  string `yaml:"tax_year"`
	Period   string `yaml:"period"`
	PANLast4 string `yaml:"pan_last4"`
	Scope    string `yaml:"scope"`
	Part     string `yaml:"part"`
}

// Values are written as text: money in rupees with two decimals ("112450.50"), rates as
// a percentage with two decimals ("12.50"), dates as YYYY-MM-DD, bools as "true"/"false".
type expectedFactYAML struct {
	Fact    string `yaml:"fact"`
	Value   string `yaml:"value"`
	Period  string `yaml:"period"`
	Locator string `yaml:"locator"`
}

type expectedRecordYAML struct {
	RecordType string           `yaml:"record_type"`
	MatchKey   string           `yaml:"match_key"`
	Locator    string           `yaml:"locator"`
	Fields     []namedValueYAML `yaml:"fields"`
	Amounts    []namedValueYAML `yaml:"amounts"`
}

type namedValueYAML struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// A SampleResult is the outcome of one sample. It passed when Problems is empty.
type SampleResult struct {
	Profile  string // Profile.Key()
	Sample   string // path of the sample file
	Problems []string
}

// TestSamples runs every sample of p. Detection is tried against the whole set, so a
// sample that another profile would also claim fails here, before any client file can.
func TestSamples(p *Profile, set *Set, reg *ingest.Registry) []SampleResult {
	dir := filepath.Dir(p.File)
	samples, _ := filepath.Glob(filepath.Join(dir, samplesDir, "*"))
	expected, _ := filepath.Glob(filepath.Join(dir, expectedDir, "*"+expectedSuffix))
	slices.Sort(samples)
	if len(samples) == 0 {
		return []SampleResult{{Profile: p.Key(), Sample: filepath.ToSlash(filepath.Join(dir, samplesDir)),
			Problems: []string{"the profile has no samples; every profile must be tested against at least one (IN-22)"}}}
	}
	var out []SampleResult
	want := map[string]bool{}
	for _, s := range samples {
		base := strings.TrimSuffix(filepath.Base(s), filepath.Ext(s))
		want[base] = true
		exp := filepath.Join(dir, expectedDir, base+expectedSuffix)
		out = append(out, SampleResult{Profile: p.Key(), Sample: filepath.ToSlash(s), Problems: testSample(p, set, reg, s, exp)})
	}
	for _, e := range expected {
		if base := strings.TrimSuffix(filepath.Base(e), expectedSuffix); !want[base] {
			out = append(out, SampleResult{Profile: p.Key(), Sample: filepath.ToSlash(e),
				Problems: []string{"expected file has no sample of the same name in " + samplesDir}})
		}
	}
	return out
}

func testSample(p *Profile, set *Set, reg *ingest.Registry, sample, expFile string) []string {
	data, err := os.ReadFile(sample)
	if err != nil {
		return []string{err.Error()}
	}
	expData, err := os.ReadFile(expFile)
	if err != nil {
		return []string{fmt.Sprintf("no expected file %s", filepath.ToSlash(expFile))}
	}
	var exp expectedYAML
	if _, err := strictyaml.Decode(filepath.ToSlash(expFile), expData, &exp); err != nil {
		return strings.Split(err.Error(), "\n")
	}
	var problems []string
	if exp.Schema != ExpectedSchema {
		problems = append(problems, fmt.Sprintf("expected file schema must be %q", ExpectedSchema))
	}
	if exp.ProfileID != p.ID || exp.ProfileVersion != p.Version {
		problems = append(problems, fmt.Sprintf("expected file is for %s v%s, not %s", exp.ProfileID, exp.ProfileVersion, p.Key()))
	}

	name := filepath.Base(sample)
	ad, err := reg.ForFile(data[:min(len(data), 512)], name)
	if err != nil {
		return append(problems, err.Error())
	}
	doc, err := ad.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return append(problems, err.Error())
	}
	// A sample of a bundle (a file that holds several documents) is recognised as every
	// profile of the bundle; this profile must be one of them.
	got, err := DetectAll(set, doc, name)
	if err != nil {
		return append(problems, err.Error())
	}
	// A superseded version's sample is now read by the version that supersedes it (SPEC-015),
	// or by a later version in the chain (SPEC-018: version 3 supersedes 2, which supersedes 1);
	// the superseded version is still tested on it, as it reads the drafts it read before.
	superseded := slices.ContainsFunc(got, func(g *Profile) bool {
		gv, e1 := strconv.Atoi(g.Version)
		pv, e2 := strconv.Atoi(p.Version)
		return g.ID == p.ID && g.Supersedes != "" && (g.Supersedes == p.Version || (e1 == nil && e2 == nil && gv > pv))
	})
	if !slices.Contains(got, p) && !superseded {
		keys := make([]string, len(got))
		for i, g := range got {
			keys[i] = g.Key()
		}
		return append(problems, fmt.Sprintf("recognised as %s, not %s", strings.Join(keys, ", "), p.Key()))
	}
	// A file with no tax year of its own (context.tax_year from_run) is read for the tax year the
	// expected file names, as a run would give it.
	src := Source{FileName: name}
	if p.context.taxYear.fromRun && exp.Context.TaxYear != "" {
		ty, err := period.ParseTaxYear(exp.Context.TaxYear)
		if err != nil {
			return append(problems, fmt.Sprintf("expected file: context tax_year %q is not a tax year: %v", exp.Context.TaxYear, err))
		}
		src.TaxYear = ty
	}
	res, err := Extract(p, doc, src)
	if err != nil {
		return append(problems, err.Error())
	}
	for _, e := range res.Report.Errors {
		problems = append(problems, fmt.Sprintf("validation error %s at %s: %s", e.Code, e.Locator.Path, e.Msg))
	}

	ctx := res.Context
	for _, c := range []struct{ what, want, got string }{
		{"tax_year", exp.Context.TaxYear, ctx.TaxYear.String()},
		{"period", exp.Context.Period, ctx.Period},
		{"pan_last4", exp.Context.PANLast4, res.Report.PANLast4},
		{"scope", exp.Context.Scope, ctx.Scope},
		{"part", exp.Context.Part, ctx.Part},
	} {
		if ctx.TaxYear.IsZero() && c.what == "tax_year" {
			c.got = ""
		}
		if c.want != c.got {
			problems = append(problems, fmt.Sprintf("context %s: expected %q, got %q", c.what, c.want, c.got))
		}
	}
	problems = append(problems, compare("fact", factLines(exp.Facts), gotFactLines(res.Facts))...)
	problems = append(problems, compare("record", recordLines(exp.Records), gotRecordLines(res.Records))...)
	return problems
}

// line is one fact or record in a comparable form: key names it, text is everything else.
type line struct{ key, text string }

func factLines(fs []expectedFactYAML) []line {
	out := make([]line, len(fs))
	for i, f := range fs {
		out[i] = line{f.Fact, fmt.Sprintf("value %s, period %s, at %s", f.Value, f.Period, f.Locator)}
	}
	return out
}

func gotFactLines(fs []domain.Fact) []line {
	out := make([]line, len(fs))
	for i, f := range fs {
		out[i] = line{f.Name, fmt.Sprintf("value %s, period %s, at %s", domain.FormatValue(f.Value), f.Period, f.Locator.Path)}
	}
	return out
}

func recordLines(rs []expectedRecordYAML) []line {
	out := make([]line, len(rs))
	for i, r := range rs {
		var parts []string
		for _, f := range sortedNamed(r.Fields) {
			parts = append(parts, f.Name+"="+strconv.Quote(f.Value))
		}
		for _, a := range sortedNamed(r.Amounts) {
			parts = append(parts, a.Name+"="+a.Value)
		}
		out[i] = line{r.RecordType + " " + r.MatchKey + " at " + r.Locator, strings.Join(parts, ", ")}
	}
	return out
}

func sortedNamed(vs []namedValueYAML) []namedValueYAML {
	vs = slices.Clone(vs)
	slices.SortFunc(vs, func(a, b namedValueYAML) int { return strings.Compare(a.Name, b.Name) })
	return vs
}

func gotRecordLines(rs []domain.Record) []line {
	out := make([]line, len(rs))
	for i, r := range rs {
		var parts []string
		for _, k := range sortedKeys(r.Fields) {
			parts = append(parts, k+"="+strconv.Quote(r.Fields[k]))
		}
		for _, k := range sortedKeys(r.Amounts) {
			parts = append(parts, k+"="+domain.FormatValue(domain.MoneyValue(r.Amounts[k])))
		}
		out[i] = line{r.RecordType + " " + r.MatchKey + " at " + r.Locator.Path, strings.Join(parts, ", ")}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// compare reports every difference between the expected and actual lines, by key.
func compare(what string, want, got []line) []string {
	var problems []string
	g := map[string][]string{}
	for _, l := range got {
		g[l.key] = append(g[l.key], l.text)
	}
	w := map[string][]string{}
	for _, l := range want {
		w[l.key] = append(w[l.key], l.text)
	}
	for _, k := range sortedKeys(w) {
		switch gt, ok := g[k]; {
		case !ok:
			problems = append(problems, fmt.Sprintf("%s %s: expected %s, but it was not read", what, k, strings.Join(w[k], "; ")))
		case !slices.Equal(gt, w[k]):
			problems = append(problems, fmt.Sprintf("%s %s: expected %s, got %s", what, k, strings.Join(w[k], "; "), strings.Join(gt, "; ")))
		}
	}
	for _, k := range sortedKeys(g) {
		if _, ok := w[k]; !ok {
			problems = append(problems, fmt.Sprintf("%s %s: read %s, but it is not in the expected file", what, k, strings.Join(g[k], "; ")))
		}
	}
	return problems
}
