package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// The Document Master (SPEC-022): the kinds of document the Partner has added, the record
// structure of each, and the vocabulary snapshots runs point at. Rules about what a valid
// definition is live in the vocabulary layer; this file stores and reads what it is given.

// A DocType is one row of document_types: a built-in or an office document.
type DocType struct {
	Code          string
	Origin        string // OriginBuiltin or OriginOffice
	Name          string
	Category      string
	Description   string
	ApplicableFor []string // "json", "xlsx", "csv", in that order
	RecordType    string   // "" while an office document is a Draft
	Active        bool
	CreatedBy     string
	CreatedAt     string
	UpdatedBy     string
	UpdatedAt     string
}

// An OfficeColumn is one column of an office record type.
type OfficeColumn struct {
	Position int
	Name     string
	Label    string
	Type     string // "text", "date" or "amount"
	Key      bool
	Required bool
}

// An OfficeRecordType is a record type the Partner defined, with its columns in order.
type OfficeRecordType struct {
	Name        string
	Description string
	CopiedFrom  string // information only
	CreatedBy   string
	CreatedAt   string
	Columns     []OfficeColumn
}

// A Usage says what reads a document or a record type. Every list is sorted.
type Usage struct {
	Formats     []string // format IDs with any version that reads the document
	Records     int      // stored rows under the record type
	Rules       []string // rule IDs with any version that reads the record type
	ActiveRules []string // rule IDs whose version in use reads it and whose status is active
}

// InUse reports whether anything at all reads it, which is what locks a record type and
// stops a delete.
func (u Usage) InUse() bool { return len(u.Formats) > 0 || u.Records > 0 || len(u.Rules) > 0 }

var documentTypeLine = regexp.MustCompile(`(?m)^document_type:[ \t]*(.*?)[ \t]*$`)

// documentTypeOfProfile reads the first top-level document_type line of a profile, or "".
func documentTypeOfProfile(body string) string {
	m := documentTypeLine.FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	v, _, _ := strings.Cut(m[1], " #")
	return strings.Trim(strings.TrimSpace(v), `"'`)
}

type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
}

// backfillFormatDocumentTypes fills format_versions.document_type for versions saved before
// migration 0015. It only touches rows where the column is empty and the profile names a
// document type, so running it again changes nothing.
func backfillFormatDocumentTypes(x execer) error {
	rows, err := x.Query(`SELECT id, body_yaml FROM format_versions WHERE document_type = '' ORDER BY id`)
	if err != nil {
		return fmt.Errorf("store: document type backfill: %w", err)
	}
	type fill struct {
		id  int64
		doc string
	}
	var fills []fill
	for rows.Next() {
		var id int64
		var body string
		if err := rows.Scan(&id, &body); err != nil {
			rows.Close()
			return fmt.Errorf("store: document type backfill: %w", err)
		}
		if doc := documentTypeOfProfile(body); doc != "" {
			fills = append(fills, fill{id, doc})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("store: document type backfill: %w", err)
	}
	rows.Close()
	for _, f := range fills {
		if _, err := x.Exec(`UPDATE format_versions SET document_type = ? WHERE id = ?`, f.doc, f.id); err != nil {
			return fmt.Errorf("store: document type backfill: %w", err)
		}
	}
	return nil
}

const docTypeColumns = `code, origin, name, category, description, applicable_for, record_type, active, created_by, created_at, updated_by, updated_at`

func scanDocType(row interface{ Scan(...any) error }) (DocType, error) {
	var d DocType
	var formats string
	var active int
	if err := row.Scan(&d.Code, &d.Origin, &d.Name, &d.Category, &d.Description, &formats, &d.RecordType, &active, &d.CreatedBy, &d.CreatedAt, &d.UpdatedBy, &d.UpdatedAt); err != nil {
		return d, err
	}
	d.Active = active == 1
	if err := json.Unmarshal([]byte(formats), &d.ApplicableFor); err != nil {
		return d, fmt.Errorf("store: document %s: applicable formats: %w", d.Code, err)
	}
	return d, nil
}

func formatsJSON(fs []string) (string, error) {
	if len(fs) == 0 {
		fs = []string{}
	}
	b, err := json.Marshal(fs)
	return string(b), err
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// SeedBuiltinDocument adds or refreshes a built-in document. The name, description and record
// type come from the YAML each time; the active flag, the category and the formats the Partner
// may have changed are kept. It reports whether anything was written.
func (t *Tx) SeedBuiltinDocument(d DocType) (bool, error) {
	d.Origin = OriginBuiltin
	formats, err := formatsJSON(d.ApplicableFor)
	if err != nil {
		return false, err
	}
	res, err := t.tx.Exec(`INSERT INTO document_types (`+docTypeColumns+`) VALUES (?, 'builtin', ?, ?, ?, ?, ?, 1, ?, ?, ?, ?)
		ON CONFLICT (code) DO UPDATE SET name = excluded.name, description = excluded.description, record_type = excluded.record_type
		WHERE document_types.name <> excluded.name OR document_types.description <> excluded.description OR document_types.record_type <> excluded.record_type`,
		d.Code, d.Name, d.Category, d.Description, formats, d.RecordType, d.CreatedBy, d.CreatedAt, d.UpdatedBy, d.UpdatedAt)
	if err != nil {
		return false, fmt.Errorf("store: built-in document %s: %w", d.Code, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// AddDocType stores a new office document. A code already in use is a ConflictError.
func (t *Tx) AddDocType(d DocType) error {
	d.Origin = OriginOffice
	formats, err := formatsJSON(d.ApplicableFor)
	if err != nil {
		return err
	}
	if _, ok, err := t.DocTypeByCode(d.Code); err != nil {
		return err
	} else if ok {
		return &ConflictError{Msg: fmt.Sprintf("a document with the code %q already exists", d.Code)}
	}
	_, err = t.tx.Exec(`INSERT INTO document_types (`+docTypeColumns+`) VALUES (?, 'office', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.Code, d.Name, d.Category, d.Description, formats, d.RecordType, btoi(d.Active), d.CreatedBy, d.CreatedAt, d.UpdatedBy, d.UpdatedAt)
	if err != nil {
		return fmt.Errorf("store: add document %s: %w", d.Code, err)
	}
	return nil
}

// UpdateDocType changes an office document's name, category, description, formats, record type
// and active flag. The code and the origin are never changed.
func (t *Tx) UpdateDocType(d DocType) error {
	formats, err := formatsJSON(d.ApplicableFor)
	if err != nil {
		return err
	}
	res, err := t.tx.Exec(`UPDATE document_types SET name = ?, category = ?, description = ?, applicable_for = ?, record_type = ?, active = ?,
		updated_by = ?, updated_at = ? WHERE code = ? AND origin = 'office'`,
		d.Name, d.Category, d.Description, formats, d.RecordType, btoi(d.Active), d.UpdatedBy, d.UpdatedAt, d.Code)
	if err != nil {
		return fmt.Errorf("store: update document %s: %w", d.Code, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return &ConflictError{Msg: fmt.Sprintf("no office document with the code %q", d.Code)}
	}
	return nil
}

// SetDocActive switches any document (built-in or office) on or off for new layouts and rules.
func (t *Tx) SetDocActive(code string, active bool, by, at string) error {
	res, err := t.tx.Exec(`UPDATE document_types SET active = ?, updated_by = ?, updated_at = ? WHERE code = ?`, btoi(active), by, at, code)
	if err != nil {
		return fmt.Errorf("store: document %s: %w", code, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return &ConflictError{Msg: fmt.Sprintf("no document with the code %q", code)}
	}
	return nil
}

// DeleteDocType removes an office document that nothing uses, and its record type when no other
// document uses that. A built-in document, or one in use, is a ConflictError.
func (t *Tx) DeleteDocType(code string) error {
	d, ok, err := t.DocTypeByCode(code)
	if err != nil {
		return err
	}
	if !ok {
		return &ConflictError{Msg: fmt.Sprintf("no document with the code %q", code)}
	}
	if d.Origin != OriginOffice {
		return &ConflictError{Msg: fmt.Sprintf("%q is a built-in document and cannot be deleted", code)}
	}
	u, err := t.DocUsage(code)
	if err != nil {
		return err
	}
	if u.InUse() {
		return &ConflictError{Msg: fmt.Sprintf("%q is in use (formats %v, stored rows %d, rules %v); deactivate it instead", code, u.Formats, u.Records, u.Rules)}
	}
	if _, err := t.tx.Exec(`DELETE FROM document_types WHERE code = ?`, code); err != nil {
		return fmt.Errorf("store: delete document %s: %w", code, err)
	}
	if d.RecordType == "" {
		return nil
	}
	var others int
	if err := t.tx.QueryRow(`SELECT COUNT(*) FROM document_types WHERE record_type = ?`, d.RecordType).Scan(&others); err != nil {
		return fmt.Errorf("store: delete document %s: %w", code, err)
	}
	if others == 0 {
		return t.deleteOfficeRecordType(d.RecordType)
	}
	return nil
}

// DocTypes lists every document, in category, name and code order.
func (t *Tx) DocTypes() ([]DocType, error) {
	rows, err := t.tx.Query(`SELECT ` + docTypeColumns + ` FROM document_types ORDER BY category, name, code`)
	if err != nil {
		return nil, fmt.Errorf("store: documents: %w", err)
	}
	defer rows.Close()
	var out []DocType
	for rows.Next() {
		d, err := scanDocType(rows)
		if err != nil {
			return nil, fmt.Errorf("store: documents: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DocTypeByCode returns one document and whether it exists.
func (t *Tx) DocTypeByCode(code string) (DocType, bool, error) {
	d, err := scanDocType(t.tx.QueryRow(`SELECT `+docTypeColumns+` FROM document_types WHERE code = ?`, code))
	if errors.Is(err, sql.ErrNoRows) {
		return DocType{}, false, nil
	}
	if err != nil {
		return DocType{}, false, fmt.Errorf("store: document %s: %w", code, err)
	}
	return d, true, nil
}

// AddOfficeRecordType stores a new record type with its columns. A name already in use is a
// ConflictError. Columns are stored in the order given.
func (t *Tx) AddOfficeRecordType(rt OfficeRecordType) error {
	if _, ok, err := t.OfficeRecordTypeByName(rt.Name); err != nil {
		return err
	} else if ok {
		return &ConflictError{Msg: fmt.Sprintf("a record type named %q already exists", rt.Name)}
	}
	if _, err := t.tx.Exec(`INSERT INTO office_record_types (name, description, copied_from, created_by, created_at) VALUES (?, ?, ?, ?, ?)`,
		rt.Name, rt.Description, rt.CopiedFrom, rt.CreatedBy, rt.CreatedAt); err != nil {
		return fmt.Errorf("store: record type %s: %w", rt.Name, err)
	}
	return t.insertColumns(rt.Name, rt.Columns)
}

func (t *Tx) insertColumns(record string, cols []OfficeColumn) error {
	for i, c := range cols {
		if _, err := t.tx.Exec(`INSERT INTO office_record_columns (record_type, position, name, label, type, is_key, required) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			record, i+1, c.Name, c.Label, c.Type, btoi(c.Key), btoi(c.Required)); err != nil {
			return fmt.Errorf("store: record type %s column %s: %w", record, c.Name, err)
		}
	}
	return nil
}

// ReplaceOfficeColumns sets a record type's description and columns. Whether the change is
// allowed (locking) is decided by the caller from RecordTypeUsage.
func (t *Tx) ReplaceOfficeColumns(name, description string, cols []OfficeColumn) error {
	res, err := t.tx.Exec(`UPDATE office_record_types SET description = ? WHERE name = ?`, description, name)
	if err != nil {
		return fmt.Errorf("store: record type %s: %w", name, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return &ConflictError{Msg: fmt.Sprintf("no office record type named %q", name)}
	}
	if _, err := t.tx.Exec(`DELETE FROM office_record_columns WHERE record_type = ?`, name); err != nil {
		return fmt.Errorf("store: record type %s: %w", name, err)
	}
	return t.insertColumns(name, cols)
}

func (t *Tx) deleteOfficeRecordType(name string) error {
	if _, err := t.tx.Exec(`DELETE FROM office_record_columns WHERE record_type = ?`, name); err != nil {
		return fmt.Errorf("store: record type %s: %w", name, err)
	}
	if _, err := t.tx.Exec(`DELETE FROM office_record_types WHERE name = ?`, name); err != nil {
		return fmt.Errorf("store: record type %s: %w", name, err)
	}
	return nil
}

// OfficeRecordTypeByName returns one record type with its columns, and whether it exists.
func (t *Tx) OfficeRecordTypeByName(name string) (OfficeRecordType, bool, error) {
	all, err := t.officeRecordTypes(`WHERE name = ?`, name)
	if err != nil || len(all) == 0 {
		return OfficeRecordType{}, false, err
	}
	return all[0], true, nil
}

// OfficeRecordTypes lists every record type with its columns, in name order.
func (t *Tx) OfficeRecordTypes() ([]OfficeRecordType, error) { return t.officeRecordTypes("") }

func (t *Tx) officeRecordTypes(where string, args ...any) ([]OfficeRecordType, error) {
	rows, err := t.tx.Query(`SELECT name, description, copied_from, created_by, created_at FROM office_record_types `+where+` ORDER BY name`, args...)
	if err != nil {
		return nil, fmt.Errorf("store: record types: %w", err)
	}
	var out []OfficeRecordType
	for rows.Next() {
		var rt OfficeRecordType
		if err := rows.Scan(&rt.Name, &rt.Description, &rt.CopiedFrom, &rt.CreatedBy, &rt.CreatedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("store: record types: %w", err)
		}
		out = append(out, rt)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for i := range out {
		crow, err := t.tx.Query(`SELECT position, name, label, type, is_key, required FROM office_record_columns WHERE record_type = ? ORDER BY position`, out[i].Name)
		if err != nil {
			return nil, fmt.Errorf("store: record type %s columns: %w", out[i].Name, err)
		}
		for crow.Next() {
			var c OfficeColumn
			var key, req int
			if err := crow.Scan(&c.Position, &c.Name, &c.Label, &c.Type, &key, &req); err != nil {
				crow.Close()
				return nil, fmt.Errorf("store: record type %s columns: %w", out[i].Name, err)
			}
			c.Key, c.Required = key == 1, req == 1
			out[i].Columns = append(out[i].Columns, c)
		}
		if err := crow.Err(); err != nil {
			crow.Close()
			return nil, err
		}
		crow.Close()
	}
	return out, nil
}

// DocUsage says what reads a document: its formats (by the structured document_type column, not
// the free-text label), and, through its record type, stored rows and rules.
func (t *Tx) DocUsage(code string) (Usage, error) {
	d, ok, err := t.DocTypeByCode(code)
	if err != nil {
		return Usage{}, err
	}
	var u Usage
	if ok && d.RecordType != "" {
		if u, err = t.RecordTypeUsage(d.RecordType); err != nil {
			return Usage{}, err
		}
	}
	rows, err := t.tx.Query(`SELECT DISTINCT format_id FROM format_versions WHERE document_type = ? ORDER BY format_id`, code)
	if err != nil {
		return Usage{}, fmt.Errorf("store: document %s usage: %w", code, err)
	}
	defer rows.Close()
	u.Formats = nil
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return Usage{}, fmt.Errorf("store: document %s usage: %w", code, err)
		}
		u.Formats = append(u.Formats, id)
	}
	return u, rows.Err()
}

// DocFormatFileTypes lists the file types ("json", "xlsx", "csv") of the formats that read a
// document, so a file type a format still uses cannot be taken off the document. Sorted.
func (t *Tx) DocFormatFileTypes(code string) ([]string, error) {
	rows, err := t.tx.Query(`SELECT DISTINCT file_type FROM format_versions WHERE document_type = ? ORDER BY file_type`, code)
	if err != nil {
		return nil, fmt.Errorf("store: document %s file types: %w", code, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			return nil, fmt.Errorf("store: document %s file types: %w", code, err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// RecordTypeUsage says what reads a record type: stored rows, and every rule with a version that
// names it as an input (any status). Formats are reported by DocUsage, through the document.
func (t *Tx) RecordTypeUsage(name string) (Usage, error) {
	var u Usage
	if err := t.tx.QueryRow(`SELECT COUNT(*) FROM records WHERE record_type = ?`, name).Scan(&u.Records); err != nil {
		return Usage{}, fmt.Errorf("store: record type %s usage: %w", name, err)
	}
	// The Rule Builder writes an input inline (a: { record_type: "name", label: "..." }); a hand-made rule may write it on a
	// line of its own. Both are read; a longer name that starts with this one is not.
	line := regexp.MustCompile(`(?m)\brecord_type:[ \t]*["']?` + regexp.QuoteMeta(name) + `["']?[ \t]*(?:[,}#]|$)`)
	rows, err := t.tx.Query(`SELECT v.id, v.rule_id, v.body_yaml, COALESCE(s.active_version_id, 0), COALESCE(s.status, '')
		FROM rule_versions v LEFT JOIN rule_state s ON s.rule_id = v.rule_id ORDER BY v.rule_id, v.revision`)
	if err != nil {
		return Usage{}, fmt.Errorf("store: record type %s usage: %w", name, err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, active int64
		var ruleID, body, status string
		if err := rows.Scan(&id, &ruleID, &body, &active, &status); err != nil {
			return Usage{}, fmt.Errorf("store: record type %s usage: %w", name, err)
		}
		if !line.MatchString(body) {
			continue
		}
		if !slices.Contains(u.Rules, ruleID) {
			u.Rules = append(u.Rules, ruleID)
		}
		if id == active && status == "active" && !slices.Contains(u.ActiveRules, ruleID) {
			u.ActiveRules = append(u.ActiveRules, ruleID)
		}
	}
	return u, rows.Err()
}

// PutOfficeSnapshot keeps the canonical office vocabulary a run used, once per hash. Writing the
// same hash again with the same body does nothing; a different body under a hash is an error.
func (t *Tx) PutOfficeSnapshot(hash, body string) error {
	if hash == "" {
		return errors.New("store: an empty office vocabulary needs no snapshot")
	}
	have, ok, err := t.OfficeSnapshot(hash)
	if err != nil {
		return err
	}
	if ok {
		if have != body {
			return fmt.Errorf("store: office vocabulary snapshot %s already holds different content", hash)
		}
		return nil
	}
	if _, err := t.tx.Exec(`INSERT INTO office_vocab_snapshots (hash, body_json) VALUES (?, ?)`, hash, body); err != nil {
		return fmt.Errorf("store: office vocabulary snapshot: %w", err)
	}
	return nil
}

// OfficeSnapshot returns the body kept under a hash, and whether there is one.
func (t *Tx) OfficeSnapshot(hash string) (string, bool, error) {
	var body string
	err := t.tx.QueryRow(`SELECT body_json FROM office_vocab_snapshots WHERE hash = ?`, hash).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("store: office vocabulary snapshot: %w", err)
	}
	return body, true, nil
}

// RunOfficeVocab returns the office vocabulary a run used: the hash it recorded and the canonical text
// kept under that hash. Both are empty for a run made with no office documents (or before the Document
// Master), which needs no snapshot.
func (t *Tx) RunOfficeVocab(runID int64) (hash, body string, err error) {
	err = t.tx.QueryRow(`SELECT r.office_vocab_hash, COALESCE(s.body_json, '') FROM runs r
		LEFT JOIN office_vocab_snapshots s ON s.hash = r.office_vocab_hash WHERE r.id = ?`, runID).Scan(&hash, &body)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("store: run %d office vocabulary: %w", runID, err)
	}
	return hash, body, nil
}
