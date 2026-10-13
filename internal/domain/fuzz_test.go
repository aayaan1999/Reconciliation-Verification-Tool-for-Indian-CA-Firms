package domain

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
)

// Subject.Key and StableID must be injective: different subjects (or rule IDs or
// engagements) never share a key, and an equal key means an equal subject.
func FuzzSubjectKeyInjective(f *testing.F) {
	f.Add("FY2025-26", "", "", "FY2025-26", "", "", "R", "R", int64(1), int64(1))
	f.Add("a|b", "c", "", "a", "b|c", "", "R|1", "R", int64(2), int64(12))
	f.Add(`p\`, "s", "k", "p", `\s`, "k", "", "", int64(0), int64(0))
	f.Fuzz(func(t *testing.T, p1, s1, m1, p2, s2, m2, r1, r2 string, e1, e2 int64) {
		a := Subject{Period: p1, Scope: s1, MatchKey: m1}
		b := Subject{Period: p2, Scope: s2, MatchKey: m2}
		if (a.Key() == b.Key()) != (a == b) {
			t.Fatalf("Key injectivity broken: %+v %+v key %q / %q", a, b, a.Key(), b.Key())
		}
		same := a == b && r1 == r2 && e1 == e2
		if (StableID(r1, e1, a) == StableID(r2, e2, b)) != same {
			t.Fatalf("StableID collision or instability: (%q,%d,%+v) (%q,%d,%+v)", r1, e1, a, r2, e2, b)
		}
		if id := StableID(r1, e1, a); len(id) != 16 || strings.Trim(id, "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567") != "" {
			t.Fatalf("bad id %q", id)
		}
	})
}

// Canonical JSON: succeeds exactly when all text is valid UTF-8, is valid JSON, is
// repeatable, and survives a decode and re-encode unchanged.
func FuzzCanonicalJSON(f *testing.F) {
	f.Add("RECON-DEP-002", "label", "text", "p7:l1", int64(100), int64(-5), uint8(1), "a.b")
	f.Add("R\xff", "", "", "", int64(0), int64(0), uint8(3), "")
	f.Add("<&>", " ", "\x00", "$.x", int64(1<<62), int64(-1<<63), uint8(9), "b|a")
	f.Fuzz(func(t *testing.T, rule, label, text, path string, n, diff int64, kind uint8, missing string) {
		k := ValueKind(kind%8 + 0)
		e := Evaluation{
			RuleID: rule, Status: StatusFail, Difference: diffPaise(diff),
			MissingFacts: []string{missing, rule},
			Values: []LabeledValue{{Input: "a", Label: label,
				Value:   Value{Kind: k, Money: diffPaise(n), Int: n, Text: text, Date: text, Bool: n%2 == 0, RateBP: n},
				Locator: Locator{FileName: label, Kind: LocatorPDFPos, Path: path}}},
			Subject: Subject{Period: text, Scope: label, MatchKey: path},
		}
		b, err := e.CanonicalJSON()
		allValid := utf8.ValidString(rule) && utf8.ValidString(label) && utf8.ValidString(path) &&
			utf8.ValidString(missing) && utf8.ValidString(text)
		// Text and date are only encoded for their own kinds, so a bad string in an
		// unused field is not an error; every other string is always checked.
		textUsed := k == KindText || k == KindDate
		if !k.Valid() {
			if err == nil {
				t.Fatalf("invalid kind %d accepted", k)
			}
			return
		}
		textOK := utf8.ValidString(text) || !textUsed
		subjOK := utf8.ValidString(text)
		wantOK := utf8.ValidString(rule) && utf8.ValidString(label) && utf8.ValidString(path) &&
			utf8.ValidString(missing) && textOK && subjOK
		_ = allValid
		if (err == nil) != wantOK {
			t.Fatalf("err=%v wantOK=%v for %q %q %q %q %q kind=%d", err, wantOK, rule, label, text, path, missing, k)
		}
		if err != nil {
			return
		}
		if !json.Valid(b) || !utf8.Valid(b) || bytes.HasSuffix(b, []byte("\n")) {
			t.Fatalf("malformed output %q", b)
		}
		b2, _ := e.CanonicalJSON()
		if !bytes.Equal(b, b2) {
			t.Fatal("not repeatable")
		}
		var generic any
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.UseNumber()
		if err := dec.Decode(&generic); err != nil {
			t.Fatal(err)
		}
		re, err := marshalCanonical(generic)
		if err != nil || !bytes.Equal(re, b) {
			t.Fatalf("decode and re-encode changed bytes:\n%s\n%s (%v)", b, re, err)
		}
	})
}

// Fingerprint ignores labels, input names and locators, and is sensitive to the value.
func FuzzValueFingerprint(f *testing.F) {
	f.Add(int64(100), "a", "x", "p1", int64(101), uint8(1))
	f.Add(int64(0), "", "", "", int64(0), uint8(3))
	f.Fuzz(func(t *testing.T, n int64, input, label, path string, other int64, kind uint8) {
		k := ValueKind(kind%6 + 1)
		mk := func(x int64, in, lb, p string) []LabeledValue {
			return []LabeledValue{{Input: in, Label: lb, Locator: Locator{Path: p, FileName: lb},
				Value: Value{Kind: k, Money: diffPaise(x), Int: x, Text: itoa(x), Date: itoa(x), Bool: x%2 != 0, RateBP: x}}}
		}
		base, err := ValueFingerprint(mk(n, "a", "L", "p"))
		if err != nil {
			t.Fatal(err)
		}
		moved, err := ValueFingerprint(mk(n, input, label, path))
		if err != nil {
			t.Fatal(err)
		}
		if base != moved {
			t.Fatal("labels, inputs or locators changed the fingerprint")
		}
		changed, err := ValueFingerprint(mk(other, "a", "L", "p"))
		if err != nil {
			t.Fatal(err)
		}
		sameFigure := n == other || (k == KindBool && (n%2 != 0) == (other%2 != 0))
		if (changed == base) != sameFigure {
			t.Fatalf("kind %v: n=%d other=%d equalFingerprint=%v", k, n, other, changed == base)
		}
	})
}

// Locator.Display never panics, never loses the file name, and for non-PDF kinds is the
// plain concatenation.
func FuzzLocatorDisplay(f *testing.F) {
	f.Add("a.pdf", LocatorPDFPos, "p7:l23:c41-52")
	f.Add("", LocatorPDFPos, "p")
	f.Add("x", "other", ":::")
	f.Fuzz(func(t *testing.T, file, kind, path string) {
		l := Locator{FileName: file, Kind: kind, Path: path}
		got := l.Display()
		if !strings.HasPrefix(got, file) {
			t.Fatalf("file name lost: %q for %q", got, file)
		}
		if kind != LocatorPDFPos {
			want := path
			if file != "" && path != "" {
				want = file + " " + path
			} else if file != "" {
				want = file
			}
			if got != want {
				t.Fatalf("got %q want %q", got, want)
			}
		}
	})
}

func diffPaise(n int64) money.Paise { return money.Paise(n) }

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
