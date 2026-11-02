package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

// The tests of SPEC-004 part 9 through the server: several types of check in one upload,
// each with the rules the CA chose, and a client's latest result per type.

// postRun uploads files with the given form fields (in order, so a field may repeat).
func (e env) postRun(t *testing.T, fields [][2]string, files ...upload) (int, map[string]json.RawMessage) {
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
	resp, err := e.client.Post(e.srv.URL+"/api/v1/runs", mw.FormDataContentType(), &body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("status %d: response is not JSON: %v", resp.StatusCode, err)
	}
	return resp.StatusCode, out
}

func (e env) addDeccan(t *testing.T) {
	t.Helper()
	body := `{"tax_year": "2026-27", "details": {"legal_name": "Deccan Byte Systems Pvt Ltd", "pan": "ZLVCD9565Z",
		"gstins": [{"gstin": "27ZLVCD9565Z1ZF", "registration_type": "regular", "primary": true}]}}`
	resp, err := e.client.Post(e.srv.URL+"/api/v1/clients", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("add client: %d", resp.StatusCode)
	}
}

type typeResults []struct {
	ComparisonID string `json:"comparison_id"`
	Report       struct {
		Counts      map[string]int    `json:"counts"`
		NotSelected []json.RawMessage `json:"not_selected"`
		Passed      []struct {
			RuleID string `json:"rule_id"`
		} `json:"passed"`
	} `json:"report"`
}

var gstPack = func(t *testing.T) []upload {
	return []upload{sample(t, "gst-sales", "SALES_REG_Aug2026.json"), sample(t, "gst-sales", "GSTR1_Aug2026.json"),
		sample(t, "gst-purchase", "PURCH_REG_Aug2026.json"), sample(t, "gst-purchase", "GSTR2B_Aug2026.json")}
}

// AC-31, AC-33, AC-34: one upload runs two types, each keeps its own result, the board adds
// them up, and a later upload of one type leaves the other's result alone.
func TestRunSeveralTypes(t *testing.T) {
	e := newEnv(t)
	e.addDeccan(t)
	base := [][2]string{{"client_id", "1"}, {"tax_year", "2026-27"}, {"include_testing", "true"}}
	fields := append(base, [2]string{"comparison", "gst_sales_vs_gstr1"}, [2]string{"comparison", "gst_purchase_vs_gstr2b"},
		[2]string{"rules:gst_sales_vs_gstr1", "RECON-GST-001"})
	status, out := e.postRun(t, fields, gstPack(t)...)
	if status != http.StatusOK {
		t.Fatalf("run: %d %s", status, out["message"])
	}
	var res typeResults
	if err := json.Unmarshal(out["results"], &res); err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 || res[0].ComparisonID != "gst_purchase_vs_gstr2b" || res[1].ComparisonID != "gst_sales_vs_gstr1" {
		t.Fatalf("results: %s", out["results"])
	}
	for _, r := range res {
		for _, p := range r.Report.Passed {
			if !strings.HasPrefix(p.RuleID, "RECON-GST-") {
				t.Errorf("%s ran %s, which is not one of its rules", r.ComparisonID, p.RuleID)
			}
		}
	}
	purchase, sales := res[0].Report.Counts, res[1].Report.Counts
	_, list := e.get(t, "/api/v1/clients?tax_year=2026-27")
	var board struct {
		Clients []struct {
			Counts map[string]int `json:"counts"`
		} `json:"clients"`
	}
	json.Unmarshal([]byte(list), &board)
	if len(board.Clients) != 1 || board.Clients[0].Counts["fail"] != sales["fail"]+purchase["fail"] ||
		board.Clients[0].Counts["checks"] != sales["checks"]+purchase["checks"] {
		t.Errorf("board %s, want the sum of %v and %v", list, sales, purchase)
	}

	// The sales type again, alone: the purchase result is still there, unchanged.
	status, out = e.postRun(t, append(base, [2]string{"comparison", "gst_sales_vs_gstr1"}), gstPack(t)[:2]...)
	if status != http.StatusOK {
		t.Fatalf("second run: %d %s", status, out["message"])
	}
	_, got := e.get(t, "/api/v1/clients/1/result?tax_year=2026-27")
	var again struct {
		Results typeResults `json:"results"`
	}
	json.Unmarshal([]byte(got), &again)
	if len(again.Results) != 2 || again.Results[0].Report.Counts["fail"] != purchase["fail"] {
		t.Errorf("after a second sales run: %.300s", got)
	}
	e.assertNoTempFiles(t)
}

// AC-31, AC-32: the rules chosen are checked against the types before anything runs; a
// refusal stores nothing, even for the types of the upload that were fine.
func TestRuleChoiceRefused(t *testing.T) {
	e := newEnv(t)
	e.addDeccan(t)
	base := [][2]string{{"client_id", "1"}, {"tax_year", "2026-27"}, {"include_testing", "true"},
		{"comparison", "gst_sales_vs_gstr1"}, {"comparison", "gst_purchase_vs_gstr2b"}}
	for _, c := range []struct {
		name   string
		fields [][2]string
		status int
		code   string
	}{
		{"rule of another type", [][2]string{{"rules:gst_purchase_vs_gstr2b", "RECON-GST-001"}}, http.StatusUnprocessableEntity, "RULE_NOT_IN_COMPARISON"},
		{"nothing chosen", [][2]string{{"rules:gst_purchase_vs_gstr2b", ""}}, http.StatusUnprocessableEntity, "NO_RULES_SELECTED"},
		{"unknown type", [][2]string{{"comparison", "no_such_type"}}, http.StatusUnprocessableEntity, "COMPARISON_NOT_FOUND"},
		{"rules for a type not run", [][2]string{{"rules:taxsuite", "RECON-DEP-001"}}, http.StatusBadRequest, "BAD_RULES"},
		{"badly written rules", [][2]string{{"rules:gst_sales_vs_gstr1", "RECON-GST-001;rm"}}, http.StatusBadRequest, "BAD_RULES"},
	} {
		t.Run(c.name, func(t *testing.T) {
			status, out := e.postRun(t, append(append([][2]string{}, base...), c.fields...), gstPack(t)...)
			if status != c.status || code(out) != c.code {
				t.Errorf("got %d %s, want %d %s (%s)", status, code(out), c.status, c.code, out["message"])
			}
		})
	}
	if _, list := e.get(t, "/api/v1/clients?tax_year=2026-27"); !strings.Contains(list, `"last_run_on":""`) || !strings.Contains(list, `"documents":0`) {
		t.Errorf("something was stored after the refusals: %s", list)
	}
	e.assertNoTempFiles(t)
}

// Without a client, each type's report comes back in results, with the rules left out.
func TestRunSeveralTypesWithoutClient(t *testing.T) {
	e := newEnv(t)
	fields := [][2]string{{"tax_year", "2025-26"}, {"include_testing", "true"}, {"comparison", "taxsuite"}, {"rules:taxsuite", "RECON-DEP-001"}}
	status, out := e.postRun(t, fields, sample(t, "taxsuite", "comp_v3_2025-26_differs.json"))
	if status != http.StatusOK {
		t.Fatalf("run: %d %s", status, out["message"])
	}
	var res typeResults
	if err := json.Unmarshal(out["results"], &res); err != nil || len(res) != 1 || res[0].ComparisonID != "taxsuite" || len(res[0].Report.NotSelected) != 1 {
		t.Errorf("results: %s (%v)", out["results"], err)
	}
	if len(out["report"]) == 0 || len(out["facts"]) == 0 {
		t.Error("report or facts missing")
	}
}
