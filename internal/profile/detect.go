package profile

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	jsonadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/json"
	xlsxadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/xlsx"
)

// Detect returns the one profile in s whose detect tests all hold for doc (IN-23). No match
// is PROFILE_NO_MATCH and several matches are PROFILE_AMBIGUOUS, naming every match: the
// tool never guesses. Only profiles for doc's file type are tried. fileName is used in
// messages. A file that holds several documents (a bundle) is read with DetectAll.
func Detect(s *Set, doc ingest.Doc, fileName string) (*Profile, error) {
	hits, err := matching(s, doc)
	if err != nil {
		return nil, named(err, fileName)
	}
	if len(hits) == 1 {
		return hits[0], nil
	}
	return nil, detectError(s, hits, doc, fileName)
}

// DetectAll returns the profiles that read doc: the one profile whose detect tests hold, or
// every profile of one bundle when all the profiles that match name the same bundle, sorted
// by document type (SPEC-005). Any other double match is PROFILE_AMBIGUOUS.
func DetectAll(s *Set, doc ingest.Doc, fileName string) ([]*Profile, error) {
	hits, err := matching(s, doc)
	if err != nil {
		return nil, named(err, fileName)
	}
	if len(hits) == 1 || oneBundle(hits) {
		slices.SortFunc(hits, func(a, b *Profile) int {
			return cmp.Or(cmp.Compare(a.DocumentType, b.DocumentType), cmp.Compare(a.ID, b.ID), cmp.Compare(a.Version, b.Version))
		})
		return hits, nil
	}
	return nil, detectError(s, hits, doc, fileName)
}

// matching returns the profiles whose detect tests hold for doc. A file problem one profile
// meets (an Excel header label found twice in a sheet it names) does not stop the others
// being tried: it is returned only when no profile matches, since it may be why.
func matching(s *Set, doc ingest.Doc) ([]*Profile, error) {
	var hits []*Profile
	var held error
	for _, p := range s.profiles {
		if !p.reads(doc) {
			continue
		}
		ok, err := p.Matches(doc)
		var ie *ingest.Error
		switch {
		case errors.As(err, &ie):
			if held == nil {
				held = err
			}
		case err != nil:
			return nil, err
		case ok:
			hits = append(hits, p)
		}
	}
	if len(hits) == 0 && held != nil {
		return nil, held
	}
	return hits, nil
}

// named gives a file error the name of the file, when it has none.
func named(err error, fileName string) error {
	var ie *ingest.Error
	if errors.As(err, &ie) && ie.File == "" {
		ie.File = fileName
	}
	return err
}

// oneBundle reports whether there are several profiles and all name the same bundle.
func oneBundle(hits []*Profile) bool {
	if len(hits) < 2 || hits[0].Bundle == "" {
		return false
	}
	for _, p := range hits[1:] {
		if p.Bundle != hits[0].Bundle {
			return false
		}
	}
	return true
}

func detectError(s *Set, hits []*Profile, doc ingest.Doc, fileName string) error {
	if len(hits) == 0 {
		msg := fmt.Sprintf("the file matches no known %s format; it may come from software or a version the tool does not know yet", doc.FileType())
		if why := excelMisses(s, doc); why != "" {
			msg += ". " + why
		}
		return &ingest.Error{Code: ingest.ProfileNoMatch, File: fileName, Msg: msg}
	}
	keys := make([]string, len(hits))
	for i, p := range hits {
		keys[i] = p.Key()
	}
	slices.Sort(keys)
	return &ingest.Error{Code: ingest.ProfileAmbiguous, File: fileName,
		Msg: fmt.Sprintf("the file matches %d formats (%s); their detect tests must be made stricter", len(hits), strings.Join(keys, ", "))}
}

// excelMisses says, for a workbook no format reads, what each Excel format looked for and
// did not find ("Books: sales register export as Excel: sheet "Sales Register": no row has
// every header label; the nearest, row 3, lacks "Taxable Value""), so staff can see whether
// a label or a sheet differs. It is "" for other files.
func excelMisses(s *Set, doc ingest.Doc) string {
	wb, ok := doc.(*xlsxadapter.Doc)
	if !ok {
		return ""
	}
	var lines []string
	for _, p := range s.profiles {
		if p.FileType != xlsxadapter.FileType {
			continue
		}
		_, err := wb.View(p.tables)
		var nf *xlsxadapter.NotFoundError
		switch {
		case errors.As(err, &nf):
			lines = append(lines, p.SourceSystem+": "+nf.Error())
		case err == nil:
			lines = append(lines, p.SourceSystem+": its sheets and headers are there, but the values that identify it differ")
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return "Excel formats tried: " + strings.Join(lines, "; ")
}

// InSection reports whether the JSON path p lies in section (or is section itself).
func InSection(p, section string) bool {
	return p == section || (strings.HasPrefix(p, section) && (p[len(section)] == '.' || p[len(section)] == '['))
}

// Matches reports whether every detect test of p holds for doc.
func (p *Profile) Matches(doc ingest.Doc) (bool, error) {
	if !p.reads(doc) {
		return false, nil
	}
	doc, ok, err := p.read(doc)
	if err != nil || !ok {
		return false, err
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

// reads reports whether p can read doc: a file of its own type, or for a JSON profile a CSV
// or Excel file that may hold a JSON document flattened into paths and values (SPEC-011).
func (p *Profile) reads(doc ingest.Doc) bool {
	_, flat := doc.(ingest.Flattened)
	return p.FileType == doc.FileType() || p.FileType == jsonadapter.FileType && flat
}

// read gives the document p reads: doc itself; for an Excel profile the workbook read as
// p's tables (SPEC-008); for a JSON profile of a CSV or Excel file, the JSON document the
// file holds flattened (SPEC-011). ok is false when a sheet or header row p names is not in
// the workbook, or the file holds no flattened document: p does not match it. A header
// label found twice is an error.
func (p *Profile) read(doc ingest.Doc) (ingest.Doc, bool, error) {
	if f, flat := doc.(ingest.Flattened); flat && p.FileType == jsonadapter.FileType {
		return f.JSONTree()
	}
	if p.FileType != xlsxadapter.FileType {
		return doc, true, nil
	}
	wb, isWorkbook := doc.(*xlsxadapter.Doc)
	if !isWorkbook {
		return nil, false, fmt.Errorf("profile %s reads Excel workbooks, not %T", p.Key(), doc)
	}
	v, err := wb.View(p.tables)
	var nf *xlsxadapter.NotFoundError
	switch {
	case errors.As(err, &nf):
		return nil, false, nil
	case err != nil:
		return nil, false, err
	}
	return v, true, nil
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
