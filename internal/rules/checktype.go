package rules

import (
	"fmt"
	"slices"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/vocab"
)

// A CheckType is one comparison shape ("equal", "sum_equals", ...) as the rule loader sees
// it: it validates the check block and inputs of every rule that uses it, so a bad rule is
// refused when it loads, never during a run. The engine's check types (internal/engine/checks,
// SPEC-008 onwards) implement it together with their evaluation.
type CheckType interface {
	Name() string
	// ValidateParams returns nil if r's inputs and check parameters suit this check type.
	// Several problems may be returned together with errors.Join; each is reported.
	ValidateParams(r *Rule, v *vocab.Vocab) error
}

// CheckTypes is the registry of check types, filled at start-up and read-only afterwards.
type CheckTypes struct {
	byName map[string]CheckType
}

// NewCheckTypes returns a registry holding cts, refusing an empty or repeated name.
func NewCheckTypes(cts ...CheckType) (*CheckTypes, error) {
	r := &CheckTypes{byName: map[string]CheckType{}}
	for _, ct := range cts {
		name := ct.Name()
		if name == "" {
			return nil, fmt.Errorf("rules: a check type has no name")
		}
		if _, dup := r.byName[name]; dup {
			return nil, fmt.Errorf("rules: check type %q is registered twice", name)
		}
		r.byName[name] = ct
	}
	return r, nil
}

// Lookup returns the check type called name.
func (r *CheckTypes) Lookup(name string) (CheckType, bool) {
	ct, ok := r.byName[name]
	return ct, ok
}

// Names returns every registered name, sorted.
func (r *CheckTypes) Names() []string {
	names := make([]string, 0, len(r.byName))
	for n := range r.byName {
		names = append(names, n)
	}
	slices.Sort(names)
	return names
}
