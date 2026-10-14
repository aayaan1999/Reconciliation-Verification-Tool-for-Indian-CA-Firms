package jsonadapter

import (
	"fmt"
	"strconv"
	"strings"
)

type segKind int

const (
	segKey   segKind = iota + 1 // .key or ['key']
	segIndex                    // [n]
	segAll                      // [*]
)

type segment struct {
	kind  segKind
	key   string
	index int
}

// A Path is a parsed JSON path in the supported subset (see the package comment). It is
// the ingest.Selector for JSON documents.
type Path struct {
	text string
	segs []segment
}

// FileType implements ingest.Selector.
func (p Path) FileType() string { return FileType }

// String returns the path as it was written.
func (p Path) String() string { return p.text }

// HasWildcard reports whether the path contains [*], so it can select many cells.
func (p Path) HasWildcard() bool {
	for _, s := range p.segs {
		if s.kind == segAll {
			return true
		}
	}
	return false
}

// A PathError is a JSON path outside the supported subset. Profiles parse their paths when
// they load, so this is a profile load error, never a run-time one.
type PathError struct {
	Path   string
	Offset int // 0-based byte position in Path where the problem starts
	Msg    string
}

func (e *PathError) Error() string {
	return fmt.Sprintf("JSON path %q, at character %d: %s (supported: $, .key, ['key'], [n], [*])", e.Path, e.Offset+1, e.Msg)
}

// maxIndexDigits keeps [n] well inside int on every platform.
const maxIndexDigits = 9

// ParsePath parses s, refusing anything outside the supported subset.
func ParsePath(s string) (Path, error) {
	fail := func(at int, format string, args ...any) (Path, error) {
		return Path{}, &PathError{Path: s, Offset: at, Msg: fmt.Sprintf(format, args...)}
	}
	if !strings.HasPrefix(s, "$") {
		return fail(0, "a path must start with $")
	}
	p := Path{text: s}
	i := 1
	for i < len(s) {
		switch s[i] {
		case '.':
			i++
			switch {
			case i == len(s):
				return fail(i-1, "a key name must follow '.'")
			case s[i] == '.':
				return fail(i-1, "recursive descent (..) is not supported")
			case s[i] == '*':
				return fail(i-1, "'.*' is not supported; use [*] to take every row of a list")
			}
			start := i
			for i < len(s) && isKeyChar(s[i], i == start) {
				i++
			}
			if i == start {
				return fail(start, "a key name after '.' must start with a letter or _; write other keys as ['key']")
			}
			p.segs = append(p.segs, segment{kind: segKey, key: s[start:i]})
		case '[':
			seg, next, err := parseBracket(s, i)
			if err != nil {
				return Path{}, err
			}
			p.segs = append(p.segs, seg)
			i = next
		default:
			return fail(i, "unexpected %q; expected '.' or '['", s[i])
		}
	}
	return p, nil
}

// parseBracket parses the [...] segment starting at s[open] == '['. It returns the
// segment and the position after the closing ']'.
func parseBracket(s string, open int) (segment, int, error) {
	fail := func(at int, format string, args ...any) (segment, int, error) {
		return segment{}, 0, &PathError{Path: s, Offset: at, Msg: fmt.Sprintf(format, args...)}
	}
	i := open + 1
	if i == len(s) {
		return fail(open, "'[' is not closed")
	}
	var seg segment
	switch c := s[i]; {
	case c == '\'':
		var key strings.Builder
		i++
		for {
			if i == len(s) {
				return fail(open, "the quoted key is not closed with '")
			}
			if s[i] == '\'' {
				break
			}
			if s[i] == '\\' {
				if i+1 == len(s) || (s[i+1] != '\'' && s[i+1] != '\\') {
					return fail(i, `only \' and \\ may be escaped inside a quoted key`)
				}
				i++
			}
			key.WriteByte(s[i])
			i++
		}
		i++ // past the closing quote
		seg = segment{kind: segKey, key: key.String()}
	case c == '"':
		return fail(i, `write quoted keys with single quotes: ['key']`)
	case c == '*':
		i++
		seg = segment{kind: segAll}
	case c >= '0' && c <= '9':
		start := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		digits := s[start:i]
		switch {
		case len(digits) > 1 && digits[0] == '0':
			return fail(start, "an index must not have leading zeros")
		case len(digits) > maxIndexDigits:
			return fail(start, "the index is too large")
		}
		if i < len(s) && s[i] == ':' {
			return fail(i, "slices ([a:b]) are not supported")
		}
		if i < len(s) && s[i] == ',' {
			return fail(i, "unions ([a,b]) are not supported")
		}
		n, _ := strconv.Atoi(digits)
		seg = segment{kind: segIndex, index: n}
	case c == '-':
		return fail(i, "negative indexes are not supported")
	case c == '?' || c == '(':
		return fail(i, "filters and expressions are not supported")
	case c == ':':
		return fail(i, "slices ([a:b]) are not supported")
	default:
		return fail(i, "unexpected %q inside [ ]", c)
	}
	if i == len(s) || s[i] != ']' {
		if i < len(s) && s[i] == ',' {
			return fail(i, "unions ([a,b]) are not supported")
		}
		return fail(open, "'[' is not closed with ']'")
	}
	return seg, i + 1, nil
}

func isKeyChar(c byte, first bool) bool {
	switch {
	case c == '_', c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		return true
	case c >= '0' && c <= '9':
		return !first
	}
	return false
}

// appendKey writes one key step of a concrete path: ".key" when the key is a plain name,
// otherwise "['key']", so every concrete path parses back to the same cell.
func appendKey(path, key string) string {
	plain := key != ""
	for i := 0; i < len(key) && plain; i++ {
		plain = isKeyChar(key[i], i == 0)
	}
	if plain {
		return path + "." + key
	}
	r := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	return path + "['" + r.Replace(key) + "']"
}

func appendIndex(path string, n int) string { return path + "[" + strconv.Itoa(n) + "]" }
