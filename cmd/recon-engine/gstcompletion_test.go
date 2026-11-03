package main

import (
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	csvadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/csv"
	jsonadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/json"
	xlsxadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/xlsx"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/profile"
)

// SPEC-018: the GST part of the September 2026 sample pack (testdata/gst_completion_sep2026,
// fictional company; the company's GSTIN is written with a valid check character, see the
// README there) read by the shipped formats and compared by RECON-GST-030..035. The Excel
// samples of the e-invoice, e-way bill and IMS formats are written from their CSV samples with
// -update (go test ./cmd/recon-engine -run TestGSTCompletionSamples -update), and every run
// checks that the workbooks read to what the CSV files read to.

var (
	completionDir = filepath.Join(repo, "testdata", "gst_completion_sep2026")
	tallyDir      = filepath.Join(profDir, "generic_tally")
	gstnDir       = filepath.Join(profDir, "generic_gstn")
)

// completionFormats are the CSV formats that also have an Excel version: the folder of the
// format, the sheet its samples are written to and the columns written as numbers.
var completionFormats = []struct {
	doc, sheet string
	numeric    []string
}{
	{"einvoice", "E-Invoice Register", []string{"taxable_value", "igst", "cgst", "sgst", "cess", "total_value"}},
	{"ewaybill", "E-Way Bill Register", []string{"total_value"}},
	{"ims", "IMS Actions", []string{"taxable_value"}},
}

// csvSamples lists the CSV samples of a format folder, by name.
func csvSamples(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "v1", "samples", "*.csv"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no CSV samples in %s: %v", dir, err)
	}
	slices.Sort(files)
	return files
}

// writeSheetBook saves a CSV file as a one-sheet workbook. The first sample of a format keeps
// the format's sheet name and header as written; the second goes on a sheet called "Sheet1",
// below a title row, with the header labels in capitals, so the same readings show that the
// table is found by its header text.
func writeSheetBook(t *testing.T, csvFile, xlsxFile, sheet string, numeric []string, layoutB bool) {
	t.Helper()
	b, err := os.ReadFile(csvFile)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(bytes.NewReader(b)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	f := excelize.NewFile()
	defer f.Close()
	name, top := sheet, 0
	if layoutB {
		name, top = "Sheet1", 2
		if err := f.SetCellStr("Sheet1", "A1", "Register download, October 2026"); err != nil {
			t.Fatal(err)
		}
	} else if err := f.SetSheetName("Sheet1", sheet); err != nil {
		t.Fatal(err)
	}
	for r, row := range rows {
		for c, v := range row {
			ref := cellRef(c+1, r+1+top)
			switch {
			case r == 0 && layoutB:
				err = f.SetCellStr(name, ref, strings.ToUpper(v))
			case r > 0 && slices.Contains(numeric, rows[0][c]) && v != "":
				err = f.SetCellDefault(name, ref, v)
			default:
				err = f.SetCellStr(name, ref, v)
			}
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	saveBook(t, f, xlsxFile)
}

// writeExpected writes what reading a sample gives, as the expected file of the format.
func writeSampleExpected(t *testing.T, p *profile.Profile, r *profile.Result, file string) {
	t.Helper()
	body := profile.ExpectedFile(p, r)
	if _, rest, ok := bytes.Cut(body, []byte("\n")); ok {
		body = append([]byte("# What reading the sample gives (SPEC-018): written by go test ./cmd/recon-engine -run TestGSTCompletionSamples -update\n"), rest...)
	}
	if err := os.WriteFile(file, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

// SPEC-018 AC-1, AC-2: every new format reads its samples to the expected rows (profile-test
// checks the expected files), and each Excel workbook reads to what its CSV file reads to.
func TestGSTCompletionSamples(t *testing.T) {
	set := gstSet(t)
	// Formats with one CSV layout only, and the JSON ones: expected files only.
	plain := []struct {
		profileDir string
		samples    []string
		ver        string
	}{
		{filepath.Join(gstnDir, "gstr3b"), []string{"GSTR3B_042026.json", "GSTR3B_092026.json"}, "v1"},
		{filepath.Join(tallyDir, "sales_register"), []string{"tally_sales_register_oct2026.csv", "tally_sales_register_sep2026.csv"}, "v1"},
		{filepath.Join(tallyDir, "purchase_register"), []string{"tally_purchase_register_oct2026.csv", "tally_purchase_register_sep2026.csv"}, "v1"},
		{filepath.Join(profDir, "generic", "gstr2b"), []string{"GSTR2B_Oct2026.json", "GSTR2B_Sep2026.json"}, "v2"},
		{filepath.Join(profDir, "generic", "gstr1"), []string{"GSTR1_Aug2026.json", "GSTR1_Aug2026_short.json"}, "v3"},
	}
	for _, d := range plain {
		ver := d.ver
		for _, s := range d.samples {
			ps, rs := readAny(t, set, filepath.Join(d.profileDir, ver, "samples", s))
			if len(ps) != 1 {
				t.Fatalf("%s was read as %d formats", s, len(ps))
			}
			if *update {
				writeSampleExpected(t, ps[0], rs[0], filepath.Join(d.profileDir, ver, "expected", strings.TrimSuffix(s, filepath.Ext(s))+".facts.yaml"))
			}
		}
	}
	for _, f := range completionFormats {
		csvDir := filepath.Join(portalCSV, f.doc)
		xlsxDir := filepath.Join(portalXLSX, f.doc)
		for i, csvFile := range csvSamples(t, csvDir) {
			stem := strings.TrimSuffix(filepath.Base(csvFile), ".csv")
			book := filepath.Join(xlsxDir, "v1", "samples", stem+".xlsx")
			if *update {
				writeSheetBook(t, csvFile, book, f.sheet, f.numeric, i == 1)
			}
			csvP, csvR := readAny(t, set, csvFile)
			xP, xR := readAny(t, set, book)
			if len(csvP) != 1 || len(xP) != 1 || csvP[0].DocumentType != f.doc || xP[0].DocumentType != f.doc {
				t.Fatalf("%s: read as %d CSV formats and %d Excel formats", stem, len(csvP), len(xP))
			}
			if xR[0].Context != csvR[0].Context {
				t.Errorf("%s: context %+v from the workbook, %+v from the CSV", stem, xR[0].Context, csvR[0].Context)
			}
			if g, w := withoutEmpty(xR[0].Records), withoutEmpty(csvR[0].Records); !slices.Equal(g, w) || len(g) == 0 {
				t.Errorf("%s: the workbook and the CSV read differently:\n%v\n%v", stem, g, w)
			}
			if *update {
				writeSampleExpected(t, csvP[0], csvR[0], filepath.Join(csvDir, "v1", "expected", stem+".facts.yaml"))
				writeSampleExpected(t, xP[0], xR[0], filepath.Join(xlsxDir, "v1", "expected", stem+".facts.yaml"))
			}
		}
	}
}

// amountsOf gives a GSTR-3B row's amounts as "name=paise", sorted.
func amountsOf(r domain.Record) []string {
	var out []string
	for k, v := range r.Amounts {
		out = append(out, k+"="+domain.FormatValue(domain.MoneyValue(v)))
	}
	slices.Sort(out)
	return out
}

// SPEC-018 AC-1: the GSTR-3B JSON gives the same rows as the CSV layout for the same figures
// (the April sample of SPEC-017 written as the portal's JSON). The portal's ITC rows have no
// taxable value, so only the amounts the JSON holds are compared; the JSON's Table 4(B)(1)
// row has no CSV counterpart.
func TestGSTCompletionGSTR3BJSONAgreesWithCSV(t *testing.T) {
	set := gstSet(t)
	jsonP, jsonR := readAny(t, set, filepath.Join(gstnDir, "gstr3b", "v1", "samples", "GSTR3B_042026.json"))
	_, csvR := readAny(t, set, filepath.Join(portalCSV, "gstr3b", "v1", "samples", "GSTR-3B_Apr2026.csv"))
	if len(jsonP) != 1 || jsonP[0].ID != "generic.gstn.gstr3b" {
		t.Fatalf("read as %v", jsonP)
	}
	if jsonR[0].Context.Period != csvR[0].Context.Period || jsonR[0].Context.TaxYear != csvR[0].Context.TaxYear {
		t.Errorf("context %+v from the JSON, %+v from the CSV", jsonR[0].Context, csvR[0].Context)
	}
	csvRows := map[string]domain.Record{}
	for _, r := range csvR[0].Records {
		csvRows[r.MatchKey] = r
	}
	n := 0
	for _, r := range jsonR[0].Records {
		c, ok := csvRows[r.MatchKey]
		if !ok {
			if r.MatchKey != "4(B)(1)" {
				t.Errorf("the JSON gives row %s, which the CSV has not", r.MatchKey)
			}
			continue
		}
		n++
		if r.Fields["description"] != c.Fields["description"] {
			t.Errorf("%s: description %q from the JSON, %q from the CSV", r.MatchKey, r.Fields["description"], c.Fields["description"])
		}
		for k, v := range r.Amounts {
			if cv, ok := c.Amounts[k]; !ok || cv != v {
				t.Errorf("%s %s: %v from the JSON, %v from the CSV", r.MatchKey, k, v, cv)
			}
		}
		if _, has := r.Amounts["taxable_value"]; !has && !strings.HasPrefix(r.MatchKey, "4(") {
			t.Errorf("%s has no taxable value", r.MatchKey)
		}
	}
	if n != 6 {
		t.Errorf("%d rows in both, want 6 (3.1(a) to (d), 4(A)(5), 4(B)(2))", n)
	}
}

// SPEC-018 AC-1: the pack's GSTR-3B JSON gives Tables 3.1(a), 3.1(d), 4(A)(5) and 4(B) with every
// amount to the paise.
func TestGSTCompletionPackGSTR3B(t *testing.T) {
	set := gstSet(t)
	_, rs := readAny(t, set, filepath.Join(completionDir, "inputs", "gstr3b_092026.json"))
	got := map[string][]string{}
	for _, r := range rs[0].Records {
		got[r.MatchKey] = amountsOf(r)
	}
	want := map[string][]string{
		"3.1(a)":  {"cess=2400.00", "cgst=28350.00", "igst=79100.40", "sgst=28350.00", "taxable_value=730000.60"},
		"3.1(d)":  {"cess=0.00", "cgst=375.00", "igst=0.00", "sgst=375.00", "taxable_value=15000.00"},
		"4(A)(5)": {"cess=0.00", "cgst=25380.00", "igst=33300.00", "sgst=25380.00"},
		"4(B)(1)": {"cess=0.00", "cgst=0.00", "igst=0.00", "sgst=0.00"},
		"4(B)(2)": {"cess=0.00", "cgst=0.00", "igst=0.00", "sgst=0.00"},
	}
	if len(got) != len(want) {
		t.Errorf("got rows %v, want %v", got, want)
	}
	for k, w := range want {
		if !slices.Equal(got[k], w) {
			t.Errorf("table %s: %v, want %v", k, got[k], w)
		}
	}
	if rs[0].Context.Period != "2026-09" || rs[0].Context.Scope != "08AAPCS9876K1Z6" {
		t.Errorf("context %+v", rs[0].Context)
	}
}

// readAny reads a JSON, CSV or Excel file with every profile that reads it.
func readAny(t *testing.T, set *profile.Set, file string) ([]*profile.Profile, []*profile.Result) {
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
	ps, err := profile.DetectAll(set, doc, filepath.Base(file))
	if err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	rs, err := profile.ExtractAll(ps, doc, profile.Source{FileName: filepath.Base(file)})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rs {
		if r.Report.HasErrors() {
			t.Fatalf("%s: %v", file, r.Report.Errors)
		}
	}
	return ps, rs
}
