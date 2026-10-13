package domain

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"
)

func sortedNames[V any](m map[string]V) []string { return slices.Sorted(maps.Keys(m)) }

func mustCanonical(t *testing.T, e Evaluation) string {
	t.Helper()
	b, err := e.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// AC-4: the same Evaluation built in two different field orders encodes byte-identically.
func TestEvaluationCanonicalJSONIgnoresFieldOrder(t *testing.T) {
	first := Evaluation{
		RunID:       12,
		RuleID:      "RECON-DEP-002",
		RuleVersion: "1",
		Subject:     Subject{Period: "FY2025-26", Scope: "", MatchKey: ""},
		Status:      StatusFail,
		Values: []LabeledValue{
			{Input: "a", Label: "Tax audit report, depreciation schedule total", Value: MoneyValue(11245000),
				Locator: Locator{DocumentVersionID: 3, FileName: "TAR_v3.pdf", Kind: LocatorPDFPos, Path: "p7:l23:c41-52"}},
			{Input: "b", Label: `Computation, "Allowed depreciation"`, Value: MoneyValue(11520000),
				Locator: Locator{DocumentVersionID: 4, FileName: "comp_20261008.json", Kind: LocatorJSONPath, Path: "$.business_income.allowed_depreciation"}},
		},
		Difference:   275000,
		MissingFacts: []string{"tar.b", "tar.a"},
		LawRefs:      []LawRef{{Act: "Income-tax Act, 1961", Section: "32", Reference: ""}},
	}
	second := Evaluation{
		LawRefs:      []LawRef{{Reference: "", Section: "32", Act: "Income-tax Act, 1961"}},
		MissingFacts: []string{"tar.a", "tar.b"}, // a set: order is not meaningful
		Difference:   275000,
		Values: []LabeledValue{
			{Locator: Locator{Path: "p7:l23:c41-52", Kind: LocatorPDFPos, FileName: "TAR_v3.pdf", DocumentVersionID: 3},
				Value: Value{Money: 11245000, Kind: KindMoney}, Label: "Tax audit report, depreciation schedule total", Input: "a"},
			{Locator: Locator{Path: "$.business_income.allowed_depreciation", Kind: LocatorJSONPath, FileName: "comp_20261008.json", DocumentVersionID: 4},
				Value: Value{Money: 11520000, Kind: KindMoney}, Label: `Computation, "Allowed depreciation"`, Input: "b"},
		},
		Status:      StatusFail,
		Subject:     Subject{MatchKey: "", Scope: "", Period: "FY2025-26"},
		RuleVersion: "1",
		RuleID:      "RECON-DEP-002",
		RunID:       12,
	}
	// A third copy decoded from JSON whose keys arrive in yet another order.
	var third Evaluation
	src := `{"Status":2,"Values":[
		{"Locator":{"Path":"p7:l23:c41-52","DocumentVersionID":3,"Kind":"pdf_pos","FileName":"TAR_v3.pdf"},"Kind":1,"Money":11245000,"Input":"a","Label":"Tax audit report, depreciation schedule total"},
		{"Label":"Computation, \"Allowed depreciation\"","Input":"b","Money":11520000,"Kind":1,"Locator":{"FileName":"comp_20261008.json","Kind":"json_path","DocumentVersionID":4,"Path":"$.business_income.allowed_depreciation"}}],
		"LawRefs":[{"Section":"32","Act":"Income-tax Act, 1961"}],"Difference":275000,"MissingFacts":["tar.b","tar.a"],
		"Subject":{"Period":"FY2025-26"},"RuleVersion":"1","RuleID":"RECON-DEP-002","RunID":12}`
	if err := json.Unmarshal([]byte(src), &third); err != nil {
		t.Fatal(err)
	}

	a, b, c := mustCanonical(t, first), mustCanonical(t, second), mustCanonical(t, third)
	if a != b || a != c {
		t.Fatalf("encodings differ:\n%s\n%s\n%s", a, b, c)
	}
}

// The exact bytes are fixed so a change to the format is a deliberate, reviewed change:
// golden files written by later specs depend on them.
func TestEvaluationCanonicalJSONGolden(t *testing.T) {
	e := Evaluation{
		RunID:       1,
		RuleID:      "RECON-DEP-002",
		RuleVersion: "1",
		Subject:     Subject{Period: "FY2025-26"},
		Status:      StatusFail,
		Values: []LabeledValue{
			{Input: "a", Label: "TAR <total> & more", Value: MoneyValue(11245000),
				Locator: Locator{DocumentVersionID: 3, FileName: "TAR_v3.pdf", Kind: LocatorPDFPos, Path: "p7:l23:c41-52"}},
		},
		Difference: -275000,
	}
	want := `{"difference":-275000,"law_refs":[],"missing_facts":[],"rule_id":"RECON-DEP-002","rule_version":"1","run_id":1,` +
		`"status":"fail","subject":{"match_key":"","period":"FY2025-26","scope":""},` +
		`"values":[{"input":"a","label":"TAR <total> & more","locator":{"document_version_id":3,"file_name":"TAR_v3.pdf","kind":"pdf_pos","path":"p7:l23:c41-52"},` +
		`"value":{"kind":"money","paise":11245000}}]}`
	if got := mustCanonical(t, e); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestCanonicalValueEncodings(t *testing.T) {
	cases := []struct {
		v    Value
		want string
	}{
		{MoneyValue(-50), `{"kind":"money","paise":-50}`},
		{IntValue(3), `{"int":3,"kind":"int"}`},
		{TextValue("194C"), `{"kind":"text","text":"194C"}`},
		{DateValue("2025-07-31"), `{"date":"2025-07-31","kind":"date"}`},
		{BoolValue(false), `{"bool":false,"kind":"bool"}`},
		{RateValue(1800), `{"kind":"rate","rate_bp":1800}`},
		{Value{Kind: KindMoney, Money: 1, Text: "ignored", RateBP: 9}, `{"kind":"money","paise":1}`},
	}
	for _, c := range cases {
		e := Evaluation{Status: StatusPass, Values: []LabeledValue{{Value: c.v}}}
		got := mustCanonical(t, e)
		if !strings.Contains(got, `"value":`+c.want+`}`) {
			t.Errorf("%s value: encoding %s does not contain %s", c.v.Kind, got, c.want)
		}
	}
}

func TestCanonicalJSONNilAndEmptyListsMatch(t *testing.T) {
	withNil := Evaluation{Status: StatusPass}
	withEmpty := Evaluation{Status: StatusPass, Values: []LabeledValue{}, MissingFacts: []string{}, LawRefs: []LawRef{}}
	if a, b := mustCanonical(t, withNil), mustCanonical(t, withEmpty); a != b {
		t.Errorf("nil and empty lists encode differently:\n%s\n%s", a, b)
	}
}

func TestCanonicalJSONRefusesInvalidInput(t *testing.T) {
	cases := map[string]Evaluation{
		"zero status":    {},
		"unknown status": {Status: 9},
		"value no kind":  {Status: StatusFail, Values: []LabeledValue{{Input: "a"}}},
		"bad UTF-8 rule": {Status: StatusFail, RuleID: "R\xff"},
		"bad UTF-8 text": {Status: StatusFail, Values: []LabeledValue{{Value: TextValue("a\xffb")}}},
		"bad UTF-8 path": {Status: StatusFail, Values: []LabeledValue{{Value: IntValue(1), Locator: Locator{Path: "\xc3"}}}},
	}
	for _, name := range sortedNames(cases) {
		if _, err := cases[name].CanonicalJSON(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestFindingCanonicalJSON(t *testing.T) {
	ev := Evaluation{RuleID: "RECON-DEP-002", RuleVersion: "1", Subject: Subject{Period: "FY2025-26"}, Status: StatusFail,
		Values: []LabeledValue{{Input: "a", Value: MoneyValue(100)}}}
	f := Finding{StableID: StableID(ev.RuleID, 42, ev.Subject), State: StateOpen, ReviewDocument: "computation", LatestEvaluation: ev}
	var err error
	if f.ValueFingerprint, err = ValueFingerprint(ev.Values); err != nil {
		t.Fatal(err)
	}
	got, err := f.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	evJSON := mustCanonical(t, ev)
	want := `{"latest_evaluation":` + evJSON + `,"review_document":"computation","stable_id":"` + f.StableID +
		`","state":"open","value_fingerprint":"` + f.ValueFingerprint + `"}`
	if string(got) != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}

	for _, bad := range []FindingState{"", "withdrawn", "Open"} {
		f.State = bad
		if _, err := f.CanonicalJSON(); err == nil {
			t.Errorf("state %q accepted", bad)
		}
	}
	f.State = StateOpen
	f.LatestEvaluation.Status = 0
	if _, err := f.CanonicalJSON(); err == nil {
		t.Error("finding with an invalid evaluation accepted")
	}
}
