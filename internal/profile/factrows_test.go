package profile

import (
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
)

// SPEC-006 part 2: a figure read from the entries of a list that meet its conditions, never
// from a position in the list.

const listJSON = `{
  "meta": { "ty": "2025-26", "ay": "2026-27" },
  "tds": [ { "section": "192", "paid": 1000 }, { "section": "194J", "paid": 7480000 }, { "section": "194C", "paid": 500 } ],
  "dup": [ { "section": "194J", "paid": 1 }, { "section": "194J", "paid": 2 } ],
  "credit": [ { "ay": "2025-26", "extra": 10 }, { "ay": "2026-27", "extra": 755049 } ],
  "months": [ { "due": "2025-05-15", "amount": 100 }, { "due": "2026-04-15", "amount": 34180 }, { "due": "2025-06-15", "amount": 200 } ],
  "tie": [ { "due": "2026-04-15", "amount": 1 }, { "due": "2026-04-15", "amount": 2 } ],
  "items": [ { "nature": "Personal expenditure", "amount": 964210 }, { "nature": "Other", "amount": 7 }, { "nature": "Personal expenditure", "amount": 4180 } ]
}`

// listProfile reads one figure, computation.allowed_depreciation, with the given rows block.
func listProfile(block string) string {
	return `schema: "recon.profile/1"
profile_id: "test.list"
profile_version: "1"
source_system: "Test"
document_type: "computation"
file_type: "json"
regime: "ITA1961"
detect:
  - json_exists: { path: "$.meta.ty" }
context:
  tax_year: { json_path: "$.meta.ty" }
  period: { const: "FY" }
facts:
  - fact: "computation.allowed_depreciation"
    unit: "rupees"
` + block
}

func TestFigureFromListEntries(t *testing.T) {
	for _, c := range []struct {
		name, block string
		want        money.Paise // -1: no figure
		locator     string
		warn        bool
	}{
		{"the entry meeting the condition", `    rows: "$.tds[*]"
    where: [ { path: ".section", equals: "194J" } ]
    json_path: ".paid"
    combine: "one"
`, 748000000, "$.tds[1].paid", false},
		{"a condition on another place of the file", `    rows: "$.credit[*]"
    where: [ { path: ".ay", equals_path: "$.meta.ay" } ]
    json_path: ".extra"
    combine: "one"
`, 75504900, "$.credit[1].extra", false},
		{"two entries meet it: no figure, a warning", `    rows: "$.dup[*]"
    where: [ { path: ".section", equals: "194J" } ]
    json_path: ".paid"
    combine: "one"
`, -1, "", true},
		{"no entry meets it: no figure", `    rows: "$.tds[*]"
    where: [ { path: ".section", equals: "194Q" } ]
    json_path: ".paid"
    combine: "one"
`, -1, "", false},
		{"the entries added up", `    rows: "$.items[*]"
    where: [ { path: ".nature", equals: "Personal expenditure" } ]
    json_path: ".amount"
    combine: "sum"
`, 96839000, "$.items[*].amount (sum of 2 entries)", false},
		{"the entries meeting one of several values, added up (SPEC-007)", `    rows: "$.items[*]"
    where: [ { path: ".nature", in: ["Personal expenditure", "Other"] } ]
    json_path: ".amount"
    combine: "sum"
`, 96839700, "$.items[*].amount (sum of 3 entries)", false},
		{"the latest due date", `    rows: "$.months[*]"
    json_path: ".amount"
    combine: "latest"
    by: ".due"
`, 3418000, "$.months[1].amount", false},
		{"two entries share the latest date: no figure, a warning", `    rows: "$.tie[*]"
    json_path: ".amount"
    combine: "latest"
    by: ".due"
`, -1, "", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			res := extract(t, load(t, listProfile(c.block)), listJSON, Source{FileName: "l.json"})
			if len(res.Report.Errors) > 0 {
				t.Fatalf("errors: %v", res.Report.Errors)
			}
			warned := len(res.Report.Warnings) == 1 && res.Report.Warnings[0].Code == ingest.EntryAmbiguous
			if warned != c.warn {
				t.Errorf("warnings %v, want an ENTRY_AMBIGUOUS warning: %v", res.Report.Warnings, c.warn)
			}
			switch {
			case c.want < 0 && len(res.Facts) != 0:
				t.Errorf("a figure was read: %v", res.Facts)
			case c.want >= 0 && (len(res.Facts) != 1 || res.Facts[0].Value.Money != c.want || res.Facts[0].Locator.Path != c.locator):
				t.Errorf("facts %+v, want %d paise at %s", res.Facts, c.want, c.locator)
			}
		})
	}
}

func TestListFigureRefusals(t *testing.T) {
	v := testVocab(t)
	for name, block := range map[string]string{
		"no combine": `    rows: "$.tds[*]"
    json_path: ".paid"
`,
		"rows that select one place": `    rows: "$.tds[0]"
    json_path: ".paid"
    combine: "one"
`,
		"a path not written from the entry": `    rows: "$.tds[*]"
    json_path: "$.tds[0].paid"
    combine: "one"
`,
		"a condition with both equals and equals_path": `    rows: "$.tds[*]"
    where: [ { path: ".section", equals: "194J", equals_path: "$.meta.ay" } ]
    json_path: ".paid"
    combine: "one"
`,
		"a condition with both equals and in": `    rows: "$.tds[*]"
    where: [ { path: ".section", equals: "194J", in: ["192", "194J"] } ]
    json_path: ".paid"
    combine: "one"
`,
		"in with one value": `    rows: "$.tds[*]"
    where: [ { path: ".section", in: ["194J"] } ]
    json_path: ".paid"
    combine: "one"
`,
		"in with an empty value": `    rows: "$.tds[*]"
    where: [ { path: ".section", in: ["194J", ""] } ]
    json_path: ".paid"
    combine: "one"
`,
		"latest without by": `    rows: "$.months[*]"
    json_path: ".amount"
    combine: "latest"
`,
		"by without latest": `    rows: "$.months[*]"
    json_path: ".amount"
    combine: "one"
    by: ".due"
`,
		"where without rows": `    json_path: "$.tds[1].paid"
    where: [ { path: ".section", equals: "194J" } ]
`,
	} {
		if _, err := Load("profile.yaml", []byte(listProfile(block)), v); err == nil {
			t.Errorf("%s: loaded", name)
		}
	}
	// sum adds up money only.
	text := strings.Replace(listProfile(`    rows: "$.items[*]"
    json_path: ".nature"
    combine: "sum"
`), `fact: "computation.allowed_depreciation"
    unit: "rupees"`, `fact: "computation.assessee_name"`, 1)
	if _, err := Load("profile.yaml", []byte(text), v); err == nil {
		t.Error("sum of a text figure loaded")
	}
}
