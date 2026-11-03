package xlsx

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	jsonadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/json"
)

// A Table is one sheet a profile reads: the sheet's name and labels its header row holds.
type Table struct {
	Sheet  string
	Header []string // normalised with Label
	// AnyName reads the table from whichever sheet holds its header row, whatever the sheet
	// is called ("Sheet1", "GST Sales"); Sheet is then only the name the profile's paths use.
	// Two sheets holding the header row are an error: the tool does not guess.
	AnyName bool
	// Totals are the texts that mark a total row ("Total", "Grand Total"): a row with one of
	// them in a table column is left out, and the report says why.
	Totals []string
	// Labels are every label the profile reads from its tables, Header included. A column
	// whose header text differs from one of them only in capitals is keyed by the profile's
	// spelling, so "Taxable value" in the file is read as "Taxable Value".
	Labels []string
	// BlankAsText are header labels whose empty cell, in a row that is read, is read as empty
	// text rather than left out (SPEC-011 part 2): a blank UDIN is blank, not missing. Every
	// other empty cell is still left out, so an amount never written stays "cannot check".
	BlankAsText []string
	// Headings, when set, is the name of a column the table gains (SPEC-011 part 2). A row
	// whose first column is filled and every other column empty is a heading; each row under
	// it whose first cell is indented (starts with spaces) gets the heading's label in this
	// column, and its own label without the indent. A row that is not indented ends the block
	// and has no heading. So two rows labelled "Total" under two headings can be told apart
	// by their conditions, never by their position.
	Headings string
}

// indented reports whether a label starts with spaces or tabs: a row under a heading.
func indented(s string) bool {
	return s != "" && strings.TrimLeft(s, " \t ") != s && strings.TrimSpace(s) != ""
}

// Label normalises a header text: spaces at either end are dropped and every run of
// spaces, tabs or line breaks inside becomes one space. Header labels are then matched
// without regard to capitals.
func Label(s string) string { return strings.Join(strings.Fields(s), " ") }

// fold is how two labels are compared: after Label, ignoring capitals.
func fold(s string) string { return strings.ToLower(Label(s)) }

// A NotFoundError says a table a profile names is not in the workbook: no such sheet, or
// no row holding every header label. Detection treats it as "this profile does not match";
// its message says what is missing, for the report when no profile matches.
type NotFoundError struct {
	Sheet   string
	Msg     string
	missing int // how many header labels the nearest row lacks
}

func (e *NotFoundError) Error() string { return fmt.Sprintf("sheet %q: %s", e.Sheet, e.Msg) }

// A Note is something about the tables a report should say: what is hidden in the
// workbook, or empty rows inside a table.
type Note struct {
	Code ingest.Code
	Msg  string
}

// A View is a workbook read as the tables of one profile, in the tree JSON profiles
// address: {"<sheet>": [ {<header label>: <value>, ...}, ... ]}. "$['Sales Register'][*]" is
// every data row, ".['Voucher No']" a column of one. Each value keeps its cell, given by
// CellName.
type View struct {
	*jsonadapter.Doc
	names map[string]string // JSON path → "Sheet!C14" (a value) or "Sheet!14:14" (a row)
	rest  []ingest.Cell     // the cells of the workbook outside every table, as "Sheet!A1"
	notes []Note
	left  map[string]leftOut // "Sheet!C14" of each cell of a row left out of a table
	all   []ingest.Cell      // Leaves, built once
}

// leftOut is a row of a table that is not read (a total row), and why.
type leftOut struct{ row, why string }

// FileType implements ingest.Doc.
func (v *View) FileType() string { return FileType }

// Leaves implements ingest.LeafLister: every value of the tables, then every other
// non-empty cell of the workbook, so a validation report lists what nothing read.
func (v *View) Leaves() []ingest.Cell { return slices.Clone(v.all) }

// CellName returns the sheet and cell of the value or row at the JSON path p.
func (v *View) CellName(p string) (string, bool) {
	n, ok := v.names[p]
	return n, ok
}

// LeftOut reports whether the cell named name ("Sheet!C14") is in a row of a table that
// was left out, such as a total row: the row ("Sheet!14:14") and why.
func (v *View) LeftOut(name string) (row, why string, ok bool) {
	l, ok := v.left[name]
	return l.row, l.why, ok
}

// Notes says what of the tables is hidden in the workbook (hidden sheets, rows and
// columns are read like visible ones) and where a table has empty rows, sorted.
func (v *View) Notes() []Note { return slices.Clone(v.notes) }

var nameEscaper = strings.NewReplacer(`\`, `\\`, `'`, `\'`)

// SheetPath is the JSON path of a sheet's rows in a View: "$['Sales Register']".
func SheetPath(name string) string {
	return "$['" + nameEscaper.Replace(name) + "']"
}

// treeTable is the sheet of a JSON document flattened into paths and values (SPEC-011),
// whatever the sheet is called.
var treeTable = Table{Sheet: "Path and Value", Header: jsonadapter.TreeHeader[:], AnyName: true, Labels: jsonadapter.TreeHeader[:]}

// JSONTree implements ingest.Flattened: the one sheet whose header row holds Path and Value
// is the JSON document its rows describe, each value's place being its cell.
func (d *Doc) JSONTree() (ingest.Doc, bool, error) {
	v, err := d.View([]Table{treeTable})
	var nf *NotFoundError
	switch {
	case errors.As(err, &nf):
		return nil, false, nil
	case err != nil:
		return nil, false, err
	}
	all, err := jsonadapter.ParsePath(SheetPath(treeTable.Sheet) + "[*]")
	if err != nil {
		return nil, false, err
	}
	rows, err := v.Select(all)
	if err != nil {
		return nil, false, err
	}
	at := func(row ingest.Cell, key string) (ingest.Cell, error) {
		sel, err := jsonadapter.ParsePath(row.Locator.Path + "." + key)
		if err != nil {
			return ingest.Cell{}, err
		}
		cells, err := v.Select(sel)
		if err != nil || len(cells) == 0 {
			return ingest.Cell{Kind: ingest.CellText, Locator: row.Locator}, err // an empty cell: empty text
		}
		return cells[0], nil
	}
	flat := make([]jsonadapter.Row, 0, len(rows))
	for _, row := range rows {
		path, err := at(row, treeTable.Header[0])
		if err != nil {
			return nil, false, err
		}
		val, err := at(row, treeTable.Header[1])
		if err != nil {
			return nil, false, err
		}
		name, _ := v.CellName(val.Locator.Path)
		val.Locator = domain.Locator{Kind: domain.LocatorSheetCell, Path: name}
		flat = append(flat, jsonadapter.Row{Path: path.Text, Value: val})
	}
	t, err := jsonadapter.BuildTree(flat)
	return t, err == nil, err
}

// View reads the workbook as tables. In each sheet the header row is the first row that
// holds every label of its Table. The table's columns are every column of that row with a
// header; its rows are every row below that holds a value under one of them, to the end of
// the sheet, so an empty row inside the data never cuts the table short (the report says
// where the empty rows are). A total row is left out. A header text found twice in the
// header row is left out of the table, or is an error when the profile names it. A sheet
// or header row that is not there is a *NotFoundError.
func (d *Doc) View(tables []Table) (*View, error) {
	key := fmt.Sprintf("%#v", tables)
	d.mu.Lock()
	defer d.mu.Unlock()
	if v, ok := d.views[key]; ok {
		return v, nil
	}
	v, err := d.view(tables)
	if err != nil {
		return nil, err
	}
	if d.views == nil {
		d.views = map[string]*View{}
	}
	d.views[key] = v
	return v, nil
}

func (d *Doc) view(tables []Table) (*View, error) {
	v := &View{names: map[string]string{}, left: map[string]leftOut{}}
	used := map[*sheet]map[int64]bool{} // per sheet, every cell in a table (header cells included), by cellKey
	var refs []string                   // the cell of every value, in the order the tree lists them
	var rowsAt [][]string               // per table, "Sheet!14:14" of each data row
	var read []*sheet                   // the sheet of each table so far
	var out bytes.Buffer
	out.WriteByte('{')
	for ti, t := range tables {
		if slices.ContainsFunc(tables[:ti], func(o Table) bool { return o.Sheet == t.Sheet }) {
			return nil, fmt.Errorf("xlsx: sheet %q is named twice", t.Sheet)
		}
		s, h, cols, err := d.find(t)
		if err != nil {
			return nil, err
		}
		if slices.Contains(read, s) {
			return nil, &ingest.Error{Code: ingest.ParseError, Msg: fmt.Sprintf("sheet %q holds two of the tables this format reads", s.name)}
		}
		read = append(read, s)
		for _, c := range cols {
			mark(used, s, h, c.col)
		}
		if ti > 0 {
			out.WriteByte(',')
		}
		k, _ := json.Marshal(t.Sheet)
		out.Write(k)
		out.WriteString(":[")
		var at []string
		prev := h // the last row of the table so far, to find empty rows inside it
		var gaps [][2]int
		var heading, headingRef string // the heading the next indented row sits under, and its cell
		for _, r := range s.rowNums {
			if r <= h || !slices.ContainsFunc(cols, func(c headerCol) bool { _, ok := s.rows[r][c.col]; return ok }) {
				continue // above the header, or nothing under the table's headings: not read
			}
			if r > prev+1 {
				gaps = append(gaps, [2]int{prev + 1, r - 1})
			}
			prev = r
			rowName := s.name + "!" + strconv.Itoa(r) + ":" + strconv.Itoa(r)
			if total, label, ok := totalRow(s, r, cols, t.Totals); ok {
				why := fmt.Sprintf("a total row (%q under %q); this format does not read totals", total, label)
				for _, c := range s.cols[r] {
					v.left[cellName(s.name, c, r)] = leftOut{row: rowName, why: why}
				}
				continue
			}
			if len(at) > 0 {
				out.WriteByte(',')
			}
			at = append(at, rowName)
			// The heading this row sits under, when the table has headings (SPEC-011 part 2).
			rowHeading, rowHeadingRef, unindent := "", "", false
			if t.Headings != "" {
				first, firstOK := s.rows[r][cols[0].col]
				others := slices.ContainsFunc(cols[1:], func(c headerCol) bool { _, ok := s.rows[r][c.col]; return ok })
				switch {
				case firstOK && !others && first.kind == ingest.CellText:
					heading, headingRef = Label(first.text), cellName(s.name, cols[0].col, r)
				case firstOK && first.kind == ingest.CellText && indented(first.text):
					rowHeading, rowHeadingRef, unindent = heading, headingRef, true
				default:
					heading, headingRef = "", ""
				}
			}
			out.WriteByte('{')
			n := 0
			for ci, c := range cols {
				val, ok := s.rows[r][c.col]
				if !ok && slices.ContainsFunc(t.BlankAsText, func(l string) bool { return fold(l) == fold(c.label) }) {
					val, ok = cell{kind: ingest.CellText}, true // a blank the profile reads as empty text
				}
				if !ok {
					continue
				}
				if ci == 0 && unindent {
					val.text = Label(val.text)
				}
				if n > 0 {
					out.WriteByte(',')
				}
				n++
				k, _ := json.Marshal(c.label)
				out.Write(k)
				out.WriteByte(':')
				switch val.kind {
				case ingest.CellNumber, ingest.CellBool:
					out.WriteString(val.text)
				default:
					b, _ := json.Marshal(val.text)
					out.Write(b)
				}
				ref := cellName(s.name, c.col, r)
				mark(used, s, r, c.col)
				refs = append(refs, ref)
			}
			if rowHeading != "" {
				// The heading's own cell is the source of the heading value.
				if n > 0 {
					out.WriteByte(',')
				}
				k, _ := json.Marshal(t.Headings)
				out.Write(k)
				out.WriteByte(':')
				b, _ := json.Marshal(rowHeading)
				out.Write(b)
				refs = append(refs, rowHeadingRef)
			}
			out.WriteByte('}')
			if s.hiddenRows[r] {
				v.notes = append(v.notes, Note{ingest.HiddenContent, fmt.Sprintf("row %d of sheet %q is hidden in the workbook; it was read like a visible row", r, s.name)})
			}
		}
		out.WriteByte(']')
		rowsAt = append(rowsAt, at)
		for _, g := range gaps {
			rows := fmt.Sprintf("row %d is", g[0])
			if g[1] > g[0] {
				rows = fmt.Sprintf("rows %d to %d are", g[0], g[1])
			}
			v.notes = append(v.notes, Note{ingest.EmptyRows, fmt.Sprintf("%s empty in the table of sheet %q; the rows below were read as part of the table", rows, s.name)})
		}
		if s.hidden {
			v.notes = append(v.notes, Note{ingest.HiddenContent, fmt.Sprintf("sheet %q is hidden in the workbook; it was read like a visible sheet", s.name)})
		}
		for _, c := range cols {
			if s.hiddenCols[c.col] {
				name, _ := excelize.ColumnNumberToName(c.col)
				v.notes = append(v.notes, Note{ingest.HiddenContent, fmt.Sprintf("column %s (%q) of sheet %q is hidden in the workbook; it was read like a visible column", name, c.label, s.name)})
			}
		}
	}
	out.WriteByte('}')

	doc, err := jsonadapter.New(int64(out.Len())+1).Open(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		return nil, err
	}
	jd, ok := doc.(*jsonadapter.Doc)
	if !ok {
		return nil, errors.New("xlsx: the JSON reader returned an unexpected document")
	}
	v.Doc = jd
	leaves := jd.Leaves()
	if len(leaves) != len(refs) {
		return nil, fmt.Errorf("xlsx: %d values in the tables but %d cells", len(leaves), len(refs))
	}
	for i, l := range leaves {
		v.names[l.Locator.Path] = refs[i]
	}
	for ti, t := range tables {
		sel, err := jsonadapter.ParsePath(SheetPath(t.Sheet) + "[*]")
		if err != nil {
			return nil, err
		}
		rows, err := jd.Select(sel)
		if err != nil {
			return nil, err
		}
		for i, row := range rows {
			v.names[row.Locator.Path] = rowsAt[ti][i]
		}
	}
	for _, s := range d.sheets { // the other cells of the workbook, in sheet, row and column order
		for _, r := range s.rowNums {
			for _, c := range s.cols[r] {
				if !used[s][cellKey(r, c)] {
					v.rest = append(v.rest, s.leaf(r, c, s.rows[r][c]))
				}
			}
		}
	}
	v.all = slices.Concat(leaves, v.rest)
	slices.SortFunc(v.notes, func(a, b Note) int { return strings.Compare(a.Msg, b.Msg) })
	return v, nil
}

// totalRow reports whether row r is a total row: a text cell under one of cols reads one
// of totals (after Label, ignoring capitals). It returns that text and its column's label.
func totalRow(s *sheet, r int, cols []headerCol, totals []string) (string, string, bool) {
	for _, c := range cols {
		val, ok := s.rows[r][c.col]
		if !ok || val.kind != ingest.CellText {
			continue
		}
		if slices.ContainsFunc(totals, func(t string) bool { return fold(t) == fold(val.text) }) {
			return Label(val.text), c.label, true
		}
	}
	return "", "", false
}

func (d *Doc) sheet(name string) *sheet {
	for _, s := range d.sheets {
		if s.name == name {
			return s
		}
	}
	return nil
}

// find returns the sheet that holds t, its header row and its columns. For a table with
// AnyName it is the one sheet whose header row holds every label of t.
func (d *Doc) find(t Table) (*sheet, int, []headerCol, error) {
	if !t.AnyName {
		s := d.sheet(t.Sheet)
		if s == nil {
			return nil, 0, nil, &NotFoundError{Sheet: t.Sheet, Msg: "the workbook has no sheet of that name"}
		}
		h, cols, err := s.header(t)
		return s, h, cols, err
	}
	var found *sheet
	var fh int
	var fcols []headerCol
	var closest *NotFoundError // the sheet that came nearest, to say what is missing
	for _, s := range d.sheets {
		h, cols, err := s.header(t)
		var nf *NotFoundError
		switch {
		case errors.As(err, &nf):
			if closest == nil || nf.missing < closest.missing {
				closest = nf
			}
		case err != nil:
			return nil, 0, nil, err
		case found != nil:
			return nil, 0, nil, &ingest.Error{Code: ingest.ParseError,
				Msg: fmt.Sprintf("sheets %q and %q both have a header row with %s; the tool does not guess which to read", found.name, s.name, quoted(t.Header))}
		default:
			found, fh, fcols = s, h, cols
		}
	}
	if found == nil {
		msg := "the workbook has no sheet"
		if closest != nil {
			msg = "no sheet has a header row with " + quoted(t.Header) + "; the nearest, " + closest.Error()
		}
		return nil, 0, nil, &NotFoundError{Sheet: t.Sheet, Msg: msg, missing: len(t.Header)}
	}
	return found, fh, fcols, nil
}

// A headerCol is one column of a table: where it is and its label.
type headerCol struct {
	col   int
	label string
}

// header finds t's header row: the first row holding every label of t, matched without
// regard to capitals. It returns the row and the table's columns in sheet order, each
// keyed by the profile's spelling of its label when the profile reads it.
func (s *sheet) header(t Table) (int, []headerCol, error) {
	spelling := map[string]string{} // folded label → the profile's spelling
	for _, l := range slices.Concat(t.Labels, t.Header) {
		spelling[fold(l)] = l
	}
	bestRow, best := 0, 0 // the row with the most header labels, to say what is missing
	for _, r := range s.rowNums {
		at := map[string][]int{} // folded label → its columns in this row
		for _, c := range s.cols[r] {
			if l := fold(s.rows[r][c].text); l != "" {
				at[l] = append(at[l], c)
			}
		}
		n := 0
		for _, l := range t.Header {
			if len(at[fold(l)]) > 0 {
				n++
			}
		}
		if n > best {
			bestRow, best = r, n
		}
		if n < len(t.Header) {
			continue
		}
		for _, l := range t.Header {
			if cs := at[fold(l)]; len(cs) > 1 {
				first, _ := excelize.CoordinatesToCellName(cs[0], r)
				second, _ := excelize.CoordinatesToCellName(cs[1], r)
				return 0, nil, &ingest.Error{Code: ingest.ParseError,
					Msg: fmt.Sprintf("sheet %q: the header row (row %d) has the label %q twice, at %s and %s; the tool does not guess which column to read", s.name, r, l, first, second)}
			}
		}
		var cols []headerCol
		for _, c := range s.cols[r] {
			text := Label(s.rows[r][c].text)
			if k := fold(text); k != "" && len(at[k]) == 1 {
				if p, ok := spelling[k]; ok {
					text = p
				}
				cols = append(cols, headerCol{col: c, label: text})
			}
		}
		return r, cols, nil
	}
	if best == 0 {
		return 0, nil, &NotFoundError{Sheet: s.name, Msg: "no row has any of the header labels " + quoted(t.Header), missing: len(t.Header)}
	}
	var lacks []string
	at := map[string]bool{}
	for _, c := range s.cols[bestRow] {
		at[fold(s.rows[bestRow][c].text)] = true
	}
	for _, l := range t.Header {
		if !at[fold(l)] {
			lacks = append(lacks, l)
		}
	}
	return 0, nil, &NotFoundError{Sheet: s.name, missing: len(lacks),
		Msg: fmt.Sprintf("no row has every header label; the nearest, row %d, lacks %s", bestRow, quoted(lacks))}
}

// guessRows is how many filled rows at the top of a sheet Guess looks at for a header row.
const guessRows = 30

// Guess proposes one table per sheet, for the mapping screen that teaches a layout: the
// header row is taken to be the first of the sheet's first filled rows with the most text
// cells (at least two), and the table's labels are that row's texts found once. A sheet
// without such a row has none. It is only a suggestion the user builds on: a taught profile
// names the labels it reads, and its header row is then found by them.
func (d *Doc) Guess() []Table {
	var out []Table
	for _, s := range d.sheets {
		best, bestRow := 1, 0
		for i, r := range s.rowNums {
			if i == guessRows {
				break
			}
			n := 0
			for _, c := range s.cols[r] {
				if s.rows[r][c].kind == ingest.CellText {
					n++
				}
			}
			if n > best {
				best, bestRow = n, r
			}
		}
		if bestRow == 0 {
			continue
		}
		count := map[string]int{}
		for _, c := range s.cols[bestRow] {
			count[fold(s.rows[bestRow][c].text)]++
		}
		t := Table{Sheet: s.name}
		for _, c := range s.cols[bestRow] {
			if v := s.rows[bestRow][c]; v.kind == ingest.CellText && count[fold(v.text)] == 1 {
				t.Header = append(t.Header, Label(v.text))
			}
		}
		if len(t.Header) >= 2 {
			out = append(out, t)
		}
	}
	return out
}

// quoted writes labels for a message: "Voucher No", "Taxable Value".
func quoted(labels []string) string {
	q := make([]string, len(labels))
	for i, l := range labels {
		q[i] = strconv.Quote(l)
	}
	return strings.Join(q, ", ")
}

// mark records that the cell at row r, column c of s is in a table.
func mark(used map[*sheet]map[int64]bool, s *sheet, r, c int) {
	if used[s] == nil {
		used[s] = map[int64]bool{}
	}
	used[s][cellKey(r, c)] = true
}
