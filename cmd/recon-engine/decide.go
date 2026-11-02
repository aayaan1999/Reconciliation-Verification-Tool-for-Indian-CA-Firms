package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/period"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
)

// Decisions on findings (SPEC-006 part 3): Fix, Not an issue, Ask client. A decision is
// kept against the finding's stable ID; it stays while the finding's values are unchanged.

// findingRef is what a stored report says about one finding.
type findingRef struct {
	RuleID, Fingerprint string
}

// findingsIn returns the findings of the given stored reports by stable ID.
func findingsIn(reports [][]byte) (map[string]findingRef, error) {
	out := map[string]findingRef{}
	for _, b := range reports {
		var rep struct {
			Findings []struct {
				Findings []struct {
					FindingID        string `json:"finding_id"`
					RuleID           string `json:"rule_id"`
					ValueFingerprint string `json:"value_fingerprint"`
				} `json:"findings"`
			} `json:"findings"`
		}
		if err := json.Unmarshal(b, &rep); err != nil {
			return nil, fmt.Errorf("stored report: %w", err)
		}
		for _, g := range rep.Findings {
			for _, f := range g.Findings {
				if f.FindingID != "" { // reports stored before SPEC-006 part 3 have none
					out[f.FindingID] = findingRef{RuleID: f.RuleID, Fingerprint: f.ValueFingerprint}
				}
			}
		}
	}
	return out, nil
}

func decisionObject(d store.Decision, current findingRef) map[string]any {
	return map[string]any{"decision": d.Decision, "note": d.Note, "decided_by": d.DecidedBy, "decided_at": d.DecidedAt,
		"on_other_values": d.ValueFingerprint != current.Fingerprint}
}

// decisionsOf returns, for each finding of the reports that has decisions, its state (the
// current decision, or "open" when its values changed since), the current decision and
// the history, newest last.
func decisionsOf(tx *store.Tx, eng int64, reports [][]byte) (map[string]any, error) {
	found, err := findingsIn(reports)
	if err != nil {
		return nil, err
	}
	all, err := tx.FindingDecisions(eng)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	for id, ref := range found {
		list := all[id]
		if len(list) == 0 {
			continue
		}
		history := make([]any, len(list))
		for i, d := range list {
			history[i] = decisionObject(d, ref)
		}
		last := list[len(list)-1]
		state := last.Decision
		if last.ValueFingerprint != ref.Fingerprint {
			state = "open" // the values moved since the decision: look at it again (FD-04)
		}
		out[id] = map[string]any{"state": state, "current": decisionObject(last, ref), "history": history}
	}
	return out, nil
}

// decide records a decision on a finding of a client's latest results. The decision comes
// on standard input as JSON, so the note can hold any text:
//
//	recon-engine decide --db <file> --client <id> --tax-year 2025-26 --json <file> < decision.json
//	{"finding_id": "...", "decision": "fix" | "not_an_issue" | "ask_client", "note": "...", "by": "..."}
func decide(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("decide", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbFile := fs.String("db", "", "database file")
	clientID := fs.Int64("client", 0, "client ID")
	taxYear := fs.String("tax-year", "", "tax year, such as 2025-26")
	jsonOut := fs.String("json", "", "write the finding's decisions as JSON to this file")
	if err := parseFlags(fs, args, "db", "client", "tax-year", "json"); err != nil {
		return err
	}
	ty, err := period.ParseTaxYear(*taxYear)
	if err != nil {
		return inputErrf("--tax-year: %v", err)
	}
	var in struct {
		FindingID string `json:"finding_id"`
		Decision  string `json:"decision"`
		Note      string `json:"note"`
		By        string `json:"by"`
	}
	dec := json.NewDecoder(io.LimitReader(stdin, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return inputErrf("BAD_REQUEST: the decision is not valid JSON: %v", err)
	}
	in.Note, in.By = strings.TrimSpace(in.Note), strings.TrimSpace(in.By)
	switch {
	case !slices.Contains(store.Decisions, in.Decision):
		return inputErrf("BAD_DECISION: decision %q must be one of %s", in.Decision, strings.Join(store.Decisions, ", "))
	case in.By == "" || utf8.RuneCountInString(in.By) > 100:
		return inputErrf("BAD_NAME: say who decides (a name of up to 100 characters)")
	case in.Decision == store.DecisionNotAnIssue && in.Note == "":
		return inputErrf("NOTE_REQUIRED: a finding decided 'Not an issue' needs a note saying why")
	case utf8.RuneCountInString(in.Note) > 2000:
		return inputErrf("BAD_NOTE: a note has at most 2,000 characters")
	}
	if _, err := os.Stat(*dbFile); err != nil {
		return inputErrf("--db: %v", err)
	}
	db, err := store.Open(*dbFile)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	c, ok, err := tx.ClientByID(*clientID)
	if err != nil {
		return err
	}
	if !ok {
		return inputErrf("CLIENT_NOT_FOUND: there is no client %d", *clientID)
	}
	eng, ok, err := tx.FindClientEngagement(c.ID, ty.String())
	if err != nil {
		return err
	}
	if !ok {
		return inputErrf("FINDING_NOT_FOUND: client %d has no results for %s", c.ID, ty)
	}
	runs, err := tx.LatestRuns(eng)
	if err != nil {
		return err
	}
	var reports [][]byte
	for _, r := range runs {
		reports = append(reports, r.Report)
	}
	found, err := findingsIn(reports)
	if err != nil {
		return err
	}
	ref, ok := found[in.FindingID]
	if !ok {
		return inputErrf("FINDING_NOT_FOUND: %q is not a finding of the client's latest results for %s", in.FindingID, ty)
	}
	if err := tx.AddDecision(eng, store.Decision{FindingID: in.FindingID, RuleID: ref.RuleID, Decision: in.Decision, Note: in.Note,
		DecidedBy: in.By, DecidedAt: time.Now().UTC().Format(time.RFC3339), ValueFingerprint: ref.Fingerprint}); err != nil {
		return err
	}
	decisions, err := decisionsOf(tx, eng, reports)
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	if err := writeJSONFile(*jsonOut, map[string]any{"finding_id": in.FindingID, "decision": decisions[in.FindingID]}); err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "finding %s: %s, by %s\n", in.FindingID, in.Decision, in.By)
	return err
}
