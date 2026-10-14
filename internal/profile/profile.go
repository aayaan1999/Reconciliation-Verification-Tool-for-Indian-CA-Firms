package profile

import (
	"cmp"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	jsonadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/json"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// FileName is the name of every profile file inside its folder,
// content/profiles/<source>/<document_type>/v<profile_version>/profile.yaml.
const FileName = "profile.yaml"

// A Profile is a loaded, validated format profile: where each figure sits in one market
// format, at one version (§A6.2). It is read-only and safe to share.
type Profile struct {
	ID           string // "generic.taxsuite.computation"
	Version      string // "1"
	SourceSystem string
	DocumentType string // a vocabulary document type
	FileType     string // an ingest adapter file type: "json"
	Regime       string
	File         string // the file it was loaded from, for messages

	detect  []predicate
	context contextMap
	facts   []factMap
	records []recordMap
}

// Key is "<id> v<version>", the way a profile is named in messages and reports.
func (p *Profile) Key() string { return p.ID + " v" + p.Version }

type predicate struct {
	kind  string // "json_exists" | "json_equals" | "json_matches"
	sel   ingest.Selector
	value string
	re    *regexp.Regexp
}

type contextMap struct {
	taxYear, pan, period, scope, part source
}

// source is a context value read from the file (sel) or fixed by the profile (constant).
type source struct {
	set      bool
	sel      ingest.Selector
	constant string
}

type factMap struct {
	fact     vocab.Fact
	sel      ingest.Selector
	unit     money.Unit
	negate   bool
	rounding money.Rounding // 0: the amount must be exact
	required bool
}

type recordMap struct {
	rt      vocab.RecordType
	rows    ingest.Selector
	unit    money.Unit
	fields  []column
	amounts []column
}

// column is one record column, at path rel inside the row (".gstin").
type column struct {
	name     string
	rel      string
	required bool
}

// selectorSyntax knows how to read the selectors of one file type.
type selectorSyntax struct {
	parse       func(string) (ingest.Selector, error)
	many        func(ingest.Selector) bool // can it select more than one cell?
	locatorKind string                     // the domain.Locator kind of its cells
}

// syntaxes holds the file types profiles can be written for. A new adapter adds itself here.
var syntaxes = map[string]selectorSyntax{
	jsonadapter.FileType: {
		parse: func(s string) (ingest.Selector, error) {
			p, err := jsonadapter.ParsePath(s)
			if err != nil {
				return nil, err
			}
			return p, nil
		},
		many:        func(s ingest.Selector) bool { return s.(jsonadapter.Path).HasWildcard() },
		locatorKind: domain.LocatorJSONPath,
	},
}

func supportedFileTypes() string {
	types := make([]string, 0, len(syntaxes))
	for ft := range syntaxes {
		types = append(types, ft)
	}
	slices.Sort(types)
	return strings.Join(types, ", ")
}

var (
	idPattern      = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z0-9_]+)+$`)
	versionPattern = regexp.MustCompile(`^[1-9][0-9]*$`)
)

// Load parses and validates one profile file against the vocabulary. Every problem is
// reported, each with file and line, as strictyaml.Errors.
func Load(name string, data []byte, v *vocab.Vocab) (*Profile, error) {
	var f fileYAML
	pos, err := strictyaml.Decode(name, data, &f)
	if err != nil {
		return nil, err
	}
	c := checker{file: name, pos: pos}
	p := &Profile{
		ID: f.ProfileID, Version: f.ProfileVersion, SourceSystem: f.SourceSystem,
		DocumentType: f.DocumentType, FileType: f.FileType, Regime: f.Regime, File: name,
	}

	if f.Schema != Schema {
		c.fail("schema", "schema must be %q, not %q", Schema, f.Schema)
	}
	if !idPattern.MatchString(f.ProfileID) {
		c.fail("profile_id", "profile_id %q must be lowercase words joined by dots, such as \"generic.taxsuite.computation\"", f.ProfileID)
	}
	if !versionPattern.MatchString(f.ProfileVersion) {
		c.fail("profile_version", "profile_version %q must be a whole number from 1, in quotes", f.ProfileVersion)
	}
	if strings.TrimSpace(f.SourceSystem) == "" {
		c.fail("source_system", "source_system must name the software that produces the format")
	}
	if _, ok := v.DocumentType(f.DocumentType); !ok {
		c.fail("document_type", "document_type %q is not in the vocabulary", f.DocumentType)
	}
	syn, ok := syntaxes[f.FileType]
	if !ok {
		c.fail("file_type", "file_type %q is not supported (supported: %s)", f.FileType, supportedFileTypes())
		return nil, c.errs // selectors cannot be checked without their syntax
	}
	c.syn = syn

	if len(f.Detect) == 0 {
		c.fail("detect", "detect must list at least one test, so the profile never matches every file")
	}
	for i, d := range f.Detect {
		at := fmt.Sprintf("detect[%d]", i)
		if pr, ok := c.predicate(at, d); ok {
			p.detect = append(p.detect, pr)
		}
	}

	p.context = c.context(f.Context)

	if len(f.Facts) == 0 && len(f.Records) == 0 {
		c.fail("", "the profile reads nothing: add facts or records")
	}
	seen := map[string]bool{}
	for i, fy := range f.Facts {
		at := fmt.Sprintf("facts[%d]", i)
		if fm, ok := c.fact(at, fy, v, f.DocumentType); ok {
			key := fy.Fact + "\x00" + fy.JSONPath
			if seen[key] {
				c.fail(at, "fact %q is already read from %q", fy.Fact, fy.JSONPath)
				continue
			}
			seen[key] = true
			p.facts = append(p.facts, fm)
		}
	}
	for i, ry := range f.Records {
		if rm, ok := c.records(fmt.Sprintf("records[%d]", i), ry, v); ok {
			p.records = append(p.records, rm)
		}
	}

	if len(c.errs) > 0 {
		c.errs.Sort()
		return nil, c.errs
	}
	return p, nil
}

type checker struct {
	file string
	pos  strictyaml.Positions
	syn  selectorSyntax
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

// selector parses a selector that must address a single cell.
func (c *checker) selector(at, text string) (ingest.Selector, bool) {
	if text == "" {
		c.fail(at, "%s is missing", lastKey(at))
		return nil, false
	}
	sel, err := c.syn.parse(text)
	if err != nil {
		c.fail(at, "%v", err)
		return nil, false
	}
	if c.syn.many(sel) {
		c.fail(at, "%q can select many values; a figure must come from exactly one place", text)
		return nil, false
	}
	return sel, true
}

func lastKey(path string) string {
	if i := strings.LastIndexByte(path, '.'); i >= 0 {
		return path[i+1:]
	}
	return path
}

func (c *checker) predicate(at string, d detectYAML) (predicate, bool) {
	n := 0
	for _, set := range []bool{d.JSONExists != nil, d.JSONEquals != nil, d.JSONMatches != nil} {
		if set {
			n++
		}
	}
	if n != 1 {
		c.fail(at, "each detect test must be exactly one of json_exists, json_equals, json_matches")
		return predicate{}, false
	}
	switch {
	case d.JSONExists != nil:
		sel, ok := c.selector(at+".json_exists.path", d.JSONExists.Path)
		return predicate{kind: "json_exists", sel: sel}, ok
	case d.JSONEquals != nil:
		sel, ok := c.selector(at+".json_equals.path", d.JSONEquals.Path)
		return predicate{kind: "json_equals", sel: sel, value: d.JSONEquals.Value}, ok
	}
	sel, ok := c.selector(at+".json_matches.path", d.JSONMatches.Path)
	re, err := regexp.Compile(`^(?:` + d.JSONMatches.Pattern + `)$`)
	if err != nil || d.JSONMatches.Pattern == "" {
		c.fail(at+".json_matches.pattern", "pattern %q is not a valid regular expression", d.JSONMatches.Pattern)
		return predicate{}, false
	}
	return predicate{kind: "json_matches", sel: sel, re: re}, ok
}

func (c *checker) context(cy contextYAML) contextMap {
	var m contextMap
	read := func(key string, sy *sourceYAML, required bool) source {
		at := "context." + key
		if sy == nil {
			if required {
				c.fail("context", "context.%s is required", key)
			}
			return source{}
		}
		switch {
		case sy.JSONPath != "" && sy.Const != "":
			c.fail(at, "context.%s must have json_path or const, not both", key)
		case sy.Const != "":
			return source{set: true, constant: sy.Const}
		case sy.JSONPath != "":
			sel, ok := c.selector(at+".json_path", sy.JSONPath)
			return source{set: ok, sel: sel}
		default:
			c.fail(at, "context.%s needs json_path or const", key)
		}
		return source{}
	}
	m.taxYear = read("tax_year", cy.TaxYear, true)
	m.period = read("period", cy.Period, true)
	m.pan = read("pan", cy.PAN, false)
	m.scope = read("scope", cy.Scope, false)
	m.part = read("part", cy.Part, false)

	if m.taxYear.set && m.taxYear.sel == nil {
		if _, err := period.ParseTaxYear(m.taxYear.constant); err != nil {
			c.fail("context.tax_year.const", "%v", err)
		}
	}
	if m.period.set && m.period.sel == nil && m.period.constant != PeriodWholeYear {
		if _, err := period.Parse(m.period.constant); err != nil {
			c.fail("context.period.const", "%v (or \"FY\" for the whole tax year)", err)
		}
	}
	return m
}

// PeriodWholeYear as context.period.const means the document covers its whole tax year.
const PeriodWholeYear = "FY"

func (c *checker) fact(at string, fy factYAML, v *vocab.Vocab, docType string) (factMap, bool) {
	vf, ok := v.Lookup(fy.Fact)
	switch {
	case !ok:
		c.fail(at+".fact", "fact %q is not in the vocabulary", fy.Fact)
		return factMap{}, false
	case vf.Deprecated():
		c.fail(at+".fact", "fact %q was retired in vocabulary version %d", fy.Fact, vf.DeprecatedIn)
		return factMap{}, false
	case vf.DocumentType != docType:
		c.fail(at+".fact", "fact %q belongs to document type %q, not %q", fy.Fact, vf.DocumentType, docType)
		return factMap{}, false
	}
	fm := factMap{fact: vf, required: fy.Required}
	sel, selOK := c.selector(at+".json_path", fy.JSONPath)
	fm.sel = sel
	ok = selOK

	if vf.Kind != domain.KindMoney {
		for _, kv := range [][2]string{{"unit", fy.Unit}, {"sign", fy.Sign}, {"rounding", fy.Rounding}} {
			if kv[1] != "" {
				c.fail(at+"."+kv[0], "%s applies only to money facts; %q is %s", kv[0], fy.Fact, vf.Kind)
				ok = false
			}
		}
		return fm, ok
	}
	unit, uok := c.unit(at+".unit", fy.Unit, true)
	fm.unit = unit
	switch fy.Sign {
	case "", "as_is":
	case "negate":
		fm.negate = true
	default:
		c.fail(at+".sign", "sign %q must be \"as_is\" or \"negate\"", fy.Sign)
		ok = false
	}
	if fy.Rounding != "" {
		r, err := money.ParseRounding(fy.Rounding)
		if err != nil || r == money.RoundNone {
			c.fail(at+".rounding", "rounding %q is not a rounding name such as \"half_up_paisa\"; leave it out to require exact amounts", fy.Rounding)
			ok = false
		}
		fm.rounding = r
	}
	return fm, ok && uok
}

func (c *checker) unit(at, text string, required bool) (money.Unit, bool) {
	switch text {
	case "rupees":
		return money.UnitRupees, true
	case "paise":
		return money.UnitPaise, true
	case "":
		if !required {
			return 0, true
		}
		c.fail(at, "unit is required for money: \"rupees\" or \"paise\"")
	default:
		c.fail(at, "unit %q must be \"rupees\" or \"paise\"", text)
	}
	return 0, false
}

func (c *checker) records(at string, ry recordsYAML, v *vocab.Vocab) (recordMap, bool) {
	rt, ok := v.RecordType(ry.RecordType)
	switch {
	case !ok:
		c.fail(at+".record_type", "record type %q is not in the vocabulary", ry.RecordType)
		return recordMap{}, false
	case rt.Deprecated():
		c.fail(at+".record_type", "record type %q was retired in vocabulary version %d", ry.RecordType, rt.DeprecatedIn)
		return recordMap{}, false
	}
	rm := recordMap{rt: rt}
	ok = true
	if ry.Rows == "" {
		c.fail(at, "rows is missing")
		ok = false
	} else if sel, err := c.syn.parse(ry.Rows); err != nil {
		c.fail(at+".rows", "%v", err)
		ok = false
	} else if !c.syn.many(sel) {
		c.fail(at+".rows", "rows %q must select every row, for example \"$.invoices[*]\"", ry.Rows)
		ok = false
	} else {
		rm.rows = sel
	}
	unit, uok := c.unit(at+".unit", ry.Unit, len(ry.Amounts) > 0)
	rm.unit, ok = unit, ok && uok

	allowed := map[string]string{}
	for _, n := range rt.MatchKey {
		allowed[n] = "field"
	}
	for _, n := range rt.Fields {
		allowed[n] = "field"
	}
	for _, n := range rt.Amounts {
		allowed[n] = "amount"
	}
	mapped := map[string]bool{}
	cols := func(list string, ys []columnYAML, want string) []column {
		var out []column
		for i, cy := range ys {
			cat := fmt.Sprintf("%s.%s[%d]", at, list, i)
			switch {
			case allowed[cy.Name] == "":
				c.fail(cat+".name", "%q is not a column of record type %q", cy.Name, rt.Name)
				ok = false
				continue
			case allowed[cy.Name] != want:
				c.fail(cat+".name", "%q is a%s column of %q; list it under %ss", cy.Name, article(allowed[cy.Name]), rt.Name, allowed[cy.Name])
				ok = false
				continue
			case mapped[cy.Name]:
				c.fail(cat+".name", "column %q is mapped twice", cy.Name)
				ok = false
				continue
			}
			mapped[cy.Name] = true
			if !strings.HasPrefix(cy.Path, ".") && !strings.HasPrefix(cy.Path, "[") {
				c.fail(cat+".path", "path %q must be written from the row, starting with '.' or '[' (\".supplier_gstin\")", cy.Path)
				ok = false
				continue
			}
			if _, good := c.selector(cat+".path", "$"+cy.Path); !good {
				ok = false
				continue
			}
			out = append(out, column{name: cy.Name, rel: cy.Path, required: cy.Required})
		}
		return out
	}
	rm.fields = cols("fields", ry.Fields, "field")
	rm.amounts = cols("amounts", ry.Amounts, "amount")
	for _, k := range rt.MatchKey {
		if !mapped[k] {
			c.fail(at, "match-key column %q of %q is not mapped", k, rt.Name)
			ok = false
		}
	}
	for _, col := range rm.fields {
		if slices.Contains(rt.MatchKey, col.name) && !col.required {
			c.fail(at, "match-key column %q must be required: true", col.name)
			ok = false
		}
	}
	return rm, ok
}

func article(word string) string {
	if strings.IndexByte("aeiou", word[0]) >= 0 {
		return "n " + word
	}
	return " " + word
}

// A Set is every installed profile, sorted by ID then version number.
type Set struct {
	profiles []*Profile
}

// NewSet builds a set, refusing two profiles with the same ID and version.
func NewSet(profiles ...*Profile) (*Set, error) {
	s := &Set{profiles: slices.Clone(profiles)}
	slices.SortFunc(s.profiles, func(a, b *Profile) int {
		av, _ := strconv.Atoi(a.Version)
		bv, _ := strconv.Atoi(b.Version)
		return cmp.Or(cmp.Compare(a.ID, b.ID), cmp.Compare(av, bv))
	})
	for i := 1; i < len(s.profiles); i++ {
		if a, b := s.profiles[i-1], s.profiles[i]; a.Key() == b.Key() {
			return nil, fmt.Errorf("profile %s is defined twice: %s and %s", a.Key(), a.File, b.File)
		}
	}
	return s, nil
}

// Profiles returns every profile, sorted by ID then version.
func (s *Set) Profiles() []*Profile { return slices.Clone(s.profiles) }

// Lookup returns the profile with this ID and version.
func (s *Set) Lookup(id, version string) (*Profile, bool) {
	for _, p := range s.profiles {
		if p.ID == id && p.Version == version {
			return p, true
		}
	}
	return nil, false
}

// LoadDir loads every profile under root. Each must sit at
// <source>/<document_type>/v<profile_version>/profile.yaml, so the folder layout and the
// file can never disagree. Every problem in every file is reported.
func LoadDir(root string, v *vocab.Vocab) (*Set, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == FileName {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(files)
	var profiles []*Profile
	var errs strictyaml.Errors
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		name := filepath.ToSlash(file)
		p, err := Load(name, data, v)
		if err != nil {
			errs = append(errs, err.(strictyaml.Errors)...)
			continue
		}
		rel, _ := filepath.Rel(root, filepath.Dir(file))
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) != 3 || parts[1] != p.DocumentType || parts[2] != "v"+p.Version {
			errs = append(errs, &strictyaml.Error{File: name, Msg: fmt.Sprintf(
				"a profile must sit at <source>/%s/v%s/%s under %s", p.DocumentType, p.Version, FileName, filepath.ToSlash(root))})
			continue
		}
		profiles = append(profiles, p)
	}
	if len(errs) > 0 {
		errs.Sort()
		return nil, errs
	}
	return NewSet(profiles...)
}
