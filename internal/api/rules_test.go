package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
)

// SPEC-010 through the server: the Rule Builder's lists, a rule read for editing, a save that
// is refused with each problem named, a save that works, disabling and enabling with a reason,
// and versions. The server passes each change to recon-engine, which does the checks and writes.

// sendJSON makes a request with a JSON body (or none) and returns the status and the answer.
func (e env) sendJSON(t *testing.T, method, path string, body any) (int, map[string]json.RawMessage) {
	t.Helper()
	var rd *bytes.Reader
	if body == nil {
		rd = bytes.NewReader(nil)
	} else {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.srv.URL+path, rd)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]json.RawMessage
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// testRuleForm is a valid new rule: two computation figures that must agree within Rs 1.00.
func testRuleForm() rules.Form {
	return rules.Form{
		ID: "RECON-OFF-101", Title: "Depreciation check through the server", RuleText: "Compares two depreciation figures.",
		Module: "tieout", Category: "R1", TaxYears: []string{"2025-26"}, ValidFrom: "2025-04-01",
		Inputs: []rules.FormInput{
			{Name: "a", Kind: "fact", Ref: "computation.allowed_depreciation", Label: "Computation"},
			{Name: "b", Kind: "fact", Ref: "computation.fixed_asset_schedule.total", Label: "Schedule"},
		},
		Check:          rules.FormCheck{Type: "equal", Tolerance: "1.00"},
		Severity:       "high",
		ReviewDocument: "computation",
		Law:            []rules.FormLaw{{Act: "Income-tax Act, 1961", Section: "32"}},
		Fixtures: []rules.FormFixture{
			{Kind: "pass", Name: "agree", Description: "They agree",
				Facts: []rules.FormFact{
					{Fact: "computation.allowed_depreciation", Value: "112450.50", Period: "FY2025-26", File: "c.json", Locator: "$.a"},
					{Fact: "computation.fixed_asset_schedule.total", Value: "112450.50", Period: "FY2025-26", File: "c.json", Locator: "$.b"},
				}},
			{Kind: "fail", Name: "differs", Description: "They differ",
				Facts: []rules.FormFact{
					{Fact: "computation.allowed_depreciation", Value: "112450.50", Period: "FY2025-26", File: "c.json", Locator: "$.a"},
					{Fact: "computation.fixed_asset_schedule.total", Value: "115200.50", Period: "FY2025-26", File: "c.json", Locator: "$.b"},
				}},
		},
	}
}

// The lists, a rule read for editing, a refused save with its problems, a saved rule, and an
// edit that makes an office version of a built-in rule.
func TestRuleSaveThroughServer(t *testing.T) {
	e := newEnv(t)
	status, opts := e.sendJSON(t, http.MethodGet, "/api/v1/rule-options", nil)
	if status != http.StatusOK || !strings.Contains(string(opts["modules"]), `"tieout"`) || !strings.Contains(string(opts["check_types"]), `"equal"`) {
		t.Fatalf("rule-options: %d %s", status, opts["message"])
	}

	status, cur := e.sendJSON(t, http.MethodGet, "/api/v1/rules/RECON-DEP-001", nil)
	if status != http.StatusOK || string(cur["editable"]) != "true" {
		t.Fatalf("read a built-in rule: %d %s", status, cur["message"])
	}

	bad := testRuleForm()
	bad.Inputs[0].Ref = "computation.made_up"
	status, out := e.sendJSON(t, http.MethodPost, "/api/v1/rules", map[string]any{"by": "A. Staff", "rule": bad})
	if status != http.StatusUnprocessableEntity || code(out) != "RULE_REFUSED" || !strings.Contains(string(out["problems"]), `"field":"inputs[0].ref"`) {
		t.Errorf("a refused save: %d %s %s", status, code(out), out["problems"])
	}

	status, out = e.sendJSON(t, http.MethodPost, "/api/v1/rules", map[string]any{"by": "A. Staff", "rule": testRuleForm()})
	if status != http.StatusCreated || string(out["status"]) != `"testing"` || string(out["source"]) != `"office"` {
		t.Fatalf("a saved rule: %d %s %s", status, code(out), out["message"])
	}

	// Editing the built-in rule makes "1-office.1"; the window sends the form it read.
	status, out = e.sendJSON(t, http.MethodPut, "/api/v1/rules/RECON-DEP-001", map[string]any{"by": "A. Staff", "rule": editOf(t, e)})
	if status != http.StatusOK || !strings.Contains(string(out["version"]), "1-office.1") {
		t.Errorf("an edit of a built-in rule: %d %s %s", status, code(out), out["message"])
	}
}

// editOf reads the built-in rule as the window does and returns it as an edit's form.
func editOf(t *testing.T, e env) rules.Form {
	t.Helper()
	status, cur := e.sendJSON(t, http.MethodGet, "/api/v1/rules/RECON-DEP-001", nil)
	if status != http.StatusOK {
		t.Fatalf("read: %d", status)
	}
	var rc struct {
		Form rules.Form `json:"form"`
	}
	b, _ := json.Marshal(cur)
	if err := json.Unmarshal(b, &rc); err != nil {
		t.Fatal(err)
	}
	f := rc.Form
	f.Mode, f.Fixtures = "edit", nil
	return f
}

// Disabling and enabling need a reason; an unknown rule is 404; versions are listed and switched.
func TestRuleDisableThroughServer(t *testing.T) {
	e := newEnv(t)
	if status, out := e.sendJSON(t, http.MethodPost, "/api/v1/rules", map[string]any{"by": "A. Staff", "rule": testRuleForm()}); status != http.StatusCreated {
		t.Fatalf("create: %d %s", status, out["message"])
	}
	if status, out := e.sendJSON(t, http.MethodPost, "/api/v1/rules/RECON-OFF-101/disable", map[string]string{"by": "A. Staff", "reason": ""}); status != http.StatusUnprocessableEntity || code(out) != "REASON_REQUIRED" {
		t.Errorf("disable without a reason: %d %s", status, code(out))
	}
	if status, out := e.sendJSON(t, http.MethodPost, "/api/v1/rules/RECON-OFF-101/disable", map[string]string{"by": "A. Staff", "reason": "fires on test data"}); status != http.StatusOK || !strings.Contains(string(out["status"]), "disabled") {
		t.Errorf("disable: %d %s %s", status, code(out), out["message"])
	}
	if status, out := e.sendJSON(t, http.MethodPost, "/api/v1/rules/RECON-OFF-101/enable", map[string]string{"by": "A. Staff", "reason": "checked"}); status != http.StatusOK || !strings.Contains(string(out["status"]), "testing") {
		t.Errorf("enable: %d %s %s", status, code(out), out["message"])
	}
	if status, out := e.sendJSON(t, http.MethodPost, "/api/v1/rules/RECON-NOPE-999/disable", map[string]string{"by": "A. Staff", "reason": "x"}); status != http.StatusNotFound || code(out) != "RULE_NOT_FOUND" {
		t.Errorf("an unknown rule: %d %s", status, code(out))
	}
	if status, out := e.sendJSON(t, http.MethodPost, "/api/v1/rules/RECON-OFF-404/disable", map[string]string{"by": "A. Staff", "reason": "x"}); status != http.StatusNotFound || code(out) != "RULE_NOT_FOUND" {
		t.Errorf("a rule that does not exist: %d %s", status, code(out))
	}
	if status, out := e.sendJSON(t, http.MethodGet, "/api/v1/rules/RECON-OFF-101/versions", nil); status != http.StatusOK || !strings.Contains(string(out["versions"]), `"version":"1"`) {
		t.Errorf("versions: %d %s", status, out["message"])
	}
	if status, out := e.sendJSON(t, http.MethodPost, "/api/v1/rules/RECON-OFF-101/reset", map[string]string{"by": "A. Staff"}); status != http.StatusUnprocessableEntity || code(out) != "NO_STANDARD_RULE" {
		t.Errorf("reset of a rule with no standard version: %d %s", status, code(out))
	}
	if status, _ := e.sendJSON(t, http.MethodGet, "/api/v1/rules/not-an-id", nil); status != http.StatusBadRequest {
		t.Errorf("a bad rule ID: %d", status)
	}
}

// A saved office rule is listed with the rules of the database.
func TestOfficeRuleListedThroughServer(t *testing.T) {
	e := newEnv(t)
	if status, out := e.sendJSON(t, http.MethodPost, "/api/v1/rules", map[string]any{"by": "A. Staff", "rule": testRuleForm()}); status != http.StatusCreated {
		t.Fatalf("create: %d %s", status, out["message"])
	}
	status, body := e.get(t, "/api/v1/rules")
	if status != http.StatusOK || !strings.Contains(body, "RECON-OFF-101") {
		t.Errorf("the rules list does not show the office rule: %d", status)
	}
}

// AC-18 through the server: a testing rule that has not been run on a client is refused with the
// reason, and the list says why it may not be approved yet. A reason is required, and an unknown
// rule is 404. The approval after a client run is proven by the engine's tests.
func TestRuleApproveThroughServer(t *testing.T) {
	e := newEnv(t)
	if status, out := e.sendJSON(t, http.MethodPost, "/api/v1/rules", map[string]any{"by": "A. Staff", "rule": testRuleForm()}); status != http.StatusCreated {
		t.Fatalf("create: %d %s", status, out["message"])
	}
	if status, out := e.sendJSON(t, http.MethodPost, "/api/v1/rules/RECON-OFF-101/approve", map[string]string{"by": "A. Partner", "reason": ""}); status != http.StatusUnprocessableEntity || code(out) != "REASON_REQUIRED" {
		t.Errorf("approve without a reason: %d %s", status, code(out))
	}
	if status, out := e.sendJSON(t, http.MethodPost, "/api/v1/rules/RECON-OFF-101/approve", map[string]string{"by": "A. Partner", "reason": "checked"}); status != http.StatusUnprocessableEntity || code(out) != "NOT_RUN_ON_CLIENT" || !strings.Contains(string(out["message"]), "not been run on a client") {
		t.Errorf("approve before a client run: %d %s %s", status, code(out), out["message"])
	}
	if status, body := e.get(t, "/api/v1/rules"); status != http.StatusOK || !strings.Contains(body, "not been run on a client") {
		t.Errorf("the list does not say why the rule cannot be approved yet: %d", status)
	}
	if status, out := e.sendJSON(t, http.MethodPost, "/api/v1/rules/RECON-NOPE-999/approve", map[string]string{"by": "A. Partner", "reason": "checked"}); status != http.StatusNotFound || code(out) != "RULE_NOT_FOUND" {
		t.Errorf("approve an unknown rule: %d %s", status, code(out))
	}
}
