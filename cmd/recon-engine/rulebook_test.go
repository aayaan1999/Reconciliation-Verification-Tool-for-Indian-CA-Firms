package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rulestore"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
)

// SPEC-010 scope 8: the whole rule-book rule in the summary, and a Partner's edit, disable and
// enable of a rule-book rule. The rule-book file itself is never written.

const rbID = "RC-ACC-003"

func rbFile() string {
	return filepath.Join(storeDir, "rules", "reconciliation", "accounting", rbID+".yaml")
}

func rulebookSaveCmd(t *testing.T, db, by, reason string, r rulestore.Rule) result {
	t.Helper()
	body, err := json.Marshal(map[string]any{"by": by, "reason": reason, "rule": r})
	if err != nil {
		t.Fatal(err)
	}
	return withStdin(t, string(body), "rulebook-save", "--db", db, "--rule", rbID, "--json", filepath.Join(t.TempDir(), "s.json"))
}

func activeOf(t *testing.T, db string) []string {
	t.Helper()
	return query(t, db, `SELECT v.source, v.schema, s.status FROM rule_state s JOIN rule_versions v ON v.id = s.active_version_id WHERE s.rule_id = ?`, rbID)[0]
}

// AC-21: rule-get returns the whole rule-book rule, as its file holds it.
func TestRuleGetReturnsTheWholeRulebookRule(t *testing.T) {
	db := newDB(t)
	rc := getRule(t, db, officeRoot(t), rbID)
	if rc.Rulebook == nil {
		t.Fatalf("rule-get gave no rule-book rule for %s", rbID)
	}
	data, err := os.ReadFile(rbFile())
	if err != nil {
		t.Fatal(err)
	}
	var want rulestore.Rule
	if _, err := strictyaml.Decode(rbID, data, &want); err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(rc.Rulebook)
	exp, _ := json.Marshal(want)
	if !bytes.Equal(got, exp) {
		t.Errorf("rule-get's rule-book rule differs from the file:\n got %s\nwant %s", got, exp)
	}
	if !rc.HasRulebookVersion || rc.Source != "rulebook" {
		t.Errorf("source %q, has rule-book version %v", rc.Source, rc.HasRulebookVersion)
	}
}

// AC-22, AC-23: a Partner's edit is a new office version that reads back strictly; the
// rule-book file is unchanged; a reload keeps the edit in use; a reset puts the rule book back.
func TestRulebookEditIsAnOfficeVersion(t *testing.T) {
	before, err := os.ReadFile(rbFile())
	if err != nil {
		t.Fatal(err)
	}
	db, office := newDB(t), officeRoot(t)
	rc := getRule(t, db, office, rbID)
	versions := func() string { return query(t, db, `SELECT COUNT(*) FROM rule_versions WHERE rule_id = ?`, rbID)[0][0] }
	n0 := versions()

	// Refusals write nothing.
	for name, tc := range map[string]struct {
		reason string
		change func(r *rulestore.Rule)
		code   string
	}{
		"no reason":      {"  ", func(r *rulestore.Rule) { r.Rule.Purpose += " More." }, "REASON_REQUIRED"},
		"changed ID":     {"why", func(r *rulestore.Rule) { r.Rule.ID = "RC-ACC-099" }, "ID_CHANGED"},
		"changed type":   {"why", func(r *rulestore.Rule) { r.Rule.Type = "statutory" }, "TYPE_CHANGED"},
		"changed domain": {"why", func(r *rulestore.Rule) { r.Rule.Domain = "gst" }, "DOMAIN_CHANGED"},
		"no change":      {"why", func(r *rulestore.Rule) {}, "NO_CHANGE"},
	} {
		r := *rc.Rulebook
		tc.change(&r)
		if res := rulebookSaveCmd(t, db, "A. Partner", tc.reason, r); res.code != exitInput || !strings.Contains(res.stderr, tc.code) {
			t.Errorf("%s: exit %d, stderr %q, want %s", name, res.code, res.stderr, tc.code)
		}
	}
	if versions() != n0 {
		t.Fatalf("a refused save wrote a version")
	}

	edited := *rc.Rulebook
	edited.Rule.Purpose = "Edited purpose.\nSecond line."
	edited.Notes = "Changed by the Partner."
	if res := rulebookSaveCmd(t, db, "A. Partner", "wording agreed with the CA", edited); res.code != 0 {
		t.Fatalf("save: exit %d\n%s%s", res.code, res.stdout, res.stderr)
	}
	if got := activeOf(t, db); !slices.Equal(got, []string{"office", rulestoreSchema, "disabled"}) {
		t.Errorf("after the edit, in use = %v, want an office rule-store version with the status unchanged", got)
	}
	body := query(t, db, `SELECT v.body_yaml FROM rule_state s JOIN rule_versions v ON v.id = s.active_version_id WHERE s.rule_id = ?`, rbID)[0][0]
	var back rulestore.Rule
	if _, err := strictyaml.Decode("office.yaml", []byte(body), &back); err != nil {
		t.Fatalf("the saved version does not read back strictly: %v", err)
	}
	if back.Rule.Purpose != edited.Rule.Purpose || back.Notes != edited.Notes {
		t.Errorf("saved purpose %q, notes %q", back.Rule.Purpose, back.Notes)
	}
	if back.Governance.LastModifiedBy != "A. Partner" || back.Governance.ChangeNote != "wording agreed with the CA" || len(back.Governance.LastModifiedOn) != 10 {
		t.Errorf("governance after edit = %+v", back.Governance)
	}
	if got := auditActions(t, db); !slices.Contains(got, "rule.edit") {
		t.Errorf("audit actions %v, want rule.edit", got)
	}
	if rc2 := getRule(t, db, office, rbID); rc2.Rulebook == nil || rc2.Rulebook.Rule.Purpose != edited.Rule.Purpose || rc2.Source != "office" {
		t.Errorf("rule-get after the edit does not show it")
	}

	// Loading the rule book again keeps the office version in use.
	if r := loadRulesCmd(t, db, rulesDir); r.code != 0 {
		t.Fatalf("load-rules: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if got := activeOf(t, db); got[0] != "office" {
		t.Errorf("a reload replaced the Partner's edit: in use %v", got)
	}

	// Reset to the rule-book version.
	if r := withStdin(t, `{"by":"A. Partner"}`, "rule-versions", "--db", db, "--rule", rbID, "--reset", "--json", filepath.Join(t.TempDir(), "v.json")); r.code != 0 {
		t.Fatalf("reset: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if got := activeOf(t, db); !slices.Equal(got, []string{"rulebook", rulestoreSchema, "disabled"}) {
		t.Errorf("after reset, in use = %v", got)
	}
	if got := auditActions(t, db); !slices.Contains(got, "rule.reset") {
		t.Errorf("audit actions %v, want rule.reset", got)
	}

	after, err := os.ReadFile(rbFile())
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("the rule-book file changed")
	}
}

// AC-24: a Partner enables and disables a rule-book rule with a reason, and a reload of the
// rule book does not change the status a person set.
func TestRulebookDisableAndEnableStandAcrossReload(t *testing.T) {
	db := newDB(t)
	sw := func(cmd, reason string) result {
		return withStdin(t, `{"by":"A. Partner","reason":"`+reason+`"}`, cmd, "--db", db, "--rule", rbID, "--json", filepath.Join(t.TempDir(), "s.json"))
	}
	reload := func() {
		if r := loadRulesCmd(t, db, rulesDir); r.code != 0 {
			t.Fatalf("load-rules: exit %d\n%s%s", r.code, r.stdout, r.stderr)
		}
	}
	state := func() []string {
		return query(t, db, `SELECT status, disabled_reason, status_set_by FROM rule_state WHERE rule_id = ?`, rbID)[0]
	}
	if got := state(); got[0] != "disabled" || got[2] != "" {
		t.Fatalf("a fresh load gave %v, want disabled following the file", got)
	}
	if r := sw("rule-enable", ""); r.code != exitInput || !strings.Contains(r.stderr, "REASON_REQUIRED") {
		t.Errorf("enable without a reason: exit %d, stderr %q", r.code, r.stderr)
	}
	if r := sw("rule-enable", "the CA wants it listed as in use"); r.code != 0 {
		t.Fatalf("enable: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	reload()
	if got := state(); !slices.Equal(got, []string{"active", "", "A. Partner"}) {
		t.Errorf("after enable and reload: %v", got)
	}
	if r := sw("rule-disable", "not used this season"); r.code != 0 {
		t.Fatalf("disable: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	reload()
	if got := state(); !slices.Equal(got, []string{"disabled", "not used this season", "A. Partner"}) {
		t.Errorf("after disable and reload: %v", got)
	}
	got := auditActions(t, db)
	if !slices.Contains(got, "rule.enable") || !slices.Contains(got, "rule.disable") {
		t.Errorf("audit actions %v", got)
	}
}
