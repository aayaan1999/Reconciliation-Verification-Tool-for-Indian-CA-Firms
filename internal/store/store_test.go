package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
)

func open(t *testing.T) (*DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "recon.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db, path
}

// Opening twice applies the migrations once; the second open writes nothing.
func TestMigrationsApplyOnce(t *testing.T) {
	db, path := open(t)
	db.Close()
	db2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	var n int
	if err := db2.db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil || n != 13 {
		t.Fatalf("schema_migrations rows = %d, err %v", n, err)
	}
	var mode string
	if err := db2.db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "wal" {
		t.Errorf("journal_mode = %q, err %v", mode, err)
	}
}

func TestPutRuleVersions(t *testing.T) {
	db, _ := open(t)
	tx, _ := db.Begin()
	defer tx.Rollback()
	f := RuleFile{RuleID: "RECON-X-001", Version: "1", Source: SourceBuiltin, Schema: "recon.rule/1", Body: []byte("a"), SHA256: "aa"}
	st := RuleStatus{Status: StatusTesting}
	for i, want := range []bool{true, false} {
		if changed, err := tx.PutRule(f, st); err != nil || changed != want {
			t.Fatalf("load %d: changed %v, err %v", i, changed, err)
		}
	}
	if _, err := tx.tx.Exec(`UPDATE rule_state SET status = 'active'`); err != nil {
		t.Fatal(err)
	}
	f.Body, f.SHA256 = []byte("b"), "bb"
	if changed, err := tx.PutRule(f, st); err != nil || !changed {
		t.Fatalf("changed file: changed %v, err %v", changed, err)
	}
	rs, err := tx.Rules()
	if err != nil || len(rs) != 1 || rs[0].Revision != 2 || rs[0].Status != StatusTesting || string(rs[0].Body) != "b" {
		t.Fatalf("rules = %+v, err %v; want revision 2, back to testing", rs, err)
	}
	f.Source = SourceRulebook
	if _, err := tx.PutRule(f, st); err == nil {
		t.Error("loading the same rule ID from another source was accepted")
	}
}

func TestUploadsAndCurrentView(t *testing.T) {
	db, _ := open(t)
	tx, _ := db.Begin()
	defer tx.Rollback()
	eng, err := tx.Engagement("clients/a", "2025-26")
	if err != nil {
		t.Fatal(err)
	}
	fact := func(p money.Paise) domain.Fact {
		return domain.Fact{Name: "computation.allowed_depreciation", Period: "FY2025-26", Value: domain.MoneyValue(p),
			Locator: domain.Locator{Kind: domain.LocatorJSONPath, Path: "$.x"}}
	}
	up := Upload{EngagementID: eng, Slot: Slot{DocumentType: "computation", Period: "FY2025-26"}, FileName: "a.json",
		SHA256: "s1", ProfileID: "p", ProfileVersion: "1", UploadedBy: "test", OK: true, ReportJSON: []byte("{}"),
		Facts: []domain.Fact{fact(100)}}
	first, isNew, err := tx.PutUpload(up)
	if err != nil || !isNew || first.DraftNo != 1 {
		t.Fatalf("first upload: %+v new %v err %v", first, isNew, err)
	}
	again, isNew, err := tx.PutUpload(up)
	if err != nil || isNew || again.ID != first.ID {
		t.Fatalf("same file again: %+v new %v err %v; want the same version", again, isNew, err)
	}
	up.SHA256, up.FileName, up.Facts = "s2", "b.json", []domain.Fact{fact(250)}
	second, isNew, err := tx.PutUpload(up)
	if err != nil || !isNew || second.DraftNo != 2 {
		t.Fatalf("new file: %+v new %v err %v", second, isNew, err)
	}
	docs, facts, err := tx.CurrentView(eng)
	if err != nil || len(docs) != 1 || docs[0].ID != second.ID || len(facts) != 1 {
		t.Fatalf("current view: %+v %+v err %v", docs, facts, err)
	}
	f := facts[0]
	if f.Money != 250 || f.Locator.FileName != "b.json" || f.Locator.DocumentVersionID != second.ID {
		t.Errorf("fact read back = %+v", f)
	}

	// The first file uploaded again becomes the latest draft: the current view is always
	// what was uploaded last.
	up.SHA256, up.FileName, up.Facts = "s1", "a.json", []domain.Fact{fact(100)}
	third, isNew, err := tx.PutUpload(up)
	if err != nil || !isNew || third.DraftNo != 3 {
		t.Fatalf("older file again: %+v new %v err %v; want draft 3", third, isNew, err)
	}
	if _, facts, err := tx.CurrentView(eng); err != nil || len(facts) != 1 || facts[0].Money != 100 {
		t.Errorf("current view after re-upload: %+v err %v", facts, err)
	}
	// The same file read with a new profile version is a new draft with the new reading.
	up.ProfileVersion = "2"
	if fourth, isNew, err := tx.PutUpload(up); err != nil || !isNew || fourth.DraftNo != 4 {
		t.Errorf("new profile version: %+v new %v err %v; want draft 4", fourth, isNew, err)
	}
}

// SPEC-004 AC-21: a client keeps only the PAN's keyed hash and last four characters; the same
// PAN twice is refused; the hash matches the PAN in any case.
func TestClients(t *testing.T) {
	db, path := open(t)
	tx, _ := db.Begin()
	c, err := tx.AddClient(" Illustrative Industries Private Limited ", "aaaci1234q")
	if err != nil || c.Name != "Illustrative Industries Private Limited" || c.PANLast4 != "234Q" || len(c.PANHash) != 64 {
		t.Fatalf("client = %+v, err %v", c, err)
	}
	if _, err := tx.AddClient("Other", "AAACI1234Q"); err == nil {
		t.Error("a second client with the same PAN was accepted")
	}
	if h, _ := tx.PANHash("AAACI1234Q "); h != c.PANHash {
		t.Error("the hash depends on case or spaces")
	}
	eng, err := tx.ClientEngagement(c.ID, "2025-26")
	if err != nil {
		t.Fatal(err)
	}
	if again, found, _ := tx.FindClientEngagement(c.ID, "2025-26"); !found || again != eng {
		t.Errorf("engagement not found again: %d %v", again, found)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	db.Close()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(bytes.ToUpper(raw), []byte("AAACI1234Q")) {
		t.Error("the full PAN is in the database file")
	}
}
