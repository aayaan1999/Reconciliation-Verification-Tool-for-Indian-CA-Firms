package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SPEC-021: the Currency page's engine commands. All values are illustrative.

func currencyChangeJSON(by, rate, date, note string) string {
	b, _ := json.Marshal(map[string]string{"by": by, "usd_rate": rate, "effective_from": date, "note": note})
	return string(b)
}

func setRate(t *testing.T, db, dir, body string) result {
	t.Helper()
	return withStdin(t, body, "currency-set", "--db", db, "--currency", dir, "--json", filepath.Join(t.TempDir(), "out.json"))
}

func getCurrency(t *testing.T, db, dir string) currencyOut {
	t.Helper()
	out := filepath.Join(t.TempDir(), "get.json")
	if r := cmd(t, "currency-get", "--db", db, "--currency", dir, "--json", out); r.code != exitOK {
		t.Fatalf("currency-get: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var v currencyOut
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func auditCount(t *testing.T, db string) int {
	t.Helper()
	rows := query(t, db, `SELECT COUNT(*) FROM audit_log WHERE action = 'currency.rate_set'`)
	n := 0
	for _, c := range rows[0][0] {
		n = n*10 + int(c-'0')
	}
	return n
}

// AC-1, AC-3: setting writes the file and one audit entry; getting returns it with the history.
func TestCurrencySetAndGet(t *testing.T) {
	db := newDB(t)
	dir := filepath.Join(t.TempDir(), "currency") // the folder does not exist yet

	v := getCurrency(t, db, dir)
	if v.Set || v.USDRate != "" || v.Problem != "dollar rate not set" || len(v.History) != 0 {
		t.Errorf("before any rate: %+v", v)
	}
	if r := setRate(t, db, dir, currencyChangeJSON("Priya Partner", "83.25", "2026-11-04", "bank rate, morning")); r.code != exitOK {
		t.Fatalf("set: exit %d\n%s%s", r.code, r.stdout, r.stderr)
	}
	v = getCurrency(t, db, dir)
	if !v.Set || v.USDRate != "83.2500" || v.EffectiveFrom != "2026-11-04" || v.Note != "bank rate, morning" || v.Problem != "" {
		t.Errorf("after the first rate: %+v", v)
	}
	if r := setRate(t, db, dir, currencyChangeJSON("Priya Partner", "84.1", "2026-11-05", "")); r.code != exitOK {
		t.Fatalf("second set: exit %d\n%s", r.code, r.stderr)
	}
	v = getCurrency(t, db, dir)
	if v.USDRate != "84.1000" || len(v.History) != 2 {
		t.Fatalf("after the second rate: %+v", v)
	}
	// Newest first, with the rate it replaced.
	if h := v.History[0]; h.USDRate != "84.1000" || h.Previous != "83.2500" || h.By != "Priya Partner" || h.EffectiveFrom != "2026-11-05" || h.At == "" {
		t.Errorf("newest entry: %+v", h)
	}
	if h := v.History[1]; h.USDRate != "83.2500" || h.Previous != "" || h.Note != "bank rate, morning" {
		t.Errorf("oldest entry: %+v", h)
	}
	if n := auditCount(t, db); n != 2 {
		t.Errorf("%d audit entries, want 2", n)
	}
	// The file is the one a run reads, in the quoted form the file's rules ask for.
	b, err := os.ReadFile(filepath.Join(dir, "usd-rate.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`schema: "recon.currency.usd/1"`, `usd_rate: "84.1000"`, `effective_from: "2026-11-05"`, `note: ""`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("file lacks %s:\n%s", want, b)
		}
	}
	leftovers, _ := filepath.Glob(filepath.Join(dir, ".usd-rate-*.tmp"))
	if len(leftovers) != 0 {
		t.Errorf("temporary files left: %v", leftovers)
	}
	// A note with quotes and a backslash is stored and read back as written.
	if r := setRate(t, db, dir, currencyChangeJSON("Priya Partner", "85", "2026-11-06", `RBI "reference" \ rate`)); r.code != exitOK {
		t.Fatalf("quoted note: exit %d\n%s", r.code, r.stderr)
	}
	if v = getCurrency(t, db, dir); v.Note != `RBI "reference" \ rate` || v.USDRate != "85.0000" {
		t.Errorf("quoted note read back as %+v", v)
	}
}

// AC-2: an invalid change is refused naming the field, and nothing is written or audited.
func TestCurrencySetRefuses(t *testing.T) {
	db := newDB(t)
	dir := filepath.Join(t.TempDir(), "currency")
	if r := setRate(t, db, dir, currencyChangeJSON("Priya Partner", "83.25", "2026-11-04", "")); r.code != exitOK {
		t.Fatal(r.stderr)
	}
	before, _ := os.ReadFile(filepath.Join(dir, "usd-rate.yaml"))
	long := strings.Repeat("x", 201)
	cases := []struct{ name, body, code string }{
		{"empty rate", currencyChangeJSON("A", "", "2026-11-04", ""), "BAD_RATE"},
		{"not a number", currencyChangeJSON("A", "eighty", "2026-11-04", ""), "BAD_RATE"},
		{"zero", currencyChangeJSON("A", "0", "2026-11-04", ""), "BAD_RATE"},
		{"negative", currencyChangeJSON("A", "-83", "2026-11-04", ""), "BAD_RATE"},
		{"five decimals", currencyChangeJSON("A", "83.12345", "2026-11-04", ""), "BAD_RATE"},
		{"a dollar sign", currencyChangeJSON("A", "$83", "2026-11-04", ""), "BAD_RATE"},
		{"below 1", currencyChangeJSON("A", "0.8325", "2026-11-04", ""), "BAD_RATE"},
		{"a slip of 8325", currencyChangeJSON("A", "8325", "2026-11-04", ""), "BAD_RATE"},
		{"exactly 1000", currencyChangeJSON("A", "1000", "2026-11-04", ""), "BAD_RATE"},
		{"no date", currencyChangeJSON("A", "83", "", ""), "BAD_DATE"},
		{"not a real date", currencyChangeJSON("A", "83", "2026-02-30", ""), "BAD_DATE"},
		{"date written dd-mm-yyyy", currencyChangeJSON("A", "83", "04-11-2026", ""), "BAD_DATE"},
		{"line break in the note", currencyChangeJSON("A", "83", "2026-11-04", "one\ntwo"), "BAD_NOTE"},
		{"long note", currencyChangeJSON("A", "83", "2026-11-04", long), "BAD_NOTE"},
		{"no actor", currencyChangeJSON("", "83", "2026-11-04", ""), "BAD_NAME"},
		{"unknown key", `{"by":"A","usd_rate":"83","effective_from":"2026-11-04","note":"","extra":1}`, "BAD_REQUEST"},
		{"not JSON", `83.25`, "BAD_REQUEST"},
	}
	for _, c := range cases {
		r := setRate(t, db, dir, c.body)
		if r.code != exitInput || !strings.Contains(r.stderr, c.code) {
			t.Errorf("%s: exit %d, %q; want exit %d and %s", c.name, r.code, strings.TrimSpace(r.stderr), exitInput, c.code)
		}
	}
	// Nothing changed: the same file, one audit entry.
	after, _ := os.ReadFile(filepath.Join(dir, "usd-rate.yaml"))
	if string(after) != string(before) {
		t.Errorf("the file changed after refused changes:\n%s", after)
	}
	if n := auditCount(t, db); n != 1 {
		t.Errorf("%d audit entries after refused changes, want 1", n)
	}
	// The edges that are accepted: 1 and 999.9999.
	for _, rate := range []string{"1", "999.9999"} {
		if r := setRate(t, db, dir, currencyChangeJSON("A", rate, "2026-11-04", "")); r.code != exitOK {
			t.Errorf("%s: exit %d %s", rate, r.code, r.stderr)
		}
	}
}

// AC-3: a file that is not a valid rate file is reported, not hidden, and a new rate replaces it.
func TestCurrencyGetInvalidFile(t *testing.T) {
	db := newDB(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "usd-rate.yaml"), []byte("usd_rate: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	v := getCurrency(t, db, dir)
	if v.Set || v.Problem != "dollar rate not valid" || v.Detail == "" {
		t.Errorf("invalid file: %+v", v)
	}
	if r := setRate(t, db, dir, currencyChangeJSON("A", "83.25", "2026-11-04", "")); r.code != exitOK {
		t.Fatal(r.stderr)
	}
	if v = getCurrency(t, db, dir); !v.Set || v.Problem != "" {
		t.Errorf("after setting: %+v", v)
	}
}

// AC-4: a run after a change converts at the new rate, with no restart, and stores the new rate.
func TestCurrencyChangeAppliesToNextRun(t *testing.T) {
	db := newDB(t)
	dir := filepath.Join(t.TempDir(), "currency")
	in := dollarInput(t, "bank_book.csv", "bank_statement.csv")
	if r := setRate(t, db, dir, currencyChangeJSON("Priya Partner", "83.25", "2026-11-04", "")); r.code != exitOK {
		t.Fatal(r.stderr)
	}
	if got := bnk010(runDollar(t, db, in, dir)); !eq(got, map[string]string{"R1": "pass", "R2": "pass", "R3": "pass"}) {
		t.Fatalf("at 83.25: %v", got)
	}
	if r := setRate(t, db, dir, currencyChangeJSON("Priya Partner", "84", "2026-11-05", "")); r.code != exitOK {
		t.Fatal(r.stderr)
	}
	if got := bnk010(runDollar(t, db, in, dir)); !eq(got, map[string]string{"R1": "fail:AMOUNT_DIFF", "R2": "pass", "R3": "fail:AMOUNT_DIFF"}) {
		t.Errorf("at 84: %v", got)
	}
	rows := query(t, db, `SELECT usd_rate, usd_rate_effective FROM runs ORDER BY id`)
	if len(rows) != 2 || rows[0][0] != "83.2500" || rows[1][0] != "84.0000" || rows[1][1] != "2026-11-05" {
		t.Errorf("stored rates: %v", rows)
	}
}
