package csvadapter

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest"
	jsonadapter "github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/ingest/adapters/json"
)

// FileType is the registry key of the CSV adapter.
const FileType = "csv"

// DefaultMaxBytes is the size limit used when none is configured.
const DefaultMaxBytes int64 = 64 << 20

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// Adapter reads CSV files. It is safe for concurrent use.
type Adapter struct {
	maxBytes int64
}

// New returns a CSV adapter that refuses files over maxBytes (DefaultMaxBytes when 0 or less).
func New(maxBytes int64) *Adapter {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	return &Adapter{maxBytes: maxBytes}
}

// FileType implements ingest.Adapter.
func (a *Adapter) FileType() string { return FileType }

// Sniff reports whether the first line looks like a row of column names: UTF-8 text with
// a comma, not starting like JSON. The name is not used.
func (a *Adapter) Sniff(head []byte, _ string) bool {
	head = bytes.TrimPrefix(head, utf8BOM)
	line, _, _ := bytes.Cut(head, []byte("\n"))
	line = bytes.TrimRight(line, "\r")
	if len(line) == 0 || line[0] == '{' || line[0] == '[' || !utf8.Valid(line) || !bytes.ContainsRune(line, ',') {
		return false
	}
	for _, r := range string(line) {
		if r < ' ' && r != '\t' {
			return false
		}
	}
	return true
}

// Doc is a CSV file read as a list of rows.
type Doc struct {
	*jsonadapter.Doc
	tree []jsonadapter.Row // a file of the columns Path and Value: its rows (SPEC-011)
}

// FileType implements ingest.Doc.
func (d *Doc) FileType() string { return FileType }

// JSONTree implements ingest.Flattened: a file whose columns are Path and Value is the JSON
// document its rows describe, each value's place being its line.
func (d *Doc) JSONTree() (ingest.Doc, bool, error) {
	if d.tree == nil {
		return nil, false, nil
	}
	t, err := jsonadapter.BuildTree(d.tree)
	return t, err == nil, err
}

// Open reads the whole file. The first row names the columns: each must be named, once.
// Every row has as many cells as there are columns. Errors are *ingest.Error with code
// FILE_TOO_LARGE or PARSE_ERROR (with line and column).
func (a *Adapter) Open(r io.ReaderAt, size int64) (ingest.Doc, error) {
	switch {
	case size < 0:
		return nil, &ingest.Error{Code: ingest.ParseError, Msg: fmt.Sprintf("invalid file size %d", size)}
	case size > a.maxBytes:
		return nil, &ingest.Error{Code: ingest.FileTooLarge, Msg: fmt.Sprintf("the file is %d bytes; CSV files may be at most %d bytes", size, a.maxBytes)}
	}
	data, err := io.ReadAll(io.NewSectionReader(r, 0, size))
	if err != nil {
		return nil, &ingest.Error{Code: ingest.ParseError, Msg: "the file could not be read: " + err.Error()}
	}
	data = bytes.TrimPrefix(data, utf8BOM)
	if !utf8.Valid(data) {
		return nil, &ingest.Error{Code: ingest.ParseError, Msg: "the file is not UTF-8 text"}
	}
	cr := csv.NewReader(bytes.NewReader(data))
	header, err := cr.Read()
	if err != nil {
		return nil, parseError(err, "the first row (the column names) cannot be read")
	}
	seen := map[string]bool{}
	for i, h := range header {
		h = strings.TrimSpace(h)
		switch {
		case h == "":
			return nil, &ingest.Error{Code: ingest.ParseError, Line: 1, Msg: fmt.Sprintf("column %d has no name in the first row", i+1)}
		case seen[h]:
			return nil, &ingest.Error{Code: ingest.ParseError, Line: 1, Msg: fmt.Sprintf("the column %q is named twice in the first row", h)}
		}
		seen[h] = true
		header[i] = h
	}
	var tree []jsonadapter.Row
	if len(header) == 2 && strings.EqualFold(header[0], jsonadapter.TreeHeader[0]) && strings.EqualFold(header[1], jsonadapter.TreeHeader[1]) {
		tree = []jsonadapter.Row{}
	}
	var out bytes.Buffer
	out.WriteByte('[')
	for n := 0; ; n++ {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, parseError(err, "a row cannot be read")
		}
		if tree != nil {
			line, _ := cr.FieldPos(1)
			tree = append(tree, jsonadapter.Row{Path: strings.TrimSpace(rec[0]), Value: ingest.Cell{Kind: ingest.CellText, Text: rec[1],
				Locator: domain.Locator{Kind: domain.LocatorLine, Path: "line " + strconv.Itoa(line)}}})
		}
		if n > 0 {
			out.WriteByte(',')
		}
		out.WriteByte('{')
		for i, cell := range rec {
			if i > 0 {
				out.WriteByte(',')
			}
			k, _ := json.Marshal(header[i])
			v, _ := json.Marshal(cell)
			out.Write(k)
			out.WriteByte(':')
			out.Write(v)
		}
		out.WriteByte('}')
	}
	out.WriteByte(']')
	doc, err := jsonadapter.New(int64(out.Len())+1).Open(bytes.NewReader(out.Bytes()), int64(out.Len()))
	if err != nil {
		return nil, err
	}
	jd, ok := doc.(*jsonadapter.Doc)
	if !ok {
		return nil, errors.New("csv: the JSON reader returned an unexpected document")
	}
	return &Doc{Doc: jd, tree: tree}, nil
}

func parseError(err error, what string) error {
	var pe *csv.ParseError
	if errors.As(err, &pe) {
		return &ingest.Error{Code: ingest.ParseError, Line: pe.Line, Column: pe.Column, Msg: fmt.Sprintf("%s: %v", what, pe.Err)}
	}
	return &ingest.Error{Code: ingest.ParseError, Msg: fmt.Sprintf("%s: %v", what, err)}
}
