package ingest

import (
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
)

// fake is an adapter that sniffs files starting with its prefix.
type fake struct{ ft, prefix string }

func (f fake) FileType() string                     { return f.ft }
func (f fake) Sniff(head []byte, _ string) bool     { return strings.HasPrefix(string(head), f.prefix) }
func (f fake) Open(io.ReaderAt, int64) (Doc, error) { return nil, errors.New("not used") }

// AC-1: adapters come back in sorted order whatever the registration order.
func TestRegistrySortedOrder(t *testing.T) {
	r := NewRegistry()
	for _, ft := range []string{"xlsx", "json", "tallyxml", "csv", "pdftext"} {
		if err := r.Register(fake{ft: ft}); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{"csv", "json", "pdftext", "tallyxml", "xlsx"}
	if got := r.FileTypes(); !slices.Equal(got, want) {
		t.Fatalf("FileTypes() = %v, want %v", got, want)
	}
	var got []string
	for _, a := range r.Adapters() {
		got = append(got, a.FileType())
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Adapters() order = %v, want %v", got, want)
	}
	if a, ok := r.Lookup("json"); !ok || a.FileType() != "json" {
		t.Fatalf("Lookup(json) = %v, %v", a, ok)
	}
	if _, ok := r.Lookup("docx"); ok {
		t.Fatal("Lookup(docx) found an unregistered adapter")
	}
}

// AC-1: a second adapter for the same file type is refused, and the first one is kept.
func TestRegistryRejectsDuplicate(t *testing.T) {
	r := NewRegistry()
	first := fake{ft: "json", prefix: "first"}
	if err := r.Register(first); err != nil {
		t.Fatal(err)
	}
	err := r.Register(fake{ft: "json", prefix: "second"})
	if err == nil || !strings.Contains(err.Error(), `"json" is already registered`) {
		t.Fatalf("duplicate registration: err = %v", err)
	}
	if a, _ := r.Lookup("json"); a != first {
		t.Fatal("the duplicate replaced the first adapter")
	}
}

func TestRegistryRejectsBadAdapters(t *testing.T) {
	r := NewRegistry()
	for _, a := range []Adapter{nil, fake{ft: ""}, fake{ft: " json"}, fake{ft: "json\n"}} {
		if err := r.Register(a); err == nil {
			t.Errorf("Register(%#v) accepted", a)
		}
	}
	if n := len(r.FileTypes()); n != 0 {
		t.Fatalf("%d adapters registered after only bad registrations", n)
	}
}

func TestForFile(t *testing.T) {
	r := NewRegistry()
	for _, a := range []Adapter{fake{ft: "json", prefix: "{"}, fake{ft: "xml", prefix: "<"}, fake{ft: "xml2", prefix: "<?"}} {
		if err := r.Register(a); err != nil {
			t.Fatal(err)
		}
	}
	a, err := r.ForFile([]byte(`{"a":1}`), "x.txt")
	if err != nil || a.FileType() != "json" {
		t.Fatalf("ForFile(json) = %v, %v", a, err)
	}

	_, err = r.ForFile([]byte("PK\x03\x04"), "book.xlsx")
	var ie *Error
	if !errors.As(err, &ie) || ie.Code != UnsupportedFileType || ie.File != "book.xlsx" ||
		!strings.Contains(ie.Msg, "it reads: json, xml, xml2") {
		t.Fatalf("unknown file: err = %v", err)
	}

	_, err = r.ForFile([]byte("<?xml"), "tally.xml")
	if !errors.As(err, &ie) || ie.Code != UnsupportedFileType || !strings.Contains(ie.Msg, "(xml, xml2)") {
		t.Fatalf("file sniffed by two adapters: err = %v", err)
	}
}

func TestPosition(t *testing.T) {
	data := []byte("ab\ncd\r\nef")
	for _, tc := range []struct {
		off        int64
		line, colm int
	}{
		{0, 1, 1}, {2, 1, 3}, {3, 2, 1}, {4, 2, 2}, {7, 3, 1}, {9, 3, 3}, {99, 3, 3}, {-1, 1, 1},
	} {
		if l, c := Position(data, tc.off); l != tc.line || c != tc.colm {
			t.Errorf("Position(%d) = %d:%d, want %d:%d", tc.off, l, c, tc.line, tc.colm)
		}
	}
}

func TestErrorText(t *testing.T) {
	e := &Error{Code: ParseError, File: "comp.json", Line: 3, Column: 7, Offset: 21, Msg: "bad"}
	if got, want := e.Error(), "PARSE_ERROR: comp.json: line 3, column 7 (byte 21): bad"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
	e = &Error{Code: FileTooLarge, Msg: "too big"}
	if got, want := e.Error(), "FILE_TOO_LARGE: too big"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// The error codes are part of the interface (validation reports, UI); pin their text.
func TestCodesAreStable(t *testing.T) {
	got := []Code{UnsupportedFileType, FileTooLarge, ParseError, ProfileNoMatch, ProfileAmbiguous, RequiredFieldMissing, ConflictingValues}
	want := []Code{"UNSUPPORTED_FILE_TYPE", "FILE_TOO_LARGE", "PARSE_ERROR", "PROFILE_NO_MATCH", "PROFILE_AMBIGUOUS", "REQUIRED_FIELD_MISSING", "CONFLICTING_VALUES"}
	if !slices.Equal(got, want) {
		t.Fatalf("codes = %v, want %v", got, want)
	}
}

func TestReportSort(t *testing.T) {
	loc := func(p string) domain.Locator { return domain.Locator{Kind: domain.LocatorJSONPath, Path: p} }
	r := &ValidationReport{
		Unread: []Unread{{loc("$.b"), "x"}, {loc("$.a"), "y"}},
		Errors: []Issue{
			{Code: RequiredFieldMissing, Msg: "m", Locator: loc("$.z")},
			{Code: ConflictingValues, Msg: "m", Locator: loc("$.a")},
			{Code: ConflictingValues, Msg: "a", Locator: loc("$.a")},
		},
	}
	if !r.HasErrors() {
		t.Fatal("HasErrors() = false")
	}
	r.Sort()
	if r.Unread[0].Locator.Path != "$.a" {
		t.Errorf("Unread not sorted: %v", r.Unread)
	}
	if got := []string{r.Errors[0].Msg + r.Errors[0].Locator.Path, r.Errors[1].Msg + r.Errors[1].Locator.Path, r.Errors[2].Locator.Path}; !slices.Equal(got, []string{"a$.a", "m$.a", "$.z"}) {
		t.Errorf("Errors not sorted: %v", got)
	}
	if (&ValidationReport{}).HasErrors() {
		t.Error("empty report has errors")
	}
}
