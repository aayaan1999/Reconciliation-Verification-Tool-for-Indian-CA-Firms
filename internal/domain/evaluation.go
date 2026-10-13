package domain

import (
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
)

// A Subject is what one evaluation of a rule is about, within an engagement: a period,
// a scope (GSTIN, branch, Tally company, or "" for the whole entity) and, for
// record-level rules, a match key ("" for scalar rules).
type Subject struct {
	Period   string
	Scope    string
	MatchKey string
}

// Key returns the canonical string "period|scope|match_key" used in stable IDs and for
// sorting. A "|" or "\" inside the period or scope is escaped with "\" so two different
// subjects can never produce the same key; the match key is last, so it is written as
// is and may itself contain "|" (as record match keys do).
func (s Subject) Key() string {
	return escapeField(s.Period) + "|" + escapeField(s.Scope) + "|" + s.MatchKey
}

var fieldEscaper = strings.NewReplacer(`\`, `\\`, `|`, `\|`)

func escapeField(s string) string { return fieldEscaper.Replace(s) }

// A LabeledValue is one side of a comparison as shown in a finding: the rule's label
// for it, the value, and where the value came from.
type LabeledValue struct {
	Input string // the rule's name for this input, e.g. "a" or "b"
	Label string // e.g. "Tax audit report, depreciation schedule total"
	Value
	Locator Locator
}

// A LawRef is the legal basis a rule cites, shown as "Rule basis" in a finding (FD-02).
type LawRef struct {
	Act       string // e.g. "Income-tax Act, 1961"
	Section   string // e.g. "32"
	Reference string // further detail, may be ""
}

// EvalStatus is the outcome of one evaluation. The zero value is invalid.
type EvalStatus uint8

const (
	// StatusPass means the figures agree within the rule's tolerance.
	StatusPass EvalStatus = iota + 1
	// StatusFail means the figures disagree; it becomes or updates a finding.
	StatusFail
	// StatusCannotCheck means an input figure was not found (RU-52). It is never a finding.
	StatusCannotCheck
)

var evalStatusNames = [...]string{
	StatusPass:        "pass",
	StatusFail:        "fail",
	StatusCannotCheck: "cannot_check",
}

// Valid reports whether s is one of the defined statuses.
func (s EvalStatus) Valid() bool { return s >= StatusPass && s <= StatusCannotCheck }

// String returns "pass", "fail" or "cannot_check", or "invalid" for an undefined status.
func (s EvalStatus) String() string {
	if !s.Valid() {
		return "invalid"
	}
	return evalStatusNames[s]
}

// An Evaluation is the outcome of one rule for one subject in one run.
type Evaluation struct {
	RunID        int64
	RuleID       string
	RuleVersion  string
	Subject      Subject
	Status       EvalStatus
	Values       []LabeledValue // every side, with locator, in rule-declared order
	Difference   money.Paise    // computed from the values, never an "expected" figure
	MissingFacts []string       // vocabulary names not found, when Status is StatusCannotCheck
	LawRefs      []LawRef       // in rule-declared order
}
