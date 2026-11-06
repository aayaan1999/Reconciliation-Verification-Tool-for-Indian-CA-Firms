package vocab

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// OfficeCategories is the fixed list a document's category comes from.
var OfficeCategories = []string{"Books", "GST", "Income tax", "TDS", "Payroll", "Bank", "Other"}

// A Problem is one reason an office document is not ready. Field names the part of the form it is
// about, so the screen can show it beside the field. Code is one of the BAD_* codes.
type Problem struct {
	Field string
	Code  string
	Msg   string
}

// Codes of the problems ValidateOfficeDocument reports, in the style of the layout wizard's.
const (
	BadDocumentCode = "BAD_DOCUMENT_CODE"
	BadName         = "BAD_NAME"
	BadDescription  = "BAD_DESCRIPTION"
	BadCategory     = "BAD_CATEGORY"
	BadFormats      = "BAD_FORMATS"
	BadRecordType   = "BAD_RECORD_TYPE"
	BadColumn       = "BAD_COLUMN"
	BadKey          = "BAD_KEY"
)

// ValidateOfficeDocument checks a document and its record type against every rule that makes a
// document ready (SPEC-022, "Validation") and returns all the problems together; none means ready.
// others is the rest of the office definitions, without this document and its record type, so a
// name is compared with every other name. The base vocabulary supplies the reserved names.
//
// The record type itself is checked by the same checker that reads records.yaml; this adds the
// limits that only the Partner's definitions have (lengths, types, labels, key flags).
func (v *Vocab) ValidateOfficeDocument(others Office, doc OfficeDocument, rt OfficeRecordType) []Problem {
	var ps []Problem
	add := func(field, code, format string, args ...any) {
		ps = append(ps, Problem{field, code, fmt.Sprintf(format, args...)})
	}
	oneLine := func(field, code, what, s string, min, max int) {
		switch n := utf8.RuneCountInString(strings.TrimSpace(s)); {
		case strings.ContainsAny(s, "\r\n"):
			add(field, code, "%s must be on one line", what)
		case n < min || n > max:
			add(field, code, "%s must be %d to %d characters", what, min, max)
		}
	}

	// The document.
	switch {
	case !simpleName.MatchString(doc.Code):
		add("code", BadDocumentCode, "the code must be lower case letters, digits and _, starting with a letter")
	case utf8.RuneCountInString(doc.Code) < 3 || utf8.RuneCountInString(doc.Code) > 40:
		add("code", BadDocumentCode, "the code must be 3 to 40 characters")
	case v.codeTaken(others, doc.Code):
		add("code", BadDocumentCode, "the code %q is already used by a built-in document, a prefix or another document", doc.Code)
	}
	oneLine("name", BadName, "the name", doc.Name, 3, 60)
	oneLine("description", BadDescription, "the description", doc.Description, 10, 300)
	if !slices.Contains(OfficeCategories, doc.Category) {
		add("category", BadCategory, "the category must be one of %s", strings.Join(OfficeCategories, ", "))
	}
	if len(doc.Formats) == 0 {
		add("applicable_for", BadFormats, "choose at least one file format (json, xlsx, csv)")
	}
	for _, f := range doc.Formats {
		if !slices.Contains(OfficeFormats, f) {
			add("applicable_for", BadFormats, "the file format %q is not one of json, xlsx, csv", f)
		}
	}

	// The record type: its name, then the limits, then the shared checker.
	switch {
	case utf8.RuneCountInString(rt.Name) < 3 || utf8.RuneCountInString(rt.Name) > 40:
		add("record_type.name", BadRecordType, "the record type name must be 3 to 40 characters")
	case v.recordTypeTaken(others, rt.Name):
		add("record_type.name", BadRecordType, "the record type name %q is already used by a built-in or another record type", rt.Name)
	}
	if utf8.RuneCountInString(rt.Description) > 300 {
		add("record_type.description", BadRecordType, "the record type description must be at most 300 characters")
	}
	for _, d := range others.Documents {
		if d.RecordType != "" && d.RecordType == rt.Name {
			add("record_type.name", BadRecordType, "the record type %q is already the structure of document %q; each document has its own", rt.Name, d.Code)
		}
	}
	if n := len(rt.Columns); n < 1 || n > 40 {
		add("columns", BadColumn, "a document needs 1 to 40 columns, not %d", n)
	}
	keys, nonKeys := 0, 0
	for i, c := range rt.Columns {
		field := fmt.Sprintf("columns[%d]", i)
		if n := utf8.RuneCountInString(c.Name); n < 2 || n > 40 {
			add(field+".name", BadColumn, "the column name %q must be 2 to 40 characters", c.Name)
		}
		oneLine(field+".label", BadColumn, fmt.Sprintf("the label of column %q", c.Name), c.Label, 1, 60)
		switch c.Type {
		case ColumnText, ColumnDate:
		case ColumnAmount:
			if c.Key {
				add(field+".key", BadKey, "column %q is an amount; a key column is text or a date", c.Name)
			}
		default:
			add(field+".type", BadColumn, "the type %q of column %q must be text, date or amount", c.Type, c.Name)
		}
		if c.Key {
			keys++
		} else {
			nonKeys++
		}
	}
	if len(rt.Columns) > 0 && keys == 0 {
		add("columns", BadKey, "mark at least one column as part of the key")
	}
	if keys > 0 && nonKeys == 0 {
		add("columns", BadColumn, "a structure of key columns only reads nothing useful; add a column that is not a key")
	}
	for _, msg := range checkOfficeRecordType(rt, doc.Code) {
		ps = append(ps, Problem{"record_type", BadRecordType, msg})
	}
	return ps
}

// codeTaken reports whether a document code is a base document name, a base prefix or an
// office document code, ignoring capitals.
func (v *Vocab) codeTaken(others Office, code string) bool {
	k := strings.ToLower(code)
	for _, d := range v.docTypes {
		if d.Office {
			continue // a merged copy's office entries are compared through others
		}
		if strings.ToLower(d.Name) == k || strings.ToLower(d.Prefix) == k {
			return true
		}
	}
	return slices.ContainsFunc(others.Documents, func(d OfficeDocument) bool { return strings.ToLower(d.Code) == k })
}

// recordTypeTaken reports whether a record type name is a base or an office record type name.
func (v *Vocab) recordTypeTaken(others Office, name string) bool {
	k := strings.ToLower(name)
	for _, r := range v.recordTypes {
		if r.DocumentType != "" && v.isOfficeDoc(r.DocumentType) {
			continue
		}
		if strings.ToLower(r.Name) == k {
			return true
		}
	}
	return slices.ContainsFunc(others.RecordTypes, func(r OfficeRecordType) bool { return strings.ToLower(r.Name) == k })
}

func (v *Vocab) isOfficeDoc(name string) bool {
	d, ok := v.DocumentType(name)
	return ok && d.Office
}
