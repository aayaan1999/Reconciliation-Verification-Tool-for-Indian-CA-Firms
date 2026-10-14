package strictyaml

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Error is one problem in a YAML file. Line is 0 when the line is not known.
type Error struct {
	File string
	Line int
	Msg  string
}

func (e *Error) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("%s:%d: %s", e.File, e.Line, e.Msg)
	}
	return fmt.Sprintf("%s: %s", e.File, e.Msg)
}

// Errors is every problem found in one or more files, in file then line order.
type Errors []*Error

func (es Errors) Error() string {
	lines := make([]string, len(es))
	for i, e := range es {
		lines[i] = e.Error()
	}
	return strings.Join(lines, "\n")
}

// Sort orders the errors by file, then line, then message.
func (es Errors) Sort() {
	slices.SortStableFunc(es, func(a, b *Error) int {
		if c := strings.Compare(a.File, b.File); c != 0 {
			return c
		}
		if a.Line != b.Line {
			return a.Line - b.Line
		}
		return strings.Compare(a.Msg, b.Msg)
	})
}

// Positions gives the line of every value in a decoded file, by path: "schema",
// "facts[2]", "facts[2].name". Loaders use it to point their own checks at a line.
type Positions map[string]int

// Line returns the line of path, or 0 if the path is unknown.
func (p Positions) Line(path string) int { return p[path] }

// Decode reads the single YAML document in data into out, which must be a pointer to a
// struct whose fields carry `yaml:"name"` tags. file is used only in messages.
func Decode(file string, data []byte, out any) (Positions, error) {
	rv := reflect.ValueOf(out)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		panic("strictyaml: Decode needs a pointer to a struct")
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, Errors{{File: file, Msg: "the file is empty"}}
		}
		return nil, Errors{parseError(file, err)}
	}
	var extra yaml.Node
	switch err := dec.Decode(&extra); {
	case errors.Is(err, io.EOF):
	case err != nil:
		return nil, Errors{parseError(file, err)}
	default:
		return nil, Errors{{File: file, Line: extra.Line, Msg: `only one document is allowed per file; remove the "---" line`}}
	}
	if len(doc.Content) == 0 {
		return nil, Errors{{File: file, Msg: "the file is empty"}}
	}

	w := walker{file: file, pos: Positions{}}
	w.walk(doc.Content[0], rv.Elem().Type(), "")
	if len(w.errs) > 0 {
		w.errs.Sort()
		return nil, w.errs
	}

	// The walk has already checked everything KnownFields would; it stays on as a second guard.
	dec = yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return nil, Errors{parseError(file, err)}
	}
	return w.pos, nil
}

var lineInMessage = regexp.MustCompile(`line (\d+): `)

// parseError turns a yaml library error into an Error with the line it names.
func parseError(file string, err error) *Error {
	msg := strings.TrimPrefix(err.Error(), "yaml: ")
	msg = strings.TrimPrefix(msg, "unmarshal errors:\n  ")
	line := 0
	if m := lineInMessage.FindStringSubmatch(msg); m != nil {
		line, _ = strconv.Atoi(m[1])
		msg = strings.Replace(msg, m[0], "", 1)
	}
	return &Error{File: file, Line: line, Msg: "not valid YAML: " + msg}
}

type walker struct {
	file string
	pos  Positions
	errs Errors
}

func (w *walker) fail(n *yaml.Node, format string, args ...any) {
	w.errs = append(w.errs, &Error{File: w.file, Line: n.Line, Msg: fmt.Sprintf(format, args...)})
}

var wholeNumber = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)

func (w *walker) walk(n *yaml.Node, t reflect.Type, path string) {
	w.pos[path] = n.Line
	switch {
	case n.Kind == yaml.AliasNode:
		w.fail(n, "aliases (*%s) are not allowed; write the value out in full", n.Value)
		return
	case n.Anchor != "":
		w.fail(n, "anchors (&%s) are not allowed; write the value out in full", n.Anchor)
		return
	case n.Style&yaml.TaggedStyle != 0:
		w.fail(n, "tags such as %s are not allowed", n.Tag)
		return
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	where := describe(path)

	switch t.Kind() {
	case reflect.Struct:
		if n.Kind != yaml.MappingNode {
			w.fail(n, "%s must be a set of \"key: value\" lines", where)
			return
		}
		fields := yamlFields(t)
		seen := map[string]bool{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			key := k.Value
			switch {
			case key == "<<":
				w.fail(k, "merge keys (<<) are not allowed; write the values out in full")
				continue
			case seen[key]:
				w.fail(k, "key %q appears twice", key)
				continue
			}
			seen[key] = true
			ft, ok := fields[key]
			if !ok {
				w.fail(k, "unknown key %q in %s (allowed: %s)", key, where, strings.Join(sortedKeys(fields), ", "))
				continue
			}
			w.walk(v, ft, join(path, key))
		}
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			panic(fmt.Sprintf("strictyaml: map key type %s is not supported", t.Key()))
		}
		if n.Kind != yaml.MappingNode {
			w.fail(n, "%s must be a set of \"name: value\" lines", where)
			return
		}
		seen := map[string]bool{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			switch {
			case k.Kind != yaml.ScalarNode || k.Anchor != "" || k.Style&yaml.TaggedStyle != 0:
				w.fail(k, "a name in %s must be plain text", where)
				continue
			case k.Value == "<<":
				w.fail(k, "merge keys (<<) are not allowed; write the values out in full")
				continue
			case seen[k.Value]:
				w.fail(k, "name %q appears twice in %s", k.Value, where)
				continue
			}
			seen[k.Value] = true
			w.walk(v, t.Elem(), join(path, k.Value))
		}
	case reflect.Slice:
		if n.Kind != yaml.SequenceNode {
			w.fail(n, "%s must be a list", where)
			return
		}
		for i, item := range n.Content {
			w.walk(item, t.Elem(), fmt.Sprintf("%s[%d]", path, i))
		}
	case reflect.String:
		switch {
		case n.Kind != yaml.ScalarNode:
			w.fail(n, "%s must be text in quotes", where)
		case n.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle|yaml.LiteralStyle|yaml.FoldedStyle) == 0:
			w.fail(n, "%s is text and must be in quotes: write \"%s\"", where, n.Value)
		}
	case reflect.Bool:
		if n.Kind != yaml.ScalarNode || n.Style != 0 || (n.Value != "true" && n.Value != "false") {
			w.fail(n, "%s must be true or false (without quotes)", where)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if n.Kind != yaml.ScalarNode || n.Style != 0 || !wholeNumber.MatchString(n.Value) {
			w.fail(n, "%s must be a whole number written in plain digits (without quotes)", where)
		}
	default:
		panic(fmt.Sprintf("strictyaml: field type %s is not supported", t))
	}
}

// yamlFields maps each `yaml:"name"` tag of struct type t to its field type.
func yamlFields(t reflect.Type) map[string]reflect.Type {
	fields := map[string]reflect.Type{}
	for f := range t.Fields() {
		name, _, _ := strings.Cut(f.Tag.Get("yaml"), ",")
		if name == "" || name == "-" {
			if f.IsExported() && name == "" {
				panic(fmt.Sprintf("strictyaml: field %s.%s has no yaml tag", t.Name(), f.Name))
			}
			continue
		}
		fields[name] = f.Type
	}
	return fields
}

func sortedKeys(m map[string]reflect.Type) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

// describe names a path for a message: "the file" for the root, otherwise the path.
func describe(path string) string {
	if path == "" {
		return "the file"
	}
	return path
}
