package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SPEC-009: every run of a type of check is kept as Comparison 1, 2, 3…, and an earlier one
// opens exactly as it was reported, with the drafts it read. Synthetic tax-suite sample.

type historyJSON struct {
	Comparisons []struct {
		No     int  `json:"no"`
		Kept   bool `json:"kept"`
		Counts *struct {
			Fail int `json:"fail"`
		} `json:"counts"`
		Files []struct {
			FileName string `json:"file_name"`
			DraftNo  int    `json:"draft_no"`
		} `json:"files"`
	} `json:"comparisons"`
	Report json.RawMessage `json:"report"`
	Facts  struct {
		Engagements []struct {
			Documents []struct {
				DraftNo int `json:"draft_no"`
			} `json:"documents"`
		} `json:"engagements"`
	} `json:"facts"`
}

func TestComparisonHistory(t *testing.T) {
	db := newDB(t)
	if r := withStdin(t, "AAKFS5678M", "client-add", "--db", db, "--name", "Sample Firm", "--tax-year", "2025-26"); r.code != exitOK {
		t.Fatalf("client-add: exit %d\n%s", r.code, r.stderr)
	}
	input := t.TempDir()
	copyTree(t, filepath.Join(runData, "taxsuite"), input)
	runType := func(comparison string) {
		t.Helper()
		if r := runCmd(t, db, input, "--include-testing", "--client", "1", "--comparison", comparison, "--comparisons", comparisonsYAML); r.code != exitOK && r.code != exitFindings {
			t.Fatalf("run %s: exit %d\n%s", comparison, r.code, r.stderr)
		}
	}
	historyOf := func(extra ...string) (historyJSON, result) {
		t.Helper()
		out := filepath.Join(t.TempDir(), "history.json")
		r := cmd(t, append([]string{"history", "--db", db, "--client", "1", "--tax-year", "2025-26", "--comparison", "taxsuite", "--json", out}, extra...)...)
		var h historyJSON
		if r.code == exitOK {
			b, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(b, &h); err != nil {
				t.Fatal(err)
			}
		}
		return h, r
	}

	runType("taxsuite")
	runType("accounts_vs_computation") // another type in between keeps its own numbers
	// The client sends the computation again: a new draft.
	file := filepath.Join(input, "comp_v3_2025-26_differs.json")
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, append(b, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	runType("taxsuite")

	h, r := historyOf()
	if r.code != exitOK {
		t.Fatalf("history: exit %d\n%s", r.code, r.stderr)
	}
	if len(h.Comparisons) != 2 {
		t.Fatalf("got %d comparisons, want 2", len(h.Comparisons))
	}
	for i, c := range h.Comparisons {
		if c.No != i+1 || !c.Kept || c.Counts == nil || c.Counts.Fail == 0 || len(c.Files) != 1 || c.Files[0].DraftNo != i+1 {
			t.Errorf("comparison %d: %+v", i+1, c)
		}
	}

	// Comparison 1 opens as it was reported, with the draft it read.
	h, r = historyOf("--no", "1")
	if r.code != exitOK {
		t.Fatalf("history --no 1: exit %d\n%s", r.code, r.stderr)
	}
	stored := query(t, db, `SELECT report_json FROM runs WHERE comparison_id = 'taxsuite' ORDER BY id LIMIT 1`)[0][0]
	var got, want bytes.Buffer
	if err := json.Compact(&got, h.Report); err != nil {
		t.Fatal(err)
	}
	if err := json.Compact(&want, []byte(stored)); err != nil {
		t.Fatal(err)
	}
	if got.String() != want.String() {
		t.Error("comparison 1's report differs from the one stored by its run")
	}
	if docs := h.Facts.Engagements[0].Documents; len(docs) != 1 || docs[0].DraftNo != 1 {
		t.Errorf("comparison 1 read %+v, want draft 1", docs)
	}

	// No comparison 3; and a run whose report was not kept is listed but cannot be opened.
	if _, r := historyOf("--no", "3"); r.code != exitInput || !strings.Contains(r.stderr, "COMPARISON_NOT_FOUND") {
		t.Errorf("--no 3: exit %d\n%s", r.code, r.stderr)
	}
	query(t, db, `UPDATE runs SET report_json = '' WHERE id = (SELECT MIN(id) FROM runs WHERE comparison_id = 'taxsuite')`)
	if h, _ := historyOf(); h.Comparisons[0].Kept || h.Comparisons[0].Counts != nil {
		t.Errorf("comparison 1 without its report: %+v", h.Comparisons[0])
	}
	if _, r := historyOf("--no", "1"); r.code != exitInput || !strings.Contains(r.stderr, "COMPARISON_NOT_FOUND") {
		t.Errorf("--no 1 without its report: exit %d\n%s", r.code, r.stderr)
	}
}
