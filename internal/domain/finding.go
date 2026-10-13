package domain

// FindingState is where a finding stands in its lifecycle (FD-03; technical spec §B7.2).
// The transitions between states belong to the findings lifecycle (SPEC-025), not here.
type FindingState string

// The proposed eighth state, "withdrawn" (rule disabled), is not in requirements v0.3
// and waits on open question T6; it is deliberately not defined yet.
const (
	StateOpen             FindingState = "open"               // rule fails; no decision yet
	StateDecidedFix       FindingState = "decided_fix"        // the source document will be corrected
	StateDecidedNotIssue  FindingState = "decided_not_issue"  // the difference is explained ("Not an issue")
	StateDecidedAskClient FindingState = "decided_ask_client" // waiting on client information
	StateAwaitingRecheck  FindingState = "awaiting_recheck"   // a newer draft of an input arrived since the decision
	StateResolved         FindingState = "resolved"           // the latest run passes
	StateReopened         FindingState = "reopened"           // was resolved or not an issue; fails again or values changed
	StateCarriedForward   FindingState = "carried_forward"    // carried to next tax year's engagement
)

// Valid reports whether s is one of the defined states.
func (s FindingState) Valid() bool {
	switch s {
	case StateOpen, StateDecidedFix, StateDecidedNotIssue, StateDecidedAskClient,
		StateAwaitingRecheck, StateResolved, StateReopened, StateCarriedForward:
		return true
	}
	return false
}

// A Finding is a failed evaluation with a lifecycle (FD-01..05). It keeps the same
// StableID across runs and rule versions, so a re-run updates it instead of adding a
// duplicate.
type Finding struct {
	StableID         string // StableID(rule, engagement, subject): no rule version, no run
	ValueFingerprint string // ValueFingerprint(values): values only, no locators (FD-04)
	State            FindingState
	ReviewDocument   string // document type the UI groups it under (FD-06); never in finding text
	LatestEvaluation Evaluation
}
