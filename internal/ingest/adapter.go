package ingest

import (
	"io"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
)

// An Adapter turns the raw bytes of one file type into a navigable Doc (§A6.1). It knows
// file syntax, never meaning: it produces no facts and contains no figure names.
type Adapter interface {
	// FileType names the file type: "json", "xlsx", "csv", "docx", "tallyxml", "pdftext"
	// or "fixedwidth". It is the registry key and the profile's file_type.
	FileType() string
	// Sniff is a cheap check of the first bytes of a file, before a full parse. It looks at
	// the contents; name is a hint at most, never the deciding factor.
	Sniff(head []byte, name string) bool
	// Open parses the whole file. Errors are *Error values.
	Open(r io.ReaderAt, size int64) (Doc, error)
}

// A Doc is an opened file that profiles query. It is read-only and safe to share.
type Doc interface {
	// FileType is the FileType of the adapter that opened it.
	FileType() string
	// Select returns the cells sel addresses, in document order. Nothing at the address is
	// an empty result, not an error: whether a figure is required is the profile's
	// decision. A selector of the wrong kind for this Doc is an error.
	Select(sel Selector) ([]Cell, error)
}

// A LeafLister is a Doc that can list every value in the file, so a validation report
// can show what a profile did not read (IN-14).
type LeafLister interface {
	// Leaves returns every value that is not a container (for JSON: every string,
	// number, bool and null), in file order.
	Leaves() []Cell
}

// A Selector addresses cells in one kind of Doc: a JSON path, a sheet and header, a table
// and row label, a PDF region or an XML path. Each adapter defines its own selector type
// and parses it from profile text when the profile loads, so a bad selector is a
// profile load error, never a run-time surprise.
type Selector interface {
	// FileType is the file type whose Doc understands this selector.
	FileType() string
	// String is the selector as written in a profile.
	String() string
}

// CellKind says what kind of value a cell holds, as the file wrote it.
type CellKind string

const (
	CellText   CellKind = "text"
	CellNumber CellKind = "number" // Text is the number exactly as written, never via float
	CellBool   CellKind = "bool"   // Text is "true" or "false"
	CellNull   CellKind = "null"   // an explicit null or empty cell
	CellObject CellKind = "object" // a JSON object; select its members with a longer selector
	CellArray  CellKind = "array"  // a JSON array; select its items with a longer selector
)

// A Cell is one value read from a file. It is raw text: turning it into money, a date or
// a count is the profile executor's job (SPEC-006).
type Cell struct {
	Kind CellKind
	Text string // the value as written; empty for objects and arrays
	// Locator says where the cell is. The adapter sets Kind and the full concrete Path
	// (for example "$.rows[3].amount"); the caller adds the document version and file name.
	Locator domain.Locator
}
