package profile

import (
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
)

// SPEC-015 AC-2: a profile version that supersedes an earlier one, a record block whose amounts
// are negated, and a field with a fixed value. All values are illustrative.

const rowsProfile = `schema: "recon.profile/1"
profile_id: "test.portal.returns"
profile_version: "1"
source_system: "Test portal"
document_type: "gst_return"
file_type: "json"
regime: "CGST2017"
detect:
  - json_exists: { path: "$.inv" }
context:
  tax_year: { const: "2026-27" }
  period: { const: "FY" }
records:
  - record_type: "gst_invoice"
    rows: "$.inv[*]"
    unit: "rupees"
    fields:
      - { name: "counterparty_gstin", path: ".ctin", required: true }
      - { name: "invoice_no", path: ".num", required: true }
      - { name: "place_of_supply", const: "27" }
    amounts:
      - { name: "taxable_value", path: ".tx" }
  - record_type: "gst_invoice"
    rows: "$.cn[*]"
    sign: "negate"
    unit: "rupees"
    fields:
      - { name: "counterparty_gstin", path: ".ctin", required: true }
      - { name: "invoice_no", path: ".num", required: true }
      - { name: "place_of_supply", const: "29" }
    amounts:
      - { name: "taxable_value", path: ".tx" }
`

const rowsDoc = `{"inv": [{"ctin": "27AAAAA0000A1Z5", "num": "1", "tx": 100}], "cn": [{"ctin": "27AAAAA0000A1Z5", "num": "C1", "tx": 30.50}]}`

func TestRecordSignAndConst(t *testing.T) {
	r := extract(t, load(t, rowsProfile), rowsDoc, Source{FileName: "returns.json"})
	if r.Report.HasErrors() {
		t.Fatalf("errors: %v", r.Report.Errors)
	}
	type row struct {
		pos   string
		paise money.Paise
	}
	got := map[string]row{}
	for _, rec := range r.Records {
		got[rec.Fields["invoice_no"]] = row{rec.Fields["place_of_supply"], rec.Amounts["taxable_value"]}
	}
	if got["1"] != (row{"27", 10000}) || got["C1"] != (row{"29", -3050}) {
		t.Errorf("records %v: want the invoice as it is and the credit note negated, each with its block's fixed place of supply", got)
	}

	v := testVocab(t)
	rep := func(old, new string) string {
		if !strings.Contains(rowsProfile, old) {
			t.Fatalf("test setup: %q not in rowsProfile", old)
		}
		return strings.Replace(rowsProfile, old, new, 1)
	}
	for _, c := range []struct{ name, text, msg string }{
		{"bad sign", rep(`sign: "negate"`, `sign: "minus"`), `sign "minus" must be "as_is", "negate" or "drcr"`},
		{"const and path", rep(`{ name: "place_of_supply", const: "27" }`, `{ name: "place_of_supply", const: "27", path: ".pos" }`), "a field has const or path, not both"},
		{"const on an amount", rep(`{ name: "taxable_value", path: ".tx" }`, `{ name: "taxable_value", const: "1" }`), "const is for fields only"},
		{"supersedes itself", rep(`profile_version: "1"`, "profile_version: \"1\"\nsupersedes: \"1\""), "must be an earlier version"},
		{"supersedes a later one", rep(`profile_version: "1"`, "profile_version: \"2\"\nsupersedes: \"3\""), "must be an earlier version"},
	} {
		if _, err := Load("profile.yaml", []byte(c.text), v); err == nil || !strings.Contains(err.Error(), c.msg) {
			t.Errorf("%s: %v, want %q", c.name, err, c.msg)
		}
	}
}

func TestSupersedingVersionReadsTheFile(t *testing.T) {
	v := testVocab(t)
	v1, err := Load("v1/profile.yaml", []byte(rowsProfile), v)
	if err != nil {
		t.Fatal(err)
	}
	v2text := strings.Replace(rowsProfile, `profile_version: "1"`, "profile_version: \"2\"\nsupersedes: \"1\"", 1)
	v2, err := Load("v2/profile.yaml", []byte(v2text), v)
	if err != nil {
		t.Fatal(err)
	}

	// Without supersedes, two versions that both match a file are ambiguous.
	plain, err := Load("v2/profile.yaml", []byte(strings.Replace(rowsProfile, `profile_version: "1"`, `profile_version: "2"`, 1)), v)
	if err != nil {
		t.Fatal(err)
	}
	set, err := NewSet(v1, plain)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Detect(set, openJSON(t, rowsDoc), "returns.json"); err == nil || !strings.Contains(err.Error(), "PROFILE_AMBIGUOUS") {
		t.Errorf("two versions without supersedes: %v, want PROFILE_AMBIGUOUS", err)
	}

	// With it, version 2 reads the file; version 1 is still installed.
	set, err = NewSet(v1, v2)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Detect(set, openJSON(t, rowsDoc), "returns.json")
	if err != nil || p.Key() != "test.portal.returns v2" {
		t.Errorf("detected %v (%v), want version 2", p, err)
	}
	if _, ok := set.Lookup("test.portal.returns", "1"); !ok {
		t.Error("version 1 is no longer installed")
	}

	// Superseding a version that is not installed is refused.
	if _, err := NewSet(v2); err == nil || !strings.Contains(err.Error(), "supersedes version 1, which is not installed") {
		t.Errorf("v2 alone: %v", err)
	}
}
