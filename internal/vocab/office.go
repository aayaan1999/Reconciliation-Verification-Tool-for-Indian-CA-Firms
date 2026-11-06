package vocab

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// The office vocabulary (SPEC-022): the document types and record types the Partner added in the
// Document Master. It sits on top of the base vocabulary (content/vocabulary), which it never
// changes: WithOffice returns a merged copy, and ContentHash of the copy is still the base hash.

// The file formats a document can be read from, in the fixed order they are stored and hashed.
var OfficeFormats = []string{"json", "xlsx", "csv"}

// Column types of an office record type.
const (
	ColumnText   = "text"
	ColumnDate   = "date"
	ColumnAmount = "amount"
)

// An OfficeColumn is one column of an office record type. Name is the key in files, Label is
// what the wizard and the Rule Builder show.
type OfficeColumn struct {
	Name     string
	Label    string
	Type     string // ColumnText, ColumnDate or ColumnAmount
	Key      bool   // part of the match key
	Required bool
}

// An OfficeRecordType is a record type the Partner defined, columns in the order shown.
type OfficeRecordType struct {
	Name        string
	Description string
	Columns     []OfficeColumn
}

// An OfficeDocument is a document type the Partner added. RecordType names its structure; it is
// empty while the document is a Draft. Active is whether new layouts and rules may use it. It
// is not part of the hash, so switching it does not change the vocabulary a run used.
type OfficeDocument struct {
	Code        string
	Name        string
	Category    string
	Description string
	Formats     []string // subset of OfficeFormats
	RecordType  string
	Active      bool `json:"-"`
}

// Office is the Partner's definitions, as stored.
type Office struct {
	Documents   []OfficeDocument
	RecordTypes []OfficeRecordType
}

// A NameCollision is an office name that equals a base or another office name.
type NameCollision struct{ Msg string }

func (e *NameCollision) Error() string { return e.Msg }

// normalFormats keeps the known formats once each, in OfficeFormats order.
func normalFormats(in []string) []string {
	out := []string{}
	for _, f := range OfficeFormats {
		if slices.Contains(in, f) {
			out = append(out, f)
		}
	}
	return out
}

// recordTypeDocOf writes an office record type in the shape the shared checker reads: amount
// columns are amounts, the other columns are fields, and the key columns that are fields are the
// match key. A key that is an amount is left out of the match key (ValidateOfficeDocument says
// so with its own message).
func recordTypeDocOf(rt OfficeRecordType, document string) recordTypeDoc {
	r := recordTypeDoc{Name: rt.Name, Description: rt.Description, DocumentType: document}
	for _, c := range rt.Columns {
		if c.Type == ColumnAmount {
			r.Amounts = append(r.Amounts, c.Name)
			continue
		}
		r.Fields = append(r.Fields, c.Name)
		if c.Key {
			r.MatchKey = append(r.MatchKey, c.Name)
		}
	}
	return r
}

// recordTypeOf is the vocabulary's record type for an office one.
func recordTypeOf(rt OfficeRecordType, document string) RecordType {
	r := recordTypeDocOf(rt, document)
	return RecordType{Name: r.Name, Description: r.Description, DocumentType: document, MatchKey: r.MatchKey, Amounts: r.Amounts, Fields: r.Fields}
}

// checkOfficeRecordType runs the shared record type checker over an office record type. The
// document type list holds the document, so its name is accepted.
func checkOfficeRecordType(rt OfficeRecordType, document string) []string {
	c := checker{}
	docs := []DocumentType{{Name: document}}
	c.checkRecordType("office", nil, "record_type", recordTypeDocOf(rt, document), docs, false)
	msgs := make([]string, len(c.errs))
	for i, e := range c.errs {
		msgs[i] = e.Msg
	}
	return msgs
}

// WithOffice returns a merged copy of the vocabulary: the base entries plus every office
// document that is ready, with its record type. The base vocabulary is not changed, and the copy
// is a strict superset of it, so a built-in rule reads it exactly as before.
//
// A document is ready when its record type is named, is among the office record types and passes
// the same checker as the YAML files. A document that is not ready is left out (it is a Draft).
// An office name equal to a base name or to another office name is refused with a *NameCollision,
// whether or not the entry is ready, so a Draft cannot be activated into a clash later.
// Document codes share one name space, and record type names another; a code must also differ
// from every base prefix. Names are compared ignoring capitals.
func (v *Vocab) WithOffice(o Office) (*Vocab, error) {
	if err := v.checkOfficeNames(o); err != nil {
		return nil, err
	}
	types := map[string]OfficeRecordType{}
	for _, rt := range o.RecordTypes {
		types[rt.Name] = rt
	}
	var docs []OfficeDocument
	var types2 []OfficeRecordType
	docTypes := slices.Clone(v.docTypes)
	records := slices.Clone(v.recordTypes)
	for _, d := range o.Documents {
		rt, ok := types[d.RecordType]
		if d.RecordType == "" || !ok || len(checkOfficeRecordType(rt, d.Code)) > 0 {
			continue
		}
		docs = append(docs, d)
		types2 = append(types2, rt)
		docTypes = append(docTypes, DocumentType{Name: d.Code, Description: d.Description, Office: true})
		records = append(records, recordTypeOf(rt, d.Code))
	}
	merged := newVocab(v.version, docTypes, slices.Clone(v.facts), records)
	merged.hash = v.hash // the base hash, whatever the office adds
	slices.SortFunc(docs, func(a, b OfficeDocument) int { return cmp.Compare(a.Code, b.Code) })
	slices.SortFunc(types2, func(a, b OfficeRecordType) int { return cmp.Compare(a.Name, b.Name) })
	merged.office = Office{Documents: docs, RecordTypes: types2}
	return merged, nil
}

// checkOfficeNames refuses a document code or record type name that collides.
func (v *Vocab) checkOfficeNames(o Office) error {
	codes := map[string]bool{}
	for _, d := range v.docTypes {
		codes[strings.ToLower(d.Name)] = true
		codes[strings.ToLower(d.Prefix)] = true
	}
	for _, d := range o.Documents {
		k := strings.ToLower(d.Code)
		if codes[k] {
			return &NameCollision{fmt.Sprintf("document code %q is already used by a built-in document, a prefix or another document", d.Code)}
		}
		codes[k] = true
	}
	names := map[string]bool{}
	for _, r := range v.recordTypes {
		names[strings.ToLower(r.Name)] = true
	}
	for _, r := range o.RecordTypes {
		k := strings.ToLower(r.Name)
		if names[k] {
			return &NameCollision{fmt.Sprintf("record type %q is already used by a built-in or another record type", r.Name)}
		}
		names[k] = true
	}
	owner := map[string]string{}
	for _, d := range o.Documents {
		if d.RecordType == "" {
			continue
		}
		if prev, ok := owner[d.RecordType]; ok {
			return &NameCollision{fmt.Sprintf("record type %q is the structure of both %q and %q; each document has its own", d.RecordType, prev, d.Code)}
		}
		owner[d.RecordType] = d.Code
	}
	return nil
}

// Office returns the ready office definitions in this vocabulary, documents sorted by code and
// record types by name; it is empty for the base vocabulary. Inactive documents are included:
// they stay readable so existing layouts and rules keep working.
func (v *Vocab) Office() Office {
	return Office{Documents: slices.Clone(v.office.Documents), RecordTypes: slices.Clone(v.office.RecordTypes)}
}

// OfficeColumns returns the Partner's columns for a record type (labels, types, flags), and
// whether it is an office record type.
func (v *Vocab) OfficeColumns(recordType string) ([]OfficeColumn, bool) {
	for _, rt := range v.office.RecordTypes {
		if rt.Name == recordType {
			return slices.Clone(rt.Columns), true
		}
	}
	return nil, false
}

// OfficeCanonical is the fixed JSON form of the ready office definitions that OfficeHash hashes
// and that a run keeps as its snapshot; "" when there are none. It covers meaning only: names,
// descriptions, category, formats, column names, labels, types and flags, in code and name
// order. The Active flag is left out. Struct fields are written in declaration order and there
// are no maps, so the same definitions always give the same bytes.
func (v *Vocab) OfficeCanonical() string {
	if len(v.office.Documents) == 0 {
		return ""
	}
	o := Office{Documents: slices.Clone(v.office.Documents), RecordTypes: slices.Clone(v.office.RecordTypes)}
	for i := range o.Documents {
		o.Documents[i].Formats = normalFormats(o.Documents[i].Formats)
	}
	b, err := json.Marshal(o)
	if err != nil {
		panic(err) // plain structs of strings and bools always marshal
	}
	return string(b)
}

// OfficeHash is the lowercase hex SHA-256 of OfficeCanonical, or "" when there are no office
// definitions, so a database with an empty Master hashes exactly as before.
func (v *Vocab) OfficeHash() string {
	c := v.OfficeCanonical()
	if c == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(c))
	return hex.EncodeToString(sum[:])
}

// ParseOfficeSnapshot reads the canonical form a run kept back into definitions, so an earlier
// comparison can be shown with the rows, columns and labels it had. Every definition read back is
// marked active; the flag is not part of a snapshot.
func ParseOfficeSnapshot(body string) (Office, error) {
	var o Office
	if err := json.Unmarshal([]byte(body), &o); err != nil {
		return Office{}, fmt.Errorf("vocab: office snapshot: %w", err)
	}
	for i := range o.Documents {
		o.Documents[i].Active = true
	}
	return o, nil
}

// IsOfficeRecordType reports whether a record type belongs to a document the Partner added
// (SPEC-022), as opposed to one of the built-in record types.
func (v *Vocab) IsOfficeRecordType(name string) bool {
	rt, ok := v.RecordType(name)
	return ok && v.isOfficeDoc(rt.DocumentType)
}
