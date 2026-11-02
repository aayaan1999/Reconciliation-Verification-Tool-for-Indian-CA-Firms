-- Migration 0007 (SPEC-010): the audit log (technical spec §B1.1 and §B1.3).

-- One row per create, edit, disable, enable and version switch of a rule. Each row carries
-- the hash of the row before it (prev_hash) and its own hash (internal/audit), so a change
-- to any earlier row breaks the chain. The table only ever grows: the triggers refuse
-- updates and deletes, and the audit time is written by the caller, never read by the engine.
CREATE TABLE audit_log (
    seq          INTEGER PRIMARY KEY,
    at           TEXT NOT NULL,       -- UTC, RFC 3339
    actor        TEXT NOT NULL,       -- who did it, as typed by the user
    action       TEXT NOT NULL,       -- for example 'rule.disable'
    object_type  TEXT NOT NULL,       -- for example 'rule'
    object_id    TEXT NOT NULL,       -- for example 'RECON-DEP-002 v1-office.1'
    details_json TEXT NOT NULL,       -- canonical JSON object, keys sorted (the reason is here)
    prev_hash    TEXT NOT NULL,
    hash         TEXT NOT NULL
);
CREATE TRIGGER audit_log_no_update BEFORE UPDATE ON audit_log
BEGIN SELECT RAISE(ABORT, 'the audit log is append-only; write a new entry instead'); END;
CREATE TRIGGER audit_log_no_delete BEFORE DELETE ON audit_log
BEGIN SELECT RAISE(ABORT, 'the audit log is append-only; write a new entry instead'); END;
