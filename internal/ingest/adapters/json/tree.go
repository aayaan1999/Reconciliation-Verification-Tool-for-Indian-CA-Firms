package jsonadapter

import (
	"fmt"
	"regexp"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
)

// TreeHeader is the header row of a table that holds a JSON document flattened, one value
// per row (SPEC-011): its JSON path, then the value.
var TreeHeader = [2]string{"Path", "Value"}

// A Row is one value of a flattened JSON document: the JSON path it has in the document
// ("$.form_67[0].country") and the value, whose locator is its place in the file.
type Row struct {
	Path  string
	Value ingest.Cell
}

// A Tree is a JSON document rebuilt from a table of paths and values, such as a tax suite's
// filing pack exported as CSV or Excel. JSON profiles read it as the document itself;
// Source gives the place in the file each value came from.
type Tree struct {
	*Doc
	from map[string]domain.Locator // concrete JSON path → the value's place in the file
}

// Source returns the place in the file ("Filing pack!B14", "line 15") of the value at the
// concrete JSON path p.
func (t *Tree) Source(p string) (domain.Locator, bool) {
	l, ok := t.from[p]
	return l, ok
}

var jsonNumber = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

// BuildTree rebuilds the document rows describe. A text value is read as JSON would write
// it: "true" and "false" are booleans, "null" is null, "[]" and "{}" are an empty list or
// object, a JSON number is a number, and anything else is text, an empty value included. Lists are
// numbered from 0 in order. A path given twice, a value where another row has a list or
// object, or a skipped list number is a PARSE_ERROR naming the row: a figure is never read
// from one of two conflicting copies.
func BuildTree(rows []Row) (*Tree, error) {
	root := &node{kind: ingest.CellObject, index: map[string]*node{}}
	t := &Tree{Doc: &Doc{root: root}, from: map[string]domain.Locator{}}
	for _, r := range rows {
		fail := func(format string, args ...any) error {
			return &ingest.Error{Code: ingest.ParseError, Msg: fmt.Sprintf("%s: %s", r.Value.Locator.Path, fmt.Sprintf(format, args...))}
		}
		p, err := ParsePath(r.Path)
		if err != nil {
			return nil, fail("%v", err)
		}
		if len(p.segs) == 0 || p.HasWildcard() {
			return nil, fail("the path %q must name one value inside the document, without [*]", r.Path)
		}
		leaf := value(r.Value)
		n, at := root, "$"
		for i, s := range p.segs {
			last := i == len(p.segs)-1
			next := &node{kind: ingest.CellObject, index: map[string]*node{}}
			if last {
				next = leaf
			} else if p.segs[i+1].kind == segIndex {
				next = &node{kind: ingest.CellArray}
			}
			var child *node
			switch {
			case s.kind == segKey && n.kind == ingest.CellObject:
				at = appendKey(at, s.key)
				if child = n.index[s.key]; child == nil {
					child = next
					n.keys, n.vals = append(n.keys, s.key), append(n.vals, child)
					n.index[s.key] = child
				}
			case s.kind == segIndex && n.kind == ingest.CellArray:
				at = appendIndex(at, s.index)
				switch {
				case s.index < len(n.vals):
					child = n.vals[s.index]
				case s.index == len(n.vals):
					child = next
					n.vals = append(n.vals, child)
				default:
					return nil, fail("%q skips a list number; number a list's items from 0 in order", r.Path)
				}
			default:
				return nil, fail("%q goes inside %s, which another row gives as a single value or a different kind of container", r.Path, at)
			}
			if last && child != leaf {
				return nil, fail("%q is given twice, or also holds other values", r.Path)
			}
			if !last && child.kind != next.kind {
				return nil, fail("%q goes inside %s, which another row gives as a single value or a different kind of container", r.Path, at)
			}
			n = child
		}
		t.from[at] = r.Value.Locator
	}
	return t, nil
}

// value is the node of a flattened value, typed as JSON would write it.
func value(c ingest.Cell) *node {
	if c.Kind != ingest.CellText {
		return &node{kind: c.Kind, text: c.Text}
	}
	switch {
	case c.Text == "true" || c.Text == "false":
		return &node{kind: ingest.CellBool, text: c.Text}
	case c.Text == "null":
		return &node{kind: ingest.CellNull}
	case c.Text == "[]":
		return &node{kind: ingest.CellArray}
	case c.Text == "{}":
		return &node{kind: ingest.CellObject, index: map[string]*node{}}
	case jsonNumber.MatchString(c.Text):
		return &node{kind: ingest.CellNumber, text: c.Text}
	}
	return &node{kind: ingest.CellText, text: c.Text}
}
