package findings

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/comparison"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/modules/newheads"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// A Report is the result of one run as the engine prints it: the validation report of
// every file read, then the findings grouped by the document to review, then what could
// not be checked. It holds no run ID, database ID or time, so the same input always gives
// the same bytes (NF-01, NF-03). Every sentence comes from the fixed wording below; a
// rule supplies only its title and input labels, which the rule loader has already
// checked for banned phrases (§B4.6).
type Report struct {
	TaxYear         string
	IncludesTesting bool                      // the run evaluated rules the CA has not approved yet
	Files           []ingest.ValidationReport // in the order the files were read
	Evaluations     []domain.Evaluation       // sorted by rule ID and subject
	Rules           map[string]*rules.Rule    // every evaluated rule, by ID
	Vocab           *vocab.Vocab              // names the document each missing figure comes from
	Comparison      *comparison.Comparison    // the type of check the CA picked; nil for all rules
	Present         []string                  // document types in the current view
	Totals          []DocTotal                // row totals per document: a totals type, or a run of every rule
	NotSelected     []NotRun                  // rules of the comparison the CA left out, sorted by rule ID
	EngagementID    int64                     // names each finding's stable ID (§B7.1), so a decision on it is kept
	// Worklist holds the year-on-year ledger items (SPEC-006 part 3) when HasWorklist: both
	// years' ledgers were there. They are shown, never counted as checks or findings.
	Worklist    []newheads.Item
	HasWorklist bool
}

// A NotRun is a rule of the comparison that the CA did not select for this run (SPEC-004
// part 9). It is listed so that a partial run never reads as a complete one, and it is not
// counted as a check.
type NotRun struct {
	RuleID string
	Title  string
}

// A DocTotal is the number of rows of one document and the sum of each of their amounts.
type DocTotal struct {
	DocumentType string
	Rows         int
	Amounts      map[string]money.Paise
}

// Totals adds up the rows of each of the given document types, sorted by document type.
// Other documents in the current view (uploaded for another comparison) are left out.
func Totals(v *vocab.Vocab, records []domain.Record, documentTypes []string) ([]DocTotal, error) {
	byDoc := map[string]*DocTotal{}
	for _, r := range records {
		rt, ok := v.RecordType(r.RecordType)
		if !ok || !slices.Contains(documentTypes, rt.DocumentType) {
			continue
		}
		t := byDoc[rt.DocumentType]
		if t == nil {
			t = &DocTotal{DocumentType: rt.DocumentType, Amounts: map[string]money.Paise{}}
			byDoc[rt.DocumentType] = t
		}
		t.Rows++
		for k, a := range r.Amounts {
			sum, err := money.Add(t.Amounts[k], a)
			if err != nil {
				return nil, fmt.Errorf("totals of %s, %s: %w", rt.DocumentType, k, err)
			}
			t.Amounts[k] = sum
		}
	}
	out := make([]DocTotal, 0, len(byDoc))
	for _, t := range byDoc {
		out = append(out, *t)
	}
	slices.SortFunc(out, func(a, b DocTotal) int { return cmp.Compare(a.DocumentType, b.DocumentType) })
	return out, nil
}

// TestingNotice is the first line of a report that includes rules not approved by the CA.
const TestingNotice = "NOTE: this run includes rules not yet approved by the CA (status testing)."

// Counts returns the number of evaluations that passed, failed and could not be checked.
func (r *Report) Counts() (pass, fail, cannotCheck int) {
	for _, e := range r.Evaluations {
		switch e.Status {
		case domain.StatusPass:
			pass++
		case domain.StatusFail:
			fail++
		case domain.StatusCannotCheck:
			cannotCheck++
		}
	}
	return pass, fail, cannotCheck
}

// A group is the findings of one review document (FD-06).
type group struct {
	documentType string
	heading      string
	findings     []domain.Evaluation
}

func (r *Report) groups() []group {
	byDoc := map[string]*group{}
	for _, e := range r.Evaluations {
		if e.Status != domain.StatusFail {
			continue
		}
		doc := r.Rules[e.RuleID].ReviewDocument
		g := byDoc[doc]
		if g == nil {
			g = &group{documentType: doc, heading: r.documentName(doc)}
			byDoc[doc] = g
		}
		g.findings = append(g.findings, e)
	}
	out := make([]group, 0, len(byDoc))
	for _, g := range byDoc {
		out = append(out, *g)
	}
	slices.SortFunc(out, func(a, b group) int { return cmp.Compare(a.documentType, b.documentType) })
	return out
}

// documentName is the vocabulary's description of a document type, or the type itself.
func (r *Report) documentName(docType string) string {
	if dt, ok := r.Vocab.DocumentType(docType); ok && dt.Description != "" {
		return dt.Description
	}
	return docType
}

// figureSource returns the document a vocabulary fact is read from, and its name.
// A missing name may be a fact or a record type (rows).
func (r *Report) figureSource(fact string) (string, string) {
	if rt, _, ok := missingAmount(fact); ok { // an amount the rows of a record type lack
		fact = rt
	}
	if rt, _, _, ok := missingRow(fact); ok { // the row of a record type that has a key
		fact = rt
	}
	if f, ok := r.Vocab.Lookup(fact); ok {
		return f.DocumentType, r.documentName(f.DocumentType)
	}
	if rt, ok := r.Vocab.RecordType(fact); ok && rt.DocumentType != "" {
		return rt.DocumentType, r.documentName(rt.DocumentType)
	}
	return "", "an unknown document"
}

// missingAmount splits a missing-amount entry ("amount:sales_b2b_invoice.invoice_value")
// into its record type and amount.
func missingAmount(name string) (string, string, bool) {
	rest, ok := strings.CutPrefix(name, rules.MissingAmountPrefix)
	if !ok {
		return "", "", false
	}
	i := strings.LastIndexByte(rest, '.')
	if i < 0 {
		return "", "", false
	}
	return rest[:i], rest[i+1:], true
}

// missingRow splits a missing-row entry ("row:gstr3b_table_row|3.1(a)", with "|repeated" when more
// than one row has the key) into its record type, its key and whether the key is repeated.
func missingRow(name string) (string, string, bool, bool) {
	rest, ok := strings.CutPrefix(name, rules.MissingRowPrefix)
	if !ok {
		return "", "", false, false
	}
	rest, repeated := strings.CutSuffix(rest, rules.MissingRowRepeated)
	rt, key, ok := strings.Cut(rest, "|")
	return rt, key, repeated, ok
}

// missingText is the Cannot check line for one missing fact or record type.
func (r *Report) missingText(rule *rules.Rule, name string) string {
	if keys, ok := strings.CutPrefix(name, rules.MissingTablePrefix); ok {
		return fmt.Sprintf("Cannot check: the rule's table has no row for %s", strings.ReplaceAll(keys, "|", ", "))
	}
	if _, key, repeated, ok := missingRow(name); ok {
		_, doc := r.figureSource(name)
		if repeated {
			return fmt.Sprintf("Cannot check: more than one row for %s in %s (%s), so it is not clear which to compare", key, doc, inputLabel(rule, name))
		}
		return fmt.Sprintf("Cannot check: no row for %s in %s (%s)", key, doc, inputLabel(rule, name))
	}
	if _, amount, ok := missingAmount(name); ok {
		_, doc := r.figureSource(name)
		return fmt.Sprintf("Cannot check: %s is not in %s (%s), so it is not compared", strings.ReplaceAll(amount, "_", " "), doc, inputLabel(rule, name))
	}
	_, doc := r.figureSource(name)
	if _, isRows := r.Vocab.RecordType(name); isRows {
		return fmt.Sprintf("Cannot check: rows not found in %s: %s (%s)", doc, inputLabel(rule, name), name)
	}
	return fmt.Sprintf("Cannot check: figure not found in %s: %s (%s)", doc, inputLabel(rule, name), name)
}

// CodeText says in words what a row-matching result is (the code of an evaluation).
func CodeText(rule *rules.Rule, code string) string {
	a, b := "the first document", "the second document"
	if len(rule.Inputs) == 2 {
		a, b = rule.Inputs[0].Label, rule.Inputs[1].Label
	}
	switch code {
	case "MATCHED":
		return "Matched"
	case "ONLY_IN_A":
		return "In " + a + " only"
	case "ONLY_IN_B":
		return "In " + b + " only"
	case "DUPLICATE_IN_A":
		return "More than once in " + a
	case "DUPLICATE_IN_B":
		return "More than once in " + b
	case "AMOUNT_DIFF":
		return "Amounts differ"
	case "FIELD_DIFF":
		return "Details differ"
	case "KEY_DIFF":
		return "Invoice number written differently"
	case "LINES_DIFFER":
		return "Lines of the invoice show different values"
	case "FIELD_REPORTED":
		return "Listed: a column holds a value the rule lists, shown as written"
	}
	return code
}

// inputLabel returns the rule's label for the input that reads fact.
func inputLabel(rule *rules.Rule, fact string) string {
	if rt, _, _, ok := missingRow(fact); ok {
		fact = rt
	}
	if rt, amount, ok := missingAmount(fact); ok {
		for _, in := range rule.Inputs {
			if in.RecordType == rt {
				return in.Label + ", " + strings.ReplaceAll(amount, "_", " ")
			}
		}
		return strings.ReplaceAll(amount, "_", " ")
	}
	for _, in := range rule.Inputs {
		if in.Fact == fact || (in.RecordType == fact && fact != "") {
			return in.Label
		}
	}
	return ""
}

// Text renders the report for the terminal.
func (r *Report) Text() (string, error) {
	var b strings.Builder
	if r.IncludesTesting {
		b.WriteString(TestingNotice + "\n")
	}
	if c := r.Comparison; c != nil {
		fmt.Fprintf(&b, "Comparison: %s\n", c.Name)
		var docs []string
		for _, d := range c.Documents {
			state := "read"
			if !slices.Contains(r.Present, d) {
				state = "not uploaded yet"
			}
			docs = append(docs, fmt.Sprintf("%s (%s)", r.documentName(d), state))
		}
		fmt.Fprintf(&b, "Documents: %s\n", strings.Join(docs, "; "))
	}
	pass, fail, cc := r.Counts()
	fmt.Fprintf(&b, "Tax year %s: %s read, %s: %d pass, %d fail, %d cannot check\n",
		r.TaxYear, r.filesText(), plural(len(r.Evaluations), "check"), pass, fail, cc)
	if n := len(r.NotSelected); n > 0 {
		fmt.Fprintf(&b, "%s of this comparison not run: not selected (listed at the end)\n", plural(n, "rule"))
	}

	b.WriteString("\n== Files read ==\n")
	for _, f := range r.Files {
		writeFile(&b, f)
	}

	b.WriteString("\n== Findings ==\n")
	gs := r.groups()
	if len(gs) == 0 {
		b.WriteString("No findings.\n")
	}
	for _, g := range gs {
		fmt.Fprintf(&b, "\n%s (%d)\n", g.heading, len(g.findings))
		for _, e := range g.findings {
			if err := r.writeFinding(&b, e); err != nil {
				return "", err
			}
		}
	}

	if len(r.Totals) > 0 {
		if err := r.writeTotals(&b); err != nil {
			return "", err
		}
	}
	if r.HasWorklist {
		if err := r.writeWorklist(&b); err != nil {
			return "", err
		}
	}

	b.WriteString("\n== Cannot check ==\n")
	none := true
	for _, e := range r.Evaluations {
		if e.Status != domain.StatusCannotCheck {
			continue
		}
		none = false
		rule := r.Rules[e.RuleID]
		fmt.Fprintf(&b, "\n[%s]  %s  (%s)\n", e.RuleID, rule.Title, subjectText(e.Subject))
		for _, m := range e.MissingFacts {
			fmt.Fprintf(&b, "  %s\n", r.missingText(rule, m))
		}
	}
	if none {
		b.WriteString("Nothing.\n")
	}
	if len(r.NotSelected) > 0 {
		b.WriteString("\n== Not run: not selected ==\n")
		for _, n := range r.NotSelected {
			fmt.Fprintf(&b, "[%s]  %s\n", n.RuleID, n.Title)
		}
	}
	return b.String(), nil
}

// totalColumns are the amounts shown side by side in a totals table, in this order. The
// invoice value is left out: registers repeat it on every line of an invoice, so a sum of
// lines would overstate it.
var totalColumns = []string{"taxable_value", "igst", "cgst", "sgst", "cess"}

// writeTotals writes the documents' row totals side by side. They are shown, not compared:
// a totals-only comparison (sales vs purchases) has no rule and gives no finding.
func (r *Report) writeTotals(b *strings.Builder) error {
	b.WriteString("\n== Totals (shown side by side, not compared) ==\n")
	b.WriteString("The rows each format reads (for GST files, B2B invoices), added up. The invoice value is left out because books repeat it on every line of an invoice.\n")
	header := []string{""}
	for _, t := range r.Totals {
		header = append(header, r.documentName(t.DocumentType))
	}
	rows := [][]string{header}
	count := []string{"rows (lines)"}
	for _, t := range r.Totals {
		count = append(count, fmt.Sprint(t.Rows))
	}
	rows = append(rows, count)
	for _, col := range totalColumns {
		row := []string{strings.ReplaceAll(col, "_", " ")}
		for _, t := range r.Totals {
			s, err := money.FormatIndian(t.Amounts[col], true)
			if err != nil {
				return err
			}
			row = append(row, s)
		}
		rows = append(rows, row)
	}
	widths := make([]int, len(header))
	for _, row := range rows {
		for i, c := range row {
			widths[i] = max(widths[i], utf8.RuneCountInString(c))
		}
	}
	for _, row := range rows {
		b.WriteString(" ")
		for i, c := range row {
			pad := strings.Repeat(" ", widths[i]-utf8.RuneCountInString(c))
			if i == 0 {
				fmt.Fprintf(b, " %s%s", c, pad)
			} else {
				fmt.Fprintf(b, "  %s%s", pad, c)
			}
		}
		b.WriteString("\n")
	}
	return nil
}

func writeFile(b *strings.Builder, f ingest.ValidationReport) {
	fmt.Fprintf(b, "\n%s\n", f.File)
	fmt.Fprintf(b, "  Recognised as : %s (profile %s v%s)\n", f.SourceSystem, f.ProfileID, f.ProfileVersion)
	ty := ""
	if !f.TaxYear.IsZero() {
		ty = f.TaxYear.String()
	}
	fmt.Fprintf(b, "  Tax year      : %s\n", orDash(ty))
	fmt.Fprintf(b, "  PAN ending    : %s\n", orDash(f.PANLast4))
	fmt.Fprintf(b, "  Figures read  : %d\n", f.FactCount)
	if f.RecordCount > 0 {
		fmt.Fprintf(b, "  Rows read     : %d\n", f.RecordCount)
	}
	fmt.Fprintf(b, "  Not read      : %d\n", len(f.Unread))
	for _, u := range f.Unread {
		fmt.Fprintf(b, "    %s (%s)\n", u.Locator.Path, u.Reason)
	}
	for _, list := range []struct {
		title  string
		issues []ingest.Issue
	}{{"Errors", f.Errors}, {"Warnings", f.Warnings}} {
		fmt.Fprintf(b, "  %-13s : %d\n", list.title, len(list.issues))
		for _, is := range list.issues {
			fmt.Fprintf(b, "    %s at %s: %s\n", is.Code, orDash(is.Locator.Path), is.Msg)
		}
	}
	if f.HasErrors() {
		b.WriteString("  The figures of this file are not used, because it has errors.\n")
	}
}

// filesText counts the files read and, when a file holds several documents (a bundle,
// SPEC-005), the documents too: "1 file (4 documents)".
func (r *Report) filesText() string {
	names := map[string]bool{}
	for _, f := range r.Files {
		names[f.File] = true
	}
	if len(names) == len(r.Files) {
		return plural(len(r.Files), "file")
	}
	return fmt.Sprintf("%s (%s)", plural(len(names), "file"), plural(len(r.Files), "document"))
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func subjectText(s domain.Subject) string {
	t := s.Period
	if s.Scope != "" {
		t += ", " + s.Scope
	}
	if s.MatchKey != "" {
		t += ", " + s.MatchKey
	}
	return t
}

// writeFinding writes one finding in the §B4.6 layout: every value with its label, the
// difference, every source and the rule basis.
func (r *Report) writeFinding(b *strings.Builder, e domain.Evaluation) error {
	rule := r.Rules[e.RuleID]
	if e.Code != "" {
		fmt.Fprintf(b, "\n[%s]  %s  (%s): %s\n", e.RuleID, rule.Title, subjectText(e.Subject), CodeText(rule, e.Code))
		if err := writeRowTable(b, rule, e); err != nil {
			return err
		}
		writeBasis(b, e)
		return nil
	}
	fmt.Fprintf(b, "\n[%s]  %s  (%s)\n", e.RuleID, rule.Title, subjectText(e.Subject))
	paise := e.Difference%money.PaisePerRupee != 0
	for _, v := range e.Values {
		if v.Kind == domain.KindMoney && v.Money%money.PaisePerRupee != 0 {
			paise = true
		}
	}
	labels := []string{"Difference"}
	texts := []string{}
	for _, v := range e.Values {
		labels = append(labels, v.Label)
		t, err := valueText(v.Value, paise)
		if err != nil {
			return fmt.Errorf("rule %s: %w", e.RuleID, err)
		}
		texts = append(texts, t)
	}
	width, valWidth := 0, 0
	for _, l := range labels {
		width = max(width, utf8.RuneCountInString(l))
	}
	diff := ""
	if len(e.Values) > 0 && e.Values[0].Kind == domain.KindMoney && ShowsDifference(rule) {
		var err error
		if diff, err = money.FormatIndian(e.Difference, paise); err != nil {
			return err
		}
	}
	for _, t := range append(slices.Clone(texts), diff) {
		valWidth = max(valWidth, utf8.RuneCountInString(t))
	}
	line := func(label, value string) {
		fmt.Fprintf(b, "  %s%s : %s%s\n", label, strings.Repeat(" ", width-utf8.RuneCountInString(label)),
			strings.Repeat(" ", valWidth-utf8.RuneCountInString(value)), value)
	}
	for i, v := range e.Values {
		line(v.Label, texts[i])
	}
	if diff != "" {
		line("Difference", diff)
	}
	writeSources(b, e)
	writeBasis(b, e)
	return nil
}

// writeSources writes each distinct source once, in order.
func writeSources(b *strings.Builder, e domain.Evaluation) {
	var sources []string
	for _, v := range e.Values {
		// A value the check computed (a sum, a percentage, a law table's value) has no source.
		if d := v.Locator.Display(); d != "" && !slices.Contains(sources, d) {
			sources = append(sources, d)
		}
	}
	fmt.Fprintf(b, "  Source: %s\n", strings.Join(sources, " | "))
}

// writeRowTable writes a row-matching result: one line per column with the value in each
// document and, for money, the difference; then the sources.
func writeRowTable(b *strings.Builder, rule *rules.Rule, e domain.Evaluation) error {
	type cell struct{ a, b string }
	var order []string
	cells := map[string]*cell{}
	diffs := map[string]money.Paise{}
	moneyCol := map[string]bool{}
	paise := false
	for _, v := range e.Values {
		if v.Kind == domain.KindMoney && v.Money%money.PaisePerRupee != 0 {
			paise = true
		}
	}
	for _, v := range e.Values {
		side, col, _ := strings.Cut(v.Input, ".")
		label := col
		if _, after, ok := strings.Cut(v.Label, ", "); ok {
			label = after
		}
		if strings.Count(col, ".") > 0 { // a duplicate or line: its own row
			label = v.Label
		}
		if cells[label] == nil {
			cells[label] = &cell{}
			order = append(order, label)
		}
		t, err := valueText(v.Value, paise)
		if err != nil {
			return fmt.Errorf("rule %s: %w", e.RuleID, err)
		}
		if side == "a" {
			cells[label].a = t
		} else {
			cells[label].b = t
		}
		if v.Kind == domain.KindMoney {
			moneyCol[label] = true
			if side == "a" {
				diffs[label] += v.Money
			} else {
				diffs[label] -= v.Money
			}
		}
	}
	ha, hb := "first", "second"
	if len(rule.Inputs) == 2 {
		ha, hb = rule.Inputs[0].Label, rule.Inputs[1].Label
	}
	rows := [][4]string{{"", ha, hb, "Difference"}}
	for _, l := range order {
		c := cells[l]
		d := ""
		if moneyCol[l] && c.a != "" && c.b != "" {
			v := diffs[l]
			if v < 0 {
				v = -v
			}
			var err error
			if d, err = money.FormatIndian(v, paise); err != nil {
				return err
			}
		}
		rows = append(rows, [4]string{l, orDash(c.a), orDash(c.b), d})
	}
	var w [4]int
	for _, row := range rows {
		for i, c := range row {
			w[i] = max(w[i], utf8.RuneCountInString(c))
		}
	}
	for _, row := range rows {
		fmt.Fprintf(b, "  %s%s  %s%s  %s%s  %s%s\n",
			row[0], strings.Repeat(" ", w[0]-utf8.RuneCountInString(row[0])),
			strings.Repeat(" ", w[1]-utf8.RuneCountInString(row[1])), row[1],
			strings.Repeat(" ", w[2]-utf8.RuneCountInString(row[2])), row[2],
			strings.Repeat(" ", w[3]-utf8.RuneCountInString(row[3])), row[3])
	}
	writeSources(b, e)
	return nil
}

// SectionText writes a legal reference: "Sec 32" for a section number, and an article, a
// rule or a schedule as it is ("Article 12", "Rule 128").
func SectionText(section string) string {
	if section != "" && section[0] >= '0' && section[0] <= '9' {
		return "Sec " + section
	}
	return section
}

func writeBasis(b *strings.Builder, e domain.Evaluation) {
	var basis []string
	for _, l := range e.LawRefs {
		s := l.Act + " – " + SectionText(l.Section)
		if l.Reference != "" {
			s += " (" + l.Reference + ")"
		}
		basis = append(basis, s)
	}
	fmt.Fprintf(b, "  Rule basis: %s\n", strings.Join(basis, "; "))
}

func valueText(v domain.Value, paise bool) (string, error) {
	switch {
	case v.Kind == domain.KindMoney:
		return money.FormatIndian(v.Money, paise)
	case v.Kind == domain.KindText && strings.TrimSpace(v.Text) == "":
		return BlankText, nil
	}
	return domain.FormatValue(v), nil
}

// BlankText shows a text value with nothing in it.
const BlankText = "(blank)"

// JSON renders the same content as canonical JSON: keys sorted, no insignificant white
// space, money in integer paise, no HTML escaping, one trailing newline.
func (r *Report) JSON() ([]byte, error) {
	pass, fail, cc := r.Counts()
	files := make([]any, 0, len(r.Files))
	for _, f := range r.Files {
		files = append(files, ValidationReportObject(f))
	}
	groups := make([]any, 0)
	for _, g := range r.groups() {
		list := make([]any, 0, len(g.findings))
		for _, e := range g.findings {
			obj, err := r.evaluationObject(e)
			if err != nil {
				return nil, err
			}
			list = append(list, obj)
		}
		groups = append(groups, map[string]any{"review_document": g.documentType, "heading": g.heading, "findings": list})
	}
	cannot := make([]any, 0)
	for _, e := range r.Evaluations {
		if e.Status != domain.StatusCannotCheck {
			continue
		}
		missing := make([]any, 0, len(e.MissingFacts))
		for _, m := range e.MissingFacts {
			docType, docName := r.figureSource(m)
			_, rows := r.Vocab.RecordType(m)
			missing = append(missing, map[string]any{"fact": m, "label": inputLabel(r.Rules[e.RuleID], m),
				"document_type": docType, "document": docName, "rows": rows})
		}
		cannot = append(cannot, map[string]any{"rule_id": e.RuleID, "rule_version": e.RuleVersion,
			"title": r.Rules[e.RuleID].Title, "category": r.Rules[e.RuleID].Category,
			"subject": subjectObject(e.Subject), "missing": missing})
	}
	// The checks that passed, so the screens can filter by status (SPEC-004 part 6).
	passed := make([]any, 0)
	for _, e := range r.Evaluations {
		if e.Status != domain.StatusPass {
			continue
		}
		rule := r.Rules[e.RuleID]
		passed = append(passed, map[string]any{"rule_id": e.RuleID, "rule_version": e.RuleVersion,
			"title": rule.Title, "category": rule.Category, "review_document": rule.ReviewDocument,
			"code": e.Code, "subject": subjectObject(e.Subject)})
	}
	obj := map[string]any{
		"tax_year":         r.TaxYear,
		"includes_testing": r.IncludesTesting,
		"counts":           map[string]any{"checks": len(r.Evaluations), "pass": pass, "fail": fail, "cannot_check": cc},
		"files":            files,
		"findings":         groups,
		"cannot_check":     cannot,
		"passed":           passed,
		"comparison":       r.comparisonObject(),
		"totals":           r.totalsObject(),
		"not_selected":     r.notSelectedObject(),
	}
	if r.HasWorklist {
		list, err := worklistList(r.Worklist)
		if err != nil {
			return nil, err
		}
		obj["worklist"] = list
	}
	return marshal(obj)
}

func (r *Report) evaluationObject(e domain.Evaluation) (map[string]any, error) {
	// Database IDs differ between databases and runs; the report shows files and places only.
	vs := slices.Clone(e.Values)
	for i := range vs {
		vs[i].Locator.DocumentVersionID = 0
	}
	values, err := domain.CanonicalValues(vs)
	if err != nil {
		return nil, err
	}
	laws := make([]any, 0, len(e.LawRefs))
	for _, l := range e.LawRefs {
		laws = append(laws, map[string]any{"act": l.Act, "section": l.Section, "reference": l.Reference})
	}
	// The finding's identity and the fingerprint of its values: a decision is kept against
	// the first and stays only while the second is unchanged (SPEC-006 part 3; FD-04).
	fp, err := domain.ValueFingerprint(e.Values)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"finding_id":        domain.StableID(e.RuleID, r.EngagementID, e.Subject),
		"value_fingerprint": fp,
		"rule_id":           e.RuleID,
		"rule_version":      e.RuleVersion,
		"title":             r.Rules[e.RuleID].Title,
		"category":          r.Rules[e.RuleID].Category,
		"code":              e.Code,
		"code_text":         codeTextOrEmpty(r.Rules[e.RuleID], e.Code),
		"subject":           subjectObject(e.Subject),
		"values":            json.RawMessage(values),
		"difference_paise":  int64(e.Difference),
		"show_difference":   ShowsDifference(r.Rules[e.RuleID]),
		"law_refs":          laws,
	}, nil
}

// noDifference are the check types whose findings state values found together, or a value
// and a law table's value: a difference there would mean nothing (SPEC-006).
var noDifference = []string{"present_together", "table_lookup"}

// ShowsDifference reports whether a finding of rule shows a difference line (when its
// values are money).
func ShowsDifference(rule *rules.Rule) bool {
	return rule != nil && !slices.Contains(noDifference, rule.Check.Type)
}

func subjectObject(s domain.Subject) map[string]any {
	return map[string]any{"period": s.Period, "scope": s.Scope, "match_key": s.MatchKey}
}

// ValidationReportObject is the JSON form of a validation report (IN-14), as printed with
// --json and stored in document_versions.validation_report_json.
func ValidationReportObject(f ingest.ValidationReport) map[string]any {
	ty := ""
	if !f.TaxYear.IsZero() {
		ty = f.TaxYear.String()
	}
	unread := make([]any, 0, len(f.Unread))
	for _, u := range f.Unread {
		unread = append(unread, map[string]any{"path": u.Locator.Path, "kind": string(u.Kind), "row": u.Row, "reason": u.Reason})
	}
	issues := func(list []ingest.Issue) []any {
		out := make([]any, 0, len(list))
		for _, is := range list {
			out = append(out, map[string]any{"code": string(is.Code), "message": is.Msg, "path": is.Locator.Path})
		}
		return out
	}
	return map[string]any{
		"file": f.File, "file_type": f.FileType, "document_type": f.DocumentType, "profile_id": f.ProfileID, "profile_version": f.ProfileVersion,
		"source_system": f.SourceSystem, "tax_year": ty, "client_match": string(f.ClientMatch), "pan_last4": f.PANLast4,
		"fact_count": f.FactCount, "record_count": f.RecordCount, "unread": unread,
		"warnings": issues(f.Warnings), "errors": issues(f.Errors),
	}
}

// marshal writes canonical JSON with a trailing newline.
func marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// MarshalValidationReport encodes a validation report for storage.
func MarshalValidationReport(f ingest.ValidationReport) ([]byte, error) {
	b, err := marshal(ValidationReportObject(f))
	return bytes.TrimSuffix(b, []byte("\n")), err
}

func (r *Report) comparisonObject() any {
	c := r.Comparison
	if c == nil {
		return nil
	}
	docs := make([]any, 0, len(c.Documents))
	for _, d := range c.Documents {
		docs = append(docs, map[string]any{"document_type": d, "name": r.documentName(d), "present": slices.Contains(r.Present, d)})
	}
	return map[string]any{"id": c.ID, "name": c.Name, "description": c.Description, "totals": c.Totals, "documents": docs}
}

func (r *Report) notSelectedObject() []any {
	out := make([]any, 0, len(r.NotSelected))
	for _, n := range r.NotSelected {
		out = append(out, map[string]any{"rule_id": n.RuleID, "title": n.Title})
	}
	return out
}

func (r *Report) totalsObject() []any {
	out := make([]any, 0, len(r.Totals))
	for _, t := range r.Totals {
		amounts := map[string]any{}
		for k, v := range t.Amounts {
			amounts[k] = int64(v)
		}
		out = append(out, map[string]any{"document_type": t.DocumentType, "document": r.documentName(t.DocumentType), "rows": t.Rows, "amounts": amounts})
	}
	return out
}

func codeTextOrEmpty(rule *rules.Rule, code string) string {
	if code == "" {
		return ""
	}
	return CodeText(rule, code)
}
