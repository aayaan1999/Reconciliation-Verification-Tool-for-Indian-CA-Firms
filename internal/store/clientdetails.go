package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/client"
)

// AddClientDetails stores a new client with all its details (SPEC-004 part 8). d must have
// been checked with client.Validate; d.PAN is required and is kept only as a keyed hash
// and its last four characters. A PAN or GSTIN already used by another client is a
// ConflictError.
func (t *Tx) AddClientDetails(d client.Details) (Client, error) {
	c, err := t.AddClient(d.LegalName, d.PAN)
	if err != nil {
		return Client{}, err
	}
	if err := t.writeDetails(c.ID, d); err != nil {
		return Client{}, err
	}
	return c, nil
}

// UpdateClient replaces the details of client id. When d.PAN is "", the PAN is kept;
// otherwise the new PAN is checked against the other clients like a new client's.
func (t *Tx) UpdateClient(id int64, d client.Details) (Client, error) {
	c, found, err := t.ClientByID(id)
	if err != nil {
		return Client{}, err
	}
	if !found {
		return Client{}, &ConflictError{Msg: fmt.Sprintf("there is no client %d", id)}
	}
	if d.PAN != "" {
		h, err := t.PANHash(d.PAN)
		if err != nil {
			return Client{}, err
		}
		if h != c.PANHash {
			var other int64
			err := t.tx.QueryRow(`SELECT id FROM clients WHERE pan_hash = ? AND id <> ?`, h, id).Scan(&other)
			switch {
			case err == nil:
				return Client{}, &ConflictError{Msg: fmt.Sprintf("a client with this PAN already exists (client %d)", other)}
			case !errors.Is(err, sql.ErrNoRows):
				return Client{}, fmt.Errorf("store: client: %w", err)
			}
			c.PANHash, c.PANLast4 = h, d.PAN[len(d.PAN)-4:]
		}
	}
	c.Name = strings.TrimSpace(d.LegalName)
	if _, err := t.tx.Exec(`UPDATE clients SET name = ?, pan_hash = ?, pan_last4 = ? WHERE id = ?`, c.Name, c.PANHash, c.PANLast4, id); err != nil {
		return Client{}, fmt.Errorf("store: client: %w", err)
	}
	if err := t.writeDetails(id, d); err != nil {
		return Client{}, err
	}
	return c, nil
}

// GSTINPANMatches reports whether the PAN inside a GSTIN (characters 3 to 12) is the PAN
// whose hash is panHash, without the full PAN being stored.
func (t *Tx) GSTINPANMatches(gstin, panHash string) (bool, error) {
	if len(gstin) != 15 {
		return false, nil
	}
	h, err := t.PANHash(gstin[2:12])
	return h == panHash, err
}

func (t *Tx) writeDetails(id int64, d client.Details) error {
	a := d.Address
	if _, err := t.tx.Exec(`UPDATE clients SET trade_name = ?, constitution = ?, date_of_incorporation = ?, cin = ?, tan = ?,
		address_line1 = ?, address_line2 = ?, city = ?, state = ?, pin = ?, contact_person = ?, email = ?, phone = ?,
		accounting_software = ?, notes = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = ?`,
		d.TradeName, d.Constitution, d.DateOfIncorporation, d.CIN, d.TAN, a.Line1, a.Line2, a.City, a.State, a.PIN,
		d.ContactPerson, d.Email, d.Phone, d.AccountingSoftware, d.Notes, id); err != nil {
		return fmt.Errorf("store: client details: %w", err)
	}
	if _, err := t.tx.Exec(`DELETE FROM client_gstins WHERE client_id = ?`, id); err != nil {
		return fmt.Errorf("store: client GSTINs: %w", err)
	}
	for i, g := range d.GSTINs {
		var other int64
		err := t.tx.QueryRow(`SELECT client_id FROM client_gstins WHERE gstin = ?`, g.GSTIN).Scan(&other)
		switch {
		case err == nil:
			return &ConflictError{Msg: fmt.Sprintf("GSTIN %s already belongs to client %d", g.GSTIN, other)}
		case !errors.Is(err, sql.ErrNoRows):
			return fmt.Errorf("store: client GSTINs: %w", err)
		}
		primary := 0
		if g.Primary {
			primary = 1
		}
		if _, err := t.tx.Exec(`INSERT INTO client_gstins (client_id, gstin, registration_type, is_primary, position) VALUES (?, ?, ?, ?, ?)`,
			id, g.GSTIN, g.RegistrationType, primary, i); err != nil {
			return fmt.Errorf("store: client GSTINs: %w", err)
		}
	}
	return nil
}

// ClientDetails returns all the details of client id. The full PAN is not stored, so it is
// never in the result: only PANLast4.
func (t *Tx) ClientDetails(id int64) (client.Details, bool, error) {
	var d client.Details
	a := &d.Address
	err := t.tx.QueryRow(`SELECT name, pan_last4, trade_name, constitution, date_of_incorporation, cin, tan,
		address_line1, address_line2, city, state, pin, contact_person, email, phone, accounting_software, notes
		FROM clients WHERE id = ?`, id).Scan(&d.LegalName, &d.PANLast4, &d.TradeName, &d.Constitution,
		&d.DateOfIncorporation, &d.CIN, &d.TAN, &a.Line1, &a.Line2, &a.City, &a.State, &a.PIN, &d.ContactPerson, &d.Email,
		&d.Phone, &d.AccountingSoftware, &d.Notes)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return d, false, nil
	case err != nil:
		return d, false, fmt.Errorf("store: client details: %w", err)
	}
	rows, err := t.tx.Query(`SELECT gstin, registration_type, is_primary FROM client_gstins WHERE client_id = ? ORDER BY position`, id)
	if err != nil {
		return d, false, fmt.Errorf("store: client GSTINs: %w", err)
	}
	defer rows.Close()
	d.GSTINs = []client.GSTIN{}
	for rows.Next() {
		var g client.GSTIN
		var primary int
		if err := rows.Scan(&g.GSTIN, &g.RegistrationType, &primary); err != nil {
			return d, false, fmt.Errorf("store: client GSTINs: %w", err)
		}
		g.Primary = primary == 1
		d.GSTINs = append(d.GSTINs, g)
	}
	return d, true, rows.Err()
}
