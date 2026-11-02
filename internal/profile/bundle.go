package profile

import (
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
)

// outsideReason replaces the end of an unread reason for a value that lies outside every
// section of a bundle and that no profile read.
const outsideReason = "not part of any document this file holds"

// ExtractAll reads doc with each of parts, as DetectAll returned them: one profile, or the
// profiles of one bundle (SPEC-005), each giving its own document. A value is listed as not
// read only if no profile of the bundle read it, and then once: in the report of the
// document whose section holds it, or, outside every section, in the first document's.
func ExtractAll(parts []*Profile, doc ingest.Doc, src Source) ([]*Result, error) {
	results := make([]*Result, len(parts))
	for i, p := range parts {
		r, err := Extract(p, doc, src)
		if err != nil {
			return nil, err
		}
		results[i] = r
	}
	if len(parts) < 2 {
		return results, nil
	}

	inAnySection := func(path string) bool {
		for _, p := range parts {
			if InSection(path, p.Section) {
				return true
			}
		}
		return false
	}
	// A value counts as not read only if no profile read it: every report lists it.
	unreadBy := map[string]int{}
	for _, r := range results {
		for _, u := range r.Report.Unread {
			unreadBy[u.Locator.Path]++
		}
	}
	for i, r := range results {
		var kept []ingest.Unread
		for _, u := range r.Report.Unread {
			if unreadBy[u.Locator.Path] < len(parts) {
				continue // another document of the file read it
			}
			switch {
			case InSection(u.Locator.Path, parts[i].Section):
				kept = append(kept, u)
			case i == 0 && !inAnySection(u.Locator.Path):
				u.Kind = ingest.UnreadOther
				if j := strings.Index(u.Reason, ";"); j >= 0 {
					u.Reason = u.Reason[:j+2] + outsideReason
				} else {
					u.Reason = outsideReason
				}
				kept = append(kept, u)
			}
		}
		r.Report.Unread = kept
		r.Report.Sort()
	}
	return results, nil
}
