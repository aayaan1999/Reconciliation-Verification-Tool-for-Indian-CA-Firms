package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
)

var storeRoot = filepath.Join("..", "..", rsDefaultStore)

// The shipped rule store loads strictly (RU-02) and passes every check.
func TestShippedRuleStore(t *testing.T) {
	rules, docs, problems := checkStore(storeRoot)
	if rules != 304 || docs != 63 {
		t.Errorf("checked %d rules and %d documents, want 304 and 63", rules, docs)
	}
	for _, p := range problems {
		t.Error(p)
	}
}

// The templates CAs copy from load through the strict reader too.
func TestTemplatesLoadStrictly(t *testing.T) {
	for file, out := range map[string]any{"RULE_TEMPLATE.yaml": &rsRule{}, "DOCUMENT_TEMPLATE.yaml": &rsDocFile{}} {
		data, err := os.ReadFile(filepath.Join(storeRoot, file))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := strictyaml.Decode(file, data, out); err != nil {
			t.Errorf("%s: %v", file, err)
		}
	}
}

func TestCheckRefuses(t *testing.T) {
	src := filepath.Join(storeRoot, "rules", "reconciliation", "gst", "RC-GST-001.yaml")
	good, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, old, new, want string }{
		{"unquoted version", `version: "1"`, `version: 1`, "version is text and must be in quotes"},
		{"enabled without review", `enabled: false`, `enabled: true`, "enabled without a CA reviewer"},
		{"unknown document", `doc_code: "SALES_REG"`, `doc_code: "SALES_REGX"`, "document SALES_REGX has no file"},
		{"bad domain", `domain: "gst"`, `domain: "vat"`, `domain "vat" is not one of`},
		{"unknown key", `notes:`, "author: \"x\"\nnotes:", `unknown key "author"`},
	} {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(string(good), c.old) {
				t.Fatalf("setup: %q not in RC-GST-001.yaml", c.old)
			}
			dir := t.TempDir()
			copyDir(t, filepath.Join(storeRoot, "documents"), filepath.Join(dir, "documents"))
			dst := filepath.Join(dir, "rules", "reconciliation", "gst", "RC-GST-001.yaml")
			_ = os.MkdirAll(filepath.Dir(dst), 0o755)
			_ = os.WriteFile(dst, []byte(strings.Replace(string(good), c.old, c.new, 1)), 0o644)
			_, _, problems := checkStore(dir)
			if !strings.Contains(strings.Join(problems, "\n"), c.want) {
				t.Errorf("problems %q do not mention %q", problems, c.want)
			}
		})
	}
}

func TestCheckParam(t *testing.T) {
	for _, c := range []struct {
		p    rsParam
		want string
	}{
		{rsParam{Key: "a", Value: "0.18", Type: "number", Max: "0.15"}, "above max 0.15"},
		{rsParam{Key: "a", Value: "-1", Type: "integer", Min: "0"}, "below min 0"},
		{rsParam{Key: "a", Value: "1.5", Type: "integer"}, "not of declared type integer"},
		{rsParam{Key: "a", Value: "yes", Type: "boolean"}, "not of declared type boolean"},
		{rsParam{Key: "a", Value: "x", Type: "float"}, `type "float" is not one of`},
		{rsParam{Key: "a", Value: "c", Type: "string", AllowedValues: []string{"a", "b"}}, "not in allowed_values"},
		{rsParam{Key: "a", Value: "0.10", Type: "number", Min: "0.1", Max: "0.1"}, ""},
	} {
		var got []string
		checkParam(func(f string, a ...any) { got = append(got, strings.TrimSpace(sprintf(f, a...))) }, "R", c.p)
		if (c.want == "") != (len(got) == 0) || (c.want != "" && !strings.Contains(strings.Join(got, ";"), c.want)) {
			t.Errorf("%+v: got %q, want %q", c.p, got, c.want)
		}
	}
}

func TestImportHelpers(t *testing.T) {
	if got := splitItems("A=1; b: x; y; C=2", paramStart); !reflect.DeepEqual(got, []string{"A=1; b: x; y", "C=2"}) {
		t.Errorf("splitItems = %q", got)
	}
	if got := splitSteps("Intro 1. load 2. match 3) report"); !reflect.DeepEqual(got, []string{"Intro", "load", "match", "report"}) {
		t.Errorf("splitSteps = %q", got)
	}
	if got := splitSteps("only 1. one"); !reflect.DeepEqual(got, []string{"only 1. one"}) {
		t.Errorf("splitSteps single = %q", got)
	}
	if got := fieldRefs("GL.abs(x) = PURCH_REG.party_gstin, GSTR2B.isd_* and BOOK.date-x"); !reflect.DeepEqual(got, [][2]string{{"PURCH_REG", "party_gstin"}}) {
		t.Errorf("fieldRefs = %q", got)
	}
	for in, want := range map[string][2]string{"Yes": {"true", "boolean"}, "15": {"15", "integer"}, "007": {"007", "string"}, "0.18": {"0.18", "number"}, "abc": {"abc", "string"}} {
		if v, ty := typedParam(in); v != want[0] || ty != want[1] {
			t.Errorf("typedParam(%q) = %q, %q", in, v, ty)
		}
	}
	cats := parseCategories("MATCHED: all ok; MISSING_IN_2B: not found", "MATCHED = Low; MISSING_IN_2B = Critical.; EXTRA = High")
	want := []rsCategory{{"MATCHED", "all ok", "Low", ""}, {"MISSING_IN_2B", "not found", "Critical", ""}, {"EXTRA", "", "High", ""}}
	if !reflect.DeepEqual(cats, want) {
		t.Errorf("parseCategories = %+v", cats)
	}
	src := parseSources("CBIC | https://cbic-gst.gov.in/x | Rule 36\nBook | p.4")
	if !reflect.DeepEqual(src, []rsSource{{"CBIC", "https://cbic-gst.gov.in/x", "Rule 36", true}, {"Book", "", "p.4", false}}) {
		t.Errorf("parseSources = %+v", src)
	}
	if got := cleanFieldNames("Invoice No / Date"); !reflect.DeepEqual(got, []string{"invoice_no", "date"}) {
		t.Errorf("cleanFieldNames = %q", got)
	}
}

func copyDir(t *testing.T, from, to string) {
	t.Helper()
	_ = os.MkdirAll(to, 0o755)
	files, _ := filepath.Glob(filepath.Join(from, "*"))
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		_ = os.WriteFile(filepath.Join(to, filepath.Base(f)), b, 0o644)
	}
}

func sprintf(f string, a ...any) string { return fmt.Sprintf(f, a...) }
