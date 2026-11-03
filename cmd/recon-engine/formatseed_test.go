package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
)

// SPEC-016: load-rules stores every standard profile as a format and every vocabulary fact and
// record type as a parameter; loading again adds nothing.
func TestLoadRulesStoresStandardFormatsAndParameters(t *testing.T) {
	db := filepath.Join(t.TempDir(), "recon.db")
	load := func() result {
		return cmd(t, "load-rules", "--db", db, "--store", storeDir, "--rules", rulesDir, "--vocab", vocabDir, "--profiles", profDir)
	}
	first := load()
	if first.code != 0 || !strings.Contains(first.stdout, "format versions and") {
		t.Fatalf("first load: exit %d\n%s%s", first.code, first.stdout, first.stderr)
	}
	read := func() ([]store.StoredFormat, []store.StoredParameter) {
		d, err := store.Open(db)
		if err != nil {
			t.Fatal(err)
		}
		defer d.Close()
		tx, err := d.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		f, err := tx.Formats()
		if err != nil {
			t.Fatal(err)
		}
		p, err := tx.Parameters()
		if err != nil {
			t.Fatal(err)
		}
		return f, p
	}
	formats, params := read()
	if len(formats) == 0 || len(params) == 0 {
		t.Fatalf("%d formats and %d parameters stored", len(formats), len(params))
	}
	for _, f := range formats {
		if f.Version.Source != store.OriginBuiltin || f.State.Status != store.FormatActive || f.Version.BodyYAML == "" {
			t.Errorf("format %s: %+v", f.State.FormatID, f.State)
		}
	}
	var sawFact, sawRecord bool
	for _, p := range params {
		sawFact = sawFact || p.Version.ParamType == "money"
		sawRecord = sawRecord || p.Version.ParamType == "record"
	}
	if !sawFact || !sawRecord {
		t.Errorf("parameters hold a money fact: %v, a record type: %v", sawFact, sawRecord)
	}

	second := load()
	if second.code != 0 || strings.Contains(second.stdout, "format versions and") {
		t.Errorf("second load added versions: exit %d\n%s", second.code, second.stdout)
	}
	if f2, p2 := read(); len(f2) != len(formats) || len(p2) != len(params) {
		t.Errorf("counts changed on a second load: %d/%d formats, %d/%d parameters", len(f2), len(formats), len(p2), len(params))
	}
}
