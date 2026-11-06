package comparison

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

var content = filepath.Join("..", "..", "content")

// The shipped comparison types load, and every rule they name exists and reads only the
// documents the type lists.
func TestShippedComparisons(t *testing.T) {
	v, err := vocab.LoadDir(filepath.Join(content, "vocabulary"))
	if err != nil {
		t.Fatal(err)
	}
	list, err := Load(filepath.Join(content, "comparisons", "comparisons.yaml"), v)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 11 {
		t.Errorf("%d comparisons, want 11", len(list))
	}
	set, err := rules.LoadDir(filepath.Join(content, "rules"), v, checks.All())
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]*rules.Rule{}
	for _, r := range set.Rules() {
		byID[r.ID] = r
	}
	for _, c := range list {
		for _, id := range c.Rules {
			r, ok := byID[id]
			if !ok {
				t.Errorf("%s: rule %s does not exist", c.ID, id)
				continue
			}
			for _, in := range r.Inputs {
				doc := ""
				if f, ok := v.Lookup(in.Fact); ok {
					doc = f.DocumentType
				} else if rt, ok := v.RecordType(in.RecordType); ok {
					doc = rt.DocumentType
				}
				if !strings.Contains(" "+strings.Join(c.Documents, " ")+" ", " "+doc+" ") {
					t.Errorf("%s: rule %s reads %s from %q, which the comparison does not list", c.ID, id, in.Name, doc)
				}
			}
		}
	}
	if _, ok := Find(list, "gst_sales_vs_gstr1"); !ok {
		t.Error("gst_sales_vs_gstr1 not found")
	}
}

func TestLoadRefuses(t *testing.T) {
	v, err := vocab.LoadDir(filepath.Join(content, "vocabulary"))
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"unknown document":       `{ id: "x", name: "X", description: "", documents: ["computation", "nope"], rules: ["R-1"], totals: false }`,
		"one document":           `{ id: "x", name: "X", description: "", documents: ["computation"], rules: ["R-1"], totals: false }`,
		"nothing to do":          `{ id: "x", name: "X", description: "", documents: ["computation", "gstr1"], rules: [], totals: false }`,
		"worklist with rules":    `{ id: "x", name: "X", description: "", documents: ["ledger"], rules: ["R-1"], totals: false, worklist: true }`,
		"worklist with two docs": `{ id: "x", name: "X", description: "", documents: ["ledger", "gstr1"], rules: [], totals: false, worklist: true }`,
	} {
		t.Run(name, func(t *testing.T) {
			f := filepath.Join(t.TempDir(), "c.yaml")
			os.WriteFile(f, []byte("schema: \"recon.comparisons/1\"\ncomparisons:\n  - "+body+"\n"), 0o644)
			if _, err := Load(f, v); err == nil {
				t.Error("accepted")
			}
		})
	}
}
