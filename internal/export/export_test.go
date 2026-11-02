package export

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

// The figures in these tests are illustrative: a fictional client, invented invoice numbers and
// paise amounts.

const testResult = `{
 "client": {"name": "Nimbus Test Co"},
 "tax_year": "2025-26",
 "report": {
  "findings": [{"review_document": "sales_register", "heading": "Sales register vs GSTR-1", "findings": [
    {"finding_id": "F1", "rule_id": "RECON-X-001", "title": "Sales differ", "code_text": "Amount differs",
     "subject": {"period": "2025-26", "scope": "", "match_key": "INV-1"},
     "values": [
       {"label": "Sales register", "value": {"kind": "money", "paise": 12345678}, "locator": {"file_name": "sales.json", "path": "$.rows[1]"}},
       {"label": "GSTR-1", "value": {"kind": "money", "paise": 12345000}, "locator": {"file_name": "gstr1.json", "path": "$.b2b[0]"}}],
     "difference_paise": 678, "show_difference": true,
     "law_refs": [{"act": "CGST Act", "section": "37", "reference": "s.37"}]},
    {"finding_id": "F2", "rule_id": "RECON-X-002", "title": "Second difference", "code_text": "",
     "subject": {"period": "2025-26", "scope": "", "match_key": "INV-2"},
     "values": [{"label": "Sales register", "value": {"kind": "money", "paise": 500}, "locator": {"file_name": "sales.json", "path": "$.rows[2]"}}],
     "difference_paise": 0, "show_difference": false, "law_refs": []}
  ]}],
  "cannot_check": [{"rule_id": "RECON-Y-001", "title": "Needs the tax audit report", "category": "R1",
    "subject": {"period": "2025-26", "scope": "", "match_key": ""},
    "missing": [{"fact": "tar.depreciation.total", "label": "Depreciation", "document_type": "tax_audit_report", "document": "Tax audit report", "rows": false}]}]
 },
 "results": [],
 "decisions": {"F1": {"state": "fix", "current": {"decision": "fix", "note": "", "decided_by": "A. Staff", "decided_at": "2026-10-29T09:00:00Z", "on_other_values": false}}}
}`

const testWorklist = `{"tax_year": "2025-26", "items": [
 {"client_id": 1, "key": "new_head|Cloud hosting", "kind_text": "New head", "ledger": "Cloud hosting", "group": "Technology",
  "text": "New head", "prior_total": 0, "current_total": 115321600, "missing_months": [], "classification": null},
 {"client_id": 2, "key": "vanished|Other", "kind_text": "Vanished", "ledger": "Other", "group": "Misc",
  "text": "Gone", "prior_total": 1000, "current_total": 0, "missing_months": ["Apr"],
  "classification": {"text": "One-off", "by": "B. Staff", "at": "2026-10-29T10:00:00Z"}}
]}`

var testWatermark = Watermark{Firm: "Test Firm LLP", User: "priya", At: time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC), ExportID: "0123456789abcdef"}

// readBook opens the workbook the export wrote.
func readBook(t *testing.T, b []byte) *excelize.File {
	t.Helper()
	f, err := excelize.OpenReader(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("the file does not open: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func cell(t *testing.T, f *excelize.File, sheet, ref string) string {
	t.Helper()
	v, err := f.GetCellValue(sheet, ref)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// AC-7a: a sheet per review document, a cannot-check sheet and a Worklist sheet, each with the
// watermark (firm, user, timestamp, export ID) on its first line.
func TestWorkbookHasTheSheetsAndTheWatermark(t *testing.T) {
	src, err := ReadSource(1, []byte(testResult), []byte(testWorklist))
	if err != nil {
		t.Fatal(err)
	}
	file, counts, err := Workbook(testWatermark, src, map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	f := readBook(t, file)
	want := []string{"Sales register vs GSTR-1", "Cannot check", "Worklist"}
	if got := f.GetSheetList(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("sheets %v, want %v", got, want)
	}
	if counts.Sheets != 3 || counts.Findings != 2 || counts.CannotCheck != 1 || counts.Worklist != 1 {
		t.Errorf("counts %+v", counts)
	}
	line := cell(t, f, "Sales register vs GSTR-1", "A1")
	for _, part := range []string{"Test Firm LLP", "priya", "2026-10-30T10:00:00Z", "0123456789abcdef"} {
		if !strings.Contains(line, part) {
			t.Errorf("watermark %q lacks %q", line, part)
		}
	}
	for _, sheet := range want {
		if !strings.HasPrefix(cell(t, f, sheet, "A1"), "Watermark:") {
			t.Errorf("sheet %s has no watermark line", sheet)
		}
	}
}

// AC-7b's figures: a finding shows its figures with amounts in Indian grouping, its locators,
// the difference and the law reference; its State is the decision's.
func TestFindingRowsShowFiguresLocatorsAndState(t *testing.T) {
	src, err := ReadSource(1, []byte(testResult), []byte(testWorklist))
	if err != nil {
		t.Fatal(err)
	}
	file, _, err := Workbook(testWatermark, src, map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := readBook(t, file).GetRows("Sales register vs GSTR-1")
	if err != nil {
		t.Fatal(err)
	}
	// Row 5 is the first finding's first figure; the header is on row 4.
	if rows[3][0] != "Finding ID" {
		t.Fatalf("header row %v", rows[3])
	}
	first := strings.Join(rows[4], " | ")
	for _, part := range []string{"F1", "Fix", "1,23,456.78", "sales.json", "$.rows[1]", "678", "CGST Act 37 s.37"} {
		if !strings.Contains(first, part) {
			t.Errorf("first row %q lacks %q", first, part)
		}
	}
	if rows[5][3] != "Fix" || rows[5][11] != "gstr1.json" {
		t.Errorf("second figure row %v", rows[5])
	}
	if second := strings.Join(rows[6], " | "); !strings.Contains(second, "F2") || !strings.Contains(second, "Open") {
		t.Errorf("finding without a decision: %q", second)
	}
}

// A finding marked deleted is left out of every sheet it would be on.
func TestDeletedFindingsAreLeftOut(t *testing.T) {
	src, err := ReadSource(1, []byte(testResult), []byte(testWorklist))
	if err != nil {
		t.Fatal(err)
	}
	file, counts, err := Workbook(testWatermark, src, map[string]bool{"F2": true})
	if err != nil {
		t.Fatal(err)
	}
	if counts.Findings != 1 {
		t.Errorf("findings %d, want 1 once F2 is deleted", counts.Findings)
	}
	f := readBook(t, file)
	for _, sheet := range f.GetSheetList() {
		rows, _ := f.GetRows(sheet)
		for _, r := range rows {
			if strings.Contains(strings.Join(r, "|"), "F2") {
				t.Errorf("deleted finding F2 is on sheet %s: %v", sheet, r)
			}
		}
	}
}

// The Worklist sheet holds this client's items only.
func TestWorklistSheetHoldsOnlyTheClient(t *testing.T) {
	src, err := ReadSource(2, []byte(testResult), []byte(testWorklist))
	if err != nil {
		t.Fatal(err)
	}
	if len(src.Worklist) != 1 || src.Worklist[0].Ledger != "Other" {
		t.Fatalf("worklist of client 2: %+v", src.Worklist)
	}
	file, counts, err := Workbook(testWatermark, src, map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if counts.Worklist != 1 {
		t.Errorf("worklist rows %d, want 1", counts.Worklist)
	}
	f := readBook(t, file)
	if got := cell(t, f, "Worklist", "I5"); got != "One-off" {
		t.Errorf("classification %q, want One-off", got)
	}
}

// Sheet names are at most 31 characters, free of the characters Excel forbids, and unique.
func TestSheetNames(t *testing.T) {
	used := map[string]bool{}
	long := strings.Repeat("Review ", 10) + "[a]"
	a := sheetName(long, used)
	b := sheetName(long, used)
	c := sheetName("", used)
	if len([]rune(a)) > 31 || strings.ContainsAny(a, "[]") {
		t.Errorf("name %q is too long or has a forbidden character", a)
	}
	if a == b {
		t.Errorf("two sheets are both named %q", a)
	}
	if c != "Review document" {
		t.Errorf("empty heading gives %q", c)
	}
}

// Money with paise is shown in Indian grouping; a rate uses basis points.
func TestValueText(t *testing.T) {
	money := int64(12345678)
	rate := int64(1850)
	for _, c := range []struct {
		v    Value
		want string
	}{
		{Value{Kind: "money", Paise: &money}, "1,23,456.78"},
		{Value{Kind: "rate", RateBP: &rate}, "18.50%"},
	} {
		got, err := c.v.text()
		if err != nil || got != c.want {
			t.Errorf("%+v: %q, %v; want %q", c.v, got, err, c.want)
		}
	}
}

// The export ID is 16 hexadecimal characters and differs between calls.
func TestNewIDIsRandomHex(t *testing.T) {
	a, err := NewID()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewID()
	if len(a) != 16 || a == b || strings.Trim(a, "0123456789abcdef") != "" {
		t.Errorf("ids %q and %q", a, b)
	}
}
