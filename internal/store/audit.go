package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/audit"
)

// Audit writes one entry to the audit log inside this transaction (technical spec §B9, RU-42).
// details is written as a canonical JSON object (its keys sorted), so the same entry always
// has the same hash. at is the caller's UTC time in RFC 3339.
func (t *Tx) Audit(at, actor, action, objectType, objectID string, details map[string]string) error {
	body, err := json.Marshal(details) // map keys are written sorted
	if err != nil {
		return fmt.Errorf("store: audit details: %w", err)
	}
	var lastSeq int64
	prev := audit.GenesisHash
	err = t.tx.QueryRow(`SELECT seq, hash FROM audit_log ORDER BY seq DESC LIMIT 1`).Scan(&lastSeq, &prev)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		prev = audit.GenesisHash
	case err != nil:
		return fmt.Errorf("store: audit: %w", err)
	}
	e := audit.Entry{Seq: lastSeq + 1, At: at, Actor: actor, Action: action, ObjectType: objectType,
		ObjectID: objectID, Details: string(body), PrevHash: prev}
	e.Hash = audit.HashOf(e)
	if _, err := t.tx.Exec(`INSERT INTO audit_log (seq, at, actor, action, object_type, object_id, details_json, prev_hash, hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, e.Seq, e.At, e.Actor, e.Action, e.ObjectType, e.ObjectID, e.Details, e.PrevHash, e.Hash); err != nil {
		return fmt.Errorf("store: audit: %w", err)
	}
	return nil
}

// AuditEntries returns the whole audit log in sequence order.
func (t *Tx) AuditEntries() ([]audit.Entry, error) {
	rows, err := t.tx.Query(`SELECT seq, at, actor, action, object_type, object_id, details_json, prev_hash, hash
		FROM audit_log ORDER BY seq`)
	if err != nil {
		return nil, fmt.Errorf("store: audit log: %w", err)
	}
	defer rows.Close()
	var out []audit.Entry
	for rows.Next() {
		var e audit.Entry
		if err := rows.Scan(&e.Seq, &e.At, &e.Actor, &e.Action, &e.ObjectType, &e.ObjectID, &e.Details, &e.PrevHash, &e.Hash); err != nil {
			return nil, fmt.Errorf("store: audit log: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
