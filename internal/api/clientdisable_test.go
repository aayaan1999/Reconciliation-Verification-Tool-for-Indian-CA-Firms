package api

import (
	"net/http"
	"strings"
	"testing"
)

// SPEC-004 part 10 (AC-36 to AC-40) over HTTP. All names and PANs are invented.
func TestDisableClientThroughServer(t *testing.T) {
	e := newEnv(t) // signed in as the first Partner
	if status, out := e.addClient(t, "Sample Engineering Works", "AAKFS5678M"); status != http.StatusCreated {
		t.Fatalf("add: %d %s", status, out)
	}
	if status, out := e.postFor(t, "1", "2025-26", true, sample(t, "compare-differs", "fs_2025-26_differs.json")); status != http.StatusOK {
		t.Fatalf("upload: %d %s", status, out["message"])
	}
	base := e.srv.URL + "/api/v1/clients/1"

	// AC-40: only a Partner may disable or enable.
	for _, role := range []string{"manager", "staff", "read_only"} {
		c := userClient(t, e, "user_"+role, role)
		for _, action := range []string{"/disable", "/enable"} {
			if status, code := codeAndStatus(t, c, "POST", base+action, `{"reason":"left"}`); status != http.StatusForbidden || code != "FORBIDDEN" {
				t.Errorf("%s %s: %d %s, want 403 FORBIDDEN", role, action, status, code)
			}
		}
	}

	// AC-36: a reason is required.
	if status, code := codeAndStatus(t, e.client, "POST", base+"/disable", `{"reason":""}`); status != http.StatusUnprocessableEntity || code != "REASON_REQUIRED" {
		t.Errorf("no reason: %d %s", status, code)
	}
	if status, out := postJSON(t, e.client, base+"/disable", map[string]string{"reason": "no longer a client of the firm"}); status != http.StatusOK || !strings.Contains(out, `"status":"disabled"`) {
		t.Fatalf("disable: %d %s", status, out)
	}
	if status, code := codeAndStatus(t, e.client, "POST", base+"/disable", `{"reason":"again"}`); status != http.StatusConflict || code != "ALREADY_DISABLED" {
		t.Errorf("second disable: %d %s", status, code)
	}

	// AC-37: off the board unless all=1, then marked disabled.
	if _, list := e.get(t, "/api/v1/clients?tax_year=2025-26"); strings.Contains(list, "Sample Engineering Works") {
		t.Errorf("the board lists a disabled client: %s", list)
	}
	if _, list := e.get(t, "/api/v1/clients?tax_year=2025-26&all=1"); !strings.Contains(list, `"status":"disabled"`) ||
		!strings.Contains(list, `"disabled_reason":"no longer a client of the firm"`) {
		t.Errorf("all=1 does not show the disabled client: %s", list)
	}

	// AC-38: read only; every change is refused with 409 CLIENT_DISABLED.
	if status, result := e.get(t, "/api/v1/clients/1/result?tax_year=2025-26"); status != http.StatusOK || !strings.Contains(result, `"status":"disabled"`) {
		t.Errorf("result of a disabled client: %d", status)
	}
	if status, out := e.postFor(t, "1", "2025-26", true, sample(t, "compare-differs", "comp_v3_2025-26_differs.json")); status != http.StatusConflict || code(out) != "CLIENT_DISABLED" {
		t.Errorf("upload for a disabled client: %d %s", status, code(out))
	}
	if status, code := codeAndStatus(t, e.client, "PUT", base, `{"details":{"legal_name":"Renamed"}}`); status != http.StatusConflict || code != "CLIENT_DISABLED" {
		t.Errorf("edit of a disabled client: %d %s", status, code)
	}
	if status, code := codeAndStatus(t, e.client, "POST", base+"/findings/ABCDEFGHJKLMNPQR/delete", `{"tax_year":"2025-26","reason":"duplicate"}`); status != http.StatusConflict || code != "CLIENT_DISABLED" {
		t.Errorf("delete finding of a disabled client: %d %s", status, code)
	}
	if status, out := e.addClient(t, "Again", "AAKFS5678M"); status != http.StatusConflict || code(out) != "CLIENT_EXISTS" {
		t.Errorf("same PAN while disabled: %d %s", status, code(out))
	}

	// AC-39: enabled again, it is back as it was.
	if status, out := postJSON(t, e.client, base+"/enable", map[string]string{"reason": "engaged again"}); status != http.StatusOK || !strings.Contains(out, `"status":"active"`) {
		t.Fatalf("enable: %d %s", status, out)
	}
	if _, list := e.get(t, "/api/v1/clients?tax_year=2025-26"); !strings.Contains(list, "Sample Engineering Works") || !strings.Contains(list, `"documents":1`) {
		t.Errorf("after enable: %s", list)
	}
}
