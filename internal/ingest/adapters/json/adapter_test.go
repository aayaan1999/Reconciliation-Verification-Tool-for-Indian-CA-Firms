package jsonadapter

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
)

// sample is a synthetic computation file; every figure and PAN is invented.
const sample = `{
  "meta": {"form": "COMPUTATION", "schema_version": "1.0", "tax_year": "2025-26"},
  "assessee": {"pan": "AAAPZ9999Z", "name": "Illustrative Traders"},
  "business_income": {"allowed_depreciation": 112450.50, "exact": 90071992547409.93},
  "Schedule BP": {"it's": 7, "back\\slash": 8, "line\nbreak": 9},
  "rows": [
    {"ledger": "Rent", "amount": 120000},
    {"ledger": "Audit fee", "amount": 35000.00, "flags": [true, false, null]},
    {"ledger": "Interest", "amount": -1e3}
  ],
  "": "empty key",
  "nothing": null
}`

func open(t *testing.T, text string) *Doc {
	t.Helper()
	doc, err := New(0).Open(strings.NewReader(text), int64(len(text)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return doc.(*Doc)
}

func sel(t *testing.T, d *Doc, path string) []ingest.Cell {
	t.Helper()
	p, err := ParsePath(path)
	if err != nil {
		t.Fatalf("ParsePath(%q): %v", path, err)
	}
	cells, err := d.Select(p)
	if err != nil {
		t.Fatalf("Select(%q): %v", path, err)
	}
	return cells
}

// AC-2: numbers come back as the exact text in the file, including values float64
// cannot hold.
func TestNumbersAreExactText(t *testing.T) {
	const exact = "90071992547409.93"
	if f, _ := strconv.ParseFloat(exact, 64); strconv.FormatFloat(f, 'f', -1, 64) == exact {
		t.Fatalf("test value %s survives float64; choose one that does not", exact)
	}
	doc := open(t, sample)
	for path, want := range map[string]string{
		"$.business_income.exact":                exact,
		"$.business_income.allowed_depreciation": "112450.50", // trailing zero kept
		"$.rows[1].amount":                       "35000.00",
		"$.rows[2].amount":                       "-1e3", // as written, not -1000
	} {
		cells := sel(t, doc, path)
		if len(cells) != 1 || cells[0].Kind != ingest.CellNumber || cells[0].Text != want {
			t.Errorf("%s = %+v, want number %q", path, cells, want)
		}
	}
	// Values far outside float64's range and precision also stay exact.
	for _, n := range []string{"1e400", "-0", "0.1000000000000000000000000000001", "123456789012345678901234567890"} {
		cells := sel(t, open(t, `{"n":`+n+`}`), "$.n")
		if len(cells) != 1 || cells[0].Text != n {
			t.Errorf("number %s came back as %+v", n, cells)
		}
	}
}

// AC-3: every form in the subset works.
func TestPathForms(t *testing.T) {
	doc := open(t, sample)
	type want struct {
		path string
		kind ingest.CellKind
		text string
	}
	cases := map[string][]want{
		"$":                               {{"$", ingest.CellObject, ""}},
		"$.meta.form":                     {{"$.meta.form", ingest.CellText, "COMPUTATION"}},
		"$['meta']['tax_year']":           {{"$.meta.tax_year", ingest.CellText, "2025-26"}},
		"$['Schedule BP']['it\\'s']":      {{`$['Schedule BP']['it\'s']`, ingest.CellNumber, "7"}},
		`$['Schedule BP']['back\\slash']`: {{`$['Schedule BP']['back\\slash']`, ingest.CellNumber, "8"}},
		"$['Schedule BP']['line\nbreak']": {{"$['Schedule BP']['line\nbreak']", ingest.CellNumber, "9"}},
		"$['']":                           {{"$['']", ingest.CellText, "empty key"}},
		"$.rows[0].ledger":                {{"$.rows[0].ledger", ingest.CellText, "Rent"}},
		"$.rows[1].flags[2]":              {{"$.rows[1].flags[2]", ingest.CellNull, ""}},
		"$.rows[*].ledger": {
			{"$.rows[0].ledger", ingest.CellText, "Rent"},
			{"$.rows[1].ledger", ingest.CellText, "Audit fee"},
			{"$.rows[2].ledger", ingest.CellText, "Interest"},
		},
		"$.rows[*].flags[*]": {
			{"$.rows[1].flags[0]", ingest.CellBool, "true"},
			{"$.rows[1].flags[1]", ingest.CellBool, "false"},
			{"$.rows[1].flags[2]", ingest.CellNull, ""},
		},
		"$.rows":    {{"$.rows", ingest.CellArray, ""}},
		"$.nothing": {{"$.nothing", ingest.CellNull, ""}},
		// Nothing at the address is an empty result, not an error.
		"$.missing":           nil,
		"$.rows[3]":           nil,
		"$.meta[0]":           nil, // index into an object
		"$.rows.ledger":       nil, // key into a list
		"$.meta[*]":           nil, // [*] on an object
		"$.meta.form.deeper":  nil, // key into text
		"$.rows[*].no_column": nil,
	}
	for path, wants := range cases {
		cells := sel(t, doc, path)
		var got []want
		for _, c := range cells {
			got = append(got, want{c.Locator.Path, c.Kind, c.Text})
		}
		if !reflect.DeepEqual(got, wants) {
			t.Errorf("%q:\n got %q\nwant %q", path, got, wants)
		}
	}
}

// AC-3: syntax outside the subset is refused with an error naming the path and the place.
func TestUnsupportedPaths(t *testing.T) {
	for _, tc := range []struct {
		path string
		at   int // 1-based character in the message
		msg  string
	}{
		{"", 1, "must start with $"},
		{"meta.form", 1, "must start with $"},
		{"$..form", 2, "recursive descent"},
		{"$.meta.*", 7, "'.*' is not supported"},
		{"$.", 2, "a key name must follow"},
		{"$.1abc", 3, "must start with a letter or _"},
		{"$.tax-year", 6, `unexpected '-'`},
		{"$meta", 2, "expected '.' or '['"},
		{"$.rows[-1]", 8, "negative indexes"},
		{"$.rows[?(@.amount>0)]", 8, "filters"},
		{"$.rows[(@.length-1)]", 8, "filters"},
		{"$.rows[0:2]", 9, "slices"},
		{"$.rows[:2]", 8, "slices"},
		{"$.rows[0,1]", 9, "unions"},
		{"$.rows[01]", 8, "leading zeros"},
		{"$.rows[1234567890]", 8, "too large"},
		{`$["meta"]`, 3, "single quotes"},
		{"$['meta]", 2, "not closed with '"},
		{"$['meta'", 2, "not closed with ']'"},
		{"$['a'x]", 2, "not closed with ']'"},
		{"$.rows[0", 7, "not closed with ']'"},
		{"$.rows[", 7, "'[' is not closed"},
		{"$.rows[*", 7, "not closed with ']'"},
		{`$['a\b']`, 5, `only \' and \\`},
		{`$['a\`, 5, `only \' and \\`},
		{"$[x]", 3, "unexpected 'x' inside [ ]"},
		{"$.rows[ 0]", 8, "unexpected ' ' inside [ ]"},
	} {
		_, err := ParsePath(tc.path)
		var pe *PathError
		if !errors.As(err, &pe) {
			t.Errorf("ParsePath(%q) = %v, want a PathError", tc.path, err)
			continue
		}
		msg := err.Error()
		if !strings.Contains(msg, strconv.Quote(tc.path)) || !strings.Contains(msg, fmt.Sprintf("at character %d:", tc.at)) ||
			!strings.Contains(msg, tc.msg) || !strings.Contains(msg, "supported: $, .key, ['key'], [n], [*]") {
			t.Errorf("ParsePath(%q):\n got %s\nwant the path, character %d and %q", tc.path, msg, tc.at, tc.msg)
		}
	}
}

// AC-4: every cell has a json_path locator with its full concrete path, and that path
// selects exactly the same cell again.
func TestLocatorsAreConcreteAndRoundTrip(t *testing.T) {
	doc := open(t, sample)
	var all []ingest.Cell
	for _, path := range []string{"$.rows[*].amount", "$.rows[*].flags[*]", "$['Schedule BP']['it\\'s']",
		`$['Schedule BP']['back\\slash']`, "$['Schedule BP']['line\nbreak']", "$['']", "$", "$.meta.tax_year"} {
		all = append(all, sel(t, doc, path)...)
	}
	if len(all) != 12 {
		t.Fatalf("selected %d cells, want 12", len(all))
	}
	for _, c := range all {
		if c.Locator.Kind != domain.LocatorJSONPath {
			t.Errorf("%s: locator kind %q, want %q", c.Locator.Path, c.Locator.Kind, domain.LocatorJSONPath)
		}
		if strings.Contains(c.Locator.Path, "[*]") {
			t.Errorf("locator %q still holds [*]", c.Locator.Path)
		}
		again := sel(t, doc, c.Locator.Path)
		if len(again) != 1 || again[0] != c {
			t.Errorf("concrete path %q selects %+v, want only %+v", c.Locator.Path, again, c)
		}
	}
	if got := sel(t, doc, "$.rows[*].amount")[2].Locator.Path; got != "$.rows[2].amount" {
		t.Errorf("third amount at %q, want $.rows[2].amount", got)
	}
}

// AC-5: malformed files are PARSE_ERROR with the line, column and byte offset of the fault.
func TestParseErrors(t *testing.T) {
	deep := strings.Repeat("[", maxDepth+2) + strings.Repeat("]", maxDepth+2)
	for _, tc := range []struct {
		name, text string
		line, col  int
		msg        string
	}{
		{"missing comma", "{\n  \"a\": 1\n  \"b\": 2\n}", 3, 3, "after object key:value pair"},
		{"trailing comma", "{\"a\": 1,}", 1, 8, "invalid character ','"}, // the trailing comma itself
		{"single quotes", "{'a': 1}", 1, 2, "invalid character '\\''"},
		{"bare word", "{\"a\": yes}", 1, 7, "invalid character 'y'"},
		{"leading zero", "{\"a\": 01}", 1, 8, "invalid character '1'"},
		{"NaN", "{\"a\": NaN}", 1, 7, "invalid character 'N'"},
		{"truncated", "{\"a\": [1, 2", 1, 12, "ends before the JSON value is complete"},
		{"unclosed string", "{\"a\": \"abc", 1, 11, "ends before the JSON value is complete"},
		{"text after value", "{\"a\": 1}\n\n  {\"b\": 2}", 3, 3, "unexpected text after the end"},
		{"two values", "1 2", 1, 3, "unexpected text after the end"},
		{"duplicate key", "{\n \"a\": 1,\n \"b\": {\"x\": 1},\n \"a\": 2\n}", 4, 2, `the key "a" appears twice`},
		{"duplicate nested key", "{\"r\": [{\"x\": 1, \"x\": 1}]}", 1, 17, `the key "x" appears twice`},
		{"invalid UTF-8", "{\"a\": \"ab\xffc\"}", 1, 10, "not valid UTF-8"},
		{"invalid UTF-8 after BOM", "\xEF\xBB\xBF{\"a\": \"\xc3\"}", 1, 11, "not valid UTF-8"},
		{"error after BOM", "\xEF\xBB\xBF{\n\"a\" 1}", 2, 5, "after object key"},
		{"too deep", deep, 1, maxDepth + 2, "nested more than 512 levels"},
	} {
		_, err := New(0).Open(strings.NewReader(tc.text), int64(len(tc.text)))
		var ie *ingest.Error
		if !errors.As(err, &ie) {
			t.Errorf("%s: err = %v, want *ingest.Error", tc.name, err)
			continue
		}
		wantOff := lineColToOffset(tc.text, tc.line, tc.col)
		if ie.Code != ingest.ParseError || ie.Line != tc.line || ie.Column != tc.col || ie.Offset != wantOff || !strings.Contains(ie.Msg, tc.msg) {
			t.Errorf("%s:\n got %s (line %d col %d off %d)\nwant PARSE_ERROR line %d col %d off %d containing %q",
				tc.name, ie, ie.Line, ie.Column, ie.Offset, tc.line, tc.col, wantOff, tc.msg)
		}
		if !strings.Contains(ie.Error(), fmt.Sprintf("line %d, column %d", tc.line, tc.col)) {
			t.Errorf("%s: message %q does not show the line and column", tc.name, ie)
		}
	}
}

func lineColToOffset(text string, line, col int) int64 {
	off := 0
	for l := 1; l < line; l++ {
		off += strings.IndexByte(text[off:], '\n') + 1
	}
	return int64(off + col - 1)
}

func TestEmptyFiles(t *testing.T) {
	for _, text := range []string{"", "   \n\t", "\xEF\xBB\xBF", "\xEF\xBB\xBF \n"} {
		_, err := New(0).Open(strings.NewReader(text), int64(len(text)))
		var ie *ingest.Error
		if !errors.As(err, &ie) || ie.Code != ingest.ParseError || ie.Msg != "the file is empty" {
			t.Errorf("Open(%q) = %v, want PARSE_ERROR the file is empty", text, err)
		}
	}
}

func TestByteOrderMarkAccepted(t *testing.T) {
	doc := open(t, "\xEF\xBB\xBF{\"a\": 5}")
	if cells := sel(t, doc, "$.a"); len(cells) != 1 || cells[0].Text != "5" {
		t.Fatalf("$.a = %+v", cells)
	}
}

func TestTopLevelArrayAndScalar(t *testing.T) {
	if cells := sel(t, open(t, `[{"a":1},{"a":2}]`), "$[*].a"); len(cells) != 2 || cells[1].Locator.Path != "$[1].a" {
		t.Errorf("$[*].a = %+v", cells)
	}
	if cells := sel(t, open(t, ` "just text" `), "$"); len(cells) != 1 || cells[0].Kind != ingest.CellText || cells[0].Text != "just text" {
		t.Errorf("$ = %+v", cells)
	}
}

// Strings are returned exactly, after JSON unescaping; nothing is trimmed or normalised.
func TestStringsExact(t *testing.T) {
	doc := open(t, `{"a": "  Rent \u20b9 \"x\" \t"}`)
	if cells := sel(t, doc, "$.a"); len(cells) != 1 || cells[0].Text != "  Rent ₹ \"x\" \t" {
		t.Fatalf("$.a = %+v", cells)
	}
}

func TestSizeLimit(t *testing.T) {
	text := `{"a": "0123456789"}` // 19 bytes
	a := New(19)
	if _, err := a.Open(strings.NewReader(text), int64(len(text))); err != nil {
		t.Fatalf("file at the limit refused: %v", err)
	}
	a = New(18)
	_, err := a.Open(strings.NewReader(text), int64(len(text)))
	var ie *ingest.Error
	if !errors.As(err, &ie) || ie.Code != ingest.FileTooLarge || !strings.Contains(ie.Msg, "19 bytes") || !strings.Contains(ie.Msg, "at most 18 bytes") {
		t.Fatalf("over the limit: err = %v", err)
	}
	if New(0).MaxBytes() != DefaultMaxBytes || New(-5).MaxBytes() != DefaultMaxBytes {
		t.Fatal("a zero or negative limit does not fall back to DefaultMaxBytes")
	}
	// A size larger than the actual data is a read problem, not a silent short file.
	if _, err := New(0).Open(strings.NewReader(text), int64(len(text))+5); !errors.As(err, &ie) || ie.Code != ingest.ParseError {
		t.Fatalf("short read: err = %v", err)
	}
	if _, err := New(0).Open(strings.NewReader(text), -1); !errors.As(err, &ie) || ie.Code != ingest.ParseError {
		t.Fatalf("negative size: err = %v", err)
	}
}

func TestSniff(t *testing.T) {
	a := New(0)
	for head, want := range map[string]bool{
		`{"a":1}`:           true,
		"  \r\n\t[1]":       true,
		"\xEF\xBB\xBF {}":   true,
		"":                  false,
		"   ":               false,
		"PK\x03\x04":        false, // xlsx/docx
		"%PDF-1.7":          false,
		"<?xml version":     false,
		"ledger,amount\n":   false,
		"\xFF\xFE{\x00":     false, // UTF-16 is not read
		"\x89PNG\r\n\x1a\n": false,
	} {
		if got := a.Sniff([]byte(head), "anything.json"); got != want {
			t.Errorf("Sniff(%q) = %v, want %v", head, got, want)
		}
	}
	// The name never decides.
	if a.Sniff([]byte("PK\x03\x04"), "comp.json") || !a.Sniff([]byte("{}"), "comp.pdf") {
		t.Error("Sniff used the file name")
	}
}

type otherSelector struct{}

func (otherSelector) FileType() string { return "xlsx" }
func (otherSelector) String() string   { return "Trial Balance!D42" }

func TestSelectRefusesOtherSelectors(t *testing.T) {
	doc := open(t, sample)
	if _, err := doc.Select(otherSelector{}); err == nil || !strings.Contains(err.Error(), "cannot use a xlsx selector") {
		t.Errorf("xlsx selector: err = %v", err)
	}
	if _, err := doc.Select(nil); err == nil {
		t.Error("nil selector accepted")
	}
	if _, err := doc.Select((*Path)(nil)); err == nil {
		t.Error("nil *Path accepted")
	}
	if _, err := doc.Select(Path{}); err == nil {
		t.Error("zero Path accepted")
	}
	p, _ := ParsePath("$.meta.form")
	if cells, err := doc.Select(&p); err != nil || len(cells) != 1 {
		t.Errorf("*Path selector: %v, %v", cells, err)
	}
	if p.FileType() != "json" || p.String() != "$.meta.form" || doc.FileType() != "json" {
		t.Error("FileType/String wrong")
	}
}

// IN-21: the same file always gives the same cells in the same order.
func TestDeterministic(t *testing.T) {
	var first []ingest.Cell
	for i := range 20 {
		cells := sel(t, open(t, sample), "$.rows[*].amount")
		cells = append(cells, sel(t, open(t, sample), "$")...)
		if i == 0 {
			first = cells
		} else if !reflect.DeepEqual(cells, first) {
			t.Fatalf("run %d differs: %+v vs %+v", i, cells, first)
		}
	}
}

// The adapter satisfies the ingest interfaces and registers under "json".
func TestRegisters(t *testing.T) {
	r := ingest.NewRegistry()
	if err := r.Register(New(0)); err != nil {
		t.Fatal(err)
	}
	a, err := r.ForFile([]byte(sample), "comp.json")
	if err != nil || a.FileType() != FileType {
		t.Fatalf("ForFile = %v, %v", a, err)
	}
	var _ ingest.Selector = Path{}
	var _ ingest.Doc = (*Doc)(nil)
	if !bytes.Equal([]byte(FileType), []byte("json")) {
		t.Fatal("FileType changed")
	}
}

func TestLeaves(t *testing.T) {
	doc := open(t, `{"b": [1, {"x": null}], "a": "t", "e": {}, "f": []}`)
	var got []string
	for _, c := range doc.Leaves() {
		got = append(got, c.Locator.Path+"="+string(c.Kind)+":"+c.Text)
	}
	want := []string{"$.b[0]=number:1", "$.b[1].x=null:", "$.a=text:t"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Leaves() = %q, want %q", got, want)
	}
	var _ ingest.LeafLister = doc
}

func TestHasWildcard(t *testing.T) {
	for path, want := range map[string]bool{"$": false, "$.a[0]": false, "$.a[*]": true, "$['x'][*].y": true} {
		p, err := ParsePath(path)
		if err != nil || p.HasWildcard() != want {
			t.Errorf("%s: HasWildcard() = %v (err %v), want %v", path, p.HasWildcard(), err, want)
		}
	}
}
