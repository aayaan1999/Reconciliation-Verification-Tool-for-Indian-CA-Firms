package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	csvadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/csv"
	jsonadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/json"
	xlsxadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/xlsx"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/profile"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// SPEC-008: the Excel samples are the JSON and CSV samples saved as workbooks, with the
// columns in another order. With -update they are written again from those samples, with
// their expected files; every run checks that each workbook reads to exactly what its JSON
// or CSV source reads to, apart from the locators.

var (
	xlsxProfiles = filepath.Join(profDir, "generic_xlsx")
	salesXLSX    = filepath.Join(xlsxProfiles, "sales_register", "v1", "samples")
	ledgerXLSX   = filepath.Join(xlsxProfiles, "ledger", "v1", "samples")
	salesJSON    = filepath.Join(profDir, "generic", "sales_register", "v1", "samples")
	ledgerCSV    = filepath.Join(profDir, "generic", "ledger", "v1", "samples")
)

// salesColumns are the keys of a sales register row, in the order the workbook shows them:
// the JSON layout's order, reversed.
var salesColumns = []string{"narration", "period", "fy", "b2b_or_b2c", "is_cancelled", "doc_type", "note_type", "port_code",
	"shipping_bill_date", "shipping_bill_no", "export_type", "supply_type", "supply_class", "ship_to_state", "place_of_supply",
	"original_invoice_no", "irn", "reverse_charge", "invoice_value", "round_off", "cess", "sgst_utgst", "cgst", "igst", "gst_rate",
	"taxable_value", "rate", "uom", "quantity", "hsn_sac", "item_name", "ledger_name", "registration_type", "party_state",
	"party_pan", "party_gstin", "party_name", "voucher_no", "voucher_type", "voucher_date"}

// header turns a JSON key into a header label: "party_gstin" is "Party GSTIN".
func header(key string) string {
	acronyms := []string{"gstin", "pan", "hsn", "sac", "uom", "gst", "igst", "cgst", "sgst", "utgst", "irn", "fy", "b2b", "b2c"}
	words := strings.Split(key, "_")
	for i, w := range words {
		switch {
		case slices.Contains(acronyms, w):
			words[i] = strings.ToUpper(w)
		case w == "or":
		default:
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// serial is the Excel date serial (1900 system) of a YYYY-MM-DD date.
func serial(t *testing.T, day string) int64 {
	t.Helper()
	d, err := time.Parse(time.DateOnly, day)
	if err != nil {
		t.Fatal(err)
	}
	return int64(d.Sub(time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)) / (24 * time.Hour))
}

func saveBook(t *testing.T, f *excelize.File, file string) {
	t.Helper()
	if err := f.SaveAs(file); err != nil {
		t.Fatal(err)
	}
}

func cellRef(col, row int) string {
	ref, _ := excelize.CoordinatesToCellName(col, row)
	return ref
}

// writeSalesBook saves the JSON sales register as a workbook: a Details sheet with the
// document, client GSTIN and return period, and a Sales Register sheet with a title row,
// an empty row and column A empty, the header in row 3 from column B, numbers as numbers
// (exact text, never through float64) and the voucher date as an Excel date.
func writeSalesBook(t *testing.T, jsonFile, xlsxFile string) {
	t.Helper()
	b, err := os.ReadFile(jsonFile)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var reg struct {
		Document    string           `json:"document"`
		ClientGSTIN string           `json:"client_gstin"`
		Period      string           `json:"period"`
		Rows        []map[string]any `json:"rows"`
	}
	if err := dec.Decode(&reg); err != nil {
		t.Fatal(err)
	}
	f := excelize.NewFile()
	defer f.Close()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(f.SetSheetName("Sheet1", "Details"))
	for i, kv := range [][2]string{{"Document", reg.Document}, {"Client GSTIN", reg.ClientGSTIN}, {"Return Period", reg.Period}} {
		must(f.SetCellStr("Details", cellRef(i+1, 1), kv[0]))
		must(f.SetCellStr("Details", cellRef(i+1, 2), kv[1]))
	}
	const sh = "Sales Register"
	_, err = f.NewSheet(sh)
	must(err)
	must(f.SetCellStr(sh, "B1", "Sales register - "+reg.Period+" (SYNTHETIC TEST DATA, fictional company)"))
	day, err := f.NewStyle(&excelize.Style{CustomNumFmt: ptr("dd-mm-yyyy")})
	must(err)
	for i, k := range salesColumns {
		must(f.SetCellStr(sh, cellRef(i+2, 3), header(k)))
	}
	for r, row := range reg.Rows {
		if len(row) != len(salesColumns) {
			t.Fatalf("%s row %d has %d columns, the workbook %d", jsonFile, r, len(row), len(salesColumns))
		}
		for i, k := range salesColumns {
			ref := cellRef(i+2, r+4)
			switch v := row[k].(type) {
			case nil:
			case json.Number:
				must(f.SetCellDefault(sh, ref, v.String()))
			case string:
				switch {
				case v == "":
				case k == "voucher_date":
					must(f.SetCellInt(sh, ref, serial(t, v)))
					must(f.SetCellStyle(sh, ref, ref, day))
				default:
					must(f.SetCellStr(sh, ref, v))
				}
			default:
				t.Fatalf("%s row %d: %s is %T", jsonFile, r, k, v)
			}
		}
	}
	saveBook(t, f, xlsxFile)
}

func ptr(s string) *string { return &s }

// ledgerColumns are the CSV columns in the order the workbook shows them, with their labels.
var ledgerColumns = [][2]string{{"ledger_head", "Ledger Head"}, {"group", "Group"}, {"period", "Month"}, {"amount_inr", "Amount (Rs)"}}

// writeLedgerBook saves the CSV ledger as a workbook with one sheet, Ledger, the months as
// text and the amounts as numbers.
func writeLedgerBook(t *testing.T, csvFile, xlsxFile string) {
	t.Helper()
	b, err := os.ReadFile(csvFile)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(bytes.NewReader(b)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	col := map[string]int{}
	for i, h := range rows[0] {
		col[h] = i
	}
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName("Sheet1", "Ledger"); err != nil {
		t.Fatal(err)
	}
	for i, c := range ledgerColumns {
		if err := f.SetCellStr("Ledger", cellRef(i+1, 1), c[1]); err != nil {
			t.Fatal(err)
		}
	}
	for r, row := range rows[1:] {
		for i, c := range ledgerColumns {
			ref, v := cellRef(i+1, r+2), row[col[c[0]]]
			var err error
			if c[0] == "amount_inr" {
				err = f.SetCellDefault("Ledger", ref, v)
			} else {
				err = f.SetCellStr("Ledger", ref, v)
			}
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	saveBook(t, f, xlsxFile)
}

// readSample detects and reads one sample with every installed profile.
func readSample(t *testing.T, set *profile.Set, file string) (*profile.Profile, *profile.Result) {
	t.Helper()
	reg := ingest.NewRegistry()
	for _, a := range []ingest.Adapter{jsonadapter.New(0), csvadapter.New(0), xlsxadapter.New(0)} {
		if err := reg.Register(a); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	ad, err := reg.ForFile(data[:min(len(data), sniffBytes)], filepath.Base(file))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := ad.Open(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	p, err := profile.Detect(set, doc, filepath.Base(file))
	if err != nil {
		t.Fatal(err)
	}
	res, err := profile.Extract(p, doc, profile.Source{FileName: filepath.Base(file)})
	if err != nil {
		t.Fatal(err)
	}
	if res.Report.HasErrors() {
		t.Fatalf("%s: %v", file, res.Report.Errors)
	}
	return p, res
}

// recordLines gives each record without its locator, sorted.
func recordLines(rs []domain.Record) []string {
	var out []string
	for _, r := range rs {
		r.Locator = domain.Locator{}
		b, _ := json.Marshal(r)
		out = append(out, string(b))
	}
	slices.Sort(out)
	return out
}

func TestExcelSamples(t *testing.T) {
	pairs := []struct{ source, book string }{
		{filepath.Join(salesJSON, "SALES_REG_Aug2026.json"), filepath.Join(salesXLSX, "SALES_REG_Aug2026.xlsx")},
		{filepath.Join(salesJSON, "SALES_REG_Aug2026_short.json"), filepath.Join(salesXLSX, "SALES_REG_Aug2026_short.xlsx")},
		{filepath.Join(ledgerCSV, "ledger_FY2024-25.csv"), filepath.Join(ledgerXLSX, "ledger_FY2024-25.xlsx")},
		{filepath.Join(ledgerCSV, "ledger_FY2025-26.csv"), filepath.Join(ledgerXLSX, "ledger_FY2025-26.xlsx")},
	}
	v, err := vocab.LoadDir(vocabDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := profile.LoadDir(profDir, v)
	if err != nil {
		t.Fatal(err)
	}
	for _, pr := range pairs {
		if *update {
			if strings.HasSuffix(pr.source, ".json") {
				writeSalesBook(t, pr.source, pr.book)
			} else {
				writeLedgerBook(t, pr.source, pr.book)
			}
		}
		_, want := readSample(t, set, pr.source)
		p, got := readSample(t, set, pr.book)
		// The workbook holds the rows of its own layout; the JSON source, read by a later format
		// version, may have more (SPEC-015: B2C and HSN rows). Compare the rows both hold.
		types := map[string]bool{}
		for _, r := range got.Records {
			types[r.RecordType] = true
		}
		want.Records = slices.DeleteFunc(slices.Clone(want.Records), func(r domain.Record) bool { return !types[r.RecordType] })
		if p.FileType != "xlsx" {
			t.Fatalf("%s was read by %s", pr.book, p.Key())
		}
		if got.Context != want.Context {
			t.Errorf("%s: context %+v, the source %+v", pr.book, got.Context, want.Context)
		}
		if g, w := recordLines(got.Records), recordLines(want.Records); !slices.Equal(g, w) || len(g) == 0 {
			t.Errorf("%s: %d records, the source %d; they differ", pr.book, len(g), len(w))
		}
		for _, r := range got.Records {
			if r.Locator.Kind != domain.LocatorSheetCell || !strings.Contains(r.Locator.Path, "!") || !strings.Contains(r.Locator.Path, ":") {
				t.Errorf("%s: record locator %+v, want a sheet row", pr.book, r.Locator)
				break
			}
		}
		expected := filepath.Join(filepath.Dir(filepath.Dir(pr.book)), "expected", strings.TrimSuffix(filepath.Base(pr.book), ".xlsx")+".facts.yaml")
		body := profile.ExpectedFile(p, got)
		if _, rest, ok := bytes.Cut(body, []byte("\n")); ok {
			body = append([]byte("# What reading the sample gives (SPEC-008): written by go test ./cmd/recon-engine -run TestExcelSamples -update\n# and checked against its JSON or CSV source on every test run.\n"), rest...)
		}
		if *update {
			if err := os.WriteFile(expected, body, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// withoutPlaces drops what names a file or a place in it (the files read, locators), so two
// reports of the same figures from different files can be compared.
func withoutPlaces(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range x {
			if k == "files" || k == "locator" || k == "documents" {
				continue
			}
			out[k] = withoutPlaces(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = withoutPlaces(val)
		}
		return out
	}
	return v
}

// reportFor runs the engine on files for one type of check and returns its JSON report.
func reportFor(t *testing.T, taxYear, comparison string, files ...string) (result, any) {
	t.Helper()
	return reportWith(t, taxYear, comparison, nil, files...)
}

// reportWith is reportFor with more flags for the run, such as --rules.
func reportWith(t *testing.T, taxYear, comparison string, extra []string, files ...string) (result, any) {
	t.Helper()
	in := t.TempDir()
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(in, filepath.Base(f)), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	jsonFile := filepath.Join(t.TempDir(), "report.json")
	args := append([]string{"run", "--db", newDB(t), "--input", in, "--tax-year", taxYear, "--vocab", vocabDir, "--profiles", profDir,
		"--include-testing", "--comparison", comparison, "--comparisons", filepath.Join(repo, "content", "comparisons", "comparisons.yaml"),
		"--json", jsonFile}, extra...)
	r := cmd(t, args...)
	b, err := os.ReadFile(jsonFile)
	if err != nil {
		t.Fatalf("exit %d, no JSON report: %v\n%s%s", r.code, err, r.stdout, r.stderr)
	}
	var rep any
	if err := json.Unmarshal(b, &rep); err != nil {
		t.Fatal(err)
	}
	return r, rep
}

// SPEC-008 AC-8: the sales register from Excel gives the same findings as from JSON, apart
// from where each figure is; and each Excel figure names its sheet and row.
func TestExcelGivesTheSameFindings(t *testing.T) {
	gstr1 := filepath.Join(runData, "gst-sales", "GSTR1_Aug2026.json")
	// The Excel layout holds the B2B invoices only (SPEC-008); the JSON register also has the B2C
	// and HSN rows (SPEC-015). The two are compared on the rule both can check.
	b2b := []string{"--rules", "RECON-GST-001"}
	rj, fromJSON := reportWith(t, "2026-27", "gst_sales_vs_gstr1", b2b, filepath.Join(runData, "gst-sales", "SALES_REG_Aug2026.json"), gstr1)
	rx, fromExcel := reportWith(t, "2026-27", "gst_sales_vs_gstr1", b2b, filepath.Join(salesXLSX, "SALES_REG_Aug2026.xlsx"), gstr1)
	if rj.code != exitFindings || rx.code != exitFindings {
		t.Fatalf("exit %d (JSON) and %d (Excel), want %d\n%s", rj.code, rx.code, exitFindings, rx.stderr)
	}
	// The document totals add up every row each layout reads: the JSON register also holds the B2C
	// and HSN rows (SPEC-015), the Excel one the B2B rows only, so the totals differ by design. The
	// findings and the counts of the rule both layouts can check are what must agree.
	comparable := func(v any) any {
		w := withoutPlaces(v).(map[string]any)
		delete(w, "totals")
		return w
	}
	a, _ := json.Marshal(comparable(fromJSON))
	b, _ := json.Marshal(comparable(fromExcel))
	if !bytes.Equal(a, b) {
		t.Errorf("the reports differ beyond the places:\nJSON  %.600s\nExcel %.600s", a, b)
	}
	raw, _ := json.Marshal(fromExcel)
	if !strings.Contains(string(raw), `"kind":"sheet_cell","path":"Sales Register!6:6"`) {
		t.Errorf("no finding names its Excel row: %.400s", raw)
	}
	if !strings.Contains(rx.stdout, "SALES_REG_Aug2026.xlsx") || !strings.Contains(rx.stdout, "generic.books.sales_register_xlsx") {
		t.Errorf("the text report does not show the workbook and its format:\n%.800s", rx.stdout)
	}
}

// SPEC-008 AC-8a: last year's and this year's ledger as Excel give exactly the Worklist the
// CSV ledgers give.
func TestExcelGivesTheSameWorklist(t *testing.T) {
	items := func(dir string, names ...string) []string {
		t.Helper()
		in := t.TempDir()
		for _, n := range names {
			b, err := os.ReadFile(filepath.Join(dir, n))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(in, n), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		db := newDB(t)
		if r := withStdin(t, nimbusPAN, "client-add", "--db", db, "--name", "Nimbus Logic Systems", "--tax-year", "2025-26"); r.code != exitOK {
			t.Fatalf("client-add: exit %d\n%s", r.code, r.stderr)
		}
		if r := runCmd(t, db, in, "--client", "1"); r.code > exitFindings {
			t.Fatalf("run: exit %d\n%s%s", r.code, r.stdout, r.stderr)
		}
		var out []string
		for _, it := range readWorklist(t, db).Items {
			b, _ := json.Marshal(it)
			out = append(out, string(b))
		}
		return out
	}
	csvItems := items(ledgerCSV, "ledger_FY2024-25.csv", "ledger_FY2025-26.csv")
	xlsxItems := items(ledgerXLSX, "ledger_FY2024-25.xlsx", "ledger_FY2025-26.xlsx")
	if len(csvItems) != 12 || !slices.Equal(csvItems, xlsxItems) {
		t.Errorf("Worklist from CSV (%d items) and from Excel (%d items) differ\nCSV   %q\nExcel %q", len(csvItems), len(xlsxItems), csvItems, xlsxItems)
	}
}
