package vocab

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestExtraAccessorsReturnCopies(t *testing.T) {
	v := mustLoad(t, goodFacts, goodRecords)
	fs := v.Facts()
	fs[0].Name = "mutated"
	if v.Facts()[0].Name == "mutated" || v.IsFact("mutated") {
		t.Error("Facts() shares its slice")
	}
	ds := v.DocumentTypes()
	ds[0].Name = "mutated"
	if v.DocumentTypes()[0].Name == "mutated" {
		t.Error("DocumentTypes() shares its slice")
	}
	rs := v.RecordTypes()
	rs[0].Fields[0], rs[0].Amounts[0], rs[0].MatchKey[0] = "x", "x", "x"
	rs[0].Name = "mutated"
	r, _ := v.RecordType("gst_invoice")
	if r.Fields[0] == "x" || r.Amounts[0] == "x" || r.MatchKey[0] == "x" || v.RecordTypes()[0].Name == "mutated" {
		t.Error("RecordTypes() shares its slices")
	}
	if _, ok := v.Lookup(""); ok {
		t.Error("empty name found")
	}
	if _, ok := v.Lookup("TAR.depreciation_schedule.total"); ok {
		t.Error("lookup is case sensitive")
	}
	if _, ok := v.RecordType("nope"); ok {
		t.Error("unknown record found")
	}
	if _, ok := v.DocumentType("nope"); ok {
		t.Error("unknown doc type found")
	}
}

// The committed Markdown must describe exactly what the YAML holds, row by row.
func TestExtraDocAgreesWithYAML(t *testing.T) {
	v, err := LoadDir(shippedDir)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(shippedDoc)
	if err != nil {
		t.Fatal(err)
	}
	doc := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if !strings.Contains(doc, "Vocabulary version: "+strconv.Itoa(v.Version())+"\n") {
		t.Error("version line does not match facts.yaml")
	}
	if !strings.Contains(doc, "Content hash: `"+v.ContentHash()+"`\n") {
		t.Error("content hash line does not match")
	}
	rows := 0
	for _, line := range strings.Split(doc, "\n") {
		if !strings.HasPrefix(line, "| `") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "| "), " | ")
		name := strings.Trim(cells[0], "`")
		if f, ok := v.Lookup(name); ok {
			rows++
			if cells[1] != f.Kind.String() || cells[2] != string(f.Periodicity) || cells[4] != strconv.Itoa(f.SinceVocab) {
				t.Errorf("row for %s disagrees with the YAML: %v vs %+v", name, cells, f)
			}
			if strings.ReplaceAll(cells[3], `\|`, "|") != f.Description {
				t.Errorf("description of %s differs", name)
			}
		}
	}
	if rows != len(v.Facts()) {
		t.Errorf("document lists %d facts, YAML has %d", rows, len(v.Facts()))
	}
	for _, d := range v.DocumentTypes() {
		if !strings.Contains(doc, "| `"+d.Name+"` | `"+d.Prefix+".` |") {
			t.Errorf("document type %s missing from document", d.Name)
		}
	}
	for _, f := range v.Facts() {
		if got := strings.Count(doc, "`"+f.Name+"`"); got != 1 {
			t.Errorf("fact %s appears %d times in the document", f.Name, got)
		}
	}
	low := strings.ToLower(doc)
	if strings.Contains(low, "should") || strings.Contains(low, "must be") || strings.Contains(low, "incorrect") {
		t.Error("generated document uses forbidden wording")
	}
	if !strings.HasSuffix(doc, "\n") || strings.HasSuffix(doc, "\n\n") {
		t.Error("document does not end in exactly one newline")
	}
}

func TestExtraCheckDocEdges(t *testing.T) {
	v := mustLoad(t, goodFacts, goodRecords)
	good := Markdown(v)
	newer := strings.Replace(string(good), "Vocabulary version: 2", "Vocabulary version: 5", 1)
	changed := mustLoad(t, strings.Replace(goodFacts, `"A text fact"`, `"Other"`, 1), goodRecords)
	if err := CheckDoc([]byte(newer), changed); err == nil || !strings.Contains(err.Error(), "increase vocab_version") {
		t.Errorf("version went backwards: err = %v", err)
	}
	for name, committed := range map[string]string{"empty": "", "garbage": "hello", "no hash": "Vocabulary version: 1\n"} {
		err := CheckDoc([]byte(committed), v)
		if err == nil || !strings.Contains(err.Error(), "regenerate") {
			t.Errorf("%s: err = %v", name, err)
		}
		if err := CheckVersionBumped([]byte(committed), v); err != nil {
			t.Errorf("%s: CheckVersionBumped = %v", name, err)
		}
	}
	edited := strings.Replace(string(good), "Fact vocabulary", "Fact vocabulary!", 1)
	if err := CheckDoc([]byte(edited), v); err == nil || !strings.Contains(err.Error(), "regenerate") {
		t.Errorf("hand edit: err = %v", err)
	}
	forged := strings.Replace(string(good), "Fact names start with", "Names start with", 1)
	if err := CheckDoc([]byte(forged), v); err == nil {
		t.Error("body edit with intact header accepted")
	}
}
