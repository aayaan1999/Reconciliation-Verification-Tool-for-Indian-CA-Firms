package profile

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	jsonadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/json"
	xlsxadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/xlsx"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// A Mapping is what a user picked on the mapping screen to teach the tool a JSON layout it
// does not know (SPEC-004 part 8.4): the document type, the list that holds the rows, which
// field is each column, and where the period and the GSTIN are. YAML turns it into a format
// profile, so a taught layout is read by the same code, with the same checks, as every
// other. Paths are in the JSON path subset of the JSON adapter; column paths are written
// from the row (".inv_no") or, with "^", from the object around the row's list.
type Mapping struct {
	Name         string          `json:"name"`          // the user's name for the layout: "Tally sales export"
	DocumentType string          `json:"document_type"` // a document type whose rows have a record type
	Rows         string          `json:"rows"`          // the list of rows: "$.rows[*]"
	Where        []MappingWhere  `json:"where"`         // keep only rows where each path equals the value
	Unit         string          `json:"unit"`          // the amounts are in "rupees" or "paise"
	Columns      []MappingColumn `json:"columns"`
	Period       MappingPeriod   `json:"period"`
	TaxYear      string          `json:"tax_year"`   // "" works it out from the period
	GSTINPath    string          `json:"gstin_path"` // the client's own GSTIN in the file; "" when absent
	Identify     []MappingWhere  `json:"identify"`   // extra tests that tell this layout apart
	// FileType is the sample's file type, set by profile-create from the file: "" or "json",
	// or "xlsx" for an Excel workbook, whose paths start in a sheet ("$['Sales Register'][*]")
	// as the mapping screen lists them (SPEC-008 part 2).
	FileType string `json:"file_type"`
}

// A MappingColumn says which field of a row holds one column of the record type.
type MappingColumn struct {
	Name       string `json:"name"`        // a column of the record type: "invoice_no"
	Path       string `json:"path"`        // from the row: ".inv_no", "['Invoice No']", "^.ctin"
	DateFormat string `json:"date_format"` // "dd-mm-yyyy" when the file writes dates that way
}

// A MappingWhere is a path and the value it must hold.
type MappingWhere struct {
	Path   string `json:"path"`
	Equals string `json:"equals"` // "" for Identify: the path only has to be present
}

// MappingPeriod is where the period comes from: a field of the file, or a fixed value.
type MappingPeriod struct {
	Path   string `json:"path"`
	Format string `json:"format"` // "mmyyyy" ("082026"), "yyyymm" or "" ("2026-08")
	Const  string `json:"const"`  // "2026-08", "FY2026-27:Q2" or "FY" for the whole tax year
}

// A MappingProblem is one thing wrong with a mapping, named by its field.
type MappingProblem struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (p MappingProblem) String() string { return p.Field + ": " + p.Code + ": " + p.Message }

// OfficePrefix starts the ID of every profile taught in the office, so a report can say
// which formats were taught there rather than shipped with the tool.
const OfficePrefix = "office."

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slug is the layout's name as a folder and ID part: "Tally sales export" → "tally_sales_export".
func (m Mapping) Slug() string {
	s := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(m.Name), "_"), "_")
	if len(s) > 40 {
		s = strings.TrimRight(s[:40], "_")
	}
	if s != "" && s[0] >= '0' && s[0] <= '9' {
		s = "f_" + s
	}
	return s
}

// ID is the profile ID of the taught layout.
func (m Mapping) ID() string { return OfficePrefix + m.Slug() + "." + m.DocumentType }

// Dir is the profile's folder under the office profiles folder: <source>/<document_type>/v1.
func (m Mapping) Dir() string { return m.Slug() + "/" + m.DocumentType + "/v1" }

// RecordTypeFor returns the record type read from documents of docType, if exactly one.
func RecordTypeFor(v *vocab.Vocab, docType string) (vocab.RecordType, bool) {
	var found []vocab.RecordType
	for _, rt := range v.RecordTypes() {
		if rt.DocumentType == docType && !rt.Deprecated() {
			found = append(found, rt)
		}
	}
	if len(found) != 1 {
		return vocab.RecordType{}, false
	}
	return found[0], true
}

// YAML checks m and writes it as a format profile. Its problems name the mapping's fields
// in the user's terms; a profile that passes is then loaded with Load like any other.
func (m Mapping) YAML(v *vocab.Vocab) ([]byte, []MappingProblem) {
	var probs []MappingProblem
	bad := func(field, code, format string, args ...any) {
		probs = append(probs, MappingProblem{Field: field, Code: code, Message: fmt.Sprintf(format, args...)})
	}
	if m.Slug() == "" {
		bad("name", "BAD_NAME", "give the layout a name with letters or digits, such as \"Tally sales export\"")
	}
	rt, ok := RecordTypeFor(v, m.DocumentType)
	if !ok {
		bad("document_type", "BAD_DOCUMENT_TYPE", "%q is not a document type whose rows can be read; choose a register or a return", m.DocumentType)
	}
	if !strings.HasPrefix(m.Rows, "$") || !strings.HasSuffix(m.Rows, "[*]") {
		bad("rows", "BAD_ROWS", "choose the list that holds one row per invoice")
	}
	unit := m.Unit
	if unit == "" {
		unit = "rupees"
	}
	if unit != "rupees" && unit != "paise" {
		bad("unit", "BAD_UNIT", "amounts are in rupees or in paise")
	}
	for i, w := range m.Where {
		if w.Path == "" || w.Equals == "" {
			bad(fmt.Sprintf("where[%d]", i), "BAD_WHERE", "a row filter needs a field and the value it must hold")
		}
	}
	seen := map[string]bool{}
	for i, c := range m.Columns {
		at := fmt.Sprintf("columns[%d]", i)
		switch {
		case ok && !slices.Contains(rt.Fields, c.Name) && !slices.Contains(rt.Amounts, c.Name):
			bad(at, "UNKNOWN_COLUMN", "%q is not a column of %s", c.Name, rt.Name)
		case seen[c.Name]:
			bad(at, "DUPLICATE_COLUMN", "%q is chosen twice", c.Name)
		case c.Path == "":
			bad(at, "BAD_PATH", "choose the field that holds %s", c.Name)
		case c.DateFormat != "" && c.DateFormat != "dd-mm-yyyy":
			bad(at, "BAD_DATE_FORMAT", "dates are read as written (yyyy-mm-dd) or as dd-mm-yyyy")
		}
		seen[c.Name] = true
	}
	if ok {
		for _, k := range rt.MatchKey {
			if !seen[k] {
				bad("columns", "MISSING_KEY_COLUMN", "choose the field that holds %s: rows are matched on it", k)
			}
		}
	}
	switch p := m.Period; {
	case p.Path != "" && p.Const != "":
		bad("period", "BAD_PERIOD", "the period comes from a field or is fixed, not both")
	case p.Path == "" && p.Const == "":
		bad("period", "BAD_PERIOD", "choose the field that holds the period, or give the period")
	case p.Format != "" && p.Format != "mmyyyy" && p.Format != "yyyymm":
		bad("period.format", "BAD_PERIOD", "a period is written as mmyyyy, yyyymm or 2026-08")
	case p.Const != "" && p.Const != PeriodWholeYear:
		if _, err := period.Parse(p.Const); err != nil {
			bad("period.const", "BAD_PERIOD", "%v", err)
		}
	}
	if m.TaxYear != "" {
		if _, err := period.ParseTaxYear(m.TaxYear); err != nil {
			bad("tax_year", "BAD_TAX_YEAR", "%v", err)
		}
	} else if m.Period.Const == PeriodWholeYear {
		bad("tax_year", "BAD_TAX_YEAR", "a document for the whole year needs its tax year")
	}
	for i, w := range m.Identify {
		if !strings.HasPrefix(w.Path, "$") {
			bad(fmt.Sprintf("identify[%d]", i), "BAD_PATH", "choose a field of the file")
		}
	}
	excel := m.FileType == xlsxadapter.FileType
	var tables []xlsxadapter.Table
	switch {
	case m.FileType != "" && m.FileType != jsonadapter.FileType && !excel:
		bad("file", "UNSUPPORTED_FILE_TYPE", "only JSON and Excel (.xlsx) layouts can be taught")
	case excel && len(probs) == 0:
		var tprobs []MappingProblem
		tables, tprobs = m.sheets()
		probs = append(probs, tprobs...)
	}
	if len(probs) > 0 {
		return nil, probs
	}
	// In a workbook a single value (the period, the GSTIN) is read from the first row of its
	// sheet's table, as the mapping screen lists it with [*].
	one := func(p string) string {
		if excel {
			return strings.ReplaceAll(p, "[*]", "[0]")
		}
		return p
	}

	var b strings.Builder
	q := strconv.Quote
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	line("# Format profile (schema %s), taught in this office on the mapping screen (SPEC-004 part 8.4).", Schema)
	line("# Written by recon-engine profile-create from the user's choices and checked against the")
	line("# sample beside it. A changed layout is taught again under a new name; this file is not edited.")
	line("")
	line("schema: %s", q(Schema))
	line("profile_id: %s", q(m.ID()))
	line("profile_version: \"1\"")
	line("source_system: %s", q("Taught in this office: "+strings.TrimSpace(m.Name)))
	line("document_type: %s", q(m.DocumentType))
	if excel {
		line("file_type: \"xlsx\"")
		line("")
		// Each sheet is found by the labels read from it, on a sheet of any name; total rows
		// under the data are left out.
		line("sheets:")
		for _, t := range tables {
			quoted := make([]string, len(t.Header))
			for i, l := range t.Header {
				quoted[i] = q(l)
			}
			line("  - name: %s", q(t.Sheet))
			line("    header: [%s]", strings.Join(quoted, ", "))
			line("    any_name: true")
			line("    total_rows: [\"Total\", \"Grand Total\"]")
		}
	} else {
		line("file_type: \"json\"")
	}
	line("")
	// The layout is recognised by the fields read from its first row, plus any test the
	// user added: a file of another layout lacks some of them.
	first := strings.ReplaceAll(m.Rows, "[*]", "[0]")
	line("detect:")
	line("  - json_exists: { path: %s }", q(first))
	for _, c := range m.Columns {
		if !strings.HasPrefix(c.Path, "^") && !strings.Contains(c.Path, "[*]") {
			line("  - json_exists: { path: %s }", q(first+c.Path))
		}
	}
	for _, w := range m.Identify {
		if w.Equals == "" {
			line("  - json_exists: { path: %s }", q(one(w.Path)))
		} else {
			line("  - json_equals: { path: %s, value: %s }", q(one(w.Path)), q(w.Equals))
		}
	}
	line("")
	line("context:")
	switch {
	case m.Period.Path != "" && m.Period.Format != "":
		line("  period: { json_path: %s, format: %s }", q(one(m.Period.Path)), q(m.Period.Format))
	case m.Period.Path != "":
		line("  period: { json_path: %s }", q(one(m.Period.Path)))
	default:
		line("  period: { const: %s }", q(m.Period.Const))
	}
	if m.TaxYear != "" {
		line("  tax_year: { const: %s }", q(m.TaxYear))
	} else {
		line("  tax_year: { from_period: true }")
	}
	if m.GSTINPath != "" {
		line("  pan: { json_path: %s, from_gstin: true }", q(one(m.GSTINPath)))
		line("  scope: { json_path: %s }", q(one(m.GSTINPath)))
	}
	line("")
	line("records:")
	line("  - record_type: %s", q(rt.Name))
	line("    rows: %s", q(m.Rows))
	if len(m.Where) > 0 {
		line("    where:")
		for _, w := range m.Where {
			line("      - { path: %s, equals: %s }", q(w.Path), q(w.Equals))
		}
	}
	line("    unit: %s", q(unit))
	var fields, amounts []string
	for _, c := range m.Columns {
		s := fmt.Sprintf("{ name: %s, path: %s", q(c.Name), q(c.Path))
		if slices.Contains(rt.MatchKey, c.Name) {
			s += ", required: true"
		}
		if c.DateFormat != "" {
			s += ", date_format: " + q(c.DateFormat)
		}
		s += " }"
		if slices.Contains(rt.Amounts, c.Name) {
			amounts = append(amounts, s)
		} else {
			fields = append(fields, s)
		}
	}
	line("    fields:")
	for _, s := range fields {
		line("      - %s", s)
	}
	if len(amounts) > 0 {
		line("    amounts:")
		for _, s := range amounts {
			line("      - %s", s)
		}
	}
	return []byte(b.String()), nil
}

// sheets lists the tables a taught Excel layout reads: the sheet of its rows first, then
// each other sheet a chosen path starts in, each with the header labels read from it.
func (m Mapping) sheets() ([]xlsxadapter.Table, []MappingProblem) {
	var probs []MappingProblem
	var out []xlsxadapter.Table
	// add records that label is read from sheet.
	add := func(sheet, label string) {
		i := slices.IndexFunc(out, func(t xlsxadapter.Table) bool { return t.Sheet == sheet })
		if i < 0 {
			out = append(out, xlsxadapter.Table{Sheet: sheet})
			i = len(out) - 1
		}
		if label != "" && !slices.Contains(out[i].Header, label) {
			out[i].Header = append(out[i].Header, label)
		}
	}
	keys := func(path string) []string {
		sel, err := jsonadapter.ParsePath(path)
		if err != nil {
			return nil
		}
		return sel.Keys()
	}
	rows := keys(m.Rows)
	if len(rows) != 1 || m.Rows != xlsxadapter.SheetPath(rows[0])+"[*]" && m.Rows != "$."+rows[0]+"[*]" {
		return nil, []MappingProblem{{Field: "rows", Code: "BAD_ROWS", Message: "choose the rows of one sheet, such as $['Sales Register'][*]"}}
	}
	add(rows[0], "")
	fromRow := func(field, path string) {
		k := keys("$" + strings.TrimLeft(path, "^"))
		if len(k) != 1 {
			probs = append(probs, MappingProblem{Field: field, Code: "BAD_PATH", Message: "choose a column of the sheet's rows"})
			return
		}
		add(rows[0], k[0])
	}
	for i, c := range m.Columns {
		fromRow(fmt.Sprintf("columns[%d]", i), c.Path)
	}
	for i, w := range m.Where {
		fromRow(fmt.Sprintf("where[%d]", i), w.Path)
	}
	fromFile := func(field, path string) {
		if path == "" {
			return
		}
		k := keys(path)
		if len(k) != 2 {
			probs = append(probs, MappingProblem{Field: field, Code: "BAD_PATH", Message: "choose a column of a sheet, such as $['Details'][*]['Return Period']"})
			return
		}
		add(k[0], k[1])
	}
	fromFile("period", m.Period.Path)
	fromFile("gstin_path", m.GSTINPath)
	for i, w := range m.Identify {
		fromFile(fmt.Sprintf("identify[%d]", i), w.Path)
	}
	for _, t := range out {
		if len(t.Header) == 0 {
			probs = append(probs, MappingProblem{Field: "columns", Code: "BAD_PATH", Message: fmt.Sprintf("choose at least one column of sheet %q", t.Sheet)})
		}
	}
	return out, probs
}
