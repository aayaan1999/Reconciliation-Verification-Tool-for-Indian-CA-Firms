package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/auth"
)

// SPEC-013 over HTTP: the role checks on the rule, run, client, export and deletion routes
// (AC-1 to AC-4, AC-7a to AC-7d), the audit entries of views and refusals (AC-4, AC-5), and the
// masking of GSTINs (AC-6). The users are made with userClient (auth_test.go); the names and
// figures are illustrative.

// The synthetic GSTIN and PAN of the test client; the GSTIN's check character is valid.
const (
	testGSTIN = "27ZLVCD9565Z1ZF"
	testPAN   = "AAKFS5678M"
)

// do sends one request as the client c and returns the status and the body text.
func do(t *testing.T, c *http.Client, base, method, path, body string) (int, string) {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, base+path, rd)
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
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

// refusedBy returns the permission a 403 body names, and the error code.
func refusedBy(body string) (perm, errCode string) {
	var out struct {
		Permission string `json:"permission"`
		ErrorCode  string `json:"error_code"`
	}
	json.Unmarshal([]byte(body), &out)
	return out.Permission, out.ErrorCode
}

// auditState is the audit log as the Partner's Admin tab reads it.
type auditState struct {
	Total int `json:"total"`
	Chain struct {
		OK bool `json:"ok"`
	} `json:"chain"`
	Entries []struct {
		Actor      string            `json:"actor"`
		Action     string            `json:"action"`
		ObjectType string            `json:"object_type"`
		ObjectID   string            `json:"object_id"`
		Details    map[string]string `json:"details"`
	} `json:"entries"`
}

// auditOf reads the audit log through the Admin endpoint, as the Partner.
func auditOf(t *testing.T, e env) auditState {
	t.Helper()
	_, raw := e.get(t, "/api/v1/admin/audit")
	var st auditState
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		t.Fatalf("the audit log: %v: %.200s", err, raw)
	}
	return st
}

// newest returns the indexes of the entries with an action, newest first (the Admin endpoint lists
// the newest entry first).
func newest(st auditState, action string) []int {
	var out []int
	for i := 0; i < len(st.Entries); i++ {
		if st.Entries[i].Action == action {
			out = append(out, i)
		}
	}
	return out
}

// users makes the three non-Partner test users and returns their sessions.
func users(t *testing.T, e env) (staff, manager, readOnly *http.Client) {
	t.Helper()
	return userClient(t, e, "sunil", "staff"), userClient(t, e, "meera", "manager"), userClient(t, e, "tara", "read_only")
}

// AC-1: activating, approving, enabling, switching versions, resetting and disabling a rule are
// refused to Staff and Manager with 403 and the permission name; the Partner's request is
// allowed. A law-table row has no route in this build (no law-table endpoint exists).
func TestAC1RuleActivationIsPartnerOnly(t *testing.T) {
	e := newEnv(t)
	staff, manager, _ := users(t, e)
	if status, out := e.sendJSON(t, http.MethodPost, "/api/v1/rules", map[string]any{"by": "A. Partner", "rule": testRuleForm()}); status != http.StatusCreated {
		t.Fatalf("create: %d %s", status, out["message"])
	}
	const id = "/api/v1/rules/RECON-OFF-101"
	for _, c := range []struct {
		method, path, body, perm string
	}{
		{"POST", id + "/disable", `{"by":"x","reason":"fires on test data"}`, "disable_rules"},
		{"POST", id + "/enable", `{"by":"x","reason":"checked"}`, "activate_rules"},
		{"POST", id + "/approve", `{"by":"x","reason":"agreed"}`, "activate_rules"},
		{"POST", id + "/versions/1/switch", `{}`, "activate_rules"},
		{"POST", id + "/reset", `{"by":"x"}`, "activate_rules"},
	} {
		for _, who := range []struct {
			name string
			c    *http.Client
		}{{"staff", staff}, {"manager", manager}} {
			status, body := do(t, who.c, e.srv.URL, c.method, c.path, c.body)
			perm, code := refusedBy(body)
			if status != http.StatusForbidden || code != "FORBIDDEN" || perm != c.perm {
				t.Errorf("%s %s as %s: %d %s (permission %q), want 403 FORBIDDEN with %s", c.method, c.path, who.name, status, code, perm, c.perm)
			}
		}
		status, body := do(t, e.client, e.srv.URL, c.method, c.path, c.body)
		t.Logf("the Partner's %s %s: %d %.120s", c.method, c.path, status, body)
		if status == http.StatusForbidden {
			t.Errorf("%s %s as the Partner: refused (%s)", c.method, c.path, body)
		}
	}
	// The Partner's disable and enable succeed on the rule (the rule is saved as testing).
	if status, body := do(t, e.client, e.srv.URL, "POST", id+"/disable", `{"by":"x","reason":"fires on test data"}`); status != http.StatusOK {
		t.Errorf("the Partner's disable: %d %s", status, body)
	}
	if status, body := do(t, e.client, e.srv.URL, "POST", id+"/enable", `{"by":"x","reason":"checked"}`); status != http.StatusOK {
		t.Errorf("the Partner's enable: %d %s", status, body)
	}
}

// AC-2: Staff, Manager and Partner can save a draft rule, which runs its fixtures (the save is the
// test run); Read-only cannot save or edit one.
func TestAC2DraftRulesStaffManagerPartner(t *testing.T) {
	e := newEnv(t)
	staff, manager, readOnly := users(t, e)
	for i, c := range []struct {
		name string
		c    *http.Client
	}{{"staff", staff}, {"manager", manager}, {"partner", e.client}} {
		form := testRuleForm()
		form.ID = "RECON-OFF-20" + string(rune('1'+i))
		b, _ := json.Marshal(map[string]any{"by": c.name, "rule": form})
		if status, body := do(t, c.c, e.srv.URL, "POST", "/api/v1/rules", string(b)); status != http.StatusCreated {
			t.Errorf("save a draft as %s: %d %s", c.name, status, body)
		}
	}
	form := testRuleForm()
	form.ID = "RECON-OFF-209"
	b, _ := json.Marshal(map[string]any{"by": "tara", "rule": form})
	if status, body := do(t, readOnly, e.srv.URL, "POST", "/api/v1/rules", string(b)); status != http.StatusForbidden {
		t.Errorf("save a draft as read-only: %d %s, want 403", status, body)
	} else if perm, _ := refusedBy(body); perm != "draft_rules" {
		t.Errorf("read-only refusal names %q, want draft_rules", perm)
	}
	if status, body := do(t, readOnly, e.srv.URL, "PUT", "/api/v1/rules/RECON-OFF-201", string(b)); status != http.StatusForbidden {
		t.Errorf("edit a draft as read-only: %d %s, want 403", status, body)
	}
}

// AC-3: Read-only cannot upload, run a reconciliation or enter client details; Staff can.
func TestAC3ReadOnlyCannotRunOrEnter(t *testing.T) {
	e := newEnv(t)
	staff, _, readOnly := users(t, e)
	clientBody := `{"tax_year":"2025-26","name":"Staff Test Firm","pan":"` + testPAN + `"}`
	for _, c := range []struct {
		method, path, body string
	}{
		{"POST", "/api/v1/clients", clientBody},
		{"PUT", "/api/v1/clients/1", clientBody},
		{"POST", "/api/v1/inspect", "{}"},
		{"POST", "/api/v1/profiles", "{}"},
	} {
		if status, body := do(t, readOnly, e.srv.URL, c.method, c.path, c.body); status != http.StatusForbidden {
			t.Errorf("%s %s as read-only: %d %s, want 403", c.method, c.path, status, body)
		} else if perm, _ := refusedBy(body); perm != "run" {
			t.Errorf("%s %s: refusal names %q, want run", c.method, c.path, perm)
		}
	}
	if status, body := e.runAs(t, readOnly, [][2]string{{"tax_year", "2025-26"}}, sample(t, "taxsuite", "comp_v3_2025-26_differs.json")); status != http.StatusForbidden {
		t.Errorf("a run as read-only: %d %s, want 403", status, body)
	}
	if status, body := e.runAs(t, staff, [][2]string{{"tax_year", "2025-26"}}, sample(t, "taxsuite", "comp_v3_2025-26_differs.json")); status != http.StatusOK {
		t.Errorf("a run as staff: %d %s", status, body)
	}
	if status, body := do(t, staff, e.srv.URL, "POST", "/api/v1/clients", clientBody); status != http.StatusCreated {
		t.Errorf("a client as staff: %d %s", status, body)
	}
}

// runAs posts a run as the client c (the test's upload form, with the caller's session).
func (e env) runAs(t *testing.T, c *http.Client, fields [][2]string, files ...upload) (int, string) {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, f := range fields {
		mw.WriteField(f[0], f[1])
	}
	for _, f := range files {
		w, _ := mw.CreateFormFile("files", f.name)
		w.Write(f.data)
	}
	mw.Close()
	resp, err := c.Post(e.srv.URL+"/api/v1/runs", mw.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

// AC-4: a refused request writes exactly one auth.denied entry with the user, the route and the
// permission and no client figures; the audit chain still verifies.
func TestAC4RefusalWritesOneEntry(t *testing.T) {
	e := newEnv(t)
	staff, _, _ := users(t, e)
	before := auditOf(t, e)
	if status, body := do(t, staff, e.srv.URL, "POST", "/api/v1/rules/RECON-OFF-101/disable", `{"by":"x","reason":"r"}`); status != http.StatusForbidden {
		t.Fatalf("staff disable: %d %s", status, body)
	}
	after := auditOf(t, e)
	if after.Total != before.Total+1 {
		t.Fatalf("audit entries went from %d to %d, want exactly one more", before.Total, after.Total)
	}
	last := after.Entries[0] // the newest entry
	if last.Action != "auth.denied" || last.Actor != "sunil" || last.ObjectID != "POST /api/v1/rules/{id}/disable" ||
		last.Details["permission"] != "disable_rules" || last.Details["role"] != "staff" || len(last.Details) != 2 {
		t.Errorf("the entry: %+v", last)
	}
	if !after.Chain.OK {
		t.Error("the audit chain does not verify")
	}
}

// AC-5: a view of a client file writes one client.view entry per user, client and tax year a
// minute, whichever role opens it; the chain verifies.
func TestAC5ClientViewsAudited(t *testing.T) {
	e := newEnv(t)
	if status, body := e.addClient(t, "View Test Firm", testPAN); status != http.StatusCreated {
		t.Fatalf("add client: %d %v", status, body)
	}
	_, _, readOnly := users(t, e)
	base := auditOf(t, e)
	views := func() int { return len(newest(auditOf(t, e), "client.view")) }
	n0 := views()
	for i := 0; i < 2; i++ { // reloads within the minute
		if status, body := do(t, readOnly, e.srv.URL, "GET", "/api/v1/clients/1/result?tax_year=2025-26", ""); status != http.StatusOK {
			t.Fatalf("result: %d %s", status, body)
		}
	}
	if got := views() - n0; got != 1 {
		t.Errorf("two reloads in one minute wrote %d entries, want 1", got)
	}
	if status, body := do(t, readOnly, e.srv.URL, "GET", "/api/v1/clients/1?tax_year=2025-26", ""); status != http.StatusOK {
		t.Fatalf("details: %d %s", status, body)
	}
	if got := views() - n0; got != 1 {
		t.Errorf("the details of the same year in the same minute wrote another entry (%d in all)", got)
	}
	if status, body := do(t, readOnly, e.srv.URL, "GET", "/api/v1/clients/1/result?tax_year=2026-27", ""); status != http.StatusOK {
		t.Fatalf("result 2026-27: %d %s", status, body)
	}
	st := auditOf(t, e)
	if got := len(newest(st, "client.view")) - n0; got != 2 {
		t.Errorf("a second tax year gives %d entries in all, want 2", got)
	}
	if idx := newest(st, "client.view"); len(idx) == 0 || st.Entries[idx[0]].Actor != "tara" || st.Entries[idx[0]].ObjectID != "1" || st.Entries[idx[0]].Details["tax_year"] != "2026-27" {
		t.Errorf("the newest view: %+v", st.Entries[idx[0]])
	}
	if !st.Chain.OK || base.Total >= st.Total {
		t.Error("the audit chain does not verify, or no entry was added")
	}
}

// AC-6 (GSTIN part): a Read-only user sees a masked GSTIN in the API response; the roles that enter
// client details see it in full. The PAN is never sent in full (only its last four characters are
// stored); see the report on SPEC-013 AC-6.
func TestAC6GSTINMaskedForReadOnly(t *testing.T) {
	e := newEnv(t)
	staff, manager, readOnly := users(t, e)
	// The GSTIN's own PAN part (characters 3 to 12) must be the client's PAN, so the PAN here is
	// the one inside the synthetic GSTIN.
	const pan = "ZLVCD9565Z"
	add := `{"tax_year":"2025-26","details":{"legal_name":"Mask Test Firm","pan":"` + pan + `","gstins":[{"gstin":"` + testGSTIN + `","registration_type":"regular","primary":true}]}}`
	if status, body := do(t, e.client, e.srv.URL, "POST", "/api/v1/clients", add); status != http.StatusCreated {
		t.Fatalf("add: %d %s", status, body)
	}
	masked := "***********Z1ZF"
	for _, c := range []struct {
		name string
		c    *http.Client
		full bool
	}{{"partner", e.client, true}, {"manager", manager, true}, {"staff", staff, true}, {"read_only", readOnly, false}} {
		status, body := do(t, c.c, e.srv.URL, "GET", "/api/v1/clients/1", "")
		if status != http.StatusOK {
			t.Fatalf("details as %s: %d %s", c.name, status, body)
		}
		if has := strings.Contains(body, testGSTIN); has != c.full {
			t.Errorf("as %s the full GSTIN is in the response: %v, want %v", c.name, has, c.full)
		}
		if !c.full && !strings.Contains(body, masked) {
			t.Errorf("as read-only the response lacks the masked GSTIN %q: %.300s", masked, body)
		}
		// The PAN is inside the GSTIN, so only the masked view can be checked for it.
		if !c.full && strings.Contains(body, pan) {
			t.Errorf("as %s the PAN shows through the masked GSTIN", c.name)
		}
	}
}

// AC-7 (walk): the export and deletion routes are declared with their permissions, and the route
// walk covers them for all four roles (TestEveryRouteFollowsTheRoleMatrix).
func TestExportAndDeleteRoutesAreDeclared(t *testing.T) {
	e := newEnv(t)
	want := map[string]auth.Permission{
		"POST /api/v1/clients/{id}/exports":                   auth.PermExport,
		"POST /api/v1/clients/{id}/query-letter":              auth.PermExport,
		"POST /api/v1/clients/{id}/findings/{finding}/delete": auth.PermDeleteFindings,
	}
	for _, ep := range e.server.endpoints() {
		if p, ok := want[ep.pattern]; ok {
			if ep.perm != p {
				t.Errorf("%s declares %s, want %s", ep.pattern, ep.perm, p)
			}
			delete(want, ep.pattern)
		}
	}
	for pattern := range want {
		t.Errorf("route %s is not declared", pattern)
	}
}

// exportFile downloads a client's Excel export as the client c.
func exportFile(t *testing.T, c *http.Client, base string, body string) (int, []byte, http.Header) {
	t.Helper()
	req, err := http.NewRequest("POST", base+"/api/v1/clients/1/exports", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, raw, resp.Header
}

// firstFindingID reads a finding's ID from a client's latest result.
func firstFindingID(t *testing.T, e env) string {
	t.Helper()
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
	return res.Results[0].Report.Findings[0].Findings[0].FindingID
}

// setUpRun gives client 1 a run of the synthetic tax-suite sample, whose findings the export
// and the deletion tests use.
func setUpRun(t *testing.T, e env) {
	t.Helper()
	if status, body := e.addClient(t, "Export Test Firm", testPAN); status != http.StatusCreated {
		t.Fatalf("add client: %d %v", status, body)
	}
	if status, out := e.postFor(t, "1", "2025-26", true, sample(t, "taxsuite", "comp_v3_2025-26_differs.json")); status != http.StatusOK {
		t.Fatalf("run: %d %s", status, out["message"])
	}
}

// AC-7a and AC-7b: Partner and Manager export an Excel file; Staff and Read-only get 403. The file
// has the review sheets, the cannot-check sheet and the Worklist sheet, each with the watermark; the
// export is recorded with its ID, client, tax year, format and row counts.
func TestAC7aAndAC7bExcelExport(t *testing.T) {
	e := newEnv(t)
	setUpRun(t, e)
	staff, manager, readOnly := users(t, e)
	const req = `{"tax_year":"2025-26","format":"xlsx"}`
	for _, c := range []struct {
		name string
		c    *http.Client
	}{{"staff", staff}, {"read_only", readOnly}} {
		status, body, _ := exportFile(t, c.c, e.srv.URL, req)
		if perm, _ := refusedBy(string(body)); status != http.StatusForbidden || perm != "export" {
			t.Errorf("export as %s: %d %s, want 403 naming export", c.name, status, body)
		}
	}
	if st := auditOf(t, e); len(newest(st, "export.created")) != 0 {
		t.Fatal("a refused export wrote an export.created entry")
	}
	status, file, header := exportFile(t, manager, e.srv.URL, req)
	if status != http.StatusOK {
		t.Fatalf("export as manager: %d %s", status, file)
	}
	if ct := header.Get("Content-Type"); !strings.Contains(ct, "spreadsheetml") {
		t.Errorf("content type %q", ct)
	}
	name := header.Get("Content-Disposition")
	exportID := strings.TrimSuffix(strings.TrimPrefix(name, `attachment; filename="recon-export-`), `.xlsx"`)
	if len(exportID) != 16 || !strings.HasPrefix(name, `attachment; filename="recon-export-`) {
		t.Fatalf("the file name %q is not server-generated", name)
	}
	f, err := excelize.OpenReader(bytes.NewReader(file))
	if err != nil {
		t.Fatalf("the export does not open: %v", err)
	}
	defer f.Close()
	sheets := f.GetSheetList()
	hasCannot, hasWorklist, reviewSheets := false, false, 0
	for _, s := range sheets {
		switch s {
		case "Cannot check":
			hasCannot = true
		case "Worklist":
			hasWorklist = true
		default:
			reviewSheets++
		}
		line, _ := f.GetCellValue(s, "A1")
		for _, part := range []string{"Watermark:", "meera", exportID} {
			if !strings.Contains(line, part) {
				t.Errorf("sheet %s watermark %q lacks %q", s, line, part)
			}
		}
	}
	if !hasCannot || !hasWorklist || reviewSheets == 0 {
		t.Errorf("sheets %v: want review sheets, Cannot check and Worklist", sheets)
	}
	st := auditOf(t, e)
	idx := newest(st, "export.created")
	if len(idx) != 1 {
		t.Fatalf("export.created entries: %d, want 1", len(idx))
	}
	ent := st.Entries[idx[0]]
	if ent.Actor != "meera" || ent.ObjectID != exportID || ent.Details["client_id"] != "1" || ent.Details["tax_year"] != "2025-26" ||
		ent.Details["format"] != "xlsx" || ent.Details["findings"] == "" || ent.Details["worklist"] == "" || ent.Details["cannot_check"] == "" {
		t.Errorf("the export entry: %+v", ent)
	}
	// Only the row counts and the identifiers: no figure of the client is in the entry.
	keys := make([]string, 0, len(ent.Details))
	for k := range ent.Details {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if strings.Join(keys, ",") != "cannot_check,client_id,findings,format,sheets,tax_year,worklist" {
		t.Errorf("the export entry holds %v", keys)
	}
	if !st.Chain.OK {
		t.Error("the audit chain does not verify")
	}
}

// AC-7c: Partner and Manager delete a finding with a reason; Staff and Read-only get 403. The
// finding is kept: a Partner still lists it, marked deleted, and the other roles do not see it.
// One finding.deleted entry holds the reason.
func TestAC7cDeleteFinding(t *testing.T) {
	e := newEnv(t)
	setUpRun(t, e)
	staff, manager, readOnly := users(t, e)
	id := firstFindingID(t, e)
	del := func(c *http.Client, body string) (int, string) {
		return do(t, c, e.srv.URL, "POST", "/api/v1/clients/1/findings/"+id+"/delete", body)
	}
	for _, c := range []struct {
		name string
		c    *http.Client
	}{{"staff", staff}, {"read_only", readOnly}} {
		if status, body := del(c.c, `{"tax_year":"2025-26","reason":"not needed"}`); status != http.StatusForbidden {
			t.Errorf("delete as %s: %d %s, want 403", c.name, status, body)
		} else if perm, _ := refusedBy(body); perm != "delete_findings" {
			t.Errorf("delete as %s names %q", c.name, perm)
		}
	}
	if status, body := del(manager, `{"tax_year":"2025-26","reason":" "}`); status != http.StatusBadRequest || !strings.Contains(body, "REASON_REQUIRED") {
		t.Errorf("delete without a reason: %d %s", status, body)
	}
	if status, body := del(manager, `{"tax_year":"2025-26","reason":"Duplicate of the schedule row; agreed with the partner"}`); status != http.StatusOK {
		t.Fatalf("delete as manager: %d %s", status, body)
	}
	if n := countFinding(t, do2(t, manager, e.srv.URL, "/api/v1/clients/1/result?tax_year=2025-26"), id); n != 0 {
		t.Errorf("the manager still sees the deleted finding")
	}
	partnerView := do2(t, e.client, e.srv.URL, "/api/v1/clients/1/result?tax_year=2025-26")
	// The finding is in the latest report and in the result of its type, so it appears more than
	// once in the body; each place must carry the deleted mark.
	if n := countFinding(t, partnerView, id); n == 0 || strings.Count(partnerView, `"deleted":{`) != n {
		t.Errorf("the Partner does not list the deleted finding marked deleted (%d listed)", n)
	}
	if status, body := del(manager, `{"tax_year":"2025-26","reason":"again"}`); status != http.StatusConflict {
		t.Errorf("a second delete: %d %s, want 409", status, body)
	}
	st := auditOf(t, e)
	idx := newest(st, "finding.deleted")
	if len(idx) != 1 || st.Entries[idx[0]].Actor != "meera" || st.Entries[idx[0]].Details["reason"] != "Duplicate of the schedule row; agreed with the partner" {
		t.Errorf("finding.deleted entries: %+v", idx)
	}
}

// do2 is a GET whose body is returned, as a string.
func do2(t *testing.T, c *http.Client, base, path string) string {
	t.Helper()
	_, body := do(t, c, base, "GET", path, "")
	return body
}

// countFinding counts the places a finding ID appears in a result body.
func countFinding(t *testing.T, body, id string) int {
	t.Helper()
	return strings.Count(body, `"finding_id":"`+id+`"`)
}

// AC-7d: a PDF request and the query letter return "not available yet" to a Partner, and write no
// export entry; Staff are refused by the role first.
func TestAC7dPDFAndQueryLetterNotAvailable(t *testing.T) {
	e := newEnv(t)
	setUpRun(t, e)
	staff, _, _ := users(t, e)
	before := auditOf(t, e)
	if status, body, _ := exportFile(t, e.client, e.srv.URL, `{"tax_year":"2025-26","format":"pdf"}`); status != http.StatusNotImplemented || !strings.Contains(string(body), "NOT_AVAILABLE_YET") {
		t.Errorf("pdf as partner: %d %s", status, body)
	}
	if status, body := do(t, e.client, e.srv.URL, "POST", "/api/v1/clients/1/query-letter", "{}"); status != http.StatusNotImplemented || !strings.Contains(body, "NOT_AVAILABLE_YET") {
		t.Errorf("query letter as partner: %d %s", status, body)
	}
	if status, body, _ := exportFile(t, staff, e.srv.URL, `{"tax_year":"2025-26","format":"pdf"}`); status != http.StatusForbidden {
		t.Errorf("pdf as staff: %d %s, want 403", status, body)
	}
	after := auditOf(t, e)
	if after.Total != before.Total+1 { // only the refusal of staff is written
		t.Errorf("entries went from %d to %d; a PDF or letter request must write no export", before.Total, after.Total)
	}
	if len(newest(after, "export.created")) != 0 {
		t.Error("a PDF request wrote an export.created entry")
	}
}
