package vocab

import (
	"strings"
	"testing"
)

func bumpBoth(f, r, to string) (string, string) {
	return strings.Replace(f, "vocab_version: 2", "vocab_version: "+to, 1), strings.Replace(r, "vocab_version: 2", "vocab_version: "+to, 1)
}

func TestExtraVersions(t *testing.T) {
	for _, v := range []string{"0", "-1", "-0"} {
		t.Run("vocab_version "+v, func(t *testing.T) {
			f, r := bumpBoth(goodFacts, goodRecords, v)
			_, err := load(t, f, r)
			if err == nil || !strings.Contains(err.Error(), "vocab_version must be 1 or more") {
				t.Errorf("err = %v", err)
			}
		})
	}
	t.Run("vocab_version large but valid", func(t *testing.T) {
		f, r := bumpBoth(goodFacts, goodRecords, "9223372036854775807")
		if v, err := load(t, f, r); err != nil || v.Version() != 9223372036854775807 {
			t.Errorf("err=%v", err)
		}
	})
	t.Run("vocab_version missing", func(t *testing.T) {
		if _, err := load(t, strings.Replace(goodFacts, "vocab_version: 2\n", "", 1), goodRecords); err == nil {
			t.Error("accepted a file without vocab_version")
		}
	})
	t.Run("vocab_version quoted or float", func(t *testing.T) {
		for _, s := range []string{`"2"`, "2.0", "0x2", "02"} {
			if _, err := load(t, strings.Replace(goodFacts, "vocab_version: 2", "vocab_version: "+s, 1), goodRecords); err == nil {
				t.Errorf("vocab_version: %s accepted", s)
			}
		}
	})
	t.Run("since_vocab 0, negative, missing, past version", func(t *testing.T) {
		for _, s := range []string{"0", "-1", "3"} {
			if _, err := load(t, strings.Replace(goodFacts, "    since_vocab: 2\n", "    since_vocab: "+s+"\n", 1), goodRecords); err == nil {
				t.Errorf("since_vocab %s accepted", s)
			}
		}
		if _, err := load(t, strings.Replace(goodFacts, "    since_vocab: 2\n", "", 1), goodRecords); err == nil {
			t.Error("missing since_vocab accepted")
		}
	})
	t.Run("deprecated_in boundaries", func(t *testing.T) {
		// the deprecated fact has since 1, version 2
		cases := map[string]bool{"2": true, "1": false, "3": false, "0": true, "-1": false}
		for dep, ok := range cases {
			_, err := load(t, strings.Replace(goodFacts, "deprecated_in: 2", "deprecated_in: "+dep, 1), goodRecords)
			if (err == nil) != ok {
				t.Errorf("deprecated_in %s: err=%v want ok=%v", dep, err, ok)
			}
		}
	})
	t.Run("deprecated_in 0 means not deprecated", func(t *testing.T) {
		v := mustLoad(t, strings.Replace(goodFacts, "deprecated_in: 2", "deprecated_in: 0", 1), goodRecords)
		if f, _ := v.Lookup("computation.allowed_depreciation"); f.Deprecated() {
			t.Error("deprecated_in 0 reported as deprecated")
		}
	})
	t.Run("record versions are checked too", func(t *testing.T) {
		for _, s := range []string{"since_vocab: 3", "since_vocab: 0", "since_vocab: 2\n    deprecated_in: 2", "since_vocab: 1\n    deprecated_in: 3"} {
			if _, err := load(t, goodFacts, strings.Replace(goodRecords, "since_vocab: 2", s, 1)); err == nil {
				t.Errorf("%q accepted", s)
			}
		}
		v := mustLoad(t, goodFacts, strings.Replace(goodRecords, "since_vocab: 2", "since_vocab: 1\n    deprecated_in: 2", 1))
		if r, _ := v.RecordType("gst_invoice"); !r.Deprecated() {
			t.Error("record type not deprecated")
		}
	})
	t.Run("the two files must agree on version both ways", func(t *testing.T) {
		f, _ := bumpBoth(goodFacts, goodRecords, "3")
		if _, err := load(t, f, goodRecords); err == nil || !strings.Contains(err.Error(), "vocab_version is 2 here but 3 in facts.yaml") {
			t.Errorf("err = %v", err)
		}
	})
}

func TestExtraDescriptions(t *testing.T) {
	cases := []struct {
		name, desc string
		ok         bool
	}{
		{"plain", `"x"`, true}, {"unicode", `"₹ Rs. figure"`, true}, {"leading and trailing spaces", `"  x  "`, true},
		{"markdown characters", "\"`a` *b* [c](d)\"", true}, {"long", `"` + strings.Repeat("word ", 2000) + `"`, true},
		{"empty", `""`, false}, {"spaces only", `"   "`, false}, {"tab only", `"\t"`, false},
		{"newline escape", `"a\nb"`, false}, {"carriage return escape", `"a\rb"`, false},
		{"unquoted", `plain text`, false}, {"block scalar", "|\n      two\n      lines", false},
		{"folded scalar", ">\n      two\n      lines", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := strings.Replace(goodFacts, `description: "A text fact"`, "description: "+c.desc, 1)
			_, err := load(t, f, goodRecords)
			if (err == nil) != c.ok {
				t.Errorf("ok=%v err=%v", c.ok, err)
			}
		})
	}
	t.Run("document type and record descriptions are required", func(t *testing.T) {
		if _, err := load(t, strings.Replace(goodFacts, `description: "Tax audit report"`, `description: ""`, 1), goodRecords); err == nil {
			t.Error("empty document type description accepted")
		}
		if _, err := load(t, goodFacts, strings.Replace(goodRecords, `description: "Invoice line"`, `description: ""`, 1)); err == nil {
			t.Error("empty record description accepted")
		}
	})
}
