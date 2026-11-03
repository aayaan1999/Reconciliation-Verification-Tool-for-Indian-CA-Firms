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

	"github.com/xuri/excelize/v2"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	csvadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/csv"
	xlsxadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/xlsx"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/profile"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// SPEC-017: GSTR-1, GSTR-2A and GSTR-3B of one month, read from three CSV files or from one
// workbook with a sheet each, and compared with each other. The files are synthetic
// (testdata/gst_returns_apr2026). With -update the May workbook sample and the expected
// files of the shipped formats are written again; every run checks that the workbooks read
// to what the CSV files read to, apart from the places.

var (
	gstReturnsDir = filepath.Join(repo, "testdata", "gst_returns_apr2026")
	portalCSV     = filepath.Join(profDir, "generic_portal")
	portalXLSX    = filepath.Join(profDir, "generic_portal_xlsx")
	// gstDocs are the three returns: the folder of the format, the CSV file stem and the sheet.
	gstDocs = []struct{ doc, stem, sheet string }{
		{"gstr1", "GSTR-1", "GSTR-1"},
		{"gstr2a", "GSTR-2A", "GSTR-2A"},
		{"gstr3b", "GSTR-3B", "GSTR-3B"},
	}
)

// gstNumeric are the columns written as numbers; every other column is text, as the
// portal's tables hold them (a date as dd-mm-yyyy text, an HSN code as text).
var gstNumeric = []string{"Invoice Value", "Quantity", "Taxable Value", "Rate (%)", "IGST", "CGST", "SGST/UTGST", "Cess"}

// writeGSTBook saves the May CSV files as a workbook in another layout than the April one:
// the sheets in another order and under other names, no title rows, and the header labels
// in capitals, so the same readings show that sheets are found by their header text.
func writeGSTBook(t *testing.T, xlsxFile string) {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	names := map[string]string{"gstr1": "Outward", "gstr2a": "ITC 2A", "gstr3b": "Summary 3B"}
	order := []string{"gstr3b", "gstr1", "gstr2a"}
	for i, doc := range order {
		var stem string
		for _, d := range gstDocs {
			if d.doc == doc {
				stem = d.stem
			}
		}
		b, err := os.ReadFile(filepath.Join(gstReturnsDir, stem+"_May2026.csv"))
		if err != nil {
			t.Fatal(err)
		}
		rows, err := csv.NewReader(bytes.NewReader(b)).ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		sheet := names[doc]
		if i == 0 {
			if err := f.SetSheetName("Sheet1", sheet); err != nil {
				t.Fatal(err)
			}
		} else if _, err := f.NewSheet(sheet); err != nil {
			t.Fatal(err)
		}
		for r, row := range rows {
			for c, v := range row {
				ref := cellRef(c+1, r+1)
				var err error
				switch {
				case r == 0:
					err = f.SetCellStr(sheet, ref, strings.ToUpper(v))
				case slices.Contains(gstNumeric, rows[0][c]):
					err = f.SetCellDefault(sheet, ref, v)
				default:
					err = f.SetCellStr(sheet, ref, v)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	saveBook(t, f, xlsxFile)
}

// gstSet loads the vocabulary and every installed profile.
func gstSet(t *testing.T) *profile.Set {
	t.Helper()
	v, err := vocab.LoadDir(vocabDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := profile.LoadDir(profDir, v)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

// readGST reads a file with every profile that reads it (a workbook may hold several documents).
func readGST(t *testing.T, set *profile.Set, file string) ([]*profile.Profile, []*profile.Result) {
	t.Helper()
	reg := ingest.NewRegistry()
	for _, a := range []ingest.Adapter{csvadapter.New(0), xlsxadapter.New(0)} {
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
		t.Fatal(err)
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

// SPEC-017 AC-1: each shipped format reads its samples to the expected rows (profile-test
// checks the expected files); the workbooks read to what the CSV files read to.
func TestGSTReturnsSamples(t *testing.T) {
	set := gstSet(t)
	books := map[string]string{
		"Apr2026": filepath.Join(gstReturnsDir, "GST_Returns_Apr2026.xlsx"),
		"May2026": filepath.Join(gstReturnsDir, "GST_Returns_May2026.xlsx"),
	}
	if *update {
		writeGSTBook(t, books["May2026"])
		for _, d := range gstDocs {
			if err := os.MkdirAll(filepath.Join(portalXLSX, d.doc, "v1", "samples"), 0o755); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(books["May2026"])
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(portalXLSX, d.doc, "v1", "samples", "GST_Returns_May2026.xlsx"), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	for month, book := range books {
		bookP, fromBook := readGST(t, set, book)
		if len(fromBook) != 3 {
			t.Fatalf("%s: the workbook was read as %d documents, want 3", book, len(fromBook))
		}
		for _, d := range gstDocs {
			ps, rs := readGST(t, set, filepath.Join(gstReturnsDir, d.stem+"_"+month+".csv"))
			csvP, csvR := ps[0], rs[0]
			if csvP.DocumentType != d.doc || len(ps) != 1 {
				t.Fatalf("%s %s was read as %d formats, the first %s", d.stem, month, len(ps), csvP.Key())
			}
			var xP *profile.Profile
			var xR *profile.Result
			for i, p := range bookP {
				if p.DocumentType == d.doc {
					xP, xR = p, fromBook[i]
				}
			}
			if xP == nil {
				t.Fatalf("%s: no %s read from the workbook", month, d.doc)
			}
			if xR.Context != csvR.Context {
				t.Errorf("%s %s: context %+v from the workbook, %+v from the CSV", month, d.doc, xR.Context, csvR.Context)
			}
			if g, w := withoutEmpty(xR.Records), withoutEmpty(csvR.Records); !slices.Equal(g, w) || len(g) == 0 {
				t.Errorf("%s %s: the workbook and the CSV read differently:\n%v\n%v", month, d.doc, g, w)
			}
			if *update {
				write := func(p *profile.Profile, r *profile.Result, dir, ext string) {
					body := profile.ExpectedFile(p, r)
					if _, rest, ok := bytes.Cut(body, []byte("\n")); ok {
						body = append([]byte("# What reading the sample gives (SPEC-017): written by go test ./cmd/recon-engine -run TestGSTReturnsSamples -update\n# and checked on every run against what the CSV files read to.\n"), rest...)
					}
					name := d.stem + "_" + month + ext
					if ext == ".xlsx" {
						name = "GST_Returns_" + month + ".facts.yaml"
					} else {
						name = d.stem + "_" + month + ".facts.yaml"
					}
					if err := os.WriteFile(filepath.Join(dir, d.doc, "v1", "expected", name), body, 0o644); err != nil {
						t.Fatal(err)
					}
				}
				write(csvP, csvR, portalCSV, ".csv")
				write(xP, xR, portalXLSX, ".xlsx")
			}
		}
	}
}

// withoutEmpty gives the records as recordLines does, leaving out a column with no text: a
// CSV file holds an empty field (a B2CS row's GSTIN), an Excel cell left empty holds none.
func withoutEmpty(rs []domain.Record) []string {
	out := make([]domain.Record, len(rs))
	for i, r := range rs {
		fields := map[string]string{}
		for k, v := range r.Fields {
			if v != "" {
				fields[k] = v
			}
		}
		r.Fields = fields
		out[i] = r
	}
	return recordLines(out)
}

// gstReport is the part of a run's JSON these tests read.
type gstReport struct {
	Findings []struct {
		Findings []gstEval `json:"findings"`
	} `json:"findings"`
	Passed      []gstEval `json:"passed"`
	CannotCheck []struct {
		RuleID  string `json:"rule_id"`
		Subject struct {
			MatchKey string `json:"match_key"`
		} `json:"subject"`
		Missing []struct {
			DocumentType string `json:"document_type"`
			Fact         string `json:"fact"`
		} `json:"missing"`
	} `json:"cannot_check"`
}

type gstEval struct {
	RuleID     string `json:"rule_id"`
	Difference int64  `json:"difference_paise"`
	Subject    struct {
		MatchKey string `json:"match_key"`
	} `json:"subject"`
	Values []struct {
		Input   string `json:"input"`
		Label   string `json:"label"`
		Locator struct {
			FileName string `json:"file_name"`
			Path     string `json:"path"`
		} `json:"locator"`
		Value struct {
			Paise int64 `json:"paise"`
		} `json:"value"`
	} `json:"values"`
}

// runGST writes the files, given by name, into one folder, runs every rule on it and returns
// the report.
func runGST(t *testing.T, files map[string][]byte) gstReport {
	t.Helper()
	in := t.TempDir()
	for name, b := range files {
		if err := os.WriteFile(filepath.Join(in, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	jsonFile := filepath.Join(t.TempDir(), "report.json")
	r := cmd(t, "run", "--db", newDB(t), "--input", in, "--tax-year", "2026-27", "--vocab", vocabDir, "--profiles", profDir,
		"--include-testing", "--json", jsonFile)
	if r.code != 0 && r.code != exitFindings {
		t.Fatalf("exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	b, err := os.ReadFile(jsonFile)
	if err != nil {
		t.Fatalf("exit %d, no JSON report: %v\n%s%s", r.code, err, r.stdout, r.stderr)
	}
	var rep gstReport
	if err := json.Unmarshal(b, &rep); err != nil {
		t.Fatal(err)
	}
	return rep
}

func fileBytes(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(gstReturnsDir, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func csvsOf(t *testing.T, month string) map[string][]byte {
	t.Helper()
	m := map[string][]byte{}
	for _, d := range gstDocs {
		m[d.stem+"_"+month+".csv"] = fileBytes(t, d.stem+"_"+month+".csv")
	}
	return m
}

// gstOutcome is one result of the four rules in a form that leaves out the file names, so a
// run on CSV files and one on a workbook can be compared.
type gstOutcome struct {
	Rule, Amount, Status string
	Difference           int64
	Sum, Total           int64 // the sum of the rows and the row it is compared with, in paise
	Rows                 int   // the rows added up
}

func (r gstReport) outcomes(t *testing.T) []gstOutcome {
	t.Helper()
	var out []gstOutcome
	add := func(e gstEval, status string) {
		if !strings.HasPrefix(e.RuleID, "RECON-GST-02") {
			return
		}
		o := gstOutcome{Rule: e.RuleID, Amount: e.Subject.MatchKey, Status: status, Difference: e.Difference}
		for _, v := range e.Values {
			switch {
			case v.Input == "sum":
				o.Sum = v.Value.Paise
			case strings.HasPrefix(v.Input, "a."):
				o.Rows++
			case strings.HasPrefix(v.Input, "b."):
				o.Total = v.Value.Paise
			}
			if v.Input != "sum" && (v.Locator.FileName == "" || v.Locator.Path == "") {
				t.Errorf("%s %s: the value %q has no source", e.RuleID, e.Subject.MatchKey, v.Label)
			}
		}
		out = append(out, o)
	}
	for _, g := range r.Findings {
		for _, e := range g.Findings {
			add(e, "fail")
		}
	}
	for _, e := range r.Passed {
		add(e, "pass")
	}
	slices.SortFunc(out, func(a, b gstOutcome) int {
		return strings.Compare(a.Rule+"|"+a.Amount, b.Rule+"|"+b.Amount)
	})
	return out
}

// aprilWant is what the April sample files give (SPEC-017 Design notes): 020 and 021 agree,
// 022 differs in IGST, CGST and SGST, 023 differs in every amount but the cess. Passing
// results of a rule with no value shown (the Passed list) carry no values, so only the
// failing ones are compared on their figures.
func aprilWant() []gstOutcome {
	var want []gstOutcome
	for _, rule := range []string{"RECON-GST-020", "RECON-GST-021"} {
		for _, a := range []string{"cess", "cgst", "igst", "sgst", "taxable_value"} {
			want = append(want, gstOutcome{Rule: rule, Amount: a, Status: "pass"})
		}
	}
	for _, a := range []string{"cess", "taxable_value"} {
		want = append(want, gstOutcome{Rule: "RECON-GST-022", Amount: a, Status: "pass"})
	}
	for _, a := range []string{"cess"} {
		want = append(want, gstOutcome{Rule: "RECON-GST-023", Amount: a, Status: "pass"})
	}
	want = append(want,
		gstOutcome{"RECON-GST-022", "cgst", "fail", 90000, 90000, 0, 1},
		gstOutcome{"RECON-GST-022", "igst", "fail", 180000, 0, 180000, 1},
		gstOutcome{"RECON-GST-022", "sgst", "fail", 90000, 90000, 0, 1},
		gstOutcome{"RECON-GST-023", "cgst", "fail", 252000, 450000, 198000, 2},
		gstOutcome{"RECON-GST-023", "igst", "fail", 1404000, 1800000, 396000, 2},
		gstOutcome{"RECON-GST-023", "sgst", "fail", 252000, 450000, 198000, 2},
		gstOutcome{"RECON-GST-023", "taxable_value", "fail", 12800000, 15000000, 2200000, 2})
	slices.SortFunc(want, func(a, b gstOutcome) int { return strings.Compare(a.Rule+"|"+a.Amount, b.Rule+"|"+b.Amount) })
	return want
}

// SPEC-017 AC-4: the three CSV files together, and the workbook alone, give the same results:
// 020 and 021 agree, 022 and 023 give the differences of the Design notes, each with both
// values, the difference and both sources, and nothing else is found.
func TestGSTReturnsCSVAndWorkbookAgree(t *testing.T) {
	fromCSV := runGST(t, csvsOf(t, "Apr2026"))
	fromBook := runGST(t, map[string][]byte{"GST_Returns_Apr2026.xlsx": fileBytes(t, "GST_Returns_Apr2026.xlsx")})
	for name, rep := range map[string]gstReport{"three CSV files": fromCSV, "workbook": fromBook} {
		got := rep.outcomes(t)
		if !slices.Equal(got, aprilWant()) {
			t.Errorf("%s:\n got %+v\nwant %+v", name, got, aprilWant())
		}
		for _, g := range rep.Findings {
			for _, f := range g.Findings {
				if f.RuleID != "RECON-GST-022" && f.RuleID != "RECON-GST-023" {
					t.Errorf("%s: an unexpected finding of %s", name, f.RuleID)
				}
			}
		}
	}
	// In a workbook every source is a sheet and row.
	for _, g := range fromBook.Findings {
		for _, f := range g.Findings {
			for _, v := range f.Values {
				if v.Input != "sum" && !strings.Contains(v.Locator.Path, "!") {
					t.Errorf("workbook source %q of %s is not a sheet and row", v.Locator.Path, f.RuleID)
				}
			}
		}
	}
}

// SPEC-017 AC-5: with one of the three files, the rules that need the others name the
// missing document, and nothing is found wrong.
func TestGSTReturnsMissingDocumentIsNamed(t *testing.T) {
	missing := map[string]map[string][]string{ // file stem, then rule, then the documents named
		"GSTR-1":  {"RECON-GST-020": {"gstr3b"}, "RECON-GST-021": {"gstr3b"}, "RECON-GST-022": {"gstr2a", "gstr3b"}, "RECON-GST-023": {"gstr2a", "gstr3b"}},
		"GSTR-2A": {"RECON-GST-020": {"gstr1", "gstr3b"}, "RECON-GST-021": {"gstr1", "gstr3b"}, "RECON-GST-022": {"gstr3b"}, "RECON-GST-023": {"gstr3b"}},
		"GSTR-3B": {"RECON-GST-020": {"gstr1"}, "RECON-GST-021": {"gstr1"}, "RECON-GST-022": {"gstr2a"}, "RECON-GST-023": {"gstr2a"}},
	}
	for stem, want := range missing {
		t.Run(stem, func(t *testing.T) {
			name := stem + "_Apr2026.csv"
			rep := runGST(t, map[string][]byte{name: fileBytes(t, name)})
			for _, g := range rep.Findings {
				if len(g.Findings) > 0 {
					t.Errorf("a finding with only %s: %s", name, g.Findings[0].RuleID)
				}
			}
			if o := rep.outcomes(t); len(o) != 0 {
				t.Errorf("results of the rules with only %s: %+v", name, o)
			}
			got := map[string][]string{}
			for _, c := range rep.CannotCheck {
				if strings.HasPrefix(c.RuleID, "RECON-GST-02") {
					for _, m := range c.Missing {
						got[c.RuleID] = append(got[c.RuleID], m.DocumentType)
					}
					slices.Sort(got[c.RuleID])
				}
			}
			for rule, docs := range want {
				if !slices.Equal(got[rule], docs) {
					t.Errorf("%s names %v as missing, want %v", rule, got[rule], docs)
				}
			}
		})
	}
}

// SPEC-017 AC-6: on a month whose three returns agree, a defect planted alone fires only the
// expected rule.
func TestGSTReturnsPlantedDefects(t *testing.T) {
	clean := runGST(t, csvsOf(t, "May2026"))
	for _, o := range clean.outcomes(t) {
		if o.Status != "pass" {
			t.Fatalf("the clean month has %+v", o)
		}
	}
	if n := len(clean.outcomes(t)); n != 20 {
		t.Fatalf("the clean month gives %d results, want 20 (4 rules, 5 amounts)", n)
	}
	change := func(file, from, to string) map[string][]byte {
		m := csvsOf(t, "May2026")
		s := string(m[file])
		if !strings.Contains(s, from) {
			t.Fatalf("%s has no %q", file, from)
		}
		m[file] = []byte(strings.Replace(s, from, to, 1))
		return m
	}
	t.Run("one GSTR-1 CGST 1 paise past the tolerance", func(t *testing.T) {
		// INV-102 CGST 900 becomes 901.01: the rows add up to Rs 1.01 more than 3.1(a)
		rep := runGST(t, change("GSTR-1_May2026.csv", "18,0,900,900,0", "18,0,901.01,900,0"))
		var failed []string
		for _, o := range rep.outcomes(t) {
			if o.Status == "fail" {
				failed = append(failed, o.Rule+" "+o.Amount)
				if o.Difference != 101 {
					t.Errorf("difference %d paise, want 101", o.Difference)
				}
			}
		}
		if !slices.Equal(failed, []string{"RECON-GST-020 cgst"}) {
			t.Errorf("failed %v, want only RECON-GST-020 cgst", failed)
		}
	})
	t.Run("one GSTR-1 CGST exactly at the tolerance", func(t *testing.T) {
		rep := runGST(t, change("GSTR-1_May2026.csv", "18,0,900,900,0", "18,0,901,900,0"))
		for _, o := range rep.outcomes(t) {
			if o.Status != "pass" {
				t.Errorf("Rs 1.00 is within the tolerance, but %+v", o)
			}
		}
	})
	for _, c := range []struct{ table, rule string }{{"3.1(a)", "RECON-GST-020"}, {"3.1(b)", "RECON-GST-021"}, {"3.1(d)", "RECON-GST-022"}, {"4(A)(5)", "RECON-GST-023"}} {
		t.Run("GSTR-3B row "+c.table+" removed", func(t *testing.T) {
			var kept []string
			for _, line := range strings.Split(string(fileBytes(t, "GSTR-3B_May2026.csv")), "\n") {
				if !strings.Contains(line, ","+c.table+",") {
					kept = append(kept, line)
				}
			}
			m := csvsOf(t, "May2026")
			m["GSTR-3B_May2026.csv"] = []byte(strings.Join(kept, "\n"))
			rep := runGST(t, m)
			for _, g := range rep.Findings {
				if len(g.Findings) > 0 {
					t.Errorf("a finding: %s", g.Findings[0].RuleID)
				}
			}
			cannot := map[string]int{}
			for _, cc := range rep.CannotCheck {
				if strings.HasPrefix(cc.RuleID, "RECON-GST-02") {
					cannot[cc.RuleID]++
					if len(cc.Missing) != 1 || cc.Missing[0].Fact != "row:gstr3b_table_row|"+c.table || cc.Missing[0].DocumentType != "gstr3b" {
						t.Errorf("%s names %+v", cc.RuleID, cc.Missing)
					}
				}
			}
			if len(cannot) != 1 || cannot[c.rule] != 5 {
				t.Errorf("Cannot check %v, want only %s, once for each of 5 amounts", cannot, c.rule)
			}
			if n := len(rep.outcomes(t)); n != 15 {
				t.Errorf("%d other results, want 15 (3 rules, 5 amounts), all passing", n)
			}
		})
	}
}
