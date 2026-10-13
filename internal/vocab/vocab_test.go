package vocab

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
)

// Shipped content, relative to this package.
var (
	shippedDir = filepath.Join("..", "..", "content", "vocabulary")
	shippedDoc = filepath.Join("..", "..", filepath.FromSlash(DocPath))
)

const goodFacts = `schema: "recon.vocab.facts/1"
vocab_version: 2
document_types:
  - name: "computation"
    prefix: "computation"
    description: "Computation of income"
    since_vocab: 1
  - name: "tax_audit_report"
    prefix: "tar"
    description: "Tax audit report"
    since_vocab: 1
facts:
  - name: "tar.depreciation_schedule.total"
    kind: "money"
    periodicity: "FY"
    document_type: "tax_audit_report"
    description: "Depreciation schedule total"
    since_vocab: 1
  - name: "computation.allowed_depreciation"
    kind: "money"
    periodicity: "FY"
    document_type: "computation"
    description: "Allowed depreciation"
    since_vocab: 1
    deprecated_in: 2
  - name: "computation.tds.section"
    kind: "text"
    periodicity: "quarter"
    document_type: "computation"
    description: "A text fact"
    since_vocab: 2
`

const goodRecords = `schema: "recon.vocab.records/1"
vocab_version: 2
record_types:
  - name: "gst_invoice"
    description: "Invoice line"
    match_key: ["counterparty_gstin", "invoice_no", "invoice_date"]
    amounts: ["taxable_value", "igst"]
    fields: ["counterparty_gstin", "invoice_no", "invoice_date", "place_of_supply"]
    since_vocab: 2
`

func load(t *testing.T, facts, records string) (*Vocab, error) {
	t.Helper()
	return Load("facts.yaml", []byte(facts), "records.yaml", []byte(records))
}

func mustLoad(t *testing.T, facts, records string) *Vocab {
	t.Helper()
	v, err := load(t, facts, records)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestLoadAndLookup(t *testing.T) {
	v := mustLoad(t, goodFacts, goodRecords)
	if v.Version() != 2 {
		t.Errorf("Version() = %d", v.Version())
	}
	f, ok := v.Lookup("tar.depreciation_schedule.total")
	if !ok || f.Kind != domain.KindMoney || f.Periodicity != PeriodicityFY || f.DocumentType != "tax_audit_report" || f.Deprecated() {
		t.Errorf("Lookup = %+v, %v", f, ok)
	}
	if f, _ := v.Lookup("computation.allowed_depreciation"); !f.Deprecated() || f.DeprecatedIn != 2 {
		t.Errorf("deprecated fact = %+v", f)
	}
	if !v.IsFact("computation.tds.section") || v.IsFact("gst_invoice") || v.IsFact("computation.nope") {
		t.Error("IsFact wrong")
	}
	if !v.IsRecordType("gst_invoice") || v.IsRecordType("tar.depreciation_schedule.total") {
		t.Error("IsRecordType wrong")
	}
	r, ok := v.RecordType("gst_invoice")
	if !ok || strings.Join(r.MatchKey, ",") != "counterparty_gstin,invoice_no,invoice_date" {
		t.Errorf("RecordType = %+v", r)
	}
	r.MatchKey[0] = "changed" // a caller's copy must not change the vocabulary
	if r2, _ := v.RecordType("gst_invoice"); r2.MatchKey[0] != "counterparty_gstin" {
		t.Error("RecordType returned shared slices")
	}
	if d, ok := v.DocumentType("tax_audit_report"); !ok || d.Prefix != "tar" {
		t.Errorf("DocumentType = %+v", d)
	}
	var names []string
	for _, d := range v.DocumentTypes() {
		names = append(names, d.Name)
	}
	if strings.Join(names, ",") != "computation,tax_audit_report" {
		t.Errorf("DocumentTypes not sorted: %v", names)
	}
	names = nil
	for _, f := range v.Facts() {
		names = append(names, f.Name)
	}
	if strings.Join(names, ",") != "computation.allowed_depreciation,computation.tds.section,tar.depreciation_schedule.total" {
		t.Errorf("Facts not sorted: %v", names)
	}
}

// AC-1: each kind of bad entry is refused with file, line and a plain reason.
func TestLoadRefuses(t *testing.T) {
	cases := []struct {
		name           string
		facts, records string
		file           string
		line           int
		want           string
	}{
		{"duplicate fact", goodFacts + "  - name: \"tar.depreciation_schedule.total\"\n    kind: \"money\"\n    periodicity: \"FY\"\n    document_type: \"tax_audit_report\"\n    description: \"again\"\n    since_vocab: 1\n",
			goodRecords, "facts.yaml", 32, `fact "tar.depreciation_schedule.total" is listed twice`},
		{"upper case name", strings.Replace(goodFacts, `"tar.depreciation_schedule.total"`, `"tar.Depreciation.total"`, 1), goodRecords,
			"facts.yaml", 13, `fact name "tar.Depreciation.total" must be lower case ASCII`},
		{"single-part name", strings.Replace(goodFacts, `"tar.depreciation_schedule.total"`, `"tar"`, 1), goodRecords,
			"facts.yaml", 13, `fact name "tar" must be`},
		{"non-ASCII name", strings.Replace(goodFacts, `"tar.depreciation_schedule.total"`, `"tar.dépreciation"`, 1), goodRecords,
			"facts.yaml", 13, `must be lower case ASCII`},
		{"wrong prefix", strings.Replace(goodFacts, `"tar.depreciation_schedule.total"`, `"computation.x.total"`, 1), goodRecords,
			"facts.yaml", 13, `belongs to document type "tax_audit_report", so its name must start with "tar."`},
		{"unknown key", strings.Replace(goodFacts, `    since_vocab: 2`, "    since_vocab: 2\n    unit: \"rupees\"", 1), goodRecords,
			"facts.yaml", 32, `unknown key "unit"`},
		{"unknown kind", strings.Replace(goodFacts, `kind: "text"`, `kind: "float"`, 1), goodRecords,
			"facts.yaml", 27, `kind "float" of fact "computation.tds.section" is unknown`},
		{"unknown periodicity", strings.Replace(goodFacts, `periodicity: "quarter"`, `periodicity: "Q"`, 1), goodRecords,
			"facts.yaml", 28, `periodicity "Q"`},
		{"unknown document type", strings.Replace(goodFacts, `document_type: "computation"`+"\n    description: \"A text", `document_type: "itr"`+"\n    description: \"A text", 1), goodRecords,
			"facts.yaml", 29, `document type "itr" of fact "computation.tds.section" is not in document_types`},
		{"unquoted text", strings.Replace(goodFacts, `kind: "text"`, `kind: text`, 1), goodRecords,
			"facts.yaml", 27, `kind is text and must be in quotes`},
		{"unquoted name", strings.Replace(goodFacts, `- name: "tar.depreciation_schedule.total"`, `- name: tar.depreciation_schedule.total`, 1), goodRecords,
			"facts.yaml", 13, `name is text and must be in quotes`},
		{"missing description", strings.Replace(goodFacts, `description: "A text fact"`, `description: ""`, 1), goodRecords,
			"facts.yaml", 30, `description of fact "computation.tds.section" is missing`},
		{"since after version", strings.Replace(goodFacts, "    since_vocab: 2\n", "    since_vocab: 3\n", 1), goodRecords,
			"facts.yaml", 31, `since_vocab must be between 1 and vocab_version (2)`},
		{"deprecated before since", strings.Replace(goodFacts, `deprecated_in: 2`, `deprecated_in: 1`, 1), goodRecords,
			"facts.yaml", 25, `deprecated_in must be after since_vocab`},
		{"wrong schema", strings.Replace(goodFacts, `recon.vocab.facts/1`, `recon.vocab/1`, 1), goodRecords,
			"facts.yaml", 1, `schema must be "recon.vocab.facts/1"`},
		{"duplicate prefix", strings.Replace(goodFacts, `prefix: "tar"`, `prefix: "computation"`, 1), goodRecords,
			"facts.yaml", 9, `prefix "computation" is used by two document types`},
		{"duplicate document type", strings.Replace(goodFacts, `- name: "tax_audit_report"`, `- name: "computation"`, 1), goodRecords,
			"facts.yaml", 8, `document type "computation" is listed twice`},
		{"version mismatch", goodFacts, strings.Replace(goodRecords, "vocab_version: 2", "vocab_version: 1", 1),
			"records.yaml", 2, `vocab_version is 1 here but 2 in facts.yaml`},
		{"match key not a field", goodFacts, strings.Replace(goodRecords, `"invoice_no", "invoice_date"]`, `"invoice_no", "invoice_month"]`, 1),
			"records.yaml", 6, `match_key of record type "gst_invoice" names "invoice_month", which is not one of its fields`},
		{"match key is an amount", goodFacts, strings.Replace(goodRecords, `match_key: ["counterparty_gstin"`, `match_key: ["igst"`, 1),
			"records.yaml", 6, `names "igst", which is not one of its fields`},
		{"empty match key", goodFacts, strings.Replace(goodRecords, `match_key: ["counterparty_gstin", "invoice_no", "invoice_date"]`, `match_key: []`, 1),
			"records.yaml", 4, `record type "gst_invoice" needs a match_key`},
		{"column in both lists", goodFacts, strings.Replace(goodRecords, `"place_of_supply"]`, `"igst"]`, 1),
			"records.yaml", 8, `column "igst" of record type "gst_invoice" is in both amounts and fields`},
		{"duplicate record type", goodFacts, goodRecords + strings.SplitN(goodRecords, "record_types:\n", 2)[1],
			"records.yaml", 10, `record type "gst_invoice" is listed twice`},
		{"record name with dot", goodFacts, strings.Replace(goodRecords, `"gst_invoice"`, `"gst.invoice"`, 1),
			"records.yaml", 4, `record type name "gst.invoice" must be`},
		{"records unquoted", goodFacts, strings.Replace(goodRecords, `amounts: ["taxable_value", "igst"]`, `amounts: [taxable_value, igst]`, 1),
			"records.yaml", 7, `is text and must be in quotes`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := load(t, c.facts, c.records)
			var es strictyaml.Errors
			if !errors.As(err, &es) {
				t.Fatalf("err = %v, want strictyaml.Errors", err)
			}
			for _, e := range es {
				if e.File == c.file && e.Line == c.line && strings.Contains(e.Msg, c.want) {
					return
				}
			}
			t.Errorf("no error at %s:%d containing %q; got:\n%v", c.file, c.line, c.want, err)
		})
	}
}

// AC-2, AC-4, AC-5: the shipped files load, expose their version and hold the SPEC-011 facts.
func TestShippedVocabulary(t *testing.T) {
	v, err := LoadDir(shippedDir)
	if err != nil {
		t.Fatal(err)
	}
	if v.Version() < 1 {
		t.Errorf("Version() = %d", v.Version())
	}
	for _, name := range []string{"tar.depreciation_schedule.total", "computation.allowed_depreciation", "computation.fixed_asset_schedule.total"} {
		f, ok := v.Lookup(name)
		if !ok || f.Kind != domain.KindMoney || f.Periodicity != PeriodicityFY || f.Deprecated() {
			t.Errorf("%s: %+v, %v", name, f, ok)
		}
	}
	for _, d := range []string{"computation", "tax_audit_report"} {
		if _, ok := v.DocumentType(d); !ok {
			t.Errorf("document type %s missing", d)
		}
	}
}

// AC-3: the committed document is exactly what the shipped vocabulary generates.
func TestCommittedDocIsUpToDate(t *testing.T) {
	v, err := LoadDir(shippedDir)
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile(shippedDoc)
	if err != nil {
		t.Fatalf("%v; generate it with: go run ./cmd/recon-dev vocab-doc", err)
	}
	if err := CheckDoc(committed, v); err != nil {
		t.Fatal(err)
	}
}

func TestCheckDoc(t *testing.T) {
	v1 := mustLoad(t, goodFacts, goodRecords)
	doc := Markdown(v1)
	if err := CheckDoc(doc, v1); err != nil {
		t.Errorf("identical document refused: %v", err)
	}
	if err := CheckDoc([]byte(strings.ReplaceAll(string(doc), "\n", "\r\n")), v1); err != nil {
		t.Errorf("CRLF line endings refused: %v", err)
	}

	changed := mustLoad(t, strings.Replace(goodFacts, `"A text fact"`, `"A different meaning"`, 1), goodRecords)
	err := CheckDoc(doc, changed)
	if err == nil || !strings.Contains(err.Error(), "increase vocab_version") {
		t.Errorf("content change without version increase: err = %v", err)
	}
	if err := CheckVersionBumped(doc, changed); err == nil {
		t.Error("CheckVersionBumped accepted a change without a version increase")
	}

	bumped := mustLoad(t,
		strings.Replace(strings.Replace(goodFacts, `"A text fact"`, `"A different meaning"`, 1), "vocab_version: 2", "vocab_version: 3", 1),
		strings.Replace(goodRecords, "vocab_version: 2", "vocab_version: 3", 1))
	if err := CheckVersionBumped(doc, bumped); err != nil {
		t.Errorf("version increase refused: %v", err)
	}
	if err := CheckDoc(doc, bumped); err == nil || !strings.Contains(err.Error(), "regenerate") {
		t.Errorf("stale document after a proper bump: err = %v", err)
	}
}

// Comments, spacing and entry order are not content: they leave the hash and document unchanged.
func TestContentHashIgnoresLayout(t *testing.T) {
	a := mustLoad(t, goodFacts, goodRecords)
	reordered := strings.Replace(goodFacts, "facts:\n", "# a new comment\nfacts:\n\n", 1)
	parts := strings.SplitN(reordered, "  - name: \"computation.allowed_depreciation\"", 2)
	tail := strings.SplitN(parts[1], "  - name: \"computation.tds.section\"", 2)
	reordered = parts[0] + "  - name: \"computation.tds.section\"" + tail[1] + "  - name: \"computation.allowed_depreciation\"" + tail[0]
	b := mustLoad(t, reordered, goodRecords)
	if a.ContentHash() != b.ContentHash() || string(Markdown(a)) != string(Markdown(b)) {
		t.Error("layout change altered the content hash or document")
	}
	c := mustLoad(t, strings.Replace(goodFacts, `kind: "text"`, `kind: "date"`, 1), goodRecords)
	if a.ContentHash() == c.ContentHash() {
		t.Error("kind change did not alter the content hash")
	}
}

func TestMarkdownEscapesPipes(t *testing.T) {
	v := mustLoad(t, strings.Replace(goodFacts, `"A text fact"`, `"Section | code"`, 1), goodRecords)
	if !strings.Contains(string(Markdown(v)), `Section \| code`) {
		t.Error("pipe in a description not escaped")
	}
	if !strings.Contains(string(Markdown(v)), "| `gst_invoice` | `counterparty_gstin`, `invoice_no`, `invoice_date` |") {
		t.Errorf("record type row missing:\n%s", Markdown(v))
	}
}
