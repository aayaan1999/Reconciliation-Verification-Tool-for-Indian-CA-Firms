package strictyaml

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

type mapEntry struct {
	Fact     string `yaml:"fact"`
	Optional bool   `yaml:"optional"`
}

type mapDoc struct {
	Inputs map[string]mapEntry `yaml:"inputs"`
	Counts map[string]int      `yaml:"counts"`
}

func TestMapsAreDecodedStrictly(t *testing.T) {
	text := "inputs:\n  a: { fact: \"x.y\" }\n  b:\n    fact: \"x.z\"\n    optional: true\ncounts:\n  \"2025-26\": 3\n"
	var d mapDoc
	pos, err := Decode("m.yaml", []byte(text), &d)
	if err != nil {
		t.Fatal(err)
	}
	want := mapDoc{
		Inputs: map[string]mapEntry{"a": {Fact: "x.y"}, "b": {Fact: "x.z", Optional: true}},
		Counts: map[string]int{"2025-26": 3},
	}
	if !reflect.DeepEqual(d, want) {
		t.Fatalf("decoded %+v, want %+v", d, want)
	}
	for path, line := range map[string]int{"inputs.a": 2, "inputs.a.fact": 2, "inputs.b.optional": 5, "counts.2025-26": 7} {
		if pos.Line(path) != line {
			t.Errorf("Line(%q) = %d, want %d", path, pos.Line(path), line)
		}
	}
}

func TestMapRefusals(t *testing.T) {
	for _, c := range []struct {
		name, text string
		line       int
		msg        string
	}{
		{"unquoted value inside", "inputs:\n  a: { fact: x.y }\n", 2, `inputs.a.fact is text and must be in quotes`},
		{"unknown key inside", "inputs:\n  a: { fact: \"x\", label: \"l\" }\n", 2, `unknown key "label"`},
		{"yes as bool inside", "inputs:\n  a: { fact: \"x\", optional: yes }\n", 2, `inputs.a.optional must be true or false`},
		{"duplicate name", "inputs:\n  a: { fact: \"x\" }\n  a: { fact: \"y\" }\n", 3, `name "a" appears twice in inputs`},
		{"merge key", "inputs:\n  <<: { fact: \"x\" }\n", 2, `merge keys (<<) are not allowed`},
		{"alias value", "inputs:\n  a: &e { fact: \"x\" }\n  b: *e\n", 2, `anchors (&e) are not allowed`},
		{"tagged name", "inputs:\n  !!str a: { fact: \"x\" }\n", 2, `a name in inputs must be plain text`},
		{"list instead of map", "inputs:\n  - { fact: \"x\" }\n", 2, `inputs must be a set of "name: value" lines`},
		{"text instead of map", "inputs: \"a\"\n", 1, `inputs must be a set of "name: value" lines`},
		{"bad number", "counts:\n  a: 0x10\n", 2, `counts.a must be a whole number`},
	} {
		t.Run(c.name, func(t *testing.T) {
			var d mapDoc
			_, err := Decode("m.yaml", []byte(c.text), &d)
			var es Errors
			if !errors.As(err, &es) {
				t.Fatalf("err = %v, want Errors", err)
			}
			for _, e := range es {
				if strings.Contains(e.Msg, c.msg) {
					if e.Line != c.line {
						t.Errorf("%q at line %d, want %d", c.msg, e.Line, c.line)
					}
					return
				}
			}
			t.Errorf("no error containing %q; got %v", c.msg, err)
		})
	}
}

func TestMapWithNonStringKeyPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("no panic for map[int]string")
		}
	}()
	var d struct {
		M map[int]string `yaml:"m"`
	}
	_, _ = Decode("m.yaml", []byte("m:\n  1: \"a\"\n"), &d)
}

func TestBlockScalarsAreText(t *testing.T) {
	var d struct {
		A string `yaml:"a"`
		B string `yaml:"b"`
	}
	if _, err := Decode("b.yaml", []byte("a: |-\n  line 1\n  line 2\nb: >-\n  folded\n  text\n"), &d); err != nil {
		t.Fatal(err)
	}
	if d.A != "line 1\nline 2" || d.B != "folded text" {
		t.Fatalf("decoded %+v", d)
	}
}
