package profile

import (
	"fmt"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
)

// Detect returns the one profile in s whose detect tests all hold for doc (IN-23). No match
// is PROFILE_NO_MATCH and several matches are PROFILE_AMBIGUOUS, naming every match: the
// tool never guesses. Only profiles for doc's file type are tried. fileName is used in
// messages.
func Detect(s *Set, doc ingest.Doc, fileName string) (*Profile, error) {
	var hits []*Profile
	for _, p := range s.profiles {
		if p.FileType != doc.FileType() {
			continue
		}
		ok, err := p.Matches(doc)
		if err != nil {
			return nil, err
		}
		if ok {
			hits = append(hits, p)
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return nil, &ingest.Error{Code: ingest.ProfileNoMatch, File: fileName,
			Msg: fmt.Sprintf("the file matches no known %s format; it may come from software or a version the tool does not know yet", doc.FileType())}
	}
	keys := make([]string, len(hits))
	for i, p := range hits {
		keys[i] = p.Key()
	}
	return nil, &ingest.Error{Code: ingest.ProfileAmbiguous, File: fileName,
		Msg: fmt.Sprintf("the file matches %d formats (%s); their detect tests must be made stricter", len(hits), strings.Join(keys, ", "))}
}

// Matches reports whether every detect test of p holds for doc.
func (p *Profile) Matches(doc ingest.Doc) (bool, error) {
	if p.FileType != doc.FileType() {
		return false, nil
	}
	for _, pr := range p.detect {
		cells, err := doc.Select(pr.sel)
		if err != nil {
			return false, fmt.Errorf("profile %s: %w", p.Key(), err)
		}
		if !pr.holds(cells) {
			return false, nil
		}
	}
	return true, nil
}

func (pr predicate) holds(cells []ingest.Cell) bool {
	if pr.kind == "json_exists" {
		return len(cells) > 0
	}
	if len(cells) != 1 || !isScalar(cells[0].Kind) {
		return false
	}
	if pr.kind == "json_equals" {
		return cells[0].Text == pr.value
	}
	return pr.re.MatchString(cells[0].Text)
}

func isScalar(k ingest.CellKind) bool {
	return k == ingest.CellText || k == ingest.CellNumber || k == ingest.CellBool
}
