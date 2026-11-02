package store

import (
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/audit"
)

// AC-8: the audit log is hash-chained, refuses updates and deletes, and a change made behind its
// back breaks the chain at the entry changed.
func TestAuditChainVerifiesAndRefusesEdits(t *testing.T) {
	db, _ := open(t)
	tx, _ := db.Begin()
	defer tx.Rollback()
	for i, action := range []string{"rule.create", "rule.disable", "rule.enable"} {
		if err := tx.Audit("2026-10-29T09:00:0"+string(rune('0'+i))+"Z", "A. Staff", action, "rule", "RECON-X-001 v1", map[string]string{"reason": "r"}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := tx.AuditEntries()
	if err != nil || len(entries) != 3 {
		t.Fatalf("entries = %d, err %v; want 3", len(entries), err)
	}
	if err := audit.Verify(entries); err != nil {
		t.Fatalf("a new log does not verify: %v", err)
	}
	if entries[0].PrevHash != audit.GenesisHash || entries[1].PrevHash != entries[0].Hash {
		t.Errorf("entries do not link to each other")
	}

	if _, err := tx.tx.Exec(`UPDATE audit_log SET actor = 'x' WHERE seq = 1`); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Errorf("update: %v, want the append-only refusal", err)
	}
	if _, err := tx.tx.Exec(`DELETE FROM audit_log WHERE seq = 1`); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Errorf("delete: %v, want the append-only refusal", err)
	}

	// Behind the triggers' back, a changed entry breaks the chain where it was changed.
	if _, err := tx.tx.Exec(`DROP TRIGGER audit_log_no_update`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.tx.Exec(`UPDATE audit_log SET details_json = '{"reason":"changed"}' WHERE seq = 2`); err != nil {
		t.Fatal(err)
	}
	entries, err = tx.AuditEntries()
	if err != nil {
		t.Fatal(err)
	}
	broken, ok := audit.Verify(entries).(*audit.BrokenError)
	if !ok || broken.Seq != 2 {
		t.Errorf("verify after a change to entry 2 = %v, want a break at entry 2", audit.Verify(entries))
	}
}

// The audit details are written with their keys sorted, so the same change has the same hash.
func TestAuditDetailsAreCanonical(t *testing.T) {
	db, _ := open(t)
	tx, _ := db.Begin()
	defer tx.Rollback()
	if err := tx.Audit("2026-10-29T09:00:00Z", "A. Staff", "rule.disable", "rule", "RECON-X-001 v1", map[string]string{"source": "office", "reason": "r"}); err != nil {
		t.Fatal(err)
	}
	entries, _ := tx.AuditEntries()
	if entries[0].Details != `{"reason":"r","source":"office"}` {
		t.Errorf("details = %s, want the keys sorted", entries[0].Details)
	}
}

// An office version of a built-in rule takes its place in use; a later built-in text does not
// replace it until the office edit is reset (RU-12).
func TestOfficeVersionStandsOverBuiltIn(t *testing.T) {
	db, _ := open(t)
	tx, _ := db.Begin()
	defer tx.Rollback()
	st := RuleStatus{Status: StatusTesting}
	builtin := RuleFile{RuleID: "RECON-X-001", Version: "1", Source: SourceBuiltin, Schema: "recon.rule/1", TaxYears: "2025-26", FilePath: "rules/a", Body: []byte("b1"), SHA256: "b1"}
	office := RuleFile{RuleID: "RECON-X-001", Version: "1-office.1", Source: SourceOffice, Schema: "recon.rule/1", TaxYears: "2025-26", FilePath: "office/a", Body: []byte("o1"), SHA256: "o1"}
	if _, err := tx.PutRule(builtin, st); err != nil {
		t.Fatal(err)
	}
	if changed, err := tx.PutRule(office, st); err != nil || !changed {
		t.Fatalf("office version: changed %v, err %v", changed, err)
	}
	if s, _, _ := tx.RuleStateOf("RECON-X-001"); s.ActiveSource != SourceOffice {
		t.Fatalf("the office version is not in use: %+v", s)
	}
	builtin.Body, builtin.SHA256 = []byte("b2"), "b2"
	if _, err := tx.PutRule(builtin, st); err != nil {
		t.Fatal(err)
	}
	if s, _, _ := tx.RuleStateOf("RECON-X-001"); s.ActiveSource != SourceOffice {
		t.Errorf("a new built-in text replaced the office version in use: %+v", s)
	}
	vs, err := tx.RuleVersions("RECON-X-001")
	if err != nil || len(vs) != 3 || !vs[1].Active || vs[0].Source != SourceBuiltin || vs[0].Revision != 1 || vs[2].Revision != 3 {
		t.Fatalf("versions = %+v, err %v; want builtin 1, office 2, builtin 3", vs, err)
	}
	if err := tx.ActivateVersion("RECON-X-001", vs[2].ID); err != nil {
		t.Fatal(err)
	}
	if s, _, _ := tx.RuleStateOf("RECON-X-001"); s.ActiveSource != SourceBuiltin || s.ActiveVersionID != vs[2].ID {
		t.Errorf("after activating the standard version: %+v", s)
	}
	if latest, ok, _ := tx.LatestVersionOf("RECON-X-001", SourceOffice); !ok || latest.Version != "1-office.1" {
		t.Errorf("latest office version = %+v, %v", latest, ok)
	}
}

// Disabling sets the status and its reason on the rule's state, and keeps every version; a
// version of another rule cannot be made the one in use.
func TestSetRuleStatusKeepsVersions(t *testing.T) {
	db, _ := open(t)
	tx, _ := db.Begin()
	defer tx.Rollback()
	st := RuleStatus{Status: StatusTesting}
	a := RuleFile{RuleID: "RECON-X-001", Version: "1", Source: SourceBuiltin, Schema: "recon.rule/1", Body: []byte("a"), SHA256: "a"}
	b := RuleFile{RuleID: "RECON-X-002", Version: "1", Source: SourceBuiltin, Schema: "recon.rule/1", Body: []byte("b"), SHA256: "b"}
	if _, err := tx.PutRule(a, st); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.PutRule(b, st); err != nil {
		t.Fatal(err)
	}
	if err := tx.SetRuleStatus("RECON-X-001", StatusDisabled, "wrong on test data"); err != nil {
		t.Fatal(err)
	}
	if s, _, _ := tx.RuleStateOf("RECON-X-001"); s.Status != StatusDisabled || s.DisabledReason != "wrong on test data" {
		t.Errorf("state after disable = %+v", s)
	}
	vs, _ := tx.RuleVersions("RECON-X-001")
	if len(vs) != 1 || !vs[0].Active {
		t.Errorf("the version was lost or unmarked: %+v", vs)
	}
	other, _ := tx.RuleVersions("RECON-X-002")
	if err := tx.ActivateVersion("RECON-X-001", other[0].ID); err == nil {
		t.Errorf("activated another rule's version")
	}
	if _, ok, _ := tx.RuleStateOf("RECON-X-404"); ok {
		t.Errorf("a rule that does not exist was found")
	}
}
