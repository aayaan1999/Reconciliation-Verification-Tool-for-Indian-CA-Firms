package findings

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// sample is a report that uses every part of the wording: a file with an error and a
// warning, a finding in whole rupees, and a Cannot check. Illustrative figures only.
func sample(t *testing.T) *Report {
	t.Helper()
	root := filepath.Join("..", "..", "content")
	v, err := vocab.LoadDir(filepath.Join(root, "vocabulary"))
	if err != nil {
		t.Fatal(err)
	}
	set, err := rules.LoadDir(filepath.Join(root, "rules"), v, checks.All())
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]*rules.Rule{}
	for _, r := range set.Rules() {
		byID[r.ID] = r
	}
	loc := func(file, path string) domain.Locator {
		return domain.Locator{DocumentVersionID: 7, FileName: file, Kind: domain.LocatorJSONPath, Path: path}
	}
	law := byID["RECON-DEP-002"].Law
	return &Report{
		TaxYear: "2025-26", IncludesTesting: true, Rules: byID, Vocab: v,
		Files: []ingest.ValidationReport{{
			File: "tar.json", FileType: "json", ProfileID: "p", ProfileVersion: "1", SourceSystem: "Generic",
			Errors:   []ingest.Issue{{Code: ingest.RequiredFieldMissing, Msg: "required figure x is not in the file"}},
			Warnings: []ingest.Issue{{Code: ingest.ParseError, Msg: "the PAN in the file is not in the usual form"}},
		}},
		Evaluations: []domain.Evaluation{
			{RuleID: "RECON-DEP-001", RuleVersion: "1", Subject: domain.Subject{Period: "FY2025-26"}, Status: domain.StatusCannotCheck,
				MissingFacts: []string{"computation.fixed_asset_schedule.total"}, LawRefs: law},
			{RuleID: "RECON-DEP-002", RuleVersion: "1", Subject: domain.Subject{Period: "FY2025-26"}, Status: domain.StatusFail,
				Difference: 275000, LawRefs: law, Values: []domain.LabeledValue{
					{Input: "a", Label: "Tax audit report, depreciation schedule total", Value: domain.MoneyValue(11245000), Locator: loc("TAR_v3.json", "$.clause_18.total_depreciation")},
					{Input: "b", Label: `Computation, "Allowed depreciation"`, Value: domain.MoneyValue(11520000), Locator: loc("comp.json", "$.business_income.allowed_depreciation")},
				}},
		},
	}
}

// The §B4.6 layout: both figures, the difference, both sources and the rule basis, in
// Indian digit grouping, with no banned phrase anywhere (CLAUDE.md invariant 1).
func TestTextWording(t *testing.T) {
	r := sample(t)
	text, err := r.Text()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		TestingNotice,
		"Tax year 2025-26: 1 file read, 2 checks: 0 pass, 1 fail, 1 cannot check",
		"Tax audit report, depreciation schedule total : 1,12,450",
		`Computation, "Allowed depreciation"           : 1,15,200`,
		"Difference                                    :    2,750",
		"Source: TAR_v3.json $.clause_18.total_depreciation | comp.json $.business_income.allowed_depreciation",
		"Rule basis: Income-tax Act, 1961 – Sec 32",
		"Cannot check: figure not found in Computation of total income prepared in the tax suite",
		"The figures of this file are not used, because it has errors.",
	} {
		if !strings.Contains(text, s) {
			t.Errorf("report lacks %q:\n%s", s, text)
		}
	}
	js, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, out := range []string{text, string(js)} {
		if b := rules.BannedIn(out); len(b) > 0 {
			t.Errorf("banned phrases %q in:\n%s", b, out)
		}
	}
	if strings.Contains(string(js), `"document_version_id":7`) {
		t.Error("the JSON report contains a database ID")
	}
}

// A stored validation report reads back into the same JSON (SPEC-004 part 7).
func TestValidationReportRoundTrip(t *testing.T) {
	r := sample(t).Files[0]
	r.Unread = []ingest.Unread{{Locator: domain.Locator{Path: "$.a"}, Kind: ingest.UnreadOther, Reason: "text; this format does not read this part of the file"}}
	b, err := MarshalValidationReport(r)
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseValidationReport(b)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := MarshalValidationReport(back)
	if err != nil || string(b2) != string(b) {
		t.Errorf("round trip changed the report:\n%s\n%s", b, b2)
	}
}
