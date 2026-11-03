package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// SPEC-016: the Formats endpoints, for the Partner. The standard formats and parameters are
// stored when the server starts; changes go through the engine and are refused for the standard
// ones. The role walk in auth_test.go proves every other role is refused.

type formatsReply struct {
	Formats []struct {
		ID     string `json:"id"`
		Source string `json:"source"`
		Status string `json:"status"`
	} `json:"formats"`
	Parameters []struct {
		Key     string   `json:"key"`
		Source  string   `json:"source"`
		Status  string   `json:"status"`
		Aliases []string `json:"aliases"`
	} `json:"parameters"`
	Unsupplied   []string   `json:"unsupplied"`
	Unrecognised []struct{} `json:"unrecognised"`
}

func getFormats(t *testing.T, e env) formatsReply {
	t.Helper()
	resp, err := e.client.Get(e.srv.URL + "/api/v1/formats")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET formats: %d %s", resp.StatusCode, raw)
	}
	var r formatsReply
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestFormatsEndpoints(t *testing.T) {
	e := newEnv(t)
	l := getFormats(t, e)
	if len(l.Formats) == 0 || len(l.Parameters) == 0 {
		t.Fatalf("%d formats, %d parameters", len(l.Formats), len(l.Parameters))
	}
	std := l.Formats[0]
	if std.Source != "builtin" || std.Status != "active" {
		t.Errorf("first format = %+v", std)
	}

	// A standard format is read only.
	status, out := postJSON(t, e.client, e.srv.URL+"/api/v1/formats/"+std.ID+"/disable", map[string]string{"reason": "x"})
	if status != http.StatusConflict || !strings.Contains(out, "FORMAT_READ_ONLY") {
		t.Errorf("disable a standard format: %d %s", status, out)
	}
	if status, out := postJSON(t, e.client, e.srv.URL+"/api/v1/formats/no-such/disable", map[string]string{"reason": "x"}); status != http.StatusNotFound || !strings.Contains(out, "FORMAT_NOT_FOUND") {
		t.Errorf("disable an unknown format: %d %s", status, out)
	}
	if status, out := postJSON(t, e.client, e.srv.URL+"/api/v1/formats/"+std.ID+"/versions/0/switch", map[string]string{"reason": "x"}); status != http.StatusBadRequest {
		t.Errorf("version 0: %d %s", status, out)
	}

	// Bad parameters and standard ones are refused.
	status, out = postJSON(t, e.client, e.srv.URL+"/api/v1/parameters", map[string]any{
		"key": "office.advance_received", "name": "Advance received", "kind": "money", "document": "no_such_document", "description": "", "aliases": []string{}})
	if status != http.StatusUnprocessableEntity || !strings.Contains(out, "BAD_DOCUMENT") {
		t.Errorf("unknown document: %d %s", status, out)
	}
	status, out = postJSON(t, e.client, e.srv.URL+"/api/v1/parameters", map[string]any{"key": "Bad Key"})
	if status != http.StatusUnprocessableEntity || !strings.Contains(out, "BAD_KEY") {
		t.Errorf("bad key: %d %s", status, out)
	}
	if status, out := postJSON(t, e.client, e.srv.URL+"/api/v1/parameters/"+l.Parameters[0].Key+"/disable", map[string]string{"reason": "x"}); status != http.StatusConflict || !strings.Contains(out, "PARAMETER_READ_ONLY") {
		t.Errorf("disable a standard parameter: %d %s", status, out)
	}
	if status, out := postJSON(t, e.client, e.srv.URL+"/api/v1/parameters/Bad%20Key/disable", map[string]string{"reason": "x"}); status != http.StatusBadRequest {
		t.Errorf("bad key in path: %d %s", status, out)
	}
}

// Anyone but the Partner is refused, whatever they send.
func TestFormatsEndpointsAreForThePartner(t *testing.T) {
	e := newEnv(t)
	for _, role := range []string{"manager", "staff", "read_only"} {
		c := userClient(t, e, "u"+role, role)
		if status, code := codeAndStatus(t, c, http.MethodGet, e.srv.URL+"/api/v1/formats", ""); status != http.StatusForbidden || code != "FORBIDDEN" {
			t.Errorf("%s: GET formats = %d %s", role, status, code)
		}
	}
}

// SPEC-016: editing a format through the API. Anyone who may run checks can teach a layout, but
// saving an edit of a format is the Partner's, and a standard format cannot be edited or published.
func TestEditAndPublishEndpoints(t *testing.T) {
	e := newEnv(t)
	std := getFormats(t, e).Formats[0].ID
	if status, out := postJSON(t, e.client, e.srv.URL+"/api/v1/formats/"+std+"/publish", map[string]any{"revision": 1, "status": "testing"}); status != http.StatusConflict || !strings.Contains(out, "FORMAT_READ_ONLY") {
		t.Errorf("publish a standard format: %d %s", status, out)
	}
	if status, out := postJSON(t, e.client, e.srv.URL+"/api/v1/formats/"+std+"/publish", map[string]any{"revision": 0}); status != http.StatusBadRequest {
		t.Errorf("publish revision 0: %d %s", status, out)
	}
	resp, err := e.client.Get(e.srv.URL + "/api/v1/formats/" + std + "/mapping")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var g struct {
		Editable bool   `json:"editable"`
		Reason   string `json:"reason"`
	}
	if err := json.Unmarshal(raw, &g); err != nil || resp.StatusCode != http.StatusOK || g.Editable || g.Reason == "" {
		t.Errorf("mapping of a standard format: %d %s", resp.StatusCode, raw)
	}
	// A staff member may teach a layout but not name a format to edit.
	staff := userClient(t, e, "sunil", "staff")
	if status, code := codeAndStatus(t, staff, http.MethodGet, e.srv.URL+"/api/v1/formats/"+std+"/mapping", ""); status != http.StatusForbidden || code != "FORBIDDEN" {
		t.Errorf("staff read a mapping: %d %s", status, code)
	}
}

// SPEC-016: unrecognised files, kept samples, and export and import, through the API.
func TestUnrecognisedSamplesAndBundles(t *testing.T) {
	e := newEnv(t)
	std := getFormats(t, e).Formats[0].ID
	post := func(path string, body any) (int, string) {
		return postJSON(t, e.client, e.srv.URL+path, body)
	}
	if status, out := post("/api/v1/unrecognised/999/resolve", map[string]string{"status": "taught"}); status != http.StatusNotFound || !strings.Contains(out, "NOT_FOUND") {
		t.Errorf("resolve an unknown file: %d %s", status, out)
	}
	if status, out := post("/api/v1/unrecognised/1/resolve", map[string]string{"status": "dismissed", "note": ""}); status != http.StatusUnprocessableEntity || !strings.Contains(out, "REASON_REQUIRED") {
		t.Errorf("dismiss with no reason: %d %s", status, out)
	}
	if status, out := post("/api/v1/unrecognised/x/resolve", map[string]string{"status": "taught"}); status != http.StatusBadRequest {
		t.Errorf("bad id: %d %s", status, out)
	}
	if status, out := post("/api/v1/format-samples/999/remove", map[string]string{}); status != http.StatusNotFound {
		t.Errorf("remove an unknown sample: %d %s", status, out)
	}
	if status, out := post("/api/v1/formats/import", map[string]any{"bundle": map[string]string{"schema": "other"}, "check_only": true}); status != http.StatusUnprocessableEntity || !strings.Contains(out, "BAD_BUNDLE") {
		t.Errorf("import a bad bundle: %d %s", status, out)
	}
	if status, out := post("/api/v1/formats/import", map[string]any{}); status != http.StatusBadRequest {
		t.Errorf("import nothing: %d %s", status, out)
	}
	resp, err := e.client.Get(e.srv.URL + "/api/v1/formats/" + std + "/export")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict || !strings.Contains(string(raw), "FORMAT_READ_ONLY") {
		t.Errorf("export a standard format: %d %s", resp.StatusCode, raw)
	}
	// The list carries the new sections.
	l := getFormats(t, e)
	if l.Unrecognised == nil {
		t.Error("the list has no unrecognised files section")
	}
}
