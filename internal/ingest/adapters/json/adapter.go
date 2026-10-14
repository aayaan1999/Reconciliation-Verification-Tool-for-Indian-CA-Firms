package jsonadapter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
)

// FileType is the registry key of the JSON adapter.
const FileType = "json"

// DefaultMaxBytes is the size limit used when none is configured. Tax-suite JSON is
// usually well under 5 MB; the limit stops a wrong upload from exhausting memory.
const DefaultMaxBytes int64 = 64 << 20

// maxDepth limits nesting, so a hostile file cannot exhaust the stack.
const maxDepth = 512

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// Adapter reads JSON files. It is safe for concurrent use.
type Adapter struct {
	maxBytes int64
}

// New returns a JSON adapter that refuses files over maxBytes (DefaultMaxBytes when
// maxBytes is 0 or less).
func New(maxBytes int64) *Adapter {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	return &Adapter{maxBytes: maxBytes}
}

// FileType implements ingest.Adapter.
func (a *Adapter) FileType() string { return FileType }

// MaxBytes returns the size limit.
func (a *Adapter) MaxBytes() int64 { return a.maxBytes }

// Sniff reports whether head starts, after an optional UTF-8 byte-order mark and white
// space, with '{' or '['. The name is not used.
func (a *Adapter) Sniff(head []byte, _ string) bool {
	head = bytes.TrimPrefix(head, utf8BOM)
	head = bytes.TrimLeft(head, " \t\r\n")
	return len(head) > 0 && (head[0] == '{' || head[0] == '[')
}

// Open reads and parses the whole file. Errors are *ingest.Error with code
// FILE_TOO_LARGE or PARSE_ERROR; a PARSE_ERROR carries the line, column and byte offset.
func (a *Adapter) Open(r io.ReaderAt, size int64) (ingest.Doc, error) {
	switch {
	case size < 0:
		return nil, &ingest.Error{Code: ingest.ParseError, Msg: fmt.Sprintf("invalid file size %d", size)}
	case size > a.maxBytes:
		return nil, &ingest.Error{Code: ingest.FileTooLarge,
			Msg: fmt.Sprintf("the file is %d bytes; JSON files may be at most %d bytes", size, a.maxBytes)}
	}
	data, err := io.ReadAll(io.NewSectionReader(r, 0, size))
	if err != nil {
		return nil, &ingest.Error{Code: ingest.ParseError, Msg: "the file could not be read: " + err.Error()}
	}
	if int64(len(data)) != size {
		return nil, &ingest.Error{Code: ingest.ParseError, Msg: fmt.Sprintf("the file is %d bytes, not the expected %d", len(data), size)}
	}
	root, err := parse(data)
	if err != nil {
		return nil, err
	}
	return &Doc{root: root}, nil
}

// node is one JSON value. Objects keep their members in file order.
type node struct {
	kind  ingest.CellKind
	text  string           // strings, numbers ("112450.50" exactly as written), "true", "false"
	keys  []string         // object member names, in file order
	vals  []*node          // object member values, parallel to keys; array items
	index map[string]*node // object member by name
}

// parser builds the node tree from the token stream. Offsets in errors are positions in
// the original file, byte-order mark included.
type parser struct {
	data []byte // the whole file
	skip int64  // length of a leading byte-order mark
	dec  *json.Decoder
}

func parse(data []byte) (*node, error) {
	p := &parser{data: data}
	body := data
	if bytes.HasPrefix(body, utf8BOM) {
		body, p.skip = body[len(utf8BOM):], int64(len(utf8BOM))
	}
	// encoding/json silently replaces invalid UTF-8 with U+FFFD; refuse it instead, so a
	// value is never changed on the way in.
	if !utf8.Valid(body) {
		at := 0
		for at < len(body) {
			r, n := utf8.DecodeRune(body[at:])
			if r == utf8.RuneError && n <= 1 {
				break
			}
			at += n
		}
		return nil, p.fail(int64(at), "the file is not valid UTF-8 text")
	}
	if len(bytes.TrimLeft(body, " \t\r\n")) == 0 {
		return nil, &ingest.Error{Code: ingest.ParseError, Msg: "the file is empty"}
	}
	p.dec = json.NewDecoder(bytes.NewReader(body))
	p.dec.UseNumber() // numbers stay exact text, never float64
	root, err := p.value(0)
	if err != nil {
		return nil, err
	}
	off := p.dec.InputOffset()
	if _, err := p.dec.Token(); !errors.Is(err, io.EOF) {
		return nil, p.fail(nextNonSpace(body, off), "unexpected text after the end of the JSON value")
	}
	return root, nil
}

func (p *parser) value(depth int) (*node, error) {
	if depth > maxDepth {
		return nil, p.fail(p.dec.InputOffset(), fmt.Sprintf("values are nested more than %d levels deep", maxDepth))
	}
	tok, err := p.dec.Token()
	if err != nil {
		return nil, p.syntax(err)
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			return p.object(depth)
		case '[':
			return p.array(depth)
		}
		return nil, p.fail(p.dec.InputOffset()-1, fmt.Sprintf("unexpected %q", rune(t)))
	case string:
		return &node{kind: ingest.CellText, text: t}, nil
	case json.Number:
		return &node{kind: ingest.CellNumber, text: string(t)}, nil
	case bool:
		if t {
			return &node{kind: ingest.CellBool, text: "true"}, nil
		}
		return &node{kind: ingest.CellBool, text: "false"}, nil
	case nil:
		return &node{kind: ingest.CellNull}, nil
	}
	return nil, p.fail(p.dec.InputOffset(), fmt.Sprintf("unexpected token %v", tok))
}

func (p *parser) object(depth int) (*node, error) {
	n := &node{kind: ingest.CellObject, index: map[string]*node{}}
	for p.dec.More() {
		keyAt := nextKey(p.data[p.skip:], p.dec.InputOffset())
		tok, err := p.dec.Token()
		if err != nil {
			return nil, p.syntax(err)
		}
		key, ok := tok.(string)
		if !ok {
			return nil, p.fail(keyAt, "an object key must be a quoted string")
		}
		if _, dup := n.index[key]; dup {
			// encoding/json would silently keep the last one; a figure must never be
			// read from one of two conflicting copies.
			return nil, p.fail(keyAt, fmt.Sprintf("the key %q appears twice in the same object", key))
		}
		v, err := p.value(depth + 1)
		if err != nil {
			return nil, err
		}
		n.keys, n.vals = append(n.keys, key), append(n.vals, v)
		n.index[key] = v
	}
	if _, err := p.dec.Token(); err != nil { // the closing '}'
		return nil, p.syntax(err)
	}
	return n, nil
}

func (p *parser) array(depth int) (*node, error) {
	n := &node{kind: ingest.CellArray}
	for p.dec.More() {
		v, err := p.value(depth + 1)
		if err != nil {
			return nil, err
		}
		n.vals = append(n.vals, v)
	}
	if _, err := p.dec.Token(); err != nil { // the closing ']'
		return nil, p.syntax(err)
	}
	return n, nil
}

// syntax converts a decoder error into a PARSE_ERROR at the right place.
func (p *parser) syntax(err error) error {
	const truncated = "the file ends before the JSON value is complete"
	var se *json.SyntaxError
	switch {
	case errors.As(err, &se) && se.Error() == "unexpected end of JSON input",
		errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		return p.fail(int64(len(p.data))-p.skip, truncated)
	case errors.As(err, &se):
		// Offset counts the bytes read, including the offending one; point at that byte.
		return p.fail(max(se.Offset-1, 0), se.Error())
	}
	return p.fail(p.dec.InputOffset(), err.Error())
}

// fail returns a PARSE_ERROR at offset (counted after the byte-order mark).
func (p *parser) fail(offset int64, msg string) error {
	offset += p.skip
	line, col := ingest.Position(p.data, offset)
	return &ingest.Error{Code: ingest.ParseError, Line: line, Column: col, Offset: offset, Msg: msg}
}

// nextNonSpace returns the offset of the first byte at or after off that is not JSON
// white space.
func nextNonSpace(b []byte, off int64) int64 {
	for off < int64(len(b)) && (b[off] == ' ' || b[off] == '\t' || b[off] == '\r' || b[off] == '\n') {
		off++
	}
	return off
}

// nextKey returns the offset of the next object key at or after off, skipping white
// space and the comma between members.
func nextKey(b []byte, off int64) int64 {
	off = nextNonSpace(b, off)
	if off < int64(len(b)) && b[off] == ',' {
		off = nextNonSpace(b, off+1)
	}
	return off
}

// Doc is a parsed JSON file. It is read-only and safe for concurrent use.
type Doc struct {
	root *node
}

// FileType implements ingest.Doc.
func (d *Doc) FileType() string { return FileType }

// Select returns the cells sel addresses, in file order, each with its full concrete path
// ("$.rows[3].amount") as a json_path locator. sel must be a Path.
func (d *Doc) Select(sel ingest.Selector) ([]ingest.Cell, error) {
	var p Path
	switch s := sel.(type) {
	case Path:
		p = s
	case *Path:
		if s == nil {
			return nil, errors.New("jsonadapter: nil selector")
		}
		p = *s
	default:
		if sel == nil {
			return nil, errors.New("jsonadapter: nil selector")
		}
		return nil, fmt.Errorf("jsonadapter: a JSON document cannot use a %s selector (%q)", sel.FileType(), sel.String())
	}
	if p.text == "" {
		return nil, errors.New("jsonadapter: the selector is an unparsed Path; use ParsePath")
	}

	type at struct {
		n    *node
		path string
	}
	cur := []at{{d.root, "$"}}
	for _, seg := range p.segs {
		var next []at
		for _, c := range cur {
			switch seg.kind {
			case segKey:
				if c.n.kind == ingest.CellObject {
					if v, ok := c.n.index[seg.key]; ok {
						next = append(next, at{v, appendKey(c.path, seg.key)})
					}
				}
			case segIndex:
				if c.n.kind == ingest.CellArray && seg.index < len(c.n.vals) {
					next = append(next, at{c.n.vals[seg.index], appendIndex(c.path, seg.index)})
				}
			case segAll:
				if c.n.kind == ingest.CellArray {
					for i, v := range c.n.vals {
						next = append(next, at{v, appendIndex(c.path, i)})
					}
				}
			}
		}
		cur = next
	}

	cells := make([]ingest.Cell, len(cur))
	for i, c := range cur {
		cells[i] = cell(c.n, c.path)
	}
	return cells, nil
}

// Leaves implements ingest.LeafLister: every string, number, bool and null in the file,
// in file order, with its concrete path.
func (d *Doc) Leaves() []ingest.Cell {
	var out []ingest.Cell
	var walk func(n *node, path string)
	walk = func(n *node, path string) {
		switch n.kind {
		case ingest.CellObject:
			for i, k := range n.keys {
				walk(n.vals[i], appendKey(path, k))
			}
		case ingest.CellArray:
			for i, v := range n.vals {
				walk(v, appendIndex(path, i))
			}
		default:
			out = append(out, cell(n, path))
		}
	}
	walk(d.root, "$")
	return out
}

func cell(n *node, path string) ingest.Cell {
	return ingest.Cell{Kind: n.kind, Text: n.text, Locator: domain.Locator{Kind: domain.LocatorJSONPath, Path: path}}
}
