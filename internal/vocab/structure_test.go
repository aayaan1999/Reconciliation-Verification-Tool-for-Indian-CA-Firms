package vocab

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
)

func TestExtraStructuralVocab(t *testing.T) {
	t.Run("empty lists", func(t *testing.T) {
		f := "schema: \"recon.vocab.facts/1\"\nvocab_version: 1\ndocument_types: []\nfacts: []\n"
		r := "schema: \"recon.vocab.records/1\"\nvocab_version: 1\nrecord_types: []\n"
		v, err := load(t, f, r)
		if err != nil || len(v.Facts()) != 0 || len(v.RecordTypes()) != 0 || len(v.DocumentTypes()) != 0 {
			t.Fatalf("err=%v", err)
		}
		if err := CheckDoc(Markdown(v), v); err != nil {
			t.Error(err)
		}
		if !strings.Contains(string(Markdown(v)), "No record types yet.") {
			t.Error("empty record section missing")
		}
	})
	t.Run("facts and records files swapped", func(t *testing.T) {
		if _, err := Load("facts.yaml", []byte(goodRecords), "records.yaml", []byte(goodFacts)); err == nil {
			t.Fatal("swapped files accepted")
		}
	})
	t.Run("two semantic errors in two files are both reported", func(t *testing.T) {
		_, err := load(t, strings.Replace(goodFacts, `kind: "text"`, `kind: "zzz"`, 1), strings.Replace(goodRecords, `"gst_invoice"`, `"Bad"`, 1))
		if err == nil || !strings.Contains(err.Error(), "facts.yaml:") || !strings.Contains(err.Error(), "records.yaml:") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("errors come out sorted by file then line", func(t *testing.T) {
		f := strings.Replace(goodFacts, `kind: "text"`, `kind: "zzz"`, 1)
		f = strings.Replace(f, `prefix: "tar"`, `prefix: "Tar"`, 1)
		_, err := load(t, f, goodRecords)
		var es strictyaml.Errors
		if !errors.As(err, &es) || len(es) < 2 {
			t.Fatal(err)
		}
		for i := 1; i < len(es); i++ {
			if es[i-1].File == es[i].File && es[i-1].Line > es[i].Line {
				t.Errorf("not sorted: %v", err)
			}
		}
	})
	t.Run("nothing returned on failure", func(t *testing.T) {
		v, err := load(t, strings.Replace(goodFacts, `kind: "text"`, `kind: "zzz"`, 1), goodRecords)
		if err == nil || v != nil {
			t.Errorf("v=%v err=%v", v, err)
		}
	})
	t.Run("same fact listed twice with different bodies", func(t *testing.T) {
		extra := "  - name: \"tar.depreciation_schedule.total\"\n    kind: \"int\"\n    periodicity: \"month\"\n    document_type: \"tax_audit_report\"\n    description: \"d\"\n    since_vocab: 1\n"
		_, err := load(t, goodFacts+extra, goodRecords)
		if err == nil || !strings.Contains(err.Error(), "listed twice") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("same name under another document type", func(t *testing.T) {
		extra := "  - name: \"tar.depreciation_schedule.total\"\n    kind: \"money\"\n    periodicity: \"FY\"\n    document_type: \"computation\"\n    description: \"d\"\n    since_vocab: 1\n"
		if _, err := load(t, goodFacts+extra, goodRecords); err == nil {
			t.Error("accepted")
		}
	})
	t.Run("prefix that is the start of another word", func(t *testing.T) {
		f := strings.Replace(goodFacts, `prefix: "tar"`, `prefix: "comp"`, 1)
		f = strings.Replace(f, `"tar.depreciation_schedule.total"`, `"comp.x.y"`, 1)
		v := mustLoad(t, f, goodRecords)
		if !v.IsFact("comp.x.y") || !v.IsFact("computation.tds.section") {
			t.Error("facts missing")
		}
		g := strings.Replace(f, `"comp.x.y"`, `"computation.x.y"`, 1)
		if _, err := load(t, g, goodRecords); err == nil {
			t.Error("name starting with a longer word accepted for prefix comp")
		}
	})
	t.Run("record columns", func(t *testing.T) {
		cases := []struct{ name, old, repl string }{
			{"duplicate in amounts", `amounts: ["taxable_value", "igst"]`, `amounts: ["igst", "igst"]`},
			{"duplicate in fields", `"place_of_supply"]`, `"invoice_no"]`},
			{"match_key twice", `match_key: ["counterparty_gstin", "invoice_no", "invoice_date"]`, `match_key: ["invoice_no", "invoice_no"]`},
			{"empty column name", `amounts: ["taxable_value", "igst"]`, `amounts: [""]`},
			{"match key names a missing field", `"invoice_date"]` + "\n    amounts", `"nope"]` + "\n    amounts"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				r := strings.Replace(goodRecords, c.old, c.repl, 1)
				if r == goodRecords {
					t.Fatal("replacement did not apply")
				}
				if _, err := load(t, goodFacts, r); err == nil {
					t.Error("accepted")
				}
			})
		}
	})
	t.Run("record with no amounts is allowed", func(t *testing.T) {
		if _, err := load(t, goodFacts, strings.Replace(goodRecords, `amounts: ["taxable_value", "igst"]`, `amounts: []`, 1)); err != nil {
			t.Error(err)
		}
		if _, err := load(t, goodFacts, strings.Replace(goodRecords, "    amounts: [\"taxable_value\", \"igst\"]\n", "", 1)); err != nil {
			t.Error(err)
		}
	})
}

func TestExtraLoadDirErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadDir(filepath.Join(dir, "missing")); err == nil {
		t.Error("missing directory accepted")
	}
	if err := os.WriteFile(filepath.Join(dir, FactsFile), []byte(goodFacts), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDir(dir); err == nil {
		t.Error("missing records.yaml accepted")
	}
	if err := os.WriteFile(filepath.Join(dir, RecordsFile), []byte(goodRecords), 0o600); err != nil {
		t.Fatal(err)
	}
	v, err := LoadDir(dir)
	if err != nil || v.Version() != 2 {
		t.Fatal(err)
	}
	crlf := strings.ReplaceAll(goodFacts, "\n", "\r\n")
	if err := os.WriteFile(filepath.Join(dir, FactsFile), []byte("\xef\xbb\xbf"+crlf), 0o600); err != nil {
		t.Fatal(err)
	}
	v2, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if v2.ContentHash() != v.ContentHash() {
		t.Error("CRLF+BOM changed the content hash")
	}
	for _, e := range []string{FactsFile, RecordsFile} {
		if err := os.WriteFile(filepath.Join(dir, e), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := LoadDir(dir); err == nil || !strings.Contains(fmt.Sprint(err), "the file is empty") {
		t.Errorf("empty files: %v", err)
	}
}
