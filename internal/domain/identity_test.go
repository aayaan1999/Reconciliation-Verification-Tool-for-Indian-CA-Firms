package domain

import (
	"regexp"
	"testing"
)

// AC-2: StableID ignores rule version, run ID and locators, and changes with rule ID,
// engagement, period, scope or match key.
func TestStableID(t *testing.T) {
	base := Evaluation{
		RunID:       7,
		RuleID:      "RECON-DEP-002",
		RuleVersion: "1",
		Subject:     Subject{Period: "FY2025-26", Scope: "", MatchKey: ""},
		Values: []LabeledValue{
			{Input: "a", Value: MoneyValue(11245000), Locator: Locator{DocumentVersionID: 3, FileName: "TAR_v3.pdf", Kind: LocatorPDFPos, Path: "p7:l23:c41-52"}},
		},
	}
	const engagement = 42
	id := func(e Evaluation, eng int64) string { return StableID(e.RuleID, eng, e.Subject) }
	want := id(base, engagement)

	if !regexp.MustCompile(`^[A-Z2-7]{16}$`).MatchString(want) {
		t.Fatalf("StableID = %q, want 16 characters of base32", want)
	}

	same := []struct {
		name   string
		change func(*Evaluation)
	}{
		{"rule version", func(e *Evaluation) { e.RuleVersion = "2-office.1" }},
		{"run ID", func(e *Evaluation) { e.RunID = 8 }},
		{"locator", func(e *Evaluation) {
			e.Values = []LabeledValue{{Input: "a", Value: MoneyValue(11245000), Locator: Locator{DocumentVersionID: 9, FileName: "TAR_v4.pdf", Kind: LocatorPDFPos, Path: "p8:l2:c1-9"}}}
		}},
		{"values", func(e *Evaluation) { e.Values = nil }},
	}
	for _, tc := range same {
		t.Run("unchanged by "+tc.name, func(t *testing.T) {
			e := base
			tc.change(&e)
			if got := id(e, engagement); got != want {
				t.Errorf("StableID changed to %q, want %q", got, want)
			}
		})
	}

	differ := []struct {
		name   string
		change func(*Evaluation, *int64)
	}{
		{"rule ID", func(e *Evaluation, _ *int64) { e.RuleID = "RECON-DEP-001" }},
		{"engagement", func(_ *Evaluation, g *int64) { *g = 43 }},
		{"period", func(e *Evaluation, _ *int64) { e.Subject.Period = "FY2024-25" }},
		{"scope", func(e *Evaluation, _ *int64) { e.Subject.Scope = "27ABCDE1234F1Z5" }},
		{"match key", func(e *Evaluation, _ *int64) { e.Subject.MatchKey = "INV-0042" }},
	}
	for _, tc := range differ {
		t.Run("changed by "+tc.name, func(t *testing.T) {
			e, eng := base, int64(engagement)
			tc.change(&e, &eng)
			if got := id(e, eng); got == want {
				t.Errorf("StableID stayed %q", got)
			}
		})
	}
}

// Moving a "|" between fields must never give the same key or ID.
func TestSubjectKeyIsUnambiguous(t *testing.T) {
	pairs := [][2]Subject{
		{{Period: "FY2025-26", Scope: "a|b", MatchKey: "c"}, {Period: "FY2025-26", Scope: "a", MatchKey: "b|c"}},
		{{Period: "x|", Scope: "", MatchKey: ""}, {Period: "x", Scope: "|", MatchKey: ""}},
		{{Period: `p\`, Scope: "s", MatchKey: ""}, {Period: "p", Scope: `\s`, MatchKey: ""}},
	}
	for _, p := range pairs {
		if p[0].Key() == p[1].Key() {
			t.Errorf("subjects %+v and %+v share key %q", p[0], p[1], p[0].Key())
		}
		if StableID("R", 1, p[0]) == StableID("R", 1, p[1]) {
			t.Errorf("subjects %+v and %+v share a StableID", p[0], p[1])
		}
	}
	if StableID("R|1", 2, Subject{Period: "P"}) == StableID("R", 12, Subject{Period: "P"}) {
		t.Error("a pipe in the rule ID collided with the engagement field")
	}
}

func TestSubjectKey(t *testing.T) {
	cases := []struct {
		s    Subject
		want string
	}{
		{Subject{Period: "FY2025-26"}, "FY2025-26||"},
		{Subject{Period: "2025-05", Scope: "27ABCDE1234F1Z5", MatchKey: "27ABCDE1234F1Z5|INV-0042|2025-05-14"}, "2025-05|27ABCDE1234F1Z5|27ABCDE1234F1Z5|INV-0042|2025-05-14"},
		{Subject{Period: "FY2025-26:Q1", Scope: `a|b\c`}, `FY2025-26:Q1|a\|b\\c|`},
	}
	for _, c := range cases {
		if got := c.s.Key(); got != c.want {
			t.Errorf("%+v.Key() = %q, want %q", c.s, got, c.want)
		}
	}
}

// AC-3: ValueFingerprint changes when any value changes and not when only locators
// (or labels) change.
func TestValueFingerprint(t *testing.T) {
	base := func() []LabeledValue {
		return []LabeledValue{
			{Input: "a", Label: "Tax audit report, depreciation schedule total", Value: MoneyValue(11245000),
				Locator: Locator{DocumentVersionID: 3, FileName: "TAR_v3.pdf", Kind: LocatorPDFPos, Path: "p7:l23:c41-52"}},
			{Input: "b", Label: `Computation, "Allowed depreciation"`, Value: MoneyValue(11520000),
				Locator: Locator{DocumentVersionID: 4, FileName: "comp_20261008.json", Kind: LocatorJSONPath, Path: "$.business_income.allowed_depreciation"}},
		}
	}
	fp := func(t *testing.T, v []LabeledValue) string {
		t.Helper()
		s, err := ValueFingerprint(v)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	want := fp(t, base())
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(want) {
		t.Fatalf("fingerprint %q is not 64 lowercase hex characters", want)
	}

	same := map[string]func([]LabeledValue){
		"page of first value": func(v []LabeledValue) { v[0].Locator.Path = "p9:l1:c41-52" },
		"file of second value": func(v []LabeledValue) {
			v[1].Locator = Locator{DocumentVersionID: 5, FileName: "comp_20261020.json", Kind: LocatorJSONPath, Path: "$.bi.dep"}
		},
		"label":                      func(v []LabeledValue) { v[0].Label = "TAR total" },
		"input name":                 func(v []LabeledValue) { v[1].Input = "computation" },
		"field outside kind ignored": func(v []LabeledValue) { v[0].Text = "noise"; v[0].Int = 5 },
	}
	for _, name := range sortedNames(same) {
		t.Run("unchanged by "+name, func(t *testing.T) {
			v := base()
			same[name](v)
			if got := fp(t, v); got != want {
				t.Errorf("fingerprint changed")
			}
		})
	}

	differ := map[string]func([]LabeledValue) []LabeledValue{
		"first amount by one paisa": func(v []LabeledValue) []LabeledValue { v[0].Money++; return v },
		"second amount":             func(v []LabeledValue) []LabeledValue { v[1].Money = 11245000; return v },
		"order swapped":             func(v []LabeledValue) []LabeledValue { v[0], v[1] = v[1], v[0]; return v },
		"kind money to int":         func(v []LabeledValue) []LabeledValue { v[0].Value = IntValue(11245000); return v },
		"value removed":             func(v []LabeledValue) []LabeledValue { return v[:1] },
		"value added":               func(v []LabeledValue) []LabeledValue { return append(v, LabeledValue{Value: MoneyValue(0)}) },
	}
	for _, name := range sortedNames(differ) {
		t.Run("changed by "+name, func(t *testing.T) {
			if got := fp(t, differ[name](base())); got == want {
				t.Errorf("fingerprint did not change")
			}
		})
	}

	kinds := []Value{IntValue(1), TextValue("194C"), DateValue("2025-07-31"), BoolValue(true), RateValue(1000)}
	changed := []Value{IntValue(2), TextValue("194J"), DateValue("2025-08-01"), BoolValue(false), RateValue(200)}
	for i := range kinds {
		a := fp(t, []LabeledValue{{Value: kinds[i]}})
		b := fp(t, []LabeledValue{{Value: changed[i]}})
		if a == b {
			t.Errorf("%s value change did not change the fingerprint", kinds[i].Kind)
		}
	}

	if _, err := ValueFingerprint([]LabeledValue{{Input: "a"}}); err == nil {
		t.Error("a value with no kind was accepted")
	}
	if _, err := ValueFingerprint([]LabeledValue{{Value: TextValue("\xff")}}); err == nil {
		t.Error("text that is not UTF-8 was accepted")
	}
}
