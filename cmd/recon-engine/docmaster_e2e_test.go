package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// SPEC-022 risk 1 and AC-8, end to end: a document the Partner adds, with column names no built-in
// uses, is taught from a JSON, a CSV and an Excel file, read into rows under its own record type,
// and a run records the office vocabulary it used. With the Master empty the same layout is
// refused with a message naming the document.
func TestOfficeDocumentTaughtFromJSONCSVAndExcel(t *testing.T) {
	type sample struct{ ref, raised, customer, net, tax, gross string }
	rows := []sample{
		{"SR-0001", "03-08-2026", "Orion Retail", "100000", "18000", "118000"},
		{"SR-0002", "04-08-2026", "Vega Traders", "50000", "9000", "59000"},
		{"SR-0003", "09-08-2026", "Orion Retail", "20000", "3600", "23600"},
	}
	// the three files hold the same rows
	var jsonRows []string
	csvRows := []string{"Ref No,Raised,Customer,Net,Tax,Gross"}
	for _, r := range rows {
		jsonRows = append(jsonRows, `{"Ref":"`+r.ref+`","Raised":"`+r.raised+`","Customer":"`+r.customer+`","Net":`+r.net+`,"Tax":`+r.tax+`,"Gross":`+r.gross+`}`)
		csvRows = append(csvRows, strings.Join([]string{r.ref, r.raised, r.customer, r.net, r.tax, r.gross}, ","))
	}
	writeXLSX := func(path string) {
		f := excelize.NewFile()
		defer f.Close()
		for i, h := range []string{"Ref No", "Raised", "Customer", "Net", "Tax", "Gross"} {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellValue("Sheet1", cell, h)
		}
		for i, r := range rows {
			for j, v := range []string{r.ref, r.raised, r.customer, r.net, r.tax, r.gross} {
				cell, _ := excelize.CoordinatesToCellName(j+1, i+2)
				f.SetCellValue("Sheet1", cell, v)
			}
		}
		if err := f.SaveAs(path); err != nil {
			t.Fatal(err)
		}
	}
	mapping := func(rowsPath string, cols map[string]string) string {
		var parts []string
		for _, name := range []string{"ref_no", "doc_date", "customer", "net_amount", "tax_amount", "gross_amount"} {
			date := ""
			if name == "doc_date" {
				date = "dd-mm-yyyy"
			}
			parts = append(parts, `{"name":"`+name+`","path":"`+cols[name]+`","date_format":"`+date+`"}`)
		}
		return `{"name":"Sales return register","document_type":"sales_return","rows":"` + rowsPath + `","where":[],"unit":"rupees","columns":[` +
			strings.Join(parts, ",") + `],"period":{"path":"","format":"","const":"2026-08"},"tax_year":"","gstin_path":"","identify":[]}`
	}
	bracket := map[string]string{"ref_no": "['Ref No']", "doc_date": "['Raised']", "customer": "['Customer']", "net_amount": "['Net']", "tax_amount": "['Tax']", "gross_amount": "['Gross']"}
	dotted := map[string]string{"ref_no": ".Ref", "doc_date": ".Raised", "customer": ".Customer", "net_amount": ".Net", "tax_amount": ".Tax", "gross_amount": ".Gross"}

	cases := []struct {
		name, file, mapping string
		write               func(path string)
	}{
		{"json", "returns.json", mapping("$.Returns[*]", dotted), func(p string) {
			if err := os.WriteFile(p, []byte(`{"Returns":[`+strings.Join(jsonRows, ",")+`]}`), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"csv", "returns.csv", mapping("$[*]", bracket), func(p string) {
			if err := os.WriteFile(p, []byte(strings.Join(csvRows, "\n")+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"xlsx", "returns.xlsx", mapping("$.Sheet1[*]", bracket), writeXLSX},
	}
	runArgs := func(db, input, office string) []string {
		return []string{"run", "--db", db, "--input", input, "--tax-year", "2026-27", "--vocab", vocabDir, "--profiles", profDir, "--office-profiles", office, "--include-testing"}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			input := filepath.Join(dir, "in")
			if err := os.Mkdir(input, 0o700); err != nil {
				t.Fatal(err)
			}
			tc.write(filepath.Join(input, tc.file))
			office := filepath.Join(dir, "office")

			db := newDB(t)
			body := sixColumnReturns
			if r, out := docCmd[docSaveOut](t, "document-save", db, body); r.code != 0 || !out.Saved || out.Document.Status != "active" {
				t.Fatalf("document-save: exit %d %s", r.code, r.stderr)
			}
			r := withStdin(t, tc.mapping, "profile-create", "--profiles", profDir, "--office-profiles", office, "--vocab", vocabDir, "--db", db, "--by", "A. Partner",
				"--rules", rulesDir, "--sample", filepath.Join(input, tc.file), "--json", filepath.Join(dir, "taught.json"))
			if r.code != exitOK {
				t.Fatalf("profile-create: exit %d\n%s%s", r.code, r.stdout, r.stderr)
			}
			if got := one(t, db, `SELECT document_type FROM format_versions WHERE source = 'office'`); got != "sales_return" {
				t.Errorf("format_versions.document_type = %q", got)
			}
			r = cmd(t, runArgs(db, input, office)...)
			if r.code != exitOK && r.code != exitFindings {
				t.Fatalf("run: exit %d\n%s%s", r.code, r.stdout, r.stderr)
			}
			got := query(t, db, `SELECT record_type, match_key, amounts_json, fields_json FROM records ORDER BY match_key`)
			if len(got) != 3 {
				t.Fatalf("%d rows stored, want 3: %v", len(got), got)
			}
			for i, want := range rows {
				g := got[i]
				if g[0] != "sales_return_row" || g[1] != want.ref {
					t.Errorf("row %d: record type %q key %q", i, g[0], g[1])
				}
				for col, rupees := range map[string]string{"net_amount": want.net, "tax_amount": want.tax, "gross_amount": want.gross} {
					if !strings.Contains(g[2], fmt.Sprintf(`"%s":%s00`, col, rupees)) {
						t.Errorf("row %d: amounts %s lack %s = %s rupees (in paise)", i, g[2], col, rupees)
					}
				}
				if !strings.Contains(g[3], `"customer":"`+want.customer+`"`) || !strings.Contains(g[3], `"doc_date":"2026-08-`) {
					t.Errorf("row %d: fields %s", i, g[3])
				}
			}
			hash := one(t, db, `SELECT office_vocab_hash FROM runs ORDER BY id DESC LIMIT 1`)
			if len(hash) != 64 || one(t, db, `SELECT COUNT(*) FROM office_vocab_snapshots WHERE hash = ?`, hash) != "1" {
				t.Errorf("the run's office vocabulary %q is not kept as a snapshot", hash)
			}

			// the stored rows and the layout show up as what reads the document			if _, d := docCmd[docDetailOut](t, "document-get", db, "", "--code", "sales_return"); d.Usage.Records != 3 || len(d.Usage.Formats) != 1 || !d.Structure.Locked {				t.Errorf("usage after a run: %+v, locked %v; want 3 rows, one layout, locked", d.Usage, d.Structure.Locked)			}
			// A document a layout reads cannot be deleted; deactivating it leaves the layout reading files.
			if r, out := docCmd[docDeleteOut](t, "document-delete", db, `{"by":"A. Partner","code":"sales_return"}`); r.code != exitInput || out.Code != "IN_USE" {
				t.Errorf("delete of a document a layout reads: exit %d %+v", r.code, out)
			}
			if r, _ := docCmd[docDetailOut](t, "document-status", db, `{"by":"A. Partner","code":"sales_return","active":false}`); r.code != 0 {
				t.Fatalf("deactivate: %s", r.stderr)
			}
			if r = cmd(t, runArgs(db, input, office)...); r.code != exitOK && r.code != exitFindings {
				t.Errorf("a run after deactivating the document: exit %d\n%s%s", r.code, r.stdout, r.stderr)
			}
			if after := one(t, db, `SELECT office_vocab_hash FROM runs ORDER BY id DESC LIMIT 1`); after != hash {
				t.Errorf("deactivating changed the office hash: %s -> %s", hash, after)
			}

			// With the Master empty (a database that never had the document) the layout is refused, naming it.
			r = cmd(t, runArgs(newDB(t), input, office)...)
			if r.code != exitInput || !strings.Contains(r.stderr, "sales_return") {
				t.Errorf("an empty Master: exit %d, stderr %q; want an input error naming sales_return", r.code, r.stderr)
			}
		})
	}
}

// sixColumnReturns is a sales return register with the six columns the test files hold.
const sixColumnReturns = `{"by":"A. Partner","code":"sales_return","name":"Sales return register","category":"Books",
 "description":"Credit notes issued to customers","applicable_for":["json","xlsx","csv"],"status":"active",
 "record_type":{"name":"sales_return_row","description":"Rows of the sales return register","columns":[
  {"name":"ref_no","label":"Reference","type":"text","key":true,"required":true},
  {"name":"doc_date","label":"Date","type":"date","key":false,"required":false},
  {"name":"customer","label":"Customer","type":"text","key":false,"required":false},
  {"name":"net_amount","label":"Net amount","type":"amount","key":false,"required":false},
  {"name":"tax_amount","label":"Tax amount","type":"amount","key":false,"required":false},
  {"name":"gross_amount","label":"Gross amount","type":"amount","key":false,"required":false}]}}`

// AC-2 and AC-22 (Part A side): with office documents in the Master, active and inactive, every
// sample case gives the very bytes of the committed samples, which were written with an empty
// Master; the rules list is the same too. The base content hash does not move.
func TestOfficeDocumentsLeaveBuiltinOutputUnchanged(t *testing.T) {
	dir := filepath.Join(repo, "web", "src", "sample")
	withOffice := func(t *testing.T) string {
		db := newDB(t)
		if r, out := docCmd[docSaveOut](t, "document-save", db, sixColumnReturns); r.code != 0 || !out.Saved {
			t.Fatalf("document-save: %s", r.stderr)
		}
		second := strings.NewReplacer(`"code":"sales_return"`, `"code":"return_ledger"`, `"sales_return_row"`, `"return_ledger_row"`).Replace(sixColumnReturns)
		if r, out := docCmd[docSaveOut](t, "document-save", db, second); r.code != 0 || !out.Saved {
			t.Fatalf("document-save: %s", r.stderr)
		}
		if r, _ := docCmd[docDetailOut](t, "document-status", db, `{"by":"A. Partner","code":"return_ledger","active":false}`); r.code != 0 {
			t.Fatalf("deactivate: %s", r.stderr)
		}
		return db
	}
	t.Run("rules", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "rules.json")
		if r := cmd(t, "rules", "--db", withOffice(t), "--vocab", vocabDir, "--json", out); r.code != exitOK {
			t.Fatalf("rules: exit %d\n%s", r.code, r.stderr)
		}
		compareSample(t, filepath.Join(dir, "rules.json"), out)
	})
	for _, c := range webSamples {
		t.Run(c, func(t *testing.T) {
			db := withOffice(t)
			input := filepath.Join(runData, c)
			runJSON := filepath.Join(t.TempDir(), "run.json")
			args := append([]string{"--include-testing", "--json", runJSON}, webSampleRun[c]...)
			if r := runCmd(t, db, input, args...); r.code > exitFindings {
				t.Fatalf("run: exit %d\n%s", r.code, r.stderr)
			}
			factsJSON := filepath.Join(t.TempDir(), "facts.json")
			if r := cmd(t, "facts", "--db", db, "--input", input, "--json", factsJSON); r.code != exitOK {
				t.Fatalf("facts: exit %d\n%s", r.code, r.stderr)
			}
			compareSample(t, filepath.Join(dir, "run-"+c+".json"), runJSON)
			compareSample(t, filepath.Join(dir, "facts-"+c+".json"), factsJSON)
			v, _, err := loadVocabMaster(db, vocabDir)
			if err != nil {
				t.Fatal(err)
			}
			if v.OfficeHash() == "" {
				t.Error("the Master's documents are not in the vocabulary, so this test proves nothing")
			}
		})
	}
}

// AC-2 for every folder of testdata/run, including those the engine refuses: the exit code, the
// messages and the report are the same with an empty Master and with office documents in it.
func TestEveryRunFolderSameWithAndWithoutOfficeDocuments(t *testing.T) {
	entries, err := os.ReadDir(runData)
	if err != nil {
		t.Fatal(err)
	}
	both := func(t *testing.T, input string) (plain, office result, jsonA, jsonB []byte) {
		args := func(db, out string) []string {
			return []string{"run", "--db", db, "--input", input, "--tax-year", "2025-26", "--vocab", vocabDir, "--profiles", profDir, "--include-testing", "--json", out}
		}
		dbA, dbB := newDB(t), newDB(t)
		if r, _ := docCmd[docSaveOut](t, "document-save", dbB, sixColumnReturns); r.code != 0 {
			t.Fatalf("document-save: %s", r.stderr)
		}
		outA, outB := filepath.Join(t.TempDir(), "a.json"), filepath.Join(t.TempDir(), "b.json")
		plain, office = cmd(t, args(dbA, outA)...), cmd(t, args(dbB, outB)...)
		jsonA, _ = os.ReadFile(outA)
		jsonB, _ = os.ReadFile(outB)
		return
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		n++
		t.Run(e.Name(), func(t *testing.T) {
			plain, office, a, b := both(t, filepath.Join(runData, e.Name()))
			if plain.code != office.code || plain.stdout != office.stdout || plain.stderr != office.stderr {
				t.Errorf("exit %d / %d, output differs:\n--- empty Master\n%s%s\n--- with office documents\n%s%s", plain.code, office.code, plain.stdout, plain.stderr, office.stdout, office.stderr)
			}
			if string(a) != string(b) {
				t.Error("the report differs between an empty Master and one with office documents")
			}
		})
	}
	if n < 9 {
		t.Fatalf("found only %d run folders", n)
	}
}
