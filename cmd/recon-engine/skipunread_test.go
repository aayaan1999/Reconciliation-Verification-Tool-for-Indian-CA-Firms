package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A folder with files the tool cannot read: without --skip-unread the first one stops the run
// (as before); with it, the readable files are compared and the others are listed as not read.
func TestSkipUnreadListsFilesNotRead(t *testing.T) {
	dir := t.TempDir()
	put := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	good, err := os.ReadFile(filepath.Join(runData, "gst-sales", "GSTR1_Aug2026.json"))
	if err != nil {
		t.Fatal(err)
	}
	put("gstr1_aug.json", good)
	put("unknown_layout.csv", []byte("A,B\n1,2\n"))
	put("tally_export.xml", []byte("<ENVELOPE/>"))
	db := formatsDB(t)

	if r := runCmd(t, db, dir, "--tax-year", "2026-27"); r.code != exitInput {
		t.Errorf("without --skip-unread: exit %d, want %d (%s)", r.code, exitInput, r.stderr)
	}
	r := runCmd(t, db, dir, "--tax-year", "2026-27", "--skip-unread")
	if r.code != exitOK && r.code != exitFindings {
		t.Fatalf("with --skip-unread: exit %d\n%s", r.code, r.stderr)
	}
	for _, want := range []string{"Files NOT read (2)", "PROFILE_NO_MATCH: unknown_layout.csv", "UNSUPPORTED_FILE_TYPE: tally_export.xml", "gstr1_aug.json"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("output lacks %q:\n%s", want, r.stdout)
		}
	}
	if strings.Contains(r.stdout, "PROFILE_NO_MATCH: gstr1_aug.json") {
		t.Error("the readable file was listed as not read")
	}

	// Nothing readable at all is still an input error, naming the files.
	none := t.TempDir()
	if err := os.WriteFile(filepath.Join(none, "x.csv"), []byte("A,B\n1,2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r := runCmd(t, db, none, "--tax-year", "2026-27", "--skip-unread"); r.code != exitInput || !strings.Contains(r.stderr, "NO_FILE_READ") {
		t.Errorf("no file readable: exit %d %s", r.code, r.stderr)
	}
}
