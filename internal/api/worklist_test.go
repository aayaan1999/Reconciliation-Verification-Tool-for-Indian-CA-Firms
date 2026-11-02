package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SPEC-006 part 3, AC-13 to AC-15: Test Pack A's two ledger CSVs uploaded through the
// server give the Worklist; a classification is kept and survives a restart. The pack is
// fictional (Nimbus Logic Systems; PAN invented).

func packLedger(t *testing.T, name string) upload {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repo, "testdata", "testpack_nimbus", name))
	if err != nil {
		t.Fatal(err)
	}
	return upload{name, b}
}

func (e env) postClassify(t *testing.T, body map[string]string) (int, map[string]json.RawMessage) {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := e.client.Post(e.srv.URL+"/api/v1/clients/1/worklist", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]json.RawMessage
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestWorklistThroughServer(t *testing.T) {
	e := newEnv(t)
	if status, _ := e.addClient(t, "Nimbus Logic Systems", "AAACN4321K"); status != http.StatusCreated {
		t.Fatalf("add client: %d", status)
	}
	if status, out := e.postFor(t, "1", "2025-26", false,
		packLedger(t, "07_expense_ledger_FY2024-25.csv"), packLedger(t, "08_expense_ledger_FY2025-26.csv")); status != http.StatusOK {
		t.Fatalf("run: %d %s", status, out["message"])
	}
	status, got := e.get(t, "/api/v1/worklist?tax_year=2025-26")
	var wl struct {
		Items []struct {
			Key            string          `json:"key"`
			ClientName     string          `json:"client_name"`
			Classification json.RawMessage `json:"classification"`
		} `json:"items"`
	}
	json.Unmarshal([]byte(got), &wl)
	if status != http.StatusOK || len(wl.Items) != 12 || wl.Items[0].Key != "new_head|Cloud hosting - overseas" ||
		wl.Items[0].ClientName != "Nimbus Logic Systems" || string(wl.Items[0].Classification) != "null" {
		t.Fatalf("worklist: %d %.400s", status, got)
	}
	if status, _ := e.get(t, "/api/v1/worklist?tax_year=2025"); status != http.StatusBadRequest {
		t.Errorf("bad tax year: %d", status)
	}

	if status, out := e.postClassify(t, map[string]string{"tax_year": "2025-26", "kind": "new_head", "ledger": "Rent", "classification": "x", "by": "A. Staff"}); status != http.StatusNotFound || code(out) != "WORKLIST_ITEM_NOT_FOUND" {
		t.Errorf("unknown item: %d %s", status, code(out))
	}
	if status, out := e.postClassify(t, map[string]string{"tax_year": "2025-26", "kind": "vanished", "ledger": "Courier and postage", "classification": "", "by": "A. Staff"}); status != http.StatusUnprocessableEntity || code(out) != "BAD_CLASSIFICATION" {
		t.Errorf("empty classification: %d %s", status, code(out))
	}
	status, out := e.postClassify(t, map[string]string{"tax_year": "2025-26", "kind": "vanished", "ledger": "Courier and postage",
		"classification": "Stopped: documents now sent by email (client confirmed)", "by": "A. Staff"})
	if status != http.StatusOK || !strings.Contains(string(out["items"]), `"by":"priya"`) {
		t.Fatalf("classify: %d %.300s", status, out["items"])
	}

	// After a restart the classification is still there.
	again := newEnvWithDB(t, e.db)
	_, after := again.get(t, "/api/v1/worklist?tax_year=2025-26")
	if !strings.Contains(after, "Stopped: documents now sent by email") || strings.Count(after, `"classification":null`) != 11 {
		t.Errorf("after a restart: %.400s", after)
	}
	e.assertNoTempFiles(t)
}
