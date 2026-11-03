package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
)

// SPEC-016: unrecognised files, kept samples read again on publish, and export and import.

func listUnrecognised(t *testing.T, db string) []unrecognisedOut {
	t.Helper()
	return listFormats(t, db).Unrecognised
}

func TestUnrecognisedFilesAreNotedAndClosed(t *testing.T) {
	office := filepath.Join(t.TempDir(), "profiles")
	db := formatsDB(t)
	run := func() result {
		return runCmd(t, db, teachSample, "--tax-year", "2026-27", "--office-profiles", office)
	}
	if r := run(); r.code != exitInput || !strings.Contains(r.stderr, "PROFILE_NO_MATCH") {
		t.Fatalf("exit %d %s", r.code, r.stderr)
	}
	got := listUnrecognised(t, db)
	if len(got) != 1 || got[0].FileName != "office_sales_Aug2026.json" || got[0].FileType != "json" || len(got[0].SHA256) != 64 || got[0].Size == 0 {
		t.Fatalf("unrecognised = %+v", got)
	}
	run() // the same file is noted once
	if n := len(listUnrecognised(t, db)); n != 1 {
		t.Errorf("the same file was noted %d times", n)
	}
	resolve := func(status, body string) result {
		return withStdin(t, body, "unrecognised-resolve", "--db", db, "--id", "1", "--status", status, "--json", filepath.Join(t.TempDir(), "r.json"))
	}
	if r := resolve("dismissed", `{"by":"A. Partner","note":""}`); !strings.Contains(r.stderr, "REASON_REQUIRED") {
		t.Errorf("dismissed with no reason: %s", r.stderr)
	}
	if r := resolve("dismissed", `{"by":"A. Partner","note":"a one-off export"}`); r.code != exitOK {
		t.Fatalf("dismiss: %s", r.stderr)
	}
	if n := len(listUnrecognised(t, db)); n != 0 {
		t.Errorf("a dismissed file is still listed: %d", n)
	}
	if r := resolve("taught", `{"by":"A. Partner"}`); !strings.Contains(r.stderr, "NOT_FOUND") {
		t.Errorf("closing a closed file: %s", r.stderr)
	}
}

// changedMapping is the teaching mapping with its amounts read as paise, so a sample reads differently.
func changedMapping(t *testing.T) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(mappingText(t)), &m); err != nil {
		t.Fatal(err)
	}
	m["unit"] = "paise"
	b, _ := json.Marshal(m)
	return string(b)
}

func TestKeptSampleIsReadAgainOnPublish(t *testing.T) {
	office := filepath.Join(t.TempDir(), "profiles")
	db := formatsDB(t)
	if r, _ := createProfile(t, office, mappingText(t), teachSample, "--db", db, "--by", "A. Partner", "--keep-sample"); r.code != exitOK {
		t.Fatalf("teach: %s", r.stderr)
	}
	var f formatOut
	for _, x := range listFormats(t, db).Formats {
		if x.Source == store.OriginOffice {
			f = x
		}
	}
	if len(f.Samples) != 1 || f.Samples[0].Name != "office_sales_Aug2026.json" || !f.Samples[0].HasFile {
		t.Fatalf("kept samples = %+v", f.Samples)
	}
	// A draft that reads the sample differently needs a confirmation and a reason.
	if r, _ := createProfile(t, office, changedMapping(t), teachSample, "--format", f.ID, "--db", db, "--by", "A. Partner"); r.code != exitOK {
		t.Fatalf("draft: %s", r.stderr)
	}
	pub := func(body string) result {
		return withStdin(t, body, "format-publish", "--db", db, "--office-profiles", office, "--format", f.ID, "--version", "2", "--status", "testing",
			"--rules", rulesDir, "--vocab", vocabDir, "--profiles", profDir, "--json", filepath.Join(t.TempDir(), "p.json"))
	}
	if r := pub(`{"by":"A. Partner"}`); r.code != exitInput || !strings.Contains(r.stderr, "SAMPLES_CHANGED") || !strings.Contains(r.stderr, "office_sales_Aug2026.json") {
		t.Fatalf("unconfirmed: exit %d %s", r.code, r.stderr)
	}
	if _, err := os.Stat(filepath.Join(office, "office_sales_export", "sales_register", "v2")); err == nil {
		t.Error("a refused publish left files on disk")
	}
	if r := pub(`{"by":"A. Partner","confirm":true,"reason":""}`); !strings.Contains(r.stderr, "SAMPLES_CHANGED") {
		t.Errorf("confirmed with no reason: %s", r.stderr)
	}
	if r := pub(`{"by":"A. Partner","confirm":true,"reason":"amounts are in paise in the new export"}`); r.code != exitOK {
		t.Fatalf("confirmed: %s", r.stderr)
	}

	// An admin may remove a kept sample.
	if r := withStdin(t, `{"by":"A. Partner"}`, "format-sample-discard", "--db", db, "--id", "1", "--json", filepath.Join(t.TempDir(), "s.json")); r.code != exitOK {
		t.Fatalf("remove: %s", r.stderr)
	}
	for _, x := range listFormats(t, db).Formats {
		if x.ID == f.ID && len(x.Samples) != 0 {
			t.Errorf("sample not removed: %+v", x.Samples)
		}
	}
}

func TestExportAndImportAFormat(t *testing.T) {
	office1, office2 := filepath.Join(t.TempDir(), "p1"), filepath.Join(t.TempDir(), "p2")
	db1, db2 := formatsDB(t), formatsDB(t)
	if r, _ := createProfile(t, office1, mappingText(t), teachSample, "--db", db1, "--by", "A. Partner"); r.code != exitOK {
		t.Fatalf("teach: %s", r.stderr)
	}
	var id string
	for _, x := range listFormats(t, db1).Formats {
		if x.Source == store.OriginOffice {
			id = x.ID
		}
	}
	param := `{"by":"A. Partner","key":"office.advance_received","name":"Advance received","kind":"money","document":"sales_register","description":"Advances.","aliases":["Advance"]}`
	if r := withStdin(t, param, "parameter-add", "--db", db1, "--vocab", vocabDir, "--json", filepath.Join(t.TempDir(), "a.json")); r.code != exitOK {
		t.Fatalf("parameter: %s", r.stderr)
	}
	// A standard format is not exported.
	if r := cmd(t, "format-export", "--db", db1, "--format", listFormats(t, db1).Formats[0].ID, "--vocab", vocabDir, "--json", filepath.Join(t.TempDir(), "x.json")); !strings.Contains(r.stderr, "FORMAT_READ_ONLY") {
		t.Errorf("exported a standard format: %s", r.stderr)
	}
	file := filepath.Join(t.TempDir(), "bundle.json")
	if r := cmd(t, "format-export", "--db", db1, "--format", id, "--vocab", vocabDir, "--json", file); r.code != exitOK {
		t.Fatalf("export: %s", r.stderr)
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var b bundle
	if err := json.Unmarshal(raw, &b); err != nil || b.Schema != bundleSchema || len(b.Parameters) != 1 || b.Format.ID != id || len(b.Format.Mapping) == 0 {
		t.Fatalf("bundle = %+v, %v", b, err)
	}

	imp := func(db, office, body string, extra ...string) result {
		args := append([]string{"format-import", "--db", db, "--office-profiles", office, "--profiles", profDir, "--vocab", vocabDir, "--by", "B. Partner",
			"--json", filepath.Join(t.TempDir(), "i.json")}, extra...)
		return withStdin(t, body, args...)
	}
	// A check shows what would be added and changes nothing.
	if r := imp(db2, office2, string(raw), "--check-only"); r.code != exitOK || !strings.Contains(r.stdout, "would add") {
		t.Fatalf("check: exit %d %s%s", r.code, r.stdout, r.stderr)
	}
	if n := len(listFormats(t, db2).Formats); n != len(listFormats(t, formatsDB(t)).Formats) {
		t.Errorf("a check stored a format: %d", n)
	}
	if r := imp(db2, office2, string(raw)); r.code != exitOK {
		t.Fatalf("import: exit %d %s", r.code, r.stderr)
	}
	var in formatOut
	for _, x := range listFormats(t, db2).Formats {
		if x.ID == id {
			in = x
		}
	}
	if in.Status != store.FormatTesting || in.Source != store.OriginOffice || len(in.ReadsFacts)+len(in.ReadsRecords) == 0 {
		t.Errorf("imported format = %+v", in)
	}
	found := false
	for _, p := range listFormats(t, db2).Parameters {
		found = found || (p.Key == "office.advance_received" && len(p.Aliases) == 1)
	}
	if !found {
		t.Error("the bundle's parameter was not imported with its alias")
	}
	if _, err := os.Stat(filepath.Join(office2, "office_sales_export", "sales_register", "v1", "profile.yaml")); err != nil {
		t.Error("the imported profile is not on disk")
	}
	// Nothing stored is changed: a second import is refused.
	if r := imp(db2, office2, string(raw)); !strings.Contains(r.stderr, "FORMAT_EXISTS") {
		t.Errorf("second import: %s", r.stderr)
	}
	// A bad bundle is refused whole.
	for body, want := range map[string]string{
		`{"schema":"other"}`: "BAD_BUNDLE",
		`not json`:           "BAD_BUNDLE",
		strings.Replace(string(raw), `"vocab_version":`, `"vocab_version":9`, 1):      "BAD_BUNDLE",
		strings.Replace(string(raw), `"profile_yaml":"`, `"profile_yaml":"x: y\n`, 1): "BAD_BUNDLE",
	} {
		if r := imp(formatsDB(t), filepath.Join(t.TempDir(), "p"), body); r.code != exitInput || !strings.Contains(r.stderr, want) {
			t.Errorf("bad bundle %.40q: exit %d %s%s (want %s; yaml %t, vocab %t)", body, r.code, r.stdout, r.stderr, want, strings.Contains(body, "x: y"), strings.Contains(body, "910"))
		}
	}
}

// SPEC-016: value settings. A layout whose amounts are written in lakhs, with Dr or Cr after each,
// is taught and read exactly: lakhs to paise, Cr negated, a minus sign or brackets as written.
func TestTeachLakhsWithDrCr(t *testing.T) {
	var m map[string]any
	if err := json.Unmarshal([]byte(mappingText(t)), &m); err != nil {
		t.Fatal(err)
	}
	m["name"], m["unit"], m["negative"] = "Lakh sales export", "lakhs", "drcr"
	body, _ := json.Marshal(m)
	sample := filepath.Join(teachData, "lakh_sales_Aug2026.json")
	office := filepath.Join(t.TempDir(), "profiles")
	r, out := createProfile(t, office, string(body), sample, "--check-only")
	if r.code != exitOK {
		t.Fatalf("check: exit %d %s", r.code, r.stderr)
	}
	var got struct {
		Preview []struct {
			MatchKey string            `json:"match_key"`
			Amounts  map[string]string `json:"amounts"`
		} `json:"preview"`
		YAML string `json:"profile_yaml"`
	}
	readJSON(t, out, &got)
	if !strings.Contains(got.YAML, `unit: "lakhs"`) || !strings.Contains(got.YAML, `sign: "drcr"`) {
		t.Errorf("the profile lacks the unit or sign:\n%s", got.YAML)
	}
	want := map[string]string{"LK/26-27/0001": "325000.00", "LK/26-27/0002": "220000.00", "LK/26-27/0003": "-50000.00"}
	if len(got.Preview) != 3 {
		t.Fatalf("preview = %+v", got.Preview)
	}
	for _, row := range got.Preview {
		if row.Amounts["taxable_value"] != want[row.MatchKey] {
			t.Errorf("%s taxable = %q, want %q", row.MatchKey, row.Amounts["taxable_value"], want[row.MatchKey])
		}
	}
	// A part of a paisa and an unknown negative style are refused with the field named.
	m["negative"] = "minus"
	bad, _ := json.Marshal(m)
	if r, _ := createProfile(t, office, string(bad), sample, "--check-only"); !strings.Contains(r.stderr, "BAD_NEGATIVE") {
		t.Errorf("unknown negative style: %s", r.stderr)
	}
	m["negative"], m["unit"] = "drcr", "millions"
	bad, _ = json.Marshal(m)
	if r, _ := createProfile(t, office, string(bad), sample, "--check-only"); !strings.Contains(r.stderr, "BAD_UNIT") {
		t.Errorf("unknown unit: %s", r.stderr)
	}
}

// SPEC-016: a CSV layout is taught like a JSON or Excel one, then recognised by a run.
func TestTeachCSVLayout(t *testing.T) {
	sample := filepath.Join(teachData, "csv_sales_Aug2026.csv")
	col := func(name, path, date string) map[string]string {
		return map[string]string{"name": name, "path": path, "date_format": date}
	}
	m := map[string]any{
		"name": "CSV sales export", "document_type": "sales_register", "rows": "$[*]", "where": []any{}, "unit": "rupees",
		"columns": []any{
			col("invoice_no", "['Bill No']", ""), col("invoice_date", "['Bill Date']", "dd-mm-yyyy"), col("counterparty_gstin", "['Buyer GSTIN']", ""),
			col("taxable_value", ".Taxable", ""), col("igst", ".IGST", ""), col("cgst", ".CGST", ""), col("sgst", ".SGST", ""),
			col("cess", ".Cess", ""), col("invoice_value", ".Total", ""),
		},
		"period": map[string]string{"path": "", "format": "", "const": "2026-08"}, "tax_year": "", "gstin_path": "", "identify": []any{},
	}
	body, _ := json.Marshal(m)
	office := filepath.Join(t.TempDir(), "profiles")
	db := formatsDB(t)
	r, out := createProfile(t, office, string(body), sample, "--check-only")
	if r.code != exitOK {
		t.Fatalf("check: exit %d %s", r.code, r.stderr)
	}
	var got struct {
		Rows    int `json:"rows"`
		Preview []struct {
			MatchKey string            `json:"match_key"`
			Amounts  map[string]string `json:"amounts"`
		} `json:"preview"`
		YAML string `json:"profile_yaml"`
	}
	readJSON(t, out, &got)
	if got.Rows != 3 || !strings.Contains(got.YAML, `file_type: "csv"`) {
		t.Fatalf("rows %d, yaml:\n%s", got.Rows, got.YAML)
	}
	want := map[string]string{"CS/26-27/0001": "325000.00", "CS/26-27/0002": "220000.00", "CS/26-27/0003": "-50000.00"}
	for _, row := range got.Preview {
		if row.Amounts["taxable_value"] != want[row.MatchKey] {
			t.Errorf("%s taxable = %q, want %q", row.MatchKey, row.Amounts["taxable_value"], want[row.MatchKey])
		}
	}
	if r, _ := createProfile(t, office, string(body), sample, "--db", db, "--by", "A. Partner"); r.code != exitOK {
		t.Fatalf("save: exit %d %s", r.code, r.stderr)
	}
	var saved formatOut
	for _, f := range listFormats(t, db).Formats {
		if f.Source == store.OriginOffice {
			saved = f
		}
	}
	if saved.FileType != "csv" || saved.Status != store.FormatActive {
		t.Errorf("stored format = %+v", saved)
	}
	if r := runCmd(t, db, sample, "--tax-year", "2026-27", "--office-profiles", office); strings.Contains(r.stderr, "PROFILE_NO_MATCH") {
		t.Errorf("the taught CSV layout is not recognised: %s", r.stderr)
	}
}
