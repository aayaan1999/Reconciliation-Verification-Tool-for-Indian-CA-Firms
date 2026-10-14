package strictyaml

import (
	"errors"
	"strings"
	"testing"
)

// refuse decodes src into the shared test shape and returns the Errors, failing the test
// if decoding succeeded or returned something that is not Errors.
func refuse(t *testing.T, src string) Errors {
	t.Helper()
	var f file
	_, err := Decode("x.yaml", []byte(src), &f)
	if err == nil {
		t.Fatalf("accepted %q", src)
	}
	var es Errors
	if !errors.As(err, &es) || len(es) == 0 {
		t.Fatalf("err = %T %v, want non-empty Errors", err, err)
	}
	return es
}

func TestExtraStructuralRefusals(t *testing.T) {
	cases := []struct {
		name, src, want string
		line            int
	}{
		{"duplicate key nested", "items:\n  - name: \"a\"\n    name: \"b\"\n", `key "name" appears twice`, 3},
		{"duplicate key reported at second", "version: 1\nschema: \"a\"\nversion: 2\n", `key "version" appears twice`, 3},
		{"triple duplicate", "version: 1\nversion: 2\nversion: 3\n", `appears twice`, 2},
		{"quoted merge key", "\"<<\": 1\n", `merge keys`, 1},
		{"alias to undefined anchor", "schema: *nope\n", `not valid YAML`, 0},
		{"anchor on mapping value", "items: &i\n  - name: \"a\"\n", `anchors (&i)`, 1},
		{"anchor on sequence item", "items:\n  - &i\n    name: \"a\"\n", `anchors (&i)`, 2},
		{"alias after anchor", "schema: &a \"x\"\nversion: *a\n", `anchors (&a)`, 1},
		{"tag !!int", "version: !!int 3\n", `tags such as !!int`, 1},
		{"tag !!binary", "schema: !!binary aGk=\n", `tags such as !!binary`, 1},
		{"tag !!timestamp", "schema: !!timestamp 2025-04-01\n", `tags such as !!timestamp`, 1},
		{"tag on document", "--- !!map\nschema: \"a\"\n", `tags such as !!map`, 1},
		{"tag on list", "items: !!seq []\n", `tags such as !!seq`, 1},
		{"tag on bool", "items:\n  - on: !!bool true\n", `tags such as !!bool`, 2},
		{"two documents with trailing", "schema: \"a\"\n---\n", `only one document`, 2},
		{"two documents second invalid", "schema: \"a\"\n---\nschema: \"b\n", `not valid YAML`, 0},
		{"three documents", "schema: \"a\"\n---\nschema: \"b\"\n---\nschema: \"c\"\n", `only one document`, 2},
		{"leading separator then second", "---\nschema: \"a\"\n---\nversion: 1\n", `only one document`, 3},
		{"integer key", "1: \"a\"\n", `unknown key "1"`, 1},
		{"boolean key", "true: \"a\"\n", `unknown key "true"`, 1},
		{"null key", "~: \"a\"\n", `unknown key "~"`, 1},
		{"complex sequence key", "? [a, b]\n: \"v\"\n", `unknown key`, 1},
		{"complex mapping key", "? {a: b}\n: \"v\"\n", `unknown key`, 1},
		{"unknown key nested", "items:\n  - name: \"a\"\n    colour: \"red\"\n", `unknown key "colour" in items[0] (allowed: count, name, on, tags)`, 3},
		{"unknown key case differs", "Schema: \"a\"\n", `unknown key "Schema"`, 1},
		{"unknown key with trailing space quoted", "\"schema \": \"a\"\n", `unknown key "schema "`, 1},
		{"scalar document", "just text\n", `must be a set of "key: value" lines`, 1},
		{"null document", "~\n", `must be a set of "key: value" lines`, 1},
		{"scalar item in list of mappings", "items:\n  - \"a\"\n", `items[0] must be a set of "key: value" lines`, 2},
		{"null item in list", "items:\n  -\n", `items[0] must be a set of`, 2},
		{"flow mapping for list", "items: {}\n", `items must be a list`, 1},
		{"string for list", "items: \"a\"\n", `items must be a list`, 1},
		{"tags is a mapping", "items:\n  - tags: {a: \"b\"}\n", `items[0].tags must be a list`, 2},
		{"nested list in string list", "items:\n  - tags: [[\"a\"]]\n", `must be text in quotes`, 2},
		{"directive then bad", "%YAML 1.1\n---\nversion: x\n", `whole number`, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			es := refuse(t, c.src)
			if !strings.Contains(es.Error(), c.want) {
				t.Errorf("error %q does not contain %q", es, c.want)
			}
			if es[0].File != "x.yaml" || !strings.HasPrefix(es.Error(), "x.yaml") {
				t.Errorf("file not named: %q", es)
			}
			if c.line > 0 && es[0].Line != c.line {
				t.Errorf("line = %d, want %d (%v)", es[0].Line, c.line, es)
			}
		})
	}
}
