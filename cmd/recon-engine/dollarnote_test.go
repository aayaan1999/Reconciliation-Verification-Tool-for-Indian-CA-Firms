package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// A run that meets an amount in US dollars with no usable rate says where the rate was looked for, so a rate file the
// run could not find (for example --currency left at its default from another folder) is not mistaken for a problem
// in the data. A run with a usable rate, or with no dollar amount, has no such note. Nothing here changes a result.
func TestRunNamesTheRateFileWhenDollarAmountsCannotBeRead(t *testing.T) {
	dollars := dollarInput(t, "bank_book.csv", "bank_statement.csv")
	run := func(in, currencyDir string) string {
		args := []string{"run", "--db", newDB(t), "--input", in, "--tax-year", "2026-27", "--vocab", vocabDir, "--profiles", profDir,
			"--include-testing", "--comparison", "bank_book_vs_statement", "--comparisons", filepath.Join(repo, "content", "comparisons", "comparisons.yaml"),
			"--currency", currencyDir}
		r := cmd(t, args...)
		if r.code != exitOK && r.code != exitFindings {
			t.Fatalf("run: exit %d\n%s%s", r.code, r.stdout, r.stderr)
		}
		return r.stdout
	}

	none := rateDir(t, "") // a folder with no rate file
	out := run(dollars, none)
	for _, want := range []string{"== Dollar rate ==", "dollar rate not set", "usd-rate.yaml is missing", filepath.ToSlash(filepath.Join(none, "usd-rate.yaml")), "--currency"} {
		if !strings.Contains(out, want) {
			t.Errorf("no rate: the note lacks %q:\n%s", want, out)
		}
	}
	bad := rateDir(t, rateFileHead+"usd_rate: \"eighty\"\n")
	if out := run(dollars, bad); !strings.Contains(out, "== Dollar rate ==") || !strings.Contains(out, "dollar rate not valid") {
		t.Errorf("a rate that is not valid: no note naming the reason:\n%s", out)
	}
	if out := run(dollars, rateDir(t, rateFileHead+"usd_rate: \"83.2500\"\n")); strings.Contains(out, "== Dollar rate ==") {
		t.Errorf("a usable rate: the note appears:\n%s", out)
	}
	// rupee amounts only: no dollar amount is met, so a missing rate is not worth a word
	if r := cmd(t, "run", "--db", newDB(t), "--input", filepath.Join(repo, "testdata", "run", "differs"), "--tax-year", "2025-26", "--vocab", vocabDir, "--profiles", profDir,
		"--include-testing", "--currency", none); strings.Contains(r.stdout, "== Dollar rate ==") {
		t.Errorf("a run with no dollar amount has the note:\n%s", r.stdout)
	}
}
