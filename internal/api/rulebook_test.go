package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// SPEC-010 scope 8 through the server: a rule-book ID opens, the whole rule-book rule comes
// back, and only a Partner edits, disables or enables it (AC-21, AC-25).
func TestRulebookRuleThroughServer(t *testing.T) {
	e := newEnv(t)
	const path = "/api/v1/rules/RC-ACC-003"
	status, out := e.sendJSON(t, http.MethodGet, path, nil)
	if status != http.StatusOK {
		t.Fatalf("GET %s: %d %s", path, status, out["message"])
	}
	var rb map[string]json.RawMessage
	if err := json.Unmarshal(out["rulebook"], &rb); err != nil || len(rb["rule"]) == 0 || len(rb["legal"]) == 0 || len(rb["outcomes"]) == 0 {
		t.Fatalf("the rule-book rule is not in the reply: %s (%v)", out["rulebook"], err)
	}
	if s, _ := e.sendJSON(t, http.MethodGet, "/api/v1/rules/rc-acc-003", nil); s != http.StatusBadRequest {
		t.Errorf("a lower-case ID: %d, want 400", s)
	}

	edit := map[string]any{"reason": "wording agreed with the CA", "rule": json.RawMessage(strings.Replace(string(out["rulebook"]),
		`"notes":"`, `"notes":"Edited by the Partner. `, 1))}
	body, err := json.Marshal(edit)
	if err != nil {
		t.Fatal(err)
	}
	staff, manager, _ := users(t, e)
	for _, c := range []struct {
		method, path, body, perm string
	}{
		{"PUT", path + "/rulebook", string(body), "activate_rules"},
		{"POST", path + "/enable", `{"reason":"listed as in use"}`, "activate_rules"},
		{"POST", path + "/disable", `{"reason":"not used"}`, "disable_rules"},
	} {
		for _, who := range []struct {
			name string
			c    *http.Client
		}{{"staff", staff}, {"manager", manager}} {
			s, b := do(t, who.c, e.srv.URL, c.method, c.path, c.body)
			perm, code := refusedBy(b)
			if s != http.StatusForbidden || code != "FORBIDDEN" || perm != c.perm {
				t.Errorf("%s %s as %s: %d %s (permission %q), want 403 with %s", c.method, c.path, who.name, s, code, perm, c.perm)
			}
		}
	}
	// The Partner's edit, enable and disable succeed.
	if s, b := do(t, e.client, e.srv.URL, "PUT", path+"/rulebook", string(body)); s != http.StatusOK {
		t.Fatalf("the Partner's edit: %d %s", s, b)
	}
	if _, after := e.sendJSON(t, http.MethodGet, path, nil); !strings.Contains(string(after["rulebook"]), "Edited by the Partner.") || !strings.Contains(string(after["source"]), "office") {
		t.Errorf("after the edit: source %s", after["source"])
	}
	if s, b := do(t, e.client, e.srv.URL, "POST", path+"/enable", `{"reason":"listed as in use"}`); s != http.StatusOK || !strings.Contains(b, "active") {
		t.Errorf("the Partner's enable: %d %s", s, b)
	}
	if s, b := do(t, e.client, e.srv.URL, "POST", path+"/disable", `{"reason":"not used"}`); s != http.StatusOK {
		t.Errorf("the Partner's disable: %d %s", s, b)
	}
	if s, b := do(t, e.client, e.srv.URL, "POST", path+"/reset", `{}`); s != http.StatusOK {
		t.Errorf("the Partner's reset to the rule-book version: %d %s", s, b)
	}
	if _, after := e.sendJSON(t, http.MethodGet, path, nil); !strings.Contains(string(after["source"]), "rulebook") {
		t.Errorf("after the reset: source %s", after["source"])
	}
}
