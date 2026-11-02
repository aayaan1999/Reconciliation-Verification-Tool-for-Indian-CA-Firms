package rulestore

import (
	"bytes"
	"fmt"

	"go.yaml.in/yaml/v3"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/strictyaml"
)

// EncodeYAML writes v as rule-store YAML: every text value quoted (multi-line text as a |
// block), two-space indent, keys in the order of the Go structure. The result is read back
// into back through the strict reader and must say the same as v, so what is written is what
// the store loads. name is used in messages only.
func EncodeYAML(name string, v any, back any) ([]byte, error) {
	var n yaml.Node
	if err := n.Encode(v); err != nil {
		return nil, err
	}
	quoteStrings(&n)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&n); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	if _, err := strictyaml.Decode(name, buf.Bytes(), back); err != nil {
		return nil, fmt.Errorf("written YAML does not read back strictly: %w", err)
	}
	if err := sameYAML(v, back); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return buf.Bytes(), nil
}

// quoteStrings gives every string scalar a quoted (or | block) style. Mapping keys stay plain.
func quoteStrings(n *yaml.Node) {
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, c := range n.Content {
			quoteStrings(c)
		}
	case yaml.MappingNode:
		for i := 1; i < len(n.Content); i += 2 {
			quoteStrings(n.Content[i])
		}
	case yaml.ScalarNode:
		if n.Tag == "!!str" {
			n.Style = yaml.DoubleQuotedStyle
			if bytes.ContainsRune([]byte(n.Value), '\n') && blockSafe(n.Value) {
				n.Style = yaml.LiteralStyle
			}
		}
	}
}

// blockSafe reports whether text survives a | block unchanged: no carriage returns,
// tabs or trailing spaces, and no leading blank line.
func blockSafe(s string) bool {
	if s == "" || s[0] == '\n' || s[0] == ' ' {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\r', '\t':
			return false
		case '\n':
			if i > 0 && s[i-1] == ' ' {
				return false
			}
		}
	}
	return s[len(s)-1] != ' ' && s[len(s)-1] != '\n'
}

// sameYAML compares what was written with what was read back.
func sameYAML(a, b any) error {
	ea, err := yaml.Marshal(a)
	if err != nil {
		return err
	}
	eb, err := yaml.Marshal(b)
	if err != nil {
		return err
	}
	if !bytes.Equal(ea, eb) {
		return fmt.Errorf("the file reads back differently from what was written")
	}
	return nil
}
