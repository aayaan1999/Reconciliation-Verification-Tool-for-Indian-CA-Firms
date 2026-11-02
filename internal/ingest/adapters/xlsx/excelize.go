package xlsx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"

	"github.com/xuri/excelize/v2"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
)

// FileType is the registry key of the Excel adapter.
const FileType = "xlsx"

// DefaultMaxBytes is the size limit used when none is configured.
const DefaultMaxBytes int64 = 64 << 20

// unzipFactor bounds the unzipped size of a workbook as a multiple of its file size limit,
// so a small file that unzips to gigabytes (a zip bomb) is refused.
const unzipFactor = 4

// maxCells bounds the filled cells of one workbook, so a large workbook cannot exhaust the
// engine's memory: reading costs about 1.6 KB per filled cell (measured 28 October 2026: a
// 200,000-row ledger of 4 columns peaked at 1.3 GB), so a million cells, a 100,000-row
// register of 10 columns, stay under about 1.6 GB.
var maxCells = 1_000_000 // a variable so a test can lower it

var (
	zipMagic = []byte("PK\x03\x04")
	// oleMagic starts an old Office file: an .xls workbook, or an .xlsx with a password,
	// which Excel stores encrypted inside the same container.
	oleMagic = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
	// encryptionInfo is the name, in UTF-16, of the stream an encrypted workbook holds.
	encryptionInfo = utf16("EncryptionInfo")
)

func utf16(s string) []byte {
	b := make([]byte, 0, 2*len(s))
	for i := 0; i < len(s); i++ {
		b = append(b, s[i], 0)
	}
	return b
}

// Adapter reads Excel workbooks. It is safe for concurrent use.
type Adapter struct {
	maxBytes int64
}

// New returns an Excel adapter that refuses files over maxBytes (DefaultMaxBytes when 0 or
// less).
func New(maxBytes int64) *Adapter {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	return &Adapter{maxBytes: maxBytes}
}

// FileType implements ingest.Adapter.
func (a *Adapter) FileType() string { return FileType }

// Sniff reports whether the file is a zip file or an old Office file. Open then tells a
// workbook from another zip file, and an .xls or a password-protected workbook from an
// .xlsx, each with its own message. The name is not used.
func (a *Adapter) Sniff(head []byte, _ string) bool {
	return bytes.HasPrefix(head, zipMagic) || bytes.HasPrefix(head, oleMagic)
}

// A cell is one non-empty value of a sheet.
type cell struct {
	kind ingest.CellKind // text, number or bool
	text string
}

// A sheet is one worksheet: its non-empty cells by row and column (from 1), and what is
// hidden in it. rowNums and cols list the filled rows and each row's filled columns in
// order, so the sheet is walked by its values only: a workbook with one value in A1 and one
// in XFD1048576 is two steps, not seventeen billion.
type sheet struct {
	name       string
	hidden     bool
	rows       map[int]map[int]cell
	rowNums    []int         // filled rows, ascending
	cols       map[int][]int // row → its filled columns, ascending
	hiddenRows map[int]bool
	hiddenCols map[int]bool
}

// Doc is an opened workbook. Its cells are read through View; Select is refused, because a
// value is never addressed by its cell position. It is safe to share: each View is built
// once and kept, so detecting and then reading with one profile reads the tables once.
type Doc struct {
	sheets []*sheet // in workbook order

	mu    sync.Mutex
	views map[string]*View // by the tables read
}

// FileType implements ingest.Doc.
func (d *Doc) FileType() string { return FileType }

// Select implements ingest.Doc. It always fails: a workbook is read through the sheets a
// profile names (View).
func (d *Doc) Select(sel ingest.Selector) ([]ingest.Cell, error) {
	return nil, fmt.Errorf("xlsx: an Excel file is read through the sheets its profile names, never by cell position (%q)", sel.String())
}

// Sheets returns the sheet names in workbook order.
func (d *Doc) Sheets() []string {
	out := make([]string, len(d.sheets))
	for i, s := range d.sheets {
		out[i] = s.name
	}
	return out
}

// Leaves implements ingest.LeafLister: every non-empty cell of every sheet, sheet by sheet,
// row by row, with its sheet and cell ("Sales Register!C14").
func (d *Doc) Leaves() []ingest.Cell {
	var out []ingest.Cell
	for _, s := range d.sheets {
		for _, r := range s.rowNums {
			for _, c := range s.cols[r] {
				out = append(out, s.leaf(r, c, s.rows[r][c]))
			}
		}
	}
	return out
}

func (s *sheet) leaf(r, c int, v cell) ingest.Cell {
	return ingest.Cell{Kind: v.kind, Text: v.text, Locator: domain.Locator{Kind: domain.LocatorSheetCell, Path: cellName(s.name, c, r)}}
}

// cellName is "Sheet!C14".
func cellName(sheetName string, col, row int) string {
	ref, _ := excelize.CoordinatesToCellName(col, row)
	return sheetName + "!" + ref
}

// Open reads the whole workbook: every sheet, every non-empty cell. Errors are
// *ingest.Error with code FILE_TOO_LARGE, UNSUPPORTED_FILE_TYPE (an .xls file or a
// password-protected workbook) or PARSE_ERROR.
func (a *Adapter) Open(r io.ReaderAt, size int64) (ingest.Doc, error) {
	switch {
	case size < 0:
		return nil, &ingest.Error{Code: ingest.ParseError, Msg: fmt.Sprintf("invalid file size %d", size)}
	case size > a.maxBytes:
		return nil, &ingest.Error{Code: ingest.FileTooLarge, Msg: fmt.Sprintf("the file is %d bytes; Excel files may be at most %d bytes", size, a.maxBytes)}
	}
	data, err := io.ReadAll(io.NewSectionReader(r, 0, size))
	if err != nil {
		return nil, &ingest.Error{Code: ingest.ParseError, Msg: "the file could not be read: " + err.Error()}
	}
	if bytes.HasPrefix(data, oleMagic) {
		if bytes.Contains(data, encryptionInfo) {
			return nil, &ingest.Error{Code: ingest.UnsupportedFileType,
				Msg: "the workbook is protected with a password; open it in Excel, remove the password and save it again as .xlsx"}
		}
		return nil, &ingest.Error{Code: ingest.UnsupportedFileType,
			Msg: "this is an old Excel file (.xls, Excel 97-2003) or another old Office file; open it in Excel and save it as .xlsx or .csv"}
	}
	limit := unzipFactor * a.maxBytes
	if err := checkZip(data, limit); err != nil {
		return nil, err
	}
	// The unzip limits are equal so that excelize never spills a sheet to a temporary file:
	// client figures stay in memory and in the database only.
	f, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{RawCellValue: true, UnzipSizeLimit: limit, UnzipXMLSizeLimit: limit})
	if err != nil {
		// The library's own words can quote the file's contents, so they are not passed on.
		return nil, &ingest.Error{Code: ingest.ParseError, Msg: "the workbook cannot be read: its contents are damaged or not those of an Excel workbook"}
	}
	defer f.Close()

	date1904 := false
	if props, err := f.GetWorkbookProps(); err == nil && props.Date1904 != nil {
		date1904 = *props.Date1904
	}
	styles := map[int]bool{} // style ID → shows a date
	// Each sheet's cell types, styles and hidden rows and columns come from one pass over its
	// XML; a workbook whose parts cannot be found that way is asked of excelize cell by cell.
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, &ingest.Error{Code: ingest.ParseError, Msg: "the workbook cannot be read: its contents are damaged or not those of an Excel workbook"}
	}
	parts, err := sheetParts(zr)
	if err != nil {
		parts = nil
	}
	d := &Doc{}
	cells := 0
	for _, name := range f.GetSheetList() {
		var attrs *sheetAttrs
		if part, ok := parts[name]; ok {
			if attrs, err = readAttrs(zr, part); err != nil {
				return nil, &ingest.Error{Code: ingest.ParseError, Msg: fmt.Sprintf("sheet %q cannot be read: its contents are damaged", name)}
			}
		}
		s, err := readSheet(f, name, attrs, date1904, styles, &cells)
		if err != nil {
			return nil, err
		}
		d.sheets = append(d.sheets, s)
	}
	return d, nil
}

// checkZip refuses a zip file that is not a workbook, or that would unzip to more than
// limit bytes by its own headers (excelize enforces the limit again as it unzips).
func checkZip(data []byte, limit int64) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return &ingest.Error{Code: ingest.ParseError, Msg: "the file is not a valid Excel workbook: " + err.Error()}
	}
	var total uint64
	workbook := false
	for _, zf := range zr.File {
		total += zf.UncompressedSize64
		workbook = workbook || zf.Name == "xl/workbook.xml"
	}
	switch {
	case !workbook:
		return &ingest.Error{Code: ingest.UnsupportedFileType, Msg: "the file is a zip file but not an Excel workbook"}
	case total > uint64(limit):
		return &ingest.Error{Code: ingest.FileTooLarge, Msg: fmt.Sprintf("the workbook unzips to %d bytes; at most %d bytes are read", total, limit)}
	}
	return nil
}

// readSheet reads one sheet, adding its filled cells to *cells, the count of the workbook.
// The values come from excelize's row reader; each cell's kind and style, and what is
// hidden, from attrs, or from excelize when attrs is nil.
func readSheet(f *excelize.File, name string, attrs *sheetAttrs, date1904 bool, styles map[int]bool, cells *int) (*sheet, error) {
	fail := func(error) error {
		// The library's own words can quote a cell's value, so they are not passed on.
		return &ingest.Error{Code: ingest.ParseError, Msg: fmt.Sprintf("sheet %q cannot be read: its contents are damaged", name)}
	}
	visible, err := f.GetSheetVisible(name)
	if err != nil {
		return nil, fail(err)
	}
	s := &sheet{name: name, hidden: !visible, rows: map[int]map[int]cell{}, cols: map[int][]int{}, hiddenRows: map[int]bool{}, hiddenCols: map[int]bool{}}
	rows, err := f.Rows(name)
	if err != nil {
		return nil, fail(err)
	}
	defer rows.Close()
	for r := 1; rows.Next(); r++ {
		cols, err := rows.Columns(excelize.Options{RawCellValue: true})
		if err != nil {
			return nil, fail(err)
		}
		for i, raw := range cols {
			if raw == "" {
				continue
			}
			if *cells++; *cells > maxCells {
				return nil, &ingest.Error{Code: ingest.FileTooLarge,
					Msg: fmt.Sprintf("the workbook has more than %d filled cells; split it or save the sheet the tool reads as .csv", maxCells)}
			}
			c := i + 1
			at, err := cellOf(f, name, attrs, r, c)
			if err != nil {
				return nil, fail(err)
			}
			v, err := readCell(f, at, raw, date1904, styles)
			if err != nil {
				return nil, fail(err)
			}
			if s.rows[r] == nil {
				s.rows[r] = map[int]cell{}
				s.rowNums = append(s.rowNums, r)
			}
			s.rows[r][c] = v
			s.cols[r] = append(s.cols[r], c)
		}
	}
	if err := rows.Error(); err != nil {
		return nil, fail(err)
	}
	var used []int // every filled column of the sheet
	for _, r := range s.rowNums {
		if attrs != nil {
			if attrs.hiddenRows[r] {
				s.hiddenRows[r] = true
			}
		} else if vis, err := f.GetRowVisible(name, r); err == nil && !vis {
			s.hiddenRows[r] = true
		}
		used = append(used, s.cols[r]...)
	}
	slices.Sort(used)
	for _, c := range slices.Compact(used) {
		if attrs != nil {
			if attrs.colHidden(c) {
				s.hiddenCols[c] = true
			}
			continue
		}
		col, _ := excelize.ColumnNumberToName(c)
		if vis, err := f.GetColVisible(name, col); err == nil && !vis {
			s.hiddenCols[c] = true
		}
	}
	return s, nil
}

// cellOf returns the kind and style of the cell at row r, column c: from attrs, or asked of
// excelize when attrs is nil.
func cellOf(f *excelize.File, sheetName string, attrs *sheetAttrs, r, c int) (cellAttrs, error) {
	if attrs != nil {
		if at, ok := attrs.cells[cellKey(r, c)]; ok {
			return at, nil
		}
		// The row reader has a value the pass did not see: ask excelize, never guess.
	}
	ref, _ := excelize.CoordinatesToCellName(c, r)
	ct, err := f.GetCellType(sheetName, ref)
	if err != nil {
		return cellAttrs{}, err
	}
	at := cellAttrs{kind: kindText}
	switch ct {
	case excelize.CellTypeBool:
		at.kind = kindBool
	case excelize.CellTypeDate:
		at.kind = kindDate
	case excelize.CellTypeUnset, excelize.CellTypeNumber:
		at.kind = kindNumber
	}
	if at.style, err = f.GetCellStyle(sheetName, ref); err != nil {
		return cellAttrs{}, err
	}
	return at, nil
}

// readCell turns one stored value into a cell: text as it is, a number as exact decimal
// text, a date as YYYY-MM-DD, a bool as true or false.
func readCell(f *excelize.File, at cellAttrs, raw string, date1904 bool, styles map[int]bool) (cell, error) {
	switch at.kind {
	case kindBool:
		text := "false"
		if raw == "1" || strings.EqualFold(raw, "true") {
			text = "true"
		}
		return cell{kind: ingest.CellBool, text: text}, nil
	case kindDate: // stored as ISO 8601 text, such as 2026-08-03T00:00:00Z
		if len(raw) >= 10 && validISODate(raw[:10]) {
			return cell{kind: ingest.CellText, text: raw[:10]}, nil
		}
		return cell{kind: ingest.CellText, text: raw}, nil
	case kindNumber:
		num, ok := plainDecimal(raw)
		if !ok {
			return cell{kind: ingest.CellText, text: raw}, nil
		}
		isDate, err := dateStyle(f, at.style, styles)
		if err != nil {
			return cell{}, err
		}
		if isDate {
			if day, ok := serialDate(num, date1904); ok {
				return cell{kind: ingest.CellText, text: day}, nil
			}
		}
		return cell{kind: ingest.CellNumber, text: num}, nil
	}
	// Shared and inline strings, a formula's text result, and an error value (#DIV/0!),
	// which then fails as a figure rather than being read as one.
	return cell{kind: ingest.CellText, text: raw}, nil
}

// dateStyle reports whether the number format of style id shows a date.
func dateStyle(f *excelize.File, id int, styles map[int]bool) (bool, error) {
	if v, ok := styles[id]; ok {
		return v, nil
	}
	st, err := f.GetStyle(id)
	if err != nil {
		return false, err
	}
	custom := ""
	if st.CustomNumFmt != nil {
		custom = *st.CustomNumFmt
	}
	v := isDateFormat(st.NumFmt, custom)
	styles[id] = v
	return v, nil
}
