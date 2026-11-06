package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SPEC-022 part A3: the Document Master endpoints. The role walk in auth_test.go proves every
// other role is refused the writes; these tests prove what the Partner gets and what any signed-in
// role may read.

// docReq sends one request with an optional JSON body and returns the status and the body.
func docReq(t *testing.T, c *http.Client, method, url, body string) (int, string) {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rd)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

const salesReturnDoc = `{"code":"sales_return","name":"Sales return register","category":"Books",
 "description":"Credit notes issued to customers","applicable_for":["csv","xlsx"],"status":"active",
 "record_type":{"name":"sales_return_row","description":"Rows of the sales return register","columns":[
  {"name":"ref_no","label":"Reference","type":"text","key":true,"required":true},
  {"name":"doc_date","label":"Date","type":"date","key":true,"required":false},
  {"name":"net_amount","label":"Net amount","type":"amount","key":false,"required":false}]}}`

type docList struct {
	Documents []struct {
		Code         string `json:"code"`
		Origin       string `json:"origin"`
		Status       string `json:"status"`
		ReferencedBy struct {
			Formats int `json:"formats"`
			Rules   int `json:"rules"`
		} `json:"referenced_by"`
	} `json:"documents"`
}

func (l docList) has(code string) bool {
	for _, d := range l.Documents {
		if d.Code == code {
			return true
		}
	}
	return false
}

func listDocs(t *testing.T, e env, c *http.Client, query string) docList {
	t.Helper()
	status, out := docReq(t, c, "GET", e.srv.URL+"/api/v1/document-types"+query, "")
	if status != http.StatusOK {
		t.Fatalf("GET document-types%s: %d %s", query, status, out)
	}
	var l docList
	if err := json.Unmarshal([]byte(out), &l); err != nil {
		t.Fatal(err)
	}
	return l
}

func TestDocumentTypesEndpoints(t *testing.T) {
	e := newEnv(t)
	url := e.srv.URL + "/api/v1/document-types"

	// built-in documents are there from the first read
	l := listDocs(t, e, e.client, "")
	if !l.has("gstr1") || !l.has("computation") {
		t.Fatalf("built-in documents are missing: %+v", l)
	}

	// add
	status, out := docReq(t, e.client, "POST", url, salesReturnDoc)
	if status != http.StatusCreated || !strings.Contains(out, `"saved":true`) || !strings.Contains(out, `"status":"active"`) {
		t.Fatalf("POST: %d %s", status, out)
	}
	if status, out := docReq(t, e.client, "POST", url, salesReturnDoc); status != http.StatusConflict || !strings.Contains(out, "DUPLICATE_CODE") {
		t.Errorf("a second POST: %d %s", status, out)
	}
	// fields the Partner cannot set are refused, the actor is always the signed-in user
	if status, out := docReq(t, e.client, "POST", url, strings.Replace(salesReturnDoc, `"code":"sales_return"`, `"code":"other_doc","origin":"builtin"`, 1)); status != http.StatusUnprocessableEntity {
		t.Errorf("an unknown field: %d %s", status, out)
	}
	if status, _ := docReq(t, e.client, "POST", url, `[1]`); status != http.StatusBadRequest {
		t.Errorf("a body that is not an object: %d", status)
	}
	if status, out := docReq(t, e.client, "POST", url, salesReturnDoc+` {"by":"someone-else"}`); status != http.StatusBadRequest {
		t.Errorf("a body with a second JSON value after the document: %d %s", status, out)
	}
	// a document whose own fields are refused: 422 with every problem and its field
	bad := strings.Replace(strings.Replace(salesReturnDoc, `"code":"sales_return"`, `"code":"x"`, 1), `"name":"Sales return register"`, `"name":""`, 1)
	status, out = docReq(t, e.client, "POST", url, bad)
	if status != http.StatusUnprocessableEntity || !strings.Contains(out, "INVALID_DOCUMENT") || !strings.Contains(out, `"problems"`) || !strings.Contains(out, `"field":"code"`) || !strings.Contains(out, `"field":"name"`) {
		t.Errorf("an invalid document: %d %s", status, out)
	}

	// list and the wizard's filters
	l = listDocs(t, e, e.client, "")
	if !l.has("sales_return") {
		t.Error("the new document is not listed")
	}
	if l := listDocs(t, e, e.client, "?active=1&file_type=csv"); !l.has("sales_return") || !l.has("gstr1") {
		t.Errorf("the wizard list for a csv file: %+v", l)
	}
	if l := listDocs(t, e, e.client, "?active=1&file_type=json"); l.has("sales_return") || !l.has("gstr1") {
		t.Errorf("the wizard list for a json file: %+v", l)
	}
	if status, out := docReq(t, e.client, "GET", url+"?file_type=pdf", ""); status != http.StatusBadRequest || !strings.Contains(out, "BAD_FORMATS") {
		t.Errorf("an unknown file type: %d %s", status, out)
	}
	if status, _ := docReq(t, e.client, "GET", url+"?active=maybe", ""); status != http.StatusBadRequest {
		t.Errorf("a bad active value: %d", status)
	}

	// one document with its structure
	status, out = docReq(t, e.client, "GET", url+"/sales_return", "")
	if status != http.StatusOK || !strings.Contains(out, `"label":"Reference"`) || !strings.Contains(out, `"history"`) || !strings.Contains(out, `"document.add"`) {
		t.Errorf("GET one: %d %s", status, out)
	}
	if status, out := docReq(t, e.client, "GET", url+"/no_such_doc", ""); status != http.StatusNotFound {
		t.Errorf("GET an unknown document: %d %s", status, out)
	}
	if status, _ := docReq(t, e.client, "GET", url+"/bad%20code", ""); status != http.StatusBadRequest {
		t.Errorf("GET a bad code: %d", status)
	}

	// validate: nothing is saved
	status, out = docReq(t, e.client, "POST", url+"/validate", strings.Replace(bad, `"code":"x"`, `"code":"debit_notes"`, 1))
	if status != http.StatusOK || !strings.Contains(out, `"valid":false`) {
		t.Errorf("validate: %d %s", status, out)
	}
	if l := listDocs(t, e, e.client, ""); l.has("debit_notes") {
		t.Error("validate saved a document")
	}

	// edit: the code is the path's; a different code in the body is refused
	edit := strings.Replace(salesReturnDoc, `"label":"Reference"`, `"label":"Reference number"`, 1)
	status, out = docReq(t, e.client, "PUT", url+"/sales_return", edit)
	if status != http.StatusOK || !strings.Contains(out, "Reference number") {
		t.Errorf("PUT: %d %s", status, out)
	}
	if status, out := docReq(t, e.client, "PUT", url+"/sales_return", strings.Replace(edit, `"sales_return"`, `"renamed_doc"`, 1)); status != http.StatusBadRequest || !strings.Contains(out, "CODE_MISMATCH") {
		t.Errorf("PUT with another code: %d %s", status, out)
	}
	if status, out := docReq(t, e.client, "PUT", url+"/gstr1", `{"name":"GSTR one","category":"GST","description":"Edited built-in doc","applicable_for":["json"]}`); status != http.StatusConflict || !strings.Contains(out, "BUILTIN") {
		t.Errorf("PUT a built-in: %d %s", status, out)
	}
	if status, _ := docReq(t, e.client, "PUT", url+"/no_such_doc", strings.Replace(edit, `"sales_return"`, `"no_such_doc"`, 1)); status != http.StatusNotFound {
		t.Errorf("PUT an unknown document: %d", status)
	}

	// status
	status, out = docReq(t, e.client, "POST", url+"/sales_return/status", `{"active":false}`)
	if status != http.StatusOK || !strings.Contains(out, `"status":"inactive"`) {
		t.Errorf("deactivate: %d %s", status, out)
	}
	if l := listDocs(t, e, e.client, "?active=1"); l.has("sales_return") {
		t.Error("an inactive document is in the wizard list")
	}
	if status, _ := docReq(t, e.client, "POST", url+"/sales_return/status", `{}`); status != http.StatusBadRequest {
		t.Errorf("a status without active: %d", status)
	}
	if status, out := docReq(t, e.client, "POST", url+"/sales_return/status", `{"active":true}`); status != http.StatusOK || !strings.Contains(out, `"status":"active"`) {
		t.Errorf("activate: %d %s", status, out)
	}

	// delete: a built-in is refused, an unused office document goes
	if status, out := docReq(t, e.client, "DELETE", url+"/gstr1", ""); status != http.StatusConflict || !strings.Contains(out, "BUILTIN") || !strings.Contains(out, `"usage"`) {
		t.Errorf("DELETE a built-in: %d %s", status, out)
	}
	if status, out := docReq(t, e.client, "DELETE", url+"/sales_return", ""); status != http.StatusOK || !strings.Contains(out, `"deleted":true`) {
		t.Errorf("DELETE: %d %s", status, out)
	}
	if status, _ := docReq(t, e.client, "DELETE", url+"/sales_return", ""); status != http.StatusNotFound {
		t.Errorf("DELETE twice: %d", status)
	}

	// every write is in the audit log with the signed-in user as the actor (AC-14)
	st := auditOf(t, e)
	for _, action := range []string{"document.add", "document.edit", "record_type.add", "record_type.edit", "document.deactivate", "document.activate", "document.delete", "record_type.delete"} {
		idx := newest(st, action)
		if len(idx) == 0 {
			t.Errorf("no audit entry %s", action)
			continue
		}
		if st.Entries[idx[0]].Actor != testPartner {
			t.Errorf("%s was done by %q, want %q", action, st.Entries[idx[0]].Actor, testPartner)
		}
	}
}

// AC-14: every role may read the list; only the Partner may change anything.
func TestDocumentTypesRoles(t *testing.T) {
	e := newEnv(t)
	url := e.srv.URL + "/api/v1/document-types"
	if status, out := docReq(t, e.client, "POST", url, salesReturnDoc); status != http.StatusCreated {
		t.Fatalf("POST: %d %s", status, out)
	}
	staff, manager, readOnly := users(t, e)
	for name, c := range map[string]*http.Client{"staff": staff, "manager": manager, "read-only": readOnly} {
		if l := listDocs(t, e, c, "?active=1"); !l.has("sales_return") {
			t.Errorf("%s cannot read the list", name)
		}
		if status, _ := docReq(t, c, "GET", url+"/sales_return", ""); status != http.StatusOK {
			t.Errorf("%s cannot read a document: %d", name, status)
		}
		for _, w := range []struct{ method, path, body string }{
			{"POST", "", salesReturnDoc},
			{"PUT", "/sales_return", salesReturnDoc},
			{"POST", "/sales_return/status", `{"active":false}`},
			{"DELETE", "/sales_return", ""},
			{"POST", "/validate", salesReturnDoc},
		} {
			if status, out := docReq(t, c, w.method, url+w.path, w.body); status != http.StatusForbidden || !strings.Contains(out, "manage_documents") {
				t.Errorf("%s %s %s as %s: %d %s", w.method, "document-types", w.path, name, status, out)
			}
		}
	}
	// nothing changed
	if l := listDocs(t, e, e.client, "?active=1"); !l.has("sales_return") {
		t.Error("a refused request changed the document")
	}
}

// The layout wizard's list: /inspect offers an office document for a file whose format it accepts,
// with the labels and flags of its columns, and not for another format or when it is switched off.
func TestInspectOffersOfficeDocuments(t *testing.T) {
	e := newEnv(t)
	url := e.srv.URL + "/api/v1/document-types"
	if status, out := docReq(t, e.client, "POST", url, strings.Replace(salesReturnDoc, `["csv","xlsx"]`, `["json"]`, 1)); status != http.StatusCreated {
		t.Fatalf("POST: %d %s", status, out)
	}
	data, err := os.ReadFile(filepath.Join(repo, "testdata", "teach", "office_sales_Aug2026.json"))
	if err != nil {
		t.Fatal(err)
	}
	file := upload{"office_sales_Aug2026.json", data}
	status, out := e.teach(t, "/api/v1/inspect", file, nil)
	types := string(out["document_types"])
	if status != http.StatusOK || !strings.Contains(types, `"sales_return"`) || !strings.Contains(types, `"label":"Reference"`) ||
		!strings.Contains(types, `"required":true`) || !strings.Contains(types, `"origin":"office"`) || !strings.Contains(types, `"sales_register"`) {
		t.Fatalf("inspect: %d %s", status, types)
	}
	// switched off: not offered, though layouts already taught on it keep working
	if status, out := docReq(t, e.client, "POST", url+"/sales_return/status", `{"active":false}`); status != http.StatusOK {
		t.Fatalf("deactivate: %d %s", status, out)
	}
	if _, out := e.teach(t, "/api/v1/inspect", file, nil); strings.Contains(string(out["document_types"]), `"sales_return"`) {
		t.Error("an inactive document is offered")
	}
	// a document for csv and xlsx only is not offered for a json file
	if status, out := docReq(t, e.client, "POST", url+"/sales_return/status", `{"active":true}`); status != http.StatusOK {
		t.Fatalf("activate: %d %s", status, out)
	}
	if status, out := docReq(t, e.client, "PUT", url+"/sales_return", strings.Replace(strings.Replace(salesReturnDoc, `["csv","xlsx"]`, `["csv"]`, 1), `"label":"Reference"`, `"label":"Reference"`, 1)); status != http.StatusOK {
		t.Fatalf("PUT: %d %s", status, out)
	}
	if _, out := e.teach(t, "/api/v1/inspect", file, nil); strings.Contains(string(out["document_types"]), `"sales_return"`) {
		t.Error("a csv document is offered for a json file")
	}
}

// docReqJSON sends one request as a client with a JSON-encoded body.
func docReqJSON(t *testing.T, c *http.Client, method, url string, body any) (int, string) {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return docReq(t, c, method, url, string(b))
}
