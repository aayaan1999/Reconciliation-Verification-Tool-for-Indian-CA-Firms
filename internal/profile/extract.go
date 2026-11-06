package profile

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/currency"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	jsonadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/json"
	xlsxadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/xlsx"
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
	// USD is the fixed dollar rate of the run (SPEC-019), read once before the run. A zero Rate is
	// "no rate": a dollar amount met is then not read and a warning says so; rupee amounts are
	// not affected.
	USD currency.Rate
	// TaxYear is the tax year of the run, for a file that holds none of its own (SPEC-020:
	// the TDS files); read only by a profile with context.tax_year from_run. Zero: not given.
	TaxYear period.TaxYear
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
	// Dollars is true when the file holds an amount written in US dollars, converted or not: its
	// figures depend on the run's dollar rate (SPEC-019).
	Dollars bool
}

// Extract reads doc with profile p (IN-20, IN-21). It is deterministic: the same document
// always gives the same result, byte for byte.
func Extract(p *Profile, doc ingest.Doc, src Source) (*Result, error) {
	if !p.reads(doc) {
		return nil, fmt.Errorf("profile %s reads %s files, not %s", p.Key(), p.FileType, doc.FileType())
	}
	fileType := doc.FileType()
	doc, ok, err := p.read(doc)
	switch {
	case err != nil:
		return nil, named(err, src.FileName)
	case !ok:
		return nil, fmt.Errorf("profile %s: %s does not hold the sheets the profile reads", p.Key(), src.FileName)
	}
	x := &extraction{p: p, doc: doc, src: src, used: map[string]bool{}, rowsRead: map[string]bool{}, rowsLeft: map[string]string{}}
	x.res.Report = ingest.ValidationReport{
		File: src.FileName, FileType: fileType, DocumentType: p.DocumentType, ProfileID: p.ID, ProfileVersion: p.Version,
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
	switch d := doc.(type) {
	case *xlsxadapter.View:
		x.sheetCells(d)
	case *jsonadapter.Tree:
		x.relocate(d.Source) // each value's cell or line in the CSV or Excel file
	}

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
	// The rows of the file's row lists: read, or left out with the reason (for the report
	// of what was not read).
	rowsRead map[string]bool
	rowsLeft map[string]string
	// refused: the context values already reported as having two rows that meet their
	// conditions, so the report does not also call them missing (SPEC-011 part 2).
	refused map[string]bool
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
	if s.constantValue() {
		return s.constant, domain.Locator{}, true, nil
	}
	sel := s.sel
	if s.rows != nil {
		// The one entry of the list meeting the conditions (SPEC-011 part 2); none is "not in the file".
		rows, err := x.doc.Select(s.rows)
		if err != nil {
			return "", domain.Locator{}, false, err
		}
		var kept []ingest.Cell
		for _, r := range rows {
			ok, _, err := x.keep(s.where, r)
			if err != nil {
				return "", domain.Locator{}, false, err
			}
			if ok {
				kept = append(kept, r)
			}
		}
		switch len(kept) {
		case 0:
			return "", domain.Locator{}, false, nil
		case 1:
		default:
			x.fail(ingest.ParseError, x.locator(kept[1]), "%s: %d entries meet the conditions; the tool does not choose between them", name, len(kept))
			if x.refused == nil {
				x.refused = map[string]bool{}
			}
			x.refused[name] = true
			return "", domain.Locator{}, false, nil
		}
		if sel, err = syntaxes[x.p.FileType].parse(kept[0].Locator.Path + s.rel); err != nil {
			return "", domain.Locator{}, false, err
		}
	}
	c, ok, err := x.one(sel)
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

	if m.taxYear.fromRun {
		// SPEC-020: the file holds no tax year; the run's is used, never a guess.
		if x.src.TaxYear.IsZero() {
			x.fail(ingest.RequiredFieldMissing, domain.Locator{}, "the tax year is not in the file and the run gave none")
		} else {
			ctx.TaxYear, rep.TaxYear = x.src.TaxYear, x.src.TaxYear
		}
	} else if !m.taxYear.fromPeriod {
		ty, loc, ok, err := x.text("the tax year", m.taxYear)
		if err != nil {
			return err
		}
		if !ok {
			if !x.refused["the tax year"] { // two rows giving it is reported already
				x.fail(ingest.RequiredFieldMissing, loc, "the tax year is not in the file")
			}
		} else if m.taxYear.fromMonth {
			if pp, perr := period.Parse(ty); perr != nil {
				x.fail(ingest.ParseError, loc, "the month %q cannot be read: %v", ty, perr)
			} else {
				ctx.TaxYear, rep.TaxYear = pp.TaxYear(), pp.TaxYear()
			}
		} else if t, perr := period.ParseTaxYear(ty); perr != nil {
			x.fail(ingest.ParseError, loc, "the tax year %q cannot be read: %v", ty, perr)
		} else {
			ctx.TaxYear, rep.TaxYear = t, t
		}
	}

	pt, loc, ok, err := x.text("the period", m.period)
	if err != nil {
		return err
	}
	unreadable := false // the period is in the file but not in its format: reported once, as that
	if !ok && x.refused["the period"] {
		unreadable = true // two rows give the period: reported once, as that (SPEC-011 part 2)
	}
	if ok && m.period.format != "" {
		canon, cerr := canonicalPeriod(pt, m.period.format)
		if cerr != nil {
			x.fail(ingest.ParseError, loc, "the period %q cannot be read as %s", pt, m.period.format)
			ok, unreadable = false, true
		} else {
			pt = canon
		}
	}
	if m.taxYear.fromPeriod {
		switch pp, perr := period.Parse(pt); {
		case unreadable:
			// already reported
		case !ok:
			x.fail(ingest.RequiredFieldMissing, loc, "the period is not in the file, so its tax year is not known")
		case perr != nil:
			x.fail(ingest.ParseError, loc, "the period %q cannot be read: %v", pt, perr)
		default:
			ctx.TaxYear, rep.TaxYear, ctx.Period = pp.TaxYear(), pp.TaxYear(), pp.String()
		}
	}
	switch {
	case m.taxYear.fromPeriod, unreadable:
		// read with the tax year above, or already reported
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
	if ok && m.pan.fromGSTIN {
		if len(pan) != 15 {
			x.warn(ingest.ParseError, loc, "the GSTIN at %s is not 15 characters, so no PAN is read from it", loc.Path)
			ok = false
		} else {
			pan = pan[2:12]
		}
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
		val, loc, state, err := x.factValue(fm)
		if err != nil {
			return err
		}
		switch state {
		case factMissing:
			if fm.required {
				x.fail(ingest.RequiredFieldMissing, loc, "required figure %s is not in the file at %s", fm.fact.Name, loc.Path)
			}
			continue
		case factBad:
			continue // already reported
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

const (
	factFound = iota
	factMissing
	factBad // a problem already reported: neither a figure nor "missing"
)

// factValue reads one figure: from its place, or from the entries of a list that meet its
// conditions (SPEC-006 part 2). It never takes an entry by its position in the list.
func (x *extraction) factValue(fm factMap) (domain.Value, domain.Locator, int, error) {
	if fm.rows == nil {
		c, ok, err := x.one(fm.sel)
		if err != nil || !ok {
			return domain.Value{}, x.at(fm.sel), factMissing, err
		}
		loc := x.locator(c)
		val, err := x.convert(fm, c)
		if x.dollarRateWarn(err, loc, fm.fact.Name, c) {
			return domain.Value{}, loc, factMissing, nil
		}
		if err != nil {
			x.fail(ingest.ParseError, loc, "%s at %s: %v", fm.fact.Name, c.Locator.Path, err)
			return domain.Value{}, loc, factBad, nil
		}
		return val, loc, factFound, nil
	}
	where := fm.rows.String() + fm.rel
	missing := domain.Locator{DocumentVersionID: x.src.DocumentVersionID, FileName: x.src.FileName,
		Kind: syntaxes[x.p.FileType].locatorKind, Path: where}
	syn := syntaxes[x.p.FileType]
	rows, err := x.doc.Select(fm.rows)
	if err != nil {
		return domain.Value{}, missing, factMissing, err
	}
	// unsure reports a figure that is there but cannot be read without guessing: a warning, or
	// for a required figure an error, since a required figure that is not read is missing.
	unsure := func(msg string) (domain.Value, domain.Locator, int, error) {
		if fm.required {
			x.fail(ingest.RequiredFieldMissing, missing, "required figure %s is not read: %s", fm.fact.Name, msg)
		} else {
			x.res.Report.Warnings = append(x.res.Report.Warnings, ingest.Issue{Code: ingest.EntryAmbiguous, Locator: missing, Msg: msg})
		}
		return domain.Value{}, missing, factBad, nil
	}
	var hits []ingest.Cell
	var has []bool // per hit, whether the entry holds the figure (only "latest" keeps entries without it)
	var dates []string
	for _, row := range rows {
		if row.Kind != ingest.CellObject {
			continue
		}
		keep, _, err := x.keep(fm.where, row)
		if err != nil {
			return domain.Value{}, missing, factMissing, err
		}
		if !keep {
			continue
		}
		for _, w := range fm.where { // the conditions of an entry used were read
			if sel, err := syn.parse(row.Locator.Path + w.rel); err == nil {
				if err := x.markUsed(sel); err != nil {
					return domain.Value{}, missing, factMissing, err
				}
			}
		}
		sel, err := syn.parse(row.Locator.Path + fm.rel)
		if err != nil {
			return domain.Value{}, missing, factMissing, err
		}
		c, ok, err := x.one(sel)
		if err != nil {
			return domain.Value{}, missing, factMissing, err
		}
		if !ok && fm.combine != CombineLatest {
			continue
		}
		// For "latest" an entry without the figure still takes part: when it is the latest,
		// there is no figure, rather than an older entry's.
		hits, has = append(hits, c), append(has, ok)
		if fm.combine == CombineLatest {
			bsel, err := syn.parse(row.Locator.Path + fm.by)
			if err != nil {
				return domain.Value{}, missing, factMissing, err
			}
			b, found, err := x.one(bsel)
			if err != nil {
				return domain.Value{}, missing, factMissing, err
			}
			// The latest entry is chosen by date, so every entry used needs a real date written
			// yyyy-mm-dd; otherwise the figure is not read rather than an entry being guessed.
			day, derr := period.ParseDate(b.Text)
			if !found || derr != nil {
				what := "has no date at " + bsel.String()
				if found {
					what = fmt.Sprintf("has %q at %s, which is not a date written yyyy-mm-dd", b.Text, b.Locator.Path)
				}
				return unsure(fmt.Sprintf("an entry of %s %s, so the latest entry for %s cannot be chosen; the figure is not read", fm.rows, what, fm.fact.Name))
			}
			dates = append(dates, day.String())
		}
	}
	if !slices.Contains(has, true) {
		return domain.Value{}, missing, factMissing, nil
	}
	ambiguous := func(n int) (domain.Value, domain.Locator, int, error) {
		return unsure(fmt.Sprintf("%d entries of %s meet the conditions of %s; the figure is not read rather than one being picked", n, fm.rows, fm.fact.Name))
	}
	pick := hits[0]
	switch fm.combine {
	case CombineOne:
		if len(hits) > 1 {
			return ambiguous(len(hits))
		}
	case CombineLatest:
		best := 0
		for i := range dates {
			if dates[i] > dates[best] {
				best = i
			}
		}
		ties := 0
		for _, d := range dates {
			if d == dates[best] {
				ties++
			}
		}
		if ties > 1 {
			return ambiguous(ties)
		}
		if !has[best] {
			return unsure(fmt.Sprintf("the latest entry of %s (dated %s) has no %s, so the figure is not read rather than an older entry's taken", fm.rows, dates[best], fm.fact.Name))
		}
		pick = hits[best]
	case CombineSum:
		var total money.Paise
		for _, c := range hits {
			v, err := x.convert(fm, c)
			if x.dollarRateWarn(err, x.locator(c), fm.fact.Name, c) {
				return domain.Value{}, missing, factMissing, nil
			}
			if err != nil {
				x.fail(ingest.ParseError, x.locator(c), "%s at %s: %v", fm.fact.Name, c.Locator.Path, err)
				return domain.Value{}, missing, factBad, nil
			}
			if total, err = money.Add(total, v.Money); err != nil {
				x.fail(ingest.ParseError, missing, "%s: the sum is too large: %v", fm.fact.Name, err)
				return domain.Value{}, missing, factBad, nil
			}
		}
		missing.Path = where + " (sum of " + fmt.Sprint(len(hits)) + " entries)"
		return domain.MoneyValue(total), missing, factFound, nil
	}
	loc := x.locator(pick)
	val, err := x.convert(fm, pick)
	if x.dollarRateWarn(err, loc, fm.fact.Name, pick) {
		return domain.Value{}, loc, factMissing, nil
	}
	if err != nil {
		x.fail(ingest.ParseError, loc, "%s at %s: %v", fm.fact.Name, pick.Locator.Path, err)
		return domain.Value{}, loc, factBad, nil
	}
	return val, loc, factFound, nil
}

// convert turns a cell into a value of the fact's kind. Money never goes through floating
// point: number cells use money.ParseJSONNumber, text cells money.Parse (which accepts
// "1,12,450.50" and "(2,750.00)"), and a profile rounding uses money.ParseRounded.
func convert(fm factMap, c ingest.Cell, usd currency.Rate) (domain.Value, error) {
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
		case c.Kind == ingest.CellText && money.IsUSDText(c.Text):
			// An amount marked as US dollars is converted at the run's fixed rate (SPEC-019); with no
			// usable rate it is not read, and never taken as zero or guessed.
			if !usd.OK() {
				reason := usd.Problem
				if reason == "" {
					reason = currency.ReasonNotSet
				}
				return domain.Value{}, &DollarRateError{Reason: reason}
			}
			p, err = money.ParseUSDAsPaise(c.Text, usd.Value)
			var pe *money.ParseError
			if errors.As(err, &pe) {
				// The message names the reason, not the figure (client data).
				err = fmt.Errorf("cannot read an amount in US dollars: %s", pe.Reason)
			}
			if errors.Is(err, money.ErrOverflow) {
				err = fmt.Errorf("an amount in US dollars is too large: %w", money.ErrOverflow)
			}
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
		if fm.yesNo && c.Kind == ingest.CellText {
			// "Yes" or "No" written as text (SPEC-011 part 2); anything else is not read as either.
			switch strings.ToLower(strings.TrimSpace(c.Text)) {
			case "yes":
				return domain.BoolValue(true), nil
			case "no":
				return domain.BoolValue(false), nil
			}
			return domain.Value{}, fmt.Errorf("expected Yes or No, found other text")
		}
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

// convert reads a cell with the run's dollar rate, and notes on the result that the file holds an
// amount in US dollars: such a file reads differently at another rate (SPEC-019).
func (x *extraction) convert(fm factMap, c ingest.Cell) (domain.Value, error) {
	if fm.fact.Kind == domain.KindMoney && c.Kind == ingest.CellText && money.IsUSDText(c.Text) {
		x.res.Dollars = true
	}
	return convert(fm, c, x.src.USD)
}

// A DollarRateError says an amount in US dollars was met and the run has no usable dollar rate
// (SPEC-019). It is not a damaged file: the figure is left out and a warning gives the reason.
type DollarRateError struct{ Reason string }

func (e *DollarRateError) Error() string { return e.Reason }

// dollarRateWarn reports whether err is a DollarRateError and, if so, records the warning. The
// warning names the file and the place, never the amount.
func (x *extraction) dollarRateWarn(err error, loc domain.Locator, name string, c ingest.Cell) bool {
	var dre *DollarRateError
	if !errors.As(err, &dre) {
		return false
	}
	x.warn(ingest.DollarRate, loc, "%s at %s: %s, so this figure is not read", name, c.Locator.Path, dre.Reason)
	return true
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
			if row.Kind == ingest.CellObject {
				keep, why, err := x.keep(rm.where, row)
				if err != nil {
					return err
				}
				if !keep {
					// A row the profile does not read, such as a credit note among invoices. With several
					// blocks over the same rows (SPEC-015) the first block's reason is kept, and a row
					// another block reads is not left out.
					if _, seen := x.rowsLeft[row.Locator.Path]; !seen {
						x.rowsLeft[row.Locator.Path] = why
					}
					continue
				}
			}
			x.rowsRead[row.Locator.Path] = true
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
	// read returns the cells of col: one, or every match of a path with [*].
	read := func(col column) ([]ingest.Cell, error) {
		base, err := parentPath(row.Locator.Path, col.up)
		if err != nil {
			return nil, err
		}
		sel, err := syntaxes[x.p.FileType].parse(base + col.rel)
		if err != nil {
			return nil, err
		}
		cells, err := x.doc.Select(sel)
		if err != nil {
			return nil, err
		}
		var out []ingest.Cell
		for _, c := range cells {
			x.markCell(c)
			if c.Kind != ingest.CellNull {
				out = append(out, c)
			}
		}
		if len(out) == 0 && col.required {
			x.fail(ingest.RequiredFieldMissing, x.locator(row), "%s row %s has no %s", rm.rt.Name, row.Locator.Path, col.name)
			ok = false
		}
		return out, nil
	}
	for _, col := range rm.fields {
		if col.constant != "" {
			rec.Fields[col.name] = col.constant // the same for every row of the block (SPEC-015)
			continue
		}
		cells, err := read(col)
		if err != nil {
			return rec, false, err
		}
		var vals []string
		for _, c := range cells {
			if !isScalar(c.Kind) {
				x.fail(ingest.ParseError, x.locator(c), "%s at %s is a%s, not a value", col.name, c.Locator.Path, article(string(c.Kind)))
				ok = false
				continue
			}
			t := c.Text
			if col.dateFormat != "" {
				iso, err := isoDate(t, col.dateFormat)
				if err != nil {
					x.fail(ingest.ParseError, x.locator(c), "%s at %s: %q is not a date written %s", col.name, c.Locator.Path, t, col.dateFormat)
					ok = false
					continue
				}
				t = iso
			}
			if col.many {
				t = escapeList(t)
			}
			if !slices.Contains(vals, t) {
				vals = append(vals, t)
			}
		}
		if len(vals) > 0 {
			slices.Sort(vals)
			rec.Fields[col.name] = strings.Join(vals, ",")
		}
	}
	// An amount in dollars that cannot be converted leaves every amount of its row unread, so a check
	// that needs the row cannot be made from the amounts that remain: a row compared on its other
	// amounts alone would pass or fail without the figure that was not read (SPEC-019).
	dollarUnread, dollarReason := false, ""
	for _, col := range rm.amounts {
		cells, err := read(col)
		if err != nil {
			return rec, false, err
		}
		if len(cells) == 0 {
			continue
		}
		var total money.Paise
		good := true
		for _, c := range cells {
			// A ledger writes the side after the amount: Dr is as it is, Cr is negated (SPEC-016).
			credit := false
			if rm.drcr {
				c.Text, credit = splitDrCr(c.Text)
			}
			v, err := x.convert(factMap{fact: vocab.Fact{Name: col.name, Kind: domain.KindMoney}, unit: rm.unit}, c)
			if x.dollarRateWarn(err, x.locator(c), col.name, c) {
				good, dollarUnread = false, true // the amount is left out of the row, not zero
				var dre *DollarRateError
				if errors.As(err, &dre) && dollarReason == "" {
					dollarReason = dre.Reason // the first reason stands when several columns are in dollars
				}
				break
			}
			if err == nil && credit {
				v.Money, err = money.Neg(v.Money)
			}
			if err == nil {
				total, err = money.Add(total, v.Money)
			}
			if err != nil {
				x.fail(ingest.ParseError, x.locator(c), "%s at %s: %v", col.name, c.Locator.Path, err)
				ok, good = false, false
				break
			}
		}
		if good {
			if rm.negate {
				if total, err = money.Neg(total); err != nil {
					return rec, false, err
				}
			}
			rec.Amounts[col.name] = total
		}
	}
	if dollarUnread {
		rec.Amounts = map[string]money.Paise{}
		names := make([]string, len(rm.amounts))
		for i, col := range rm.amounts {
			names[i] = col.name
		}
		slices.Sort(names)
		rec.Fields[domain.UnreadField] = dollarReason + "|" + strings.Join(names, ",")
	}
	rec.MatchKey = domain.JoinMatchKey(rec.Fields, rm.rt.MatchKey)
	return rec, ok, nil
}

// escapeList escapes the separator of a field joined from several values, so {"A,B"} and
// {"A", "B"} never give the same field (or match key).
var listEscaper = strings.NewReplacer(`\`, `\\`, `,`, `\,`)

func escapeList(s string) string {
	return listEscaper.Replace(s)
}

// keep reports whether row meets every where-condition and, when it does not, says which
// condition it fails, in words.
func (x *extraction) keep(where []rowFilter, row ingest.Cell) (bool, string, error) {
	for _, w := range where {
		sel, err := syntaxes[x.p.FileType].parse(row.Locator.Path + w.rel)
		if err != nil {
			return false, "", err
		}
		cells, err := x.doc.Select(sel)
		if err != nil {
			return false, "", err
		}
		want, wanted := w.equals, strconv.Quote(w.equals)
		if w.year != "" {
			// The file's own year, read with its context before any figure (SPEC-011 part 2).
			ty := x.res.Context.TaxYear
			if ty.IsZero() {
				return false, fmt.Sprintf("%s cannot be compared: the tax year is not in the file", w.rel), nil
			}
			if w.year == YearAssessment {
				ty = period.TaxYear{StartYear: ty.StartYear + 1}
			}
			want, wanted = ty.String(), "the file's "+strings.ReplaceAll(w.year, "_", " ")
		}
		if w.equalsSel != nil {
			other, err := x.doc.Select(w.equalsSel)
			if err != nil {
				return false, "", err
			}
			if len(other) != 1 || !isScalar(other[0].Kind) {
				return false, fmt.Sprintf("%s cannot be compared: %s is not in the file", w.rel, w.equalsSel), nil
			}
			x.markCell(other[0]) // the place compared with is read, once for all rows
			// The other value is the client's (a GSTIN, say), so the reason names its place.
			want, wanted = other[0].Text, "the value at "+other[0].Locator.Path
		}
		scalar := len(cells) == 1 && isScalar(cells[0].Kind)
		if w.not != "" {
			if scalar && cells[0].Text != w.not {
				continue
			}
			got := "missing"
			if scalar {
				got = strconv.Quote(w.not)
			}
			return false, fmt.Sprintf("%s is %s; this format reads only rows where %s is not %q", w.rel, got, w.rel, w.not), nil
		}
		if scalar && (w.in == nil && cells[0].Text == want || slices.Contains(w.in, cells[0].Text)) {
			continue
		}
		// The row's own value is client data: the reason says only that it differs.
		got := "missing"
		if scalar {
			got = "another value"
		}
		if w.in != nil {
			quoted := make([]string, len(w.in))
			for i, v := range w.in {
				quoted[i] = strconv.Quote(v)
			}
			return false, fmt.Sprintf("%s is %s; this format reads only rows where %s is one of %s", w.rel, got, w.rel, strings.Join(quoted, ", ")), nil
		}
		return false, fmt.Sprintf("%s is %s; this format reads only rows where %s is %s", w.rel, got, w.rel, wanted), nil
	}
	return true, "", nil
}

// parentPath goes up `up` levels from a row path: from "$.b2b[0].inv[3]" one level up is
// "$.b2b[0]", the object that holds the list the row is in.
func parentPath(path string, up int) (string, error) {
	for range up {
		i := strings.LastIndexByte(path, '[')
		if i <= 0 || !strings.HasSuffix(path, "]") || strings.Trim(path[i+1:len(path)-1], "0123456789") != "" || i+2 == len(path) {
			// The last step must be an index ("[3]"): a quoted key ("['k']") is an object, not a
			// list entry, and taking it for one would go up two levels.
			return "", fmt.Errorf("row %s is not inside a list, so ^ cannot go up from it", path)
		}
		path = path[:i]
		if strings.HasSuffix(path, "']") {
			j := strings.LastIndex(path, "['")
			if j < 0 {
				return "", fmt.Errorf("cannot go up from %s", path)
			}
			path = path[:j]
		} else if k := strings.LastIndexByte(path, '.'); k > 0 {
			path = path[:k]
		} else {
			return "", fmt.Errorf("cannot go up from %s", path)
		}
	}
	return path, nil
}

// canonicalPeriod turns a month written "082026" (mmyyyy), "202608" (yyyymm) or "Aug-2026"
// (mon-yyyy, as the GST portal's tables write it) into "2026-08".
func canonicalPeriod(s, format string) (string, error) {
	if format == "yyyy-mm-dd" {
		// A date (a register with no period column): its month is the period (SPEC-018).
		d, err := isoDate(s, "yyyy-mm-dd")
		if err != nil {
			return "", fmt.Errorf("not a date such as 2026-09-03")
		}
		return d[:7], nil
	}
	if format == "mon-yyyy" {
		const months = "janfebmaraprmayjunjulaugsepoctnovdec"
		if len(s) != 8 || s[3] != '-' || strings.Trim(s[4:], "0123456789") != "" {
			return "", fmt.Errorf("not a month and year such as Aug-2026")
		}
		n := strings.Index(months, strings.ToLower(s[:3]))
		if n < 0 || n%3 != 0 {
			return "", fmt.Errorf("not a month and year such as Aug-2026")
		}
		return fmt.Sprintf("%s-%02d", s[4:], n/3+1), nil
	}
	if len(s) != 6 || strings.Trim(s, "0123456789") != "" {
		return "", fmt.Errorf("not six digits")
	}
	switch format {
	case "mmyyyy":
		return s[2:] + "-" + s[:2], nil
	case "yyyymm":
		return s[:4] + "-" + s[4:], nil
	}
	return "", fmt.Errorf("unknown period format %q", format)
}

// isoDate turns a date written in format into "yyyy-mm-dd", checking it is a real date.
func isoDate(s, format string) (string, error) {
	iso := s
	if format == "dd-mm-yyyy" {
		if len(s) != 10 || s[2] != '-' || s[5] != '-' {
			return "", fmt.Errorf("not dd-mm-yyyy")
		}
		iso = s[6:] + "-" + s[3:5] + "-" + s[:2]
	}
	d, err := period.ParseDate(iso)
	if err != nil {
		return "", err
	}
	return d.String(), nil
}

// listUnread adds every value of the file that nothing read to the report (IN-14), with
// why: in a row the format left out (and the condition it failed), a column of a row that
// was read which no check uses yet, or another part of the file.
func (x *extraction) listUnread() {
	ll, ok := x.doc.(ingest.LeafLister)
	if !ok {
		return
	}
	for _, c := range ll.Leaves() {
		if x.used[c.Locator.Path] {
			continue
		}
		u := ingest.Unread{Locator: x.locator(c), Kind: ingest.UnreadOther,
			Reason: string(c.Kind) + "; this format does not read this part of the file"}
		if v, ok := x.doc.(*xlsxadapter.View); ok {
			if row, why, left := v.LeftOut(c.Locator.Path); left { // a total row of an Excel table
				u.Kind, u.Row, u.Reason = ingest.UnreadRowLeftOut, row, "row left out: "+why
				x.res.Report.Unread = append(x.res.Report.Unread, u)
				continue
			}
		}
		if row, ok := x.rowOf(c.Locator.Path); ok {
			u.Row = row
			if why, left := x.rowsLeft[row]; left && !x.rowsRead[row] {
				u.Kind, u.Reason = ingest.UnreadRowLeftOut, "row left out: "+why
			} else {
				u.Kind, u.Reason = ingest.UnreadColumn, "column of a row that was read; no check uses it yet"
			}
		}
		x.res.Report.Unread = append(x.res.Report.Unread, u)
	}
}

// sheetCells says, for an Excel file, what of its tables is hidden, and turns every
// locator from the path of a value in the tables into its sheet and cell ("Sales
// Register!C14", or "Sales Register!14:14" for a row). A place the file does not have keeps
// its path, which names the sheet and the header label looked for.
func (x *extraction) sheetCells(v *xlsxadapter.View) {
	for _, n := range v.Notes() {
		x.warn(n.Code, domain.Locator{DocumentVersionID: x.src.DocumentVersionID, FileName: x.src.FileName}, "%s", n.Msg)
	}
	x.relocate(func(p string) (domain.Locator, bool) {
		n, ok := v.CellName(p)
		return domain.Locator{Kind: domain.LocatorSheetCell, Path: n}, ok
	})
}

// relocate turns every locator from the path of a value into its place in the file, as
// place gives it (only Kind and Path are used). A place the file does not have keeps its
// path.
func (x *extraction) relocate(place func(path string) (domain.Locator, bool)) {
	cell := func(l *domain.Locator) {
		if n, ok := place(l.Path); ok {
			l.Kind, l.Path = n.Kind, n.Path
		}
	}
	r := &x.res
	for i := range r.Facts {
		cell(&r.Facts[i].Locator)
	}
	for i := range r.Records {
		cell(&r.Records[i].Locator)
	}
	for _, list := range [][]ingest.Issue{r.Report.Errors, r.Report.Warnings} {
		for i := range list {
			// The message names the same place: as its sheet and cell, too.
			old := list[i].Locator.Path
			cell(&list[i].Locator)
			if n := list[i].Locator.Path; old != "" && n != old {
				list[i].Msg = replacePath(list[i].Msg, old, n)
			}
		}
	}
	for i := range r.Report.Unread {
		u := &r.Report.Unread[i]
		cell(&u.Locator)
		if n, ok := place(u.Row); ok {
			u.Row = n.Path
		}
	}
}

// replacePath replaces each whole occurrence of path old in msg with n. An occurrence that
// is only the start of a longer path is left alone: "$.Ledger" inside "$.Ledgers[2]", or
// "$['S'][1]" inside "$['S'][1].['Taxable Value']", names a different place.
func replacePath(msg, old, n string) string {
	var b strings.Builder
	for {
		i := strings.Index(msg, old)
		if i < 0 {
			b.WriteString(msg)
			return b.String()
		}
		end := i + len(old)
		b.WriteString(msg[:i])
		if continuesPath(msg[end:]) {
			b.WriteString(old)
		} else {
			b.WriteString(n)
		}
		msg = msg[end:]
	}
}

// continuesPath reports whether rest, the text right after a path in a message, makes it a
// longer path: a name character or "[" directly after it, or a "." followed by more of a
// path (a "." that ends the sentence does not).
func continuesPath(rest string) bool {
	if rest == "" {
		return false
	}
	switch c := rest[0]; {
	case c == '[' || pathChar(c):
		return true
	case c == '.':
		return len(rest) > 1 && (rest[1] == '[' || pathChar(rest[1]))
	}
	return false
}

func pathChar(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// rowOf returns the innermost row (read or left out) that holds the value at path.
func (x *extraction) rowOf(path string) (string, bool) {
	for i := len(path) - 1; i > 0; i-- {
		if path[i] != '.' && path[i] != '[' {
			continue
		}
		if p := path[:i]; x.rowsRead[p] || x.rowsLeft[p] != "" {
			return p, true
		}
	}
	return "", false
}

// splitDrCr takes a trailing "Dr" or "Cr" (any case, with or without a full stop or a space before
// it) off an amount, and says whether it was "Cr". A text with neither is returned as it is.
func splitDrCr(text string) (string, bool) {
	s := strings.TrimSpace(text)
	s = strings.TrimSuffix(s, ".")
	if len(s) < 3 {
		return text, false
	}
	switch strings.ToLower(s[len(s)-2:]) {
	case "dr":
		return strings.TrimSpace(s[:len(s)-2]), false
	case "cr":
		return strings.TrimSpace(s[:len(s)-2]), true
	}
	return text, false
}
