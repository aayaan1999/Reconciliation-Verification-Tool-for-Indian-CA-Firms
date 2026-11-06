package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// The Document Master commands (SPEC-022 part A2), run through the real command entry point.

// docCmd runs one command with body on standard input and returns the result and the JSON it wrote.
func docCmd[T any](t *testing.T, name, db, body string, extra ...string) (result, T) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "out.json")
	args := append([]string{name, "--db", db, "--vocab", vocabDir, "--json", out}, extra...)
	old := stdin
	stdin = strings.NewReader(body)
	defer func() { stdin = old }()
	r := cmd(t, args...)
	var v T
	if data, err := os.ReadFile(out); err == nil {
		if err := json.Unmarshal(data, &v); err != nil {
			t.Fatalf("%s: output is not JSON: %v\n%s", name, err, data)
		}
	}
	return r, v
}

const salesReturnBody = `{"by":"A. Partner","code":"sales_return","name":"Sales return register","category":"Books",
 "description":"Credit notes issued to customers","applicable_for":["csv","xlsx"],"status":"active",
 "record_type":{"name":"sales_return_row","description":"Rows of the sales return register","columns":[
  {"name":"ref_no","label":"Reference","type":"text","key":true,"required":true},
  {"name":"doc_date","label":"Date","type":"date","key":true,"required":false},
  {"name":"net_amount","label":"Net amount","type":"amount","key":false,"required":false}]}}`

func masterDB(t *testing.T) string {
	t.Helper()
	db := filepath.Join(t.TempDir(), "recon.db")
	s, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	return db
}

func addSalesReturn(t *testing.T) string {
	t.Helper()
	db := masterDB(t)
	r, out := docCmd[docSaveOut](t, "document-save", db, salesReturnBody)
	if r.code != 0 || !out.Saved || out.Document.Status != "active" {
		t.Fatalf("document-save: exit %d, %+v\n%s%s", r.code, out, r.stdout, r.stderr)
	}
	return db
}

func codes(l []docOut) []string {
	var c []string
	for _, d := range l {
		c = append(c, d.Code)
	}
	return c
}

// AC-3, AC-5: a new document is saved, listed, audited, and offered to the wizard only for the
// formats it accepts; the built-in documents are there with their default categories.
func TestDocumentAddListAndWizardFilter(t *testing.T) {
	db := addSalesReturn(t)
	_, list := docCmd[struct{ Documents []docOut }](t, "document-list", db, "")
	got := codes(list.Documents)
	for _, want := range []string{"sales_return", "gstr1", "computation", "bank_statement"} {
		if !slices.Contains(got, want) {
			t.Errorf("list is missing %s: %v", want, got)
		}
	}
	for _, d := range list.Documents {
		if d.Code == "gstr1" && (d.Origin != "builtin" || d.Category != "GST" || d.Status != "active") {
			t.Errorf("gstr1 = %+v", d)
		}
		if d.Code == "sales_return" && (d.Origin != "office" || d.RecordType != "sales_return_row" || !slices.Equal(d.ApplicableFor, []string{"xlsx", "csv"})) {
			t.Errorf("sales_return = %+v", d)
		}
	}
	for ft, want := range map[string]bool{"json": false, "csv": true, "xlsx": true} {
		_, l := docCmd[struct{ Documents []docOut }](t, "document-list", db, "", "--active", "--file-type", ft)
		if slices.Contains(codes(l.Documents), "sales_return") != want {
			t.Errorf("--file-type %s: sales_return listed = %v, want %v", ft, !want, want)
		}
	}
	if got := query(t, db, `SELECT action, object_type, object_id, actor FROM audit_log ORDER BY seq`); len(got) != 2 ||
		got[0][0] != "document.add" || got[1][0] != "record_type.add" || got[1][2] != "sales_return_row" || got[0][3] != "A. Partner" {
		t.Errorf("audit = %v", got)
	}
	// the merged vocabulary now has it, and the base does not
	v, mst, err := loadVocabMaster(db, vocabDir)
	if err != nil {
		t.Fatal(err)
	}
	if !v.IsRecordType("sales_return_row") || v.OfficeHash() == "" {
		t.Error("the merged vocabulary lacks the document")
	}
	if wiz := teachableTypes(v, mst, "json"); slices.ContainsFunc(wiz, func(x teachableType) bool { return x.Name == "sales_return" }) {
		t.Error("a csv/xlsx document is offered for a json file")
	}
	var found *teachableType
	wiz := teachableTypes(v, mst, "csv")
	for i := range wiz {
		if wiz[i].Name == "sales_return" {
			found = &wiz[i]
		}
	}
	if found == nil || found.Origin != "office" || len(found.Columns) != 3 || found.Columns[0].Label != "Reference" ||
		!found.Columns[0].Required || !found.Columns[0].Key || found.Columns[1].Type != "date" || !found.Columns[2].Amount {
		t.Errorf("wizard entry = %+v", found)
	}
	base, _ := vocab.LoadDir(vocabDir)
	if base.IsRecordType("sales_return_row") {
		t.Error("the base vocabulary was changed")
	}
}

// The wizard's built-in entries keep their columns, and now carry a label and a type (a
// field named *_date is a date).
func TestTeachableTypesBuiltinColumns(t *testing.T) {
	v, mst, err := loadVocabMaster("", vocabDir)
	if err != nil {
		t.Fatal(err)
	}
	if v.OfficeHash() != "" || len(mst.docs) != 0 {
		t.Error("an empty Master changed the vocabulary")
	}
	wiz := teachableTypes(v, mst, "")
	if len(wiz) == 0 {
		t.Fatal("no document is offered")
	}
	for _, d := range wiz {
		if d.Origin != "builtin" {
			t.Errorf("%s is %s", d.Name, d.Origin)
		}
		for _, c := range d.Columns {
			if c.Label == "" || (c.Amount) != (c.Type == "amount") || (strings.HasSuffix(c.Name, "_date") && c.Type != "date") {
				t.Errorf("%s column %+v", d.Name, c)
			}
		}
	}
}

// AC-4: a copied structure is independent of its source.
func TestDocumentCopyStructure(t *testing.T) {
	db := masterDB(t)
	body := `{"by":"A. Partner","code":"credit_notes","name":"Credit notes","category":"GST","description":"Credit notes copied from GSTR-1 rows",
	 "applicable_for":["json"],"record_type":{"name":"credit_note_row","description":"Rows of credit notes","copy_from":"gstr1_b2b_invoice"}}`
	r, out := docCmd[docSaveOut](t, "document-save", db, body)
	if r.code != 0 || !out.Saved || out.Document.Structure == nil || len(out.Document.Structure.Columns) == 0 || out.Document.Structure.CopiedFrom != "gstr1_b2b_invoice" {
		t.Fatalf("copy: exit %d, %+v\n%s", r.code, out, r.stderr)
	}
	base, _ := vocab.LoadDir(vocabDir)
	src, _ := base.RecordType("gstr1_b2b_invoice")
	if n := len(out.Document.Structure.Columns); n != len(src.Fields)+len(src.Amounts) {
		t.Errorf("copied %d columns, source has %d", n, len(src.Fields)+len(src.Amounts))
	}
	// edit a label of the copy: the source is a base record type and cannot change; the stored copy does
	edit := strings.Replace(body, `"copy_from":"gstr1_b2b_invoice"`, `"copy_from":"gstr1_b2b_invoice","columns":`+columnsOf(out.Document.Structure.Columns, "Changed label"), 1)
	if r, o := docCmd[docSaveOut](t, "document-save", db, edit, "--edit"); r.code != 0 || !o.Saved || o.Document.Structure.Columns[0].Label != "Changed label" {
		t.Fatalf("edit of the copy: exit %d %+v\n%s", r.code, o, r.stderr)
	}
	again, _ := base.RecordType("gstr1_b2b_invoice")
	if !slices.Equal(again.Fields, src.Fields) || !slices.Equal(again.Amounts, src.Amounts) {
		t.Error("the source changed")
	}
	// the other way round: an office record type copied into a second document, then edited
	body2 := `{"by":"A. Partner","code":"credit_notes_2","name":"Credit notes two","category":"GST","description":"A copy of an office structure",
	 "applicable_for":["json"],"record_type":{"name":"credit_note_row_2","description":"Rows","copy_from":"credit_note_row"}}`
	r2, o2 := docCmd[docSaveOut](t, "document-save", db, body2)
	if r2.code != 0 || o2.Document.Structure.Columns[0].Label != "Changed label" || o2.Document.Structure.CopiedFrom != "credit_note_row" {
		t.Fatalf("copy of an office type: exit %d %+v\n%s", r2.code, o2, r2.stderr)
	}
	edit2 := strings.Replace(body2, `"copy_from":"credit_note_row"`, `"copy_from":"credit_note_row","columns":`+columnsOf(o2.Document.Structure.Columns, "Second label"), 1)
	if r, _ := docCmd[docSaveOut](t, "document-save", db, edit2, "--edit"); r.code != 0 {
		t.Fatalf("edit of the second copy: %s", r.stderr)
	}
	_, first := docCmd[docDetailOut](t, "document-get", db, "", "--code", "credit_notes")
	if first.Structure.Columns[0].Label != "Changed label" {
		t.Errorf("editing the second copy changed the first: %q", first.Structure.Columns[0].Label)
	}
}

func columnsOf(cols []docColumnJSON, firstLabel string) string {
	cp := slices.Clone(cols)
	cp[0].Label = firstLabel
	b, _ := json.Marshal(cp)
	return string(b)
}

// AC-6: a structure that is not valid saves a Draft that says what is missing; a Draft is not
// offered anywhere, cannot be activated, and becomes ready when its structure is completed.
func TestDocumentDraftLifecycle(t *testing.T) {
	db := masterDB(t)
	noStructure := `{"by":"A. Partner","code":"debit_notes","name":"Debit notes","category":"Books","description":"Debit notes received from suppliers","applicable_for":["xlsx"]}`
	r, out := docCmd[docSaveOut](t, "document-save", db, noStructure)
	if r.code != 0 || !out.Saved || out.Document.Status != "draft" || out.Document.Structure != nil {
		t.Fatalf("draft without structure: exit %d %+v\n%s", r.code, out, r.stderr)
	}
	badStructure := strings.TrimSuffix(noStructure, "}") + `,"record_type":{"name":"debit_note_row","description":"Rows","columns":[{"name":"ref_no","label":"Ref","type":"text","key":false,"required":false}]}}`
	r, out = docCmd[docSaveOut](t, "document-save", db, badStructure, "--edit")
	if r.code != 0 || out.Document.Status != "draft" || len(out.Problems) == 0 || out.Document.RecordType != "" {
		t.Fatalf("edit with an incomplete structure: exit %d %+v\n%s", r.code, out, r.stderr)
	}
	if got := query(t, db, `SELECT COUNT(*) FROM office_record_types`); got[0][0] != "0" {
		t.Error("an invalid structure was stored")
	}
	_, l := docCmd[struct{ Documents []docOut }](t, "document-list", db, "", "--active")
	if slices.Contains(codes(l.Documents), "debit_notes") {
		t.Error("a Draft is offered")
	}
	v, _, err := loadVocabMaster(db, vocabDir)
	if err != nil || v.OfficeHash() != "" {
		t.Errorf("a Draft is in the merged vocabulary: %v", err)
	}
	if r, _ := docCmd[docDetailOut](t, "document-status", db, `{"by":"A. Partner","code":"debit_notes","active":true}`); r.code != exitInput || !strings.Contains(r.stderr, "NOT_READY") {
		t.Errorf("activating a Draft: exit %d %s", r.code, r.stderr)
	}
	good := strings.TrimSuffix(noStructure, "}") + `,"record_type":{"name":"debit_note_row","description":"Rows","columns":[` +
		`{"name":"ref_no","label":"Ref","type":"text","key":true,"required":false},{"name":"amount","label":"Amount","type":"amount","key":false,"required":false}]}}`
	r, out = docCmd[docSaveOut](t, "document-save", db, good, "--edit")
	if r.code != 0 || out.Document.Status != "active" || len(out.Problems) != 0 {
		t.Fatalf("completing the structure: exit %d %+v\n%s", r.code, out, r.stderr)
	}
}

// AC-7 and the refusals: duplicate and clashing codes, bad fields, editing a built-in document.
func TestDocumentRefusals(t *testing.T) {
	db := addSalesReturn(t)
	cases := map[string]struct{ body, want string }{
		"duplicate code":   {salesReturnBody, "DUPLICATE_CODE"},
		"code, other case": {strings.Replace(salesReturnBody, `"sales_return"`, `"SALES_RETURN"`, 1), "INVALID_DOCUMENT"},
		"base document":    {strings.Replace(salesReturnBody, `"code":"sales_return"`, `"code":"gstr1"`, 1), "DUPLICATE_CODE"},
		"short name":       {strings.Replace(strings.Replace(salesReturnBody, `"code":"sales_return"`, `"code":"other_doc"`, 1), `"name":"Sales return register"`, `"name":"x"`, 1), "INVALID_DOCUMENT"},
		"bad status":       {strings.Replace(strings.Replace(salesReturnBody, `"code":"sales_return"`, `"code":"other_doc"`, 1), `"status":"active"`, `"status":"maybe"`, 1), "BAD_STATUS"},
	}
	for name, tc := range cases {
		r, out := docCmd[docSaveOut](t, "document-save", db, tc.body)
		if r.code != exitInput || !strings.Contains(r.stderr, tc.want) {
			t.Errorf("%s: exit %d, stderr %q, want %s", name, r.code, r.stderr, tc.want)
		}
		if tc.want == "INVALID_DOCUMENT" && (out.Saved || len(out.Problems) == 0 || out.Problems[0].Field == "") {
			t.Errorf("%s: problems = %+v", name, out)
		}
	}
	// A structure that clashes with another document's saves only as a Draft, naming the problem.
	again := strings.Replace(salesReturnBody, `"code":"sales_return"`, `"code":"other_doc"`, 1)
	if r, out := docCmd[docSaveOut](t, "document-save", db, again); r.code != 0 || !out.Saved || out.Document.Status != "draft" || len(out.Problems) == 0 || out.Document.RecordType != "" {
		t.Errorf("record type again: exit %d %+v", r.code, out)
	}
	builtin := `{"by":"A. Partner","code":"gstr1","name":"GSTR one","category":"GST","description":"Edited built-in document","applicable_for":["json"]}`
	if r, _ := docCmd[docSaveOut](t, "document-save", db, builtin, "--edit"); r.code != exitInput || !strings.Contains(r.stderr, "BUILTIN") {
		t.Errorf("editing a built-in: exit %d %s", r.code, r.stderr)
	}
	if got := query(t, db, `SELECT COUNT(*) FROM document_types WHERE origin = 'office'`); got[0][0] != "2" {
		t.Errorf("a refused save left rows: %v", got)
	}
}

// document-validate reports every problem together and writes nothing.
func TestDocumentValidateWritesNothing(t *testing.T) {
	db := masterDB(t)
	before := query(t, db, `SELECT COUNT(*) FROM audit_log`)[0][0]
	bad := `{"by":"x","code":"ab","name":"","category":"Misc","description":"short","applicable_for":[],
	 "record_type":{"name":"rt","description":"d","columns":[{"name":"a","label":"","type":"number","key":true,"required":false}]}}`
	r, v := docCmd[docValidateOut](t, "document-validate", db, bad)
	if r.code != 0 || v.Valid || len(v.Problems) < 6 {
		t.Fatalf("validate: exit %d %+v", r.code, v)
	}
	r, v = docCmd[docValidateOut](t, "document-validate", db, salesReturnBody)
	if r.code != 0 || !v.Valid || v.RecordType == nil || len(v.RecordType.Columns) != 3 {
		t.Fatalf("validate of a valid document: %+v", v)
	}
	copyBody := `{"by":"x","code":"credit_notes","name":"Credit notes","category":"GST","description":"Credit notes copied from rows","applicable_for":["json"],
	 "record_type":{"name":"credit_note_row","description":"Rows","copy_from":"gstr1_b2b_invoice"}}`
	if _, v := docCmd[docValidateOut](t, "document-validate", db, copyBody); !v.Valid || len(v.RecordType.Columns) == 0 {
		t.Errorf("a preview of a copy has no columns: %+v", v)
	}
	if after := query(t, db, `SELECT COUNT(*) FROM audit_log`)[0][0]; after != before {
		t.Error("validate wrote an audit entry")
	}
	if got := query(t, db, `SELECT COUNT(*) FROM document_types WHERE origin = 'office'`); got[0][0] != "0" {
		t.Error("validate saved a document")
	}
}

// AC-9: deactivating warns through the counts, always proceeds, keeps the document in the merged
// vocabulary and the office hash, removes it from the wizard, and records the counts.
func TestDocumentDeactivateKeepsEverything(t *testing.T) {
	db := addSalesReturn(t)
	// a format that reads it
	s, _ := store.Open(db)
	tx, _ := s.Begin()
	if _, err := tx.AddFormatVersion(store.FormatVersion{FormatID: "sr-xlsx", Name: "Sales returns", Source: store.OriginOffice, FileType: "xlsx", Document: "Sales return register",
		BodyYAML: "document_type: sales_return\n", SHA256: "aa", Status: store.FormatActive, CreatedBy: "p"}); err != nil {
		t.Fatal(err)
	}
	tx.Commit()
	s.Close()
	v1, _, _ := loadVocabMaster(db, vocabDir)
	hash := v1.OfficeHash()

	r, out := docCmd[docDetailOut](t, "document-status", db, `{"by":"A. Partner","code":"sales_return","active":false}`)
	if r.code != 0 || out.Status != "inactive" || len(out.Usage.Formats) != 1 || out.ReferencedBy.Formats != 1 {
		t.Fatalf("deactivate: exit %d %+v\n%s", r.code, out, r.stderr)
	}
	v2, mst, err := loadVocabMaster(db, vocabDir)
	if err != nil || v2.OfficeHash() != hash || !v2.IsRecordType("sales_return_row") {
		t.Errorf("deactivating changed the vocabulary (hash %q -> %q, err %v)", hash, v2.OfficeHash(), err)
	}
	if slices.ContainsFunc(teachableTypes(v2, mst, "xlsx"), func(x teachableType) bool { return x.Name == "sales_return" }) {
		t.Error("an inactive document is offered")
	}
	got := query(t, db, `SELECT action, details_json FROM audit_log WHERE action = 'document.deactivate'`)
	if len(got) != 1 || !strings.Contains(got[0][1], `"formats":"1"`) || !strings.Contains(got[0][1], `"rules":"0"`) {
		t.Errorf("audit = %v", got)
	}
	// switching to the same state again writes nothing
	docCmd[docDetailOut](t, "document-status", db, `{"by":"A. Partner","code":"sales_return","active":false}`)
	if n := query(t, db, `SELECT COUNT(*) FROM audit_log WHERE action = 'document.deactivate'`)[0][0]; n != "1" {
		t.Errorf("a repeated deactivation was audited again: %s", n)
	}
	if r, out := docCmd[docDetailOut](t, "document-status", db, `{"by":"A. Partner","code":"sales_return","active":true}`); r.code != 0 || out.Status != "active" {
		t.Errorf("activate: exit %d %+v", r.code, out)
	}
	// a built-in document can be switched off too
	if r, out := docCmd[docDetailOut](t, "document-status", db, `{"by":"A. Partner","code":"gstr1","active":false}`); r.code != 0 || out.Status != "inactive" {
		t.Errorf("deactivating a built-in: exit %d %+v", r.code, out)
	}
	v3, mst3, _ := loadVocabMaster(db, vocabDir)
	if slices.ContainsFunc(teachableTypes(v3, mst3, ""), func(x teachableType) bool { return x.Name == "gstr1" }) {
		t.Error("an inactive built-in document is offered")
	}
}

// AC-10: delete is refused for a built-in and a used document (with what uses it), and works for
// an unused one, which takes its record type with it.
func TestDocumentDelete(t *testing.T) {
	db := addSalesReturn(t)
	r, out := docCmd[docDeleteOut](t, "document-delete", db, `{"by":"A. Partner","code":"gstr1"}`)
	if r.code != exitInput || out.Code != "BUILTIN" || !strings.Contains(r.stderr, "BUILTIN") {
		t.Errorf("built-in: exit %d %+v %s", r.code, out, r.stderr)
	}
	s, _ := store.Open(db)
	tx, _ := s.Begin()
	tx.AddFormatVersion(store.FormatVersion{FormatID: "sr-xlsx", Name: "n", Source: store.OriginOffice, FileType: "xlsx", Document: "d",
		BodyYAML: "document_type: sales_return\n", SHA256: "aa", Status: store.FormatActive, CreatedBy: "p"})
	tx.Commit()
	s.Close()
	r, out = docCmd[docDeleteOut](t, "document-delete", db, `{"by":"A. Partner","code":"sales_return"}`)
	if r.code != exitInput || out.Code != "IN_USE" || !slices.Equal(out.Usage.Formats, []string{"sr-xlsx"}) || !strings.Contains(r.stderr, "IN_USE") {
		t.Errorf("used: exit %d %+v %s", r.code, out, r.stderr)
	}

	db2 := addSalesReturn(t)
	r, out = docCmd[docDeleteOut](t, "document-delete", db2, `{"by":"A. Partner","code":"sales_return"}`)
	if r.code != 0 || !out.Deleted {
		t.Fatalf("unused: exit %d %+v %s", r.code, out, r.stderr)
	}
	if got := query(t, db2, `SELECT (SELECT COUNT(*) FROM office_record_types), (SELECT COUNT(*) FROM office_record_columns), (SELECT COUNT(*) FROM document_types WHERE code = 'sales_return')`); got[0][0] != "0" || got[0][1] != "0" || got[0][2] != "0" {
		t.Errorf("rows left after delete: %v", got)
	}
	if got := query(t, db2, `SELECT action FROM audit_log WHERE action LIKE '%delete' ORDER BY seq`); len(got) != 2 || got[0][0] != "document.delete" || got[1][0] != "record_type.delete" {
		t.Errorf("audit = %v", got)
	}
}

// AC-11: a structure something reads allows labels, the description and new optional columns, and
// nothing else; a file type a format still reads cannot be taken off.
func TestDocumentLockedStructure(t *testing.T) {
	db := addSalesReturn(t)
	s, _ := store.Open(db)
	tx, _ := s.Begin()
	tx.AddFormatVersion(store.FormatVersion{FormatID: "sr-xlsx", Name: "n", Source: store.OriginOffice, FileType: "xlsx", Document: "d",
		BodyYAML: "document_type: sales_return\n", SHA256: "aa", Status: store.FormatActive, CreatedBy: "p"})
	tx.Commit()
	s.Close()
	_, cur := docCmd[docDetailOut](t, "document-get", db, "", "--code", "sales_return")
	if !cur.Structure.Locked {
		t.Fatal("a structure a format reads is not locked")
	}
	withCols := func(cols []docColumnJSON, formats string) string {
		b, _ := json.Marshal(cols)
		return `{"by":"A. Partner","code":"sales_return","name":"Sales return register","category":"Books","description":"Credit notes issued to customers",
		 "applicable_for":` + formats + `,"record_type":{"name":"sales_return_row","description":"Rows, reworded","columns":` + string(b) + `}}`
	}
	edit := func(mod func([]docColumnJSON) []docColumnJSON, formats string) (result, docSaveOut) {
		return docCmd[docSaveOut](t, "document-save", db, withCols(mod(slices.Clone(cur.Structure.Columns)), formats), "--edit")
	}
	refused := map[string]func([]docColumnJSON) []docColumnJSON{
		"remove a column":   func(c []docColumnJSON) []docColumnJSON { return c[:2] },
		"rename a column":   func(c []docColumnJSON) []docColumnJSON { c[2].Name = "gross_amount"; return c },
		"retype a column":   func(c []docColumnJSON) []docColumnJSON { c[2].Type = "text"; return c },
		"change a key flag": func(c []docColumnJSON) []docColumnJSON { c[1].Key = false; return c },
		"change required":   func(c []docColumnJSON) []docColumnJSON { c[1].Required = true; return c },
		"add a required one": func(c []docColumnJSON) []docColumnJSON {
			return append(c, docColumnJSON{Name: "memo", Label: "Memo", Type: "text", Required: true})
		},
		"add a key column": func(c []docColumnJSON) []docColumnJSON {
			return append(c, docColumnJSON{Name: "memo", Label: "Memo", Type: "text", Key: true})
		},
	}
	for name, mod := range refused {
		r, out := edit(mod, `["csv","xlsx"]`)
		if r.code != exitInput || out.Saved || !slices.ContainsFunc(out.Problems, func(p docProblemJSON) bool { return p.Code == "LOCKED" }) {
			t.Errorf("%s: exit %d %+v", name, r.code, out)
		}
	}
	allowed := func(c []docColumnJSON) []docColumnJSON {
		c[0].Label = "Reference number"
		return append(c, docColumnJSON{Name: "memo", Label: "Memo", Type: "text"})
	}
	if r, out := edit(allowed, `["csv","xlsx"]`); r.code != 0 || !out.Saved || len(out.Document.Structure.Columns) != 4 || out.Document.Structure.Columns[0].Label != "Reference number" {
		t.Fatalf("a label change and a new optional column: exit %d %+v\n%s", r.code, out, r.stderr)
	}
	_, cur = docCmd[docDetailOut](t, "document-get", db, "", "--code", "sales_return") // the structure as it is now: four columns
	if r, out := edit(func(c []docColumnJSON) []docColumnJSON { return c }, `["csv"]`); r.code != exitInput || !slices.ContainsFunc(out.Problems, func(p docProblemJSON) bool { return p.Field == "applicable_for" }) {
		t.Errorf("taking off a file type a format reads: exit %d %+v", r.code, out)
	}
	if r, out := edit(func(c []docColumnJSON) []docColumnJSON { return c }, `["csv","xlsx","json"]`); r.code != 0 || !out.Saved {
		t.Errorf("adding a file type: exit %d %+v", r.code, out)
	}
	var edits int
	for _, row := range query(t, db, `SELECT action FROM audit_log WHERE action IN ('document.edit', 'record_type.edit')`) {
		_ = row
		edits++
	}
	if edits != 3 {
		t.Errorf("audit has %d edit entries, want 3 (two document edits, one record type edit)", edits)
	}
	// an unlocked structure takes any edit
	db2 := addSalesReturn(t)
	if r, out := docCmd[docSaveOut](t, "document-save", db2, strings.Replace(salesReturnBody, `"net_amount"`, `"gross_amount"`, 1), "--edit"); r.code != 0 || !out.Saved {
		t.Errorf("an unlocked rename: exit %d %+v\n%s", r.code, out, r.stderr)
	}
	// the record type cannot be renamed
	if r, _ := docCmd[docSaveOut](t, "document-save", db2, strings.Replace(salesReturnBody, `"name":"sales_return_row"`, `"name":"renamed_row"`, 1), "--edit"); r.code != exitInput {
		t.Errorf("renaming the record type: exit %d", r.code)
	}
}

// A run records the office vocabulary it used and keeps its snapshot; with an empty Master it
// records nothing (AC-2, AC-8 at the store).
func TestRunKeepsOfficeVocabulary(t *testing.T) {
	db := newDB(t)
	r := runCmd(t, db, filepath.Join(runData, "differs"), "--include-testing")
	if r.code != exitFindings && r.code != exitOK {
		t.Fatalf("run: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if got := one(t, db, `SELECT office_vocab_hash FROM runs ORDER BY id DESC LIMIT 1`); got != "" {
		t.Errorf("an empty Master gave an office hash: %q", got)
	}
	if rr, out := docCmd[docSaveOut](t, "document-save", db, salesReturnBody); rr.code != 0 || !out.Saved {
		t.Fatalf("document-save: %s", rr.stderr)
	}
	r = runCmd(t, db, filepath.Join(runData, "differs"), "--include-testing")
	if r.code != exitFindings && r.code != exitOK {
		t.Fatalf("run with an office document: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	hash := one(t, db, `SELECT office_vocab_hash FROM runs ORDER BY id DESC LIMIT 1`)
	if len(hash) != 64 {
		t.Fatalf("office hash = %q", hash)
	}
	v, _, _ := loadVocabMaster(db, vocabDir)
	if hash != v.OfficeHash() {
		t.Errorf("the run recorded %s, the vocabulary hashes to %s", hash, v.OfficeHash())
	}
	if body := one(t, db, `SELECT body_json FROM office_vocab_snapshots WHERE hash = ?`, hash); body != v.OfficeCanonical() {
		t.Errorf("snapshot = %s", body)
	}
	// the same run again keeps one snapshot
	runCmd(t, db, filepath.Join(runData, "differs"), "--include-testing")
	if n := one(t, db, `SELECT COUNT(*) FROM office_vocab_snapshots`); n != "1" {
		t.Errorf("%s snapshots after two runs with the same Master", n)
	}
	// the base content hash is what it was
	if got := one(t, db, `SELECT vocab_hash FROM runs ORDER BY id DESC LIMIT 1`); got != v.ContentHash() {
		t.Error("the run's base vocabulary hash is not the base hash")
	}
}

// AC-1: load-rules (which the server runs at every start) fills the built-in documents; loading
// again writes nothing, and the Partner's active switch survives it.
func TestLoadRulesSeedsBuiltinDocuments(t *testing.T) {
	db := newDB(t)
	n := one(t, db, `SELECT COUNT(*) FROM document_types WHERE origin = 'builtin'`)
	base, err := vocab.LoadDir(vocabDir)
	if err != nil {
		t.Fatal(err)
	}
	if n == "0" || n != fmt.Sprint(len(base.DocumentTypes())) {
		t.Fatalf("%s built-in documents after load-rules, vocabulary has %d", n, len(base.DocumentTypes()))
	}
	if got := one(t, db, `SELECT category FROM document_types WHERE code = 'gstr1'`); got != "GST" {
		t.Errorf("gstr1 category = %q", got)
	}
	if r, _ := docCmd[docDetailOut](t, "document-status", db, `{"by":"A. Partner","code":"gstr1","active":false}`); r.code != 0 {
		t.Fatalf("deactivate: %s", r.stderr)
	}
	before := one(t, db, `SELECT COUNT(*) FROM audit_log`) + one(t, db, `SELECT group_concat(code || active || updated_at) FROM (SELECT * FROM document_types ORDER BY code)`)
	if r := loadRulesCmd(t, db, rulesDir); r.code != 0 {
		t.Fatalf("second load-rules: %s", r.stderr)
	}
	after := one(t, db, `SELECT COUNT(*) FROM audit_log`) + one(t, db, `SELECT group_concat(code || active || updated_at) FROM (SELECT * FROM document_types ORDER BY code)`)
	if before != after {
		t.Error("a second load-rules changed the documents")
	}
	if got := one(t, db, `SELECT active FROM document_types WHERE code = 'gstr1'`); got != "0" {
		t.Error("load-rules switched a deactivated built-in document back on")
	}
}

// AC-4, both ways: a document copied from an office one is independent. Editing the source leaves the
// copy as it was, and editing the copy leaves the source as it was.
func TestCopyIsIndependentBothWays(t *testing.T) {
	db := masterDB(t)
	if r, out := docCmd[docSaveOut](t, "document-save", db, sixColumnReturns); r.code != 0 || !out.Saved {
		t.Fatalf("source: %s", r.stderr)
	}
	copyBody := `{"by":"A. Partner","code":"return_ledger","name":"Returns ledger","category":"Books","description":"A copy of the sales return structure",
	 "applicable_for":["csv"],"record_type":{"name":"return_ledger_row","description":"Rows","copy_from":"sales_return_row"}}`
	r, copied := docCmd[docSaveOut](t, "document-save", db, copyBody)
	if r.code != 0 || !copied.Saved || copied.Document.Structure == nil || len(copied.Document.Structure.Columns) != 6 {
		t.Fatalf("copy: exit %d %+v\n%s", r.code, copied, r.stderr)
	}
	column := func(code string, i int) docColumnJSON {
		_, d := docCmd[docDetailOut](t, "document-get", db, "", "--code", code)
		return d.Structure.Columns[i]
	}
	// edit the source: rename a column, change a label, flip a key flag (it is unlocked: nothing reads it)
	_, src := docCmd[docDetailOut](t, "document-get", db, "", "--code", "sales_return")
	cols := slices.Clone(src.Structure.Columns)
	cols[3].Name, cols[3].Label, cols[0].Required = "net_value", "Net value", false
	body, _ := json.Marshal(map[string]any{"by": "A. Partner", "code": "sales_return", "name": "Sales return register", "category": "Books",
		"description": "Credit notes issued to customers", "applicable_for": []string{"json", "xlsx", "csv"}, "status": "active",
		"record_type": map[string]any{"name": "sales_return_row", "description": "Rows of the sales return register", "columns": cols}})
	if r, out := docCmd[docSaveOut](t, "document-save", db, string(body), "--edit"); r.code != 0 || !out.Saved {
		t.Fatalf("edit of the source: exit %d %+v\n%s", r.code, out, r.stderr)
	}
	if c := column("return_ledger", 3); c.Name != "net_amount" || c.Label != "Net amount" {
		t.Errorf("the copy followed the source: %+v", c)
	}
	if c := column("return_ledger", 0); !c.Required {
		t.Error("the copy followed the source's required flag")
	}
	// edit the copy: the source keeps its own
	_, cp := docCmd[docDetailOut](t, "document-get", db, "", "--code", "return_ledger")
	ccols := slices.Clone(cp.Structure.Columns)
	ccols[1].Label, ccols[2].Type = "Copy date", "text"
	cb, _ := json.Marshal(map[string]any{"by": "A. Partner", "code": "return_ledger", "name": "Returns ledger", "category": "Books",
		"description": "A copy of the sales return structure", "applicable_for": []string{"csv"}, "status": "active",
		"record_type": map[string]any{"name": "return_ledger_row", "description": "Rows", "columns": ccols}})
	if r, out := docCmd[docSaveOut](t, "document-save", db, string(cb), "--edit"); r.code != 0 || !out.Saved {
		t.Fatalf("edit of the copy: exit %d %+v\n%s", r.code, out, r.stderr)
	}
	if c := column("sales_return", 1); c.Label != "Date" || c.Type != "date" {
		t.Errorf("the source followed the copy: %+v", c)
	}
	if c := column("sales_return", 3); c.Name != "net_value" {
		t.Errorf("the source lost its own edit: %+v", c)
	}
}

// AC-10 for a rule: an office rule version that names the record type keeps the document from
// being deleted, locks its structure, and is listed in the answer.
func TestRuleVersionBlocksDeleteAndLocksStructure(t *testing.T) {
	db := masterDB(t)
	if r, out := docCmd[docSaveOut](t, "document-save", db, sixColumnReturns); r.code != 0 || !out.Saved {
		t.Fatalf("document-save: %s", r.stderr)
	}
	s, err := store.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	tx, _ := s.Begin()
	body := "id: RECON-OFF-001\ninputs:\n  a: { record_type: \"sales_return_row\", label: \"Returns\" }\n"
	inputs := "inputs:\n  - record_type: \"sales_return_row\"\n"
	if _, err := tx.PutRule(store.RuleFile{RuleID: "RECON-OFF-001", Version: "1", Source: store.SourceOffice, Schema: "recon.rule/1", Body: []byte(body + inputs), SHA256: "a1"},
		store.RuleStatus{Status: store.StatusTesting}); err != nil {
		t.Fatal(err)
	}
	tx.Commit()
	s.Close()
	_, d := docCmd[docDetailOut](t, "document-get", db, "", "--code", "sales_return")
	if !slices.Equal(d.Usage.Rules, []string{"RECON-OFF-001"}) || !d.Structure.Locked || d.ReferencedBy.Rules != 1 {
		t.Fatalf("usage %+v locked %v", d.Usage, d.Structure.Locked)
	}
	r, out := docCmd[docDeleteOut](t, "document-delete", db, `{"by":"A. Partner","code":"sales_return"}`)
	if r.code != exitInput || out.Code != "IN_USE" || !slices.Equal(out.Usage.Rules, []string{"RECON-OFF-001"}) || !strings.Contains(r.stderr, "RECON-OFF-001") {
		t.Errorf("delete: exit %d %+v %s", r.code, out, r.stderr)
	}
	// deactivating warns through the counts and still works; the rule is not touched
	if r, _ := docCmd[docDetailOut](t, "document-status", db, `{"by":"A. Partner","code":"sales_return","active":false}`); r.code != 0 {
		t.Fatalf("deactivate: %s", r.stderr)
	}
	if got := query(t, db, `SELECT status FROM rule_state WHERE rule_id = 'RECON-OFF-001'`); got[0][0] != "testing" {
		t.Errorf("deactivating changed the rule: %v", got)
	}
	if got := query(t, db, `SELECT details_json FROM audit_log WHERE action = 'document.deactivate'`); !strings.Contains(got[0][0], `"rules":"1"`) {
		t.Errorf("the audit entry lacks the counts: %v", got)
	}
}

// AC-12: adding a document creates no format, no layout file, no column mapping and no rule.
func TestAddingADocumentCreatesNothingElse(t *testing.T) {
	db := newDB(t)
	counts := func() string {
		return strings.Join([]string{
			one(t, db, `SELECT COUNT(*) FROM format_versions WHERE source = 'office'`),
			one(t, db, `SELECT COUNT(*) FROM format_state WHERE format_id IN (SELECT format_id FROM format_versions WHERE source = 'office')`),
			one(t, db, `SELECT COUNT(*) FROM rule_versions WHERE source = 'office'`),
			one(t, db, `SELECT COUNT(*) FROM parameter_versions WHERE source = 'office'`),
			one(t, db, `SELECT COUNT(*) FROM records`),
		}, ",")
	}
	before := counts()
	folder := t.TempDir()
	if r, out := docCmd[docSaveOut](t, "document-save", db, sixColumnReturns); r.code != 0 || !out.Saved {
		t.Fatalf("document-save: %s", r.stderr)
	}
	if after := counts(); after != before {
		t.Errorf("adding a document changed formats, rules, parameters or rows: %s -> %s", before, after)
	}
	if entries, _ := os.ReadDir(folder); len(entries) != 0 {
		t.Errorf("files appeared: %v", entries)
	}
}
