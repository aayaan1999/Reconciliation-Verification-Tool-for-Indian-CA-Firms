package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// GenesisHash is the previous hash of the first entry of the log.
const GenesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

// An Entry is one row of the audit log (technical spec §B1.1). Entries are only ever added;
// the store refuses updates and deletes (migration 0007).
type Entry struct {
	Seq        int64
	At         string // UTC, RFC 3339; written by the caller, never read from the clock here
	Actor      string
	Action     string // for example "rule.disable"
	ObjectType string // for example "rule"
	ObjectID   string // for example "RECON-DEP-002 v1-office.1"
	Details    string // a canonical JSON object, with its keys sorted
	PrevHash   string // the hash of the entry before, GenesisHash for the first
	Hash       string // HashOf(the entry)
}

// HashOf returns the hash that entry e must carry: SHA-256 over its fields, in order, and
// the hash of the entry before it (e.PrevHash). Each field is written as JSON, so no two
// different entries can give the same input.
func HashOf(e Entry) string {
	payload, _ := json.Marshal([]any{e.Seq, e.At, e.Actor, e.Action, e.ObjectType, e.ObjectID, e.Details, e.PrevHash})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// A BrokenError says where the chain stops matching: the first entry whose previous hash or
// own hash is wrong, or whose sequence number does not follow the one before.
type BrokenError struct {
	Seq    int64
	Reason string
}

func (e *BrokenError) Error() string {
	return fmt.Sprintf("audit log broken at entry %d: %s", e.Seq, e.Reason)
}

// Verify checks the whole chain, entries in sequence order. It returns nil when every entry
// links to the one before it and carries the hash of its own content.
func Verify(entries []Entry) error {
	prev := GenesisHash
	var last int64
	for i, e := range entries {
		switch {
		case i > 0 && e.Seq != last+1:
			return &BrokenError{Seq: e.Seq, Reason: fmt.Sprintf("the entry after %d is %d", last, e.Seq)}
		case e.PrevHash != prev:
			return &BrokenError{Seq: e.Seq, Reason: "the previous hash does not match the entry before it"}
		case e.Hash != HashOf(e):
			return &BrokenError{Seq: e.Seq, Reason: "the hash does not match the entry's content"}
		}
		prev, last = e.Hash, e.Seq
	}
	return nil
}
