package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/profile"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// sampleExpected is a recon.expected/1 file, read to compare two of them without their places.
type sampleExpected struct {
	Schema         string            `yaml:"schema"`
	ProfileID      string            `yaml:"profile_id"`
	ProfileVersion string            `yaml:"profile_version"`
	Context        map[string]string `yaml:"context"`
	Facts          []struct {
		Fact    string `yaml:"fact"`
		Value   string `yaml:"value"`
		Period  string `yaml:"period"`
		Locator string `yaml:"locator"`
	} `yaml:"facts"`
	Records []struct {
		RecordType string      `yaml:"record_type"`
		MatchKey   string      `yaml:"match_key"`
		Locator    string      `yaml:"locator"`
		Fields     []namedPair `yaml:"fields"`
		Amounts    []namedPair `yaml:"amounts"`
	} `yaml:"records"`
}

// namedPair is one field or amount of a row.
type namedPair struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// lines is the file's figures and rows without their places, sorted.
func (e sampleExpected) lines() []string {
	var out []string
	for _, f := range e.Facts {
		out = append(out, "fact "+f.Fact+" = "+f.Value+" ("+f.Period+")")
	}
	byName := func(a, b namedPair) int { return strings.Compare(a.Name, b.Name) }
	for _, r := range e.Records {
		slices.SortFunc(r.Fields, byName)
		slices.SortFunc(r.Amounts, byName)
		b, _ := json.Marshal([]any{r.Fields, r.Amounts})
		out = append(out, "row "+r.RecordType+" "+r.MatchKey+" "+string(b))
	}
	slices.Sort(out)
	return out
}

func readSampleExpected(t *testing.T, name string, data []byte) sampleExpected {
	t.Helper()
	var e sampleExpected
	if _, err := strictyaml.Decode(name, data, &e); err != nil {
		t.Fatal(err)
	}
	return e
}

// writeReordered writes a copy of a workbook with its sheets in reverse order and, on every sheet
// but those listed in keep, the rows under the header (row 5) in reverse order. It proves the
// format finds each figure by its conditions, never by its place. Each cell keeps its value as
// written (numbers as their text, never through floating point) and its style, so a date stays
// a date.
func writeReordered(t *testing.T, src, dst string, keep ...string) {
	t.Helper()
	f, err := excelize.OpenFile(src)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	const header = 5
	type cellCopy struct {
		value  string
		number bool
		style  int
	}
	sheets := f.GetSheetList()
	for _, sh := range sheets {
		if slices.Contains(keep, sh) {
			continue
		}
		rows, err := f.GetRows(sh, excelize.Options{RawCellValue: true})
		must(err)
		var data [][]cellCopy // the rows under the header, each as wide as the widest
		width := 0
		for _, r := range rows[header:] {
			width = max(width, len(r))
		}
		for r := header + 1; r <= len(rows); r++ {
			var row []cellCopy
			for c := 1; c <= width; c++ {
				ref := cellRef(c, r)
				v, err := f.GetCellValue(sh, ref, excelize.Options{RawCellValue: true})
				must(err)
				typ, err := f.GetCellType(sh, ref)
				must(err)
				st, err := f.GetCellStyle(sh, ref)
				must(err)
				row = append(row, cellCopy{v, typ == excelize.CellTypeNumber || typ == excelize.CellTypeUnset && v != "", st})
			}
			data = append(data, row)
		}
		slices.Reverse(data)
		for i, row := range data {
			for c, cc := range row {
				ref := cellRef(c+1, header+1+i)
				switch {
				case cc.value == "":
					must(f.SetCellValue(sh, ref, nil))
				case cc.number:
					must(f.SetCellDefault(sh, ref, cc.value))
				default:
					must(f.SetCellStr(sh, ref, cc.value))
				}
				must(f.SetCellStyle(sh, ref, ref, cc.style))
			}
		}
	}
	for i := len(sheets) - 1; i > 0; i-- { // the last sheet first, and so on
		must(f.MoveSheet(sheets[i], sheets[0]))
	}
	saveBook(t, f, dst)
}

// SPEC-011 part 2 (AC-6): each workbook sample is read by its own Excel format, and what it reads
// is, figure for figure and row for row, what the JSON twin reads from the JSON pack (its
// expected file), apart from the places. -update writes each format's expected file.
func TestTestPackAWorkbookSamples(t *testing.T) {
	v, err := vocab.LoadDir(vocabDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := profile.LoadDir(profDir, v)
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range []string{"tax_audit_report", "mat_report", "form_67", "computation", "form_26as", "financial_statements"} {
		dir := filepath.Join(profDir, "taxsuite_xlsx", doc, "v1")
		if *update {
			// The second sample: the same workbook with its sheets and rows in another order. The
			// Computation sheet keeps its order: its rows sit under their headings.
			writeReordered(t, filepath.Join(dir, "samples", "nimbus_"+doc+".xlsx"), filepath.Join(dir, "samples", "nimbus_"+doc+"_reordered.xlsx"), "Computation")
		}
		for _, name := range []string{"nimbus_" + doc, "nimbus_" + doc + "_reordered"} {
			sample := filepath.Join(dir, "samples", name+".xlsx")
			p, got := readSample(t, set, sample)
			if p.ID != "taxsuite.xlsx."+doc {
				t.Fatalf("%s was read by %s", sample, p.Key())
			}
			body := profile.ExpectedFile(p, got)
			if _, rest, ok := bytes.Cut(body, []byte("\n")); ok {
				body = append([]byte("# What reading the sample gives (SPEC-011 part 2): written by go test ./cmd/recon-engine -run TestTestPackAWorkbookSamples -update\n# and checked against the JSON filing pack's expected file for the same document on every test run.\n"), rest...)
			}
			expected := filepath.Join(dir, "expected", name+".facts.yaml")
			if *update {
				if err := os.WriteFile(expected, body, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			twinData, err := os.ReadFile(filepath.Join(profDir, "taxsuite_pack", doc, "v1", "expected", "nimbus_pack.facts.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			xlsx, twin := readSampleExpected(t, expected, body), readSampleExpected(t, "twin", twinData)
			if g, w := xlsx.lines(), twin.lines(); !slices.Equal(g, w) || len(g) == 0 {
				t.Errorf("%s reads %d figures and rows, the JSON twin %d; they differ:\nxlsx %q\njson %q", doc, len(g), len(w), g, w)
			}
			if xlsx.Context["tax_year"] != twin.Context["tax_year"] || xlsx.Context["period"] != twin.Context["period"] {
				t.Errorf("%s context %v, the JSON twin %v", doc, xlsx.Context, twin.Context)
			}
			for _, f := range xlsx.Facts {
				// A sum names the rows it adds up, as the JSON format does; every other figure its cell.
				if !strings.Contains(f.Locator, "!") && !strings.Contains(f.Locator, "(sum of ") {
					t.Errorf("%s: %s at %q, want a sheet cell", doc, f.Fact, f.Locator)
				}
			}
		}
	}
}

// SPEC-011 part 2 (AC-6, AC-7): Test Pack A's six workbooks, one per document (fictional Nimbus
// Logic Systems), are each recognised by their own Excel format and, uploaded together, give the
// same report as the JSON filing pack, apart from where each figure is: the same checks, the
// same findings with the same values and differences.
func TestTestPackAWorkbooksGiveTheJSONFindings(t *testing.T) {
	var books []string
	for _, n := range []string{"01_Form_3CD", "02_Form_29B", "03_Form_67", "04_Computation", "05_Form_26AS", "06_Financial_Statements"} {
		books = append(books, filepath.Join(testPack, n+".xlsx"))
	}
	rj, fromJSON := reportFor(t, "2025-26", "taxsuite_filing_pack", filepath.Join(testPack, "09_filing_pack_structured.json"))
	rx, fromBooks := reportFor(t, "2025-26", "taxsuite_filing_pack", books...)
	if rx.code != rj.code || rj.code != exitFindings {
		t.Fatalf("workbooks: exit %d, the JSON %d\n%s", rx.code, rj.code, rx.stderr)
	}
	want, _ := json.Marshal(withoutPlaces(fromJSON))
	got, _ := json.Marshal(withoutPlaces(fromBooks))
	if !bytes.Equal(got, want) {
		t.Errorf("the workbooks' report differs from the JSON's beyond the places:\nJSON %.800s\nxlsx %.800s", want, got)
	}

	// Each workbook is read by its own format, and each figure names its cell.
	raw, _ := json.Marshal(fromBooks)
	for _, s := range []string{
		`"profile_id":"taxsuite.xlsx.tax_audit_report"`, `"profile_id":"taxsuite.xlsx.mat_report"`,
		`"profile_id":"taxsuite.xlsx.form_67"`, `"profile_id":"taxsuite.xlsx.computation"`,
		`"profile_id":"taxsuite.xlsx.form_26as"`, `"profile_id":"taxsuite.xlsx.financial_statements"`,
		`"kind":"sheet_cell","path":"Form 29B!B`, `"kind":"sheet_cell","path":"Computation!B`,
	} {
		if !strings.Contains(string(raw), s) {
			t.Errorf("the workbooks' report lacks %s", s)
		}
	}
}
