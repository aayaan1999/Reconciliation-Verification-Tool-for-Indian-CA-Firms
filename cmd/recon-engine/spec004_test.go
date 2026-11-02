package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// The acceptance tests of SPEC-004 parts 1 and 2: client JSON files read to the paisa,
// refused when damaged, unknown or ambiguous, and every figure stored with its source.

type expectedFile struct {
	Context struct {
		TaxYear string `yaml:"tax_year"`
	} `yaml:"context"`
	Facts []struct {
		Fact    string `yaml:"fact"`
		Value   string `yaml:"value"`
		Period  string `yaml:"period"`
		Scope   string `yaml:"scope"`
		Locator string `yaml:"locator"`
	} `yaml:"facts"`
}

type factsJSON struct {
	Engagements []struct {
		TaxYear   string `json:"tax_year"`
		Documents []struct {
			FileName string `json:"file_name"`
			Facts    []struct {
				Name   string `json:"name"`
				Period string `json:"period"`
				Scope  string `json:"scope"`
				Value  struct {
					Kind   string      `json:"kind"`
					Paise  json.Number `json:"paise"`
					Text   string      `json:"text"`
					Int    json.Number `json:"int"`
					Date   string      `json:"date"`
					Bool   bool        `json:"bool"`
					RateBP int64       `json:"rate_bp"`
				} `json:"value"`
				Locator struct {
					FileName string `json:"file_name"`
					Path     string `json:"path"`
				} `json:"locator"`
			} `json:"facts"`
		} `json:"documents"`
	} `json:"engagements"`
}

// AC-4: for every profile sample, the figures read back from the database equal the
// hand-checked expected facts, including the place each was found.
func TestEverySampleRoundTripsThroughTheDatabase(t *testing.T) {
	v, err := vocab.LoadDir(vocabDir)
	if err != nil {
		t.Fatal(err)
	}
	samples, _ := filepath.Glob(filepath.Join(profDir, "*", "*", "v*", "samples", "*.json"))
	if len(samples) < 2 {
		t.Fatalf("found %d profile samples", len(samples))
	}
	db := newDB(t)
	for _, sample := range samples {
		t.Run(filepath.Base(sample), func(t *testing.T) {
			base := strings.TrimSuffix(filepath.Base(sample), ".json")
			expPath := filepath.Join(filepath.Dir(filepath.Dir(sample)), "expected", base+".facts.yaml")
			raw, err := os.ReadFile(expPath)
			if err != nil {
				t.Fatal(err)
			}
			var exp expectedFile
			if err := yaml.Unmarshal(raw, &exp); err != nil {
				t.Fatal(err)
			}
			input := t.TempDir()
			copyTree(t, filepath.Dir(sample), input)
			for _, other := range mustGlob(t, filepath.Join(input, "*")) {
				if filepath.Base(other) != filepath.Base(sample) {
					os.Remove(other)
				}
			}
			if r := runCmd(t, db, input, "--tax-year", exp.Context.TaxYear); r.code > exitFindings {
				t.Fatalf("run: exit %d\n%s", r.code, r.stderr)
			}
			jsonFile := filepath.Join(t.TempDir(), "facts.json")
			if r := cmd(t, "facts", "--db", db, "--input", input, "--json", jsonFile); r.code != exitOK {
				t.Fatalf("facts: exit %d\n%s", r.code, r.stderr)
			}
			// A sample of a bundle (SPEC-005) gives several documents; this profile's expected
			// figures are those of its own document type (the profile's folder).
			docType := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(sample))))
			var got []string
			for _, l := range readFactLines(t, jsonFile) {
				name, _, _ := strings.Cut(l, " | ")
				if vf, ok := v.Lookup(name); ok && vf.DocumentType == docType {
					got = append(got, l)
				}
			}
			var want []string
			for _, f := range exp.Facts {
				vf, ok := v.Lookup(f.Fact)
				if !ok {
					t.Fatalf("expected fact %s is not in the vocabulary", f.Fact)
				}
				val, err := domain.ParseValue(vf.Kind, f.Value)
				if err != nil {
					t.Fatal(err)
				}
				want = append(want, line(f.Fact, f.Period, f.Scope, valueKey(val), filepath.Base(sample), f.Locator))
			}
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Errorf("stored figures:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
			}
		})
	}
}

func line(parts ...string) string { return strings.Join(parts, " | ") }

func valueKey(v domain.Value) string {
	if v.Kind == domain.KindMoney {
		return "money:" + strconv.FormatInt(int64(v.Money), 10)
	}
	return v.Kind.String() + ":" + domain.FormatValue(v)
}

func readFactLines(t *testing.T, file string) []string {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var fj factsJSON
	if err := dec.Decode(&fj); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range fj.Engagements {
		for _, d := range e.Documents {
			for _, f := range d.Facts {
				val := f.Value.Kind + ":" + f.Value.Text
				switch f.Value.Kind {
				case "money":
					val = "money:" + f.Value.Paise.String()
				case "int":
					val = "int:" + f.Value.Int.String()
				case "date":
					val = "date:" + f.Value.Date
				case "bool":
					val = "bool:" + strconv.FormatBool(f.Value.Bool)
				case "rate":
					val = "rate:" + domain.FormatValue(domain.Value{Kind: domain.KindRate, RateBP: f.Value.RateBP})
				}
				out = append(out, line(f.Name, f.Period, f.Scope, val, f.Locator.FileName, f.Locator.Path))
			}
		}
	}
	slices.Sort(out)
	return out
}

func mustGlob(t *testing.T, pattern string) []string {
	t.Helper()
	m, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// AC-2: amounts are read to the paisa with no rounding; three decimals or a non-number is
// a validation error at that place and gives no figure; a figure written twice is refused.
func TestAmountsAreExact(t *testing.T) {
	sample, err := os.ReadFile(filepath.Join(runData, "clean", "comp_2025-26_agrees.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, old, new string
		code           int
		want           string
	}{
		{"three decimals", `"allowed_depreciation": 112450.50`, `"allowed_depreciation": 112450.505`, exitOK,
			"PARSE_ERROR at $.business_income.allowed_depreciation: computation.allowed_depreciation at $.business_income.allowed_depreciation: money: cannot read \"112450.505\" as an amount: more than two decimal places"},
		{"not a number", `"allowed_depreciation": 112450.50`, `"allowed_depreciation": "abc"`, exitOK,
			"PARSE_ERROR at $.business_income.allowed_depreciation"},
		{"written twice", `"schedule_total": 112450.50`, `"schedule_total": 112450.50, "schedule_total": 99.00`, exitInput,
			`the key "schedule_total" appears twice`},
	} {
		t.Run(c.name, func(t *testing.T) {
			if !bytes.Contains(sample, []byte(c.old)) {
				t.Fatalf("setup: %q not in the sample", c.old)
			}
			input := t.TempDir()
			os.WriteFile(filepath.Join(input, "comp.json"), bytes.Replace(sample, []byte(c.old), []byte(c.new), 1), 0o644)
			db := newDB(t)
			r := runCmd(t, db, input, "--include-testing")
			if r.code != c.code || !strings.Contains(r.stdout+r.stderr, c.want) {
				t.Fatalf("exit %d, want %d and %q\n%s%s", r.code, c.code, c.want, r.stdout, r.stderr)
			}
			if n := one(t, db, `SELECT COUNT(*) FROM facts`); n != "0" {
				t.Errorf("%s figures stored from a file with an error", n)
			}
		})
	}
	// A whole-paise amount is stored exactly.
	db := newDB(t)
	runCmd(t, db, filepath.Join(runData, "clean"))
	if got := one(t, db, `SELECT money_paise FROM facts WHERE name = 'computation.allowed_depreciation'`); got != "11245050" {
		t.Errorf("allowed depreciation stored as %s paise, want 11245050", got)
	}
}

// AC-3: a file two profiles both recognise is refused with PROFILE_AMBIGUOUS, and nothing
// is written.
func TestAmbiguousFormatIsRefused(t *testing.T) {
	profiles := t.TempDir()
	copyTree(t, profDir, profiles)
	src := filepath.Join(profiles, "generic", "computation", "v1", "profile.yaml")
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dup := filepath.Join(profiles, "other", "computation", "v1")
	os.MkdirAll(dup, 0o755)
	os.WriteFile(filepath.Join(dup, "profile.yaml"),
		bytes.Replace(b, []byte(`profile_id: "generic.taxsuite.computation"`), []byte(`profile_id: "other.taxsuite.computation"`), 1), 0o644)
	db := newDB(t)
	r := cmd(t, "run", "--db", db, "--input", filepath.Join(runData, "clean"), "--tax-year", "2025-26",
		"--vocab", vocabDir, "--profiles", profiles)
	if r.code != exitInput || !strings.Contains(r.stderr, "PROFILE_AMBIGUOUS: comp_2025-26_agrees.json") {
		t.Fatalf("exit %d, stderr %q", r.code, r.stderr)
	}
	if n := one(t, db, `SELECT COUNT(*) FROM document_versions`); n != "0" {
		t.Errorf("%s document versions written", n)
	}
}

// AC-5: facts lists every stored figure with its file and place, byte for byte the same
// every time, with no IDs or times; an input never read is an input error.
func TestFactsCommand(t *testing.T) {
	db := newDB(t)
	input := filepath.Join(runData, "missing")
	runCmd(t, db, input)
	var outs []string
	for range 2 {
		r := cmd(t, "facts", "--db", db, "--input", input)
		if r.code != exitOK {
			t.Fatalf("exit %d\n%s", r.code, r.stderr)
		}
		outs = append(outs, r.stdout)
	}
	if outs[0] != outs[1] {
		t.Error("two listings differ")
	}
	for _, s := range []string{
		"computation.allowed_depreciation (FY2025-26): 1,12,450.50  [comp_2025-26_agrees.json $.business_income.allowed_depreciation]",
		"tax_audit_report, FY2025-26, draft 1: tar_2025-26_no_total.json",
		"No figures",
	} {
		if !strings.Contains(outs[0], s) {
			t.Errorf("listing lacks %q:\n%s", s, outs[0])
		}
	}
	if m := regexp.MustCompile(`\d{4}-\d{2}-\d{2}T|\d{2}:\d{2}:\d{2}`).FindString(outs[0]); m != "" {
		t.Errorf("listing contains a timestamp %q", m)
	}
	if r := cmd(t, "facts", "--db", db, "--input", filepath.Join(runData, "clean")); r.code != exitInput {
		t.Errorf("an input never read: exit %d, want 2", r.code)
	}
}
