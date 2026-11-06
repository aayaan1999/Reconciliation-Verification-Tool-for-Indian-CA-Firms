package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SPEC-021: the Currency endpoints. Any signed-in user may read the rate in force; only the
// Partner may change it; a change goes through the engine, which audits it. Values are illustrative.

type currencyReply struct {
	Set           bool   `json:"set"`
	USDRate       string `json:"usd_rate"`
	EffectiveFrom string `json:"effective_from"`
	Note          string `json:"note"`
	Problem       string `json:"problem"`
	History       []struct {
		By       string `json:"by"`
		USDRate  string `json:"usd_rate"`
		Previous string `json:"previous"`
	} `json:"history"`
}

func getCurrency(t *testing.T, c *http.Client, base string) currencyReply {
	t.Helper()
	status, body := do(t, c, base, "GET", "/api/v1/currency", "")
	if status != http.StatusOK {
		t.Fatalf("GET currency: %d %s", status, body)
	}
	var r currencyReply
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

const setBody = `{"usd_rate": "83.2500", "effective_from": "2026-11-04", "note": "morning rate"}`

// AC-5: reading is for any signed-in user, changing is for the Partner, and the answer says who changed it.
func TestCurrencyEndpoints(t *testing.T) {
	e := newEnv(t)
	staff, manager, readOnly := users(t, e)

	if r := getCurrency(t, e.client, e.srv.URL); r.Set || r.Problem != "dollar rate not set" {
		t.Errorf("before any rate: %+v", r)
	}
	// Every signed-in role may read.
	for name, c := range map[string]*http.Client{"staff": staff, "manager": manager, "read_only": readOnly} {
		if status, body := do(t, c, e.srv.URL, "GET", "/api/v1/currency", ""); status != http.StatusOK {
			t.Errorf("%s read: %d %s", name, status, body)
		}
	}
	// Nobody who is not signed in may.
	anon := e.newClient(t)
	if status, _ := do(t, anon, e.srv.URL, "GET", "/api/v1/currency", ""); status != http.StatusUnauthorized {
		t.Errorf("not signed in: %d, want 401", status)
	}
	// Only the Partner may change; the refusal changes nothing.
	for name, c := range map[string]*http.Client{"staff": staff, "manager": manager, "read_only": readOnly, "not signed in": anon} {
		status, body := do(t, c, e.srv.URL, "PUT", "/api/v1/currency", setBody)
		if status != http.StatusForbidden && status != http.StatusUnauthorized {
			t.Errorf("%s changed the rate: %d %s", name, status, body)
		}
	}
	if r := getCurrency(t, e.client, e.srv.URL); r.Set {
		t.Fatalf("a refused change was applied: %+v", r)
	}

	status, body := do(t, e.client, e.srv.URL, "PUT", "/api/v1/currency", setBody)
	if status != http.StatusOK {
		t.Fatalf("Partner change: %d %s", status, body)
	}
	r := getCurrency(t, staff, e.srv.URL) // a Staff user now sees the rate in force
	if !r.Set || r.USDRate != "83.2500" || r.EffectiveFrom != "2026-11-04" || r.Note != "morning rate" {
		t.Errorf("after the change: %+v", r)
	}
	if len(r.History) != 1 || r.History[0].By != testPartner || r.History[0].Previous != "" {
		t.Errorf("history: %+v", r.History)
	}
	// The file is in the currency folder beside the database, where the next run reads it.
	if _, err := os.Stat(filepath.Join(filepath.Dir(e.db), "currency", "usd-rate.yaml")); err != nil {
		t.Errorf("the rate file: %v", err)
	}
}

// Invalid changes are refused with the engine's message and change nothing; a request cannot
// name the actor.
func TestCurrencyEndpointRefusals(t *testing.T) {
	e := newEnv(t)
	for _, c := range []struct{ name, body, status, code string }{
		{"not a number", `{"usd_rate": "abc", "effective_from": "2026-11-04", "note": ""}`, "422", "BAD_RATE"},
		{"a slip", `{"usd_rate": "8325", "effective_from": "2026-11-04", "note": ""}`, "422", "BAD_RATE"},
		{"bad date", `{"usd_rate": "83", "effective_from": "2026-13-01", "note": ""}`, "422", "BAD_DATE"},
		{"a line break", `{"usd_rate": "83", "effective_from": "2026-11-04", "note": "a\nb"}`, "422", "BAD_NOTE"},
		{"names the actor", `{"by": "someone else", "usd_rate": "83", "effective_from": "2026-11-04", "note": ""}`, "400", "BAD_REQUEST"},
		{"not JSON", `83.25`, "400", "BAD_REQUEST"},
	} {
		status, body := do(t, e.client, e.srv.URL, "PUT", "/api/v1/currency", c.body)
		if got := map[int]string{422: "422", 400: "400"}[status]; got != c.status || !strings.Contains(body, c.code) {
			t.Errorf("%s: %d %s, want %s %s", c.name, status, body, c.status, c.code)
		}
	}
	if r := getCurrency(t, e.client, e.srv.URL); r.Set || len(r.History) != 0 {
		t.Errorf("a refused change was applied: %+v", r)
	}
}

// AC-4: a rate set on the page is the one a run through the server uses, and the run stores it.
func TestCurrencyChangeReachesTheRun(t *testing.T) {
	e := newEnv(t)
	if status, body := do(t, e.client, e.srv.URL, "PUT", "/api/v1/currency", `{"usd_rate": "84", "effective_from": "2026-11-05", "note": ""}`); status != http.StatusOK {
		t.Fatalf("set: %d %s", status, body)
	}
	book := "date,voucher_no,particulars,instrument_ref,receipt,payment\n2026-09-03,RCPT-1,Customer A (fictional),R1,\"$1,000.00\",0\n"
	stmt := "txn_date,value_date,description,ref_no,debit,credit\n2026-09-03,2026-09-03,NEFT CR R1,R1,0,84000.00\n"
	status, out := e.post(t, "2026-27", true, upload{"bank_book.csv", []byte(book)}, upload{"bank_statement.csv", []byte(stmt)})
	if status != http.StatusOK {
		t.Fatalf("run: %d %v", status, out)
	}
	report := string(out["report"])
	if !strings.Contains(report, `"code":"MATCHED"`) || !strings.Contains(report, `"rule_id":"RECON-BNK-010"`) || strings.Contains(report, "dollar rate not") {
		t.Errorf("$1,000.00 at 84 should match 84,000.00 in the statement (and not be unreadable for want of a rate): %.600s", report)
	}
}
