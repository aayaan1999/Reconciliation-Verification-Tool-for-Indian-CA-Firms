package store

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
)

// A RunInfo is the snapshot of a run (RU-53; technical spec §A10): everything versioned
// that the run used.
type RunInfo struct {
	EngagementID   int64
	TaxYear        string
	EngineVersion  string
	VocabVersion   int
	VocabHash      string
	IncludeTesting bool
	Comparison     string       // the type of check run (SPEC-004 part 9); "" for every rule
	Documents      []DocVersion // the current view
	Rules          []StoredRule // the rules evaluated, with the status each had
}

// CreateRun stores the run and its snapshot and returns the run's ID.
func (t *Tx) CreateRun(ri RunInfo) (int64, error) {
	incl := 0
	if ri.IncludeTesting {
		incl = 1
	}
	res, err := t.tx.Exec(`INSERT INTO runs (engagement_id, tax_year, engine_version, vocab_version, vocab_hash, include_testing, comparison_id) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		ri.EngagementID, ri.TaxYear, ri.EngineVersion, ri.VocabVersion, ri.VocabHash, incl, ri.Comparison)
	if err != nil {
		return 0, fmt.Errorf("store: run: %w", err)
	}
	runID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, d := range ri.Documents {
		if _, err := t.tx.Exec(`INSERT INTO run_documents (run_id, document_version_id, document_type, file_sha256, profile_id, profile_version, profile_version_id)
			VALUES (?, ?, ?, ?, ?, ?, (SELECT profile_version_id FROM document_versions WHERE id = ?))`,
			runID, d.ID, d.Slot.DocumentType, d.SHA256, d.ProfileID, d.ProfileVersion, d.ID); err != nil {
			return 0, fmt.Errorf("store: run documents: %w", err)
		}
	}
	for _, r := range ri.Rules {
		if _, err := t.tx.Exec(`INSERT INTO run_rules (run_id, rule_version_id, rule_id, version, status) VALUES (?, ?, ?, ?, ?)`,
			runID, r.VersionID, r.RuleID, r.Version, r.Status); err != nil {
			return 0, fmt.Errorf("store: run rules: %w", err)
		}
	}
	return runID, nil
}

// PutEvaluations stores evs for the run, which must already be sorted by
// (rule_id, subject_key): their position becomes the stored seq.
func (t *Tx) PutEvaluations(runID int64, evs []domain.Evaluation) error {
	if !slices.IsSortedFunc(evs, func(a, b domain.Evaluation) int {
		return cmp.Or(cmp.Compare(a.RuleID, b.RuleID), cmp.Compare(a.Subject.Key(), b.Subject.Key()))
	}) {
		return fmt.Errorf("store: evaluations are not sorted by rule and subject")
	}
	for i, e := range evs {
		values, err := domain.CanonicalValues(e.Values)
		if err != nil {
			return fmt.Errorf("store: evaluation of %s: %w", e.RuleID, err)
		}
		missing, err := json.Marshal(append([]string{}, slices.Sorted(slices.Values(e.MissingFacts))...))
		if err != nil {
			return err
		}
		if _, err := t.tx.Exec(`INSERT INTO evaluations (run_id, seq, rule_id, rule_version, subject_key, status, values_json, difference_paise, missing_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			runID, i+1, e.RuleID, e.RuleVersion, e.Subject.Key(), e.Status.String(), string(values), int64(e.Difference), string(missing)); err != nil {
			return fmt.Errorf("store: evaluation of %s: %w", e.RuleID, err)
		}
	}
	return nil
}

// FinishRun records the end of the run.
func (t *Tx) FinishRun(runID int64) error {
	if _, err := t.tx.Exec(`UPDATE runs SET finished_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = ?`, runID); err != nil {
		return fmt.Errorf("store: run: %w", err)
	}
	return nil
}

func sortFacts(fs []domain.Fact) {
	slices.SortStableFunc(fs, func(a, b domain.Fact) int {
		return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.Period, b.Period), cmp.Compare(a.Scope, b.Scope),
			cmp.Compare(a.Locator.FileName, b.Locator.FileName), cmp.Compare(a.Locator.Path, b.Locator.Path))
	})
}
