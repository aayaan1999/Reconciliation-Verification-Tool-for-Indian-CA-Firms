package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// SPEC-008 part 2: a plain Excel export — the register on its own, without the Details sheet
// the shipped sample adds, so neither the period nor the client's GSTIN is in the file — is
// taught on the mapping screen, and a run for a client with one GSTIN gives the same GST
// findings as the JSON register: the client's GSTIN is taken from its details, and the
// report says so.
func TestTeachPlainExcelExport(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "sales_export.xlsx")
	f, err := excelize.OpenFile(filepath.Join(repo, "content", "profiles", "generic_xlsx", "sales_register", "v1", "samples", "SALES_REG_Aug2026.xlsx"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.DeleteSheet("Details"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetSheetName("Sales Register", "Sheet1"); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(plain); err != nil {
		t.Fatal(err)
	}
	f.Close()

	office := filepath.Join(dir, "office")
	out := filepath.Join(dir, "out.json")
	col := func(name, label string) string {
		return `{"name": "` + name + `", "path": "['` + label + `']", "date_format": ""}`
	}
	mapping := `{"name": "Plain sales export", "document_type": "sales_register", "rows": "$.Sheet1[*]", "unit": "rupees",
 "where": [{"path": "['Supply Type']", "equals": "B2B"}],
 "columns": [` + strings.Join([]string{col("invoice_no", "Voucher No"), col("counterparty_gstin", "Party GSTIN"),
		col("invoice_date", "Voucher Date"), col("place_of_supply", "Place Of Supply"), col("rate", "GST Rate"),
		col("taxable_value", "Taxable Value"), col("igst", "IGST"), col("cgst", "CGST"), col("sgst", "SGST UTGST"),
		col("cess", "Cess"), col("invoice_value", "Invoice Value")}, ", ") + `],
 "period": {"path": "", "format": "", "const": "2026-08"}, "tax_year": "", "gstin_path": "", "identify": []}`
	r := withStdin(t, mapping, "profile-create", "--profiles", profDir, "--office-profiles", office, "--vocab", vocabDir, "--sample", plain, "--json", out)
	if r.code != exitOK {
		t.Fatalf("profile-create: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}

	// The shipped profile reads only Doc Type INV rows that are not cancelled; one filter is
	// all the mapping screen offers, so a copy holds only the rows the shipped one reads.
	input := filepath.Join(dir, "in")
	if err := os.Mkdir(input, 0o700); err != nil {
		t.Fatal(err)
	}
	keepInvoices(t, plain, filepath.Join(input, "sales_export.xlsx"))
	gstr1, err := os.ReadFile(filepath.Join(repo, "testdata", "gst_pack_aug2026", "GSTR1_Aug2026.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(input, "GSTR1_Aug2026.json"), gstr1, 0o600); err != nil {
		t.Fatal(err)
	}

	db := newDB(t)
	if r := withStdin(t, deccanJSON, "client-add", "--db", db, "--tax-year", "2026-27"); r.code != exitOK {
		t.Fatalf("client-add: exit %d\n%s", r.code, r.stderr)
	}
	r = cmd(t, "run", "--db", db, "--input", input, "--tax-year", "2026-27", "--vocab", vocabDir, "--profiles", profDir,
		"--office-profiles", office, "--include-testing", "--client", "1",
		"--comparison", "gst_sales_vs_gstr1", "--comparisons", filepath.Join(repo, "content", "comparisons", "comparisons.yaml"))
	if r.code != exitFindings {
		t.Fatalf("run: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	// The taught layout reads the B2B invoices only, so the three B2C and HSN rules of the type
	// (SPEC-015) cannot check: they are not reported as passing.
	for _, want := range []string{"26 checks: 17 pass, 6 fail, 3 cannot check", "GSTIN_FROM_CLIENT", "client's only GSTIN (ending Z1ZF)", "Sheet1!"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("report lacks %q:\n%s", want, r.stdout)
		}
	}
}

// keepInvoices copies the register keeping only the rows whose Doc Type is INV and that
// are not cancelled, as the shipped Excel profile reads.
func keepInvoices(t *testing.T, from, to string) {
	t.Helper()
	f, err := excelize.OpenFile(from)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := f.GetRows("Sheet1")
	if err != nil {
		t.Fatal(err)
	}
	hdr := map[string]int{}
	for i, v := range rows[2] {
		hdr[v] = i
	}
	for r := len(rows) - 1; r >= 3; r-- {
		get := func(label string) string {
			if i := hdr[label]; i < len(rows[r]) {
				return rows[r][i]
			}
			return ""
		}
		if get("Doc Type") != "INV" || get("Is Cancelled") != "N" {
			if err := f.RemoveRow("Sheet1", r+1); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := f.SaveAs(to); err != nil {
		t.Fatal(err)
	}
}
