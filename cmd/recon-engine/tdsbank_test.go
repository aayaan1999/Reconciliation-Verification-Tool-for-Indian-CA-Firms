package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
)

// SPEC-020: the TDS and bank part of the September 2026 sample pack (testdata/tds_bank_sep2026,
// fictional company) read by the shipped formats and compared by RECON-TDS-010..013 and
// RECON-BNK-010..011. The expected files of the formats are written with
//
//	go test ./cmd/recon-engine -run TestTDSBankSamples -update
//
// and every run checks that each sample is read by exactly one format.

var tdsBankDir = filepath.Join(repo, "testdata", "tds_bank_sep2026")

// tdsBankFormats are the new formats: the folder of the format under content/profiles and its document type.
var tdsBankFormats = []struct{ dir, doc string }{
	{"generic_tally/tds_ledger", "tds_ledger"},
	{"generic_portal/form140", "form140"},
	{"generic_tally/tds_challan", "tds_challan"},
	{"generic_portal/oltas_status", "oltas_status"},
	{"generic_tally/tds_receivable", "tds_receivable"},
	{"generic_portal/form168", "form168"},
	{"generic_tally/bank_book", "bank_book"},
	{"generic_portal/bank_statement", "bank_statement"},
	{"generic_portal/bank_balances", "bank_balances"},
}

// SPEC-020 AC-2: each sample of each new format is read by that format alone, and -update
// writes what it reads to.
func TestTDSBankSamples(t *testing.T) {
	set := gstSet(t)
	ty := period.TaxYear{StartYear: 2026}
	for _, f := range tdsBankFormats {
		files, err := filepath.Glob(filepath.Join(profDir, filepath.FromSlash(f.dir), "v1", "samples", "*"))
		if err != nil || len(files) != 2 {
			t.Fatalf("%s: want two samples, got %v (%v)", f.dir, files, err)
		}
		for _, file := range files {
			ps, rs := readAnyFor(t, set, file, ty)
			if len(ps) != 1 || ps[0].DocumentType != f.doc {
				t.Fatalf("%s was read as %d formats (want the %s format only)", filepath.Base(file), len(ps), f.doc)
			}
			if len(rs[0].Records) == 0 && len(rs[0].Facts) == 0 {
				t.Errorf("%s: nothing was read", filepath.Base(file))
			}
			if *update {
				stem := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
				writeSampleExpected(t, ps[0], rs[0], filepath.Join(profDir, filepath.FromSlash(f.dir), "v1", "expected", stem+".facts.yaml"))
			}
		}
	}
}

// tdsBankExcel are the formats that also have an Excel version: the CSV folder, the Excel folder,
// the sheet the first sample is written to and the columns written as numbers (SPEC-020).
var tdsBankExcel = []struct {
	csvDir, xlsxDir, doc, sheet string
	numeric                     []string
}{
	{"generic_tally", "generic_tally_xlsx", "tds_ledger", "TDS Ledger", []string{"amount_paid", "tds_rate", "tds_amount"}},
	{"generic_portal", "generic_portal_xlsx", "form140", "Form 140", []string{"amount_paid", "tds_deducted", "tds_deposited"}},
	{"generic_tally", "generic_tally_xlsx", "tds_challan", "TDS Challans", []string{"amount"}},
	{"generic_portal", "generic_portal_xlsx", "oltas_status", "OLTAS Status", []string{"amount", "consumed", "available"}},
	{"generic_tally", "generic_tally_xlsx", "tds_receivable", "TDS Receivable", []string{"amount_credited", "tds_expected"}},
	{"generic_portal", "generic_portal_xlsx", "form168", "Form 168", []string{"amount_paid", "tds_deposited"}},
	{"generic_tally", "generic_tally_xlsx", "bank_book", "Bank Book", []string{"receipt", "payment"}},
	{"generic_portal", "generic_portal_xlsx", "bank_statement", "Bank Statement", []string{"debit", "credit"}},
}

// SPEC-020 AC-2: each Excel workbook reads to what its CSV file reads to. The workbooks are
// written from the CSV samples with -update; the second sample of a format goes on a sheet
// called "Sheet1", below a title row, with the headings in capitals.
func TestTDSBankExcelSamples(t *testing.T) {
	set := gstSet(t)
	ty := period.TaxYear{StartYear: 2026}
	for _, f := range tdsBankExcel {
		csvDir := filepath.Join(profDir, f.csvDir, f.doc)
		xlsxDir := filepath.Join(profDir, f.xlsxDir, f.doc)
		for i, csvFile := range csvSamples(t, csvDir) {
			stem := strings.TrimSuffix(filepath.Base(csvFile), ".csv")
			book := filepath.Join(xlsxDir, "v1", "samples", stem+".xlsx")
			if *update {
				writeSheetBook(t, csvFile, book, f.sheet, f.numeric, i == 1)
			}
			csvP, csvR := readAnyFor(t, set, csvFile, ty)
			xP, xR := readAnyFor(t, set, book, ty)
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
				writeSampleExpected(t, xP[0], xR[0], filepath.Join(xlsxDir, "v1", "expected", stem+".facts.yaml"))
			}
		}
	}
}
