package xlsx

import (
	"slices"
	"testing"

	"github.com/xuri/excelize/v2"
)

// SPEC-011 part 2 (AC-8): a workbook of labelled rows. A blank cell under a blank_as_text
// heading is read as empty text; every other blank stays missing. Rows indented under a heading
// row carry that heading, so two rows labelled "Total" are told apart by condition.
func TestLabelledRowsBlanksAndHeadings(t *testing.T) {
	data := book(t, func(f *excelize.File) {
		if err := f.SetSheetName("Sheet1", "Calc"); err != nil {
			t.Fatal(err)
		}
		set(t, f, "Calc", "A1", "Synthetic Co - computation")
		set(t, f, "Calc", "A3", "Particulars")
		set(t, f, "Calc", "B3", "Value")
		set(t, f, "Calc", "C3", "Note")
		rows := [][]any{
			{"Net Profit", 500},
			{"Add Back"}, // a heading: only its label is filled
			{"   Depreciation", 20},
			{"   Total", 20},
			{"Normal Tax"},
			{"   Tax", 30},
			{"   Total", 30},
			{"Pgbp", 520}, // not indented: ends the block, has no heading
			{"UDIN"},      // a blank value
		}
		for i, r := range rows {
			for j, v := range r {
				ref, _ := excelize.CoordinatesToCellName(j+1, i+4)
				set(t, f, "Calc", ref, v)
			}
		}
	})
	tab := Table{Sheet: "Calc", Header: []string{"Particulars", "Value"}, BlankAsText: []string{"Value"}, Headings: "Heading",
		Labels: []string{"Calc", "Particulars", "Value", "Note"}}
	v, err := open(t, data).View([]Table{tab})
	if err != nil {
		t.Fatal(err)
	}
	if got := selectText(t, v, "$.Calc[*].Particulars"); !slices.Equal(got, []string{
		"text:Net Profit", "text:Add Back", "text:Depreciation", "text:Total", "text:Normal Tax", "text:Tax", "text:Total", "text:Pgbp", "text:UDIN"}) {
		t.Errorf("labels %v: indented labels lose their indent, nothing else changes", got)
	}
	if got := selectText(t, v, "$.Calc[*].Heading"); !slices.Equal(got, []string{"text:Add Back", "text:Add Back", "text:Normal Tax", "text:Normal Tax"}) {
		t.Errorf("headings %v, want each indented row's heading and none for the others", got)
	}
	if got := selectText(t, v, "$.Calc[6].Heading"); !slices.Equal(got, []string{"text:Normal Tax"}) {
		t.Errorf("the second Total's heading %v", got)
	}
	if n, _ := v.CellName("$.Calc[6].Heading"); n != "Calc!A8" {
		t.Errorf("the heading's place %q, want its own cell Calc!A8", n)
	}
	// The blank UDIN is empty text, with its own cell; a heading row's Value is empty text too.
	if got := selectText(t, v, "$.Calc[8].Value"); !slices.Equal(got, []string{"text:"}) {
		t.Errorf("blank UDIN %v, want empty text", got)
	}
	if n, _ := v.CellName("$.Calc[8].Value"); n != "Calc!B12" {
		t.Errorf("blank UDIN's place %q, want Calc!B12", n)
	}
	// Note is not a blank_as_text heading: its empty cells stay missing, never empty text or zero.
	if got := selectText(t, v, "$.Calc[*].Note"); got != nil {
		t.Errorf("Note %v, want nothing", got)
	}

	// Without the options the same sheet reads as before: no Heading, and a blank is missing.
	plain, err := open(t, data).View([]Table{{Sheet: "Calc", Header: tab.Header, Labels: tab.Labels}})
	if err != nil {
		t.Fatal(err)
	}
	if got := selectText(t, plain, "$.Calc[*].Heading"); got != nil {
		t.Errorf("headings without the option: %v", got)
	}
	if got := selectText(t, plain, "$.Calc[8].Value"); got != nil {
		t.Errorf("blank UDIN without the option: %v, want missing", got)
	}
	if got := selectText(t, plain, "$.Calc[2].Particulars"); !slices.Equal(got, []string{"text:   Depreciation"}) {
		t.Errorf("label without the option %v, want it as written", got)
	}
}
