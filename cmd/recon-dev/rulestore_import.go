package main

import (
	"errors"
	"flag"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// rulestoreImport writes the YAML rule store from the rule-book workbook: every sheet
// whose row 5 starts with "rule_id" gives rules, and the "Documents" and "Document Fields"
// sheets give document types.
//
//	go run ./cmd/recon-dev rulestore-import [-xlsx file] [-out folder] [-date YYYY-MM-DD]
func rulestoreImport(args []string) error {
	fs := flag.NewFlagSet("rulestore-import", flag.ContinueOnError)
	xlsx := fs.String("xlsx", rsDefaultWorkbook, "rule-book workbook")
	out := fs.String("out", rsDefaultStore, "rule store folder")
	date := fs.String("date", time.Now().Format("2006-01-02"), "date written as last_modified_on and generated_on")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return errors.New("unexpected arguments; use -xlsx and -out")
	}
	f, err := excelize.OpenFile(*xlsx)
	if err != nil {
		return err
	}
	defer f.Close()

	var index []rsRuleIndexRow
	referenced := map[[2]string][]string{} // DOC.field → rule IDs using it
	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			return err
		}
		if len(rows) < 5 || len(rows[4]) == 0 || rows[4][0] != "rule_id" {
			continue
		}
		keys := rows[4]
		for _, row := range rows[5:] {
			if len(row) == 0 || row[0] == "" {
				continue
			}
			r := rowMap(keys, row)
			typ := "statutory"
			if cell(r, "rule_type") == "reconciliation_rule" {
				typ = "reconciliation"
			}
			rule := buildRule(r, typ, *date)
			for _, p := range append(slices.Clone(rule.Matching.Keys), rule.Matching.Comparisons...) {
				for _, e := range []string{p.Left, p.Right, p.Expression} {
					for _, ref := range fieldRefs(e) {
						referenced[ref] = append(referenced[ref], rule.Rule.ID)
					}
				}
			}
			rel := path.Join("rules", typ, rule.Rule.Domain, rule.Rule.ID+".yaml")
			if err := rsWrite(filepath.Join(*out, rel), rule, &rsRule{}); err != nil {
				return err
			}
			docs := []string{}
			for _, d := range rule.Inputs.Documents {
				docs = append(docs, d.DocCode)
			}
			index = append(index, rsRuleIndexRow{ID: rule.Rule.ID, Type: typ, Domain: rule.Rule.Domain, Name: rule.Rule.Name,
				Documents: docs, VerificationStatus: rule.Governance.VerificationStatus, File: rel})
		}
	}
	if err := rsWrite(filepath.Join(*out, "rules", "index.yaml"),
		rsRuleIndex{SchemaVersion: rsSchemaVersion, GeneratedOn: *date, Count: len(index), Rules: index}, &rsRuleIndex{}); err != nil {
		return err
	}

	docRows, err := sheetTable(f, "Documents", 4)
	if err != nil {
		return err
	}
	fieldRows, err := sheetTable(f, "Document Fields", 4)
	if err != nil {
		return err
	}
	byDoc := map[string][]rsField{}
	names := map[string]map[string]bool{}
	for _, fr := range fieldRows {
		raw := cell(fr, "Field name (canonical)")
		for _, nm := range cleanFieldNames(raw) {
			desc := cell(fr, "Description")
			if nm != raw {
				desc = strings.TrimSpace(desc + ` (catalogue name: "` + raw + `")`)
			}
			code := cell(fr, "Doc code")
			byDoc[code] = append(byDoc[code], rsField{
				Name: nm, Label: cell(fr, "Official label"), SourcePath: cell(fr, "Table / JSON path"),
				DataType: cell(fr, "Data type"), Required: yesNo(cell(fr, "Required")),
				Validation: cell(fr, "Format validation"), Description: desc, Origin: cell(fr, "Origin"),
			})
			if names[code] == nil {
				names[code] = map[string]bool{}
			}
			names[code][nm] = true
		}
	}
	// Fields rules use but the catalogue lacks become design fields, so every rule resolves.
	refs := make([][2]string, 0, len(referenced))
	for k := range referenced {
		refs = append(refs, k)
	}
	slices.SortFunc(refs, func(a, b [2]string) int { return strings.Compare(a[0]+"."+a[1], b[0]+"."+b[1]) })
	for _, ref := range refs {
		doc, fld := ref[0], ref[1]
		if names[doc] == nil || names[doc][fld] {
			continue
		}
		ids := slices.Compact(slices.Sorted(slices.Values(referenced[ref])))
		byDoc[doc] = append(byDoc[doc], rsField{Name: fld, Origin: "new (design)",
			Description: "Referenced by " + strings.Join(ids, ", ") + "; not in the researched catalogue – define with the first real file."})
		names[doc][fld] = true
	}

	var dindex []rsDocIndexRow
	for _, d := range docRows {
		code := cell(d, "Doc code")
		fields := byDoc[code]
		if fields == nil {
			fields = []rsField{}
		}
		doc := rsDocFile{SchemaVersion: rsSchemaVersion, Fields: fields, Document: rsDocument{
			Code: code, Name: cell(d, "Document name"), IssuerSystem: cell(d, "Issuer / system"),
			LegalBasis: cell(d, "Legal basis"), NewFormNo: cell(d, "New form no. (2025 Act)"),
			Frequency: cell(d, "Frequency"), FormatsAvailable: cell(d, "Formats available"),
			HowObtained: cell(d, "How obtained"), DueDate: cell(d, "Due date"), TablesUsed: cell(d, "Tables / parts used"),
			UsedByRules: splitComma(cell(d, "Used by rules")), VerificationStatus: statusCode(cell(d, "Verification status")),
			Notes: cell(d, "Notes"), Sources: parseSources(cell(d, "Sources (title | URL | ref)")),
		}}
		if err := rsWrite(filepath.Join(*out, "documents", code+".yaml"), doc, &rsDocFile{}); err != nil {
			return err
		}
		dindex = append(dindex, rsDocIndexRow{Code: code, Name: doc.Document.Name, Fields: len(fields), File: "documents/" + code + ".yaml"})
	}
	if err := rsWrite(filepath.Join(*out, "documents", "index.yaml"),
		rsDocIndex{SchemaVersion: rsSchemaVersion, Count: len(dindex), Documents: dindex}, &rsDocIndex{}); err != nil {
		return err
	}
	fmt.Printf("%d rules and %d documents written to %s\n", len(index), len(dindex), *out)
	return nil
}

// sheetTable returns the rows below the header row (1-based) of sheet as maps; rows with
// an empty first cell are skipped.
func sheetTable(f *excelize.File, sheet string, header int) ([]map[string]string, error) {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	if len(rows) < header {
		return nil, fmt.Errorf("sheet %q has no header row %d", sheet, header)
	}
	var out []map[string]string
	for _, row := range rows[header:] {
		if len(row) > 0 && row[0] != "" {
			out = append(out, rowMap(rows[header-1], row))
		}
	}
	return out, nil
}

func rowMap(keys, row []string) map[string]string {
	m := map[string]string{}
	for i, k := range keys {
		if i < len(row) && k != "" {
			if _, dup := m[k]; !dup {
				m[k] = row[i]
			}
		}
	}
	return m
}

// cell returns a cleaned cell: outer space trimmed and trailing spaces removed from every
// line, so the text can be written as a | block.
func cell(r map[string]string, key string) string {
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(r[key], "\r\n", "\n")), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.Join(lines, "\n")
}

var statusCodes = map[string]string{
	"Verified – official source":                "verified_official",
	"Verified – secondary source only":          "verified_secondary",
	"Partially verified":                        "partially_verified",
	"Unverified – needs CA review":              "unverified",
	"Ambiguous / conflicting – needs CA review": "conflicting",
}

func statusCode(label string) string {
	if c, ok := statusCodes[label]; ok {
		return c
	}
	return "unverified"
}

var domainByPrefix = [][2]string{
	{"RC-GST-", "gst"}, {"RC-TDS-", "tds_tcs"}, {"RC-IT-", "income_tax"}, {"RC-PAY-", "payroll"},
	{"RC-BNK-", "bank"}, {"RC-ACC-", "accounting"},
	{"ITR-", "income_tax"}, {"IT-", "income_tax"}, {"ADV-", "income_tax"}, {"TDS-", "tds_tcs"}, {"TCS-", "tds_tcs"},
	{"GST-", "gst"}, {"PAY-", "payroll"}, {"BRS-", "bank"}, {"TB-", "accounting"}, {"LED-", "accounting"},
	{"SAL-", "accounting"}, {"PUR-", "accounting"}, {"FS-", "accounting"}, {"MSME-", "msme"},
	{"MCA-", "company_law"}, {"AUD-", "audit"},
}

func domainOf(id string) string {
	for _, p := range domainByPrefix {
		if strings.HasPrefix(id, p[0]) {
			return p[1]
		}
	}
	return "other"
}

var isoDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func onlyISODate(s string) string {
	if isoDate.MatchString(s) {
		return s
	}
	return ""
}

func buildRule(r map[string]string, typ, today string) rsRule {
	rec := typ == "reconciliation"
	id := cell(r, "rule_id")
	label := cell(r, "verification_status")
	var rule rsRule
	rule.SchemaVersion = rsSchemaVersion
	rule.Rule = rsRuleID{ID: id, Version: "1", Type: typ, Domain: domainOf(id),
		Category: cell(r, "category"), Name: cell(r, "name"), Purpose: cell(r, "purpose")}
	rule.Governance = rsGovernance{VerificationStatus: statusCode(label), VerificationLabel: label,
		LastModifiedBy: "import", LastModifiedOn: today,
		ChangeNote: "Imported from Recon_Tool_Rulebook_v2.xlsx (research date 13-10-2026)"}
	if rec {
		rule.Validity = rsValidity{ValidFrom: onlyISODate(cell(r, "effective_from")), ValidTo: onlyISODate(cell(r, "effective_to")),
			EffectiveText: cell(r, "effective_from")}
	} else {
		rule.Validity.EffectiveText = cell(r, "effective_date")
	}
	rule.Validity.PeriodBasis = cell(r, "period_basis")
	rule.Legal = rsLegal{Provisions: cell(r, "legal_provision"), Applicability: cell(r, "applicability"),
		ExemptionsExceptions: cell(r, "exemptions_exceptions"), ThresholdsRates: cell(r, "thresholds_rates"),
		Rounding: cell(r, "rounding"), DueDates: cell(r, "due_dates")}
	rule.Inputs = rsInputs{Documents: []rsDocUse{}, Fields: []string{},
		Prerequisites: cell(r, "prerequisites"), InputValidations: cell(r, "validations")}
	for _, d := range [][2]string{{"A", "doc_a"}, {"B", "doc_b"}, {"C", "doc_c"}} {
		if c := cell(r, d[1]); c != "" {
			rule.Inputs.Documents = append(rule.Inputs.Documents, rsDocUse{Role: d[0], DocCode: c, Required: d[0] != "C"})
		}
	}
	if !rec {
		rule.Inputs.Fields = splitComma(cell(r, "inputs"))
	}
	rule.Matching = rsMatching{Keys: []rsPair{}, Comparisons: []rsPair{}, KeyNormalisation: cell(r, "key_normalisation"),
		Aggregation: cell(r, "aggregation"), Tolerance: cell(r, "tolerance")}
	rule.Parameters = []rsParam{}
	rule.Outcomes.Categories = []rsCategory{}
	if rec {
		rule.Matching.Keys = parsePairs(cell(r, "match_keys"), " = ")
		rule.Matching.Comparisons = parsePairs(cell(r, "compare_fields"), " vs ")
		rule.Parameters = parseParams(cell(r, "params"))
		rule.Outcomes.Categories = parseCategories(cell(r, "mismatch_categories"), cell(r, "severity_by_category"))
	}
	rule.Calculation = rsCalculation{Formula: cell(r, "formula"), Steps: splitSteps(cell(r, "logic_steps"))}
	action := cell(r, "software_action")
	rule.Outcomes.DefaultSeverity = defaultSeverity(action)
	rule.Outcomes.ExpectedOutput = cell(r, "expected_output")
	rule.Outcomes.RecommendedAction = action
	rule.Outcomes.CommonDiscrepancies = cell(r, "common_discrepancies")
	example := "sample_calc"
	if rec {
		example = "sample_data_and_result"
	}
	rule.Tests = rsTests{WorkedExample: cell(r, example), Positive: cell(r, "test_positive"), Negative: cell(r, "test_negative")}
	rule.Implementation = rsImplementation{Pseudocode: cell(r, "pseudocode")}
	rule.Sources = parseSources(cell(r, "sources"))
	rule.Notes = cell(r, "notes")
	return rule
}

func splitComma(s string) []string {
	out := []string{}
	for _, x := range strings.Split(s, ",") {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}

// splitItems splits "a; b; c" only where the next item starts with start, so a ";" inside
// a description stays.
func splitItems(text string, start *regexp.Regexp) []string {
	if text == "" {
		return nil
	}
	sep := regexp.MustCompile(`;\s*`)
	var parts []string
	last := 0
	for _, m := range sep.FindAllStringIndex(text, -1) {
		if start.MatchString(text[m[1]:]) {
			parts = append(parts, text[last:m[0]])
			last = m[1]
		}
	}
	parts = append(parts, text[last:])
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(strings.TrimSpace(p))
		p = strings.TrimSpace(strings.TrimRight(p, ";"))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// splitSteps turns "1. x 2. y 3. z" into [x y z] when the numbering runs 1, 2, 3 ...; text
// before step 1 is kept as the first item. Nothing is lost.
func splitSteps(text string) []string {
	if text == "" {
		return []string{}
	}
	type span struct{ a, b int }
	var pos []span
	start := 0
	for n := 1; ; n++ {
		re := regexp.MustCompile(fmt.Sprintf(`%d[.)]\s+`, n))
		found := false
		for from := start; from <= len(text); {
			m := re.FindStringIndex(text[from:])
			if m == nil {
				break
			}
			a, b := from+m[0], from+m[1]
			if a == 0 || isSpace(text[a-1]) {
				pos = append(pos, span{a, b})
				start, found = b, true
				break
			}
			from = a + 1
		}
		if !found {
			break
		}
	}
	if len(pos) < 2 {
		return []string{text}
	}
	var out []string
	if pre := strings.TrimSpace(text[:pos[0].a]); pre != "" {
		out = append(out, pre)
	}
	for i, p := range pos {
		end := len(text)
		if i+1 < len(pos) {
			end = pos[i+1].a
		}
		out = append(out, strings.TrimSpace(text[p.b:end]))
	}
	return out
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\n' || c == '\t' || c == '\r' || c == '\f' || c == '\v'
}

var (
	paramStart    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*\s*=`)
	categoryStart = regexp.MustCompile(`^[A-Z0-9_]{2,}\s*(?::|;|$)`)
	severityStart = regexp.MustCompile(`^[A-Z0-9_]{2,}\s*=`)
	pairStart     = regexp.MustCompile(`^[A-Za-z_(]`)
	intText       = regexp.MustCompile(`^-?\d+$`)
	decimalText   = regexp.MustCompile(`^-?\d+\.\d+$`)
)

// typedParam reads a parameter value as written in the workbook and returns its text and
// type. Values stay text; the type says how to read them.
func typedParam(v string) (string, string) {
	v = strings.TrimSpace(v)
	switch strings.ToLower(v) {
	case "true", "yes":
		return "true", "boolean"
	case "false", "no":
		return "false", "boolean"
	}
	digits := strings.TrimPrefix(v, "-")
	if intText.MatchString(v) && !(len(v) > 1 && strings.HasPrefix(digits, "0")) {
		return v, "integer"
	}
	if decimalText.MatchString(v) {
		return v, "number"
	}
	return v, "string"
}

func paramUnit(key string) string {
	k := strings.ToLower(key)
	for _, su := range [][2]string{{"_inr", "INR"}, {"_days", "days"}, {"_months", "months"}, {"_years", "years"},
		{"_pct", "percent"}, {"pct_", "percent"}, {"_percent", "percent"}, {"_rate", "rate (fraction)"}} {
		if strings.HasSuffix(k, su[0]) || strings.HasPrefix(k, su[0]) {
			return su[1]
		}
	}
	return ""
}

func parseParams(text string) []rsParam {
	out := []rsParam{}
	for _, item := range splitItems(text, paramStart) {
		k, v, ok := strings.Cut(item, "=")
		if !ok {
			out = append(out, rsParam{Value: item, Type: "string", Editable: true, Note: "Unparsed parameter text – CA to structure"})
			continue
		}
		k = strings.TrimSpace(k)
		val, typ := typedParam(v)
		label := strings.ToLower(strings.ReplaceAll(k, "_", " "))
		if label != "" {
			label = strings.ToUpper(label[:1]) + label[1:]
		}
		out = append(out, rsParam{Key: k, Value: val, Type: typ, Unit: paramUnit(k), Label: label, Editable: true})
	}
	return out
}

var severities = []string{"Critical", "High", "Medium", "Low"}

func parseCategories(catText, sevText string) []rsCategory {
	sev := map[string]string{}
	var sevOrder []string
	for _, item := range splitItems(sevText, severityStart) {
		k, v, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		f := strings.Fields(v)
		val := ""
		if len(f) > 0 {
			val = strings.TrimRight(f[0], ".,;")
		}
		if !slices.Contains(severities, val) {
			val = ""
		}
		k = strings.TrimSpace(k)
		if _, seen := sev[k]; !seen {
			sevOrder = append(sevOrder, k)
		}
		sev[k] = val
	}
	cats := []rsCategory{}
	seen := map[string]bool{}
	for _, item := range splitItems(catText, categoryStart) {
		code, desc, _ := strings.Cut(item, ":")
		code = strings.TrimSpace(code)
		cats = append(cats, rsCategory{Code: code, Description: strings.TrimSpace(desc), Severity: sev[code]})
		seen[code] = true
	}
	for _, code := range sevOrder {
		if !seen[code] {
			cats = append(cats, rsCategory{Code: code, Severity: sev[code]})
		}
	}
	return cats
}

func parsePairs(text, sep string) []rsPair {
	out := []rsPair{}
	for _, item := range splitItems(text, pairStart) {
		if l, r, ok := strings.Cut(item, sep); ok {
			out = append(out, rsPair{Left: strings.TrimSpace(l), Right: strings.TrimSpace(r)})
		} else {
			out = append(out, rsPair{Expression: item})
		}
	}
	return out
}

var (
	urlPattern = regexp.MustCompile(`https?://\S+`)
	official   = []string{".gov.in", "nic.in", "icai.org", "rbi.org.in", "sansad.in", "protean-tinpan.com"}
)

func parseSources(text string) []rsSource {
	out := []rsSource{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var src rsSource
		if m := urlPattern.FindStringIndex(line); m != nil {
			src.URL = strings.TrimSpace(strings.TrimRight(line[m[0]:m[1]], "|"))
			src.Title = strings.TrimSpace(strings.TrimRight(strings.TrimSpace(line[:m[0]]), "|"))
			src.Ref = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(line[m[1]:]), "|"))
		} else {
			parts := strings.Split(line, " | ")
			for i := range parts {
				parts[i] = strings.TrimSpace(parts[i])
			}
			src.Title = parts[0]
			var rest []string
			for _, p := range parts[1:] {
				if p != "" {
					rest = append(rest, p)
				}
			}
			src.Ref = strings.Join(rest, " | ")
		}
		for _, o := range official {
			if src.URL != "" && strings.Contains(src.URL, o) {
				src.Official = true
			}
		}
		out = append(out, src)
	}
	return out
}

var leadingSeverity = regexp.MustCompile(`^\s*(Critical|High|Medium|Low)\b`)

func defaultSeverity(text string) string {
	if m := leadingSeverity.FindStringSubmatch(text); m != nil {
		return m[1]
	}
	return ""
}

// fieldRefPattern finds DOC.field references; fieldRefs then drops function calls
// (GL.abs(..)), wildcards (isd_*) and date masks (yyyy-mm).
var fieldRefPattern = regexp.MustCompile(`\b([A-Z][A-Z0-9_]{1,20})\.([a-z][a-z0-9_]*[a-z0-9])`)

func fieldRefs(expr string) [][2]string {
	var out [][2]string
	for _, m := range fieldRefPattern.FindAllStringSubmatchIndex(expr, -1) {
		if m[1] < len(expr) {
			c := expr[m[1]]
			if c == '(' || c == '*' || c == '-' || c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
				continue
			}
		}
		out = append(out, [2]string{expr[m[2]:m[3]], expr[m[4]:m[5]]})
	}
	return out
}

var nonSnake = regexp.MustCompile(`[^a-z0-9]+`)

// cleanFieldNames turns a catalogue field name into snake_case names: "a/b" gives a and b,
// "x_*" (a group marker) gives none.
func cleanFieldNames(name string) []string {
	if strings.Contains(name, "*") {
		return nil
	}
	var out []string
	for _, part := range strings.Split(name, "/") {
		part = strings.Trim(nonSnake.ReplaceAllString(strings.ToLower(strings.TrimSpace(part)), "_"), "_")
		if part != "" && part[0] >= 'a' && part[0] <= 'z' {
			out = append(out, part)
		}
	}
	return out
}

func yesNo(v string) string {
	switch u := strings.ToUpper(v); {
	case strings.HasPrefix(u, "Y"):
		return "yes"
	case strings.HasPrefix(u, "N"):
		return "no"
	}
	return ""
}
