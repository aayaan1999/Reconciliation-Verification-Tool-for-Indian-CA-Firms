package store

import (
	"database/sql"
	"errors"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func docRow(code string) DocType {
	return DocType{Code: code, Name: "Sales return register", Category: "Books", Description: "Credit notes issued to customers",
		ApplicableFor: []string{"xlsx", "csv"}, Active: true, CreatedBy: "partner", CreatedAt: "2026-11-06T00:00:00Z", UpdatedBy: "partner", UpdatedAt: "2026-11-06T00:00:00Z"}
}

func recType(name string) OfficeRecordType {
	return OfficeRecordType{Name: name, Description: "Rows of the sales return register", CreatedBy: "partner", CreatedAt: "2026-11-06T00:00:00Z",
		Columns: []OfficeColumn{
			{Name: "ref_no", Label: "Reference", Type: "text", Key: true, Required: true},
			{Name: "doc_date", Label: "Date", Type: "date", Key: true},
			{Name: "net_amount", Label: "Net amount", Type: "amount"},
		}}
}

func begin(t *testing.T, db *DB) *Tx {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tx.Rollback)
	return tx
}

// Migration 0015 on a populated database at 0014: nothing is lost, document_type is filled from
// each profile that names one, and opening again changes nothing (AC-1).
func TestMigration0015FromPopulated0014(t *testing.T) {
	file := filepath.Join(t.TempDir(), "recon.db")
	raw, err := sql.Open("sqlite3", dsn(file))
	if err != nil {
		t.Fatal(err)
	}
	raw.SetMaxOpenConns(1)
	if _, err := raw.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	names, _ := fs.Glob(migrations, "migrations/*.sql")
	for _, name := range names {
		base := path.Base(name)
		num, _, _ := strings.Cut(base, "_")
		if v, _ := strconv.Atoi(num); v > 14 {
			continue
		}
		body, err := migrations.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := raw.Exec(string(body)); err != nil {
			t.Fatalf("%s: %v", base, err)
		}
		v, _ := strconv.Atoi(num)
		if _, err := raw.Exec(`INSERT INTO schema_migrations (version, name) VALUES (?, ?)`, v, base); err != nil {
			t.Fatal(err)
		}
	}
	insert := `INSERT INTO format_versions (format_id, revision, version, name, source, file_type, document, body_yaml, sha256, status, created_by)
		VALUES (?, 1, 'v1', 'n', 'office', 'json', 'd', ?, 'x', 'active', 'u')`
	for _, f := range [][2]string{
		{"with-quotes", "schema: recon.profile/1\ndocument_type: \"gstr1\"\nname: a\n"},
		{"plain", "document_type: sales_register # note\n"},
		{"none", "schema: recon.profile/1\nname: b\n"},
	} {
		if _, err := raw.Exec(insert, f[0], f[1]); err != nil {
			t.Fatal(err)
		}
	}
	raw.Close()

	for pass := 0; pass < 2; pass++ { // the second open must change nothing
		db, err := Open(file)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		rows, err := db.db.Query(`SELECT format_id, document_type FROM format_versions ORDER BY format_id`)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var id, doc string
			if err := rows.Scan(&id, &doc); err != nil {
				t.Fatal(err)
			}
			got[id] = doc
		}
		rows.Close()
		want := map[string]string{"with-quotes": "gstr1", "plain": "sales_register", "none": ""}
		for k, w := range want {
			if got[k] != w {
				t.Errorf("pass %d: %s document_type = %q, want %q", pass, k, got[k], w)
			}
		}
		var n int
		if err := db.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil || n != 15 {
			t.Errorf("pass %d: migrations = %d, err %v", pass, n, err)
		}
		db.Close()
	}
}

// A new format version gets its document type from the profile when the caller leaves it empty.
func TestAddFormatVersionFillsDocumentType(t *testing.T) {
	db, _ := open(t)
	tx := begin(t, db)
	v := formatV("f1", FormatActive)
	v.BodyYAML = "document_type: \"sales_return\"\n"
	id, err := tx.AddFormatVersion(v)
	if err != nil {
		t.Fatal(err)
	}
	got, ok, err := tx.FormatVersionByID(id)
	if err != nil || !ok || got.DocumentType != "sales_return" {
		t.Fatalf("version = %+v, ok %v, err %v", got, ok, err)
	}
}

func TestSeedBuiltinDocumentKeepsPartnerChoices(t *testing.T) {
	db, _ := open(t)
	tx := begin(t, db)
	d := docRow("gstr1")
	d.Name, d.RecordType = "GSTR-1", "gst_invoice"
	d.ApplicableFor = []string{"json", "xlsx", "csv"}
	if changed, err := tx.SeedBuiltinDocument(d); err != nil || !changed {
		t.Fatalf("first seed: changed %v, err %v", changed, err)
	}
	if changed, err := tx.SeedBuiltinDocument(d); err != nil || changed {
		t.Fatalf("second seed: changed %v, err %v; want no write", changed, err)
	}
	if err := tx.SetDocActive("gstr1", false, "partner", "t"); err != nil {
		t.Fatal(err)
	}
	d.Description = "changed in the YAML"
	d.Category = "Other"
	if changed, err := tx.SeedBuiltinDocument(d); err != nil || !changed {
		t.Fatalf("seed after YAML change: changed %v, err %v", changed, err)
	}
	got, _, _ := tx.DocTypeByCode("gstr1")
	if got.Active || got.Category != "Books" || got.Description != "changed in the YAML" || got.Origin != OriginBuiltin {
		t.Errorf("after seed: %+v; want inactive and category kept, description refreshed", got)
	}
	var ce *ConflictError
	if err := tx.DeleteDocType("gstr1"); !errors.As(err, &ce) {
		t.Errorf("deleting a built-in document: %v, want a ConflictError", err)
	}
}

func TestDocTypeAddUpdateListDelete(t *testing.T) {
	db, _ := open(t)
	tx := begin(t, db)
	rt := recType("sales_return_row")
	if err := tx.AddOfficeRecordType(rt); err != nil {
		t.Fatal(err)
	}
	d := docRow("sales_return")
	d.RecordType = rt.Name
	if err := tx.AddDocType(d); err != nil {
		t.Fatal(err)
	}
	var ce *ConflictError
	if err := tx.AddDocType(d); !errors.As(err, &ce) {
		t.Errorf("duplicate code: %v, want a ConflictError", err)
	}
	if err := tx.AddOfficeRecordType(rt); !errors.As(err, &ce) {
		t.Errorf("duplicate record type: %v, want a ConflictError", err)
	}
	b := docRow("bank_extra")
	b.Category = "Bank"
	if err := tx.AddDocType(b); err != nil {
		t.Fatal(err)
	}
	list, err := tx.DocTypes()
	if err != nil || len(list) != 2 || list[0].Code != "bank_extra" || list[1].Code != "sales_return" {
		t.Fatalf("list = %+v, err %v; want category order (Bank, Books)", list, err)
	}
	got, ok, _ := tx.DocTypeByCode("sales_return")
	if !ok || !slices.Equal(got.ApplicableFor, []string{"xlsx", "csv"}) || got.RecordType != rt.Name || !got.Active {
		t.Fatalf("got %+v", got)
	}
	got.Name, got.UpdatedBy = "Sales returns", "p2"
	if err := tx.UpdateDocType(got); err != nil {
		t.Fatal(err)
	}
	if again, _, _ := tx.DocTypeByCode("sales_return"); again.Name != "Sales returns" || again.CreatedBy != "partner" || again.UpdatedBy != "p2" {
		t.Errorf("after update: %+v", again)
	}
	if err := tx.DeleteDocType("sales_return"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := tx.OfficeRecordTypeByName(rt.Name); ok {
		t.Error("the record type stayed after its only document was deleted")
	}
	if _, ok, _ := tx.DocTypeByCode("sales_return"); ok {
		t.Error("the document stayed after delete")
	}
}

// A record type keeps its columns in order and a copy is independent of its source.
func TestOfficeRecordTypeColumnsAndIndependence(t *testing.T) {
	db, _ := open(t)
	tx := begin(t, db)
	a := recType("rt_a")
	b := recType("rt_b")
	b.CopiedFrom = "rt_a"
	for _, rt := range []OfficeRecordType{a, b} {
		if err := tx.AddOfficeRecordType(rt); err != nil {
			t.Fatal(err)
		}
	}
	changed := append(slices.Clone(b.Columns), OfficeColumn{Name: "note", Label: "Note", Type: "text"})
	changed[2].Label = "Amount after tax"
	if err := tx.ReplaceOfficeColumns("rt_b", "edited", changed); err != nil {
		t.Fatal(err)
	}
	gotA, _, _ := tx.OfficeRecordTypeByName("rt_a")
	gotB, _, _ := tx.OfficeRecordTypeByName("rt_b")
	if len(gotA.Columns) != 3 || gotA.Columns[2].Label != "Net amount" || gotA.Description == "edited" {
		t.Errorf("source changed by editing the copy: %+v", gotA)
	}
	if len(gotB.Columns) != 4 || gotB.Columns[2].Label != "Amount after tax" || gotB.Columns[3].Position != 4 || gotB.CopiedFrom != "rt_a" {
		t.Errorf("copy = %+v", gotB)
	}
	if !gotB.Columns[0].Key || !gotB.Columns[0].Required || gotB.Columns[2].Type != "amount" {
		t.Errorf("flags or types lost: %+v", gotB.Columns)
	}
	all, _ := tx.OfficeRecordTypes()
	if len(all) != 2 || all[0].Name != "rt_a" {
		t.Errorf("list order: %+v", all)
	}
}

// Usage: formats by the structured column, stored rows, and rules that name the record type.
func TestUsageLocksAndBlocksDelete(t *testing.T) {
	db, _ := open(t)
	tx := begin(t, db)
	rt := recType("sales_return_row")
	if err := tx.AddOfficeRecordType(rt); err != nil {
		t.Fatal(err)
	}
	d := docRow("sales_return")
	d.RecordType = rt.Name
	if err := tx.AddDocType(d); err != nil {
		t.Fatal(err)
	}
	if u, err := tx.DocUsage("sales_return"); err != nil || u.InUse() {
		t.Fatalf("fresh usage = %+v, err %v", u, err)
	}

	// a format version that reads it
	v := formatV("sr-xlsx", FormatActive)
	v.BodyYAML = "document_type: sales_return\n"
	if _, err := tx.AddFormatVersion(v); err != nil {
		t.Fatal(err)
	}
	u, err := tx.DocUsage("sales_return")
	if err != nil || !slices.Equal(u.Formats, []string{"sr-xlsx"}) || !u.InUse() {
		t.Fatalf("usage with a format = %+v, err %v", u, err)
	}
	var ce *ConflictError
	if err := tx.DeleteDocType("sales_return"); !errors.As(err, &ce) {
		t.Errorf("delete of a document with a format: %v, want a ConflictError", err)
	}
}

func TestUsageByRuleVersions(t *testing.T) {
	db, _ := open(t)
	tx := begin(t, db)
	body := func(rec string) []byte {
		return []byte("id: RECON-OFF-001\ninputs:\n  - record_type: \"" + rec + "\"\n    as: a\n  - record_type: gst_invoice\n    as: b\n")
	}
	f := RuleFile{RuleID: "RECON-OFF-001", Version: "1", Source: SourceOffice, Schema: "recon.rule/1", Body: body("sales_return_row"), SHA256: "a1"}
	if _, err := tx.PutRule(f, RuleStatus{Status: StatusTesting}); err != nil {
		t.Fatal(err)
	}
	u, err := tx.RecordTypeUsage("sales_return_row")
	if err != nil || !slices.Equal(u.Rules, []string{"RECON-OFF-001"}) || len(u.ActiveRules) != 0 {
		t.Fatalf("usage = %+v, err %v", u, err)
	}
	if u, _ := tx.RecordTypeUsage("sales_return"); u.InUse() {
		t.Errorf("a longer name matched by prefix: %+v", u)
	}
	if _, err := tx.tx.Exec(`UPDATE rule_state SET status = 'active'`); err != nil {
		t.Fatal(err)
	}
	if u, _ := tx.RecordTypeUsage("sales_return_row"); !slices.Equal(u.ActiveRules, []string{"RECON-OFF-001"}) {
		t.Errorf("active rules = %+v", u.ActiveRules)
	}
	// a new version that no longer reads it: the old version still locks the record type
	f.Body, f.SHA256 = body("other_row"), "a2"
	if _, err := tx.PutRule(f, RuleStatus{Status: StatusTesting}); err != nil {
		t.Fatal(err)
	}
	u, _ = tx.RecordTypeUsage("sales_return_row")
	if !slices.Equal(u.Rules, []string{"RECON-OFF-001"}) || len(u.ActiveRules) != 0 {
		t.Errorf("after the rule moved on: %+v", u)
	}
}

func TestOfficeSnapshotOnce(t *testing.T) {
	db, _ := open(t)
	tx := begin(t, db)
	if err := tx.PutOfficeSnapshot("", "{}"); err == nil {
		t.Error("an empty hash was accepted")
	}
	for i := 0; i < 2; i++ {
		if err := tx.PutOfficeSnapshot("h1", `{"a":1}`); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.PutOfficeSnapshot("h1", `{"a":2}`); err == nil {
		t.Error("different content under one hash was accepted")
	}
	if body, ok, err := tx.OfficeSnapshot("h1"); err != nil || !ok || body != `{"a":1}` {
		t.Errorf("snapshot = %q, %v, %v", body, ok, err)
	}
	if _, ok, _ := tx.OfficeSnapshot("none"); ok {
		t.Error("a snapshot appeared from nowhere")
	}
}

// The Rule Builder writes an input inline; usage is found in that form, with a quote or without,
// and a longer name that merely starts with the record type's name is not a use of it.
func TestUsageReadsInlineInputs(t *testing.T) {
	db, _ := open(t)
	tx := begin(t, db)
	for i, line := range []string{
		`  a: { record_type: "sales_return_row", label: "Returns" }`,
		`  a: { label: "Returns", record_type: "sales_return_row" }`,
		`  a: { record_type: sales_return_row, label: "Returns" }`,
		`  - record_type: "sales_return_row"`,
		`    record_type: sales_return_row # a note`,
	} {
		id := "RECON-INL-00" + string(rune('1'+i))
		body := "id: " + id + "\ninputs:\n" + line + "\n"
		if _, err := tx.PutRule(RuleFile{RuleID: id, Version: "1", Source: SourceOffice, Schema: "recon.rule/1", Body: []byte(body), SHA256: id}, RuleStatus{Status: StatusTesting}); err != nil {
			t.Fatal(err)
		}
	}
	for i, line := range []string{
		`  a: { record_type: "sales_return_row_2", label: "Another" }`,
		`  a: { record_type: "old_sales_return_row", label: "Another" }`,
		`  a: { fact: "sales_return_row", label: "Not a record type" }`,
	} {
		id := "RECON-NOT-00" + string(rune('1'+i))
		body := "id: " + id + "\ninputs:\n" + line + "\n"
		if _, err := tx.PutRule(RuleFile{RuleID: id, Version: "1", Source: SourceOffice, Schema: "recon.rule/1", Body: []byte(body), SHA256: id}, RuleStatus{Status: StatusTesting}); err != nil {
			t.Fatal(err)
		}
	}
	u, err := tx.RecordTypeUsage("sales_return_row")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"RECON-INL-001", "RECON-INL-002", "RECON-INL-003", "RECON-INL-004", "RECON-INL-005"}
	if !slices.Equal(u.Rules, want) {
		t.Errorf("rules reading the record type = %v, want %v", u.Rules, want)
	}
}
