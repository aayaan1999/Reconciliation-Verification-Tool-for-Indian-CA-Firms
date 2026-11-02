package profile

import (
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	jsonadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/json"
)

// Fixes from the code review of 28 October 2026.

// combine "latest" chooses by real dates: dates not written yyyy-mm-dd, or an entry with no
// date, give no figure and a warning, never a guessed entry.
func TestLatestNeedsRealDates(t *testing.T) {
	const doc = `{
  "meta": { "ty": "2025-26" },
  "dmy": [ { "due": "15-06-2025", "amount": 1 }, { "due": "01-03-2026", "amount": 2 } ],
  "nodate": [ { "due": "2026-04-15", "amount": 1 }, { "amount": 2 } ],
  "iso": [ { "due": "2025-06-15", "amount": 1 }, { "due": "2026-03-01", "amount": 2 } ]
}`
	read := func(list string) *Result {
		t.Helper()
		return extract(t, load(t, listProfile(`    rows: "$.`+list+`[*]"
    json_path: ".amount"
    combine: "latest"
    by: ".due"
`)), doc, Source{FileName: "l.json"})
	}
	for _, list := range []string{"dmy", "nodate"} {
		res := read(list)
		if len(res.Facts) != 0 || len(res.Report.Errors) != 0 || len(res.Report.Warnings) != 1 || res.Report.Warnings[0].Code != ingest.EntryAmbiguous {
			t.Errorf("%s: facts %v, errors %v, warnings %v; want no figure and one warning", list, res.Facts, res.Report.Errors, res.Report.Warnings)
		}
	}
	if res := read("iso"); len(res.Facts) != 1 || res.Facts[0].Value.Money != 200 {
		t.Errorf("iso: facts %+v, want the 2026 entry (200 paise)", res.Facts)
	}
}

// A period in the file that is not in its format is one error, not also "missing".
func TestUnreadablePeriodIsOneError(t *testing.T) {
	for _, ctx := range []string{
		"  tax_year: { from_period: true }\n  period: { json_path: \"$.meta.p\", format: \"mmyyyy\" }\n",
		"  tax_year: { json_path: \"$.meta.ty\" }\n  period: { json_path: \"$.meta.p\", format: \"mmyyyy\" }\n",
	} {
		text := strings.Replace(listProfile("    json_path: \"$.x\"\n"),
			"  tax_year: { json_path: \"$.meta.ty\" }\n  period: { const: \"FY\" }\n", ctx, 1)
		res := extract(t, load(t, text), `{"meta": {"ty": "2026-27", "p": "8-2026"}, "x": 1}`, Source{FileName: "p.json"})
		if len(res.Report.Errors) != 1 || res.Report.Errors[0].Code != ingest.ParseError {
			t.Errorf("errors %v, want one PARSE_ERROR", res.Report.Errors)
		}
	}
}

// A field joined from several values escapes its separator, so {"A,B"} and {"A", "B"} are
// different fields and different match keys.
func TestJoinedFieldsAreEscaped(t *testing.T) {
	p, err := Load("profile.yaml", []byte(`schema: "recon.profile/1"
profile_id: "test.books.groups"
profile_version: "1"
source_system: "Test"
document_type: "ledger"
file_type: "json"
regime: "ITA1961"
detect:
  - json_exists: { path: "$.rows" }
context:
  tax_year: { json_path: "$.rows[0].month", from_month: true }
  period: { const: "FY" }
records:
  - record_type: "ledger_month"
    rows: "$.rows[*]"
    fields:
      - { name: "ledger_head", path: ".head", required: true }
      - { name: "group", path: ".groups[*]" }
      - { name: "month", path: ".month", required: true }
`), realVocab(t))
	if err != nil {
		t.Fatal(err)
	}
	doc := `{"rows": [ {"head": "One", "month": "2025-04", "groups": ["A,B"]}, {"head": "Two", "month": "2025-04", "groups": ["B", "A"]} ]}`
	d, err := jsonadapter.New(0).Open(strings.NewReader(doc), int64(len(doc)))
	if err != nil {
		t.Fatal(err)
	}
	res, err := Extract(p, d, Source{FileName: "g.json"})
	if err != nil || res.Report.HasErrors() || len(res.Records) != 2 {
		t.Fatalf("err %v, report %v, %d records", err, res.Report.Errors, len(res.Records))
	}
	one, two := res.Records[0].Fields["group"], res.Records[1].Fields["group"]
	if one != `A\,B` || two != "A,B" {
		t.Errorf("joined fields %q and %q, want %q and %q", one, two, `A\,B`, "A,B")
	}
}

// ^ goes up only from a row that is a list entry; a quoted key is not one.
func TestParentPath(t *testing.T) {
	for _, c := range []struct{ path, want string }{
		{"$.b2b[0].inv[3]", "$.b2b[0]"},
		{"$['Sales Register'][3]", "$"},
		{"$.a['x y'][12]", "$.a"},
	} {
		if got, err := parentPath(c.path, 1); err != nil || got != c.want {
			t.Errorf("parentPath(%q) = %q, %v; want %q", c.path, got, err, c.want)
		}
	}
	for _, bad := range []string{"$.data['k']", "$.data", "$[]", "$"} {
		if got, err := parentPath(bad, 1); err == nil {
			t.Errorf("parentPath(%q) = %q, want an error", bad, got)
		}
	}
}

// Fixes from the second code review of 28 October 2026.

// "latest" takes the latest entry or nothing: when the latest entry has no figure, an older
// entry's figure is not taken in its place. A tie names how many entries share the date.
func TestLatestEntryWithoutFigure(t *testing.T) {
	const doc = `{
  "meta": { "ty": "2025-26" },
  "gap": [ { "due": "2026-03-01", "amount": null }, { "due": "2025-06-15", "amount": 5000 } ],
  "tie": [ { "due": "2026-03-01", "amount": 1 }, { "due": "2026-03-01", "amount": 2 }, { "due": "2026-03-01", "amount": 3 } ]
}`
	read := func(list, extra string) *Result {
		t.Helper()
		return extract(t, load(t, listProfile(extra+`    rows: "$.`+list+`[*]"
    json_path: ".amount"
    combine: "latest"
    by: ".due"
`)), doc, Source{FileName: "l.json"})
	}
	res := read("gap", "")
	if len(res.Facts) != 0 || len(res.Report.Warnings) != 1 || !strings.Contains(res.Report.Warnings[0].Msg, "dated 2026-03-01") {
		t.Errorf("gap: facts %v, warnings %v; want no figure and one warning", res.Facts, res.Report.Warnings)
	}
	if res := read("tie", ""); len(res.Report.Warnings) != 1 || !strings.Contains(res.Report.Warnings[0].Msg, "3 entries") {
		t.Errorf("tie: warnings %v, want one naming 3 entries", res.Report.Warnings)
	}
	// A required figure that cannot be read without guessing is missing: an error.
	for _, list := range []string{"gap", "tie"} {
		res := read(list, "    required: true\n")
		if len(res.Facts) != 0 || len(res.Report.Errors) != 1 || res.Report.Errors[0].Code != ingest.RequiredFieldMissing || len(res.Report.Warnings) != 0 {
			t.Errorf("%s required: errors %v, warnings %v; want one REQUIRED_FIELD_MISSING", list, res.Report.Errors, res.Report.Warnings)
		}
	}
}

// Messages name places, never the client's values (security-and-privacy.md).
func TestMessagesHoldNoClientValues(t *testing.T) {
	gstin := strings.Replace(listProfile("    json_path: \"$.x\"\n"), "  period: { const: \"FY\" }\n",
		"  period: { const: \"FY\" }\n  pan: { json_path: \"$.meta.gstin\", from_gstin: true }\n", 1)
	res := extract(t, load(t, gstin), `{"meta": {"ty": "2025-26", "gstin": "27ZZZZZ9999Z1Z"}, "x": 1}`, Source{FileName: "g.json"})
	if len(res.Report.Warnings) != 1 || strings.Contains(res.Report.Warnings[0].Msg, "ZZZZZ") {
		t.Errorf("warnings %v, want one that does not quote the GSTIN", res.Report.Warnings)
	}

	const doc = `{"meta": {"ty": "2025-26", "ay": "2026-27"},
  "credit": [ { "ay": "2026-27", "kind": "A", "m": "2025-04" }, { "ay": "2024-25", "kind": "SECRETVAL", "m": "2025-05" } ]}`
	p, err := Load("profile.yaml", []byte(`schema: "recon.profile/1"
profile_id: "test.rows"
profile_version: "1"
source_system: "Test"
document_type: "ledger"
file_type: "json"
regime: "ITA1961"
detect:
  - json_exists: { path: "$.meta.ty" }
context:
  tax_year: { json_path: "$.meta.ty" }
  period: { const: "FY" }
records:
  - record_type: "ledger_month"
    rows: "$.credit[*]"
    where: [ { path: ".ay", equals_path: "$.meta.ay" }, { path: ".kind", equals: "A" } ]
    fields:
      - { name: "ledger_head", path: ".kind", required: true }
      - { name: "month", path: ".m", required: true }
`), realVocab(t))
	if err != nil {
		t.Fatal(err)
	}
	res = extract(t, p, doc, Source{FileName: "c.json"})
	left := 0
	for _, u := range res.Report.Unread {
		if u.Reason == "row left out: .ay is another value; this format reads only rows where .ay is the value at $.meta.ay" {
			left++
		}
		if u.Locator.Path == "$.meta.ay" {
			t.Error("the place an equals_path condition compares with is listed as unread")
		}
		if strings.Contains(u.Reason, "2024-25") || strings.Contains(u.Reason, "2026-27") || strings.Contains(u.Reason, "SECRETVAL") {
			t.Errorf("reason %q quotes a value of the file", u.Reason)
		}
	}
	if left == 0 {
		t.Errorf("unread %+v; want the 2024-25 row left out, naming the place compared with", res.Report.Unread)
	}
}

// A fixed period in a format is checked when the profile loads.
func TestFixedPeriodFormatCheckedAtLoad(t *testing.T) {
	for _, c := range []struct {
		period string
		ok     bool
	}{{`{ const: "082026", format: "mmyyyy" }`, true}, {`{ const: "132026", format: "mmyyyy" }`, false}, {`{ const: "8-2026", format: "mmyyyy" }`, false}} {
		text := strings.Replace(listProfile("    json_path: \"$.x\"\n"), `{ const: "FY" }`, c.period, 1)
		text = strings.Replace(text, `{ json_path: "$.meta.ty" }`, `{ from_period: true }`, 1)
		if _, err := Load("profile.yaml", []byte(text), testVocab(t)); (err == nil) != c.ok {
			t.Errorf("%s: err %v, want ok %v", c.period, err, c.ok)
		}
	}
}
