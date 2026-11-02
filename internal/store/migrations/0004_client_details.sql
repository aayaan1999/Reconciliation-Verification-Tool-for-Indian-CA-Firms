-- Migration 0004 (SPEC-004 part 8): the client master. name stays the legal name; the PAN
-- stays a keyed hash plus its last four characters. A GSTIN contains the PAN, so a client
-- with a GSTIN has its PAN readable here: the database is to be encrypted at rest
-- (Module 14, ADR 0002).
ALTER TABLE clients ADD COLUMN trade_name TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN constitution TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN date_of_incorporation TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN cin TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN tan TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN address_line1 TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN address_line2 TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN city TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN state TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN pin TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN contact_person TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN email TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN phone TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN accounting_software TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN notes TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN updated_at TEXT NOT NULL DEFAULT '';

-- The client's GST registrations. One GSTIN belongs to one client.
CREATE TABLE client_gstins (
    client_id         INTEGER NOT NULL REFERENCES clients (id),
    gstin             TEXT NOT NULL UNIQUE,
    registration_type TEXT NOT NULL,
    is_primary        INTEGER NOT NULL CHECK (is_primary IN (0, 1)),
    position          INTEGER NOT NULL,
    PRIMARY KEY (client_id, gstin)
);
