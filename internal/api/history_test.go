package api

import (
	"net/http"
	"strings"
	"testing"
)

// SPEC-009: a client's comparisons through the server, and the refusals. Synthetic
// tax-suite sample.
func TestComparisonHistoryThroughServer(t *testing.T) {
	e := newEnv(t)
	if status, _ := e.addClient(t, "Sample Firm", "AAKFS5678M"); status != http.StatusCreated {
		t.Fatalf("add client: %d", status)
	}
	for range 2 {
		if status, out := e.postFor(t, "1", "2025-26", true, sample(t, "taxsuite", "comp_v3_2025-26_differs.json")); status != http.StatusOK {
			t.Fatalf("run: %d %s", status, out["message"])
		}
	}
	if status, got := e.get(t, "/api/v1/clients/1/comparisons?tax_year=2025-26&comparison="); status != http.StatusOK ||
		!strings.Contains(got, `"no":1`) || !strings.Contains(got, `"no":2`) || strings.Contains(got, `"report"`) {
		t.Errorf("list: %d %.300s", status, got)
	}
	if status, got := e.get(t, "/api/v1/clients/1/comparisons?tax_year=2025-26&comparison=&no=1"); status != http.StatusOK ||
		!strings.Contains(got, `"report"`) || !strings.Contains(got, `"facts"`) {
		t.Errorf("open 1: %d %.300s", status, got)
	}
	for url, want := range map[string]int{
		"/api/v1/clients/1/comparisons?tax_year=2025-26&comparison=&no=3":   http.StatusNotFound,
		"/api/v1/clients/99/comparisons?tax_year=2025-26&comparison=":       http.StatusNotFound,
		"/api/v1/clients/1/comparisons?tax_year=2025&comparison=":           http.StatusBadRequest,
		"/api/v1/clients/1/comparisons?tax_year=2025-26&comparison=../x":    http.StatusBadRequest,
		"/api/v1/clients/1/comparisons?tax_year=2025-26&comparison=&no=-1":  http.StatusBadRequest,
		"/api/v1/clients/1/comparisons?tax_year=2025-26&comparison=&no=one": http.StatusBadRequest,
	} {
		if status, _ := e.get(t, url); status != want {
			t.Errorf("%s: %d, want %d", url, status, want)
		}
	}
	e.assertNoTempFiles(t)
}
