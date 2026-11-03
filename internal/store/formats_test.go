package store

import (
	"strings"
	"testing"
)

func formatV(id, status string) FormatVersion {
	return FormatVersion{FormatID: id, Name: "Form 26AS (Excel)", Source: OriginOffice, FileType: "xlsx", Document: "Form 26AS",
		BodyYAML: "id: " + id + "\n", SHA256: "aa", Status: status, CreatedBy: "A. Partner", PublishedAt: "2026-11-03T10:00:00Z", PublishedBy: "A. Partner"}
}

func TestFormatVersionsAreKeptAndSwitched(t *testing.T) {
	db, _ := open(t)
	tx, _ := db.Begin()
	defer tx.Rollback()
	v1, err := tx.AddFormatVersion(formatV("form26as", FormatTesting))
	if err != nil {
		t.Fatal(err)
	}
	v2, err := tx.AddFormatVersion(formatV("form26as", FormatTesting))
	if err != nil {
		t.Fatal(err)
	}
	vs, _ := tx.FormatVersions("form26as")
	if len(vs) != 2 || vs[0].Version != "v1" || vs[1].Version != "v2" {
		t.Fatalf("versions = %+v", vs)
	}
	fs, _ := tx.Formats()
	if len(fs) != 1 || fs[0].State.ActiveVersionID != v2 {
		t.Fatalf("formats = %+v", fs)
	}
	// Switching back uses an earlier version and changes none.
	if err := tx.UseFormatVersion("form26as", v1, FormatTesting); err != nil {
		t.Fatal(err)
	}
	if s, _, _ := tx.FormatStateOf("form26as"); s.ActiveVersionID != v1 {
		t.Errorf("active = %d, want %d", s.ActiveVersionID, v1)
	}
	if err := tx.UseFormatVersion("other", v1, FormatTesting); err == nil {
		t.Error("a version of another format was accepted")
	}
}

func TestFormatDisableNeedsReason(t *testing.T) {
	db, _ := open(t)
	tx, _ := db.Begin()
	defer tx.Rollback()
	if _, err := tx.AddFormatVersion(formatV("f", FormatActive)); err != nil {
		t.Fatal(err)
	}
	if err := tx.SetFormatStatus("f", FormatDisabled, "", "A", "2026-11-03T10:00:00Z"); err == nil {
		t.Error("disabled with no reason")
	}
	if err := tx.SetFormatStatus("f", FormatDisabled, "replaced", "A", "2026-11-03T10:00:00Z"); err != nil {
		t.Fatal(err)
	}
	s, _, _ := tx.FormatStateOf("f")
	if s.Status != FormatDisabled || s.DisabledReason != "replaced" {
		t.Errorf("state = %+v", s)
	}
	if err := tx.SetFormatStatus("f", FormatActive, "", "B. Partner", "2026-11-03T11:00:00Z"); err != nil {
		t.Fatal(err)
	}
	s, _, _ = tx.FormatStateOf("f")
	if s.DisabledReason != "" || s.PublishedBy != "B. Partner" || s.PublishedAt != "2026-11-03T11:00:00Z" {
		t.Errorf("after enabling: %+v", s)
	}
}

// A version is never changed or removed.
func TestFormatAndParameterVersionsRefuseChange(t *testing.T) {
	db, _ := open(t)
	tx, _ := db.Begin()
	if _, err := tx.AddFormatVersion(formatV("f", FormatTesting)); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.AddParameterVersion(ParameterVersion{ParameterID: "office.advance", Name: "Advance", Source: OriginOffice,
		ParamType: "money", Document: "Expense ledger", BodyYAML: "key: office.advance\n", SHA256: "bb", CreatedBy: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`UPDATE format_versions SET name = 'x'`, `DELETE FROM format_versions`,
		`UPDATE parameter_versions SET name = 'x'`, `DELETE FROM parameter_versions`,
	} {
		if _, err := db.db.Exec(q); err == nil || !strings.Contains(err.Error(), "never") {
			t.Errorf("%s: err = %v", q, err)
		}
	}
}

func TestParameterStatusAndSamples(t *testing.T) {
	db, _ := open(t)
	tx, _ := db.Begin()
	defer tx.Rollback()
	if _, err := tx.AddParameterVersion(ParameterVersion{ParameterID: "office.advance", Name: "Advance", Source: OriginOffice,
		ParamType: "money", Document: "Expense ledger", BodyYAML: "k: v\n", SHA256: "bb", CreatedBy: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := tx.SetParameterStatus("office.advance", "disabled", ""); err == nil {
		t.Error("disabled with no reason")
	}
	if err := tx.SetParameterStatus("office.advance", "disabled", "not used"); err != nil {
		t.Fatal(err)
	}
	ps, _ := tx.Parameters()
	if len(ps) != 1 || ps[0].State.Status != "disabled" {
		t.Fatalf("parameters = %+v", ps)
	}
	// A new version of a disabled parameter stays disabled.
	if _, err := tx.AddParameterVersion(ParameterVersion{ParameterID: "office.advance", Name: "Advance rec.", Source: OriginOffice,
		ParamType: "money", Document: "Expense ledger", BodyYAML: "k: w\n", SHA256: "cc", CreatedBy: "A"}); err != nil {
		t.Fatal(err)
	}
	if ps, _ := tx.Parameters(); ps[0].State.Status != "disabled" || ps[0].Version.Version != "v2" {
		t.Errorf("after new version: %+v", ps[0])
	}

	if _, err := tx.AddFormatVersion(formatV("f", FormatTesting)); err != nil {
		t.Fatal(err)
	}
	s := FormatSample{FormatID: "f", FileSHA256: "dd", FileName: "sample.xlsx", SizeBytes: 3, KeptBy: "A"}
	if _, err := tx.KeepSample(s); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.KeepSample(s); err != nil { // the same file is kept once
		t.Fatal(err)
	}
	got, _ := tx.FormatSamples("f")
	if len(got) != 1 || got[0].Content != nil {
		t.Fatalf("samples = %+v", got)
	}
	if err := tx.RemoveSample(got[0].ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := tx.FormatSamples("f"); len(got) != 0 {
		t.Errorf("sample not removed: %+v", got)
	}
}

// SPEC-016: an edit is a draft that no run uses until it is published; a version is published
// once; only a published version can be switched back to.
func TestFormatDraftAndPublish(t *testing.T) {
	db, _ := open(t)
	tx, _ := db.Begin()
	defer tx.Rollback()
	v1, err := tx.AddFormatVersion(formatV("f", FormatActive))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.AddFormatDraft(formatV("nope", FormatActive)); err == nil {
		t.Error("a draft of a format that is not stored was accepted")
	}
	d := formatV("f", FormatActive) // the status is replaced by draft
	d.MappingJSON, d.ProfileVersion = `{"name":"x"}`, "2"
	draft, err := tx.AddFormatDraft(d)
	if err != nil {
		t.Fatal(err)
	}
	if st, _, _ := tx.FormatStateOf("f"); st.ActiveVersionID != v1 || st.Status != FormatActive {
		t.Fatalf("a draft changed the version in use: %+v", st)
	}
	got, _, _ := tx.FormatVersionByID(draft)
	if got.Status != FormatDraft || got.PublishedAt != "" || got.MappingJSON == "" {
		t.Fatalf("draft = %+v", got)
	}
	if err := tx.UseFormatVersion("f", draft, FormatTesting); err == nil {
		t.Error("an unpublished draft was switched to")
	}
	if err := tx.PublishFormatVersion("f", draft, "draft", "A", "2026-11-03T11:00:00Z"); err == nil {
		t.Error("published as draft")
	}
	if err := tx.PublishFormatVersion("f", draft, FormatTesting, "A. Partner", "2026-11-03T11:00:00Z"); err != nil {
		t.Fatal(err)
	}
	st, _, _ := tx.FormatStateOf("f")
	got, _, _ = tx.FormatVersionByID(draft)
	if st.ActiveVersionID != draft || st.Status != FormatTesting || got.PublishedAt != "2026-11-03T11:00:00Z" || got.PublishedBy != "A. Partner" {
		t.Errorf("after publishing: %+v, %+v", st, got)
	}
	// Publishing is recorded once, and the rest of a version cannot change.
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{`UPDATE format_versions SET published_at = 'x'`, `UPDATE format_versions SET body_yaml = 'x'`} {
		if _, err := db.db.Exec(q); err == nil {
			t.Errorf("%s was accepted", q)
		}
	}
	tx2, _ := db.Begin()
	defer tx2.Rollback()
	if v, ok, _ := tx2.FormatVersionBySHA("f", "aa"); !ok || v.Revision != 2 {
		t.Errorf("by sha = %+v, %v", v, ok)
	}
}
