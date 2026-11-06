package vocab

import (
	"slices"
	"strings"
	"testing"
)

func baseVocab(t *testing.T) *Vocab {
	t.Helper()
	v, err := LoadDir(shippedDir)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func salesReturn() (OfficeDocument, OfficeRecordType) {
	return OfficeDocument{Code: "sales_return", Name: "Sales return register", Category: "Books", Description: "Credit notes issued to customers",
			Formats: []string{"csv", "xlsx"}, RecordType: "sales_return_row", Active: true},
		OfficeRecordType{Name: "sales_return_row", Description: "Rows of the sales return register", Columns: []OfficeColumn{
			{Name: "ref_no", Label: "Reference", Type: ColumnText, Key: true, Required: true},
			{Name: "doc_date", Label: "Date", Type: ColumnDate, Key: true},
			{Name: "net_amount", Label: "Net amount", Type: ColumnAmount},
		}}
}

func officeOf(d OfficeDocument, rt OfficeRecordType) Office {
	return Office{Documents: []OfficeDocument{d}, RecordTypes: []OfficeRecordType{rt}}
}

// A merged copy adds the ready office document and record type, keeps the base entries and the
// base hash, and does not change the base vocabulary.
func TestWithOfficeMerges(t *testing.T) {
	base := baseVocab(t)
	hash, docs, recs := base.ContentHash(), len(base.DocumentTypes()), len(base.RecordTypes())
	d, rt := salesReturn()
	m, err := base.WithOffice(officeOf(d, rt))
	if err != nil {
		t.Fatal(err)
	}
	if m.ContentHash() != hash || base.ContentHash() != hash {
		t.Error("the content hash changed")
	}
	if len(base.DocumentTypes()) != docs || len(base.RecordTypes()) != recs || base.IsRecordType("sales_return_row") || base.OfficeHash() != "" {
		t.Error("the base vocabulary was changed")
	}
	if len(m.DocumentTypes()) != docs+1 || len(m.RecordTypes()) != recs+1 || len(m.Facts()) != len(base.Facts()) {
		t.Errorf("merged counts: %d documents, %d record types", len(m.DocumentTypes()), len(m.RecordTypes()))
	}
	for _, b := range base.RecordTypes() {
		got, ok := m.RecordType(b.Name)
		if !ok || !slices.Equal(got.Amounts, b.Amounts) || !slices.Equal(got.MatchKey, b.MatchKey) {
			t.Fatalf("base record type %s differs in the merged copy", b.Name)
		}
	}
	got, ok := m.RecordType("sales_return_row")
	if !ok || got.DocumentType != "sales_return" || !slices.Equal(got.MatchKey, []string{"ref_no", "doc_date"}) ||
		!slices.Equal(got.Amounts, []string{"net_amount"}) || !slices.Equal(got.Fields, []string{"ref_no", "doc_date"}) {
		t.Errorf("record type = %+v", got)
	}
	if dt, ok := m.DocumentType("sales_return"); !ok || !dt.Office || dt.Prefix != "" {
		t.Errorf("document type = %+v", dt)
	}
	if cols, ok := m.OfficeColumns("sales_return_row"); !ok || cols[2].Label != "Net amount" {
		t.Errorf("columns = %+v", cols)
	}
	if _, ok := base.OfficeColumns("gstr1_b2b_invoice"); ok {
		t.Error("a built-in record type has office columns")
	}
}

// With nothing in the Master the merged copy equals the base, and the office hash is empty.
func TestWithOfficeEmptyEqualsBase(t *testing.T) {
	base := baseVocab(t)
	m, err := base.WithOffice(Office{})
	if err != nil {
		t.Fatal(err)
	}
	if m.OfficeHash() != "" || m.OfficeCanonical() != "" || m.ContentHash() != base.ContentHash() ||
		!slices.Equal(m.DocumentTypes(), base.DocumentTypes()) || len(m.RecordTypes()) != len(base.RecordTypes()) {
		t.Error("an empty office changed the vocabulary")
	}
}

func TestWithOfficeLeavesOutNotReady(t *testing.T) {
	base := baseVocab(t)
	d, rt := salesReturn()
	bad := rt
	bad.Columns = []OfficeColumn{{Name: "ref_no", Label: "R", Type: ColumnText}} // no key
	for name, o := range map[string]Office{
		"draft, no record type":    {Documents: []OfficeDocument{func() OfficeDocument { x := d; x.RecordType = ""; return x }()}},
		"record type not stored":   {Documents: []OfficeDocument{d}},
		"record type fails checks": officeOf(d, bad),
	} {
		m, err := base.WithOffice(o)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if _, ok := m.DocumentType("sales_return"); ok || m.IsRecordType("sales_return_row") || m.OfficeHash() != "" {
			t.Errorf("%s: a document that is not ready is in the merged vocabulary", name)
		}
	}
}

func TestWithOfficeRefusesCollisions(t *testing.T) {
	base := baseVocab(t)
	d, rt := salesReturn()
	first := base.DocumentTypes()[0]
	rec := base.RecordTypes()[0]
	with := func(f func(*OfficeDocument, *OfficeRecordType)) Office {
		dd, rr := d, rt
		f(&dd, &rr)
		return officeOf(dd, rr)
	}
	cases := map[string]Office{
		"base document name":         with(func(d *OfficeDocument, _ *OfficeRecordType) { d.Code = first.Name }),
		"base document, capitals":    with(func(d *OfficeDocument, _ *OfficeRecordType) { d.Code = strings.ToUpper(first.Name) }),
		"base prefix":                with(func(d *OfficeDocument, _ *OfficeRecordType) { d.Code = first.Prefix }),
		"base record type":           with(func(_ *OfficeDocument, r *OfficeRecordType) { r.Name = rec.Name }),
		"two documents, one code":    {Documents: []OfficeDocument{d, d}, RecordTypes: []OfficeRecordType{rt}},
		"two record types, one name": {Documents: []OfficeDocument{d}, RecordTypes: []OfficeRecordType{rt, rt}},
		"one record type, two documents": {Documents: []OfficeDocument{d, func() OfficeDocument { x := d; x.Code = "other_doc"; return x }()},
			RecordTypes: []OfficeRecordType{rt}},
	}
	for name, o := range cases {
		if name == "base record type" {
			o.Documents[0].RecordType = rec.Name
		}
		_, err := base.WithOffice(o)
		var nc *NameCollision
		if err == nil || !errorsAs(err, &nc) {
			t.Errorf("%s: err = %v, want a NameCollision", name, err)
		}
	}
}

func errorsAs(err error, target **NameCollision) bool {
	nc, ok := err.(*NameCollision)
	if ok {
		*target = nc
	}
	return ok
}

// The office hash is the same in any order, changes when a column changes, ignores Active and the
// order formats were given in, and is empty for nothing (AC-13).
func TestOfficeHash(t *testing.T) {
	base := baseVocab(t)
	d, rt := salesReturn()
	d2 := OfficeDocument{Code: "bank_extra", Name: "Bank extra", Category: "Bank", Description: "Extra bank lines", Formats: []string{"json"}, RecordType: "bank_extra_row", Active: true}
	rt2 := OfficeRecordType{Name: "bank_extra_row", Description: "Rows", Columns: []OfficeColumn{
		{Name: "line_no", Label: "Line", Type: ColumnText, Key: true}, {Name: "debit", Label: "Debit", Type: ColumnAmount}}}
	hashOf := func(o Office) string {
		m, err := base.WithOffice(o)
		if err != nil {
			t.Fatal(err)
		}
		return m.OfficeHash()
	}
	h := hashOf(Office{Documents: []OfficeDocument{d, d2}, RecordTypes: []OfficeRecordType{rt, rt2}})
	if len(h) != 64 {
		t.Fatalf("hash = %q", h)
	}
	for i := 0; i < 20; i++ {
		o := Office{Documents: []OfficeDocument{d, d2}, RecordTypes: []OfficeRecordType{rt, rt2}}
		if i%2 == 1 {
			o = Office{Documents: []OfficeDocument{d2, d}, RecordTypes: []OfficeRecordType{rt2, rt}}
		}
		if hashOf(o) != h {
			t.Fatalf("hash depends on order (round %d)", i)
		}
	}
	off := d
	off.Active = false
	off.Formats = []string{"xlsx", "csv", "csv"}
	if hashOf(Office{Documents: []OfficeDocument{off, d2}, RecordTypes: []OfficeRecordType{rt, rt2}}) != h {
		t.Error("the hash changed with Active or the order of formats")
	}
	for name, edit := range map[string]func(*OfficeRecordType){
		"rename":   func(r *OfficeRecordType) { r.Columns[2].Name = "net_value" },
		"retype":   func(r *OfficeRecordType) { r.Columns[2].Type = ColumnText },
		"relabel":  func(r *OfficeRecordType) { r.Columns[2].Label = "Value" },
		"required": func(r *OfficeRecordType) { r.Columns[1].Required = true },
		"add column": func(r *OfficeRecordType) {
			r.Columns = append(r.Columns, OfficeColumn{Name: "memo", Label: "Memo", Type: ColumnText})
		},
		"description": func(r *OfficeRecordType) { r.Description = "changed" },
	} {
		c := rt
		c.Columns = slices.Clone(rt.Columns)
		edit(&c)
		if hashOf(Office{Documents: []OfficeDocument{d, d2}, RecordTypes: []OfficeRecordType{c, rt2}}) == h {
			t.Errorf("%s did not change the hash", name)
		}
	}
}

// The canonical form reads back, with labels, so an old comparison shows what it had.
func TestOfficeSnapshotRoundTrip(t *testing.T) {
	base := baseVocab(t)
	d, rt := salesReturn()
	m, _ := base.WithOffice(officeOf(d, rt))
	back, err := ParseOfficeSnapshot(m.OfficeCanonical())
	if err != nil {
		t.Fatal(err)
	}
	again, err := base.WithOffice(back)
	if err != nil {
		t.Fatal(err)
	}
	if again.OfficeHash() != m.OfficeHash() {
		t.Error("a snapshot read back hashes differently")
	}
	if cols, _ := again.OfficeColumns("sales_return_row"); cols[0].Label != "Reference" || !cols[0].Required {
		t.Errorf("columns after the round trip: %+v", cols)
	}
	if _, err := ParseOfficeSnapshot("not json"); err == nil {
		t.Error("garbage was accepted")
	}
}

func TestValidateOfficeDocument(t *testing.T) {
	base := baseVocab(t)
	d, rt := salesReturn()
	if ps := base.ValidateOfficeDocument(Office{}, d, rt); len(ps) != 0 {
		t.Fatalf("a valid document has problems: %+v", ps)
	}
	first := base.DocumentTypes()[0]
	long := strings.Repeat("x", 61)
	cases := []struct {
		name  string
		edit  func(*OfficeDocument, *OfficeRecordType)
		field string
		code  string
	}{
		{"code too short", func(d *OfficeDocument, _ *OfficeRecordType) { d.Code = "ab" }, "code", BadDocumentCode},
		{"code 41 long", func(d *OfficeDocument, _ *OfficeRecordType) { d.Code = strings.Repeat("a", 41) }, "code", BadDocumentCode},
		{"code with capitals", func(d *OfficeDocument, _ *OfficeRecordType) { d.Code = "Sales" }, "code", BadDocumentCode},
		{"code is a base name", func(d *OfficeDocument, _ *OfficeRecordType) { d.Code = first.Name }, "code", BadDocumentCode},
		{"code is a base prefix", func(d *OfficeDocument, _ *OfficeRecordType) { d.Code = first.Prefix }, "code", BadDocumentCode},
		{"name too short", func(d *OfficeDocument, _ *OfficeRecordType) { d.Name = "ab" }, "name", BadName},
		{"name too long", func(d *OfficeDocument, _ *OfficeRecordType) { d.Name = long }, "name", BadName},
		{"name two lines", func(d *OfficeDocument, _ *OfficeRecordType) { d.Name = "ab\ncd" }, "name", BadName},
		{"description 9", func(d *OfficeDocument, _ *OfficeRecordType) { d.Description = "123456789" }, "description", BadDescription},
		{"description 301", func(d *OfficeDocument, _ *OfficeRecordType) { d.Description = strings.Repeat("d", 301) }, "description", BadDescription},
		{"category", func(d *OfficeDocument, _ *OfficeRecordType) { d.Category = "Misc" }, "category", BadCategory},
		{"no format", func(d *OfficeDocument, _ *OfficeRecordType) { d.Formats = nil }, "applicable_for", BadFormats},
		{"unknown format", func(d *OfficeDocument, _ *OfficeRecordType) { d.Formats = []string{"pdf"} }, "applicable_for", BadFormats},
		{"record type name short", func(d *OfficeDocument, r *OfficeRecordType) { r.Name = "ab" }, "record_type.name", BadRecordType},
		{"record type is built-in", func(_ *OfficeDocument, r *OfficeRecordType) { r.Name = base.RecordTypes()[0].Name }, "record_type.name", BadRecordType},
		{"no columns", func(_ *OfficeDocument, r *OfficeRecordType) { r.Columns = nil }, "columns", BadColumn},
		{"41 columns", func(_ *OfficeDocument, r *OfficeRecordType) {
			for i := 0; i < 38; i++ {
				r.Columns = append(r.Columns, OfficeColumn{Name: "col_" + strings.Repeat("a", 2) + string(rune('a'+i%26)) + string(rune('a'+i/26)), Label: "L", Type: ColumnText})
			}
		}, "columns", BadColumn},
		{"column name 1 char", func(_ *OfficeDocument, r *OfficeRecordType) { r.Columns[2].Name = "a" }, "columns[2].name", BadColumn},
		{"column label empty", func(_ *OfficeDocument, r *OfficeRecordType) { r.Columns[2].Label = " " }, "columns[2].label", BadColumn},
		{"column type", func(_ *OfficeDocument, r *OfficeRecordType) { r.Columns[2].Type = "number" }, "columns[2].type", BadColumn},
		{"amount key", func(_ *OfficeDocument, r *OfficeRecordType) { r.Columns[2].Key = true }, "columns[2].key", BadKey},
		{"no key", func(_ *OfficeDocument, r *OfficeRecordType) { r.Columns[0].Key, r.Columns[1].Key = false, false }, "columns", BadKey},
		{"keys only", func(_ *OfficeDocument, r *OfficeRecordType) { r.Columns = r.Columns[:2] }, "columns", BadColumn},
		{"duplicate column", func(_ *OfficeDocument, r *OfficeRecordType) { r.Columns[1].Name = "ref_no" }, "record_type", BadRecordType},
		{"column name capitals", func(_ *OfficeDocument, r *OfficeRecordType) { r.Columns[2].Name = "Net_amount" }, "record_type", BadRecordType},
	}
	for _, tc := range cases {
		dd, rr := d, rt
		rr.Columns = slices.Clone(rt.Columns)
		tc.edit(&dd, &rr)
		ps := base.ValidateOfficeDocument(Office{}, dd, rr)
		if !slices.ContainsFunc(ps, func(p Problem) bool { return p.Field == tc.field && p.Code == tc.code && p.Msg != "" }) {
			t.Errorf("%s: want a %s problem on %s, got %+v", tc.name, tc.code, tc.field, ps)
		}
	}
	// the boundaries that are allowed
	ok := d
	ok.Code, ok.Name, ok.Description = strings.Repeat("c", 3), strings.Repeat("n", 3), strings.Repeat("d", 10)
	if ps := base.ValidateOfficeDocument(Office{}, ok, rt); len(ps) != 0 {
		t.Errorf("lower boundaries refused: %+v", ps)
	}
	ok.Code, ok.Name, ok.Description = strings.Repeat("c", 40), strings.Repeat("n", 60), strings.Repeat("d", 300)
	if ps := base.ValidateOfficeDocument(Office{}, ok, rt); len(ps) != 0 {
		t.Errorf("upper boundaries refused: %+v", ps)
	}
	// every problem is returned together
	dd, rr := d, rt
	dd.Name, dd.Category, dd.Formats = "", "x", nil
	rr.Columns = nil
	if ps := base.ValidateOfficeDocument(Office{}, dd, rr); len(ps) < 4 {
		t.Errorf("problems are not returned together: %+v", ps)
	}
}

// Another office document's code and record type name, and its record type, are compared too.
func TestValidateOfficeDocumentAgainstOthers(t *testing.T) {
	base := baseVocab(t)
	d, rt := salesReturn()
	other := OfficeDocument{Code: "SALES_RETURN", Name: "Other", Category: "Books", Description: "Another document", Formats: []string{"csv"}, RecordType: "sales_return_row"}
	ps := base.ValidateOfficeDocument(officeOf(other, rt), d, rt)
	var code, name, owner bool
	for _, p := range ps {
		code = code || p.Field == "code"
		name = name || (p.Field == "record_type.name" && strings.Contains(p.Msg, "already used"))
		owner = owner || (p.Field == "record_type.name" && strings.Contains(p.Msg, "structure of"))
	}
	if !code || !name || !owner {
		t.Errorf("code %v, name %v, owner %v: %+v", code, name, owner, ps)
	}
	// a merged copy validates a new document against its own office entries through others only
	m, _ := base.WithOffice(officeOf(d, rt))
	if ps := m.ValidateOfficeDocument(Office{}, d, rt); len(ps) != 0 {
		t.Errorf("editing a document in a merged copy clashes with itself: %+v", ps)
	}
}
