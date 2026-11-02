package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The client tests of SPEC-004 part 7 (AC-21 to AC-23). All names and PANs are invented.

func (e env) addClient(t *testing.T, name, pan string) (int, map[string]json.RawMessage) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"name": name, "pan": pan, "tax_year": "2025-26"})
	resp, err := e.client.Post(e.srv.URL+"/api/v1/clients", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]json.RawMessage
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (e env) get(t *testing.T, path string) (int, string) {
	t.Helper()
	resp, err := e.client.Get(e.srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func code(out map[string]json.RawMessage) string {
	var c string
	json.Unmarshal(out["error_code"], &c)
	return c
}

// AC-21: a client is added, refused twice, listed, and still there after a restart; the full
// PAN is in no response and not in the database file.
func TestAddClient(t *testing.T) {
	e := newEnv(t)
	status, out := e.addClient(t, "Sample Engineering Works", "aakfs5678m")
	if status != http.StatusCreated || !strings.Contains(string(out["client"]), `"pan_last4":"678M"`) {
		t.Fatalf("add: %d %s", status, out)
	}
	if status, out := e.addClient(t, "Again", "AAKFS5678M"); status != http.StatusConflict || code(out) != "CLIENT_EXISTS" {
		t.Errorf("same PAN twice: %d %s", status, code(out))
	}
	if status, out := e.addClient(t, "Bad", "12345"); status != http.StatusUnprocessableEntity || code(out) != "BAD_PAN" {
		t.Errorf("bad PAN: %d %s", status, code(out))
	}
	if status, out := e.addClient(t, "", "AAACI1234Q"); status != http.StatusUnprocessableEntity || code(out) != "BAD_CLIENT_NAME" {
		t.Errorf("no name: %d %s", status, code(out))
	}
	status, list := e.get(t, "/api/v1/clients?tax_year=2025-26")
	if status != http.StatusOK || !strings.Contains(list, `"name":"Sample Engineering Works"`) || strings.Count(list, `"id":`) != 1 {
		t.Errorf("list: %d %s", status, list)
	}

	restarted := newEnvWithDB(t, e.db)
	if _, list := restarted.get(t, "/api/v1/clients?tax_year=2025-26"); !strings.Contains(list, "Sample Engineering Works") {
		t.Errorf("the client is gone after a restart: %s", list)
	}
	for _, f := range []string{e.db, e.db + "-wal"} {
		raw, _ := os.ReadFile(f)
		if bytes.Contains(bytes.ToUpper(raw), []byte("AAKFS5678M")) {
			t.Errorf("the full PAN is in %s", filepath.Base(f))
		}
	}
	for _, s := range []string{list, string(out["client"])} {
		if strings.Contains(strings.ToUpper(s), "AAKFS5678M") {
			t.Errorf("a response shows the full PAN: %s", s)
		}
	}
}

// AC-22 and AC-23: the client's accounts and our computation, uploaded separately, are
// compared; a file with another PAN is refused and nothing of it is kept.
func TestCompareClientReports(t *testing.T) {
	e := newEnv(t)
	if status, out := e.addClient(t, "Sample Engineering Works", "AAKFS5678M"); status != http.StatusCreated {
		t.Fatalf("add: %d %s", status, out)
	}
	accounts := sample(t, "compare-differs", "fs_2025-26_differs.json")
	ours := sample(t, "compare-differs", "comp_v3_2025-26_differs.json")

	status, out := e.postFor(t, "1", "2025-26", true, accounts)
	if status != http.StatusOK || strings.Contains(string(out["report"]), `"difference_paise":2700000`) {
		t.Fatalf("accounts alone: %d %s %s", status, out["message"], out["report"])
	}
	status, out = e.postFor(t, "1", "2025-26", true, ours)
	report := string(out["report"])
	if status != http.StatusOK || !strings.Contains(report, `"difference_paise":2700000`) ||
		!strings.Contains(report, `"file":"fs_2025-26_differs.json"`) || !strings.Contains(report, `"file":"comp_v3_2025-26_differs.json"`) {
		t.Fatalf("both reports: %d %s", status, report)
	}
	for _, s := range []string{`"path":"$.profit_and_loss.profit_before_tax"`, `"path":"$.income.business_profession.net_profit_as_per_pl"`, `"paise":126354000`, `"paise":123654000`} {
		if !strings.Contains(report, s) {
			t.Errorf("report lacks %s", s)
		}
	}

	status, out = e.postFor(t, "1", "2025-26", true, sample(t, "compare-clean", "comp_v3_2025-26_agrees.json"))
	if status != http.StatusUnprocessableEntity || code(out) != "CONFLICTING_VALUES" || !strings.Contains(string(out["message"]), "comp_v3_2025-26_agrees.json") {
		t.Errorf("another client's file: %d %s %s", status, code(out), out["message"])
	}
	status, result := e.get(t, "/api/v1/clients/1/result?tax_year=2025-26")
	if status != http.StatusOK || !strings.Contains(result, `"difference_paise":2700000`) || !strings.Contains(result, `"documents":2`) {
		t.Errorf("result after the refused file: %d %s", status, result)
	}
	if status, _ := e.get(t, "/api/v1/clients/99/result?tax_year=2025-26"); status != http.StatusNotFound {
		t.Errorf("unknown client: %d", status)
	}
	if status, out := e.postFor(t, "99", "2025-26", true, ours); status != http.StatusNotFound || code(out) != "CLIENT_NOT_FOUND" {
		t.Errorf("upload for an unknown client: %d %s", status, code(out))
	}
	e.assertNoTempFiles(t)
}

// SPEC-004 AC-25 and AC-26: a client with all its details, edited later; bad values refused
// with their code; a GST upload by comparison type runs only that type's rule.
func TestClientMasterAndComparison(t *testing.T) {
	e := newEnv(t)
	details := map[string]any{
		"legal_name": "Deccan Byte Systems Private Limited", "trade_name": "Deccan Byte", "pan": "ZLVCD9565Z",
		"constitution": "company", "date_of_incorporation": "2014-03-12", "cin": "U72200MH2014PTC123456", "tan": "PNED12345B",
		"gstins":         []map[string]any{{"gstin": "27ZLVCD9565Z1ZF", "registration_type": "regular", "primary": true}},
		"address":        map[string]any{"line1": "Plot 4", "line2": "", "city": "Pune", "state": "27", "pin": "411057"},
		"contact_person": "A. Accountant", "email": "accounts@example.com", "phone": "+91 20 1234 5678",
		"accounting_software": "Tally Prime", "notes": "",
	}
	send := func(method, path string, body any) (int, map[string]json.RawMessage) {
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, e.srv.URL+path, bytes.NewReader(b))
		req.Header.Set("content-type", "application/json")
		resp, err := e.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]json.RawMessage
		json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out
	}
	if status, out := send("POST", "/api/v1/clients", map[string]any{"tax_year": "2026-27", "details": details}); status != http.StatusCreated {
		t.Fatalf("add: %d %s", status, out)
	}
	status, got := e.get(t, "/api/v1/clients/1")
	if status != http.StatusOK || !strings.Contains(got, `"tan":"PNED12345B"`) || !strings.Contains(got, `"gstin":"27ZLVCD9565Z1ZF"`) || strings.Contains(got, "ZLVCD9565Z\"") {
		t.Errorf("details: %d %s", status, got)
	}

	bad := map[string]any{}
	for k, v := range details {
		bad[k] = v
	}
	bad["gstins"] = []map[string]any{{"gstin": "27ZLVCD9565Z1ZG", "registration_type": "regular", "primary": true}}
	if status, out := send("PUT", "/api/v1/clients/1", map[string]any{"details": bad}); status != http.StatusUnprocessableEntity || code(out) != "GSTIN_CHECK_DIGIT" {
		t.Errorf("wrong check digit: %d %s", status, code(out))
	}
	edited := map[string]any{}
	for k, v := range details {
		edited[k] = v
	}
	edited["pan"], edited["trade_name"] = "", "DBS"
	if status, out := send("PUT", "/api/v1/clients/1", map[string]any{"details": edited}); status != http.StatusOK {
		t.Fatalf("edit: %d %s", status, out)
	}
	if _, got := e.get(t, "/api/v1/clients/1"); !strings.Contains(got, `"trade_name":"DBS"`) || !strings.Contains(got, `"pan_last4":"565Z"`) {
		t.Errorf("after edit: %s", got)
	}

	if status, list := e.get(t, "/api/v1/comparisons"); status != http.StatusOK || !strings.Contains(list, `"id":"gst_sales_vs_gstr1"`) {
		t.Errorf("comparisons: %d %s", status, list)
	}
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	mw.WriteField("client_id", "1")
	mw.WriteField("tax_year", "2026-27")
	mw.WriteField("include_testing", "true")
	mw.WriteField("comparison", "gst_sales_vs_gstr1")
	for _, f := range []upload{sample(t, "gst-sales", "SALES_REG_Aug2026.json"), sample(t, "gst-sales", "GSTR1_Aug2026.json")} {
		w, _ := mw.CreateFormFile("files", f.name)
		w.Write(f.data)
	}
	mw.Close()
	resp, err := e.client.Post(e.srv.URL+"/api/v1/runs", mw.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	report := string(b)
	if resp.StatusCode != http.StatusOK || !strings.Contains(report, `"id":"gst_sales_vs_gstr1"`) ||
		!strings.Contains(report, `"code":"ONLY_IN_A"`) || strings.Contains(report, `"rule_id":"RECON-DEP-001"`) {
		t.Errorf("GST run: %d %.400s", resp.StatusCode, report)
	}
	e.assertNoTempFiles(t)
}
