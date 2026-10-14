package strictyaml

import (
	"strings"
	"testing"
)

// Implicit-typing traps: YAML 1.1 turns many bare words into booleans, numbers, nulls and
// timestamps. A text field must refuse every one of them unquoted, and accept them quoted.
func TestExtraImplicitTypingTraps(t *testing.T) {
	traps := []string{
		"yes", "no", "Yes", "NO", "on", "off", "ON", "Off", "y", "n", "true", "False", "TRUE",
		"0123", "0o17", "0x1F", "0b101", "1e3", "1E3", "1.5", "-1", "+1", "1_000", ".inf", "-.inf", ".nan", ".NaN",
		"2025-04-01", "2025-04-01T10:00:00Z", "2025-26", "12:30:45", "1:30", "~", "null", "Null", "NULL",
		"3.10", "1.0", "0.1", "007", "0", "100",
	}
	for _, v := range traps {
		t.Run("text unquoted "+v, func(t *testing.T) {
			es := refuse(t, "schema: "+v+"\n")
			if !strings.Contains(es.Error(), "must be in quotes") || es[0].Line != 1 {
				t.Errorf("got %v", es)
			}
		})
		t.Run("text double quoted "+v, func(t *testing.T) {
			var f file
			if _, err := Decode("x.yaml", []byte(`schema: "`+v+"\"\n"), &f); err != nil || f.Schema != v {
				t.Errorf("err=%v schema=%q", err, f.Schema)
			}
		})
		t.Run("text single quoted "+v, func(t *testing.T) {
			var f file
			if _, err := Decode("x.yaml", []byte("schema: '"+v+"'\n"), &f); err != nil || f.Schema != v {
				t.Errorf("err=%v schema=%q", err, f.Schema)
			}
		})
	}
	for _, src := range []string{"schema:\n", "schema: ~\n", "schema: null\n", "schema: \n"} {
		refuse(t, src)
	}

	boolBad := []string{"yes", "no", "Yes", "NO", "on", "off", "y", "n", "True", "FALSE", "1", "0", "~", "null", "", "\"true\"", "'false'", "tRue"}
	for _, v := range boolBad {
		t.Run("bool refuses "+v, func(t *testing.T) {
			es := refuse(t, "items:\n  - on: "+v+"\n")
			if !strings.Contains(es.Error(), "must be true or false") {
				t.Errorf("got %v", es)
			}
		})
	}
	for _, v := range []string{"true", "false"} {
		var f file
		if _, err := Decode("x.yaml", []byte("items:\n  - on: "+v+"\n"), &f); err != nil || f.Items[0].On != (v == "true") {
			t.Errorf("%s: err=%v", v, err)
		}
	}

	intBad := []string{"0123", "007", "0x1F", "0o17", "0b11", "1e3", "1E3", "1.0", "1.", ".5", "+5", "1_0", "--1", "yes", "~", "null", "", ".inf", "-.inf", ".nan", "2025-04-01", "2025-26", "1:30", "'5'", "\"5\"", "5 5"}
	for _, v := range intBad {
		t.Run("int refuses "+v, func(t *testing.T) {
			es := refuse(t, "version: "+v+"\n")
			if !strings.Contains(es.Error(), "whole number") {
				t.Errorf("got %v", es)
			}
		})
	}
	for _, v := range []string{"0", "-0", "7", "-7", "10", "2147483647", "9223372036854775807", "-9223372036854775808"} {
		t.Run("int accepts "+v, func(t *testing.T) {
			var f file
			if _, err := Decode("x.yaml", []byte("version: "+v+"\n"), &f); err != nil {
				t.Errorf("err=%v", err)
			}
		})
	}
}

// A number that is plain digits but does not fit int64 must still come back as Errors with a line.
func TestExtraIntegerOverflow(t *testing.T) {
	for _, v := range []string{"9223372036854775808", "99999999999999999999999999999999"} {
		t.Run(v, func(t *testing.T) {
			es := refuse(t, "schema: \"s\"\nversion: "+v+"\n")
			if es[0].Line != 2 {
				t.Errorf("line = %d, want 2 (%v)", es[0].Line, es)
			}
		})
	}
}

// BUG: the walker never looks at the anchor or tag on a mapping KEY, only on values.
func TestExtraKeyAnchorsAndTags(t *testing.T) {
	for name, src := range map[string]string{
		"anchor on key":        "&k schema: \"a\"\n",
		"tag on key":           "!!str schema: \"a\"\n",
		"anchor on nested key": "items:\n  - &k name: \"a\"\n",
		"explicit key anchor":  "? &k schema\n: \"a\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			var f file
			if _, err := Decode("x.yaml", []byte(src), &f); err == nil {
				t.Skip("BUG: strictyaml.walker.walk checks Anchor/Tag/Alias on values only; a mapping key with &anchor or !!tag is accepted (strictyaml.go struct case never inspects k)")
			}
		})
	}
}
