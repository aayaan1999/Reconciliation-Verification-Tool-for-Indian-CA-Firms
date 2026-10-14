package vocab

import (
	"reflect"
	"sort"
	"testing"
)

// FuzzLoad: Load never panics. Whenever it accepts the input, the result is sorted,
// deterministic, renders a document that CheckDoc accepts, and every fact is findable.
func FuzzLoad(f *testing.F) {
	f.Add([]byte(goodFacts), []byte(goodRecords))
	f.Add([]byte(""), []byte(""))
	f.Add([]byte("schema: \"recon.vocab.facts/1\"\nvocab_version: 1\n"), []byte("schema: \"recon.vocab.records/1\"\nvocab_version: 1\n"))
	f.Add([]byte(goodFacts+"  - name: \"tar.x\"\n"), []byte(goodRecords+"  - name: 1\n"))
	f.Add([]byte("\xef\xbb\xbf"+goodFacts), []byte("\xff"))
	f.Fuzz(func(t *testing.T, facts, records []byte) {
		v, err := Load("f.yaml", facts, "r.yaml", records)
		if err != nil {
			if v != nil {
				t.Fatal("value returned with an error")
			}
			return
		}
		v2, err2 := Load("f.yaml", facts, "r.yaml", records)
		if err2 != nil || v.ContentHash() != v2.ContentHash() || string(Markdown(v)) != string(Markdown(v2)) {
			t.Fatal("second load differs")
		}
		fs := v.Facts()
		if !sort.SliceIsSorted(fs, func(i, j int) bool { return fs[i].Name < fs[j].Name }) {
			t.Fatal("facts not sorted")
		}
		for _, x := range fs {
			if got, ok := v.Lookup(x.Name); !ok || !reflect.DeepEqual(got, x) {
				t.Fatalf("lookup %q failed", x.Name)
			}
			if !factName.MatchString(x.Name) || x.SinceVocab < 1 || x.SinceVocab > v.Version() {
				t.Fatalf("invalid fact accepted: %+v", x)
			}
		}
		for _, r := range v.RecordTypes() {
			if len(r.MatchKey) == 0 || !simpleName.MatchString(r.Name) {
				t.Fatalf("invalid record accepted: %+v", r)
			}
		}
		if v.Version() < 1 {
			t.Fatal("version < 1 accepted")
		}
		if err := CheckDoc(Markdown(v), v); err != nil {
			t.Fatalf("own document refused: %v", err)
		}
	})
}

// FuzzCheckDoc: CheckDoc and CheckVersionBumped never panic on arbitrary committed text.
func FuzzCheckDoc(f *testing.F) {
	v, err := Load("f.yaml", []byte(goodFacts), "r.yaml", []byte(goodRecords))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(Markdown(v))
	f.Add([]byte("Vocabulary version: 99999999999999999999\n\nContent hash: `" + v.ContentHash() + "`\n"))
	f.Add([]byte(""))
	f.Fuzz(func(t *testing.T, doc []byte) {
		e1, e2 := CheckDoc(doc, v), CheckDoc(doc, v)
		if (e1 == nil) != (e2 == nil) {
			t.Fatal("not deterministic")
		}
		_ = CheckVersionBumped(doc, v)
	})
}
