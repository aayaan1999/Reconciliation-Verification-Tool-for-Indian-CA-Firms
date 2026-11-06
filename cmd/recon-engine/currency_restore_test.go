package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The rate file is written before the change is committed, and a file cannot be rolled back with the
// database. If the change does not complete, the file goes back to what it held, so a rate is never in
// force without an audit entry for it. Values are illustrative.

// failAfterWrite makes every currency change fail after the new rate file is in place, for one test.
func failAfterWrite(t *testing.T) {
	t.Helper()
	old := afterRateWrite
	afterRateWrite = func() error { return errors.New("the change did not complete") }
	t.Cleanup(func() { afterRateWrite = old })
}

func TestCurrencySetPutsTheOldFileBackWhenTheChangeDoesNotComplete(t *testing.T) {
	db, dir := newDB(t), filepath.Join(t.TempDir(), "currency")
	if r := setRate(t, db, dir, currencyChangeJSON("A. Partner", "83.2500", "2026-04-01", "first")); r.code != exitOK {
		t.Fatalf("first set: exit %d\n%s", r.code, r.stderr)
	}
	file := filepath.Join(dir, "usd-rate.yaml")
	before, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	failAfterWrite(t)
	r := setRate(t, db, dir, currencyChangeJSON("A. Partner", "90.0000", "2026-05-01", "second"))
	if r.code == exitOK || !strings.Contains(r.stderr, "did not complete") {
		t.Fatalf("a change that did not complete: exit %d, stderr %q", r.code, r.stderr)
	}
	after, err := os.ReadFile(file)
	if err != nil || string(after) != string(before) {
		t.Errorf("the rate file changed although the change did not complete:\n%s\n--- was\n%s", after, before)
	}
	if got := getCurrency(t, db, dir); got.USDRate != "83.2500" || got.Note != "first" {
		t.Errorf("rate in force = %q (%q), want the earlier 83.2500", got.USDRate, got.Note)
	}
	if n := auditCount(t, db); n != 1 {
		t.Errorf("%d audit entries, want only the first change's", n)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".usd-rate-*.tmp")); len(left) != 0 {
		t.Errorf("temporary files left behind: %v", left)
	}
}

// With no file before, a change that does not complete leaves no file: there is still no rate.
func TestCurrencySetLeavesNoFileWhenTheFirstChangeDoesNotComplete(t *testing.T) {
	db, dir := newDB(t), filepath.Join(t.TempDir(), "currency")
	failAfterWrite(t)
	if r := setRate(t, db, dir, currencyChangeJSON("A. Partner", "83.2500", "2026-04-01", "")); r.code == exitOK {
		t.Fatal("a change that did not complete reported success")
	}
	if _, err := os.Stat(filepath.Join(dir, "usd-rate.yaml")); !os.IsNotExist(err) {
		t.Errorf("a rate file exists although no change was made: %v", err)
	}
	if got := getCurrency(t, db, dir); got.Set {
		t.Errorf("a rate is in force: %+v", got)
	}
	if n := auditCount(t, db); n != 0 {
		t.Errorf("%d audit entries, want none", n)
	}
}

// A rate file that cannot be read is not replaced: its text could not be put back.
func TestCurrencySetRefusesToReplaceAFileItCannotRead(t *testing.T) {
	db, dir := newDB(t), t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "usd-rate.yaml"), 0o755); err != nil { // a folder where the file should be
		t.Fatal(err)
	}
	r := setRate(t, db, dir, currencyChangeJSON("A. Partner", "83.2500", "2026-04-01", ""))
	if r.code == exitOK || !strings.Contains(r.stderr, "cannot be read") {
		t.Errorf("exit %d, stderr %q; want a refusal saying the file cannot be read", r.code, r.stderr)
	}
	if n := auditCount(t, db); n != 0 {
		t.Errorf("%d audit entries, want none", n)
	}
}

// The note on the Currency page is the one the loader decoded, however the file quotes it.
func TestCurrencyGetShowsTheNoteWhicheverWayItIsQuoted(t *testing.T) {
	head := "schema: \"recon.currency.usd/1\"\nusd_rate: \"83.2500\"\neffective_from: \"2026-04-01\"\n"
	for name, tc := range map[string]struct{ line, want string }{
		"double quotes":      {`note: "from the bank"`, "from the bank"},
		"single quotes":      {`note: 'from the bank'`, "from the bank"},
		"comment after":      {`note: "from the bank"   # checked on Monday`, "from the bank"},
		"escaped characters": {`note: "card rate é \"x\""`, "card rate é \"x\""},
		"no note":            {`note: ""`, ""},
	} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "usd-rate.yaml"), []byte(head+tc.line+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := getCurrency(t, newDB(t), dir); got.Note != tc.want || got.USDRate != "83.2500" {
			t.Errorf("%s: note %q rate %q, want %q", name, got.Note, got.USDRate, tc.want)
		}
	}
}
