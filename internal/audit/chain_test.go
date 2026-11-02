package audit

import (
	"errors"
	"testing"
)

// chainOf builds a valid chain of n entries.
func chainOf(n int) []Entry {
	var out []Entry
	prev := GenesisHash
	for i := 1; i <= n; i++ {
		e := Entry{Seq: int64(i), At: "2026-10-29T09:00:00Z", Actor: "A. Staff", Action: "rule.disable",
			ObjectType: "rule", ObjectID: "RECON-X-001 v1", Details: `{"reason":"r"}`, PrevHash: prev}
		e.Hash = HashOf(e)
		out = append(out, e)
		prev = e.Hash
	}
	return out
}

func TestVerifyAcceptsAValidChain(t *testing.T) {
	if err := Verify(nil); err != nil {
		t.Errorf("an empty log: %v", err)
	}
	if err := Verify(chainOf(4)); err != nil {
		t.Errorf("a valid chain: %v", err)
	}
}

// Any change to an entry, or to the link before it, is found at that entry.
func TestVerifyFindsAChangedEntry(t *testing.T) {
	cases := map[string]func(es []Entry){
		"the details":  func(es []Entry) { es[2].Details = `{"reason":"changed"}` },
		"the actor":    func(es []Entry) { es[1].Actor = "someone else" },
		"the link":     func(es []Entry) { es[3].PrevHash = GenesisHash },
		"the own hash": func(es []Entry) { es[0].Hash = GenesisHash },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			es := chainOf(4)
			change(es)
			var b *BrokenError
			if err := Verify(es); !errors.As(err, &b) {
				t.Fatalf("verify = %v, want a BrokenError", err)
			}
		})
	}
}

// A missing entry shows as a gap in the sequence.
func TestVerifyFindsAGap(t *testing.T) {
	es := chainOf(3)
	es = append(es[:1], es[2:]...)
	var b *BrokenError
	if err := Verify(es); !errors.As(err, &b) || b.Seq != 3 {
		t.Errorf("verify without entry 2 = %v, want a break at entry 3", err)
	}
}

// The hash covers every field, so two entries that differ in one field never share a hash.
func TestHashCoversEveryField(t *testing.T) {
	base := chainOf(1)[0]
	variants := []Entry{base, base, base, base, base, base, base, base}
	variants[0].At += "x"
	variants[1].Actor += "x"
	variants[2].Action += "x"
	variants[3].ObjectType += "x"
	variants[4].ObjectID += "x"
	variants[5].Details += " "
	variants[6].PrevHash += "x"
	variants[7].Seq++
	seen := map[string]bool{HashOf(base): true}
	for i, v := range variants {
		h := HashOf(v)
		if seen[h] {
			t.Errorf("variant %d has the hash of another entry", i)
		}
		seen[h] = true
	}
}
