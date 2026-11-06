package store

import (
	"cmp"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/domain"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/money"
)

// An EngagementRef is a stored engagement.
type EngagementRef struct {
	ID      int64
	TaxYear string
}

// EngagementsByKey returns the engagements named key, one per tax year, sorted by tax year.
func (t *Tx) EngagementsByKey(key string) ([]EngagementRef, error) {
	rows, err := t.tx.Query(`SELECT id, tax_year FROM engagements WHERE cli_key = ? ORDER BY tax_year`, key)
	if err != nil {
		return nil, fmt.Errorf("store: engagements: %w", err)
	}
	defer rows.Close()
	var out []EngagementRef
	for rows.Next() {
		var e EngagementRef
		if err := rows.Scan(&e.ID, &e.TaxYear); err != nil {
			return nil, fmt.Errorf("store: engagements: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Engagement returns the ID of the engagement named key for taxYear, creating it if needed.
func (t *Tx) Engagement(key, taxYear string) (int64, error) {
	if _, err := t.tx.Exec(`INSERT INTO engagements (cli_key, tax_year) VALUES (?, ?) ON CONFLICT DO NOTHING`, key, taxYear); err != nil {
		return 0, fmt.Errorf("store: engagement: %w", err)
	}
	var id int64
	if err := t.tx.QueryRow(`SELECT id FROM engagements WHERE cli_key = ? AND tax_year = ?`, key, taxYear).Scan(&id); err != nil {
		return 0, fmt.Errorf("store: engagement: %w", err)
	}
	return id, nil
}

// A Slot is a document's place in an engagement (IN-11): files with the same slot are
// drafts of one document.
type Slot struct {
	DocumentType, Period, Scope, Part string
}

// An Upload is one file read in a run, with what it gave.
type Upload struct {
	EngagementID   int64
	Slot           Slot
	FileName       string
	SHA256         string
	ProfileID      string
	ProfileVersion string
	UploadedBy     string
	// ProfileVersionID is the stored format version the file was read with (SPEC-016); 0 when the
	// profile has none stored.
	ProfileVersionID int64
	// ReadWith names the dollar rate a file with dollar amounts was read with (SPEC-019), "" for a
	// file with none. The same file read with another rate is a new draft.
	ReadWith   string
	OK         bool   // false when the validation report has errors; Facts is then empty
	ReportJSON []byte // the validation report (IN-14)
	Facts      []domain.Fact
	Records    []domain.Record
}

// A DocVersion is one stored document version.
type DocVersion struct {
	ID             int64
	Slot           Slot
	DraftNo        int64
	FileName       string
	SHA256         string
	ProfileID      string
	ProfileVersion string
	ReportJSON     []byte // the validation report stored with the version (set by CurrentView)
}

// PutUpload stores u as the next draft of its slot, with its facts. The same file read with
// the same profile as the slot's latest draft is not a new draft: that version is returned
// and nothing is written (isNew is false). Any other upload, including an older draft
// uploaded again, becomes the new latest draft, so the current view is always what was
// uploaded last.
func (t *Tx) PutUpload(u Upload) (dv DocVersion, isNew bool, err error) {
	s := u.Slot
	if _, err := t.tx.Exec(`INSERT INTO documents (engagement_id, document_type, period, scope, part) VALUES (?, ?, ?, ?, ?) ON CONFLICT DO NOTHING`,
		u.EngagementID, s.DocumentType, s.Period, s.Scope, s.Part); err != nil {
		return dv, false, fmt.Errorf("store: document: %w", err)
	}
	var docID int64
	if err := t.tx.QueryRow(`SELECT id FROM documents WHERE engagement_id = ? AND document_type = ? AND period = ? AND scope = ? AND part = ?`,
		u.EngagementID, s.DocumentType, s.Period, s.Scope, s.Part).Scan(&docID); err != nil {
		return dv, false, fmt.Errorf("store: document: %w", err)
	}
	dv = DocVersion{Slot: s, FileName: u.FileName, SHA256: u.SHA256, ProfileID: u.ProfileID, ProfileVersion: u.ProfileVersion}
	var latest DocVersion
	var latestSHA, latestReadWith string
	err = t.tx.QueryRow(`SELECT id, draft_no, file_name, file_sha256, profile_id, profile_version, read_with FROM document_versions
		WHERE document_id = ? ORDER BY draft_no DESC LIMIT 1`, docID).
		Scan(&latest.ID, &latest.DraftNo, &latest.FileName, &latestSHA, &latest.ProfileID, &latest.ProfileVersion, &latestReadWith)
	switch {
	case err == nil && latestSHA == u.SHA256 && latest.ProfileID == u.ProfileID && latest.ProfileVersion == u.ProfileVersion && latestReadWith == u.ReadWith:
		latest.Slot, latest.SHA256 = s, latestSHA
		return latest, false, nil
	case err != nil && !errors.Is(err, sql.ErrNoRows):
		return dv, false, fmt.Errorf("store: document version: %w", err)
	}
	dv.DraftNo = latest.DraftNo + 1
	status := "failed"
	if u.OK {
		status = "ok"
	}
	res, err := t.tx.Exec(`INSERT INTO document_versions (document_id, draft_no, file_sha256, file_name, profile_id, profile_version, uploaded_by, extraction_status, validation_report_json, profile_version_id, read_with)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, 0), ?)`,
		docID, dv.DraftNo, u.SHA256, u.FileName, u.ProfileID, u.ProfileVersion, u.UploadedBy, status, string(u.ReportJSON), u.ProfileVersionID, u.ReadWith)
	if err != nil {
		return dv, false, fmt.Errorf("store: document version: %w", err)
	}
	if dv.ID, err = res.LastInsertId(); err != nil {
		return dv, false, err
	}
	for _, f := range u.Facts {
		if err := t.putFact(dv.ID, u.EngagementID, f); err != nil {
			return dv, false, err
		}
	}
	for _, rec := range u.Records {
		if err := t.putRecord(dv.ID, u.EngagementID, rec); err != nil {
			return dv, false, err
		}
	}
	return dv, true, nil
}

func (t *Tx) putRecord(dvID, engagementID int64, r domain.Record) error {
	amounts := map[string]int64{}
	for k, v := range r.Amounts {
		amounts[k] = int64(v)
	}
	a, err := json.Marshal(amounts)
	if err != nil {
		return err
	}
	f, err := json.Marshal(r.Fields)
	if err != nil {
		return err
	}
	if _, err := t.tx.Exec(`INSERT INTO records (document_version_id, engagement_id, record_type, period, scope, match_key, amounts_json, fields_json, locator_kind, locator_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, dvID, engagementID, r.RecordType, r.Period, r.Scope, r.MatchKey, string(a), string(f), r.Locator.Kind, r.Locator.Path); err != nil {
		return fmt.Errorf("store: record %s %s: %w", r.RecordType, r.MatchKey, err)
	}
	return nil
}

// CurrentRecords returns the rows of the engagement's current view (the latest draft of
// every document slot), sorted by record type, match key and place.
func (t *Tx) CurrentRecords(engagementID int64) ([]domain.Record, error) {
	docs, _, err := t.CurrentView(engagementID)
	if err != nil {
		return nil, err
	}
	return t.RecordsOf(engagementID, docs)
}

// RecordsOf returns the rows of the document versions docs, sorted as CurrentRecords.
func (t *Tx) RecordsOf(engagementID int64, docs []DocVersion) ([]domain.Record, error) {
	var out []domain.Record
	for _, dv := range docs {
		rows, err := t.tx.Query(`SELECT record_type, period, scope, match_key, amounts_json, fields_json, locator_kind, locator_path
			FROM records WHERE document_version_id = ? ORDER BY id`, dv.ID)
		if err != nil {
			return nil, fmt.Errorf("store: records: %w", err)
		}
		for rows.Next() {
			r := domain.Record{EngagementID: engagementID, Fields: map[string]string{}, Amounts: map[string]money.Paise{}}
			var amounts, fields string
			if err := rows.Scan(&r.RecordType, &r.Period, &r.Scope, &r.MatchKey, &amounts, &fields, &r.Locator.Kind, &r.Locator.Path); err != nil {
				rows.Close()
				return nil, fmt.Errorf("store: records: %w", err)
			}
			var a map[string]int64
			if err := json.Unmarshal([]byte(amounts), &a); err != nil {
				rows.Close()
				return nil, fmt.Errorf("store: record amounts: %w", err)
			}
			for k, v := range a {
				r.Amounts[k] = money.Paise(v)
			}
			if err := json.Unmarshal([]byte(fields), &r.Fields); err != nil {
				rows.Close()
				return nil, fmt.Errorf("store: record fields: %w", err)
			}
			r.Locator.DocumentVersionID, r.Locator.FileName = dv.ID, dv.FileName
			out = append(out, r)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	slices.SortStableFunc(out, func(a, b domain.Record) int {
		return cmp.Or(cmp.Compare(a.RecordType, b.RecordType), cmp.Compare(a.MatchKey, b.MatchKey),
			cmp.Compare(a.Locator.FileName, b.Locator.FileName), cmp.Compare(a.Locator.Path, b.Locator.Path))
	})
	return out, nil
}

func (t *Tx) putFact(dvID, engagementID int64, f domain.Fact) error {
	v := f.Value
	if !v.Kind.Valid() {
		return fmt.Errorf("store: fact %s has no valid kind", f.Name)
	}
	boolInt := 0
	if v.Bool {
		boolInt = 1
	}
	_, err := t.tx.Exec(`INSERT INTO facts (document_version_id, engagement_id, name, period, scope, kind,
		money_paise, int_value, text_value, date_value, bool_value, rate_bp, locator_kind, locator_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		dvID, engagementID, f.Name, f.Period, f.Scope, v.Kind.String(),
		int64(v.Money), v.Int, v.Text, v.Date, boolInt, v.RateBP, f.Locator.Kind, f.Locator.Path)
	if err != nil {
		return fmt.Errorf("store: fact %s: %w", f.Name, err)
	}
	return nil
}

// CurrentView returns the engagement's current view (technical spec §A7 step 3): the
// latest draft of every document slot, sorted by slot, and the facts of those drafts,
// sorted by name, period, scope and locator path.
func (t *Tx) CurrentView(engagementID int64) ([]DocVersion, []domain.Fact, error) {
	return t.view(engagementID, `SELECT v.id, d.document_type, d.period, d.scope, d.part, v.draft_no, v.file_name, v.file_sha256, v.profile_id, v.profile_version, v.validation_report_json
		FROM documents d JOIN document_versions v ON v.document_id = d.id
		WHERE d.engagement_id = ? AND v.draft_no = (SELECT MAX(draft_no) FROM document_versions WHERE document_id = d.id)
		ORDER BY d.document_type, d.period, d.scope, d.part`, engagementID)
}

// RunView returns the view a run of the engagement read (its snapshot in run_documents),
// sorted as CurrentView, so an earlier comparison shows the drafts it used (SPEC-009).
func (t *Tx) RunView(engagementID, runID int64) ([]DocVersion, []domain.Fact, error) {
	return t.view(engagementID, `SELECT v.id, d.document_type, d.period, d.scope, d.part, v.draft_no, v.file_name, v.file_sha256, v.profile_id, v.profile_version, v.validation_report_json
		FROM run_documents r JOIN document_versions v ON v.id = r.document_version_id JOIN documents d ON d.id = v.document_id
		WHERE d.engagement_id = ? AND r.run_id = ?
		ORDER BY d.document_type, d.period, d.scope, d.part`, engagementID, runID)
}

// view reads the document versions query selects, and their facts.
func (t *Tx) view(engagementID int64, query string, args ...any) ([]DocVersion, []domain.Fact, error) {
	rows, err := t.tx.Query(query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("store: current view: %w", err)
	}
	var docs []DocVersion
	for rows.Next() {
		var dv DocVersion
		var report string
		if err := rows.Scan(&dv.ID, &dv.Slot.DocumentType, &dv.Slot.Period, &dv.Slot.Scope, &dv.Slot.Part,
			&dv.DraftNo, &dv.FileName, &dv.SHA256, &dv.ProfileID, &dv.ProfileVersion, &report); err != nil {
			rows.Close()
			return nil, nil, fmt.Errorf("store: current view: %w", err)
		}
		dv.ReportJSON = []byte(report)
		docs = append(docs, dv)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("store: current view: %w", err)
	}

	var facts []domain.Fact
	for _, dv := range docs {
		fs, err := t.facts(engagementID, dv)
		if err != nil {
			return nil, nil, err
		}
		facts = append(facts, fs...)
	}
	sortFacts(facts)
	return docs, facts, nil
}

func (t *Tx) facts(engagementID int64, dv DocVersion) ([]domain.Fact, error) {
	rows, err := t.tx.Query(`SELECT name, period, scope, kind, money_paise, int_value, text_value, date_value, bool_value, rate_bp, locator_kind, locator_path
		FROM facts WHERE document_version_id = ? ORDER BY id`, dv.ID)
	if err != nil {
		return nil, fmt.Errorf("store: facts: %w", err)
	}
	defer rows.Close()
	var out []domain.Fact
	for rows.Next() {
		var (
			f       domain.Fact
			kind    string
			paise   int64
			boolInt int
		)
		if err := rows.Scan(&f.Name, &f.Period, &f.Scope, &kind, &paise, &f.Int, &f.Text, &f.Date, &boolInt, &f.RateBP,
			&f.Locator.Kind, &f.Locator.Path); err != nil {
			return nil, fmt.Errorf("store: facts: %w", err)
		}
		k, ok := domain.ParseValueKind(kind)
		if !ok {
			return nil, fmt.Errorf("store: fact %s has unknown kind %q", f.Name, kind)
		}
		f.Kind, f.Money, f.Bool = k, money.Paise(paise), boolInt == 1
		f.EngagementID = engagementID
		f.Locator.DocumentVersionID, f.Locator.FileName = dv.ID, dv.FileName
		out = append(out, f)
	}
	return out, rows.Err()
}
