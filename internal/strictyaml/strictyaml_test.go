package strictyaml

import (
	"errors"
	"strings"
	"testing"
)

type item struct {
	Name  string   `yaml:"name"`
	Count int      `yaml:"count"`
	On    bool     `yaml:"on"`
	Tags  []string `yaml:"tags"`
}

type file struct {
	Schema  string `yaml:"schema"`
	Version int    `yaml:"version"`
	Items   []item `yaml:"items"`
}

func TestDecodeAcceptsStrictFile(t *testing.T) {
	src := `# a comment
schema: "test/1"
version: 3
items:
  - name: "a"
    count: -2
    on: true
    tags: ["x", 'y']
  - name: ""
    count: 0
    on: false
    tags: []
`
	var f file
	pos, err := Decode("t.yaml", []byte(src), &f)
	if err != nil {
		t.Fatal(err)
	}
	if f.Schema != "test/1" || f.Version != 3 || len(f.Items) != 2 || f.Items[0].Count != -2 || !f.Items[0].On ||
		strings.Join(f.Items[0].Tags, ",") != "x,y" || f.Items[1].Name != "" {
		t.Errorf("decoded %+v", f)
	}
	if pos.Line("items[1].name") != 9 || pos.Line("version") != 3 || pos.Line("missing") != 0 {
		t.Errorf("positions wrong: items[1].name=%d version=%d", pos.Line("items[1].name"), pos.Line("version"))
	}
}

func TestDecodeRefuses(t *testing.T) {
	cases := []struct {
		name, src, want string
		line            int
	}{
		{"unquoted text", "schema: test/1\n", `schema is text and must be in quotes: write "test/1"`, 1},
		{"unquoted tax year in list", "schema: \"s\"\nitems:\n  - tags: [2025-26]\n", `items[0].tags[0] is text and must be in quotes`, 3},
		{"unquoted number as text", "schema: 1\n", `must be in quotes: write "1"`, 1},
		{"empty text value", "schema:\n", `schema is text and must be in quotes`, 1},
		{"block text", "schema: |\n  x\n", `must be in quotes`, 1},
		{"unknown key", "schema: \"s\"\nversoin: 1\n", `unknown key "versoin" in the file (allowed: items, schema, version)`, 2},
		{"duplicate key", "schema: \"a\"\nschema: \"b\"\n", `key "schema" appears twice`, 2},
		{"yes as bool", "items:\n  - on: yes\n", `items[0].on must be true or false`, 2},
		{"quoted bool", "items:\n  - on: \"true\"\n", `items[0].on must be true or false`, 2},
		{"quoted number", "version: \"1\"\n", `version must be a whole number`, 1},
		{"hex number", "version: 0x10\n", `whole number`, 1},
		{"underscore number", "version: 1_000\n", `whole number`, 1},
		{"plus number", "version: +1\n", `whole number`, 1},
		{"float", "version: 1.5\n", `whole number`, 1},
		{"leading zero", "version: 01\n", `whole number`, 1},
		{"anchor", "schema: &a \"s\"\n", `anchors (&a) are not allowed`, 1},
		{"alias", "items:\n  - &i {name: \"a\"}\n  - *i\n", `anchors (&i)`, 2},
		{"merge key", "items:\n  - <<: {name: \"a\"}\n", `merge keys (<<) are not allowed`, 2},
		{"tag", "schema: !!str s\n", `tags such as !!str are not allowed`, 1},
		{"custom tag", "schema: !secret \"s\"\n", `tags such as !secret`, 1},
		{"two documents", "schema: \"a\"\n---\nschema: \"b\"\n", `only one document is allowed`, 2},
		{"empty file", "", "the file is empty", 0},
		{"only comments", "# nothing\n", "the file is empty", 0},
		{"list where mapping expected", "- a\n", `the file must be a set of "key: value" lines`, 1},
		{"mapping where list expected", "items: {name: \"a\"}\n", `items must be a list`, 1},
		{"bad syntax", "schema: \"a\n", `not valid YAML`, 0},
		{"tab indentation", "items:\n\t- name: \"a\"\n", `not valid YAML`, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var f file
			_, err := Decode("bad.yaml", []byte(c.src), &f)
			var es Errors
			if !errors.As(err, &es) || len(es) == 0 {
				t.Fatalf("err = %v, want Errors", err)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q does not contain %q", err, c.want)
			}
			if !strings.HasPrefix(err.Error(), "bad.yaml") {
				t.Errorf("error %q does not name the file", err)
			}
			if c.line > 0 && es[0].Line != c.line {
				t.Errorf("line = %d, want %d (%v)", es[0].Line, c.line, err)
			}
		})
	}
}

// Every problem in a file is reported at once, in line order.
func TestDecodeReportsAllProblems(t *testing.T) {
	src := "version: \"1\"\nschema: s\nitems:\n  - name: n\n    extra: 1\n"
	var f file
	_, err := Decode("many.yaml", []byte(src), &f)
	var es Errors
	if !errors.As(err, &es) || len(es) != 4 {
		t.Fatalf("want 4 errors, got %v", err)
	}
	for i, want := range []int{1, 2, 4, 5} {
		if es[i].Line != want {
			t.Errorf("error %d at line %d, want %d", i, es[i].Line, want)
		}
	}
}

func TestDecodeNeedsStructPointer(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("no panic for a non-pointer target")
		}
	}()
	Decode("x.yaml", []byte("a: 1"), file{})
}
