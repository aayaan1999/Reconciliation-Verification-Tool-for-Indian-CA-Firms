package checks

import (
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks/equal"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks/gte"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks/matchrecords"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks/percentof"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks/presenttogether"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks/sumequals"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/engine/checks/tablelookup"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
)

// All returns the registry of every check type. A new check type is one new sub-package
// plus one line here.
func All() *rules.CheckTypes {
	cts, err := rules.NewCheckTypes(
		equal.Check{},
		gte.Check{},
		matchrecords.Check{},
		percentof.Check{},
		presenttogether.Check{},
		sumequals.Check{},
		tablelookup.Check{},
	)
	if err != nil {
		panic(err) // a programming error: two check types with one name
	}
	return cts
}
