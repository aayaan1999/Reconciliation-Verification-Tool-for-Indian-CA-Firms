package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// SPEC-006 part 3, AC-11: a decision taken through the server is kept, and survives a
// restart. The PAN and figures are those of the synthetic tax-suite sample.

func (e env) postDecision(t *testing.T, body map[string]string) (int, map[string]json.RawMessage) {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := e.client.Post(e.srv.URL+"/api/v1/clients/1/decisions", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]json.RawMessage
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestDecisionsThroughServer(t *testing.T) {
	e := newEnv(t)
	if status, _ := e.addClient(t, "Sample Firm", "AAKFS5678M"); status != http.StatusCreated {
		t.Fatalf("add client: %d", status)
	}
	if status, out := e.postFor(t, "1", "2025-26", true, sample(t, "taxsuite", "comp_v3_2025-26_differs.json")); status != http.StatusOK {
		t.Fatalf("run: %d %s", status, out["message"])
	}
	_, got := e.get(t, "/api/v1/clients/1/result?tax_year=2025-26")
	var res struct {
		Results []struct {
			Report struct {
				Findings []struct {
					Findings []struct {
						FindingID string `json:"finding_id"`
					} `json:"findings"`
				} `json:"findings"`
			} `json:"report"`
		} `json:"results"`
	}
	json.Unmarshal([]byte(got), &res)
	if len(res.Results) == 0 || len(res.Results[0].Report.Findings) == 0 {
		t.Fatalf("no findings: %.300s", got)
	}
	id := res.Results[0].Report.Findings[0].Findings[0].FindingID

	if status, out := e.postDecision(t, map[string]string{"tax_year": "2025-26", "finding_id": id, "decision": "not_an_issue", "by": "A. Staff"}); status != http.StatusUnprocessableEntity || code(out) != "NOTE_REQUIRED" {
		t.Errorf("not an issue without a note: %d %s", status, code(out))
	}
	if status, out := e.postDecision(t, map[string]string{"tax_year": "2025-26", "finding_id": "AAAAAAAAAAAAAAAA", "decision": "fix", "by": "A. Staff"}); status != http.StatusNotFound || code(out) != "FINDING_NOT_FOUND" {
		t.Errorf("unknown finding: %d %s", status, code(out))
	}
	status, out := e.postDecision(t, map[string]string{"tax_year": "2025-26", "finding_id": id, "decision": "not_an_issue", "note": "Rounding in the schedule; agreed with the partner", "by": "A. Staff"})
	if status != http.StatusOK || !strings.Contains(string(out["decision"]), `"state":"not_an_issue"`) {
		t.Fatalf("decide: %d %s", status, out["decision"])
	}

	// After a restart the decision is still there.
	again := newEnvWithDB(t, e.db)
	_, after := again.get(t, "/api/v1/clients/1/result?tax_year=2025-26")
	if !strings.Contains(after, `"state":"not_an_issue"`) || !strings.Contains(after, "Rounding in the schedule") || !strings.Contains(after, `"decided_by":"priya"`) {
		t.Errorf("after a restart: %.400s", after)
	}
	e.assertNoTempFiles(t)
}
