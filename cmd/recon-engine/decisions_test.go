package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SPEC-006 part 3, AC-11 and AC-12: decisions on findings. Figures are the synthetic
// tax-suite sample's (testdata/run/taxsuite; PAN invented).

type resultJSON struct {
	Results []struct {
		Report struct {
			Findings []struct {
				Findings []struct {
					FindingID        string `json:"finding_id"`
					RuleID           string `json:"rule_id"`
					ValueFingerprint string `json:"value_fingerprint"`
				} `json:"findings"`
			} `json:"findings"`
		} `json:"report"`
	} `json:"results"`
	Decisions map[string]struct {
		State   string `json:"state"`
		Current struct {
			Decision      string `json:"decision"`
			Note          string `json:"note"`
			DecidedBy     string `json:"decided_by"`
			DecidedAt     string `json:"decided_at"`
			OnOtherValues bool   `json:"on_other_values"`
		} `json:"current"`
		History []json.RawMessage `json:"history"`
	} `json:"decisions"`
}

func readResult(t *testing.T, db string) resultJSON {
	t.Helper()
	out := filepath.Join(t.TempDir(), "result.json")
	if r := cmd(t, "result", "--db", db, "--client", "1", "--tax-year", "2025-26", "--json", out); r.code != exitOK {
		t.Fatalf("result: exit %d\n%s", r.code, r.stderr)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var res resultJSON
	if err := json.Unmarshal(b, &res); err != nil {
		t.Fatal(err)
	}
	return res
}

func TestFindingDecisions(t *testing.T) {
	db := newDB(t)
	if r := withStdin(t, "AAKFS5678M", "client-add", "--db", db, "--name", "Sample Firm", "--tax-year", "2025-26"); r.code != exitOK {
		t.Fatalf("client-add: exit %d\n%s", r.code, r.stderr)
	}
	input := t.TempDir()
	copyTree(t, filepath.Join(runData, "taxsuite"), input)
	runFor := func() {
		t.Helper()
		if r := runCmd(t, db, input, "--include-testing", "--client", "1"); r.code != exitFindings {
			t.Fatalf("run: exit %d\n%s", r.code, r.stderr)
		}
	}
	runFor()
	res := readResult(t, db)
	var id string
	for _, g := range res.Results[0].Report.Findings {
		for _, f := range g.Findings {
			if f.RuleID == "RECON-DEP-001" {
				id = f.FindingID
			}
		}
	}
	if len(id) != 16 || len(res.Decisions) != 0 {
		t.Fatalf("finding id %q, decisions %v", id, res.Decisions)
	}
	decideFor := func(body string) result {
		t.Helper()
		return withStdin(t, body, "decide", "--db", db, "--client", "1", "--tax-year", "2025-26", "--json", filepath.Join(t.TempDir(), "d.json"))
	}

	// Refused: an unknown decision, no name, Not an issue without a note, an unknown finding.
	for body, code := range map[string]string{
		`{"finding_id": "` + id + `", "decision": "ignore", "by": "A. Staff"}`:       "BAD_DECISION",
		`{"finding_id": "` + id + `", "decision": "fix", "by": " "}`:                 "BAD_NAME",
		`{"finding_id": "` + id + `", "decision": "not_an_issue", "by": "A. Staff"}`: "NOTE_REQUIRED",
		`{"finding_id": "AAAAAAAAAAAAAAAA", "decision": "fix", "by": "A. Staff"}`:    "FINDING_NOT_FOUND",
	} {
		if r := decideFor(body); r.code != exitInput || !strings.Contains(r.stderr, code) {
			t.Errorf("%s: exit %d, want %s\n%s", body, r.code, code, r.stderr)
		}
	}

	if r := decideFor(`{"finding_id": "` + id + `", "decision": "ask_client", "note": "Which schedule is final?", "by": "A. Staff"}`); r.code != exitOK {
		t.Fatalf("decide: exit %d\n%s", r.code, r.stderr)
	}
	if r := decideFor(`{"finding_id": "` + id + `", "decision": "fix", "note": "Computation to be corrected", "by": "B. Manager"}`); r.code != exitOK {
		t.Fatalf("decide: exit %d\n%s", r.code, r.stderr)
	}
	d := readResult(t, db).Decisions[id]
	if d.State != "fix" || d.Current.DecidedBy != "B. Manager" || d.Current.Note != "Computation to be corrected" || d.Current.DecidedAt == "" || len(d.History) != 2 || d.Current.OnOtherValues {
		t.Errorf("after two decisions: %+v", d)
	}

	// The same values again: the decision stays.
	runFor()
	if d := readResult(t, db).Decisions[id]; d.State != "fix" {
		t.Errorf("after a run with the same values: state %q", d.State)
	}

	// The values change: the finding is open again, the decision shown as made on other values.
	file := filepath.Join(input, "comp_v3_2025-26_differs.json")
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(b), `"allowed_depreciation": 98760.00`, `"allowed_depreciation": 98761.00`, 1)
	if changed == string(b) {
		t.Fatal("the sample no longer has the figure this test changes")
	}
	if err := os.WriteFile(file, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	runFor()
	d = readResult(t, db).Decisions[id]
	if d.State != "open" || !d.Current.OnOtherValues || len(d.History) != 2 {
		t.Errorf("after the values changed: %+v", d)
	}

	// Decisions are kept as written: the database refuses to change one.
	if _, err := sqlExec(t, db, `UPDATE finding_decisions SET decision = 'fix'`); err == nil || !strings.Contains(err.Error(), "kept as written") {
		t.Errorf("update of a decision: %v", err)
	}
}

// sqlExec runs one statement on the database directly, as another program could.
func sqlExec(t *testing.T, db, stmt string) (sql.Result, error) {
	t.Helper()
	conn, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(db))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	return conn.Exec(stmt)
}
