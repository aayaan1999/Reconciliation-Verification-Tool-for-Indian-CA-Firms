package ingest

import (
	"cmp"
	"slices"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
)

// ClientMatch says whether the PAN read from a file matches the engagement's client.
type ClientMatch string

const (
	ClientNotChecked ClientMatch = "not_checked" // the file has no PAN, or the profile reads none
	ClientMatched    ClientMatch = "matched"
	ClientMismatched ClientMatch = "mismatched"
)

// A ValidationReport is shown for every upload (IN-14): which format the file was
// recognised as, what was read and what was not. The adapter layer defines it; the
// profile executor (SPEC-006) fills it in.
type ValidationReport struct {
	File           string // as uploaded
	FileType       string // adapter file type, "json"
	ProfileID      string // "generic.taxsuite.computation"; empty when no profile matched
	ProfileVersion string
	SourceSystem   string         // the profile's human-readable source, for "Recognised as"
	TaxYear        period.TaxYear // zero when not read
	ClientMatch    ClientMatch
	PANLast4       string // for display only; the full PAN is never put in a report
	FactCount      int
	RecordCount    int
	Unread         []Unread // things in the file the profile did not read
	Warnings       []Issue
	Errors         []Issue // any error means the file's figures are not used
}

// Unread is one part of a file that was not turned into a fact or record.
type Unread struct {
	Locator domain.Locator
	Reason  string // "text, not in vocabulary"
}

// An Issue is one warning or error in a validation report.
type Issue struct {
	Code    Code
	Msg     string
	Locator domain.Locator // where in the file, when known
}

// HasErrors reports whether the file must not be used.
func (r *ValidationReport) HasErrors() bool { return len(r.Errors) > 0 }

// Sort puts the lists in a fixed order (by locator path, then code and message), so the
// same file always gives byte-identical reports (IN-21).
func (r *ValidationReport) Sort() {
	slices.SortStableFunc(r.Unread, func(a, b Unread) int {
		return cmp.Or(cmp.Compare(a.Locator.Path, b.Locator.Path), cmp.Compare(a.Reason, b.Reason))
	})
	byPlace := func(a, b Issue) int {
		return cmp.Or(cmp.Compare(a.Locator.Path, b.Locator.Path), cmp.Compare(a.Code, b.Code), cmp.Compare(a.Msg, b.Msg))
	}
	slices.SortStableFunc(r.Warnings, byPlace)
	slices.SortStableFunc(r.Errors, byPlace)
}
