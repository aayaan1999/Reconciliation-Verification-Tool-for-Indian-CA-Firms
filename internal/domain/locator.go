package domain

import "strings"

// Locator kinds: how Locator.Path is written for each kind of source.
const (
	LocatorJSONPath  = "json_path"  // "$.business_income.allowed_depreciation"
	LocatorSheetCell = "sheet_cell" // "Trial Balance!D42"
	LocatorTableCell = "table_cell" // "T3!R7C4": table 3, row 7, column 4 of a Word file
	LocatorPDFPos    = "pdf_pos"    // "p7:l23:c41-52": page 7, line 23, characters 41 to 52
	LocatorXMLPath   = "xml_path"   // an XPath into a Tally XML export
	LocatorLine      = "line"       // a line of a text or fixed-width file
)

// A Locator says exactly where a value came from (FV-01, FD-02): which uploaded document
// version, and where inside it.
type Locator struct {
	DocumentVersionID int64
	FileName          string // as uploaded, for display
	Kind              string // one of the Locator* constants
	Path              string // position inside the file, written as the Kind describes
}

// Display returns the short text shown in reports, for example
// "comp_20261008.json $.business_income.allowed_depreciation", "TAR_v3.pdf p.7" or
// "TB.xlsx Trial Balance!D42". A PDF position shows only its page, since line and
// character ranges mean nothing to a reader holding the printed document.
func (l Locator) Display() string {
	where := l.Path
	if l.Kind == LocatorPDFPos {
		if page, ok := pdfPage(l.Path); ok {
			where = "p." + page
		}
	}
	switch {
	case l.FileName == "":
		return where
	case where == "":
		return l.FileName
	}
	return l.FileName + " " + where
}

// pdfPage returns the page number of a pdf_pos path "p7:l23:c41-52" (or just "p7").
func pdfPage(path string) (string, bool) {
	first, _, _ := strings.Cut(path, ":")
	page, ok := strings.CutPrefix(first, "p")
	if !ok || page == "" {
		return "", false
	}
	for _, r := range page {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return page, true
}
