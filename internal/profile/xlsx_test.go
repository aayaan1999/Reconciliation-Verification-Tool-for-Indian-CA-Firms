package profile

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	xlsxadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/xlsx"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// SPEC-008: Excel profiles read sheets as tables by header text.

const ledgerXLSXProfile = `schema: "recon.profile/1"
profile_id: "test.books.ledger_xlsx"
profile_version: "1"
source_system: "Test ledger (Excel)"
document_type: "ledger"
file_type: "xlsx"
regime: "ITA1961"
sheets:
  - name: "Ledger"
    header: ["Ledger Head", "Month"]
detect:
  - json_exists: { path: "$.Ledger[0].Month" }
context:
  tax_year: { json_path: "$.Ledger[0].Month", from_month: true }
  period: { const: "FY" }
records:
  - record_type: "ledger_month"
    rows: "$.Ledger[*]"
    unit: "rupees"
    fields:
      - { name: "ledger_head", path: "['Ledger Head']", required: true }
      - { name: "group", path: ".Group" }
      - { name: "month", path: ".Month", required: true }
    amounts:
      - { name: "amount", path: ".Amount" }
`

func realVocab(t *testing.T) *vocab.Vocab {
	t.Helper()
	v, err := vocab.LoadDir(filepath.Join("..", "..", "content", "vocabulary"))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// ledgerBook is a ledger workbook: a title row, the header in row 2, two data rows, and
// what fn adds.
func ledgerBook(t *testing.T, header []string, rows [][]string, fn func(f *excelize.File)) ingest.Doc {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", "Ledger"); err != nil {
		t.Fatal(err)
	}
	put := func(col, row int, v string) {
		ref, _ := excelize.CoordinatesToCellName(col, row)
		var err error
		if strings.HasPrefix(v, "=") {
			err = f.SetCellFormula("Ledger", ref, v[1:])
		} else if v != "" && strings.Trim(v, "0123456789.") == "" {
			err = f.SetCellDefault("Ledger", ref, v)
		} else if v != "" {
			err = f.SetCellStr("Ledger", ref, v)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	put(1, 1, "Expense ledger (synthetic)")
	for i, h := range header {
		put(i+1, 2, h)
	}
	for r, row := range rows {
		for i, v := range row {
			put(i+1, r+3, v)
		}
	}
	if fn != nil {
		fn(f)
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	doc, err := xlsxadapter.New(0).Open(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func readBook(t *testing.T, doc ingest.Doc) *Result {
	t.Helper()
	p, err := Load("profile.yaml", []byte(ledgerXLSXProfile), realVocab(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := NewSet(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Detect(set, doc, "ledger.xlsx"); err != nil {
		t.Fatal(err)
	}
	res, err := Extract(p, doc, Source{FileName: "ledger.xlsx"})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

var ledgerRows = [][]string{{"Rent", "Premises", "2025-04", "15000"}, {"Rent", "Premises", "2025-05", "15000.50"}}

func TestExcelProfileReadsByHeader(t *testing.T) {
	res := readBook(t, ledgerBook(t, []string{"Ledger Head", "Group", "Month", "Amount", "Remarks"}, ledgerRows, nil))
	if res.Report.HasErrors() || len(res.Records) != 2 {
		t.Fatalf("errors %v, %d records", res.Report.Errors, len(res.Records))
	}
	r := res.Records[1]
	if r.Amounts["amount"] != 1500050 || r.Fields["month"] != "2025-05" {
		t.Errorf("record %+v", r)
	}
	if r.Locator.Kind != domain.LocatorSheetCell || r.Locator.Path != "Ledger!4:4" || r.Locator.FileName != "ledger.xlsx" {
		t.Errorf("locator %+v, want ledger.xlsx Ledger!4:4", r.Locator)
	}
	if res.Context.TaxYear.String() != "2025-26" {
		t.Errorf("tax year %s", res.Context.TaxYear)
	}
	// AC-5: what nothing read is listed with its sheet and cell.
	var unread []string
	for _, u := range res.Report.Unread {
		unread = append(unread, u.Locator.Path)
	}
	if got := strings.Join(unread, " "); got != "Ledger!A1" {
		t.Errorf("unread %q, want the title cell only", got)
	}
	// AC-4: moving and inserting columns gives the same records.
	moved := readBook(t, ledgerBook(t, []string{"Amount", "Note", "Month", "Ledger Head", "Group"},
		[][]string{{"15000", "x", "2025-04", "Rent", "Premises"}, {"15000.50", "", "2025-05", "Rent", "Premises"}}, nil))
	a, _ := json.Marshal(stripLocators(res.Records))
	b, _ := json.Marshal(stripLocators(moved.Records))
	if !bytes.Equal(a, b) {
		t.Errorf("moving columns changed the records:\n%s\n%s", a, b)
	}
}

func stripLocators(rs []domain.Record) []domain.Record {
	out := make([]domain.Record, len(rs))
	for i, r := range rs {
		r.Locator = domain.Locator{}
		out[i] = r
	}
	return out
}

func TestExcelProfileProblems(t *testing.T) {
	// AC-6: a formula with no saved value is missing, never a zero.
	formula := readBook(t, ledgerBook(t, []string{"Ledger Head", "Group", "Month", "Amount"},
		[][]string{{"Rent", "Premises", "2025-04", "15000"}, {"=A3", "Premises", "2025-05", "1"}}, nil))
	if !formula.Report.HasErrors() || formula.Report.Errors[0].Code != ingest.RequiredFieldMissing || len(formula.Records) != 0 {
		t.Errorf("a required formula cell with no saved value: %+v", formula.Report.Errors)
	}
	// AC-4: a label the profile names, found twice, is an error naming the label.
	p, err := Load("profile.yaml", []byte(ledgerXLSXProfile), realVocab(t))
	if err != nil {
		t.Fatal(err)
	}
	set, _ := NewSet(p)
	_, err = Detect(set, ledgerBook(t, []string{"Ledger Head", "Month", "Month"}, ledgerRows, nil), "twice.xlsx")
	if err == nil || !strings.Contains(err.Error(), `"Month" twice`) || !strings.Contains(err.Error(), "twice.xlsx") {
		t.Errorf("a label found twice: %v", err)
	}
	// A header without a label the profile names: the format does not match.
	_, err = Detect(set, ledgerBook(t, []string{"Ledger Head", "Period"}, ledgerRows, nil), "other.xlsx")
	if ie, ok := err.(*ingest.Error); !ok || ie.Code != ingest.ProfileNoMatch {
		t.Errorf("a missing label: %v, want PROFILE_NO_MATCH", err)
	}
	// Hidden rows are read and the report says so.
	hidden := readBook(t, ledgerBook(t, []string{"Ledger Head", "Group", "Month", "Amount"}, ledgerRows, func(f *excelize.File) {
		if err := f.SetRowVisible("Ledger", 4, false); err != nil {
			t.Fatal(err)
		}
	}))
	if len(hidden.Records) != 2 || len(hidden.Report.Warnings) != 1 || hidden.Report.Warnings[0].Code != ingest.HiddenContent {
		t.Errorf("a hidden row: %d records, warnings %+v", len(hidden.Records), hidden.Report.Warnings)
	}
}

// AC-9: the same workbook read twice gives byte-identical results.
func TestExcelProfileDeterministic(t *testing.T) {
	doc := ledgerBook(t, []string{"Ledger Head", "Group", "Month", "Amount", "Remarks"}, ledgerRows, nil)
	a, _ := json.Marshal(readBook(t, doc))
	b, _ := json.Marshal(readBook(t, doc))
	if !bytes.Equal(a, b) {
		t.Error("two reads of one workbook differ")
	}
}

func TestExcelProfileRefusals(t *testing.T) {
	v := realVocab(t)
	for _, c := range []struct{ name, from, to, want string }{
		{"no sheets", "sheets:\n  - name: \"Ledger\"\n    header: [\"Ledger Head\", \"Month\"]\n", "", "names the sheets it reads"},
		{"unknown sheet in a path", `rows: "$.Ledger[*]"`, `rows: "$.Ledgr[*]"`, "does not start in a sheet"},
		{"label twice", `header: ["Ledger Head", "Month"]`, `header: ["Month", " Month "]`, `"Month" is named twice`},
		{"no labels", `header: ["Ledger Head", "Month"]`, `header: []`, "at least one label"},
		{"sheets on JSON", `file_type: "xlsx"`, `file_type: "json"`, "sheets is for Excel"},
		{"bundle", `regime: "ITA1961"`, "regime: \"ITA1961\"\nbundle: \"test.pack\"\nsection: \"$.Ledger\"", "bundle and section are for JSON"},
	} {
		t.Run(c.name, func(t *testing.T) {
			text := strings.Replace(ledgerXLSXProfile, c.from, c.to, 1)
			if text == ledgerXLSXProfile {
				t.Fatal("the change does not apply")
			}
			_, err := Load("profile.yaml", []byte(text), v)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %v, want %q", err, c.want)
			}
		})
	}
}

// A header label found twice in a sheet one format names does not stop another format that
// reads the workbook from matching; with no match, the label error is what is reported.
func TestHeaderErrorDoesNotStopOtherFormats(t *testing.T) {
	v := realVocab(t)
	a, err := Load("a.yaml", []byte(ledgerXLSXProfile), v)
	if err != nil {
		t.Fatal(err)
	}
	other := strings.NewReplacer(`"test.books.ledger_xlsx"`, `"test.books.ledger_by_group"`, `["Ledger Head", "Month"]`, `["Ledger Head", "Group"]`,
		`json_exists: { path: "$.Ledger[0].Month" }`, `json_exists: { path: "$.Ledger[0].Group" }`).Replace(ledgerXLSXProfile)
	b, err := Load("b.yaml", []byte(other), v)
	if err != nil {
		t.Fatal(err)
	}
	both, err := NewSet(a, b)
	if err != nil {
		t.Fatal(err)
	}
	book := ledgerBook(t, []string{"Ledger Head", "Group", "Month", "Month"}, ledgerRows, nil)
	if p, err := Detect(both, book, "twice.xlsx"); err != nil || p != b {
		t.Errorf("Detect = %v, %v; want %s", p, err, b.Key())
	}
	alone, _ := NewSet(a)
	if _, err := Detect(alone, book, "twice.xlsx"); err == nil || !strings.Contains(err.Error(), `"Month" twice`) {
		t.Errorf("with no other format: %v, want the label error", err)
	}
}
