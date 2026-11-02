package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
)

// SPEC-006 part 3, AC-13 to AC-15: the Worklist from Test Pack A's two ledger CSVs
// (fictional Nimbus Logic Systems; PAN invented).

const nimbusPAN = "AAACN4321K"

type worklistJSON struct {
	Items []struct {
		ClientID       int64    `json:"client_id"`
		Kind           string   `json:"kind"`
		Key            string   `json:"key"`
		Ledger         string   `json:"ledger"`
		Text           string   `json:"text"`
		PriorTotal     int64    `json:"prior_total"`
		CurrentTotal   int64    `json:"current_total"`
		MissingMonths  []string `json:"missing_months"`
		Times          string   `json:"times"`
		Classification *struct {
			Text string `json:"text"`
			By   string `json:"by"`
			At   string `json:"at"`
		} `json:"classification"`
	} `json:"items"`
}

func readWorklist(t *testing.T, db string) worklistJSON {
	t.Helper()
	out := filepath.Join(t.TempDir(), "worklist.json")
	if r := cmd(t, "worklist", "--db", db, "--tax-year", "2025-26", "--json", out); r.code != exitOK {
		t.Fatalf("worklist: exit %d\n%s", r.code, r.stderr)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var wl worklistJSON
	if err := json.Unmarshal(b, &wl); err != nil {
		t.Fatal(err)
	}
	return wl
}

// ledgers copies the pack's ledgers (both, or the ones named) into a new folder.
func ledgers(t *testing.T, names ...string) string {
	t.Helper()
	if len(names) == 0 {
		names = []string{"07_expense_ledger_FY2024-25.csv", "08_expense_ledger_FY2025-26.csv"}
	}
	dir := t.TempDir()
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(testPack, n))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, n), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestTestPackAWorklist(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(testPack, "score.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var score scoreFile
	if _, err := strictyaml.Decode("score.yaml", data, &score); err != nil {
		t.Fatal(err)
	}
	var want []string // "kind|ledger|missing|times", in the answer key's order
	for _, f := range score.Findings {
		for _, w := range f.Worklist {
			want = append(want, w.Kind+"|"+w.Ledger+"|"+w.Missing+"|"+w.Times)
		}
	}
	if len(want) != 12 {
		t.Fatalf("score.yaml lists %d Worklist items, want 12 (F27 to F30)", len(want))
	}

	db := newDB(t)
	if r := withStdin(t, nimbusPAN, "client-add", "--db", db, "--name", "Nimbus Logic Systems", "--tax-year", "2025-26"); r.code != exitOK {
		t.Fatalf("client-add: exit %d\n%s", r.code, r.stderr)
	}
	input := ledgers(t)
	r := runCmd(t, db, input, "--client", "1")
	if r.code > exitFindings {
		t.Fatalf("run: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if !strings.Contains(r.stdout, "== Worklist: 12 ==") || !strings.Contains(r.stdout,
		"Step change: Travelling expenses - overseas (Travel): Rs 9,83,627 last year, Rs 37,72,925 this year (3.84 times)") {
		t.Errorf("report has no Worklist section:\n%s", r.stdout)
	}

	// AC-13: last year's ledger is kept as last year's document, not refused.
	if got := query(t, db, `SELECT e.tax_year, v.file_name FROM document_versions v JOIN documents d ON d.id = v.document_id
		JOIN engagements e ON e.id = d.engagement_id WHERE d.document_type = 'ledger' ORDER BY e.tax_year`); len(got) != 2 ||
		got[0][0] != "2024-25" || got[0][1] != "07_expense_ledger_FY2024-25.csv" || got[1][0] != "2025-26" {
		t.Errorf("ledgers stored %v", got)
	}

	// AC-14: exactly F27 to F30, and nothing else.
	wl := readWorklist(t, db)
	var got []string
	for _, it := range wl.Items {
		got = append(got, it.Kind+"|"+it.Ledger+"|"+strings.Join(it.MissingMonths, ",")+"|"+it.Times)
		if it.ClientID != 1 || it.Classification != nil {
			t.Errorf("%s: client %d, classification %v", it.Key, it.ClientID, it.Classification)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("Worklist\n got %q\nwant %q", got, want)
	}

	// AC-15: staff classify an item; the classification is kept across runs; the tool
	// never fills one in.
	classify := func(body string) result {
		t.Helper()
		return withStdin(t, body, "classify", "--db", db, "--client", "1", "--tax-year", "2025-26", "--json", filepath.Join(t.TempDir(), "c.json"))
	}
	if r := classify(`{"kind": "new_head", "ledger": "Lead generation charges", "classification": "Clause 21: professional fees; TDS 194J", "by": "Asha"}`); r.code != exitOK {
		t.Fatalf("classify: exit %d\n%s", r.code, r.stderr)
	}
	for body, code := range map[string]string{
		`{"kind": "new_head", "ledger": "Rent", "classification": "x", "by": "Asha"}`:                         "WORKLIST_ITEM_NOT_FOUND",
		`{"kind": "vanished", "ledger": "Lead generation charges", "classification": "x", "by": "Asha"}`:      "WORKLIST_ITEM_NOT_FOUND",
		`{"kind": "new_head", "ledger": "Lead generation charges", "classification": "  ", "by": "Asha"}`:     "BAD_CLASSIFICATION",
		`{"kind": "new_head", "ledger": "Lead generation charges", "classification": "x", "by": ""}`:          "BAD_NAME",
		`{"kind": "odd", "ledger": "Lead generation charges", "classification": "x", "by": "Asha"}`:           "BAD_KIND",
		`{"kind": "new_head", "ledger": "Lead generation charges", "classification": "x", "by": "A", "z": 1}`: "BAD_REQUEST",
	} {
		if r := classify(body); r.code != exitInput || !strings.Contains(r.stderr, code) {
			t.Errorf("%s: exit %d, %s; want %s", body, r.code, r.stderr, code)
		}
	}
	if r := runCmd(t, db, ledgers(t, "08_expense_ledger_FY2025-26.csv"), "--client", "1"); r.code > exitFindings {
		t.Fatalf("second run: exit %d\n%s", r.code, r.stderr)
	}
	wl = readWorklist(t, db)
	classified := 0
	for _, it := range wl.Items {
		if it.Classification == nil {
			continue
		}
		classified++
		if it.Key != "new_head|Lead generation charges" || it.Classification.By != "Asha" || it.Classification.At == "" ||
			it.Classification.Text != "Clause 21: professional fees; TDS 194J" {
			t.Errorf("classification of %s: %+v", it.Key, it.Classification)
		}
	}
	if len(wl.Items) != 12 || classified != 1 {
		t.Errorf("after a second run (this year's ledger only, last year's from the first): %d items, %d classified", len(wl.Items), classified)
	}
	if n := one(t, db, `SELECT COUNT(*) FROM worklist_classifications`); n != "1" {
		t.Errorf("%s classifications stored, want 1", n)
	}
	if _, err := sqlExec(t, db, `DELETE FROM worklist_classifications`); err == nil || !strings.Contains(err.Error(), "kept as written") {
		t.Errorf("a classification was deleted: %v", err)
	}
}

func TestWorklistNeedsBothYears(t *testing.T) {
	// This year's ledger alone: no Worklist section (nothing is new without last year's).
	r := runCmd(t, newDB(t), ledgers(t, "08_expense_ledger_FY2025-26.csv"))
	if r.code > exitFindings || strings.Contains(r.stdout, "== Worklist") {
		t.Errorf("exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	// A ledger two years back is not last year's: refused as before.
	r = cmd(t, "run", "--db", newDB(t), "--input", ledgers(t, "07_expense_ledger_FY2024-25.csv", "08_expense_ledger_FY2025-26.csv"),
		"--tax-year", "2026-27", "--vocab", vocabDir, "--profiles", profDir)
	if !strings.Contains(r.stdout, "the file is for tax year 2024-25; this run is for 2026-27") {
		t.Errorf("exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
}

// The expense ledgers have no rules, so with "include rules not yet approved by the CA" on,
// the report still does not say testing rules ran; a run that does use one says so
// (TestRunClean).
func TestLedgersRunNoTestingNotice(t *testing.T) {
	jsonFile := filepath.Join(t.TempDir(), "report.json")
	r := runCmd(t, newDB(t), ledgers(t), "--include-testing", "--comparison", "ledger_year_on_year",
		"--comparisons", filepath.Join(repo, "content", "comparisons", "comparisons.yaml"), "--json", jsonFile)
	if r.code > exitFindings {
		t.Fatalf("run: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	if strings.Contains(r.stdout, "not yet approved by the CA") {
		t.Errorf("the report says testing rules ran:\n%s", r.stdout)
	}
	b, err := os.ReadFile(jsonFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"includes_testing":true`) {
		t.Error("the JSON report says includes_testing")
	}
}
