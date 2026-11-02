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
	csvadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/csv"
	jsonadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/json"
	xlsxadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/xlsx"
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
	// Bundle and Section are set together, for a file that holds several documents (such
	// as a tax suite's filing pack): every profile of the bundle reads its own section of
	// the same file, as its own document (SPEC-005).
	Bundle  string
	Section string

	tables  []xlsxadapter.Table // xlsx only: the sheets read, each as a table (SPEC-008)
	detect  []predicate
	context contextMap
	facts   []factMap
	records []recordMap
}

// Key is "<id> v<version>", the way a profile is named in messages and reports.
func (p *Profile) Key() string { return p.ID + " v" + p.Version }

// ReadsScope reports whether the profile reads the document's scope (the client's GSTIN)
// from the file or fixes it.
func (p *Profile) ReadsScope() bool { return p.context.scope.set }

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
	set        bool
	sel        ingest.Selector
	constant   string
	format     string // period only: "mmyyyy" | "yyyymm"; "" when canonical
	fromPeriod bool   // tax_year only
	fromMonth  bool   // tax_year only: the value is a month ("2025-04"); the tax year holds it
	fromGSTIN  bool   // pan only
}

// Period formats a profile may name for context.period.
var periodFormats = []string{"mmyyyy", "yyyymm"}

// Date formats a profile may name for a record column.
var dateFormats = []string{"dd-mm-yyyy", "yyyy-mm-dd"}

type factMap struct {
	fact vocab.Fact
	sel  ingest.Selector
	// From the entries of a list (SPEC-006 part 2): rows selects the entries, where keeps
	// those that meet every condition, rel is the figure's path inside an entry.
	rows     ingest.Selector
	rel      string
	where    []rowFilter
	combine  string // "one" | "sum" | "latest"
	by       string // latest: the date column, inside the entry
	unit     money.Unit
	negate   bool
	rounding money.Rounding // 0: the amount must be exact
	required bool
}

type recordMap struct {
	rt      vocab.RecordType
	rows    ingest.Selector
	where   []rowFilter
	unit    money.Unit
	fields  []column
	amounts []column
}

// rowFilter keeps a row only when the value at rel equals equals, or equals the value at
// equalsSel (a place elsewhere in the file, such as the assessment year).
type rowFilter struct {
	rel       string
	equals    string
	equalsSel ingest.Selector
	in        []string // any one of these, instead of equals
}

// Ways of reading one figure from the entries of a list that meet its conditions.
const (
	CombineOne    = "one"
	CombineSum    = "sum"
	CombineLatest = "latest"
)

// column is one record column, at path rel inside the row (".gstin"), after going up `up`
// levels from the row.
type column struct {
	name       string
	rel        string
	up         int
	many       bool // the path has [*]: amounts are added up, field values joined
	required   bool
	dateFormat string
}

// selectorSyntax knows how to read the selectors of one file type.
type selectorSyntax struct {
	parse       func(string) (ingest.Selector, error)
	many        func(ingest.Selector) bool // can it select more than one cell?
	locatorKind string                     // the domain.Locator kind of its cells
}

// jsonPathSyntax is the selector syntax of JSON paths ("$.a.b", "$[*]", ".['Col']"), shared
// by every file type whose paths are written like JSON ones.
var jsonPathSyntax = selectorSyntax{
	parse: func(s string) (ingest.Selector, error) {
		p, err := jsonadapter.ParsePath(s)
		if err != nil {
			return nil, err
		}
		return p, nil
	},
	many:        func(s ingest.Selector) bool { return s.(jsonadapter.Path).HasWildcard() },
	locatorKind: domain.LocatorJSONPath,
}

// syntaxes holds the file types profiles can be written for. A new adapter adds itself here.
var syntaxes = map[string]selectorSyntax{
	jsonadapter.FileType: jsonPathSyntax,
	// A CSV file is read as a list of rows keyed by column name, so its paths are written
	// like JSON ones: "$[*]", ".amount_inr" (SPEC-006 part 3).
	csvadapter.FileType: jsonPathSyntax,
	// An Excel file is read as the tables its profile names, each sheet a list of rows keyed
	// by header text, so its paths are written like JSON ones too: "$['Sales Register'][*]",
	// ".['Voucher No']" (SPEC-008). Locators become "Sales Register!C14" once read.
	xlsxadapter.FileType: jsonPathSyntax,
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
		Bundle: f.Bundle, Section: f.Section,
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

	p.tables = c.sheets(f)

	switch {
	case f.FileType == xlsxadapter.FileType && f.Bundle != "":
		c.fail("bundle", "an Excel profile reads its own sheets; bundle and section are for JSON files that hold several documents")
	case (f.Bundle == "") != (f.Section == ""):
		c.fail("bundle", "bundle and section go together: a profile of a file that holds several documents names both")
	case f.Bundle != "" && !idPattern.MatchString(f.Bundle):
		c.fail("bundle", "bundle %q must be lowercase words joined by dots, such as \"taxsuite.filing_pack\"", f.Bundle)
	case f.Section != "":
		c.selector("section", f.Section)
	}

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
	// A profile of a bundle reads its rows inside its section. A figure may come from outside
	// it (the assessee's status, kept once for the whole file, SPEC-006).
	if f.Section != "" {
		for i, ry := range f.Records {
			if ry.Rows != "" && !InSection(ry.Rows, f.Section) {
				c.fail(fmt.Sprintf("records[%d].rows", i), "%q is outside the profile's section %q", ry.Rows, f.Section)
			}
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

// sheets reads the sheets of an Excel profile: at least one, each named once, each with at
// least one header label, every label named once. Every path of the profile from the top of
// the file must start in one of its sheets, so a misspelt sheet is found when it loads.
func (c *checker) sheets(f fileYAML) []xlsxadapter.Table {
	if f.FileType != xlsxadapter.FileType {
		if len(f.Sheets) > 0 {
			c.fail("sheets", "sheets is for Excel (xlsx) profiles only")
		}
		return nil
	}
	if len(f.Sheets) == 0 {
		c.fail("", "an Excel profile names the sheets it reads under sheets, each with the labels of its header row")
		return nil
	}
	var out []xlsxadapter.Table
	for i, sy := range f.Sheets {
		at := fmt.Sprintf("sheets[%d]", i)
		name := strings.TrimSpace(sy.Name)
		switch {
		case name == "" || name != sy.Name:
			c.fail(at+".name", "name must be the sheet's name exactly as the workbook shows it, with no spaces at either end")
			continue
		case slices.ContainsFunc(out, func(t xlsxadapter.Table) bool { return t.Sheet == name }):
			c.fail(at+".name", "sheet %q is named twice", name)
			continue
		case len(sy.Header) == 0:
			c.fail(at+".header", "header lists at least one label of the sheet's header row, so the row can be found")
			continue
		}
		t := xlsxadapter.Table{Sheet: name, AnyName: sy.AnyName}
		for j, l := range sy.TotalRows {
			l = xlsxadapter.Label(l)
			switch {
			case l == "":
				c.fail(fmt.Sprintf("%s.total_rows[%d]", at, j), "a total row text cannot be empty")
			case slices.ContainsFunc(t.Totals, func(o string) bool { return strings.EqualFold(o, l) }):
				c.fail(fmt.Sprintf("%s.total_rows[%d]", at, j), "total row text %q is named twice", l)
			default:
				t.Totals = append(t.Totals, l)
			}
		}
		for j, l := range sy.Header {
			l = xlsxadapter.Label(l)
			switch {
			case l == "":
				c.fail(fmt.Sprintf("%s.header[%d]", at, j), "a header label cannot be empty")
			case slices.ContainsFunc(t.Header, func(o string) bool { return strings.EqualFold(o, l) }):
				c.fail(fmt.Sprintf("%s.header[%d]", at, j), "header label %q is named twice", l)
			default:
				t.Header = append(t.Header, l)
			}
		}
		out = append(out, t)
	}
	labels := labelsOf(f)
	for i := range out {
		out[i].Labels = labels
	}
	inSheet := func(path string) bool { // "$['Sales Register']..." or, for a plain name, "$.Ledger..."
		return slices.ContainsFunc(out, func(t xlsxadapter.Table) bool {
			return InSection(path, xlsxadapter.SheetPath(t.Sheet)) || InSection(path, "$."+t.Sheet)
		})
	}
	for _, p := range profilePaths(f) {
		if p.top && p.path != "" && !inSheet(p.path) {
			c.fail(p.at, "%q does not start in a sheet this profile names (write it as \"$['<sheet>'][*]...\")", p.path)
		}
	}
	return out
}

// profilePath is one path written in a profile: where it is written (for messages), the
// path, and whether it is read from the top of the file (rather than from a row).
type profilePath struct {
	at, path string
	top      bool
}

// profilePaths lists every path a profile writes, in the order the profile writes them:
// detect, context, facts (rows, json_path, by, where) and records (rows, where, columns).
// sheets and labelsOf both walk this one list, so a path key added to the profile YAML is
// added here once. Empty paths are included; callers skip them.
func profilePaths(f fileYAML) []profilePath {
	var out []profilePath
	add := func(top bool, path, format string, args ...any) {
		out = append(out, profilePath{at: fmt.Sprintf(format, args...), path: path, top: top})
	}
	for i, d := range f.Detect {
		if d.JSONExists != nil {
			add(true, d.JSONExists.Path, "detect[%d].json_exists.path", i)
		}
		if d.JSONEquals != nil {
			add(true, d.JSONEquals.Path, "detect[%d].json_equals.path", i)
		}
		if d.JSONMatches != nil {
			add(true, d.JSONMatches.Path, "detect[%d].json_matches.path", i)
		}
	}
	cy := f.Context
	for _, ks := range []struct {
		key string
		sy  *sourceYAML
	}{{"tax_year", cy.TaxYear}, {"pan", cy.PAN}, {"period", cy.Period}, {"scope", cy.Scope}, {"part", cy.Part}} {
		if ks.sy != nil {
			add(true, ks.sy.JSONPath, "context.%s.json_path", ks.key)
		}
	}
	where := func(at string, ws []whereYAML) {
		for j, w := range ws {
			add(false, w.Path, "%s.where[%d].path", at, j)
			add(false, w.EqualsPath, "%s.where[%d].equals_path", at, j)
		}
	}
	for i, fy := range f.Facts {
		at := fmt.Sprintf("facts[%d]", i)
		// A fact read from rows has its json_path written from the row; otherwise json_path
		// is read from the top of the file.
		add(fy.Rows != "", fy.Rows, "%s.rows", at)
		add(fy.Rows == "", fy.JSONPath, "%s.json_path", at)
		add(false, fy.By, "%s.by", at)
		where(at, fy.Where)
	}
	for i, ry := range f.Records {
		at := fmt.Sprintf("records[%d]", i)
		add(true, ry.Rows, "%s.rows", at)
		where(at, ry.Where)
		for j, c := range slices.Concat(ry.Fields, ry.Amounts) {
			add(false, c.Path, "%s.columns[%d].path", at, j)
		}
	}
	return out
}

// labelsOf lists every key the paths of an Excel profile name: sheet names and the header
// labels it reads, so a column whose header differs only in capitals is keyed by the
// profile's spelling (SPEC-008).
func labelsOf(f fileYAML) []string {
	var out []string
	for _, pp := range profilePaths(f) {
		p := strings.TrimLeft(pp.path, "^")
		if p == "" {
			continue
		}
		if !strings.HasPrefix(p, "$") {
			p = "$" + p // a path written from the row
		}
		if sel, err := jsonadapter.ParsePath(p); err == nil {
			for _, k := range sel.Keys() {
				if !slices.Contains(out, k) {
					out = append(out, k)
				}
			}
		}
	}
	return out
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
		case sy.Format != "" && key != "period":
			c.fail(at+".format", "format is for context.period only")
		case sy.Format != "" && !slices.Contains(periodFormats, sy.Format):
			c.fail(at+".format", "period format %q must be one of: %s", sy.Format, strings.Join(periodFormats, ", "))
		case sy.FromPeriod && key != "tax_year":
			c.fail(at+".from_period", "from_period is for context.tax_year only")
		case sy.FromMonth && (key != "tax_year" || sy.JSONPath == ""):
			c.fail(at+".from_month", "from_month is for context.tax_year, with the json_path of a month")
		case sy.FromGSTIN && key != "pan":
			c.fail(at+".from_gstin", "from_gstin is for context.pan only")
		}
		switch {
		case sy.FromPeriod && (sy.JSONPath != "" || sy.Const != ""):
			c.fail(at, "context.%s with from_period takes no json_path or const", key)
		case sy.FromPeriod:
			return source{set: true, fromPeriod: true}
		case sy.JSONPath != "" && sy.Const != "":
			c.fail(at, "context.%s must have json_path or const, not both", key)
		case sy.Const != "":
			return source{set: true, constant: sy.Const, format: sy.Format, fromGSTIN: sy.FromGSTIN}
		case sy.JSONPath != "":
			sel, ok := c.selector(at+".json_path", sy.JSONPath)
			return source{set: ok, sel: sel, format: sy.Format, fromGSTIN: sy.FromGSTIN, fromMonth: sy.FromMonth}
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

	if m.taxYear.fromPeriod && m.period.set && m.period.sel == nil && m.period.constant == PeriodWholeYear {
		c.fail("context.tax_year.from_period", "the tax year cannot be worked out from a period of \"FY\"")
	}
	if m.taxYear.set && m.taxYear.sel == nil && !m.taxYear.fromPeriod {
		if _, err := period.ParseTaxYear(m.taxYear.constant); err != nil {
			c.fail("context.tax_year.const", "%v", err)
		}
	}
	if m.period.set && m.period.sel == nil && m.period.constant != PeriodWholeYear {
		p := m.period.constant
		var err error
		if m.period.format != "" {
			// A fixed period in a format is checked now, not first when a file is read.
			if p, err = canonicalPeriod(p, m.period.format); err != nil {
				c.fail("context.period.const", "%q is not a period written %s", m.period.constant, m.period.format)
			}
		}
		if err == nil {
			if _, err := period.Parse(p); err != nil {
				c.fail("context.period.const", "%v (or \"FY\" for the whole tax year)", err)
			}
		}
	}
	return m
}

// rowFilter reads one where-condition: a path written from the row, and either a value, a
// list of values any one of which it may be, or the path of another place in the file
// whose value it must equal.
func (c *checker) rowFilter(at string, w whereYAML) (rowFilter, bool) {
	f := rowFilter{rel: w.Path, equals: w.Equals, in: w.In}
	set := 0
	for _, on := range []bool{w.Equals != "", w.EqualsPath != "", w.In != nil} {
		if on {
			set++
		}
	}
	switch {
	case set != 1:
		c.fail(at, "a condition has one of equals, equals_path or in")
		return f, false
	case w.In != nil && (len(w.In) < 2 || slices.Contains(w.In, "")):
		c.fail(at+".in", "in lists at least two values, none of them empty (for one value, use equals)")
		return f, false
	case w.EqualsPath != "":
		sel, ok := c.selector(at+".equals_path", w.EqualsPath)
		f.equalsSel = sel
		return f, ok
	}
	return f, true
}

// factRows reads a figure taken from the entries of a list that meet conditions.
func (c *checker) factRows(at string, fy factYAML, fm *factMap) bool {
	ok := true
	sel, err := c.syn.parse(fy.Rows)
	switch {
	case err != nil:
		c.fail(at+".rows", "%v", err)
		ok = false
	case !c.syn.many(sel):
		c.fail(at+".rows", "rows %q must select every entry, for example \"$.list[*]\"", fy.Rows)
		ok = false
	default:
		fm.rows = sel
	}
	relOK := func(key, rel string) bool {
		if !strings.HasPrefix(rel, ".") && !strings.HasPrefix(rel, "[") {
			c.fail(at+"."+key, "%s %q must be written from the entry, starting with '.' or '['", key, rel)
			return false
		}
		_, good := c.selector(at+"."+key, "$"+rel)
		return good
	}
	if !relOK("json_path", fy.JSONPath) {
		ok = false
	}
	fm.rel = fy.JSONPath
	for i, w := range fy.Where {
		wat := fmt.Sprintf("%s.where[%d]", at, i)
		if !relOK("where", w.Path) {
			ok = false
			continue
		}
		f, good := c.rowFilter(wat, w)
		if !good {
			ok = false
			continue
		}
		fm.where = append(fm.where, f)
	}
	fm.combine, fm.by = fy.Combine, fy.By
	switch fy.Combine {
	case CombineOne:
	case CombineSum:
		if fm.fact.Kind != domain.KindMoney {
			c.fail(at+".combine", "sum adds up money; %q is %s", fy.Fact, fm.fact.Kind)
			ok = false
		}
	case CombineLatest:
		if fy.By == "" || !relOK("by", fy.By) {
			c.fail(at+".by", "latest needs by: the date column of an entry, such as \".due\"")
			ok = false
		}
	default:
		c.fail(at+".combine", "combine %q must be \"one\", \"sum\" or \"latest\"", fy.Combine)
		ok = false
	}
	if fy.Combine != CombineLatest && fy.By != "" {
		c.fail(at+".by", "by goes with combine \"latest\"")
		ok = false
	}
	return ok
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
	if fy.Rows != "" {
		ok = c.factRows(at, fy, &fm)
	} else {
		if len(fy.Where) > 0 || fy.Combine != "" || fy.By != "" {
			c.fail(at, "where, combine and by go with rows")
		}
		sel, selOK := c.selector(at+".json_path", fy.JSONPath)
		fm.sel = sel
		ok = selOK
	}

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
	for i, w := range ry.Where {
		wat := fmt.Sprintf("%s.where[%d]", at, i)
		if !strings.HasPrefix(w.Path, ".") && !strings.HasPrefix(w.Path, "[") {
			c.fail(wat+".path", "path %q must be written from the row, starting with '.' or '['", w.Path)
			ok = false
			continue
		}
		if _, good := c.selector(wat+".path", "$"+w.Path); !good {
			ok = false
			continue
		}
		f, good := c.rowFilter(wat, w)
		if !good {
			ok = false
			continue
		}
		rm.where = append(rm.where, f)
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
			rel := strings.TrimLeft(cy.Path, "^")
			up := len(cy.Path) - len(rel)
			if !strings.HasPrefix(rel, ".") && !strings.HasPrefix(rel, "[") {
				c.fail(cat+".path", "path %q must be written from the row, starting with '.' or '[' (\".supplier_gstin\"), or '^.' for the object around the row's list", cy.Path)
				ok = false
				continue
			}
			// A record column may select many values (item lines): they are added up or joined.
			sel, err := c.syn.parse("$" + rel)
			if err != nil {
				c.fail(cat+".path", "%v", err)
				ok = false
				continue
			}
			switch {
			case cy.DateFormat != "" && want != "field":
				c.fail(cat+".date_format", "date_format is for fields only")
				ok = false
				continue
			case cy.DateFormat != "" && !slices.Contains(dateFormats, cy.DateFormat):
				c.fail(cat+".date_format", "date format %q must be one of: %s", cy.DateFormat, strings.Join(dateFormats, ", "))
				ok = false
				continue
			}
			out = append(out, column{name: cy.Name, rel: rel, up: up, many: c.syn.many(sel), required: cy.Required, dateFormat: cy.DateFormat})
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
