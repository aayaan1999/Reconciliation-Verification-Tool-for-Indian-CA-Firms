package engine

import (
	"slices"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
)

// FixtureResults runs fixture f of rule r through the same evaluation a run uses and returns
// the results in the form a fixture file records them (rules.Expected), sorted by subject.
// The Rule Builder uses it to check a fixture before it saves the rule (SPEC-010 AC-4).
func FixtureResults(r *rules.Rule, f *rules.Fixture, cts *rules.CheckTypes) ([]rules.Expected, error) {
	evs, err := Evaluate([]*rules.Rule{r}, f.Facts, f.Records, f.TaxYear, cts, 1)
	if err != nil {
		return nil, err
	}
	out := make([]rules.Expected, 0, len(evs))
	for _, e := range evs {
		out = append(out, rules.Expected{
			Status:   e.Status.String(),
			Period:   e.Subject.Period,
			Scope:    e.Subject.Scope,
			MatchKey: e.Subject.MatchKey,
			Code:     e.Code,
			Missing:  slices.Sorted(slices.Values(e.MissingFacts)),
		})
	}
	return out, nil
}
