package domain

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// Zero values: every zero enum is invalid and every zero struct is refused or inert.
func TestZeroValues(t *testing.T) {
	if (ValueKind(0)).Valid() || (EvalStatus(0)).Valid() || FindingState("").Valid() {
		t.Error("a zero enum reported valid")
	}
	if got := (Subject{}).Key(); got != "||" {
		t.Errorf("zero Subject key = %q", got)
	}
	if got := (Locator{}).Display(); got != "" {
		t.Errorf("zero Locator display = %q", got)
	}
	if _, err := (Evaluation{}).CanonicalJSON(); err == nil {
		t.Error("zero Evaluation encoded")
	}
	if _, err := (Finding{}).CanonicalJSON(); err == nil {
		t.Error("zero Finding encoded")
	}
	if _, err := ValueFingerprint([]LabeledValue{{}}); err == nil {
		t.Error("zero LabeledValue fingerprinted")
	}
	a, errA := ValueFingerprint(nil)
	b, errB := ValueFingerprint([]LabeledValue{})
	if errA != nil || errB != nil || a != b || len(a) != 64 {
		t.Errorf("nil/empty fingerprints: %q %v / %q %v", a, errA, b, errB)
	}
}

func TestEnumStringsAndValidity(t *testing.T) {
	for _, k := range []ValueKind{0, 7, 255} {
		if k.Valid() || k.String() != "invalid" {
			t.Errorf("kind %d: valid=%v string=%q", k, k.Valid(), k.String())
		}
	}
	for _, s := range []EvalStatus{0, 4, 255} {
		if s.Valid() || s.String() != "invalid" {
			t.Errorf("status %d: valid=%v string=%q", s, s.Valid(), s.String())
		}
	}
	states := []FindingState{StateOpen, StateDecidedFix, StateDecidedNotIssue, StateDecidedAskClient,
		StateAwaitingRecheck, StateResolved, StateReopened, StateCarriedForward}
	seen := map[FindingState]bool{}
	for _, s := range states {
		if !s.Valid() || seen[s] || s == "" {
			t.Errorf("state %q: valid=%v duplicate=%v", s, s.Valid(), seen[s])
		}
		seen[s] = true
	}
	for _, bad := range []FindingState{"", "OPEN", "open ", "withdrawn", "decided"} {
		if bad.Valid() {
			t.Errorf("state %q accepted", bad)
		}
	}
}

func valueObjectJSON(v Value) ([]byte, error) {
	o, err := valueObject(v)
	if err != nil {
		return nil, err
	}
	return marshalCanonical(o)
}

// Extreme integers must be written exactly (never through a float).
func TestCanonicalExtremeIntegers(t *testing.T) {
	cases := []struct {
		name string
		v    Value
		want string
	}{
		{"max paise", MoneyValue(math.MaxInt64), `{"kind":"money","paise":9223372036854775807}`},
		{"min paise", MoneyValue(math.MinInt64), `{"kind":"money","paise":-9223372036854775808}`},
		{"above 2^53", IntValue(1<<53 + 1), `{"int":9007199254740993,"kind":"int"}`},
		{"rate negative", RateValue(-1800), `{"kind":"rate","rate_bp":-1800}`},
		{"bool false", BoolValue(false), `{"bool":false,"kind":"bool"}`},
		{"text empty", TextValue(""), `{"kind":"text","text":""}`},
		{"html and separators", TextValue("<a&b>" + string(rune(0x2028))), `{"kind":"text","text":"<a&b>` + string(rune(0x5c)) + "u2028" + `"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, err := valueObjectJSON(c.v)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != c.want {
				t.Errorf("got  %s\nwant %s", b, c.want)
			}
		})
	}
	e := Evaluation{Status: StatusFail, Difference: math.MinInt64}
	if s := mustCanonical(t, e); !strings.Contains(s, `"difference":-9223372036854775808`) {
		t.Errorf("difference lost precision: %s", s)
	}
}

// Fields of a Value that its Kind does not name never reach the output.
func TestOffKindFieldsIgnored(t *testing.T) {
	clean := Evaluation{Status: StatusFail, Values: []LabeledValue{{Input: "a", Value: MoneyValue(5)}}}
	dirty := clean
	dirty.Values = []LabeledValue{{Input: "a", Value: Value{Kind: KindMoney, Money: 5, Int: 9, Text: "x", Date: "2025-01-01", Bool: true, RateBP: 7}}}
	if a, b := mustCanonical(t, clean), mustCanonical(t, dirty); a != b {
		t.Errorf("off-kind fields leaked:\n%s\n%s", a, b)
	}
}

// The same digits under a different kind must not collide in the fingerprint.
func TestFingerprintKindSeparation(t *testing.T) {
	vals := []Value{MoneyValue(1), IntValue(1), RateValue(1), TextValue("1"), DateValue("1"), BoolValue(true)}
	seen := map[string]int{}
	for i, v := range vals {
		fp, err := ValueFingerprint([]LabeledValue{{Value: v}})
		if err != nil {
			t.Fatal(err)
		}
		if j, dup := seen[fp]; dup {
			t.Errorf("values %d and %d share fingerprint %s", j, i, fp)
		}
		seen[fp] = i
	}
}

func permute(a []string, f func([]string)) {
	var rec func(int)
	rec = func(k int) {
		if k == len(a) {
			f(a)
			return
		}
		for i := k; i < len(a); i++ {
			a[k], a[i] = a[i], a[k]
			rec(k + 1)
			a[k], a[i] = a[i], a[k]
		}
	}
	rec(0)
}

// Fingerprint depends on the order of values (rule-declared order) and not on input
// names; MissingFacts is a set, so every permutation encodes the same.
func TestOrderingSemantics(t *testing.T) {
	ab := []LabeledValue{{Input: "a", Value: MoneyValue(1)}, {Input: "b", Value: MoneyValue(2)}}
	ba := []LabeledValue{ab[1], ab[0]}
	f1, _ := ValueFingerprint(ab)
	f2, _ := ValueFingerprint(ba)
	f3, _ := ValueFingerprint(ab)
	if f1 == f2 {
		t.Error("swapping value order did not change the fingerprint")
	}
	if f1 != f3 {
		t.Error("fingerprint not stable across calls")
	}
	renamed := []LabeledValue{{Input: "x", Value: MoneyValue(1)}, {Input: "y", Value: MoneyValue(2)}}
	if f4, _ := ValueFingerprint(renamed); f4 != f1 {
		t.Error("renaming inputs changed the fingerprint")
	}

	names := []string{"c.z", "a.b", "b.a", "a.a"}
	var want string
	permute(names, func(p []string) {
		e := Evaluation{Status: StatusCannotCheck, MissingFacts: append([]string(nil), p...)}
		got := mustCanonical(t, e)
		if want == "" {
			want = got
		} else if got != want {
			t.Errorf("permutation %v encodes differently", p)
		}
	})
	if !strings.Contains(want, `"missing_facts":["a.a","a.b","b.a","c.z"]`) {
		t.Errorf("missing facts not sorted: %s", want)
	}
	in := []string{"b", "a"}
	_ = mustCanonical(t, Evaluation{Status: StatusCannotCheck, MissingFacts: in})
	if in[0] != "b" {
		t.Error("CanonicalJSON sorted the callers MissingFacts slice in place")
	}
	e1 := Evaluation{Status: StatusFail, Values: ab}
	e2 := Evaluation{Status: StatusFail, Values: ba}
	if mustCanonical(t, e1) == mustCanonical(t, e2) {
		t.Error("value order ignored by canonical JSON")
	}
	l1 := Evaluation{Status: StatusFail, LawRefs: []LawRef{{Act: "A"}, {Act: "B"}}}
	l2 := Evaluation{Status: StatusFail, LawRefs: []LawRef{{Act: "B"}, {Act: "A"}}}
	if mustCanonical(t, l1) == mustCanonical(t, l2) {
		t.Error("law ref order ignored by canonical JSON")
	}
}

// Repeating the encoding gives the same bytes, and the output is valid JSON.
func TestCanonicalRepeatableAndWellFormed(t *testing.T) {
	e := Evaluation{RunID: 1, RuleID: "R", RuleVersion: "1", Status: StatusFail, Difference: -3,
		Values: []LabeledValue{{Input: "a", Label: "u q", Value: TextValue("\x00\t"), Locator: Locator{Path: "$.a"}}}}
	first := mustCanonical(t, e)
	for i := 0; i < 50; i++ {
		if got := mustCanonical(t, e); got != first {
			t.Fatalf("run %d differs", i)
		}
	}
	if !json.Valid([]byte(first)) || strings.HasSuffix(first, "\n") || !utf8.ValidString(first) {
		t.Errorf("malformed output %q", first)
	}
}

func TestStableIDNearMisses(t *testing.T) {
	s := Subject{Period: "FY2025-26", Scope: "27ABCDE1234F1Z5", MatchKey: "k"}
	if StableID("R", 1, s) != StableID("R", 1, s) {
		t.Error("not repeatable")
	}
	near := []struct {
		name string
		id   string
	}{
		{"base", StableID("R", 1, s)},
		{"engagement 10", StableID("R", 10, s)},
		{"engagement 0", StableID("R", 0, s)},
		{"engagement -1", StableID("R", -1, s)},
		{"rule case", StableID("r", 1, s)},
		{"rule trailing space", StableID("R ", 1, s)},
		{"empty rule", StableID("", 1, s)},
		{"empty subject", StableID("R", 1, Subject{})},
		{"matchkey case", StableID("R", 1, Subject{Period: s.Period, Scope: s.Scope, MatchKey: "K"})},
		{"scope moved to period", StableID("R", 1, Subject{Period: s.Period + "|" + s.Scope, MatchKey: "k"})},
	}
	seen := map[string]string{}
	for _, n := range near {
		if len(n.id) != 16 {
			t.Errorf("%s: length %d", n.name, len(n.id))
		}
		if prev, dup := seen[n.id]; dup {
			t.Errorf("%s collides with %s", n.name, prev)
		}
		seen[n.id] = n.name
	}
}

func TestLocatorDisplayMore(t *testing.T) {
	cases := []struct {
		name string
		l    Locator
		want string
	}{
		{"pdf page only", Locator{FileName: "a.pdf", Kind: LocatorPDFPos, Path: "p7"}, "a.pdf p.7"},
		{"pdf no p prefix", Locator{FileName: "a.pdf", Kind: LocatorPDFPos, Path: "7:l1"}, "a.pdf 7:l1"},
		{"pdf empty page", Locator{FileName: "a.pdf", Kind: LocatorPDFPos, Path: "p:l1"}, "a.pdf p:l1"},
		{"pdf non digit page", Locator{FileName: "a.pdf", Kind: LocatorPDFPos, Path: "p7x:l1"}, "a.pdf p7x:l1"},
		{"pdf path empty", Locator{FileName: "a.pdf", Kind: LocatorPDFPos}, "a.pdf"},
		{"non-pdf keeps full path", Locator{FileName: "a.xlsx", Kind: LocatorSheetCell, Path: "p7:l1"}, "a.xlsx p7:l1"},
		{"no file name", Locator{Kind: LocatorSheetCell, Path: "TB!A1"}, "TB!A1"},
		{"table cell", Locator{FileName: "w.docx", Kind: LocatorTableCell, Path: "T3!R7C4"}, "w.docx T3!R7C4"},
		{"document id not shown", Locator{DocumentVersionID: 99, FileName: "x", Kind: LocatorLine, Path: "L4"}, "x L4"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.l.Display(); got != c.want {
				t.Errorf("got %q want %q", got, c.want)
			}
		})
	}
}

// Invariant 1: no finding-carrying type has a field that could hold a correct or
// expected figure, or judgment text. Free text exists only where it quotes a source.
func TestNoCorrectValueOrJudgmentFields(t *testing.T) {
	banned := []string{"correct", "expected", "should", "suggest", "recommend", "advice", "advise",
		"fix", "proper", "right", "judg", "opinion", "applicab", "message", "narrative",
		"explanation", "reason", "comment", "remedy", "action", "target", "ideal"}
	roots := []reflect.Type{reflect.TypeOf(Finding{}), reflect.TypeOf(Evaluation{}), reflect.TypeOf(Fact{}),
		reflect.TypeOf(Record{}), reflect.TypeOf(RunSnapshot{})}
	seen := map[reflect.Type]bool{}
	var walk func(path string, ty reflect.Type)
	walk = func(path string, ty reflect.Type) {
		for ty.Kind() == reflect.Pointer || ty.Kind() == reflect.Slice || ty.Kind() == reflect.Array || ty.Kind() == reflect.Map {
			ty = ty.Elem()
		}
		if ty.Kind() != reflect.Struct || seen[ty] {
			return
		}
		seen[ty] = true
		for i := 0; i < ty.NumField(); i++ {
			f := ty.Field(i)
			name := strings.ToLower(f.Name)
			for _, b := range banned {
				if strings.Contains(name, b) {
					t.Errorf("%s.%s looks like a judgment or correct-value field (%q)", path, f.Name, b)
				}
			}
			if k := f.Type.Kind(); k == reflect.Float32 || k == reflect.Float64 {
				t.Errorf("%s.%s is a float", path, f.Name)
			}
			walk(path+"."+f.Name, f.Type)
		}
	}
	for _, r := range roots {
		walk(r.Name(), r)
	}
	for _, s := range []FindingState{StateOpen, StateDecidedFix, StateDecidedNotIssue, StateDecidedAskClient,
		StateAwaitingRecheck, StateResolved, StateReopened, StateCarriedForward} {
		for _, w := range []string{"should", "incorrect", "must be", "wrong", "error"} {
			if strings.Contains(string(s), w) {
				t.Errorf("state %q contains %q", s, w)
			}
		}
	}
}

// The canonical encoding carries only these keys, so no extra text field can slip into
// the output unnoticed.
func TestCanonicalKeySetIsClosed(t *testing.T) {
	ev := Evaluation{Status: StatusFail, Values: []LabeledValue{{Input: "a", Value: MoneyValue(1)}}}
	f := Finding{State: StateOpen, LatestEvaluation: ev}
	b, err := f.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	want := []string{"latest_evaluation", "review_document", "stable_id", "state", "value_fingerprint"}
	if got := sortedNames(m); !reflect.DeepEqual(got, want) {
		t.Errorf("finding keys %v want %v", got, want)
	}
	em := m["latest_evaluation"].(map[string]any)
	wantE := []string{"difference", "law_refs", "missing_facts", "rule_id", "rule_version", "run_id", "status", "subject", "values"}
	if got := sortedNames(em); !reflect.DeepEqual(got, wantE) {
		t.Errorf("evaluation keys %v want %v", got, wantE)
	}
}
