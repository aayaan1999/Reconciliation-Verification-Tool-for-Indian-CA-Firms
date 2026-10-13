package domain

import "testing"

// AC-5: Display produces the report formats of requirements §6.9 and the SPEC-003 scope.
func TestLocatorDisplay(t *testing.T) {
	cases := []struct {
		l    Locator
		want string
	}{
		{Locator{FileName: "comp_20261008.json", Kind: LocatorJSONPath, Path: "$.business_income.allowed_depreciation"},
			"comp_20261008.json $.business_income.allowed_depreciation"},
		{Locator{FileName: "comp_20261008.json", Kind: LocatorJSONPath, Path: "$.allowed_depreciation"},
			"comp_20261008.json $.allowed_depreciation"},
		{Locator{FileName: "TAR_v3.pdf", Kind: LocatorPDFPos, Path: "p7:l23:c41-52"}, "TAR_v3.pdf p.7"},
		{Locator{FileName: "TAR_v3.pdf", Kind: LocatorPDFPos, Path: "p12"}, "TAR_v3.pdf p.12"},
		{Locator{FileName: "TB.xlsx", Kind: LocatorSheetCell, Path: "Trial Balance!D42"}, "TB.xlsx Trial Balance!D42"},
		{Locator{FileName: "notes.docx", Kind: LocatorTableCell, Path: "T3!R7C4"}, "notes.docx T3!R7C4"},
		// A PDF path that does not start with a page number is shown as stored, not guessed at.
		{Locator{FileName: "TAR_v3.pdf", Kind: LocatorPDFPos, Path: "page7"}, "TAR_v3.pdf page7"},
		{Locator{FileName: "TAR_v3.pdf", Kind: LocatorPDFPos, Path: "p:l1"}, "TAR_v3.pdf p:l1"},
		{Locator{FileName: "TAR_v3.pdf", Kind: LocatorPDFPos}, "TAR_v3.pdf"},
		{Locator{Kind: LocatorJSONPath, Path: "$.a"}, "$.a"},
		{Locator{}, ""},
	}
	for _, c := range cases {
		if got := c.l.Display(); got != c.want {
			t.Errorf("%+v.Display() = %q, want %q", c.l, got, c.want)
		}
	}
}

func TestKindAndStatusNames(t *testing.T) {
	kinds := map[ValueKind]string{KindMoney: "money", KindInt: "int", KindText: "text", KindDate: "date",
		KindBool: "bool", KindRate: "rate", 0: "invalid", 7: "invalid"}
	for k, want := range kinds {
		if got := k.String(); got != want {
			t.Errorf("ValueKind(%d).String() = %q, want %q", k, got, want)
		}
	}
	statuses := map[EvalStatus]string{StatusPass: "pass", StatusFail: "fail", StatusCannotCheck: "cannot_check", 0: "invalid", 4: "invalid"}
	for s, want := range statuses {
		if got := s.String(); got != want {
			t.Errorf("EvalStatus(%d).String() = %q, want %q", s, got, want)
		}
	}
	for _, s := range []FindingState{StateOpen, StateDecidedFix, StateDecidedNotIssue, StateDecidedAskClient,
		StateAwaitingRecheck, StateResolved, StateReopened, StateCarriedForward} {
		if !s.Valid() {
			t.Errorf("state %q not valid", s)
		}
	}
}

func TestParseValueKind(t *testing.T) {
	for k := KindMoney; k <= KindRate; k++ {
		if got, ok := ParseValueKind(k.String()); !ok || got != k {
			t.Errorf("ParseValueKind(%q) = %v, %v", k.String(), got, ok)
		}
	}
	for _, s := range []string{"", "invalid", "Money", "float", " money"} {
		if _, ok := ParseValueKind(s); ok {
			t.Errorf("ParseValueKind(%q) accepted", s)
		}
	}
}
