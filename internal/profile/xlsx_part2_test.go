package profile

import (
	"slices"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
)

// SPEC-008 part 2: what real exports do, found by the manual test of 28 October 2026.

// A Grand Total row is left out with why, not read as a ledger head (which refused the
// whole ledger); a problem in a row names its sheet row, not a JSON path.
func TestExcelTotalRowAndRowNames(t *testing.T) {
	text := strings.Replace(ledgerXLSXProfile, "    header: [\"Ledger Head\", \"Month\"]\n",
		"    header: [\"Ledger Head\", \"Month\"]\n    total_rows: [\"Grand Total\"]\n", 1)
	p, err := Load("profile.yaml", []byte(text), realVocab(t))
	if err != nil {
		t.Fatal(err)
	}
	header := []string{"Ledger Head", "Group", "Month", "Amount"}
	res, err := Extract(p, ledgerBook(t, header, append(slices.Clone(ledgerRows), []string{"Grand Total", "", "", "30000.50"}), nil), Source{FileName: "l.xlsx"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Report.HasErrors() || len(res.Records) != 2 {
		t.Fatalf("errors %v, %d records; want the two rows and no error", res.Report.Errors, len(res.Records))
	}
	left := 0
	for _, u := range res.Report.Unread {
		if u.Kind == ingest.UnreadRowLeftOut && u.Row == "Ledger!5:5" && strings.Contains(u.Reason, "a total row") {
			left++
		}
	}
	if left != 2 { // "Grand Total" and its amount
		t.Errorf("unread %+v, want the total row's two cells left out as a total row", res.Report.Unread)
	}

	res, err = Extract(p, ledgerBook(t, header, append(slices.Clone(ledgerRows), []string{"Rent", "", "", "10"}), nil), Source{FileName: "l.xlsx"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Report.Errors) != 1 || !strings.Contains(res.Report.Errors[0].Msg, "Ledger!5:5") || strings.Contains(res.Report.Errors[0].Msg, "$.") {
		t.Errorf("errors %v, want one naming row Ledger!5:5", res.Report.Errors)
	}
}

// A workbook no format reads says what each Excel format looked for.
func TestExcelNoMatchSaysWhy(t *testing.T) {
	p, err := Load("profile.yaml", []byte(ledgerXLSXProfile), realVocab(t))
	if err != nil {
		t.Fatal(err)
	}
	set, _ := NewSet(p)
	_, err = Detect(set, ledgerBook(t, []string{"Ledger Head", "Period"}, ledgerRows, nil), "other.xlsx")
	if ie, ok := err.(*ingest.Error); !ok || ie.Code != ingest.ProfileNoMatch || !strings.Contains(ie.Msg, `lacks "Month"`) || !strings.Contains(ie.Msg, "Test ledger (Excel)") {
		t.Errorf("error %v, want PROFILE_NO_MATCH naming the format and the missing label", err)
	}
}

// An Excel layout is taught on the mapping screen like a JSON one: the inspection lists the
// sheets' columns by header text, and the profile written from the choices reads the file,
// and the same layout on a sheet of another name with headers in other capitals.
func TestTeachExcelLayout(t *testing.T) {
	header := []string{"Ledger Head", "Group", "Month", "Amount"}
	sample := ledgerBook(t, header, ledgerRows, nil)
	in, err := Inspect(sample)
	if err != nil {
		t.Fatal(err)
	}
	var lists, fields []string
	for _, l := range in.Lists {
		lists = append(lists, l.Path)
	}
	for _, f := range in.Fields {
		fields = append(fields, f.Path)
	}
	if !slices.Equal(lists, []string{"$.Ledger[*]"}) || !slices.Contains(fields, "$.Ledger[*]['Ledger Head']") {
		t.Fatalf("inspection lists %v, fields %v", lists, fields)
	}
	m := Mapping{Name: "Books ledger export", DocumentType: "ledger", Rows: "$.Ledger[*]", Unit: "rupees", FileType: "xlsx",
		Columns: []MappingColumn{{Name: "ledger_head", Path: "['Ledger Head']"}, {Name: "month", Path: ".Month"}, {Name: "amount", Path: ".Amount"}},
		Period:  MappingPeriod{Const: PeriodWholeYear}, TaxYear: "2025-26"}
	data, probs := m.YAML(realVocab(t))
	if len(probs) > 0 {
		t.Fatalf("problems %v", probs)
	}
	for _, want := range []string{`file_type: "xlsx"`, `  - name: "Ledger"`, `    header: ["Ledger Head", "Month", "Amount"]`, "    any_name: true"} {
		if !strings.Contains(string(data), want) {
			t.Errorf("profile lacks %q:\n%s", want, data)
		}
	}
	p, err := Load(m.Dir()+"/profile.yaml", data, realVocab(t))
	if err != nil {
		t.Fatalf("%v\n%s", err, data)
	}
	other := ledgerBook(t, []string{"LEDGER HEAD", "Group", "MONTH", "amount"}, ledgerRows, func(f *excelize.File) {
		if err := f.SetSheetName("Ledger", "Export 2025-26"); err != nil {
			t.Fatal(err)
		}
	})
	for name, doc := range map[string]ingest.Doc{"sample": sample, "renamed": other} {
		if ok, err := p.Matches(doc); err != nil || !ok {
			t.Errorf("%s: matches %v, %v", name, ok, err)
			continue
		}
		res, err := Extract(p, doc, Source{FileName: name + ".xlsx"})
		if err != nil || res.Report.HasErrors() || len(res.Records) != 2 || res.Records[1].Amounts["amount"] != 1500050 {
			t.Errorf("%s: err %v, errors %v, records %+v", name, err, res.Report.Errors, res.Records)
		}
	}

	// A path that is not a column of a sheet is a problem named in the user's terms.
	m.Columns[0].Path = "['Ledger Head'].x"
	if _, probs := m.YAML(realVocab(t)); len(probs) != 1 || probs[0].Field != "columns[0]" {
		t.Errorf("problems %v, want one for columns[0]", probs)
	}
}
