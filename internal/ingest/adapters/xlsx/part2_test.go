package xlsx

import (
	"bytes"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
)

// SPEC-008 part 2: what real exports do, found by the manual test of 28 October 2026.

// A real export: the sheet called "Sheet1", headers in other capitals, an empty row inside
// the data and a Grand Total row under it. Every invoice is read, the total is left out
// with why, columns are keyed by the profile's spelling and cells keep the real sheet name.
func TestRealExportShape(t *testing.T) {
	data := book(t, func(f *excelize.File) {
		set(t, f, "Sheet1", "A1", "Synthetic Traders - sales register")
		for i, h := range []string{"VOUCHER NO", "taxable value", "Igst"} {
			ref, _ := excelize.CoordinatesToCellName(i+1, 3)
			set(t, f, "Sheet1", ref, h)
		}
		set(t, f, "Sheet1", "A4", "INV-1")
		set(t, f, "Sheet1", "B4", 100)
		set(t, f, "Sheet1", "A6", "INV-2") // row 5 is empty
		set(t, f, "Sheet1", "B6", 200)
		set(t, f, "Sheet1", "A7", "Grand Total")
		set(t, f, "Sheet1", "B7", 300)
	})
	tab := Table{Sheet: "Register", Header: []string{"Voucher No", "Taxable Value"}, AnyName: true,
		Totals: []string{"Total", "Grand Total"}, Labels: []string{"Register", "Voucher No", "Taxable Value", "IGST"}}
	v, err := open(t, data).View([]Table{tab})
	if err != nil {
		t.Fatal(err)
	}
	if got := selectText(t, v, "$.Register[*]['Voucher No']"); !slices.Equal(got, []string{"text:INV-1", "text:INV-2"}) {
		t.Errorf("vouchers %v, want both invoices and not the total", got)
	}
	if got := selectText(t, v, "$.Register[1].IGST"); got != nil {
		t.Errorf("IGST of INV-2 %v, want nothing (empty)", got)
	}
	if n, _ := v.CellName("$.Register[1]['Taxable Value']"); n != "Sheet1!B6" {
		t.Errorf("cell %q, want Sheet1!B6 (the real sheet name)", n)
	}
	if row, why, ok := v.LeftOut("Sheet1!B7"); !ok || row != "Sheet1!7:7" || !strings.Contains(why, `a total row ("Grand Total" under "Voucher No")`) {
		t.Errorf("total row: %q %q %v", row, why, ok)
	}
	notes := v.Notes()
	if len(notes) != 1 || notes[0].Code != ingest.EmptyRows || !strings.Contains(notes[0].Msg, "row 5 is empty") {
		t.Errorf("notes %v, want EMPTY_ROWS for row 5", notes)
	}

	// Without AnyName the sheet must be named; two sheets with the header row are not guessed.
	if _, err := open(t, data).View([]Table{{Sheet: "Register", Header: tab.Header}}); !isNotFound(err) {
		t.Errorf("a named sheet that is not there: %v", err)
	}
	two := book(t, func(f *excelize.File) {
		for _, s := range []string{"Sheet1", "Copy"} {
			if s != "Sheet1" {
				if _, err := f.NewSheet(s); err != nil {
					t.Fatal(err)
				}
			}
			set(t, f, s, "A1", "Voucher No")
			set(t, f, s, "B1", "Taxable Value")
			set(t, f, s, "A2", "INV-1")
		}
	})
	_, err = open(t, two).View([]Table{tab})
	var ie *ingest.Error
	if !errors.As(err, &ie) || !strings.Contains(ie.Msg, `sheets "Sheet1" and "Copy" both have a header row`) {
		t.Errorf("two sheets with the header: %v", err)
	}
}

func isNotFound(err error) bool {
	var nf *NotFoundError
	return errors.As(err, &nf)
}

// A missing table says what is missing: the nearest row and the label it lacks.
func TestNotFoundSaysWhatIsMissing(t *testing.T) {
	data := book(t, func(f *excelize.File) {
		register(t, f, []string{"Voucher No", "Taxable Amt"})
		set(t, f, "Register", "B4", "INV-1")
	})
	_, err := open(t, data).View([]Table{{Sheet: "Register", Header: []string{"Voucher No", "Taxable Value"}}})
	if !isNotFound(err) || !strings.Contains(err.Error(), `the nearest, row 3, lacks "Taxable Value"`) {
		t.Errorf("error %v", err)
	}
}

// The one pass over a sheet's XML gives the same cells as asking excelize cell by cell.
func TestAttributePassMatchesExcelize(t *testing.T) {
	data := book(t, func(f *excelize.File) {
		register(t, f, []string{"Voucher No", "Date", "Amount", "Paid"})
		set(t, f, "Register", "B4", "INV-1")
		day, _ := f.NewStyle(&excelize.Style{NumFmt: 14})
		set(t, f, "Register", "C4", 46237)
		if err := f.SetCellStyle("Register", "C4", "C4", day); err != nil {
			t.Fatal(err)
		}
		set(t, f, "Register", "D4", 1250.75)
		set(t, f, "Register", "E4", true)
		if err := f.SetRowVisible("Register", 4, false); err != nil {
			t.Fatal(err)
		}
		if err := f.SetColVisible("Register", "D", false); err != nil {
			t.Fatal(err)
		}
	})
	f, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{RawCellValue: true})
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d := open(t, data)
	n := 0
	slow, err := readSheet(f, "Register", nil, false, map[int]bool{}, &n)
	if err != nil {
		t.Fatal(err)
	}
	fast := d.sheets[0]
	for _, c := range []struct {
		name string
		a, b any
	}{{"cells", fast.rows, slow.rows}, {"hidden rows", fast.hiddenRows, slow.hiddenRows}, {"hidden columns", fast.hiddenCols, slow.hiddenCols}} {
		if !reflect.DeepEqual(c.a, c.b) {
			t.Errorf("%s: pass %v, excelize %v", c.name, c.a, c.b)
		}
	}
	if !fast.hiddenRows[4] || !fast.hiddenCols[4] || fast.rows[4][3].text != "2026-08-03" || fast.rows[4][5].kind != ingest.CellBool {
		t.Errorf("sheet read as %+v", fast)
	}
}

// Guess proposes a header row per sheet for the mapping screen, skipping a title row.
func TestGuess(t *testing.T) {
	data := book(t, func(f *excelize.File) {
		register(t, f, []string{"Voucher No", "Party", "Amount"})
		set(t, f, "Register", "B4", "INV-1")
		set(t, f, "Register", "D4", 10)
		if _, err := f.NewSheet("Notes"); err != nil {
			t.Fatal(err)
		}
		set(t, f, "Notes", "A1", "only one cell")
	})
	got := open(t, data).Guess()
	if len(got) != 1 || got[0].Sheet != "Register" || !slices.Equal(got[0].Header, []string{"Voucher No", "Party", "Amount"}) {
		t.Errorf("guess %+v", got)
	}
}
