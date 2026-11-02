package xlsx

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	jsonadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/json"
)

func TestPlainDecimal(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"325000", "325000"}, {"0.1", "0.1"}, {"112450.5", "112450.5"}, {"-12.50", "-12.5"},
		{"1.2345E5", "123450"}, {"1.2345e+5", "123450"}, {"-1.5e-3", "-0.0015"}, {"5E-1", "0.5"},
		{"007", "7"}, {"0", "0"}, {"-0", "0"}, {"0.30000000000000004", "0.30000000000000004"}, {"+3", "3"},
	} {
		got, ok := plainDecimal(c.in)
		if !ok || got != c.want {
			t.Errorf("plainDecimal(%q) = %q, %v; want %q", c.in, got, ok, c.want)
		}
	}
	for _, bad := range []string{"", "-", "1.2.3", "1E", "1E999", "12a", "#DIV/0!", "1,000", "Rs 5"} {
		if got, ok := plainDecimal(bad); ok {
			t.Errorf("plainDecimal(%q) = %q, want not a number", bad, got)
		}
	}
}

func TestSerialDate(t *testing.T) {
	for _, c := range []struct {
		num      string
		date1904 bool
		want     string
	}{
		{"1", false, "1900-01-01"}, {"59", false, "1900-02-28"}, {"61", false, "1900-03-01"},
		{"45292", false, "2024-01-01"}, {"45383", false, "2024-04-01"}, {"46237", false, "2026-08-03"},
		{"0", true, "1904-01-01"}, {"1", true, "1904-01-02"}, {"43821", true, "2023-12-23"},
	} {
		got, ok := serialDate(c.num, c.date1904)
		if !ok || got != c.want {
			t.Errorf("serialDate(%q, 1904=%v) = %q, %v; want %q", c.num, c.date1904, got, ok, c.want)
		}
	}
	for _, bad := range []string{"60", "0", "45383.5", "-1", "99999999"} {
		if got, ok := serialDate(bad, false); ok {
			t.Errorf("serialDate(%q) = %q, want no date", bad, got)
		}
	}
}

func TestIsDateFormat(t *testing.T) {
	for _, c := range []struct {
		id     int
		custom string
		want   bool
	}{
		{14, "", true}, {22, "", true}, {0, "", false}, {2, "", false}, {20, "", false},
		{164, "dd-mm-yyyy", true}, {164, "d mmm yy", true}, {164, "#,##0.00", false},
		{164, `"Day" 0`, false}, {164, "[Red]0.00", false}, {164, `0\d`, false}, {164, "mm:ss", false},
	} {
		if got := isDateFormat(c.id, c.custom); got != c.want {
			t.Errorf("isDateFormat(%d, %q) = %v, want %v", c.id, c.custom, got, c.want)
		}
	}
}

// book builds a workbook in memory with fn and returns its bytes.
func book(t *testing.T, fn func(f *excelize.File)) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	fn(f)
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func open(t *testing.T, data []byte) *Doc {
	t.Helper()
	a := New(0)
	if !a.Sniff(data[:min(len(data), 512)], "x.xlsx") {
		t.Fatal("Sniff does not recognise the workbook")
	}
	doc, err := a.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	return doc.(*Doc)
}

func set(t *testing.T, f *excelize.File, sheet, ref string, v any) {
	t.Helper()
	if err := f.SetCellValue(sheet, ref, v); err != nil {
		t.Fatal(err)
	}
}

// register is a sheet with a title row, a blank row and column A empty, and the header in
// row 3 from column B.
func register(t *testing.T, f *excelize.File, header []string) {
	t.Helper()
	if err := f.SetSheetName("Sheet1", "Register"); err != nil {
		t.Fatal(err)
	}
	set(t, f, "Register", "B1", "Sales register - synthetic")
	for i, h := range header {
		ref, _ := excelize.CoordinatesToCellName(i+2, 3)
		set(t, f, "Register", ref, h)
	}
}

func selectText(t *testing.T, v *View, path string) []string {
	t.Helper()
	sel, err := jsonadapter.ParsePath(path)
	if err != nil {
		t.Fatal(err)
	}
	cells, err := v.Select(sel)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, c := range cells {
		out = append(out, string(c.Kind)+":"+c.Text)
	}
	return out
}

func TestViewReadsByHeaderText(t *testing.T) {
	data := book(t, func(f *excelize.File) {
		register(t, f, []string{"Voucher No", "  Taxable\nValue ", "Date", "Exempt", "Remarks"})
		set(t, f, "Register", "B4", "INV-1")
		set(t, f, "Register", "C4", 325000.5)
		day, _ := f.NewStyle(&excelize.Style{NumFmt: 14})
		set(t, f, "Register", "D4", 46237)
		if err := f.SetCellStyle("Register", "D4", "D4", day); err != nil {
			t.Fatal(err)
		}
		set(t, f, "Register", "E4", true)
		set(t, f, "Register", "B5", "INV-2")
		if err := f.SetCellFormula("Register", "C5", "C4*2"); err != nil { // never calculated: no saved value
			t.Fatal(err)
		}
		set(t, f, "Register", "B7", "after the blank row")
		set(t, f, "Register", "H4", "no header above")
	})
	d := open(t, data)
	v, err := d.View([]Table{{Sheet: "Register", Header: []string{"Voucher No", "Taxable Value"}}})
	if err != nil {
		t.Fatal(err)
	}
	// An empty row inside the data does not end the table (SPEC-008 part 2): the row below
	// it is read too, and the report says where the empty row is.
	if got := selectText(t, v, "$.Register[*]['Voucher No']"); !slices.Equal(got, []string{"text:INV-1", "text:INV-2", "text:after the blank row"}) {
		t.Errorf("voucher numbers %v", got)
	}
	if notes := v.Notes(); len(notes) != 1 || notes[0].Code != ingest.EmptyRows || !strings.Contains(notes[0].Msg, "row 6 is empty") {
		t.Errorf("notes %v, want one EMPTY_ROWS note for row 6", notes)
	}
	if got := selectText(t, v, "$.Register[0]['Taxable Value']"); !slices.Equal(got, []string{"number:325000.5"}) {
		t.Errorf("taxable value %v, want the exact number", got)
	}
	if got := selectText(t, v, "$.Register[1]['Taxable Value']"); len(got) != 0 {
		t.Errorf("a formula with no saved value gave %v, want nothing (missing)", got)
	}
	if got := selectText(t, v, "$.Register[0].Date"); !slices.Equal(got, []string{"text:2026-08-03"}) {
		t.Errorf("date %v", got)
	}
	if got := selectText(t, v, "$.Register[0].Exempt"); !slices.Equal(got, []string{"bool:true"}) {
		t.Errorf("bool %v", got)
	}
	if n, _ := v.CellName("$.Register[0]['Taxable Value']"); n != "Register!C4" {
		t.Errorf("cell name %q, want Register!C4", n)
	}
	if n, _ := v.CellName("$.Register[1]"); n != "Register!5:5" {
		t.Errorf("row name %q, want Register!5:5", n)
	}
	var rest []string
	for _, l := range v.Leaves()[len(v.Doc.Leaves()):] {
		rest = append(rest, l.Locator.Path)
	}
	want := []string{"Register!B1", "Register!H4"}
	if !slices.Equal(rest, want) {
		t.Errorf("cells outside the table %v, want %v", rest, want)
	}
}

func TestMovedColumnReadsTheSame(t *testing.T) {
	read := func(header []string, values []any) []string {
		data := book(t, func(f *excelize.File) {
			register(t, f, header)
			for i, val := range values {
				ref, _ := excelize.CoordinatesToCellName(i+2, 4)
				set(t, f, "Register", ref, val)
			}
		})
		v, err := open(t, data).View([]Table{{Sheet: "Register", Header: []string{"Voucher No", "Amount"}}})
		if err != nil {
			t.Fatal(err)
		}
		return append(selectText(t, v, "$.Register[0]['Voucher No']"), selectText(t, v, "$.Register[0].Amount")...)
	}
	a := read([]string{"Voucher No", "Amount"}, []any{"INV-9", 1500})
	b := read([]string{"Extra", "Amount", "Note", "Voucher No"}, []any{"x", 1500, "y", "INV-9"})
	if !slices.Equal(a, b) || len(a) != 2 {
		t.Errorf("moving columns changed what is read: %v and %v", a, b)
	}
}

func TestHeaderProblems(t *testing.T) {
	data := book(t, func(f *excelize.File) {
		register(t, f, []string{"Voucher No", "Amount", "Amount", "Note", "Note"})
		set(t, f, "Register", "B4", "INV-1")
	})
	d := open(t, data)
	_, err := d.View([]Table{{Sheet: "Register", Header: []string{"Voucher No", "Amount"}}})
	var ie *ingest.Error
	if !errors.As(err, &ie) || !strings.Contains(ie.Msg, `"Amount" twice, at C3 and D3`) {
		t.Errorf("a named label found twice: %v", err)
	}
	// A repeated label the profile does not name is left out of the table, not an error.
	v, err := d.View([]Table{{Sheet: "Register", Header: []string{"Voucher No"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := selectText(t, v, "$.Register[0].Note"); len(got) != 0 {
		t.Errorf("a repeated label was read: %v", got)
	}
	var nf *NotFoundError
	if _, err := d.View([]Table{{Sheet: "Register", Header: []string{"Voucher No", "GSTIN"}}}); !errors.As(err, &nf) {
		t.Errorf("a label not in the sheet: %v, want NotFoundError", err)
	}
	if _, err := d.View([]Table{{Sheet: "Purchases", Header: []string{"Voucher No"}}}); !errors.As(err, &nf) {
		t.Errorf("a sheet not in the workbook: %v, want NotFoundError", err)
	}
}

func TestHiddenAndMerged(t *testing.T) {
	data := book(t, func(f *excelize.File) {
		register(t, f, []string{"Voucher No", "Amount", "Note"})
		set(t, f, "Register", "B4", "INV-1")
		set(t, f, "Register", "C4", 10)
		set(t, f, "Register", "B5", "INV-2")
		set(t, f, "Register", "C5", 20)
		set(t, f, "Register", "D4", "merged")
		if err := f.MergeCell("Register", "D4", "D5"); err != nil {
			t.Fatal(err)
		}
		if err := f.SetRowVisible("Register", 5, false); err != nil {
			t.Fatal(err)
		}
		if err := f.SetColVisible("Register", "C", false); err != nil {
			t.Fatal(err)
		}
		if _, err := f.NewSheet("Other"); err != nil {
			t.Fatal(err)
		}
		set(t, f, "Other", "A1", "Key")
		set(t, f, "Other", "A2", "v")
		if err := f.SetSheetVisible("Other", false); err != nil {
			t.Fatal(err)
		}
	})
	d := open(t, data)
	v, err := d.View([]Table{{Sheet: "Register", Header: []string{"Voucher No"}}, {Sheet: "Other", Header: []string{"Key"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := selectText(t, v, "$.Register[*].Note"); !slices.Equal(got, []string{"text:merged"}) {
		t.Errorf("a merged cell gave %v, want its value once", got)
	}
	if got := selectText(t, v, "$.Register[*].Amount"); !slices.Equal(got, []string{"number:10", "number:20"}) {
		t.Errorf("hidden row and column: %v", got)
	}
	var msgs []string
	for _, n := range v.Notes() {
		msgs = append(msgs, string(n.Code)+": "+n.Msg)
	}
	notes := strings.Join(msgs, "\n")
	for _, want := range []string{`row 5 of sheet "Register" is hidden`, `column C ("Amount") of sheet "Register" is hidden`, `sheet "Other" is hidden`} {
		if !strings.Contains(notes, want) {
			t.Errorf("notes do not say %q:\n%s", want, notes)
		}
	}
}

func TestDate1904(t *testing.T) {
	data := book(t, func(f *excelize.File) {
		yes := true
		if err := f.SetWorkbookProps(&excelize.WorkbookPropsOptions{Date1904: &yes}); err != nil {
			t.Fatal(err)
		}
		register(t, f, []string{"Date"})
		day, _ := f.NewStyle(&excelize.Style{CustomNumFmt: ptr("dd-mm-yyyy")})
		set(t, f, "Register", "B4", 43821)
		if err := f.SetCellStyle("Register", "B4", "B4", day); err != nil {
			t.Fatal(err)
		}
	})
	v, err := open(t, data).View([]Table{{Sheet: "Register", Header: []string{"Date"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := selectText(t, v, "$.Register[0].Date"); !slices.Equal(got, []string{"text:2023-12-23"}) {
		t.Errorf("1904 date %v", got)
	}
}

func ptr(s string) *string { return &s }

func TestRefusals(t *testing.T) {
	a := New(0)
	openErr := func(data []byte) *ingest.Error {
		t.Helper()
		_, err := a.Open(bytes.NewReader(data), int64(len(data)))
		var ie *ingest.Error
		if !errors.As(err, &ie) {
			t.Fatalf("error %v, want *ingest.Error", err)
		}
		return ie
	}
	ole := append(slices.Clone(oleMagic), make([]byte, 600)...)
	if e := openErr(ole); e.Code != ingest.UnsupportedFileType || !strings.Contains(e.Msg, "save it as .xlsx or .csv") {
		t.Errorf("an .xls file: %v", e)
	}
	locked := append(slices.Clone(ole), encryptionInfo...)
	if e := openErr(locked); e.Code != ingest.UnsupportedFileType || !strings.Contains(e.Msg, "password") {
		t.Errorf("a password-protected workbook: %v", e)
	}
	var zb bytes.Buffer
	zw := zip.NewWriter(&zb)
	w, _ := zw.Create("word/document.xml")
	w.Write([]byte("<doc/>"))
	zw.Close()
	if e := openErr(zb.Bytes()); e.Code != ingest.UnsupportedFileType || !strings.Contains(e.Msg, "not an Excel workbook") {
		t.Errorf("a zip that is not a workbook: %v", e)
	}
	small := New(100)
	data := book(t, func(f *excelize.File) { set(t, f, "Sheet1", "A1", "x") })
	if _, err := small.Open(bytes.NewReader(data), int64(len(data))); !strings.Contains(err.Error(), "at most 100 bytes") {
		t.Errorf("a file over the limit: %v", err)
	}
	if err := checkZip(data, 10); err == nil || !strings.Contains(err.Error(), "unzips to") {
		t.Errorf("a workbook that unzips over the limit: %v", err)
	}
	if a.Sniff([]byte(`{"a":1}`), "x.xlsx") || a.Sniff([]byte("a,b\n1,2\n"), "x.xlsx") {
		t.Error("Sniff accepts JSON or CSV named .xlsx")
	}
	if _, err := open(t, data).Select(jsonadapter.Path{}); err == nil {
		t.Error("a workbook can be read by position")
	}
}

// SPEC-008 AC-2: no floating point in the money or date path of the adapter.
func TestNoFloatingPoint(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, banned := range []string{"ParseFloat", "ExcelDateToTime", "float64", "float32", "GetCellValue("} {
			if bytes.Contains(b, []byte(banned)) {
				t.Errorf("%s uses %s", f, banned)
			}
		}
	}
}

// A sparse workbook is walked by its values only: one value in A1 and one in XFD1048576
// is two cells to visit, not seventeen billion (code review, 28 October 2026).
func TestSparseWorkbook(t *testing.T) {
	data := book(t, func(f *excelize.File) {
		set(t, f, "Sheet1", "A1", "Key")
		set(t, f, "Sheet1", "A2", "v")
		set(t, f, "Sheet1", "XFD1048576", "far away")
	})
	d := open(t, data)
	if got := len(d.Leaves()); got != 3 {
		t.Errorf("%d cells, want 3", got)
	}
	v, err := d.View([]Table{{Sheet: "Sheet1", Header: []string{"Key"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := selectText(t, v, "$.Sheet1[*].Key"); !slices.Equal(got, []string{"text:v"}) {
		t.Errorf("rows %v", got)
	}
	if _, err := d.View([]Table{{Sheet: "Sheet1", Header: []string{"Missing"}}}); err == nil {
		t.Error("a header not in the sheet was found")
	}
}

// A View is built once per set of tables and kept (code review, 28 October 2026).
func TestViewIsKept(t *testing.T) {
	d := open(t, book(t, func(f *excelize.File) { set(t, f, "Sheet1", "A1", "Key") }))
	tables := []Table{{Sheet: "Sheet1", Header: []string{"Key"}}}
	a, err := d.View(tables)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := d.View(tables)
	c, _ := d.View([]Table{{Sheet: "Sheet1", Header: []string{"Key"}}})
	if a != b || a != c {
		t.Error("the same tables gave a new View")
	}
}

// A workbook with more filled cells than the limit is refused, counted across its sheets,
// and the library's own error words (which can quote a cell) are not passed on.
func TestCellLimitAndPlainErrors(t *testing.T) {
	defer func(n int) { maxCells = n }(maxCells)
	maxCells = 3
	data := book(t, func(f *excelize.File) {
		set(t, f, "Sheet1", "A1", "a")
		set(t, f, "Sheet1", "B1", "b")
		if _, err := f.NewSheet("Two"); err != nil {
			t.Fatal(err)
		}
		set(t, f, "Two", "A1", "c")
		set(t, f, "Two", "A2", "d")
	})
	_, err := New(0).Open(bytes.NewReader(data), int64(len(data)))
	var ie *ingest.Error
	if !errors.As(err, &ie) || ie.Code != ingest.FileTooLarge || !strings.Contains(ie.Msg, "more than 3 filled cells") {
		t.Errorf("a workbook over the cell limit: %v", err)
	}
	maxCells = 4
	if _, err := New(0).Open(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Errorf("a workbook at the cell limit: %v", err)
	}

	// A workbook part that is not XML: the message says so without the library's text.
	var zb bytes.Buffer
	zw := zip.NewWriter(&zb)
	for _, name := range []string{"[Content_Types].xml", "xl/workbook.xml"} {
		w, _ := zw.Create(name)
		w.Write([]byte("SECRET-CELL-VALUE <<<"))
	}
	zw.Close()
	_, err = New(0).Open(bytes.NewReader(zb.Bytes()), int64(zb.Len()))
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Errorf("a damaged workbook: %v, want an error that quotes nothing of the file", err)
	}
}
