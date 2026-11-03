package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// SPEC-010 scope 9, AC-29 and AC-30 (engine part): several rules switched off or on with one
// reason, in one transaction, with one audit entry each.
func TestRuleSwitchSeveralAllOrNone(t *testing.T) {
	db := newDB(t)
	var book []string // three rule-book rules, switched off by the file on load
	for _, row := range query(t, db, `SELECT rule_id FROM rule_state WHERE rule_id LIKE 'RC-%' ORDER BY rule_id LIMIT 3`) {
		book = append(book, row[0])
	}
	if len(book) != 3 {
		t.Fatalf("rule-book rules %v", book)
	}
	const engineRule = "RECON-DEP-001" // testing on load
	status := func(id string) string {
		t.Helper()
		return query(t, db, `SELECT status FROM rule_state WHERE rule_id = ?`, id)[0][0]
	}
	sw := func(command, reason string, ids ...string) result {
		t.Helper()
		return withStdin(t, `{"by":"A. Partner","reason":"`+reason+`"}`, command, "--db", db, "--rules", strings.Join(ids, ","),
			"--json", filepath.Join(t.TempDir(), "s.json"))
	}
	audits := len(auditActions(t, db))

	// One refusal refuses the whole request; nothing changes.
	for _, c := range []struct {
		command, reason, code string
		ids                   []string
	}{
		{"rule-enable", "", "REASON_REQUIRED", book},
		{"rule-enable", "listed", "RULE_NOT_FOUND", append(slices.Clone(book), "RC-NOPE-999")},
		{"rule-enable", "listed", "NOT_DISABLED", append(slices.Clone(book), engineRule)},
		{"rule-enable", "listed", "BAD_REQUEST", []string{book[0], book[0]}},
	} {
		if r := sw(c.command, c.reason, c.ids...); r.code != exitInput || !strings.Contains(r.stderr, c.code) {
			t.Errorf("%s %v: exit %d, want %s\n%s", c.command, c.ids, r.code, c.code, r.stderr)
		}
	}
	for _, id := range book {
		if s := status(id); s != "disabled" {
			t.Errorf("%s is %s after refused requests, want disabled", id, s)
		}
	}
	if n := len(auditActions(t, db)); n != audits {
		t.Errorf("refused requests wrote %d audit entries", n-audits)
	}
	if r := withStdin(t, `{"by":"A. Partner","reason":"x"}`, "rule-enable", "--db", db, "--rule", book[0], "--rules", book[1], "--json", filepath.Join(t.TempDir(), "s.json")); r.code != exitInput {
		t.Errorf("--rule with --rules: exit %d", r.code)
	}
	many := make([]string, maxSwitchRules+1)
	for i := range many {
		many[i] = "RC-X-" + strings.Repeat("1", 3) + string(rune('A'+i%26)) + strings.Repeat("Z", i/26)
	}
	if r := sw("rule-enable", "x", many...); r.code != exitInput || !strings.Contains(r.stderr, "TOO_MANY_RULES") {
		t.Errorf("%d rules: exit %d\n%s", len(many), r.code, r.stderr)
	}

	// All three switched on with one reason: one audit entry each.
	out := filepath.Join(t.TempDir(), "s.json")
	r := withStdin(t, `{"by":"A. Partner","reason":"listed as in use"}`, "rule-enable", "--db", db, "--rules", strings.Join(book, ","), "--json", out)
	if r.code != exitOK {
		t.Fatalf("rule-enable --rules: exit %d\n%s", r.code, r.stderr)
	}
	var res struct {
		Rules []struct {
			RuleID, Status string
		} `json:"rules"`
		Reason string `json:"reason"`
	}
	if b, err := os.ReadFile(out); err != nil {
		t.Fatal(err)
	} else if err := json.Unmarshal(b, &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Rules) != 3 || res.Reason != "listed as in use" {
		t.Errorf("rule-enable --rules wrote %+v", res)
	}
	for _, id := range book {
		if s := status(id); s != "active" {
			t.Errorf("%s is %s after enable, want active (a rule-book rule)", id, s)
		}
	}
	rows := query(t, db, `SELECT action, actor, details_json FROM audit_log ORDER BY seq`)[audits:]
	if len(rows) != 3 {
		t.Fatalf("enable wrote %d audit entries, want 3", len(rows))
	}
	for _, row := range rows {
		if row[0] != "rule.enable" || row[1] != "A. Partner" || !strings.Contains(row[2], `"reason":"listed as in use"`) {
			t.Errorf("audit entry %v", row)
		}
	}

	// A rule-book rule and an engine rule switched off together; the engine rule comes back as testing.
	if r := sw("rule-disable", "not used this season", book[0], engineRule); r.code != exitOK {
		t.Fatalf("rule-disable --rules: exit %d\n%s", r.code, r.stderr)
	}
	if status(book[0]) != "disabled" || status(engineRule) != "disabled" || status(book[1]) != "active" {
		t.Errorf("after disable: %s %s %s", status(book[0]), status(engineRule), status(book[1]))
	}
	if r := sw("rule-enable", "back", book[0], engineRule); r.code != exitOK {
		t.Fatalf("rule-enable: exit %d\n%s", r.code, r.stderr)
	}
	if status(book[0]) != "active" || status(engineRule) != "testing" {
		t.Errorf("after enable: %s %s, want active and testing", status(book[0]), status(engineRule))
	}
}
