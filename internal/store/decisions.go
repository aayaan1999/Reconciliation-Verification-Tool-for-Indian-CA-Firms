package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// Decisions on findings (SPEC-006 part 3). The engine is the database's only writer; these
// are called by recon-engine decide and read by recon-engine result.

// The decisions staff can take on a finding.
const (
	DecisionFix        = "fix"          // the document is to be corrected
	DecisionNotAnIssue = "not_an_issue" // reviewed; nothing to change (a note says why)
	DecisionAskClient  = "ask_client"   // a query for the client
)

// Decisions lists the decisions, in the order they are shown.
var Decisions = []string{DecisionFix, DecisionNotAnIssue, DecisionAskClient}

// A Decision is one decision taken on a finding.
type Decision struct {
	FindingID        string
	RuleID           string
	Decision         string
	Note             string
	DecidedBy        string
	DecidedAt        string // UTC, RFC 3339
	ValueFingerprint string // the finding's values when it was decided
}

// AddDecision records a decision on a finding of the engagement. Earlier decisions stay as
// its history; nothing is overwritten (the table refuses updates and deletes).
func (t *Tx) AddDecision(engagementID int64, d Decision) error {
	_, err := t.tx.Exec(`INSERT INTO finding_decisions (engagement_id, finding_id, rule_id, decision, note, decided_by, decided_at, value_fingerprint)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, engagementID, d.FindingID, d.RuleID, d.Decision, d.Note, d.DecidedBy, d.DecidedAt, d.ValueFingerprint)
	if err != nil {
		return fmt.Errorf("store: decision: %w", err)
	}
	return nil
}

// FindingDecisions returns every decision of the engagement by finding, oldest first: the
// last of each list is the finding's current decision.
func (t *Tx) FindingDecisions(engagementID int64) (map[string][]Decision, error) {
	rows, err := t.tx.Query(`SELECT finding_id, rule_id, decision, note, decided_by, decided_at, value_fingerprint
		FROM finding_decisions WHERE engagement_id = ? ORDER BY id`, engagementID)
	if err != nil {
		return nil, fmt.Errorf("store: decisions: %w", err)
	}
	defer rows.Close()
	out := map[string][]Decision{}
	for rows.Next() {
		var d Decision
		if err := rows.Scan(&d.FindingID, &d.RuleID, &d.Decision, &d.Note, &d.DecidedBy, &d.DecidedAt, &d.ValueFingerprint); err != nil {
			return nil, fmt.Errorf("store: decisions: %w", err)
		}
		out[d.FindingID] = append(out[d.FindingID], d)
	}
	return out, rows.Err()
}

// A Classification is the classification staff gave one Worklist item.
type Classification struct {
	Kind, Ledger   string
	Classification string
	ClassifiedBy   string
	ClassifiedAt   string // UTC, RFC 3339
}

// AddClassification records the classification of a Worklist item; earlier ones stay as its
// history.
func (t *Tx) AddClassification(engagementID int64, c Classification) error {
	_, err := t.tx.Exec(`INSERT INTO worklist_classifications (engagement_id, kind, ledger, classification, classified_by, classified_at)
		VALUES (?, ?, ?, ?, ?, ?)`, engagementID, c.Kind, c.Ledger, c.Classification, c.ClassifiedBy, c.ClassifiedAt)
	if err != nil {
		return fmt.Errorf("store: classification: %w", err)
	}
	return nil
}

// Classifications returns the current classification of each Worklist item of the
// engagement, by kind and ledger ("new_head|Lead generation charges").
func (t *Tx) Classifications(engagementID int64) (map[string]Classification, error) {
	rows, err := t.tx.Query(`SELECT kind, ledger, classification, classified_by, classified_at
		FROM worklist_classifications WHERE engagement_id = ? ORDER BY id`, engagementID)
	if err != nil {
		return nil, fmt.Errorf("store: classifications: %w", err)
	}
	defer rows.Close()
	out := map[string]Classification{}
	for rows.Next() {
		var c Classification
		if err := rows.Scan(&c.Kind, &c.Ledger, &c.Classification, &c.ClassifiedBy, &c.ClassifiedAt); err != nil {
			return nil, fmt.Errorf("store: classifications: %w", err)
		}
		out[c.Kind+"|"+c.Ledger] = c // a later one replaces an earlier one
	}
	return out, rows.Err()
}

// SetRunWorklist keeps the Worklist items a run worked out (canonical JSON).
func (t *Tx) SetRunWorklist(runID int64, items []byte) error {
	if _, err := t.tx.Exec(`UPDATE runs SET worklist_json = ? WHERE id = ?`, string(items), runID); err != nil {
		return fmt.Errorf("store: run worklist: %w", err)
	}
	return nil
}

// LatestWorklist returns the Worklist items of the engagement's most recent finished run
// that worked them out (canonical JSON), and that run's ID; found is false when no run
// had both years' ledgers.
func (t *Tx) LatestWorklist(engagementID int64) (items []byte, runID int64, found bool, err error) {
	var s string
	err = t.tx.QueryRow(`SELECT id, worklist_json FROM runs WHERE engagement_id = ? AND finished_at <> '' AND worklist_json <> ''
		ORDER BY id DESC LIMIT 1`, engagementID).Scan(&runID, &s)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, 0, false, nil
	case err != nil:
		return nil, 0, false, fmt.Errorf("store: latest worklist: %w", err)
	}
	return []byte(s), runID, true, nil
}
