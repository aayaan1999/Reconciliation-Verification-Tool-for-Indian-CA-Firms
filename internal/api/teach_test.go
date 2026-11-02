package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// teach posts a file (and, for /profiles, a mapping) as the mapping screen does.
func (e env) teach(t *testing.T, path string, file upload, fields map[string]string) (int, map[string]json.RawMessage) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	fw, _ := mw.CreateFormFile("file", file.name)
	fw.Write(file.data)
	mw.Close()
	resp, err := e.client.Post(e.srv.URL+path, mw.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]json.RawMessage
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// AC-29 through the server: a file no profile knows is refused with PROFILE_NO_MATCH,
// inspected, previewed, taught, and then read by the next run.
func TestTeachLayoutThroughServer(t *testing.T) {
	e := newEnv(t)
	dir := filepath.Join(repo, "testdata", "teach")
	data, err := os.ReadFile(filepath.Join(dir, "office_sales_Aug2026.json"))
	if err != nil {
		t.Fatal(err)
	}
	mapping, err := os.ReadFile(filepath.Join(dir, "office_sales.mapping.json"))
	if err != nil {
		t.Fatal(err)
	}
	file := upload{"office_sales_Aug2026.json", data}

	if status, out := e.post(t, "2026-27", false, file); status != http.StatusUnprocessableEntity || code(out) != "PROFILE_NO_MATCH" {
		t.Fatalf("before teaching: %d %s", status, code(out))
	}
	status, out := e.teach(t, "/api/v1/inspect", file, nil)
	if status != http.StatusOK || !strings.Contains(string(out["fields"]), `"$.Vouchers[*]['Bill No']"`) ||
		!strings.Contains(string(out["document_types"]), `"sales_register"`) {
		t.Fatalf("inspect: %d %s", status, out)
	}

	status, out = e.teach(t, "/api/v1/profiles", file, map[string]string{"mapping": string(mapping), "check_only": "true"})
	if status != http.StatusOK || string(out["saved"]) != "false" || string(out["rows"]) != "4" {
		t.Fatalf("preview: %d %s", status, out)
	}
	bad := strings.Replace(string(mapping), `"name": "invoice_no"`, `"name": "place_of_supply"`, 1)
	status, out = e.teach(t, "/api/v1/profiles", file, map[string]string{"mapping": bad})
	if status != http.StatusUnprocessableEntity || code(out) != "INVALID_MAPPING" || !strings.Contains(string(out["problems"]), "MISSING_KEY_COLUMN") {
		t.Fatalf("bad mapping: %d %s", status, out)
	}
	if status, out = e.teach(t, "/api/v1/profiles", file, map[string]string{"mapping": string(mapping)}); status != http.StatusOK || string(out["saved"]) != "true" {
		t.Fatalf("teach: %d %s", status, out)
	}
	if status, out = e.teach(t, "/api/v1/profiles", file, map[string]string{"mapping": string(mapping)}); status != http.StatusConflict {
		t.Errorf("taught twice: %d %s", status, out)
	}

	status, out = e.post(t, "2026-27", false, file)
	if status != http.StatusOK || !strings.Contains(string(out["report"]), "office.office_sales_export.sales_register") {
		t.Errorf("after teaching: %d %s", status, out)
	}
	e.assertNoTempFiles(t)
}
