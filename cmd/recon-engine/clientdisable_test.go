package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SPEC-004 part 10: disabling a client and enabling it again. Figures are the synthetic
// tax-suite sample's (testdata/run/taxsuite; PAN invented).

type listedClient struct {
	ID             int64  `json:"id"`
	Status         string `json:"status"`
	DisabledOn     string `json:"disabled_on"`
	DisabledBy     string `json:"disabled_by"`
	DisabledReason string `json:"disabled_reason"`
}

func listClients(t *testing.T, db string, extra ...string) []listedClient {
	t.Helper()
	out := filepath.Join(t.TempDir(), "clients.json")
	args := append([]string{"clients", "--db", db, "--tax-year", "2025-26", "--json", out}, extra...)
	if r := cmd(t, args...); r.code != exitOK {
		t.Fatalf("clients: exit %d\n%s", r.code, r.stderr)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var list struct {
		Clients []listedClient `json:"clients"`
	}
	if err := json.Unmarshal(b, &list); err != nil {
		t.Fatal(err)
	}
	return list.Clients
}

// AC-36 to AC-39 (engine part).
func TestClientDisableAndEnable(t *testing.T) {
	db := newDB(t)
	if r := withStdin(t, "AAKFS5678M", "client-add", "--db", db, "--name", "Sample Firm", "--tax-year", "2025-26"); r.code != exitOK {
		t.Fatalf("client-add: exit %d\n%s", r.code, r.stderr)
	}
	input := t.TempDir()
	copyTree(t, filepath.Join(runData, "taxsuite"), input)
	if r := runCmd(t, db, input, "--include-testing", "--client", "1"); r.code != exitFindings {
		t.Fatalf("run: exit %d\n%s", r.code, r.stderr)
	}
	before := readResult(t, db)

	// A reason is required, and an unknown client is refused.
	for _, c := range []struct{ stdin, client, code string }{
		{`{"by":"A. Partner","reason":"  "}`, "1", "REASON_REQUIRED"},
		{`{"by":"","reason":"left the firm"}`, "1", "BAD_NAME"},
		{`{"by":"A. Partner","reason":"left the firm"}`, "9", "CLIENT_NOT_FOUND"},
	} {
		r := withStdin(t, c.stdin, "client-disable", "--db", db, "--client", c.client)
		if r.code != exitInput || !strings.Contains(r.stderr, c.code) {
			t.Errorf("client-disable %s: exit %d, want %s\n%s", c.stdin, r.code, c.code, r.stderr)
		}
	}
	if r := withStdin(t, `{"by":"A. Partner","reason":"is in use"}`, "client-enable", "--db", db, "--client", "1"); r.code != exitInput || !strings.Contains(r.stderr, "NOT_DISABLED") {
		t.Errorf("enable of an active client: exit %d\n%s", r.code, r.stderr)
	}

	if r := withStdin(t, `{"by":"A. Partner","reason":"no longer a client of the firm"}`, "client-disable", "--db", db, "--client", "1"); r.code != exitOK {
		t.Fatalf("client-disable: exit %d\n%s", r.code, r.stderr)
	}
	if r := withStdin(t, `{"by":"A. Partner","reason":"again"}`, "client-disable", "--db", db, "--client", "1"); r.code != exitInput || !strings.Contains(r.stderr, "ALREADY_DISABLED") {
		t.Errorf("second disable: exit %d\n%s", r.code, r.stderr)
	}

	// Off the list, unless --all; then marked disabled with who, when and why.
	if got := listClients(t, db); len(got) != 0 {
		t.Errorf("clients lists a disabled client: %+v", got)
	}
	all := listClients(t, db, "--all")
	if len(all) != 1 || all[0].Status != "disabled" || all[0].DisabledBy != "A. Partner" ||
		all[0].DisabledReason != "no longer a client of the firm" || len(all[0].DisabledOn) != 10 {
		t.Errorf("clients --all = %+v", all)
	}

	// Still readable, exactly as it was.
	if got := readResult(t, db); len(got.Results) != len(before.Results) {
		t.Errorf("result of a disabled client has %d results, want %d", len(got.Results), len(before.Results))
	}

	// Every change is refused, and nothing is written.
	countRuns := func() int {
		t.Helper()
		sdb, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(db))
		if err != nil {
			t.Fatal(err)
		}
		defer sdb.Close()
		var n int
		if err := sdb.QueryRow(`SELECT COUNT(*) FROM runs`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	runs := countRuns()
	if r := runCmd(t, db, input, "--include-testing", "--client", "1"); r.code != exitInput || !strings.Contains(r.stderr, "CLIENT_DISABLED") {
		t.Errorf("run for a disabled client: exit %d\n%s", r.code, r.stderr)
	}
	if n := countRuns(); n != runs {
		t.Errorf("a refused run wrote %d runs", n-runs)
	}
	if r := withStdin(t, `{"legal_name":"Sample Firm","pan":""}`, "client-update", "--db", db, "--client", "1"); r.code != exitInput || !strings.Contains(r.stderr, "CLIENT_DISABLED") {
		t.Errorf("client-update of a disabled client: exit %d\n%s", r.code, r.stderr)
	}
	id := before.Results[0].Report.Findings[0].Findings[0].FindingID
	if r := withStdin(t, `{"finding_id":"`+id+`","decision":"fix","note":"","by":"A. Staff"}`, "decide", "--db", db, "--client", "1", "--tax-year", "2025-26", "--json", filepath.Join(t.TempDir(), "d.json")); r.code != exitInput || !strings.Contains(r.stderr, "CLIENT_DISABLED") {
		t.Errorf("decide for a disabled client: exit %d\n%s", r.code, r.stderr)
	}
	if r := withStdin(t, `{"kind":"new_head","ledger":"Rent","classification":"expense","by":"A. Staff"}`, "classify", "--db", db, "--client", "1", "--tax-year", "2025-26", "--json", filepath.Join(t.TempDir(), "c.json")); r.code != exitInput || !strings.Contains(r.stderr, "CLIENT_DISABLED") {
		t.Errorf("classify for a disabled client: exit %d\n%s", r.code, r.stderr)
	}

	// The PAN stays taken.
	if r := withStdin(t, "AAKFS5678M", "client-add", "--db", db, "--name", "Sample Firm again", "--tax-year", "2025-26"); r.code != exitInput ||
		!strings.Contains(r.stderr, "CLIENT_EXISTS") || !strings.Contains(r.stderr, "disabled") {
		t.Errorf("client-add with a disabled client's PAN: exit %d\n%s", r.code, r.stderr)
	}

	// Enabled again, it is back with everything it had.
	if r := withStdin(t, `{"by":"A. Partner","reason":"engaged again"}`, "client-enable", "--db", db, "--client", "1"); r.code != exitOK {
		t.Fatalf("client-enable: exit %d\n%s", r.code, r.stderr)
	}
	if got := listClients(t, db); len(got) != 1 || got[0].Status != "active" || got[0].DisabledReason != "" {
		t.Errorf("clients after enable = %+v", got)
	}
	after := readResult(t, db)
	if len(after.Results) != len(before.Results) || after.Results[0].Report.Findings[0].Findings[0].FindingID != id {
		t.Error("the client's results changed across disable and enable")
	}
	if r := runCmd(t, db, input, "--include-testing", "--client", "1"); r.code != exitFindings {
		t.Errorf("run after enable: exit %d\n%s", r.code, r.stderr)
	}

	// One audit entry each, with the reason.
	sdb, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(db))
	if err != nil {
		t.Fatal(err)
	}
	defer sdb.Close()
	rows, err := sdb.Query(`SELECT action, actor, object_id, details_json FROM audit_log WHERE object_type = 'client' ORDER BY seq`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var action, actor, obj, details string
		if err := rows.Scan(&action, &actor, &obj, &details); err != nil {
			t.Fatal(err)
		}
		got = append(got, action+" "+actor+" "+obj+" "+details)
	}
	want := []string{
		`client.disable A. Partner 1 {"reason":"no longer a client of the firm"}`,
		`client.enable A. Partner 1 {"reason":"engaged again"}`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("audit entries:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	// A client is never deleted, not even by hand in the database.
	if _, err := sdb.Exec(`DELETE FROM clients WHERE id = 1`); err == nil {
		t.Error("a client row was deleted; the trigger should refuse it")
	}
}

// AC-37 (Worklist part): a disabled client's Worklist items leave the Worklist and cannot be
// classified; enabled again, they come back with their classifications. Test Pack A ledgers
// (fictional Nimbus Logic Systems).
func TestDisabledClientLeavesTheWorklist(t *testing.T) {
	db := newDB(t)
	if r := withStdin(t, nimbusPAN, "client-add", "--db", db, "--name", "Nimbus Logic Systems", "--tax-year", "2025-26"); r.code != exitOK {
		t.Fatalf("client-add: exit %d\n%s", r.code, r.stderr)
	}
	if r := runCmd(t, db, ledgers(t), "--client", "1"); r.code > exitFindings {
		t.Fatalf("run: exit %d\n%s", r.code, r.stderr)
	}
	classify := func() result {
		t.Helper()
		return withStdin(t, `{"kind": "new_head", "ledger": "Lead generation charges", "classification": "Clause 21: professional fees", "by": "Asha"}`,
			"classify", "--db", db, "--client", "1", "--tax-year", "2025-26", "--json", filepath.Join(t.TempDir(), "c.json"))
	}
	if r := classify(); r.code != exitOK {
		t.Fatalf("classify: exit %d\n%s", r.code, r.stderr)
	}
	if n := len(readWorklist(t, db).Items); n != 12 {
		t.Fatalf("Worklist has %d items, want 12", n)
	}
	if r := withStdin(t, `{"by":"A. Partner","reason":"engagement ended"}`, "client-disable", "--db", db, "--client", "1"); r.code != exitOK {
		t.Fatalf("client-disable: exit %d\n%s", r.code, r.stderr)
	}
	if n := len(readWorklist(t, db).Items); n != 0 {
		t.Errorf("Worklist of a disabled client has %d items, want 0", n)
	}
	if r := classify(); r.code != exitInput || !strings.Contains(r.stderr, "CLIENT_DISABLED") {
		t.Errorf("classify for a disabled client: exit %d\n%s", r.code, r.stderr)
	}
	if r := withStdin(t, `{"by":"A. Partner","reason":"engaged again"}`, "client-enable", "--db", db, "--client", "1"); r.code != exitOK {
		t.Fatalf("client-enable: exit %d\n%s", r.code, r.stderr)
	}
	wl := readWorklist(t, db)
	classified := 0
	for _, it := range wl.Items {
		if it.Classification != nil {
			classified++
		}
	}
	if len(wl.Items) != 12 || classified != 1 {
		t.Errorf("after enable: %d items, %d classified; want 12 and 1", len(wl.Items), classified)
	}
}
