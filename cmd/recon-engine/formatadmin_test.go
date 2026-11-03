package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
)

// SPEC-016: the format and parameter commands, on a database that holds the standard formats.

func formatsDB(t *testing.T) string {
	t.Helper()
	db := filepath.Join(t.TempDir(), "recon.db")
	if r := cmd(t, "load-rules", "--db", db, "--store", storeDir, "--rules", rulesDir, "--vocab", vocabDir, "--profiles", profDir); r.code != exitOK {
		t.Fatalf("load-rules: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	return db
}

func listFormats(t *testing.T, db string) formatList {
	t.Helper()
	out := filepath.Join(t.TempDir(), "f.json")
	if r := cmd(t, "formats", "--db", db, "--rules", rulesDir, "--vocab", vocabDir, "--json", out); r.code != exitOK {
		t.Fatalf("formats: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var l formatList
	if err := json.Unmarshal(b, &l); err != nil {
		t.Fatal(err)
	}
	return l
}

// addOfficeFormat stores an office format of two versions, copied from a standard profile.
func addOfficeFormat(t *testing.T, db string) {
	t.Helper()
	l := listFormats(t, db)
	d, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	tx, _ := d.Begin()
	defer tx.Rollback()
	std := l.Formats[0]
	vs, _ := tx.FormatVersions(std.ID)
	for i := 0; i < 2; i++ {
		if _, err := tx.AddFormatVersion(store.FormatVersion{FormatID: "office.copy", Name: "Our copy", Source: store.OriginOffice, FileType: std.FileType,
			Document: std.Document, BodyYAML: vs[0].BodyYAML, SHA256: sumHex([]byte{byte(i)}), Status: store.FormatTesting, CreatedBy: "A. Partner", PublishedAt: "2026-11-03T10:00:00Z", PublishedBy: "A. Partner"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func TestFormatsListShowsStandardFormatsAndParameters(t *testing.T) {
	db := formatsDB(t)
	l := listFormats(t, db)
	if len(l.Formats) == 0 || len(l.Parameters) == 0 {
		t.Fatalf("%d formats, %d parameters", len(l.Formats), len(l.Parameters))
	}
	reads := 0
	for _, f := range l.Formats {
		if f.Source != store.OriginBuiltin || f.Status != store.FormatActive {
			t.Errorf("format %s: %+v", f.ID, f)
		}
		reads += len(f.ReadsFacts) + len(f.ReadsRecords)
	}
	if reads == 0 {
		t.Error("no format says which figures it reads")
	}
	// A standard format and a standard parameter cannot be changed.
	std := l.Formats[0].ID
	if r := withStdin(t, `{"by":"A. Partner","reason":"x"}`, "format-disable", "--db", db, "--format", std, "--json", filepath.Join(t.TempDir(), "o.json")); r.code != exitInput || !strings.Contains(r.stderr, "FORMAT_READ_ONLY") {
		t.Errorf("standard format disabled: exit %d %s", r.code, r.stderr)
	}
	if r := withStdin(t, `{"by":"A. Partner","reason":"x"}`, "parameter-disable", "--db", db, "--parameter", l.Parameters[0].Key, "--rules", rulesDir, "--vocab", vocabDir, "--json", filepath.Join(t.TempDir(), "o.json")); r.code != exitInput || !strings.Contains(r.stderr, "PARAMETER_READ_ONLY") {
		t.Errorf("standard parameter disabled: exit %d %s", r.code, r.stderr)
	}
	// The same input gives the same list.
	again := listFormats(t, db)
	a, _ := json.Marshal(l)
	b, _ := json.Marshal(again)
	if string(a) != string(b) {
		t.Error("the list differs between two reads")
	}
}

func TestOfficeFormatSwitchAndVersions(t *testing.T) {
	db := formatsDB(t)
	addOfficeFormat(t, db)
	o := filepath.Join(t.TempDir(), "o.json")
	do := func(stdin string, args ...string) result {
		return withStdin(t, stdin, append(args, "--db", db, "--format", "office.copy", "--json", o)...)
	}
	if r := do(`{"by":"A. Partner","reason":""}`, "format-disable"); r.code != exitInput || !strings.Contains(r.stderr, "REASON_REQUIRED") {
		t.Errorf("no reason: exit %d %s", r.code, r.stderr)
	}
	if r := do(`{"by":"A. Partner","reason":"replaced"}`, "format-disable"); r.code != exitOK {
		t.Fatalf("disable: %s", r.stderr)
	}
	if r := do(`{"by":"A. Partner","reason":"again"}`, "format-disable"); !strings.Contains(r.stderr, "ALREADY_DISABLED") {
		t.Errorf("disable twice: %s", r.stderr)
	}
	if r := do(`{"by":"A. Partner","reason":"needed again"}`, "format-enable"); r.code != exitOK {
		t.Fatalf("enable: %s", r.stderr)
	}
	var f formatOut
	for _, x := range listFormats(t, db).Formats {
		if x.ID == "office.copy" {
			f = x
		}
	}
	if f.Status != store.FormatTesting || f.Revision != 2 || len(f.Versions) != 2 {
		t.Fatalf("after enabling: %+v", f)
	}
	if r := do(`{"by":"A. Partner","reason":"v2 was wrong"}`, "format-switch", "--version", "2"); !strings.Contains(r.stderr, "ALREADY_IN_USE") {
		t.Errorf("switch to the version in use: %s", r.stderr)
	}
	if r := do(`{"by":"A. Partner","reason":"v2 was wrong"}`, "format-switch", "--version", "1"); r.code != exitOK {
		t.Fatalf("switch back: %s", r.stderr)
	}
	for _, x := range listFormats(t, db).Formats {
		if x.ID == "office.copy" && x.Revision != 1 {
			t.Errorf("revision = %d after switching back", x.Revision)
		}
	}
}

func TestParameterAddAliasAndSwitch(t *testing.T) {
	db := formatsDB(t)
	l := listFormats(t, db)
	std := l.Parameters[0]
	good := `{"by":"A. Partner","key":"office.advance_received","name":"Advance received","kind":"money","document":"` + std.Document +
		`","description":"Advances received.","aliases":["Customer advance"]}`
	o := filepath.Join(t.TempDir(), "o.json")
	add := func(body string) result {
		return withStdin(t, body, "parameter-add", "--db", db, "--vocab", vocabDir, "--json", o)
	}
	if r := add(good); r.code != exitOK {
		t.Fatalf("add: exit %d %s", r.code, r.stderr)
	}
	for body, want := range map[string]string{
		good: "PARAMETER_EXISTS",
		strings.Replace(good, "office.advance_received", "Bad Key", 1):      "BAD_KEY",
		strings.Replace(good, `"money"`, `"percent"`, 1):                    "BAD_KIND",
		strings.Replace(good, std.Document, "no_such_document", 1):          "BAD_DOCUMENT",
		strings.Replace(good, "office.advance_received", "office.other", 1): "ALIAS_TAKEN", // the name is already used
		strings.Replace(good, "office.advance_received", std.Key, 1):        "PARAMETER_EXISTS",
	} {
		if r := add(body); r.code != exitInput || !strings.Contains(r.stderr, want) {
			t.Errorf("want %s: exit %d %s", want, r.code, r.stderr)
		}
	}
	alias := func(body string) result {
		return withStdin(t, body, "parameter-alias", "--db", db, "--parameter", "office.advance_received", "--json", o)
	}
	if r := alias(`{"by":"A. Partner","alias":"Advances"}`); r.code != exitOK {
		t.Fatalf("alias add: %s", r.stderr)
	}
	if r := alias(`{"by":"A. Partner","alias":"advances"}`); !strings.Contains(r.stderr, "ALIAS_TAKEN") {
		t.Errorf("same alias twice: %s", r.stderr)
	}
	if r := alias(`{"by":"A. Partner","alias":"Customer advance","remove":true}`); r.code != exitOK {
		t.Fatalf("alias remove: %s", r.stderr)
	}
	var got parameterOut
	for _, p := range listFormats(t, db).Parameters {
		if p.Key == "office.advance_received" {
			got = p
		}
	}
	if got.Source != store.OriginOffice || got.Version != "v3" || len(got.Aliases) != 1 || got.Aliases[0] != "Advances" {
		t.Fatalf("parameter = %+v", got)
	}
	sw := func(name, reason string) result {
		return withStdin(t, `{"by":"A. Partner","reason":"`+reason+`"}`, name, "--db", db, "--parameter", got.Key, "--rules", rulesDir, "--vocab", vocabDir, "--json", o)
	}
	if r := sw("parameter-disable", "not used"); r.code != exitOK {
		t.Fatalf("disable: %s", r.stderr)
	}
	if r := sw("parameter-disable", "again"); !strings.Contains(r.stderr, "ALREADY_DISABLED") {
		t.Errorf("disable twice: %s", r.stderr)
	}
	if r := sw("parameter-enable", "needed"); r.code != exitOK {
		t.Fatalf("enable: %s", r.stderr)
	}
}

// SPEC-016: a layout taught with --db is also stored as an office format, with its audit entry,
// and a check-only run stores nothing.
func TestTaughtLayoutIsStoredAsAnOfficeFormat(t *testing.T) {
	office := filepath.Join(t.TempDir(), "profiles")
	db := formatsDB(t)
	before := len(listFormats(t, db).Formats)
	if r, _ := createProfile(t, office, mappingText(t), teachSample, "--check-only", "--db", db, "--by", "A. Partner"); r.code != exitOK {
		t.Fatalf("check-only: %s", r.stderr)
	}
	if got := len(listFormats(t, db).Formats); got != before {
		t.Fatalf("a check-only run stored a format: %d formats, was %d", got, before)
	}
	if r, _ := createProfile(t, office, mappingText(t), teachSample, "--db", db); r.code != exitInput || !strings.Contains(r.stderr, "BAD_NAME") {
		t.Fatalf("no --by: exit %d %s", r.code, r.stderr)
	}
	if r, _ := createProfile(t, office, mappingText(t), teachSample, "--db", db, "--by", "A. Partner"); r.code != exitOK {
		t.Fatalf("save: %s", r.stderr)
	}
	var office1 formatOut
	for _, f := range listFormats(t, db).Formats {
		if f.Source == store.OriginOffice {
			office1 = f
		}
	}
	if office1.ID == "" || office1.Status != store.FormatActive || office1.Version != "v1" || len(office1.ReadsFacts)+len(office1.ReadsRecords) == 0 {
		t.Errorf("office format = %+v", office1)
	}
}

// SPEC-016: an office format is edited as a draft, published as testing, and then read only by
// runs that include testing; switching back reads the earlier version. A draft is never read.
func TestEditPublishAndDryRun(t *testing.T) {
	office := filepath.Join(t.TempDir(), "profiles")
	db := formatsDB(t)
	if r, _ := createProfile(t, office, mappingText(t), teachSample, "--db", db, "--by", "A. Partner"); r.code != exitOK {
		t.Fatalf("teach: %s", r.stderr)
	}
	var id string
	for _, f := range listFormats(t, db).Formats {
		if f.Source == store.OriginOffice {
			id = f.ID
		}
	}
	if id == "" {
		t.Fatal("no office format stored")
	}
	run := func(extra ...string) result {
		return runCmd(t, db, teachSample, append([]string{"--tax-year", "2026-27", "--office-profiles", office}, extra...)...)
	}
	if r := run(); r.code == exitInput && strings.Contains(r.stderr, "PROFILE_NO_MATCH") {
		t.Fatalf("the taught format is not read: %s", r.stderr)
	}

	// An edit must name the same layout, and a standard format cannot be edited.
	if r, _ := createProfile(t, office, strings.Replace(mappingText(t), `"Office sales export"`, `"Another name"`, 1), teachSample, "--format", id, "--db", db, "--by", "A. Partner"); !strings.Contains(r.stderr, "FORMAT_MISMATCH") {
		t.Errorf("another name: exit %d %s", r.code, r.stderr)
	}
	std := listFormats(t, db).Formats[0].ID
	if r, _ := createProfile(t, office, mappingText(t), teachSample, "--format", std, "--db", db, "--by", "A. Partner"); r.code != exitInput {
		t.Errorf("standard format edited: exit %d", r.code)
	}

	// The check shows which rules can run, and saves nothing.
	r, out := createProfile(t, office, mappingText(t), teachSample, "--check-only", "--format", id, "--db", db, "--rules", rulesDir)
	if r.code != exitOK {
		t.Fatalf("check: %s", r.stderr)
	}
	var checked struct {
		DryRun *dryRun `json:"dry_run"`
	}
	readJSON(t, out, &checked)
	if checked.DryRun == nil || len(checked.DryRun.CanRun)+len(checked.DryRun.CannotCheck) == 0 {
		t.Errorf("no dry run: %+v", checked.DryRun)
	}

	// Saving the edit makes a draft: version 2 exists, but nothing changes for runs.
	r, out = createProfile(t, office, mappingText(t), teachSample, "--format", id, "--db", db, "--by", "A. Partner", "--rules", rulesDir)
	if r.code != exitOK {
		t.Fatalf("draft: exit %d %s", r.code, r.stderr)
	}
	var saved struct {
		Draft    bool `json:"draft"`
		Revision int  `json:"format_revision"`
	}
	readJSON(t, out, &saved)
	if !saved.Draft || saved.Revision != 2 {
		t.Errorf("draft = %+v", saved)
	}
	if _, err := os.Stat(filepath.Join(office, "office_sales_export", "sales_register", "v2")); err == nil {
		t.Error("a draft wrote its profile files")
	}
	var f formatOut
	for _, x := range listFormats(t, db).Formats {
		if x.ID == id {
			f = x
		}
	}
	if f.Revision != 1 || len(f.Versions) != 2 || f.Versions[1].Status != store.FormatDraft || f.Status != store.FormatActive {
		t.Fatalf("after the draft: %+v", f)
	}
	get := filepath.Join(t.TempDir(), "g.json")
	if r := cmd(t, "format-get", "--db", db, "--format", id, "--json", get); r.code != exitOK {
		t.Fatal(r.stderr)
	}
	var g formatGet
	readJSON(t, get, &g)
	if !g.Editable || g.InUse == nil || g.Draft == nil || g.Draft.Revision != 2 {
		t.Errorf("format-get = %+v", g)
	}

	// Publishing as testing writes the files; only a run that includes testing reads it.
	pub := func(version, status string) result {
		return withStdin(t, `{"by":"A. Partner"}`, "format-publish", "--db", db, "--office-profiles", office, "--format", id,
			"--version", version, "--status", status, "--rules", rulesDir, "--vocab", vocabDir, "--profiles", profDir, "--json", filepath.Join(t.TempDir(), "p.json"))
	}
	if r := pub("1", "testing"); !strings.Contains(r.stderr, "ALREADY_PUBLISHED") {
		t.Errorf("publish a published version: %s", r.stderr)
	}
	if r := pub("2", "testing"); r.code != exitOK {
		t.Fatalf("publish: %s", r.stderr)
	}
	if _, err := os.Stat(filepath.Join(office, "office_sales_export", "sales_register", "v2", "profile.yaml")); err != nil {
		t.Error("the published profile is not on disk")
	}
	if r := run(); r.code != exitInput || !strings.Contains(r.stderr, "PROFILE_NO_MATCH") {
		t.Errorf("a testing format was read by a run without testing: exit %d %s", r.code, r.stderr)
	}
	if r := run("--include-testing"); r.code == exitInput {
		t.Errorf("a run with testing did not read the published format: %s", r.stderr)
	}

	// Switching back to the first version reads that one again, and a disabled format is not read.
	sw := func(name string, extra ...string) result {
		return withStdin(t, `{"by":"A. Partner","reason":"v2 was wrong"}`, append([]string{name, "--db", db, "--format", id,
			"--json", filepath.Join(t.TempDir(), "s.json")}, extra...)...)
	}
	if r := sw("format-switch", "--version", "1"); r.code != exitOK {
		t.Fatalf("switch back: %s", r.stderr)
	}
	if r := run("--include-testing"); r.code == exitInput {
		t.Errorf("the first version is not read after switching back: %s", r.stderr)
	}
	if r := sw("format-disable"); r.code != exitOK {
		t.Fatalf("disable: %s", r.stderr)
	}
	if r := run("--include-testing"); r.code != exitInput || !strings.Contains(r.stderr, "PROFILE_NO_MATCH") {
		t.Errorf("a disabled format was read: exit %d %s", r.code, r.stderr)
	}
}

func auditCounts(t *testing.T, db string) map[string]int {
	t.Helper()
	d, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	tx, _ := d.Begin()
	defer tx.Rollback()
	entries, err := tx.AuditEntries()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]int{}
	for _, e := range entries {
		out[e.Action]++
		if e.Actor == "" || e.At == "" || e.ObjectID == "" {
			t.Errorf("audit entry %d lacks its user, time or object: %+v", e.Seq, e)
		}
	}
	return out
}

// SPEC-016 AC-14: each create, edit, publish, switch and disable of a format, and each parameter
// change, writes one audit entry with its user, time and object.
func TestFormatChangesAreAudited(t *testing.T) {
	office := filepath.Join(t.TempDir(), "profiles")
	db := formatsDB(t)
	if r, _ := createProfile(t, office, mappingText(t), teachSample, "--db", db, "--by", "A. Partner"); r.code != exitOK {
		t.Fatal(r.stderr)
	}
	id := ""
	for _, f := range listFormats(t, db).Formats {
		if f.Source == store.OriginOffice {
			id = f.ID
		}
	}
	if r, _ := createProfile(t, office, mappingText(t), teachSample, "--format", id, "--db", db, "--by", "A. Partner"); r.code != exitOK {
		t.Fatal(r.stderr)
	}
	step := func(stdin string, args ...string) {
		t.Helper()
		if r := withStdin(t, stdin, append(args, "--db", db, "--json", filepath.Join(t.TempDir(), "o.json"))...); r.code != exitOK {
			t.Fatalf("%v: %s", args[0], r.stderr)
		}
	}
	step(`{"by":"A. Partner"}`, "format-publish", "--office-profiles", office, "--format", id, "--version", "2", "--status", "testing", "--vocab", vocabDir, "--profiles", profDir, "--rules", rulesDir)
	step(`{"by":"A. Partner","reason":"v2 was wrong"}`, "format-switch", "--format", id, "--version", "1")
	step(`{"by":"A. Partner","reason":"replaced"}`, "format-disable", "--format", id)
	step(`{"by":"A. Partner","reason":"needed again"}`, "format-enable", "--format", id)
	step(`{"by":"A. Partner","key":"office.advance","name":"Advance","kind":"money","document":"sales_register","description":"","aliases":[]}`, "parameter-add", "--vocab", vocabDir)
	step(`{"by":"A. Partner","alias":"Adv"}`, "parameter-alias", "--parameter", "office.advance")
	step(`{"by":"A. Partner","reason":"not used"}`, "parameter-disable", "--parameter", "office.advance", "--rules", rulesDir, "--vocab", vocabDir)
	got := auditCounts(t, db)
	for _, want := range []string{"format.create", "format.draft", "format.publish", "format.switch_version", "format.disable", "format.enable",
		"parameter.add", "parameter.alias_add", "parameter.disable"} {
		if got[want] != 1 {
			t.Errorf("audit entries for %s = %d, want 1 (all: %v)", want, got[want], got)
		}
	}
}

// SPEC-016 AC-9: the gaps are consistent with what the active formats read.
func TestGapsAreConsistent(t *testing.T) {
	l := listFormats(t, formatsDB(t))
	supplied := map[string]bool{}
	for _, f := range l.Formats {
		if f.Status != store.FormatActive {
			continue
		}
		for _, n := range append(append([]string{}, f.ReadsFacts...), f.ReadsRecords...) {
			supplied[n] = true
		}
		if f.NeededRead > f.Needed {
			t.Errorf("%s reads %d of %d needed figures", f.ID, f.NeededRead, f.Needed)
		}
		if f.LastUsed != "" {
			t.Errorf("%s was never used in this database but says %q", f.ID, f.LastUsed)
		}
	}
	byKey := map[string]bool{}
	for _, p := range l.Parameters {
		byKey[p.Key] = p.Status == "active" && !p.Deprecated
	}
	for _, k := range l.Unsupplied {
		if !byKey[k] || supplied[k] {
			t.Errorf("%s is listed as unsupplied but %v active, supplied %v", k, byKey[k], supplied[k])
		}
	}
	for _, p := range l.Parameters {
		if byKey[p.Key] && !supplied[p.Key] && !slices.Contains(l.Unsupplied, p.Key) {
			t.Errorf("%s is read by no active format but is not listed", p.Key)
		}
	}
	for _, u := range l.Uncovered {
		for _, m := range u.Missing {
			if supplied[m] {
				t.Errorf("rule %s is listed as missing %s, which an active format reads", u.ID, m)
			}
		}
	}
}

// SPEC-016 AC-12: a run records the stored format version it read with, so an earlier comparison
// still names it after the format is edited.
func TestRunRecordsTheFormatVersionItRead(t *testing.T) {
	office := filepath.Join(t.TempDir(), "profiles")
	db := formatsDB(t)
	if r, _ := createProfile(t, office, mappingText(t), teachSample, "--db", db, "--by", "A. Partner"); r.code != exitOK {
		t.Fatal(r.stderr)
	}
	if r := runCmd(t, db, teachSample, "--tax-year", "2026-27", "--office-profiles", office); r.code == exitInput {
		t.Fatalf("run: %s", r.stderr)
	}
	want := query(t, db, `SELECT id FROM format_versions WHERE source = 'office'`)
	docs := query(t, db, `SELECT profile_version_id FROM document_versions WHERE profile_id LIKE 'office.%'`)
	runs := query(t, db, `SELECT profile_version_id FROM run_documents WHERE profile_id LIKE 'office.%'`)
	if len(want) != 1 || len(docs) != 1 || len(runs) != 1 || docs[0][0] != want[0][0] || runs[0][0] != want[0][0] {
		t.Fatalf("format version %v; the file was read with %v, the run recorded %v", want, docs, runs)
	}
	// A standard profile's version is recorded too.
	std := query(t, db, `SELECT COUNT(*) FROM format_versions WHERE source = 'builtin'`)
	if std[0][0] == "0" {
		t.Error("no standard format version is stored for runs to point at")
	}
}
