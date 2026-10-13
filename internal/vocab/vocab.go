package vocab

import (
	"cmp"
	"slices"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
)

// Periodicity says which period a fact is reported for.
type Periodicity string

const (
	PeriodicityFY      Periodicity = "FY"      // the whole financial year, "FY2025-26"
	PeriodicityQuarter Periodicity = "quarter" // a quarter, "FY2025-26:Q1"
	PeriodicityMonth   Periodicity = "month"   // a month, "2025-04"
)

// A DocumentType is a kind of document the tool reads, such as the computation or the tax
// audit report. Every fact belongs to one; its names start with the type's Prefix.
type DocumentType struct {
	Name        string // "tax_audit_report"
	Prefix      string // "tar": first part of every fact name of this document type
	Description string
	SinceVocab  int // vocabulary version that added it
}

// A Fact is one entry of the fact vocabulary: a figure name and what it means.
type Fact struct {
	Name         string // "tar.depreciation_schedule.total"
	Kind         domain.ValueKind
	Periodicity  Periodicity
	DocumentType string
	Description  string
	SinceVocab   int // vocabulary version that added it
	DeprecatedIn int // vocabulary version that retired it; 0 while in use
}

// Deprecated reports whether the name has been retired. A retired name stays in the
// vocabulary, so old runs can still be explained, but its meaning is never reused.
func (f Fact) Deprecated() bool { return f.DeprecatedIn > 0 }

// A RecordType is one kind of row read from a register or portal file (RC-03..05).
type RecordType struct {
	Name         string // "gst_invoice"
	Description  string
	MatchKey     []string // fields that together identify one row, in key order
	Amounts      []string // money columns
	Fields       []string // every other column
	SinceVocab   int
	DeprecatedIn int
}

// Deprecated reports whether the record type has been retired.
func (r RecordType) Deprecated() bool { return r.DeprecatedIn > 0 }

// Vocab is a loaded, validated vocabulary. It is read-only and safe to share.
type Vocab struct {
	version     int
	hash        string
	docTypes    []DocumentType // sorted by name
	facts       []Fact         // sorted by name
	recordTypes []RecordType   // sorted by name
	factIndex   map[string]int
	recordIndex map[string]int
	docIndex    map[string]int
}

func newVocab(version int, docTypes []DocumentType, facts []Fact, records []RecordType) *Vocab {
	v := &Vocab{
		version:     version,
		docTypes:    slices.SortedFunc(slices.Values(docTypes), func(a, b DocumentType) int { return cmp.Compare(a.Name, b.Name) }),
		facts:       slices.SortedFunc(slices.Values(facts), func(a, b Fact) int { return cmp.Compare(a.Name, b.Name) }),
		recordTypes: slices.SortedFunc(slices.Values(records), func(a, b RecordType) int { return cmp.Compare(a.Name, b.Name) }),
		factIndex:   map[string]int{},
		recordIndex: map[string]int{},
		docIndex:    map[string]int{},
	}
	for i, d := range v.docTypes {
		v.docIndex[d.Name] = i
	}
	for i, f := range v.facts {
		v.factIndex[f.Name] = i
	}
	for i, r := range v.recordTypes {
		v.recordIndex[r.Name] = i
	}
	v.hash = contentHash(v)
	return v
}

// Version returns the vocabulary version, recorded in every run (§A10).
func (v *Vocab) Version() int { return v.version }

// ContentHash returns the lowercase hex SHA-256 of the vocabulary's content (not of the
// file bytes, so comments and spacing do not count). It is printed in the generated
// document, which is how a content change without a version increase is caught.
func (v *Vocab) ContentHash() string { return v.hash }

// Lookup returns the fact named name, including a deprecated one.
func (v *Vocab) Lookup(name string) (Fact, bool) {
	i, ok := v.factIndex[name]
	if !ok {
		return Fact{}, false
	}
	return v.facts[i], true
}

// IsFact reports whether name is a fact in the vocabulary.
func (v *Vocab) IsFact(name string) bool { _, ok := v.factIndex[name]; return ok }

// RecordType returns the record type named name.
func (v *Vocab) RecordType(name string) (RecordType, bool) {
	i, ok := v.recordIndex[name]
	if !ok {
		return RecordType{}, false
	}
	return cloneRecordType(v.recordTypes[i]), true
}

// IsRecordType reports whether name is a record type in the vocabulary.
func (v *Vocab) IsRecordType(name string) bool { _, ok := v.recordIndex[name]; return ok }

// DocumentType returns the document type named name.
func (v *Vocab) DocumentType(name string) (DocumentType, bool) {
	i, ok := v.docIndex[name]
	if !ok {
		return DocumentType{}, false
	}
	return v.docTypes[i], true
}

// DocumentTypes returns every document type, sorted by name.
func (v *Vocab) DocumentTypes() []DocumentType { return slices.Clone(v.docTypes) }

// Facts returns every fact, sorted by name.
func (v *Vocab) Facts() []Fact { return slices.Clone(v.facts) }

// RecordTypes returns every record type, sorted by name.
func (v *Vocab) RecordTypes() []RecordType {
	out := make([]RecordType, len(v.recordTypes))
	for i, r := range v.recordTypes {
		out[i] = cloneRecordType(r)
	}
	return out
}

// cloneRecordType copies the slices so a caller cannot change the shared vocabulary.
func cloneRecordType(r RecordType) RecordType {
	r.MatchKey, r.Amounts, r.Fields = slices.Clone(r.MatchKey), slices.Clone(r.Amounts), slices.Clone(r.Fields)
	return r
}
