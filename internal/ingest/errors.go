package ingest

import (
	"fmt"
	"strings"
)

// Code is a stable error code. Codes are shown in validation reports and matched by tests
// and the UI, so their text never changes.
type Code string

const (
	UnsupportedFileType  Code = "UNSUPPORTED_FILE_TYPE"  // no adapter recognises the file
	FileTooLarge         Code = "FILE_TOO_LARGE"         // the file is over the adapter's size limit
	ParseError           Code = "PARSE_ERROR"            // the file is not valid for its type
	ProfileNoMatch       Code = "PROFILE_NO_MATCH"       // no profile's detect block matches (SPEC-006)
	ProfileAmbiguous     Code = "PROFILE_AMBIGUOUS"      // several profiles match (SPEC-006)
	RequiredFieldMissing Code = "REQUIRED_FIELD_MISSING" // a required figure is not in the file (SPEC-006)
	ConflictingValues    Code = "CONFLICTING_VALUES"     // one figure read with two different values (SPEC-006)
)

// An Error is an ingestion failure with a stable Code. Position fields are 0 when unknown.
type Error struct {
	Code   Code
	File   string // file name as uploaded, for messages; may be empty
	Line   int    // 1-based line in the file
	Column int    // 1-based column (byte) in the line
	Offset int64  // 0-based byte offset in the file
	Msg    string
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString(string(e.Code))
	if e.File != "" {
		b.WriteString(": ")
		b.WriteString(e.File)
	}
	if e.Line > 0 {
		fmt.Fprintf(&b, ": line %d, column %d (byte %d)", e.Line, e.Column, e.Offset)
	}
	b.WriteString(": ")
	b.WriteString(e.Msg)
	return b.String()
}

// Position returns the 1-based line and column of byte offset in data. Lines end at "\n";
// a "\r" before it is part of the line ending, not a column.
func Position(data []byte, offset int64) (line, column int) {
	if offset < 0 {
		offset = 0
	}
	if offset > int64(len(data)) {
		offset = int64(len(data))
	}
	line, start := 1, 0
	for i, c := range data[:offset] {
		if c == '\n' {
			line++
			start = i + 1
		}
	}
	return line, int(offset) - start + 1
}
