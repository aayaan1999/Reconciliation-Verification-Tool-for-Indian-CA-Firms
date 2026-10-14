package profile

import (
	"cmp"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// A Source identifies the uploaded document being read. Its fields go into every locator
// and fact; ExpectedPAN is the engagement client's PAN, for the client-match line of the
// validation report (empty: not checked).
type Source struct {
	EngagementID      int64
	DocumentVersionID int64
	FileName          string
	ExpectedPAN       string
}

// Context is what a document says about itself: the slot keys used to combine files of
// one engagement (IN-11).
type Context struct {
	TaxYear period.TaxYear
	Period  string // canonical: "FY2025-26" | "FY2025-26:Q1" | "2025-04"
	PAN     string // as read; never put in a report, only its last four characters
	Scope   string // GSTIN, branch or company; "" when the profile reads none
	Part    string // part of a document split over several files; "" when whole
}

// A Result is what one document gave. If Report has errors, Facts and Records are empty:
// a file with any error is not used at all, so a half-read file can never produce
// findings.
type Result struct {
	Context Context
	Facts   []domain.Fact   // sorted by name, then locator path
	Records []domain.Record // sorted by record type, then match key, then locator path
	Report  ingest.ValidationReport
}

// Extract reads doc with profile p (IN-20, IN-21). It is deterministic: the same document
// always gives the same result, byte for byte.
func Extract(p *Profile, doc ingest.Doc, src Source) (*Result, error) {
	if doc.FileType() != p.FileType {
		return nil, fmt.Errorf("profile %s reads %s files, not %s", p.Key(), p.FileType, doc.FileType())
	}
	x := &extraction{p: p, doc: doc, src: src, used: map[string]bool{}}
	x.res.Report = ingest.ValidationReport{
		File: src.FileName, FileType: p.FileType, ProfileID: p.ID, ProfileVersion: p.Version,
		SourceSystem: p.SourceSystem, ClientMatch: ingest.ClientNotChecked,
	}
	for _, pr := range p.detect { // what identified the format counts as read
		if err := x.markUsed(pr.sel); err != nil {
			return nil, err
		}
	}
	if err := x.readContext(); err != nil {
		return nil, err
	}
	if err := x.readFacts(); err != nil {
		return nil, err
	}
	if err := x.readRecords(); err != nil {
		return nil, err
	}
	x.listUnread()

	r := &x.res
	if r.Report.HasErrors() {
		r.Facts, r.Records = nil, nil
	}
	slices.SortStableFunc(r.Facts, func(a, b domain.Fact) int {
		return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.Locator.Path, b.Locator.Path))
	})
	slices.SortStableFunc(r.Records, func(a, b domain.Record) int {
		return cmp.Or(cmp.Compare(a.RecordType, b.RecordType), cmp.Compare(a.MatchKey, b.MatchKey), cmp.Compare(a.Locator.Path, b.Locator.Path))
	})
	r.Report.FactCount, r.Report.RecordCount = len(r.Facts), len(r.Records)
	r.Report.Sort()
	return r, nil
}

type extraction struct {
	p    *Profile
	doc  ingest.Doc
	src  Source
	used map[string]bool // concrete paths of every cell read
	res  Result
}

func (x *extraction) locator(c ingest.Cell) domain.Locator {
	l := c.Locator
	l.DocumentVersionID, l.FileName = x.src.DocumentVersionID, x.src.FileName
	return l
}

// at is the locator of a place the profile names but the file does not have.
func (x *extraction) at(sel ingest.Selector) domain.Locator {
	return domain.Locator{DocumentVersionID: x.src.DocumentVersionID, FileName: x.src.FileName,
		Kind: syntaxes[x.p.FileType].locatorKind, Path: sel.String()}
}

func (x *extraction) fail(code ingest.Code, loc domain.Locator, format string, args ...any) {
	x.res.Report.Errors = append(x.res.Report.Errors, ingest.Issue{Code: code, Msg: fmt.Sprintf(format, args...), Locator: loc})
}

func (x *extraction) warn(code ingest.Code, loc domain.Locator, format string, args ...any) {
	x.res.Report.Warnings = append(x.res.Report.Warnings, ingest.Issue{Code: code, Msg: fmt.Sprintf(format, args...), Locator: loc})
}

func (x *extraction) markUsed(sel ingest.Selector) error {
	cells, err := x.doc.Select(sel)
	if err != nil {
		return err
	}
	for _, c := range cells {
		x.markCell(c)
	}
	return nil
}

// markCell marks c as read. For a container (a row) only the container itself is marked:
// its columns that no mapping reads are still listed as unread.
func (x *extraction) markCell(c ingest.Cell) {
	x.used[c.Locator.Path] = true
}

// one selects the single cell sel addresses; ok is false when nothing is there. A
// selector that addresses one place can never return more than one cell.
func (x *extraction) one(sel ingest.Selector) (ingest.Cell, bool, error) {
	cells, err := x.doc.Select(sel)
	if err != nil || len(cells) == 0 {
		return ingest.Cell{}, false, err
	}
	x.markCell(cells[0])
	if cells[0].Kind == ingest.CellNull {
		return cells[0], false, nil
	}
	return cells[0], true, nil
}

// text reads a context source as text.
func (x *extraction) text(name string, s source) (string, domain.Locator, bool, error) {
	if !s.set {
		return "", domain.Locator{}, false, nil
	}
	if s.sel == nil {
		return s.constant, domain.Locator{}, true, nil
	}
	c, ok, err := x.one(s.sel)
	if err != nil || !ok {
		return "", domain.Locator{}, false, err
	}
	if !isScalar(c.Kind) {
		x.fail(ingest.ParseError, x.locator(c), "%s at %s is a%s, not a value", name, c.Locator.Path, article(string(c.Kind)))
		return "", domain.Locator{}, false, nil
	}
	return c.Text, x.locator(c), true, nil
}

var panPattern = regexp.MustCompile(`^[A-Z]{5}[0-9]{4}[A-Z]$`)

func (x *extraction) readContext() error {
	ctx := &x.res.Context
	rep := &x.res.Report
	m := x.p.context

	ty, loc, ok, err := x.text("the tax year", m.taxYear)
	if err != nil {
		return err
	}
	if !ok {
		x.fail(ingest.RequiredFieldMissing, loc, "the tax year is not in the file")
	} else if t, perr := period.ParseTaxYear(ty); perr != nil {
		x.fail(ingest.ParseError, loc, "the tax year %q cannot be read: %v", ty, perr)
	} else {
		ctx.TaxYear, rep.TaxYear = t, t
	}

	pt, loc, ok, err := x.text("the period", m.period)
	if err != nil {
		return err
	}
	switch {
	case !ok:
		x.fail(ingest.RequiredFieldMissing, loc, "the period is not in the file")
	case ctx.TaxYear.IsZero():
		// already reported
	case pt == PeriodWholeYear:
		ctx.Period = period.Year(ctx.TaxYear).String()
	default:
		pp, perr := period.Parse(pt)
		switch {
		case perr != nil:
			x.fail(ingest.ParseError, loc, "the period %q cannot be read: %v", pt, perr)
		case !period.Contains(period.Year(ctx.TaxYear), pp):
			x.fail(ingest.ParseError, loc, "the period %s is outside tax year %s", pp, ctx.TaxYear)
		default:
			ctx.Period = pp.String()
		}
	}

	pan, loc, ok, err := x.text("the PAN", m.pan)
	if err != nil {
		return err
	}
	if ok {
		ctx.PAN = pan
		if len(pan) >= 4 {
			rep.PANLast4 = pan[len(pan)-4:]
		}
		if !panPattern.MatchString(pan) {
			x.warn(ingest.ParseError, loc, "the PAN in the file is not in the usual form (five letters, four digits, one letter)")
		}
		if x.src.ExpectedPAN != "" {
			if strings.EqualFold(pan, x.src.ExpectedPAN) {
				rep.ClientMatch = ingest.ClientMatched
			} else {
				rep.ClientMatch = ingest.ClientMismatched
				x.fail(ingest.ConflictingValues, loc, "the PAN in the file (ending %s) is not this client's PAN", rep.PANLast4)
			}
		}
	}

	if ctx.Scope, _, _, err = x.text("the scope", m.scope); err != nil {
		return err
	}
	if ctx.Part, _, _, err = x.text("the document part", m.part); err != nil {
		return err
	}
	return nil
}

// periodFits reports whether a fact of periodicity p can be read from a document of
// period docPeriod.
func periodFits(p vocab.Periodicity, docPeriod string) bool {
	pp, err := period.Parse(docPeriod)
	if err != nil {
		return false
	}
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

func (x *extraction) readFacts() error {
	byName := map[string]domain.Fact{}
	conflicted := map[string]bool{}
	for _, fm := range x.p.facts {
		c, ok, err := x.one(fm.sel)
		if err != nil {
			return err
		}
		if !ok {
			if fm.required {
				x.fail(ingest.RequiredFieldMissing, x.at(fm.sel), "required figure %s is not in the file at %s", fm.fact.Name, fm.sel)
			}
			continue
		}
		loc := x.locator(c)
		val, err := convert(fm, c)
		if err != nil {
			x.fail(ingest.ParseError, loc, "%s at %s: %v", fm.fact.Name, c.Locator.Path, err)
			continue
		}
		if x.res.Context.Period == "" {
			continue // the period error is already reported
		}
		if !periodFits(fm.fact.Periodicity, x.res.Context.Period) {
			x.fail(ingest.ParseError, loc, "%s is a %s figure but the file covers %s", fm.fact.Name, fm.fact.Periodicity, x.res.Context.Period)
			continue
		}
		f := domain.Fact{
			EngagementID: x.src.EngagementID, Name: fm.fact.Name, Period: x.res.Context.Period,
			Scope: x.res.Context.Scope, Value: val, Locator: loc,
		}
		if prev, dup := byName[f.Name]; dup {
			if prev.Value != f.Value {
				conflicted[f.Name] = true
				x.fail(ingest.ConflictingValues, loc, "%s is %s at %s but %s at %s", f.Name,
					display(prev.Value), prev.Locator.Path, display(f.Value), loc.Path)
			}
			continue // the same figure twice with the same value: keep the first place
		}
		byName[f.Name] = f
		x.res.Facts = append(x.res.Facts, f)
	}
	// A conflict means neither copy can be trusted.
	x.res.Facts = slices.DeleteFunc(x.res.Facts, func(f domain.Fact) bool { return conflicted[f.Name] })
	return nil
}

// convert turns a cell into a value of the fact's kind. Money never goes through floating
// point: number cells use money.ParseJSONNumber, text cells money.Parse (which accepts
// "1,12,450.50" and "(2,750.00)"), and a profile rounding uses money.ParseRounded.
func convert(fm factMap, c ingest.Cell) (domain.Value, error) {
	want := func(kinds ...ingest.CellKind) error {
		if slices.Contains(kinds, c.Kind) {
			return nil
		}
		return fmt.Errorf("expected a %s value, found a%s", fm.fact.Kind, article(string(c.Kind)))
	}
	switch fm.fact.Kind {
	case domain.KindMoney:
		if err := want(ingest.CellNumber, ingest.CellText); err != nil {
			return domain.Value{}, err
		}
		var p money.Paise
		var err error
		switch {
		case fm.rounding != 0:
			p, err = money.ParseRounded(c.Text, fm.unit, fm.rounding)
		case c.Kind == ingest.CellNumber:
			p, err = money.ParseJSONNumber(json.Number(c.Text), fm.unit)
		default:
			p, err = money.Parse(c.Text, fm.unit)
		}
		if err == nil && fm.negate {
			p, err = money.Neg(p)
		}
		if err != nil {
			return domain.Value{}, err
		}
		return domain.MoneyValue(p), nil
	case domain.KindInt:
		if err := want(ingest.CellNumber, ingest.CellText); err != nil {
			return domain.Value{}, err
		}
		return domain.ParseValue(domain.KindInt, c.Text)
	case domain.KindText:
		if err := want(ingest.CellText, ingest.CellNumber); err != nil {
			return domain.Value{}, err
		}
		return domain.TextValue(c.Text), nil
	case domain.KindDate:
		if err := want(ingest.CellText); err != nil {
			return domain.Value{}, err
		}
		return domain.ParseValue(domain.KindDate, c.Text)
	case domain.KindBool:
		if err := want(ingest.CellBool); err != nil {
			return domain.Value{}, err
		}
		return domain.BoolValue(c.Text == "true"), nil
	case domain.KindRate:
		if err := want(ingest.CellNumber, ingest.CellText); err != nil {
			return domain.Value{}, err
		}
		return domain.ParseValue(domain.KindRate, c.Text)
	}
	return domain.Value{}, fmt.Errorf("unknown kind %s", fm.fact.Kind)
}

// display writes a value for a message: text in quotes, everything else as FormatValue.
func display(v domain.Value) string {
	if v.Kind == domain.KindText {
		return strconv.Quote(v.Text)
	}
	return domain.FormatValue(v)
}

func (x *extraction) readRecords() error {
	for _, rm := range x.p.records {
		rows, err := x.doc.Select(rm.rows)
		if err != nil {
			return err
		}
		for _, row := range rows {
			x.markCell(row)
			if row.Kind != ingest.CellObject {
				x.fail(ingest.ParseError, x.locator(row), "row %s of %s is a%s, not a row", row.Locator.Path, rm.rt.Name, article(string(row.Kind)))
				continue
			}
			if rec, ok, err := x.record(rm, row); err != nil {
				return err
			} else if ok {
				x.res.Records = append(x.res.Records, rec)
			}
		}
	}
	return nil
}

func (x *extraction) record(rm recordMap, row ingest.Cell) (domain.Record, bool, error) {
	rec := domain.Record{
		EngagementID: x.src.EngagementID, RecordType: rm.rt.Name, Period: x.res.Context.Period,
		Scope: x.res.Context.Scope, Fields: map[string]string{}, Amounts: map[string]money.Paise{}, Locator: x.locator(row),
	}
	ok := true
	read := func(col column) (ingest.Cell, bool, error) {
		sel, err := syntaxes[x.p.FileType].parse(row.Locator.Path + col.rel)
		if err != nil {
			return ingest.Cell{}, false, err
		}
		c, found, err := x.one(sel)
		if err != nil {
			return ingest.Cell{}, false, err
		}
		if !found && col.required {
			x.fail(ingest.RequiredFieldMissing, x.locator(row), "%s row %s has no %s", rm.rt.Name, row.Locator.Path, col.name)
			ok = false
		}
		return c, found, nil
	}
	for _, col := range rm.fields {
		c, found, err := read(col)
		if err != nil {
			return rec, false, err
		}
		if !found {
			continue
		}
		if !isScalar(c.Kind) {
			x.fail(ingest.ParseError, x.locator(c), "%s at %s is a%s, not a value", col.name, c.Locator.Path, article(string(c.Kind)))
			ok = false
			continue
		}
		rec.Fields[col.name] = c.Text
	}
	for _, col := range rm.amounts {
		c, found, err := read(col)
		if err != nil {
			return rec, false, err
		}
		if !found {
			continue
		}
		v, err := convert(factMap{fact: vocab.Fact{Name: col.name, Kind: domain.KindMoney}, unit: rm.unit}, c)
		if err != nil {
			x.fail(ingest.ParseError, x.locator(c), "%s at %s: %v", col.name, c.Locator.Path, err)
			ok = false
			continue
		}
		rec.Amounts[col.name] = v.Money
	}
	parts := make([]string, len(rm.rt.MatchKey))
	for i, k := range rm.rt.MatchKey {
		parts[i] = escapeKey(rec.Fields[k])
	}
	rec.MatchKey = strings.Join(parts, "|")
	return rec, ok, nil
}

// escapeKey escapes the separator so two different rows can never share a match key.
func escapeKey(s string) string {
	return strings.NewReplacer(`\`, `\\`, `|`, `\|`).Replace(s)
}

// listUnread adds every value of the file that nothing read to the report (IN-14).
func (x *extraction) listUnread() {
	ll, ok := x.doc.(ingest.LeafLister)
	if !ok {
		return
	}
	for _, c := range ll.Leaves() {
		if x.used[c.Locator.Path] {
			continue
		}
		reason := string(c.Kind) + ", not read by the profile"
		x.res.Report.Unread = append(x.res.Report.Unread, ingest.Unread{Locator: x.locator(c), Reason: reason})
	}
}
