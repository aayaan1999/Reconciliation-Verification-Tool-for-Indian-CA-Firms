package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the screens' sample data in web/src/sample from the engine")

// webSamples are the engine outputs the screens show as sample data (SPEC-004 part 3):
// each testdata/run case, as `run --json` and `facts --json` write it.
var webSamples = []string{"clean", "differs", "missing", "compare-differs", "gst-sales", "gst-purchase", "filing-pack"}

// webSampleRun says, for a sample that needs it, its tax year and comparison type.
var webSampleRun = map[string][]string{
	"gst-sales":    {"--tax-year", "2026-27", "--comparison", "gst_sales_vs_gstr1", "--comparisons", comparisonsFile},
	"gst-purchase": {"--tax-year", "2026-27", "--comparison", "gst_purchase_vs_gstr2b", "--comparisons", comparisonsFile},
	"filing-pack":  {"--tax-year", "2025-26", "--comparison", "taxsuite_filing_pack", "--comparisons", comparisonsFile},
}

var comparisonsFile = filepath.Join(repo, "content", "comparisons", "comparisons.yaml")

// TestWebSamplesMatchEngine keeps the screens' sample data equal, byte for byte, to what
// the engine writes for the same input, so connecting the screens to the real engine later
// changes nothing on screen. Run with -update after an intended change to the output.
func TestWebSamplesMatchEngine(t *testing.T) {
	dir := filepath.Join(repo, "web", "src", "sample")
	t.Run("rules", func(t *testing.T) {
		db := newDB(t)
		out := filepath.Join(t.TempDir(), "rules.json")
		if r := cmd(t, "rules", "--db", db, "--vocab", vocabDir, "--json", out); r.code != exitOK {
			t.Fatalf("rules: exit %d\n%s", r.code, r.stderr)
		}
		compareSample(t, filepath.Join(dir, "rules.json"), out)
	})
	t.Run("comparisons", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "comparisons.json")
		if r := cmd(t, "comparisons", "--comparisons", comparisonsFile, "--vocab", vocabDir, "--json", out); r.code != exitOK {
			t.Fatalf("comparisons: exit %d\n%s", r.code, r.stderr)
		}
		compareSample(t, filepath.Join(dir, "comparisons.json"), out)
	})
	// The Worklist screen's sample: the engine's Worklist on Test Pack A's two ledgers
	// (SPEC-006 part 3), for a client named as the Test Pack's sample client.
	t.Run("worklist", func(t *testing.T) {
		db := newDB(t)
		if r := withStdin(t, nimbusPAN, "client-add", "--db", db, "--name", "Nimbus Logic Systems Private Limited (Test Pack A)", "--tax-year", "2025-26"); r.code != exitOK {
			t.Fatalf("client-add: exit %d\n%s", r.code, r.stderr)
		}
		if r := runCmd(t, db, ledgers(t), "--client", "1"); r.code > exitFindings {
			t.Fatalf("run: exit %d\n%s", r.code, r.stderr)
		}
		out := filepath.Join(t.TempDir(), "worklist.json")
		if r := cmd(t, "worklist", "--db", db, "--tax-year", "2025-26", "--json", out); r.code != exitOK {
			t.Fatalf("worklist: exit %d\n%s", r.code, r.stderr)
		}
		compareSample(t, filepath.Join(dir, "worklist.json"), out)
	})
	for _, c := range webSamples {
		t.Run(c, func(t *testing.T) {
			db := newDB(t)
			input := filepath.Join(runData, c)
			runJSON := filepath.Join(t.TempDir(), "run.json")
			args := append([]string{"--include-testing", "--json", runJSON}, webSampleRun[c]...)
			if r := runCmd(t, db, input, args...); r.code > exitFindings {
				t.Fatalf("run: exit %d\n%s", r.code, r.stderr)
			}
			factsJSON := filepath.Join(t.TempDir(), "facts.json")
			if r := cmd(t, "facts", "--db", db, "--input", input, "--json", factsJSON); r.code != exitOK {
				t.Fatalf("facts: exit %d\n%s", r.code, r.stderr)
			}
			for name, file := range map[string]string{"run-" + c + ".json": runJSON, "facts-" + c + ".json": factsJSON} {
				compareSample(t, filepath.Join(dir, name), file)
			}
		})
	}
}

// compareSample checks (or with -update, rewrites) one sample file against engine output.
func compareSample(t *testing.T, sample, engineOutput string) {
	t.Helper()
	got, err := os.ReadFile(engineOutput)
	if err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.WriteFile(sample, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(sample)
	if err != nil || string(want) != string(got) {
		t.Errorf("web/src/sample/%s differs from the engine's output; run: go test ./cmd/recon-engine -run TestWebSamplesMatchEngine -update", filepath.Base(sample))
	}
}
