package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// Rule statuses (technical spec §B1.1, rule_state.status).
const (
	StatusDraft    = "draft"
	StatusTesting  = "testing"
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

// Rule sources (rule_versions.source).
const (
	SourceRulebook = "rulebook" // the firm's rule book, as the YAML rule store
	SourceBuiltin  = "builtin"  // runnable recon.rule/1 rules shipped with the tool
	SourceOffice   = "office"   // edits made in the office (Rule Builder, SPEC-010)
)

// A RuleFile is one rule file to load.
type RuleFile struct {
	RuleID   string
	Version  string // as written in the file
	Source   string // SourceRulebook, SourceBuiltin or SourceOffice
	Schema   string // "recon.rule/1" for rules the engine can evaluate
	TaxYears string // comma-separated, "" when the file names none
	FilePath string // for messages and the audit trail
	Body     []byte
	SHA256   string
}

// RuleStatus says how a load sets a rule's status.
type RuleStatus struct {
	Status string // the status a new rule gets
	Reason string // disabled_reason, when Status is StatusDisabled
	// Follow sets Status on every load, also for a rule already stored (a rule-book rule's
	// status follows its governance.enabled). Without it, a stored rule keeps its status,
	// except that an active rule whose file changed goes back to testing: the CA approved
	// the old text, not the new one.
	Follow bool
}

// PutRule stores f. Every source keeps its own revisions of a rule: an unchanged file (the
// same SHA-256 as the source's latest revision) adds nothing, a changed one adds a new
// revision of that source. A new revision becomes the rule's version in use, except that a
// built-in revision does not replace an office version in use (the office edit stands until
// it is reset, RU-12). It reports whether anything was written.
//
// A rule-book rule and an engine rule never share an ID. An office version of a built-in
// rule shares its ID by design.
func (t *Tx) PutRule(f RuleFile, st RuleStatus) (bool, error) {
	var clash string
	// A rule-book rule and an engine rule clash. An office edit with the structure of the rule
	// it edits does not: a Partner's edit of a rule-book rule (SPEC-010 scope 8).
	err := t.tx.QueryRow(`SELECT source FROM rule_versions WHERE rule_id = ?1 AND source <> ?2 AND (?2 = 'rulebook' OR source = 'rulebook')
		AND NOT (schema = ?3 AND (source = 'office' OR ?2 = 'office')) ORDER BY revision LIMIT 1`,
		f.RuleID, f.Source, f.Schema).Scan(&clash)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return false, fmt.Errorf("store: rule %s: %w", f.RuleID, err)
	default:
		return false, &ConflictError{Msg: fmt.Sprintf("rule %s is already loaded from the %s rules; %s cannot load it as a %s rule",
			f.RuleID, clash, f.FilePath, f.Source)}
	}

	var latestSHA string
	err = t.tx.QueryRow(`SELECT sha256 FROM rule_versions WHERE rule_id = ? AND source = ? ORDER BY revision DESC LIMIT 1`,
		f.RuleID, f.Source).Scan(&latestSHA)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return false, fmt.Errorf("store: rule %s: %w", f.RuleID, err)
	case latestSHA == f.SHA256:
		if !st.Follow {
			return false, nil
		}
		// A status a person set stands (SPEC-010 scope 8); only an unset one follows the file.
		res, err := t.tx.Exec(`UPDATE rule_state SET status = ?, disabled_reason = ?
			WHERE rule_id = ? AND status_set_by = '' AND (status <> ? OR disabled_reason <> ?)`,
			st.Status, reasonFor(st), f.RuleID, st.Status, reasonFor(st))
		if err != nil {
			return false, fmt.Errorf("store: rule %s: %w", f.RuleID, err)
		}
		n, _ := res.RowsAffected()
		return n > 0, nil
	}

	var maxRevision int64
	if err := t.tx.QueryRow(`SELECT COALESCE(MAX(revision), 0) FROM rule_versions WHERE rule_id = ?`, f.RuleID).Scan(&maxRevision); err != nil {
		return false, fmt.Errorf("store: rule %s: %w", f.RuleID, err)
	}
	res, err := t.tx.Exec(`INSERT INTO rule_versions (rule_id, revision, version, source, schema, tax_years, file_path, body_yaml, sha256, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'load-rules')`,
		f.RuleID, maxRevision+1, f.Version, f.Source, f.Schema, f.TaxYears, f.FilePath, string(f.Body), f.SHA256)
	if err != nil {
		return false, fmt.Errorf("store: rule %s: %w", f.RuleID, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return false, err
	}

	var activeSource string
	var activeStatus string
	err = t.tx.QueryRow(`SELECT v.source, s.status FROM rule_state s JOIN rule_versions v ON v.id = s.active_version_id WHERE s.rule_id = ?`,
		f.RuleID).Scan(&activeSource, &activeStatus)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = t.tx.Exec(`INSERT INTO rule_state (rule_id, active_version_id, status, disabled_reason) VALUES (?, ?, ?, ?)`,
			f.RuleID, id, st.Status, reasonFor(st))
	case err != nil:
		return false, fmt.Errorf("store: rule %s: %w", f.RuleID, err)
	case st.Follow:
		// A rule-book rule: a changed file becomes the version in use unless an office edit
		// is in use (it stands until reset), and its status follows the file unless a person
		// set it (SPEC-010 scope 8).
		_, err = t.tx.Exec(`UPDATE rule_state SET
			active_version_id = CASE WHEN ? = 'office' THEN active_version_id ELSE ? END,
			status = CASE WHEN status_set_by = '' THEN ? ELSE status END,
			disabled_reason = CASE WHEN status_set_by = '' THEN ? ELSE disabled_reason END
			WHERE rule_id = ?`,
			activeSource, id, st.Status, reasonFor(st), f.RuleID)
	case f.Source == SourceBuiltin && activeSource == SourceOffice:
		// The office version stays in use; the new built-in text waits for a reset.
	default:
		// A changed text of an active rule is testing again: the Partner approved the old text (RU-40).
		_, err = t.tx.Exec(`UPDATE rule_state SET active_version_id = ?,
			status = CASE status WHEN 'active' THEN 'testing' ELSE status END,
			approved_by = CASE status WHEN 'active' THEN '' ELSE approved_by END,
			approved_at = CASE status WHEN 'active' THEN '' ELSE approved_at END
			WHERE rule_id = ?`, id, f.RuleID)
	}
	if err != nil {
		return false, fmt.Errorf("store: rule %s: %w", f.RuleID, err)
	}
	return true, nil
}

func reasonFor(st RuleStatus) string {
	if st.Status != StatusDisabled {
		return ""
	}
	return st.Reason
}

// A StoredRule is a rule's active version and its status.
type StoredRule struct {
	VersionID int64
	RuleID    string
	Version   string
	Revision  int64
	Source    string
	Schema    string
	Status    string
	FilePath  string
	Body      []byte
	SHA256    string
}

// Rules returns every rule's active version, sorted by rule ID.
func (t *Tx) Rules() ([]StoredRule, error) {
	rows, err := t.tx.Query(`SELECT v.id, v.rule_id, v.version, v.revision, v.source, v.schema, s.status, v.file_path, v.body_yaml, v.sha256
		FROM rule_state s JOIN rule_versions v ON v.id = s.active_version_id ORDER BY v.rule_id`)
	if err != nil {
		return nil, fmt.Errorf("store: rules: %w", err)
	}
	defer rows.Close()
	var out []StoredRule
	for rows.Next() {
		var r StoredRule
		var body string
		if err := rows.Scan(&r.VersionID, &r.RuleID, &r.Version, &r.Revision, &r.Source, &r.Schema, &r.Status, &r.FilePath, &body, &r.SHA256); err != nil {
			return nil, fmt.Errorf("store: rules: %w", err)
		}
		r.Body = []byte(body)
		out = append(out, r)
	}
	return out, rows.Err()
}

// A RuleState is what the database says about one rule: the version in use and its status.
type RuleState struct {
	RuleID          string
	ActiveVersionID int64
	ActiveSource    string
	Status          string
	DisabledReason  string
}

// RuleStateOf returns the state of rule id; found is false when the database has no such rule.
func (t *Tx) RuleStateOf(id string) (RuleState, bool, error) {
	var s RuleState
	err := t.tx.QueryRow(`SELECT s.rule_id, s.active_version_id, v.source, s.status, s.disabled_reason
		FROM rule_state s JOIN rule_versions v ON v.id = s.active_version_id WHERE s.rule_id = ?`, id).
		Scan(&s.RuleID, &s.ActiveVersionID, &s.ActiveSource, &s.Status, &s.DisabledReason)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return RuleState{}, false, nil
	case err != nil:
		return RuleState{}, false, fmt.Errorf("store: rule %s: %w", id, err)
	}
	return s, true, nil
}

// SetRuleStatus sets the status of rule id, with the reason it is disabled for (RU-40, RU-44).
// The rule's versions are not touched. Leaving active clears the approval: a rule switched off
// or on again must be approved again (RU-40).
func (t *Tx) SetRuleStatus(id, status, reason string) error {
	return t.SetRuleStatusBy(id, status, reason, "")
}

// SetRuleStatusBy is SetRuleStatus by a named person. A non-empty by is kept in status_set_by:
// from then on, loading the rule book again does not change the status (SPEC-010 scope 8).
func (t *Tx) SetRuleStatusBy(id, status, reason, by string) error {
	res, err := t.tx.Exec(`UPDATE rule_state SET status = ?, disabled_reason = ?,
		approved_by = '', approved_at = '', status_set_by = CASE WHEN ? = '' THEN status_set_by ELSE ? END WHERE rule_id = ?`,
		status, reason, by, by, id)
	if err != nil {
		return fmt.Errorf("store: rule %s: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return fmt.Errorf("store: rule %s is not in the rule state", id)
	}
	return nil
}

// PutOfficeVersion stores body as a new office revision of rule id, made by by, and makes it
// the version in use. The rule's status is not changed. It is how a Partner's edit of a
// rule-book rule is kept (SPEC-010 scope 8): the rule-book file itself is never written.
// It returns the new version's row ID.
func (t *Tx) PutOfficeVersion(id, version, schema, filePath string, body []byte, sha, by string) (int64, error) {
	var maxRevision int64
	if err := t.tx.QueryRow(`SELECT COALESCE(MAX(revision), 0) FROM rule_versions WHERE rule_id = ?`, id).Scan(&maxRevision); err != nil {
		return 0, fmt.Errorf("store: rule %s: %w", id, err)
	}
	if maxRevision == 0 {
		return 0, fmt.Errorf("store: rule %s is not in the rule state", id)
	}
	res, err := t.tx.Exec(`INSERT INTO rule_versions (rule_id, revision, version, source, schema, tax_years, file_path, body_yaml, sha256, created_by)
		VALUES (?, ?, ?, ?, ?, '', ?, ?, ?, ?)`, id, maxRevision+1, version, SourceOffice, schema, filePath, string(body), sha, by)
	if err != nil {
		return 0, fmt.Errorf("store: rule %s: %w", id, err)
	}
	vid, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err := t.tx.Exec(`UPDATE rule_state SET active_version_id = ? WHERE rule_id = ?`, vid, id); err != nil {
		return 0, fmt.Errorf("store: rule %s: %w", id, err)
	}
	return vid, nil
}

// ApproveRule makes testing rule id active, approved by by at time at (RU-40). The caller
// checks the gate and writes the audit entry.
func (t *Tx) ApproveRule(id, by, at string) error {
	res, err := t.tx.Exec(`UPDATE rule_state SET status = ?, approved_by = ?, approved_at = ?, disabled_reason = ''
		WHERE rule_id = ?`, StatusActive, by, at, id)
	if err != nil {
		return fmt.Errorf("store: rule %s: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return fmt.Errorf("store: rule %s is not in the rule state", id)
	}
	return nil
}

// ClientRunsWithTesting counts the finished runs for a client that included testing rules and
// used version versionID of rule id (SPEC-010 §4: the approval gate needs one).
func (t *Tx) ClientRunsWithTesting(id string, versionID int64) (int, error) {
	var n int
	err := t.tx.QueryRow(`SELECT COUNT(*) FROM run_rules rr JOIN runs r ON r.id = rr.run_id
		JOIN engagements e ON e.id = r.engagement_id
		WHERE rr.rule_id = ? AND rr.rule_version_id = ? AND r.include_testing = 1 AND e.client_id > 0 AND r.finished_at <> ''`,
		id, versionID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("store: rule %s: %w", id, err)
	}
	return n, nil
}

// A RuleVersion is one stored version of a rule (rule_versions) and whether it is in use.
type RuleVersion struct {
	ID        int64
	RuleID    string
	Version   string
	Revision  int64
	Source    string
	Schema    string
	TaxYears  string // comma-separated, "" when the file names none
	FilePath  string
	Body      []byte
	SHA256    string
	CreatedBy string
	CreatedAt string
	Active    bool
}

// RuleVersions returns every stored version of rule id, oldest first.
func (t *Tx) RuleVersions(id string) ([]RuleVersion, error) {
	rows, err := t.tx.Query(`SELECT v.id, v.rule_id, v.version, v.revision, v.source, v.schema, v.tax_years, v.file_path, v.body_yaml, v.sha256, v.created_by, v.created_at,
		(s.active_version_id IS v.id) FROM rule_versions v LEFT JOIN rule_state s ON s.rule_id = v.rule_id
		WHERE v.rule_id = ? ORDER BY v.revision`, id)
	if err != nil {
		return nil, fmt.Errorf("store: rule versions %s: %w", id, err)
	}
	defer rows.Close()
	var out []RuleVersion
	for rows.Next() {
		var v RuleVersion
		var body string
		var active int
		if err := rows.Scan(&v.ID, &v.RuleID, &v.Version, &v.Revision, &v.Source, &v.Schema, &v.TaxYears, &v.FilePath, &body, &v.SHA256, &v.CreatedBy, &v.CreatedAt, &active); err != nil {
			return nil, fmt.Errorf("store: rule versions %s: %w", id, err)
		}
		v.Body, v.Active = []byte(body), active == 1
		out = append(out, v)
	}
	return out, rows.Err()
}

// ActivateVersion makes version versionID of rule id the version in use (RU-41: switching
// back to an earlier version, or resetting to the standard rule). The version must belong to
// the rule. An active rule that changes version is testing again: the Partner approved the
// version it had, not this one (RU-40).
func (t *Tx) ActivateVersion(id string, versionID int64) error {
	// A described-only (rule-book) version is never approved, so its status stays as it is.
	res, err := t.tx.Exec(`UPDATE rule_state SET active_version_id = ?1,
		status = CASE WHEN status = 'active' AND (SELECT schema FROM rule_versions WHERE id = ?1) = 'recon.rule/1' THEN 'testing' ELSE status END,
		approved_by = CASE WHEN status = 'active' AND (SELECT schema FROM rule_versions WHERE id = ?1) = 'recon.rule/1' THEN '' ELSE approved_by END,
		approved_at = CASE WHEN status = 'active' AND (SELECT schema FROM rule_versions WHERE id = ?1) = 'recon.rule/1' THEN '' ELSE approved_at END
		WHERE rule_id = ?2 AND EXISTS (SELECT 1 FROM rule_versions WHERE id = ?1 AND rule_id = ?2)`, versionID, id)
	if err != nil {
		return fmt.Errorf("store: rule %s: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return fmt.Errorf("store: rule %s has no version %d", id, versionID)
	}
	return nil
}

// LatestVersionOf returns the newest stored version of rule id from source; found is false
// when the rule has none from that source.
func (t *Tx) LatestVersionOf(id, source string) (RuleVersion, bool, error) {
	vs, err := t.RuleVersions(id)
	if err != nil {
		return RuleVersion{}, false, err
	}
	for i := len(vs) - 1; i >= 0; i-- {
		if vs[i].Source == source {
			return vs[i], true, nil
		}
	}
	return RuleVersion{}, false, nil
}
