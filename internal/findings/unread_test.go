package findings

import (
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/rules"
)

// An amount that was in a document but not read says so with the reason the reader gave (SPEC-019). An entry in
// a shape the checks never write still shows the reason, not a generic "figure not found".
func TestMissingTextForAnUnreadAmount(t *testing.T) {
	r := sample(t)
	rule := r.Rules["RECON-DEP-001"]
	const reason = "dollar rate not set"

	got := r.missingText(rule, rules.MissingUnreadPrefix+"gstr1_b2b_invoice.igst"+rules.MissingUnreadReason+reason)
	if !strings.HasPrefix(got, "Cannot check: igst is not read in") || !strings.HasSuffix(got, ": "+reason) {
		t.Errorf("well-formed entry: %q", got)
	}

	got = r.missingText(rule, rules.MissingUnreadPrefix+"igst"+rules.MissingUnreadReason+reason)
	if want := "Cannot check: an amount was not read: " + reason; got != want {
		t.Errorf("entry without a record type: %q, want %q", got, want)
	}
	if strings.Contains(got, "figure not found") {
		t.Errorf("the reason was lost behind the generic wording: %q", got)
	}
}
