package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// SPEC-011: the tax suite filing pack saved as CSV or Excel, one value per row (columns Path
// and Value), is read by the six JSON formats of the pack and gives the same report as the
// JSON, apart from where each figure is. With -update the CSV and Excel files are written
// again from the JSON packs.

var manualPack = filepath.Join(repo, "testdata", "manual_filing_pack")

// A flatRow is one value of a JSON file: its path and the value as JSON decoded it
// (json.Number, string, bool, nil, or "[]" / "{}" for an empty list or object).
type flatRow struct {
	path  string
	value any
}

var plainKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// flatten lists every value of the JSON file in file order, with its path.
func flatten(t *testing.T, file string) []flatRow {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var rows []flatRow
	var walk func(path string) error
	walk = func(path string) error {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch tok {
		case json.Delim('{'):
			n := 0
			for ; dec.More(); n++ {
				k, err := dec.Token()
				if err != nil {
					return err
				}
				key := k.(string)
				if plainKey.MatchString(key) {
					err = walk(path + "." + key)
				} else {
					err = walk(path + "['" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(key) + "']")
				}
				if err != nil {
					return err
				}
			}
			if n == 0 && path != "$" {
				rows = append(rows, flatRow{path, "{}"})
			}
			_, err = dec.Token()
			return err
		case json.Delim('['):
			n := 0
			for ; dec.More(); n++ {
				if err := walk(path + "[" + strconv.Itoa(n) + "]"); err != nil {
					return err
				}
			}
			if n == 0 {
				rows = append(rows, flatRow{path, "[]"})
			}
			_, err = dec.Token()
			return err
		}
		rows = append(rows, flatRow{path, tok})
		return nil
	}
	if err := walk("$"); err != nil {
		t.Fatal(err)
	}
	return rows
}

// text is how a value is written in the Value column.
func text(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		if x {
			return "true"
		}
		return "false"
	case json.Number:
		return x.String()
	}
	return v.(string)
}

func writeFlatCSV(t *testing.T, rows []flatRow, file string) {
	t.Helper()
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.UseCRLF = true
	if err := w.Write([]string{"Path", "Value"}); err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if err := w.Write([]string{r.path, text(r.value)}); err != nil {
			t.Fatal(err)
		}
	}
	w.Flush()
	if err := os.WriteFile(file, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeFlatBook saves rows as a workbook with one sheet, Filing pack: numbers as numbers,
// true and false as booleans, text as text.
func writeFlatBook(t *testing.T, rows []flatRow, file string) {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	const sh = "Filing pack"
	must(f.SetSheetName("Sheet1", sh))
	must(f.SetCellStr(sh, "A1", "Path"))
	must(f.SetCellStr(sh, "B1", "Value"))
	must(f.SetColWidth(sh, "A", "A", 70))
	must(f.SetColWidth(sh, "B", "B", 40))
	for i, r := range rows {
		ref := cellRef(2, i+2)
		must(f.SetCellStr(sh, cellRef(1, i+2), r.path))
		switch v := r.value.(type) {
		case json.Number:
			must(f.SetCellDefault(sh, ref, v.String()))
		case bool:
			must(f.SetCellBool(sh, ref, v))
		case string:
			if v != "" {
				must(f.SetCellStr(sh, ref, v))
			}
		default:
			must(f.SetCellStr(sh, ref, text(v)))
		}
	}
	saveBook(t, f, file)
}

func TestFlatFilingPacks(t *testing.T) {
	for _, name := range []string{"01_filing_pack_as_received", "02_filing_pack_audit_report_and_29B_fixed", "03_filing_pack_all_fixed"} {
		source := filepath.Join(manualPack, name+".json")
		if *update {
			rows := flatten(t, source)
			writeFlatCSV(t, rows, filepath.Join(manualPack, name+".csv"))
			writeFlatBook(t, rows, filepath.Join(manualPack, name+".xlsx"))
		}
		rj, fromJSON := reportFor(t, "2025-26", "taxsuite_filing_pack", source)
		want, _ := json.Marshal(withoutPlaces(fromJSON))
		for ext, place := range map[string]string{".csv": `"kind":"line","path":"line `, ".xlsx": `"kind":"sheet_cell","path":"Filing pack!B`} {
			rf, fromFlat := reportFor(t, "2025-26", "taxsuite_filing_pack", filepath.Join(manualPack, name+ext))
			if rf.code != rj.code {
				t.Fatalf("%s%s: exit %d, the JSON %d\n%s", name, ext, rf.code, rj.code, rf.stderr)
			}
			got, _ := json.Marshal(withoutPlaces(fromFlat))
			if !bytes.Equal(got, want) {
				t.Errorf("%s%s: the report differs from the JSON's beyond the places:\nJSON %.600s\nflat %.600s", name, ext, want, got)
			}
			if raw, _ := json.Marshal(fromFlat); rj.code == exitFindings && !strings.Contains(string(raw), place) {
				t.Errorf("%s%s: no figure names its place as %s…: %.300s", name, ext, place, raw)
			}
		}
	}
}

// A flattened file that contradicts itself is refused, naming the row.
func TestFlatFileConflicts(t *testing.T) {
	for body, want := range map[string]string{
		"Path,Value\n$.a,1\n$.a,2\n":        "line 3",
		"Path,Value\n$.a,1\n$.a.b,2\n":      "line 3",
		"Path,Value\n$.l[1],1\n":            "skips a list number",
		"Path,Value\n$.l[*],1\n":            "without [*]",
		"Path,Value\nform_3cd.clause_8,x\n": "must start with $",
	} {
		in := t.TempDir()
		if err := os.WriteFile(filepath.Join(in, "pack.csv"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		r := runCmd(t, newDB(t), in, "--include-testing")
		if r.code != exitInput || !strings.Contains(r.stderr, want) {
			t.Errorf("%q: exit %d, want %q in\n%s", body, r.code, want, r.stderr)
		}
	}
}
