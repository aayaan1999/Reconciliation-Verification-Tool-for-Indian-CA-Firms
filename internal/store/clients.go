package store

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// A Client is one client of the firm. The full PAN is never kept: only a keyed hash, for
// matching uploads to the client, and the last four characters, for display (§B1.3).
type Client struct {
	ID       int64
	Name     string
	PANHash  string
	PANLast4 string
}

// panKey returns this database's key for hashing PANs, creating it on first use. A keyed
// hash (HMAC-SHA-256) means a stolen hash cannot be looked up in a table of all PANs
// without the key.
func (t *Tx) panKey() ([]byte, error) {
	var key []byte
	err := t.tx.QueryRow(`SELECT value FROM settings WHERE name = 'pan_hmac_key'`).Scan(&key)
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("store: PAN key: %w", err)
	}
	key = make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if _, err := t.tx.Exec(`INSERT INTO settings (name, value) VALUES ('pan_hmac_key', ?)`, key); err != nil {
		return nil, fmt.Errorf("store: PAN key: %w", err)
	}
	return key, nil
}

// PANHash returns the keyed hash of a PAN, ignoring case and surrounding spaces.
func (t *Tx) PANHash(pan string) (string, error) {
	key, err := t.panKey()
	if err != nil {
		return "", err
	}
	m := hmac.New(sha256.New, key)
	m.Write([]byte(strings.ToUpper(strings.TrimSpace(pan))))
	return hex.EncodeToString(m.Sum(nil)), nil
}

// AddClient stores a new client. A second client with the same PAN is a ConflictError.
func (t *Tx) AddClient(name, pan string) (Client, error) {
	h, err := t.PANHash(pan)
	if err != nil {
		return Client{}, err
	}
	var existing int64
	err = t.tx.QueryRow(`SELECT id FROM clients WHERE pan_hash = ?`, h).Scan(&existing)
	switch {
	case err == nil:
		return Client{}, &ConflictError{Msg: fmt.Sprintf("a client with this PAN already exists (client %d)", existing)}
	case !errors.Is(err, sql.ErrNoRows):
		return Client{}, fmt.Errorf("store: client: %w", err)
	}
	p := strings.ToUpper(strings.TrimSpace(pan))
	c := Client{Name: strings.TrimSpace(name), PANHash: h, PANLast4: p[max(0, len(p)-4):]}
	res, err := t.tx.Exec(`INSERT INTO clients (name, pan_hash, pan_last4) VALUES (?, ?, ?)`, c.Name, c.PANHash, c.PANLast4)
	if err != nil {
		return Client{}, fmt.Errorf("store: client: %w", err)
	}
	c.ID, err = res.LastInsertId()
	return c, err
}

// Clients returns every active client, sorted by name then ID.
func (t *Tx) Clients() ([]Client, error) {
	rows, err := t.tx.Query(`SELECT id, name, pan_hash, pan_last4 FROM clients WHERE status = 'active' ORDER BY name, id`)
	if err != nil {
		return nil, fmt.Errorf("store: clients: %w", err)
	}
	defer rows.Close()
	var out []Client
	for rows.Next() {
		var c Client
		if err := rows.Scan(&c.ID, &c.Name, &c.PANHash, &c.PANLast4); err != nil {
			return nil, fmt.Errorf("store: clients: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ClientByID returns the client with this ID.
func (t *Tx) ClientByID(id int64) (Client, bool, error) {
	var c Client
	err := t.tx.QueryRow(`SELECT id, name, pan_hash, pan_last4 FROM clients WHERE id = ? AND status = 'active'`, id).
		Scan(&c.ID, &c.Name, &c.PANHash, &c.PANLast4)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Client{}, false, nil
	case err != nil:
		return Client{}, false, fmt.Errorf("store: client: %w", err)
	}
	return c, true, nil
}

// ClientEngagement returns the client's engagement for the tax year, creating it if needed.
func (t *Tx) ClientEngagement(clientID int64, taxYear string) (int64, error) {
	id, err := t.Engagement("client:"+strconv.FormatInt(clientID, 10), taxYear)
	if err != nil {
		return 0, err
	}
	if _, err := t.tx.Exec(`UPDATE engagements SET client_id = ? WHERE id = ?`, clientID, id); err != nil {
		return 0, fmt.Errorf("store: engagement: %w", err)
	}
	return id, nil
}

// FindClientEngagement returns the client's engagement for the tax year without creating one.
func (t *Tx) FindClientEngagement(clientID int64, taxYear string) (int64, bool, error) {
	var id int64
	err := t.tx.QueryRow(`SELECT id FROM engagements WHERE cli_key = ? AND tax_year = ?`,
		"client:"+strconv.FormatInt(clientID, 10), taxYear).Scan(&id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return 0, false, nil
	case err != nil:
		return 0, false, fmt.Errorf("store: engagement: %w", err)
	}
	return id, true, nil
}

// SetRunReport keeps the run's report (canonical JSON).
func (t *Tx) SetRunReport(runID int64, report []byte) error {
	if _, err := t.tx.Exec(`UPDATE runs SET report_json = ? WHERE id = ?`, string(report), runID); err != nil {
		return fmt.Errorf("store: run report: %w", err)
	}
	return nil
}

// A RunRecord is a finished run's date and report.
type RunRecord struct {
	ID         int64
	Comparison string // the type of check run; "" for every rule
	FinishedOn string // YYYY-MM-DD, the local date
	Report     []byte // canonical JSON; empty for runs made before migration 0002
}

// LatestRun returns the engagement's most recent finished run.
func (t *Tx) LatestRun(engagementID int64) (RunRecord, bool, error) {
	var r RunRecord
	var report, finished string
	// The date is shown to staff, so it is the office computer's local date.
	err := t.tx.QueryRow(`SELECT id, date(finished_at, 'localtime'), report_json FROM runs WHERE engagement_id = ? AND finished_at <> '' ORDER BY id DESC LIMIT 1`,
		engagementID).Scan(&r.ID, &finished, &report)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return RunRecord{}, false, nil
	case err != nil:
		return RunRecord{}, false, fmt.Errorf("store: latest run: %w", err)
	}
	r.FinishedOn = finished
	r.Report = []byte(report)
	return r, true, nil
}

// LatestRuns returns the engagement's most recent finished run of each type of check
// (SPEC-004 part 9), sorted by type, so running one type never hides another's result.
func (t *Tx) LatestRuns(engagementID int64) ([]RunRecord, error) {
	// The date is shown to staff, so it is the office computer's local date.
	rows, err := t.tx.Query(`SELECT r.id, r.comparison_id, date(r.finished_at, 'localtime'), r.report_json FROM runs r
		WHERE r.engagement_id = ? AND r.finished_at <> '' AND r.id = (
			SELECT MAX(l.id) FROM runs l WHERE l.engagement_id = r.engagement_id AND l.comparison_id = r.comparison_id AND l.finished_at <> '')
		ORDER BY r.comparison_id`, engagementID)
	if err != nil {
		return nil, fmt.Errorf("store: latest runs: %w", err)
	}
	defer rows.Close()
	var out []RunRecord
	for rows.Next() {
		var r RunRecord
		var report string
		if err := rows.Scan(&r.ID, &r.Comparison, &r.FinishedOn, &report); err != nil {
			return nil, fmt.Errorf("store: latest runs: %w", err)
		}
		r.Report = []byte(report)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: latest runs: %w", err)
	}
	return out, nil
}

// A Comparison is one finished run of a type of check, numbered from 1 in run order within
// its engagement and type (SPEC-009). Runs are never deleted, so a number never changes.
type Comparison struct {
	RunRecord
	No             int
	FinishedAt     string // YYYY-MM-DD HH:MM, local time
	IncludeTesting bool
}

// Comparisons returns every finished run of the type of check in the engagement, oldest
// first.
func (t *Tx) Comparisons(engagementID int64, comparisonID string) ([]Comparison, error) {
	// The time is shown to staff, so it is the office computer's local time.
	rows, err := t.tx.Query(`SELECT id, strftime('%Y-%m-%d %H:%M', finished_at, 'localtime'), include_testing, report_json
		FROM runs WHERE engagement_id = ? AND comparison_id = ? AND finished_at <> '' ORDER BY id`, engagementID, comparisonID)
	if err != nil {
		return nil, fmt.Errorf("store: comparisons: %w", err)
	}
	defer rows.Close()
	var out []Comparison
	for rows.Next() {
		c := Comparison{No: len(out) + 1, RunRecord: RunRecord{Comparison: comparisonID}}
		var report string
		if err := rows.Scan(&c.ID, &c.FinishedAt, &c.IncludeTesting, &report); err != nil {
			return nil, fmt.Errorf("store: comparisons: %w", err)
		}
		c.Report = []byte(report)
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: comparisons: %w", err)
	}
	return out, nil
}

// ClientRunTaxYears returns the tax years in which the client has a finished run, oldest
// first, so its Client file can open on the latest year with a result.
func (t *Tx) ClientRunTaxYears(clientID int64) ([]string, error) {
	rows, err := t.tx.Query(`SELECT DISTINCT e.tax_year FROM engagements e JOIN runs r ON r.engagement_id = e.id
		WHERE e.client_id = ? AND r.finished_at <> '' ORDER BY e.tax_year`, clientID)
	if err != nil {
		return nil, fmt.Errorf("store: client run tax years: %w", err)
	}
	defer rows.Close()
	years := []string{}
	for rows.Next() {
		var y string
		if err := rows.Scan(&y); err != nil {
			return nil, fmt.Errorf("store: client run tax years: %w", err)
		}
		years = append(years, y)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: client run tax years: %w", err)
	}
	return years, nil
}
