-- Migration 0002 (SPEC-004 part 7): clients, and runs that keep their report.

-- Settings of this database, such as the key that hashes PANs.
CREATE TABLE settings (
    name  TEXT PRIMARY KEY,
    value BLOB NOT NULL
);

-- A client of the firm (§B1.1). The PAN is kept only as a keyed hash, for matching
-- uploads to the client, and its last four characters, for display (§B1.3).
CREATE TABLE clients (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    pan_hash   TEXT NOT NULL UNIQUE,
    pan_last4  TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- An engagement may belong to a client; 0 is an engagement named by the command line's
-- input folder (SPEC-003).
ALTER TABLE engagements ADD COLUMN client_id INTEGER NOT NULL DEFAULT 0;

-- The run's report as canonical JSON, so the latest result can be shown again without
-- re-running.
ALTER TABLE runs ADD COLUMN report_json TEXT NOT NULL DEFAULT '';
