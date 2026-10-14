package strictyaml

import (
	"fmt"
	"strings"
	"testing"
)

func TestExtraMergeKeyWithDefinedAnchor(t *testing.T) {
	es := refuse(t, "items:\n  - &x {name: \"a\"}\n  - <<: *x\n")
	if !strings.Contains(es.Error(), "merge keys (<<)") {
		t.Errorf("merge key not reported: %v", es)
	}
	found := false
	for _, e := range es {
		if e.Line == 3 && strings.Contains(e.Msg, "merge keys") {
			found = true
		}
	}
	if !found {
		t.Errorf("no merge-key error at line 3: %v", es)
	}
}

func TestExtraWhitespaceAndEncoding(t *testing.T) {
	good := func(name, src string) {
		t.Run(name, func(t *testing.T) {
			var f file
			if _, err := Decode("x.yaml", []byte(src), &f); err != nil {
				t.Fatalf("refused: %v", err)
			}
			if f.Schema != "s" || f.Version != 2 {
				t.Errorf("decoded %+v", f)
			}
		})
	}
	good("CRLF", "schema: \"s\"\r\nversion: 2\r\n")
	good("CRLF no trailing newline", "schema: \"s\"\r\nversion: 2")
	good("LF no trailing newline", "schema: \"s\"\nversion: 2")
	good("lone CR", "schema: \"s\"\rversion: 2\r")
	good("UTF-8 BOM", "\xef\xbb\xbfschema: \"s\"\nversion: 2\n")
	good("document end marker", "schema: \"s\"\nversion: 2\n...\n")
	good("trailing comment", "schema: \"s\" # why\nversion: 2 # n\n")
	good("blank lines and comments", "\n# c\n\nschema: \"s\"\n\n\nversion: 2\n")
	good("trailing spaces", "schema: \"s\"   \nversion: 2  \n")
	good("flow style mapping root", "{schema: \"s\", version: 2}\n")

	t.Run("BOM positions", func(t *testing.T) {
		var f file
		pos, err := Decode("x.yaml", []byte("\xef\xbb\xbfschema: \"s\"\nversion: 2\n"), &f)
		if err != nil || pos.Line("version") != 2 {
			t.Errorf("err=%v version line=%d", err, pos.Line("version"))
		}
	})
	t.Run("CRLF lines are counted once", func(t *testing.T) {
		es := refuse(t, "schema: \"s\"\r\nversion: 2\r\nversion: 3\r\nfoo: 1\r\n")
		if es[0].Line != 3 || es[1].Line != 4 {
			t.Errorf("lines %d,%d want 3,4: %v", es[0].Line, es[1].Line, es)
		}
	})
	t.Run("BOM then bad line keeps line numbers", func(t *testing.T) {
		es := refuse(t, "\xef\xbb\xbfschema: \"s\"\nbogus: 1\n")
		if es[0].Line != 2 {
			t.Errorf("line = %d want 2", es[0].Line)
		}
	})
	t.Run("tab indentation", func(t *testing.T) {
		refuse(t, "items:\n\t- name: \"a\"\n")
	})
	t.Run("tab after colon is allowed as separator", func(t *testing.T) {
		var f file
		if _, err := Decode("x.yaml", []byte("version:\t2\n"), &f); err != nil || f.Version != 2 {
			t.Errorf("err=%v v=%d", err, f.Version)
		}
	})
	t.Run("tab inside quoted text", func(t *testing.T) {
		var f file
		if _, err := Decode("x.yaml", []byte("schema: \"a\tb\"\n"), &f); err != nil || f.Schema != "a\tb" {
			t.Errorf("err=%v s=%q", err, f.Schema)
		}
	})
	for name, src := range map[string]string{
		"invalid UTF-8 byte":      "schema: \"\xff\"\n",
		"truncated UTF-8":         "schema: \"\xe2\x82\"\n",
		"overlong encoding":       "schema: \"\xc0\xaf\"\n",
		"UTF-8 encoded surrogate": "schema: \"\xed\xa0\x80\"\n",
		"NUL byte":                "schema: \"a\x00b\"\n",
		"control char":            "schema: \"a\x01b\"\n",
		"invalid in comment":      "# \xff\nschema: \"s\"\n",
		"invalid in key":          "sch\xffema: \"s\"\n",
	} {
		t.Run("refuses "+name, func(t *testing.T) {
			es := refuse(t, src)
			if !strings.Contains(es.Error(), "not valid YAML") {
				t.Errorf("got %v", es)
			}
		})
	}
	t.Run("unicode text is kept", func(t *testing.T) {
		var f file
		if _, err := Decode("x.yaml", []byte("schema: \"₹ ₹१२\"\n"), &f); err != nil || f.Schema != "₹ ₹१२" {
			t.Errorf("err=%v s=%q", err, f.Schema)
		}
	})
}

func TestExtraLimits(t *testing.T) {
	t.Run("billion laughs is refused quickly", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("a0: &a0 [\"x\", \"x\", \"x\", \"x\", \"x\", \"x\", \"x\", \"x\", \"x\"]\n")
		for i := 1; i < 12; i++ {
			fmt.Fprintf(&b, "a%d: &a%d [", i, i)
			for j := 0; j < 9; j++ {
				if j > 0 {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "*a%d", i-1)
			}
			b.WriteString("]\n")
		}
		es := refuse(t, b.String())
		if len(es) == 0 {
			t.Error("no errors")
		}
	})
	t.Run("alias bomb under a known key", func(t *testing.T) {
		src := "items:\n  - &a {name: \"x\"}\n" + strings.Repeat("  - *a\n", 20000)
		es := refuse(t, src)
		if es[0].Line != 2 {
			t.Errorf("first error line = %d, want 2", es[0].Line)
		}
	})
	t.Run("deep flow nesting", func(t *testing.T) {
		es := refuse(t, "items: "+strings.Repeat("[", 100000)+strings.Repeat("]", 100000)+"\n")
		if !strings.Contains(es.Error(), "not valid YAML") {
			t.Errorf("got %.200s", es)
		}
	})
	t.Run("deep flow nesting unclosed", func(t *testing.T) {
		refuse(t, "items: "+strings.Repeat("[", 100000)+"\n")
	})
	t.Run("deep block nesting", func(t *testing.T) {
		var b strings.Builder
		for i := 0; i < 3000; i++ {
			b.WriteString(strings.Repeat(" ", i) + "- \n")
		}
		refuse(t, "items:\n"+b.String())
	})
	t.Run("deep nesting inside an unknown key", func(t *testing.T) {
		es := refuse(t, "zzz: "+strings.Repeat("{a: ", 2000)+"1"+strings.Repeat("}", 2000)+"\n")
		if !strings.Contains(es.Error(), `unknown key "zzz"`) {
			t.Errorf("got %.200s", es)
		}
	})
	t.Run("large valid file", func(t *testing.T) {
		var b strings.Builder
		b.WriteString("schema: \"s\"\nversion: 1\nitems:\n")
		const n = 50000
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, "  - name: \"n%d\"\n    count: %d\n    on: true\n    tags: [\"a\", \"b\"]\n", i, i)
		}
		var f file
		pos, err := Decode("big.yaml", []byte(b.String()), &f)
		if err != nil || len(f.Items) != n {
			t.Fatalf("err=%v items=%d", err, len(f.Items))
		}
		if got, want := pos.Line(fmt.Sprintf("items[%d].count", n-1)), 4+4*(n-1)+1; got != want {
			t.Errorf("line of last count = %d, want %d", got, want)
		}
	})
	t.Run("very long scalar", func(t *testing.T) {
		var f file
		long := strings.Repeat("a", 4<<20)
		if _, err := Decode("x.yaml", []byte("schema: \""+long+"\"\n"), &f); err != nil || len(f.Schema) != len(long) {
			t.Errorf("err=%v len=%d", err, len(f.Schema))
		}
	})
	t.Run("many errors are all reported", func(t *testing.T) {
		var b strings.Builder
		for i := 0; i < 1000; i++ {
			fmt.Fprintf(&b, "k%d: 1\n", i)
		}
		if es := refuse(t, b.String()); len(es) != 1000 {
			t.Errorf("got %d errors, want 1000", len(es))
		}
	})
}
