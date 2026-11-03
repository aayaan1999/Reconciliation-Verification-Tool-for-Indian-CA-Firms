package api

import (
	"net/http"
	"strings"
	"testing"
)

// SPEC-010 scope 9 over HTTP (AC-29, AC-30): several rules switched with one reason, all or none,
// by a Partner only.
func TestBulkRuleSwitchThroughServer(t *testing.T) {
	e := newEnv(t) // signed in as the first Partner
	base := e.srv.URL + "/api/v1/rules/bulk"
	ids := `["RC-ACC-001","RC-ACC-002","RC-ACC-003"]`

	staff, manager, readOnly := users(t, e)
	for name, c := range map[string]*http.Client{"staff": staff, "manager": manager, "read_only": readOnly} {
		for _, action := range []string{"/disable", "/enable"} {
			if status, code := codeAndStatus(t, c, "POST", base+action, `{"rule_ids":`+ids+`,"reason":"x"}`); status != http.StatusForbidden || code != "FORBIDDEN" {
				t.Errorf("%s %s: %d %s, want 403", name, action, status, code)
			}
		}
	}

	for _, c := range []struct {
		body, code string
		status     int
	}{
		{`{"rule_ids":[],"reason":"x"}`, "NO_RULES_SELECTED", http.StatusBadRequest},
		{`{"rule_ids":["../etc"],"reason":"x"}`, "BAD_RULE_ID", http.StatusBadRequest},
		{`{"rule_ids":` + ids + `,"reason":""}`, "REASON_REQUIRED", http.StatusUnprocessableEntity},
		{`{"rule_ids":["RC-ACC-001","RECON-DEP-001"],"reason":"x"}`, "NOT_DISABLED", http.StatusConflict},
		{`{"rule_ids":["RC-ACC-001"],"reason":"x","by":"someone else"}`, "BAD_REQUEST", http.StatusBadRequest},
	} {
		if status, code := codeAndStatus(t, e.client, "POST", base+"/enable", c.body); status != c.status || code != c.code {
			t.Errorf("%s: %d %s, want %d %s", c.body, status, code, c.status, c.code)
		}
	}
	tooMany := `{"rule_ids":["RC-ACC-001"` + strings.Repeat(`,"RC-ACC-001"`, 500) + `],"reason":"x"}`
	if status, code := codeAndStatus(t, e.client, "POST", base+"/enable", tooMany); status != http.StatusBadRequest || code != "TOO_MANY_RULES" {
		t.Errorf("501 rules: %d %s", status, code)
	}

	if status, out := postJSON(t, e.client, base+"/enable", map[string]any{
		"rule_ids": []string{"RC-ACC-001", "RC-ACC-002", "RC-ACC-003"}, "reason": "listed as in use"}); status != http.StatusOK || strings.Count(out, `"status":"active"`) != 3 {
		t.Fatalf("bulk enable: %d %s", status, out)
	}
	if status, out := postJSON(t, e.client, base+"/disable", map[string]any{
		"rule_ids": []string{"RC-ACC-001", "RC-ACC-002"}, "reason": "not used"}); status != http.StatusOK || strings.Count(out, `"status":"disabled"`) != 2 {
		t.Fatalf("bulk disable: %d %s", status, out)
	}
	_, list := e.get(t, "/api/v1/rules")
	for id, want := range map[string]string{"RC-ACC-001": "disabled", "RC-ACC-002": "disabled", "RC-ACC-003": "active"} {
		i := strings.Index(list, `"id":"`+id+`"`)
		if i < 0 {
			t.Fatalf("%s is not listed", id)
		}
		end := strings.Index(list[i:], "}")
		if !strings.Contains(list[i:i+end], `"status":"`+want+`"`) {
			t.Errorf("%s: %s, want %s", id, list[i:i+end], want)
		}
	}
}
