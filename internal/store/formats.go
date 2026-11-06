package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// Document formats and parameters (SPEC-016, migration 0011). They are stored as rules are:
// every version is kept and never changed (the tables refuse updates and deletes), and a state
// row says which version is in use and whether it may be used. The engine is the database's
// only writer; recon-server asks it, as it does for rules.

// Where a format or parameter came from.
const (
	OriginBuiltin = "builtin"
	OriginOffice  = "office"
)

// The states of a format. A parameter is only active or disabled.
const (
	FormatDraft    = "draft"
	FormatTesting  = "testing"
	FormatActive   = "active"
	FormatDisabled = "disabled"
)

// A FormatVersion is one version of a format. Revision and Version are set by AddFormatVersion.
type FormatVersion struct {
	ID        int64
	FormatID  string
	Revision  int
	Version   string // "v1", "v2", ...
	Name      string
	Source    string // OriginBuiltin or OriginOffice
	FileType  string // "json", "xlsx" or "csv"
	Document  string // which parameters it fills
	BodyYAML  string
	SHA256    string
	Note      string
	Status    string // the status it was saved with
	CreatedBy string
	CreatedAt string
	// SPEC-016 part 2: the mapping the screen edits, the profile's own version, and when the version was
	// published. PublishedAt is empty for a draft; AddFormatVersion publishes at once when it is set.
	MappingJSON    string
	ProfileVersion string
	PublishedAt    string
	PublishedBy    string
	// SPEC-022: the document type (code) the format reads. AddFormatVersion fills it from the
	// document_type line of BodyYAML when it is left empty.
	DocumentType string
}

// A FormatState is where a format stands now.
type FormatState struct {
	FormatID        string
	ActiveVersionID int64
	Status          string
	PublishedBy     string
	PublishedAt     string
	DisabledReason  string
}

// A StoredFormat is a format with its version in use.
type StoredFormat struct {
	State   FormatState
	Version FormatVersion
}

// AddFormatVersion saves a new version of a format (the first one when the format is new) and
// points the format's state at it, with the version's status. Nothing earlier is changed.
func (t *Tx) AddFormatVersion(v FormatVersion) (int64, error) {
	if v.FormatID == "" || v.Name == "" || v.BodyYAML == "" || v.SHA256 == "" || v.CreatedBy == "" {
		return 0, errors.New("store: format version needs an ID, a name, a body, a hash and an author")
	}
	var last sql.NullInt64
	if err := t.tx.QueryRow(`SELECT MAX(revision) FROM format_versions WHERE format_id = ?`, v.FormatID).Scan(&last); err != nil {
		return 0, fmt.Errorf("store: format version: %w", err)
	}
	rev := int(last.Int64) + 1
	if v.DocumentType == "" {
		v.DocumentType = documentTypeOfProfile(v.BodyYAML)
	}
	res, err := t.tx.Exec(`INSERT INTO format_versions (format_id, revision, version, name, source, file_type, document, body_yaml, sha256, note, status, created_by,
		mapping_json, profile_version, published_at, published_by, document_type) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.FormatID, rev, fmt.Sprintf("v%d", rev), v.Name, v.Source, v.FileType, v.Document, v.BodyYAML, v.SHA256, v.Note, v.Status, v.CreatedBy,
		v.MappingJSON, v.ProfileVersion, v.PublishedAt, v.PublishedBy, v.DocumentType)
	if err != nil {
		return 0, fmt.Errorf("store: format version: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if v.Status == FormatDraft {
		return id, nil // a draft is not in use: the state is not touched (see AddFormatDraft)
	}
	_, err = t.tx.Exec(`INSERT INTO format_state (format_id, active_version_id, status) VALUES (?, ?, ?)
		ON CONFLICT (format_id) DO UPDATE SET active_version_id = excluded.active_version_id, status = excluded.status, disabled_reason = ''`,
		v.FormatID, id, v.Status)
	if err != nil {
		return 0, fmt.Errorf("store: format state: %w", err)
	}
	return id, nil
}

const formatVersionColumns = `id, format_id, revision, version, name, source, file_type, document, body_yaml, sha256, note, status, created_by, created_at, mapping_json, profile_version, published_at, published_by, document_type`

func scanFormatVersion(row interface{ Scan(...any) error }) (FormatVersion, error) {
	var v FormatVersion
	err := row.Scan(&v.ID, &v.FormatID, &v.Revision, &v.Version, &v.Name, &v.Source, &v.FileType, &v.Document, &v.BodyYAML, &v.SHA256, &v.Note, &v.Status, &v.CreatedBy, &v.CreatedAt, &v.MappingJSON, &v.ProfileVersion, &v.PublishedAt, &v.PublishedBy, &v.DocumentType)
	return v, err
}

// FormatStateOf returns the state of a format, and whether it exists.
func (t *Tx) FormatStateOf(id string) (FormatState, bool, error) {
	var s FormatState
	err := t.tx.QueryRow(`SELECT format_id, active_version_id, status, published_by, published_at, disabled_reason FROM format_state WHERE format_id = ?`, id).
		Scan(&s.FormatID, &s.ActiveVersionID, &s.Status, &s.PublishedBy, &s.PublishedAt, &s.DisabledReason)
	if errors.Is(err, sql.ErrNoRows) {
		return FormatState{}, false, nil
	}
	if err != nil {
		return FormatState{}, false, fmt.Errorf("store: format state: %w", err)
	}
	return s, true, nil
}

// Formats lists every format with its version in use, by format ID.
func (t *Tx) Formats() ([]StoredFormat, error) {
	rows, err := t.tx.Query(`SELECT s.format_id, s.active_version_id, s.status, s.published_by, s.published_at, s.disabled_reason,
		v.id, v.format_id, v.revision, v.version, v.name, v.source, v.file_type, v.document, v.body_yaml, v.sha256, v.note, v.status, v.created_by, v.created_at, v.mapping_json, v.profile_version, v.published_at, v.published_by
		FROM format_state s JOIN format_versions v ON v.id = s.active_version_id ORDER BY s.format_id`)
	if err != nil {
		return nil, fmt.Errorf("store: formats: %w", err)
	}
	defer rows.Close()
	var out []StoredFormat
	for rows.Next() {
		var f StoredFormat
		s, v := &f.State, &f.Version
		if err := rows.Scan(&s.FormatID, &s.ActiveVersionID, &s.Status, &s.PublishedBy, &s.PublishedAt, &s.DisabledReason,
			&v.ID, &v.FormatID, &v.Revision, &v.Version, &v.Name, &v.Source, &v.FileType, &v.Document, &v.BodyYAML, &v.SHA256, &v.Note, &v.Status, &v.CreatedBy, &v.CreatedAt, &v.MappingJSON, &v.ProfileVersion, &v.PublishedAt, &v.PublishedBy); err != nil {
			return nil, fmt.Errorf("store: formats: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// FormatVersions lists every version of a format, oldest first.
func (t *Tx) FormatVersions(id string) ([]FormatVersion, error) {
	rows, err := t.tx.Query(`SELECT `+formatVersionColumns+` FROM format_versions WHERE format_id = ? ORDER BY revision`, id)
	if err != nil {
		return nil, fmt.Errorf("store: format versions: %w", err)
	}
	defer rows.Close()
	var out []FormatVersion
	for rows.Next() {
		v, err := scanFormatVersion(rows)
		if err != nil {
			return nil, fmt.Errorf("store: format versions: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// FormatVersionByID returns one version, and whether it exists.
func (t *Tx) FormatVersionByID(id int64) (FormatVersion, bool, error) {
	v, err := scanFormatVersion(t.tx.QueryRow(`SELECT `+formatVersionColumns+` FROM format_versions WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return FormatVersion{}, false, nil
	}
	if err != nil {
		return FormatVersion{}, false, fmt.Errorf("store: format version: %w", err)
	}
	return v, true, nil
}

// SetFormatStatus changes whether a format may be used, with the reason (kept for a disabled
// one). Enabling again is a change to testing or active; the caller decides which.
func (t *Tx) SetFormatStatus(id, status, reason, by, at string) error {
	switch status {
	case FormatDraft, FormatTesting, FormatActive, FormatDisabled:
	default:
		return fmt.Errorf("store: format status %q is not known", status)
	}
	if status == FormatDisabled {
		if reason == "" {
			return errors.New("store: a format is disabled with a reason")
		}
	} else {
		reason = ""
	}
	res, err := t.tx.Exec(`UPDATE format_state SET status = ?, disabled_reason = ?,
		published_by = CASE WHEN ?1 = 'active' THEN ?3 ELSE published_by END,
		published_at = CASE WHEN ?1 = 'active' THEN ?4 ELSE published_at END WHERE format_id = ?5`, status, reason, by, at, id)
	if err != nil {
		return fmt.Errorf("store: format status: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store: format %s is not stored", id)
	}
	return nil
}

// UseFormatVersion points a format at an earlier version (switching back) with the status the
// caller gives it. No version is changed or removed.
func (t *Tx) UseFormatVersion(id string, versionID int64, status string) error {
	v, ok, err := t.FormatVersionByID(versionID)
	if err != nil {
		return err
	}
	if !ok || v.FormatID != id {
		return fmt.Errorf("store: version %d is not a version of format %s", versionID, id)
	}
	if v.PublishedAt == "" {
		return fmt.Errorf("store: version %d of format %s was never published, so it cannot be switched to", versionID, id)
	}
	res, err := t.tx.Exec(`UPDATE format_state SET active_version_id = ?, status = ?, disabled_reason = '' WHERE format_id = ?`, versionID, status, id)
	if err != nil {
		return fmt.Errorf("store: use format version: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store: format %s is not stored", id)
	}
	return nil
}

// A ParameterVersion is one version of a parameter; its aliases are in the body.
type ParameterVersion struct {
	ID          int64
	ParameterID string // the key, for example "gst.taxable_value"
	Revision    int
	Version     string
	Name        string
	Source      string
	ParamType   string // money, int, text, date, bool, rate or record (the vocabulary kinds)
	Document    string
	Required    bool
	BodyYAML    string
	SHA256      string
	CreatedBy   string
	CreatedAt   string
}

// A ParameterState says which version of a parameter is in use and whether it is offered.
type ParameterState struct {
	ParameterID     string
	ActiveVersionID int64
	Status          string // "active" or "disabled"
	DisabledReason  string
}

// A StoredParameter is a parameter with its version in use.
type StoredParameter struct {
	State   ParameterState
	Version ParameterVersion
}

// AddParameterVersion saves a new version of a parameter (the first when it is new) and points
// its state at it. A new parameter starts active; a parameter that was disabled stays so.
func (t *Tx) AddParameterVersion(v ParameterVersion) (int64, error) {
	if v.ParameterID == "" || v.Name == "" || v.BodyYAML == "" || v.SHA256 == "" || v.CreatedBy == "" {
		return 0, errors.New("store: parameter version needs a key, a name, a body, a hash and an author")
	}
	var last sql.NullInt64
	if err := t.tx.QueryRow(`SELECT MAX(revision) FROM parameter_versions WHERE parameter_id = ?`, v.ParameterID).Scan(&last); err != nil {
		return 0, fmt.Errorf("store: parameter version: %w", err)
	}
	rev := int(last.Int64) + 1
	required := 0
	if v.Required {
		required = 1
	}
	res, err := t.tx.Exec(`INSERT INTO parameter_versions (parameter_id, revision, version, name, source, param_type, document, required, body_yaml, sha256, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ParameterID, rev, fmt.Sprintf("v%d", rev), v.Name, v.Source, v.ParamType, v.Document, required, v.BodyYAML, v.SHA256, v.CreatedBy)
	if err != nil {
		return 0, fmt.Errorf("store: parameter version: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	_, err = t.tx.Exec(`INSERT INTO parameter_state (parameter_id, active_version_id, status) VALUES (?, ?, 'active')
		ON CONFLICT (parameter_id) DO UPDATE SET active_version_id = excluded.active_version_id`, v.ParameterID, id)
	if err != nil {
		return 0, fmt.Errorf("store: parameter state: %w", err)
	}
	return id, nil
}

// Parameters lists every parameter with its version in use, by key.
func (t *Tx) Parameters() ([]StoredParameter, error) {
	rows, err := t.tx.Query(`SELECT s.parameter_id, s.active_version_id, s.status, s.disabled_reason,
		v.id, v.parameter_id, v.revision, v.version, v.name, v.source, v.param_type, v.document, v.required, v.body_yaml, v.sha256, v.created_by, v.created_at
		FROM parameter_state s JOIN parameter_versions v ON v.id = s.active_version_id ORDER BY s.parameter_id`)
	if err != nil {
		return nil, fmt.Errorf("store: parameters: %w", err)
	}
	defer rows.Close()
	var out []StoredParameter
	for rows.Next() {
		var p StoredParameter
		s, v := &p.State, &p.Version
		var required int
		if err := rows.Scan(&s.ParameterID, &s.ActiveVersionID, &s.Status, &s.DisabledReason,
			&v.ID, &v.ParameterID, &v.Revision, &v.Version, &v.Name, &v.Source, &v.ParamType, &v.Document, &required, &v.BodyYAML, &v.SHA256, &v.CreatedBy, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: parameters: %w", err)
		}
		v.Required = required == 1
		out = append(out, p)
	}
	return out, rows.Err()
}

// SetParameterStatus switches a parameter off (with a reason) or on again.
func (t *Tx) SetParameterStatus(key, status, reason string) error {
	switch status {
	case "active":
		reason = ""
	case "disabled":
		if reason == "" {
			return errors.New("store: a parameter is disabled with a reason")
		}
	default:
		return fmt.Errorf("store: parameter status %q is not known", status)
	}
	res, err := t.tx.Exec(`UPDATE parameter_state SET status = ?, disabled_reason = ? WHERE parameter_id = ?`, status, reason, key)
	if err != nil {
		return fmt.Errorf("store: parameter status: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store: parameter %s is not stored", key)
	}
	return nil
}

// A FormatSample is a sample file kept for a format. Content is nil when only the hash was kept.
type FormatSample struct {
	ID         int64
	FormatID   string
	FileSHA256 string
	FileName   string
	SizeBytes  int64
	Content    []byte
	KeptBy     string
	KeptAt     string
}

// KeepSample records a sample for a format. The same file (by hash) is kept once.
func (t *Tx) KeepSample(s FormatSample) (int64, error) {
	if s.FormatID == "" || s.FileSHA256 == "" || s.FileName == "" || s.KeptBy == "" {
		return 0, errors.New("store: a sample needs a format, a hash, a name and who kept it")
	}
	res, err := t.tx.Exec(`INSERT INTO format_samples (format_id, file_sha256, file_name, size_bytes, content, kept_by) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (format_id, file_sha256) DO NOTHING`, s.FormatID, s.FileSHA256, s.FileName, s.SizeBytes, s.Content, s.KeptBy)
	if err != nil {
		return 0, fmt.Errorf("store: keep sample: %w", err)
	}
	return res.LastInsertId()
}

// FormatSamples lists the samples kept for a format, oldest first.
func (t *Tx) FormatSamples(formatID string) ([]FormatSample, error) {
	rows, err := t.tx.Query(`SELECT id, format_id, file_sha256, file_name, size_bytes, content, kept_by, kept_at FROM format_samples WHERE format_id = ? ORDER BY id`, formatID)
	if err != nil {
		return nil, fmt.Errorf("store: samples: %w", err)
	}
	defer rows.Close()
	var out []FormatSample
	for rows.Next() {
		var s FormatSample
		if err := rows.Scan(&s.ID, &s.FormatID, &s.FileSHA256, &s.FileName, &s.SizeBytes, &s.Content, &s.KeptBy, &s.KeptAt); err != nil {
			return nil, fmt.Errorf("store: samples: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// RemoveSample deletes a kept sample (an admin may; the audit entry stays).
func (t *Tx) RemoveSample(id int64) error {
	if _, err := t.tx.Exec(`DELETE FROM format_samples WHERE id = ?`, id); err != nil {
		return fmt.Errorf("store: remove sample: %w", err)
	}
	return nil
}

// AddFormatDraft saves an edit of an existing format as a draft version. The format keeps the
// version it had: a draft is not used by any run until it is published.
func (t *Tx) AddFormatDraft(v FormatVersion) (int64, error) {
	if _, ok, err := t.FormatStateOf(v.FormatID); err != nil {
		return 0, err
	} else if !ok {
		return 0, fmt.Errorf("store: format %s is not stored, so it has no draft", v.FormatID)
	}
	v.Status, v.PublishedAt, v.PublishedBy = FormatDraft, "", ""
	return t.AddFormatVersion(v)
}

// PublishFormatVersion records that a version was published and makes it the format's version in
// use, as testing or active. A version is published once. It does not write the profile files:
// the caller does that first, because the engine reads them.
func (t *Tx) PublishFormatVersion(formatID string, versionID int64, status, by, at string) error {
	if status != FormatTesting && status != FormatActive {
		return fmt.Errorf("store: a version is published as testing or active, not %q", status)
	}
	v, ok, err := t.FormatVersionByID(versionID)
	if err != nil {
		return err
	}
	if !ok || v.FormatID != formatID {
		return fmt.Errorf("store: version %d is not a version of format %s", versionID, formatID)
	}
	if v.PublishedAt == "" {
		if _, err := t.tx.Exec(`UPDATE format_versions SET published_at = ?, published_by = ? WHERE id = ?`, at, by, versionID); err != nil {
			return fmt.Errorf("store: publish format version: %w", err)
		}
	}
	res, err := t.tx.Exec(`UPDATE format_state SET active_version_id = ?, status = ?, disabled_reason = '',
		published_by = CASE WHEN ?2 = 'active' THEN ?3 ELSE published_by END, published_at = CASE WHEN ?2 = 'active' THEN ?4 ELSE published_at END
		WHERE format_id = ?5`, versionID, status, by, at, formatID)
	if err != nil {
		return fmt.Errorf("store: publish format version: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store: format %s is not stored", formatID)
	}
	return nil
}

// FormatVersionBySHA returns the version of a format whose profile has this hash, and whether
// there is one. A run uses it to say which stored version it read with.
func (t *Tx) FormatVersionBySHA(formatID, sha string) (FormatVersion, bool, error) {
	v, err := scanFormatVersion(t.tx.QueryRow(`SELECT `+formatVersionColumns+` FROM format_versions WHERE format_id = ? AND sha256 = ? ORDER BY revision DESC LIMIT 1`, formatID, sha))
	if errors.Is(err, sql.ErrNoRows) {
		return FormatVersion{}, false, nil
	}
	if err != nil {
		return FormatVersion{}, false, fmt.Errorf("store: format version: %w", err)
	}
	return v, true, nil
}

// An UnrecognisedFile is a file no format recognised. Its content is never kept.
type UnrecognisedFile struct {
	ID           int64
	FileName     string
	FileSHA256   string
	SizeBytes    int64
	FileType     string
	ClientID     int64 // 0 when the run had no client
	TaxYear      string
	NotedBy      string
	NotedAt      string
	Status       string // "open", "taught" or "dismissed"
	ResolvedBy   string
	ResolvedAt   string
	ResolvedNote string
}

// NoteUnrecognised records a file no format recognised. The same file is noted once while open.
func (t *Tx) NoteUnrecognised(u UnrecognisedFile) error {
	if u.FileName == "" || u.FileSHA256 == "" || u.NotedBy == "" {
		return errors.New("store: an unrecognised file needs a name, a hash and who noted it")
	}
	_, err := t.tx.Exec(`INSERT INTO unrecognised_files (file_name, file_sha256, size_bytes, file_type, client_id, tax_year, noted_by, noted_at)
		VALUES (?, ?, ?, ?, NULLIF(?, 0), ?, ?, ?) ON CONFLICT DO NOTHING`,
		u.FileName, u.FileSHA256, u.SizeBytes, u.FileType, u.ClientID, u.TaxYear, u.NotedBy, u.NotedAt)
	if err != nil {
		return fmt.Errorf("store: note unrecognised: %w", err)
	}
	return nil
}

// OpenUnrecognised lists the files still waiting for a format, oldest first.
func (t *Tx) OpenUnrecognised() ([]UnrecognisedFile, error) {
	rows, err := t.tx.Query(`SELECT id, file_name, file_sha256, size_bytes, file_type, COALESCE(client_id, 0), tax_year, noted_by, noted_at
		FROM unrecognised_files WHERE status = 'open' ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("store: unrecognised: %w", err)
	}
	defer rows.Close()
	var out []UnrecognisedFile
	for rows.Next() {
		u := UnrecognisedFile{Status: "open"}
		if err := rows.Scan(&u.ID, &u.FileName, &u.FileSHA256, &u.SizeBytes, &u.FileType, &u.ClientID, &u.TaxYear, &u.NotedBy, &u.NotedAt); err != nil {
			return nil, fmt.Errorf("store: unrecognised: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// ResolveUnrecognised closes an open note as taught or dismissed. It reports false when there is
// no open note with this ID.
func (t *Tx) ResolveUnrecognised(id int64, status, by, at, note string) (bool, error) {
	if status != "taught" && status != "dismissed" {
		return false, fmt.Errorf("store: an unrecognised file is closed as taught or dismissed, not %q", status)
	}
	res, err := t.tx.Exec(`UPDATE unrecognised_files SET status = ?, resolved_by = ?, resolved_at = ?, resolved_note = ? WHERE id = ? AND status = 'open'`,
		status, by, at, note, id)
	if err != nil {
		return false, fmt.Errorf("store: resolve unrecognised: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// FormatSampleByID returns one kept sample, and whether it exists.
func (t *Tx) FormatSampleByID(id int64) (FormatSample, bool, error) {
	var s FormatSample
	err := t.tx.QueryRow(`SELECT id, format_id, file_sha256, file_name, size_bytes, content, kept_by, kept_at FROM format_samples WHERE id = ?`, id).
		Scan(&s.ID, &s.FormatID, &s.FileSHA256, &s.FileName, &s.SizeBytes, &s.Content, &s.KeptBy, &s.KeptAt)
	if errors.Is(err, sql.ErrNoRows) {
		return FormatSample{}, false, nil
	}
	if err != nil {
		return FormatSample{}, false, fmt.Errorf("store: sample: %w", err)
	}
	return s, true, nil
}

// LastUsedByProfile is when a file was last read with a profile of this ID (the upload time of the
// newest document version), or "" when none was.
func (t *Tx) LastUsedByProfile(profileID string) (string, error) {
	var at sql.NullString
	if err := t.tx.QueryRow(`SELECT MAX(uploaded_at) FROM document_versions WHERE profile_id = ?`, profileID).Scan(&at); err != nil {
		return "", fmt.Errorf("store: last used: %w", err)
	}
	return at.String, nil
}
