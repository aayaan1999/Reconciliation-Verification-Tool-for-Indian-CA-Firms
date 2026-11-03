-- Migration 0010 (SPEC-004 part 10): disabling a client.
-- A disabled client is stored with status 'archived', which migration 0002 already allows, so
-- the table is not rebuilt. These columns say who changed the status last, when and why; the
-- full history is in audit_log (client.disable, client.enable).
ALTER TABLE clients ADD COLUMN status_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN status_by     TEXT NOT NULL DEFAULT '';
ALTER TABLE clients ADD COLUMN status_at     TEXT NOT NULL DEFAULT ''; -- UTC, RFC 3339; '' when never changed

-- A client is disabled, never deleted: its documents, runs and decisions stay with it.
CREATE TRIGGER clients_no_delete BEFORE DELETE ON clients
BEGIN SELECT RAISE(ABORT, 'clients are disabled, never deleted; set status to archived instead'); END;
