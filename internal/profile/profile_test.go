package profile

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	jsonadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/json"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// testVocab has one fact of every kind and one record type. All names are invented for
// the tests; the shipped vocabulary is tested through the shipped profiles below.
const testFacts = `schema: "recon.vocab.facts/1"
vocab_version: 3
document_types:
  - { name: "computation", prefix: "computation", description: "Computation", since_vocab: 1 }
  - { name: "tax_audit_report", prefix: "tar", description: "Tax audit report", since_vocab: 1 }
  - { name: "gst_return", prefix: "gst", description: "GST return", since_vocab: 1 }
facts:
  - { name: "computation.allowed_depreciation", kind: "money", periodicity: "FY", document_type: "computation", description: "d", since_vocab: 1 }
  - { name: "computation.schedule_total", kind: "money", periodicity: "FY", document_type: "computation", description: "d", since_vocab: 1 }
  - { name: "computation.old_figure", kind: "money", periodicity: "FY", document_type: "computation", description: "d", since_vocab: 1, deprecated_in: 2 }
  - { name: "computation.employee_count", kind: "int", periodicity: "FY", document_type: "computation", description: "d", since_vocab: 1 }
  - { name: "computation.assessee_name", kind: "text", periodicity: "FY", document_type: "computation", description: "d", since_vocab: 1 }
  - { name: "computation.filing_date", kind: "date", periodicity: "FY", document_type: "computation", description: "d", since_vocab: 1 }
  - { name: "computation.is_audited", kind: "bool", periodicity: "FY", document_type: "computation", description: "d", since_vocab: 1 }
  - { name: "computation.surcharge_rate", kind: "rate", periodicity: "FY", document_type: "computation", description: "d", since_vocab: 1 }
  - { name: "computation.monthly_turnover", kind: "money", periodicity: "month", document_type: "computation", description: "d", since_vocab: 1 }
  - { name: "tar.depreciation_schedule.total", kind: "money", periodicity: "FY", document_type: "tax_audit_report", description: "d", since_vocab: 1 }
`

const testRecords = `schema: "recon.vocab.records/1"
vocab_version: 3
record_types:
  - name: "gst_invoice"
    description: "Invoice line"
    match_key: ["counterparty_gstin", "invoice_no"]
    amounts: ["taxable_value", "igst"]
    fields: ["counterparty_gstin", "invoice_no", "place_of_supply"]
    since_vocab: 1
`

func testVocab(t *testing.T) *vocab.Vocab {
	t.Helper()
	v, err := vocab.Load("facts.yaml", []byte(testFacts), "records.yaml", []byte(testRecords))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

const goodProfile = `schema: "recon.profile/1"
profile_id: "test.suite.computation"
profile_version: "1"
source_system: "Test suite"
document_type: "computation"
file_type: "json"
regime: "ITA1961"
detect:
  - json_equals: { path: "$.meta.form", value: "COMPUTATION" }
  - json_exists: { path: "$.meta.tax_year" }
context:
  tax_year: { json_path: "$.meta.tax_year" }
  pan: { json_path: "$.assessee.pan" }
  period: { const: "FY" }
facts:
  - fact: "computation.allowed_depreciation"
    json_path: "$.income.allowed_depreciation"
    unit: "rupees"
    required: true
  - fact: "computation.schedule_total"
    json_path: "$.assets.total"
    unit: "rupees"
`

const goodDoc = `{
  "meta": {"form": "COMPUTATION", "tax_year": "2025-26"},
  "assessee": {"pan": "ABCDE1234F", "name": "Illustrative Traders"},
  "income": {"allowed_depreciation": 112450.50},
  "assets": {"total": "1,12,450.50", "note": "n"}
}`

func load(t *testing.T, text string) *Profile {
	t.Helper()
	p, err := Load("profile.yaml", []byte(text), testVocab(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return p
}

func openJSON(t *testing.T, text string) ingest.Doc {
	t.Helper()
	d, err := jsonadapter.New(0).Open(strings.NewReader(text), int64(len(text)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return d
}

func extract(t *testing.T, p *Profile, doc string, src Source) *Result {
	t.Helper()
	r, err := Extract(p, openJSON(t, doc), src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	return r
}

// AC-1 and the other load checks: each broken profile is refused at the right line with
// a message that says what is wrong.
func TestLoadRefusals(t *testing.T) {
	v := testVocab(t)
	rep := func(old, new string) string {
		if !strings.Contains(goodProfile, old) {
			t.Fatalf("test setup: %q not in goodProfile", old)
		}
		return strings.Replace(goodProfile, old, new, 1)
	}
	cases := []struct {
		name, text string
		line       int
		msg        string
	}{
		// AC-1
		{"unknown key", rep(`regime: "ITA1961"`, `regime: "ITA1961"`+"\nauthor: \"x\""), 8, `unknown key "author"`},
		{"unknown key in fact", rep(`    required: true`, "    required: true\n    scale: \"1\""), 20, `unknown key "scale"`},
		{"fact not in vocabulary", rep(`"computation.schedule_total"`, `"computation.schedule_totl"`), 20, `fact "computation.schedule_totl" is not in the vocabulary`},
		{"invalid selector", rep(`"$.assets.total"`, `"$..total"`), 21, `recursive descent (..) is not supported`},
		{"unquoted version", rep(`profile_version: "1"`, `profile_version: 1`), 3, `profile_version is text and must be in quotes`},
		{"unquoted tax year", rep(`tax_year: { json_path: "$.meta.tax_year" }`, `tax_year: { const: 2025-26 }`), 12, `const is text and must be in quotes`},
		// identity
		{"wrong schema", rep(`"recon.profile/1"`, `"recon.profile/2"`), 1, `schema must be "recon.profile/1"`},
		{"bad id", rep(`"test.suite.computation"`, `"Test Suite"`), 2, `profile_id "Test Suite" must be lowercase words`},
		{"bad version", rep(`profile_version: "1"`, `profile_version: "1.0"`), 3, `profile_version "1.0" must be a whole number`},
		{"no source", rep(`source_system: "Test suite"`, `source_system: ""`), 4, `source_system must name`},
		{"unknown document type", rep(`document_type: "computation"`, `document_type: "balance_sheet"`), 5, `document_type "balance_sheet" is not in the vocabulary`},
		{"unsupported file type", rep(`file_type: "json"`, `file_type: "xlsx"`), 6, `file_type "xlsx" is not supported (supported: json)`},
		// detect
		{"no detect", rep("detect:\n  - json_equals: { path: \"$.meta.form\", value: \"COMPUTATION\" }\n  - json_exists: { path: \"$.meta.tax_year\" }", "detect: []"), 8, `detect must list at least one test`},
		{"two predicates in one", rep(`  - json_exists: { path: "$.meta.tax_year" }`, "  - json_exists: { path: \"$.meta.tax_year\" }\n    json_equals: { path: \"$.a\", value: \"b\" }"), 10, `exactly one of json_exists, json_equals, json_matches`},
		{"bad regex", rep(`  - json_exists: { path: "$.meta.tax_year" }`, `  - json_matches: { path: "$.meta.tax_year", pattern: "20(" }`), 10, `pattern "20(" is not a valid regular expression`},
		{"wildcard in detect", rep(`{ path: "$.meta.tax_year" }`, `{ path: "$.rows[*]" }`), 10, `can select many values`},
		// context
		{"no tax year", rep("  tax_year: { json_path: \"$.meta.tax_year\" }\n", ""), 12, `context.tax_year is required`},
		{"both sources", rep(`{ json_path: "$.assessee.pan" }`, `{ json_path: "$.assessee.pan", const: "X" }`), 13, `json_path or const, not both`},
		{"empty source", rep(`period: { const: "FY" }`, `period: { const: "" }`), 14, `context.period needs json_path or const`},
		{"bad const tax year", rep(`tax_year: { json_path: "$.meta.tax_year" }`, `tax_year: { const: "2025" }`), 12, `tax year`},
		{"bad const period", rep(`period: { const: "FY" }`, `period: { const: "2025-13" }`), 14, `month is outside 01 to 12`},
		// facts
		{"nothing read", rep(goodProfile[strings.Index(goodProfile, "facts:"):], "facts: []\n"), 1, `the profile reads nothing`},
		{"retired fact", rep(`"computation.schedule_total"`, `"computation.old_figure"`), 20, `was retired in vocabulary version 2`},
		{"fact of another document", rep(`"computation.schedule_total"`, `"tar.depreciation_schedule.total"`), 20, `belongs to document type "tax_audit_report", not "computation"`},
		{"wildcard in fact", rep(`"$.assets.total"`, `"$.assets[*].total"`), 21, `a figure must come from exactly one place`},
		{"missing path", rep(`    json_path: "$.assets.total"`+"\n", ""), 20, `json_path is missing`},
		{"money without unit", rep("    json_path: \"$.assets.total\"\n    unit: \"rupees\"\n", "    json_path: \"$.assets.total\"\n"), 20, `unit is required for money`},
		{"bad unit", rep(`    unit: "rupees"`+"\n    required: true", `    unit: "crore"`+"\n    required: true"), 18, `unit "crore" must be "rupees" or "paise"`},
		{"bad sign", rep("    unit: \"rupees\"\n    required: true", "    unit: \"rupees\"\n    sign: \"flip\"\n    required: true"), 19, `sign "flip" must be "as_is" or "negate"`},
		{"bad rounding", rep("    unit: \"rupees\"\n    required: true", "    unit: \"rupees\"\n    rounding: \"bankers\"\n    required: true"), 19, `rounding "bankers" is not a rounding name`},
		{"rounding none", rep("    unit: \"rupees\"\n    required: true", "    unit: \"rupees\"\n    rounding: \"none\"\n    required: true"), 19, `leave it out to require exact amounts`},
		{"unit on text fact", rep(`"computation.schedule_total"`, `"computation.assessee_name"`), 22, `unit applies only to money facts`},
		{"same fact same path twice", goodProfile + "  - fact: \"computation.schedule_total\"\n    json_path: \"$.assets.total\"\n    unit: \"rupees\"\n", 23, `already read from "$.assets.total"`},
		{"required not a bool", rep(`required: true`, `required: "yes"`), 19, `required must be true or false`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Load("profile.yaml", []byte(c.text), v)
			var es strictyaml.Errors
			if !errors.As(err, &es) {
				t.Fatalf("err = %v, want strictyaml.Errors", err)
			}
			for _, e := range es {
				if strings.Contains(e.Msg, c.msg) {
					if e.Line != c.line || e.File != "profile.yaml" {
						t.Errorf("%q reported at %s:%d, want line %d", c.msg, e.File, e.Line, c.line)
					}
					return
				}
			}
			t.Errorf("no error containing %q; got:\n%v", c.msg, err)
		})
	}
}

func TestLoadReportsEveryProblem(t *testing.T) {
	text := strings.NewReplacer(`"computation.schedule_total"`, `"computation.nope"`, `profile_version: "1"`, `profile_version: "x"`).Replace(goodProfile)
	_, err := Load("profile.yaml", []byte(text), testVocab(t))
	var es strictyaml.Errors
	if !errors.As(err, &es) || len(es) != 2 || es[0].Line != 3 || es[1].Line != 20 {
		t.Fatalf("want two errors at lines 3 and 20, got %v", err)
	}
}

const recordsProfile = `schema: "recon.profile/1"
profile_id: "test.portal.gstr2b"
profile_version: "1"
source_system: "Test portal"
document_type: "gst_return"
file_type: "json"
detect:
  - json_equals: { path: "$.form", value: "GSTR2B" }
context:
  tax_year: { const: "2025-26" }
  period: { json_path: "$.period" }
  scope: { json_path: "$.gstin" }
records:
  - record_type: "gst_invoice"
    rows: "$.invoices[*]"
    unit: "rupees"
    fields:
      - { name: "counterparty_gstin", path: ".supplier", required: true }
      - { name: "invoice_no", path: "['Invoice No']", required: true }
      - { name: "place_of_supply", path: ".pos" }
    amounts:
      - { name: "taxable_value", path: ".taxable", required: true }
      - { name: "igst", path: ".igst" }
`

func TestRecordProfileRefusals(t *testing.T) {
	v := testVocab(t)
	rep := func(old, new string) string {
		if !strings.Contains(recordsProfile, old) {
			t.Fatalf("test setup: %q not found", old)
		}
		return strings.Replace(recordsProfile, old, new, 1)
	}
	for _, c := range []struct {
		name, text, msg string
	}{
		{"unknown record type", rep(`"gst_invoice"`, `"gst_invoyce"`), `record type "gst_invoyce" is not in the vocabulary`},
		{"rows without wildcard", rep(`"$.invoices[*]"`, `"$.invoices"`), `must select every row`},
		{"unknown column", rep(`name: "place_of_supply"`, `name: "state"`), `"state" is not a column of record type "gst_invoice"`},
		{"amount listed as field", rep(`name: "place_of_supply"`, `name: "igst"`), `"igst" is an amount column of "gst_invoice"; list it under amounts`},
		{"column twice", rep(`name: "igst"`, `name: "taxable_value"`), `column "taxable_value" is mapped twice`},
		{"absolute column path", rep(`path: ".pos"`, `path: "$.pos"`), `must be written from the row`},
		{"bad column path", rep(`path: ".pos"`, `path: "..pos"`), `recursive descent`},
		{"match key not mapped", rep("      - { name: \"invoice_no\", path: \"['Invoice No']\", required: true }\n", ""), `match-key column "invoice_no" of "gst_invoice" is not mapped`},
		{"match key optional", rep(`path: ".supplier", required: true`, `path: ".supplier"`), `match-key column "counterparty_gstin" must be required: true`},
		{"amounts without unit", rep(`    unit: "rupees"`+"\n", ""), `unit is required for money`},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := Load("profile.yaml", []byte(c.text), v)
			if err == nil || !strings.Contains(err.Error(), c.msg) {
				t.Errorf("err = %v, want it to contain %q", err, c.msg)
			}
		})
	}
}

func TestExtractRecords(t *testing.T) {
	p := load(t, recordsProfile)
	doc := `{
  "form": "GSTR2B", "period": "2025-05", "gstin": "27AAAAA0000A1Z5",
  "invoices": [
    {"supplier": "29BBBBB1111B1Z6", "Invoice No": "INV|7", "pos": "29", "taxable": 1000.00, "igst": 180, "extra": "x"},
    {"supplier": "07CCCCC2222C1Z7", "Invoice No": "A-1", "taxable": "2,500.50"}
  ]
}`
	r := extract(t, p, doc, Source{EngagementID: 9, DocumentVersionID: 4, FileName: "2b.json"})
	if r.Report.HasErrors() {
		t.Fatalf("errors: %+v", r.Report.Errors)
	}
	want := []domain.Record{
		{EngagementID: 9, RecordType: "gst_invoice", Period: "2025-05", Scope: "27AAAAA0000A1Z5", MatchKey: `07CCCCC2222C1Z7|A-1`,
			Fields:  map[string]string{"counterparty_gstin": "07CCCCC2222C1Z7", "invoice_no": "A-1"},
			Amounts: map[string]money.Paise{"taxable_value": 250050},
			Locator: domain.Locator{DocumentVersionID: 4, FileName: "2b.json", Kind: domain.LocatorJSONPath, Path: "$.invoices[1]"}},
		{EngagementID: 9, RecordType: "gst_invoice", Period: "2025-05", Scope: "27AAAAA0000A1Z5", MatchKey: `29BBBBB1111B1Z6|INV\|7`,
			Fields:  map[string]string{"counterparty_gstin": "29BBBBB1111B1Z6", "invoice_no": "INV|7", "place_of_supply": "29"},
			Amounts: map[string]money.Paise{"taxable_value": 100000, "igst": 18000},
			Locator: domain.Locator{DocumentVersionID: 4, FileName: "2b.json", Kind: domain.LocatorJSONPath, Path: "$.invoices[0]"}},
	}
	if !reflect.DeepEqual(r.Records, want) {
		t.Fatalf("records:\n got %+v\nwant %+v", r.Records, want)
	}
	if r.Report.RecordCount != 2 || r.Report.FactCount != 0 {
		t.Errorf("counts = %d facts, %d records", r.Report.FactCount, r.Report.RecordCount)
	}
	var unread []string
	for _, u := range r.Report.Unread {
		unread = append(unread, u.Locator.Path)
	}
	if !reflect.DeepEqual(unread, []string{"$.invoices[0].extra"}) {
		t.Errorf("unread = %v", unread)
	}

	// A row without a required column makes the whole file unusable.
	bad := strings.Replace(doc, `"taxable": "2,500.50"`, `"tax": 1`, 1)
	r = extract(t, p, bad, Source{FileName: "2b.json"})
	if len(r.Records) != 0 || len(r.Report.Errors) != 1 || r.Report.Errors[0].Code != ingest.RequiredFieldMissing ||
		!strings.Contains(r.Report.Errors[0].Msg, "gst_invoice row $.invoices[1] has no taxable_value") {
		t.Fatalf("missing column: %+v", r.Report)
	}
	// A row that is not an object.
	r = extract(t, p, `{"form": "GSTR2B", "period": "2025-05", "gstin": "x", "invoices": [7]}`, Source{})
	if len(r.Report.Errors) != 1 || !strings.Contains(r.Report.Errors[0].Msg, "is a number, not a row") {
		t.Fatalf("scalar row: %+v", r.Report.Errors)
	}
	// A month period outside the constant tax year.
	r = extract(t, p, strings.Replace(doc, `"2025-05"`, `"2026-05"`, 1), Source{})
	if len(r.Report.Errors) != 1 || !strings.Contains(r.Report.Errors[0].Msg, "the period 2026-05 is outside tax year 2025-26") {
		t.Fatalf("period outside tax year: %+v", r.Report.Errors)
	}
}

func TestExtractFacts(t *testing.T) {
	p := load(t, goodProfile)
	r := extract(t, p, goodDoc, Source{EngagementID: 7, DocumentVersionID: 3, FileName: "comp.json", ExpectedPAN: "abcde1234f"})
	loc := func(path string) domain.Locator {
		return domain.Locator{DocumentVersionID: 3, FileName: "comp.json", Kind: domain.LocatorJSONPath, Path: path}
	}
	want := []domain.Fact{
		{EngagementID: 7, Name: "computation.allowed_depreciation", Period: "FY2025-26", Value: domain.MoneyValue(11245050), Locator: loc("$.income.allowed_depreciation")},
		{EngagementID: 7, Name: "computation.schedule_total", Period: "FY2025-26", Value: domain.MoneyValue(11245050), Locator: loc("$.assets.total")},
	}
	if !reflect.DeepEqual(r.Facts, want) {
		t.Fatalf("facts:\n got %+v\nwant %+v", r.Facts, want)
	}
	rep := r.Report
	if rep.HasErrors() || len(rep.Warnings) != 0 || rep.ProfileID != "test.suite.computation" || rep.ProfileVersion != "1" ||
		rep.TaxYear.String() != "2025-26" || rep.ClientMatch != ingest.ClientMatched || rep.PANLast4 != "234F" || rep.FactCount != 2 ||
		rep.File != "comp.json" || rep.FileType != "json" || rep.SourceSystem != "Test suite" {
		t.Fatalf("report = %+v", rep)
	}
	if r.Context.PAN != "ABCDE1234F" || r.Context.Period != "FY2025-26" {
		t.Errorf("context = %+v", r.Context)
	}
	var unread []string
	for _, u := range rep.Unread {
		unread = append(unread, u.Locator.Path+" ("+u.Reason+")")
	}
	if want := []string{"$.assessee.name (text, not read by the profile)", "$.assets.note (text, not read by the profile)"}; !reflect.DeepEqual(unread, want) {
		t.Errorf("unread = %q, want %q", unread, want)
	}
}

// AC-4: a missing required figure is a report error and produces no fact — never a zero.
func TestRequiredMissing(t *testing.T) {
	p := load(t, goodProfile)
	for name, doc := range map[string]string{
		"absent": strings.Replace(goodDoc, `"allowed_depreciation": 112450.50`, `"other": 1`, 1),
		"null":   strings.Replace(goodDoc, `112450.50`, `null`, 1),
	} {
		r := extract(t, p, doc, Source{FileName: "comp.json"})
		if len(r.Facts) != 0 || r.Report.FactCount != 0 {
			t.Errorf("%s: facts %+v were produced", name, r.Facts)
		}
		errs := r.Report.Errors
		if len(errs) != 1 || errs[0].Code != ingest.RequiredFieldMissing || errs[0].Locator.Path != "$.income.allowed_depreciation" ||
			!strings.Contains(errs[0].Msg, "required figure computation.allowed_depreciation is not in the file") {
			t.Errorf("%s: errors = %+v", name, errs)
		}
	}
	// An optional figure that is missing is simply not read: no error, no zero.
	r := extract(t, p, strings.Replace(goodDoc, `"total": "1,12,450.50", `, ``, 1), Source{})
	if r.Report.HasErrors() || len(r.Facts) != 1 || r.Facts[0].Name != "computation.allowed_depreciation" {
		t.Errorf("optional missing: %+v %+v", r.Facts, r.Report.Errors)
	}
}

// AC-3: the same file twice gives identical facts in identical order.
func TestExtractDeterministic(t *testing.T) {
	p := load(t, goodProfile)
	first := extract(t, p, goodDoc, Source{FileName: "comp.json"})
	for range 20 {
		if again := extract(t, p, goodDoc, Source{FileName: "comp.json"}); !reflect.DeepEqual(again, first) {
			t.Fatalf("results differ:\n%+v\n%+v", again, first)
		}
	}
}

func TestExtractProblems(t *testing.T) {
	p := load(t, goodProfile)
	for _, c := range []struct {
		name, doc string
		code      ingest.Code
		msg       string
	}{
		{"float-only amount", strings.Replace(goodDoc, `112450.50`, `112450.505`, 1), ingest.ParseError, "more than two decimal places"},
		{"exponent", strings.Replace(goodDoc, `112450.50`, `1.1245050e5`, 1), ingest.ParseError, "computation.allowed_depreciation at $.income.allowed_depreciation"},
		{"amount is a bool", strings.Replace(goodDoc, `112450.50`, `true`, 1), ingest.ParseError, "expected a money value, found a bool"},
		{"amount is an object", strings.Replace(goodDoc, `112450.50`, `{"v": 1}`, 1), ingest.ParseError, "found an object"},
		{"bad tax year", strings.Replace(goodDoc, `"2025-26"`, `"2025-27"`, 1), ingest.ParseError, `the tax year "2025-27" cannot be read`},
		{"tax year is an object", strings.Replace(goodDoc, `"tax_year": "2025-26"`, `"tax_year": {}`, 1), ingest.ParseError, "the tax year at $.meta.tax_year is an object"},
		{"PAN mismatch", goodDoc, ingest.ConflictingValues, "the PAN in the file (ending 234F) is not this client's PAN"},
	} {
		t.Run(c.name, func(t *testing.T) {
			src := Source{FileName: "comp.json"}
			if c.name == "PAN mismatch" {
				src.ExpectedPAN = "ZZZZZ9999Z"
			}
			r := extract(t, p, c.doc, src)
			if len(r.Facts) != 0 {
				t.Errorf("facts produced despite an error: %+v", r.Facts)
			}
			for _, e := range r.Report.Errors {
				if e.Code == c.code && strings.Contains(e.Msg, c.msg) {
					return
				}
			}
			t.Errorf("no %s error containing %q; got %+v", c.code, c.msg, r.Report.Errors)
		})
	}
	r := extract(t, p, goodDoc, Source{ExpectedPAN: "ZZZZZ9999Z"})
	if r.Report.ClientMatch != ingest.ClientMismatched {
		t.Errorf("ClientMatch = %s", r.Report.ClientMatch)
	}
	r = extract(t, p, strings.Replace(goodDoc, `"ABCDE1234F"`, `"ABC"`, 1), Source{})
	if len(r.Report.Warnings) != 1 || !strings.Contains(r.Report.Warnings[0].Msg, "not in the usual form") || r.Report.HasErrors() {
		t.Errorf("odd PAN: %+v", r.Report)
	}
	if r.Report.ClientMatch != ingest.ClientNotChecked {
		t.Errorf("ClientMatch without an expected PAN = %s", r.Report.ClientMatch)
	}
}

func TestConflictingValues(t *testing.T) {
	text := goodProfile + "  - fact: \"computation.allowed_depreciation\"\n    json_path: \"$.summary.depreciation\"\n    unit: \"rupees\"\n"
	p := load(t, text)
	same := strings.Replace(goodDoc, `"assets"`, `"summary": {"depreciation": "1,12,450.50"}, "assets"`, 1)
	r := extract(t, p, same, Source{})
	if r.Report.HasErrors() || len(r.Facts) != 2 || r.Facts[0].Locator.Path != "$.income.allowed_depreciation" {
		t.Fatalf("same value twice: %+v %+v", r.Facts, r.Report.Errors)
	}
	differ := strings.Replace(goodDoc, `"assets"`, `"summary": {"depreciation": 112540.50}, "assets"`, 1)
	r = extract(t, p, differ, Source{})
	if len(r.Facts) != 0 || len(r.Report.Errors) != 1 || r.Report.Errors[0].Code != ingest.ConflictingValues ||
		r.Report.Errors[0].Msg != "computation.allowed_depreciation is 112450.50 at $.income.allowed_depreciation but 112540.50 at $.summary.depreciation" {
		t.Fatalf("conflict: %+v", r.Report.Errors)
	}
}

func TestValueKinds(t *testing.T) {
	text := goodProfile + `  - { fact: "computation.employee_count", json_path: "$.x.n" }
  - { fact: "computation.assessee_name", json_path: "$.assessee.name" }
  - { fact: "computation.filing_date", json_path: "$.x.d" }
  - { fact: "computation.is_audited", json_path: "$.x.b" }
  - { fact: "computation.surcharge_rate", json_path: "$.x.r" }
`
	p := load(t, text)
	doc := strings.Replace(goodDoc, `"assets"`, `"x": {"n": 42, "d": "2026-09-30", "b": true, "r": "12.5"}, "assets"`, 1)
	r := extract(t, p, doc, Source{})
	if r.Report.HasErrors() {
		t.Fatalf("errors: %+v", r.Report.Errors)
	}
	got := map[string]domain.Value{}
	for _, f := range r.Facts {
		got[f.Name] = f.Value
	}
	for name, want := range map[string]domain.Value{
		"computation.employee_count": domain.IntValue(42), "computation.assessee_name": domain.TextValue("Illustrative Traders"),
		"computation.filing_date": domain.DateValue("2026-09-30"), "computation.is_audited": domain.BoolValue(true),
		"computation.surcharge_rate": domain.RateValue(1250),
	} {
		if got[name] != want {
			t.Errorf("%s = %+v, want %+v", name, got[name], want)
		}
	}
	for _, c := range []struct{ field, value, msg string }{
		{`"n": 42`, `"n": 4.2`, `"4.2" is not a whole number`},
		{`"n": 42`, `"n": "007"`, `"007" is not a whole number`},
		{`"d": "2026-09-30"`, `"d": "30/09/2026"`, `computation.filing_date at $.x.d`},
		{`"d": "2026-09-30"`, `"d": 20260930`, `expected a date value, found a number`},
		{`"b": true`, `"b": "yes"`, `expected a bool value, found a text`},
		{`"r": "12.5"`, `"r": "12.555"`, `not a percentage with at most two decimal places`},
		{`"r": "12.5"`, `"r": "-1"`, `not a percentage`},
	} {
		r := extract(t, p, strings.Replace(doc, c.field, c.value, 1), Source{})
		if len(r.Report.Errors) != 1 || !strings.Contains(r.Report.Errors[0].Msg, c.msg) {
			t.Errorf("%s: errors = %+v, want %q", c.value, r.Report.Errors, c.msg)
		}
	}
}

func TestMoneyOptions(t *testing.T) {
	p := load(t, strings.Replace(goodProfile, "    unit: \"rupees\"\n    required: true", "    unit: \"paise\"\n    sign: \"negate\"\n    required: true", 1))
	r := extract(t, p, strings.Replace(goodDoc, `112450.50`, `11245050`, 1), Source{})
	if r.Report.HasErrors() || r.Facts[0].Value != domain.MoneyValue(-11245050) {
		t.Fatalf("paise + negate: %+v %+v", r.Facts, r.Report.Errors)
	}
	p = load(t, strings.Replace(goodProfile, "    unit: \"rupees\"\n    required: true", "    unit: \"rupees\"\n    rounding: \"half_up_rupee\"\n    required: true", 1))
	r = extract(t, p, strings.Replace(goodDoc, `112450.50`, `112450.505`, 1), Source{})
	if r.Report.HasErrors() || r.Facts[0].Value != domain.MoneyValue(11245100) {
		t.Fatalf("rounding: %+v %+v", r.Facts, r.Report.Errors)
	}
}

func TestPeriodicityMustFit(t *testing.T) {
	text := goodProfile + "  - { fact: \"computation.monthly_turnover\", json_path: \"$.m\", unit: \"rupees\" }\n"
	p := load(t, text)
	r := extract(t, p, strings.Replace(goodDoc, `"assets"`, `"m": 5, "assets"`, 1), Source{})
	if len(r.Report.Errors) != 1 || !strings.Contains(r.Report.Errors[0].Msg, "computation.monthly_turnover is a month figure but the file covers FY2025-26") {
		t.Fatalf("errors = %+v", r.Report.Errors)
	}
}

// AC-2: no match and two matches are refused, naming the file and the profiles.
func TestDetect(t *testing.T) {
	v := testVocab(t)
	a := load(t, goodProfile)
	b, err := Load("b.yaml", []byte(strings.Replace(goodProfile, `"test.suite.computation"`, `"other.suite.computation"`, 1)), v)
	if err != nil {
		t.Fatal(err)
	}
	c, err := Load("c.yaml", []byte(strings.NewReplacer(`"test.suite.computation"`, `"test.suite.computation"`, `profile_version: "1"`, `profile_version: "2"`,
		`value: "COMPUTATION"`, `value: "COMPUTATION_V2"`).Replace(goodProfile)), v)
	if err != nil {
		t.Fatal(err)
	}
	doc := openJSON(t, goodDoc)

	one, _ := NewSet(a, c)
	got, err := Detect(one, doc, "comp.json")
	if err != nil || got != a {
		t.Fatalf("Detect = %v, %v; want %s", got, err, a.Key())
	}

	none, _ := NewSet(c)
	_, err = Detect(none, doc, "comp.json")
	var ie *ingest.Error
	if !errors.As(err, &ie) || ie.Code != ingest.ProfileNoMatch || ie.File != "comp.json" || !strings.Contains(ie.Error(), "PROFILE_NO_MATCH: comp.json: the file matches no known json format") {
		t.Fatalf("no match: %v", err)
	}

	two, _ := NewSet(a, b, c)
	_, err = Detect(two, doc, "comp.json")
	if !errors.As(err, &ie) || ie.Code != ingest.ProfileAmbiguous ||
		!strings.Contains(ie.Error(), "PROFILE_AMBIGUOUS: comp.json: the file matches 2 formats (other.suite.computation v1, test.suite.computation v1)") {
		t.Fatalf("ambiguous: %v", err)
	}

	if _, err := NewSet(a, a); err == nil || !strings.Contains(err.Error(), "test.suite.computation v1 is defined twice") {
		t.Fatalf("duplicate profile: %v", err)
	}
}

func TestDetectPredicates(t *testing.T) {
	text := strings.Replace(goodProfile, `  - json_exists: { path: "$.meta.tax_year" }`, `  - json_matches: { path: "$.meta.tax_year", pattern: "20[0-9]{2}-[0-9]{2}" }`, 1)
	p := load(t, text)
	for doc, want := range map[string]bool{
		goodDoc: true,
		strings.Replace(goodDoc, `"2025-26"`, `"FY2025-26"`, 1):                         false, // the pattern is anchored at both ends
		strings.Replace(goodDoc, `"COMPUTATION"`, `"COMPUTATION "`, 1):                  false, // equals is exact
		strings.Replace(goodDoc, `"form": "COMPUTATION"`, `"form": ["COMPUTATION"]`, 1): false,
	} {
		if got, err := p.Matches(openJSON(t, doc)); err != nil || got != want {
			t.Errorf("Matches = %v, %v; want %v for %s", got, err, want, doc[:80])
		}
	}
}

// The shipped profiles load, sit in the right folders, and pass their sample tests (IN-22,
// AC-5). This runs in CI, so a broken profile or sample can never be merged.
func TestShippedProfiles(t *testing.T) {
	root := filepath.Join("..", "..")
	v, err := vocab.LoadDir(filepath.Join(root, "content", "vocabulary"))
	if err != nil {
		t.Fatal(err)
	}
	set, err := LoadDir(filepath.Join(root, "content", "profiles"), v)
	if err != nil {
		t.Fatal(err)
	}
	reg := ingest.NewRegistry()
	if err := reg.Register(jsonadapter.New(0)); err != nil {
		t.Fatal(err)
	}
	var keys []string
	samples := map[string]int{}
	for _, p := range set.Profiles() {
		keys = append(keys, p.Key())
		for _, r := range TestSamples(p, set, reg) {
			samples[p.ID]++
			for _, pr := range r.Problems {
				t.Errorf("%s %s: %s", r.Profile, r.Sample, pr)
			}
		}
	}
	// AC-7: version 2 of the computation format is a second folder beside version 1.
	want := []string{"generic.taxsuite.computation v1", "generic.taxsuite.computation v2", "generic.taxsuite.tax_audit_report v1"}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("profiles = %v, want %v", keys, want)
	}
	// IN-26: each generic profile has at least two samples.
	for id, n := range samples {
		if n < 2 {
			t.Errorf("%s has %d samples; at least two are required", id, n)
		}
	}
}

// AC-5: editing one expected value makes the sample fail, naming the fact.
func TestSampleFailureNamesTheFact(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join("..", "..", "content", "profiles", "generic", "tax_audit_report", "v1")
	copyTree(t, src, filepath.Join(dir, "generic", "tax_audit_report", "v1"))
	exp := filepath.Join(dir, "generic", "tax_audit_report", "v1", "expected", "tar_2025-26_agrees.facts.yaml")
	data, _ := os.ReadFile(exp)
	if err := os.WriteFile(exp, []byte(strings.Replace(string(data), `value: "112450.50"`, `value: "112450.00"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err := vocab.LoadDir(filepath.Join("..", "..", "content", "vocabulary"))
	if err != nil {
		t.Fatal(err)
	}
	set, err := LoadDir(dir, v)
	if err != nil {
		t.Fatal(err)
	}
	reg := ingest.NewRegistry()
	_ = reg.Register(jsonadapter.New(0))
	var problems []string
	for _, r := range TestSamples(set.Profiles()[0], set, reg) {
		problems = append(problems, r.Problems...)
	}
	want := "fact tar.depreciation_schedule.total: expected value 112450.00, period FY2025-26, at $.clause_18.total_depreciation, got value 112450.50, period FY2025-26, at $.clause_18.total_depreciation"
	if !reflect.DeepEqual(problems, []string{want}) {
		t.Fatalf("problems = %q\nwant [%q]", problems, want)
	}
}

func TestSampleProblems(t *testing.T) {
	dir := t.TempDir()
	pdir := filepath.Join(dir, "generic", "tax_audit_report", "v1")
	copyTree(t, filepath.Join("..", "..", "content", "profiles", "generic", "tax_audit_report", "v1"), pdir)
	// An expected file without a sample, and a sample without an expected file.
	if err := os.Rename(filepath.Join(pdir, "samples", "tar_2024-25_differs.json"), filepath.Join(pdir, "samples", "renamed.json")); err != nil {
		t.Fatal(err)
	}
	v, _ := vocab.LoadDir(filepath.Join("..", "..", "content", "vocabulary"))
	set, err := LoadDir(dir, v)
	if err != nil {
		t.Fatal(err)
	}
	reg := ingest.NewRegistry()
	_ = reg.Register(jsonadapter.New(0))
	var all string
	for _, r := range TestSamples(set.Profiles()[0], set, reg) {
		all += strings.Join(r.Problems, "\n") + "\n"
	}
	for _, want := range []string{"no expected file", "expected file has no sample of the same name"} {
		if !strings.Contains(all, want) {
			t.Errorf("problems do not mention %q:\n%s", want, all)
		}
	}
	// A profile in the wrong folder is refused.
	if err := os.Rename(filepath.Join(dir, "generic", "tax_audit_report"), filepath.Join(dir, "generic", "tar")); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDir(dir, v); err == nil || !strings.Contains(err.Error(), "a profile must sit at <source>/tax_audit_report/v1/profile.yaml") {
		t.Errorf("wrong folder: %v", err)
	}
}

func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(to, rel), 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(to, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
