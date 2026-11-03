-- Migration 0013 (SPEC-016): files that no format recognised. A run that finds no profile for a
-- file notes it here, so the Partner can teach a format for it. Only the name, the hash and the
-- size are kept, never the file's content. A note is closed (taught or dismissed), never deleted.
CREATE TABLE unrecognised_files (
    id            INTEGER PRIMARY KEY,
    file_name     TEXT NOT NULL,
    file_sha256   TEXT NOT NULL,
    size_bytes    INTEGER NOT NULL CHECK (size_bytes >= 0),
    file_type     TEXT NOT NULL,              -- 'json', 'xlsx', 'csv' or '' when not known
    client_id     INTEGER,                    -- NULL for a run with no client
    tax_year      TEXT NOT NULL DEFAULT '',
    noted_by      TEXT NOT NULL,
    noted_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    status        TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'taught', 'dismissed')),
    resolved_by   TEXT NOT NULL DEFAULT '',
    resolved_at   TEXT NOT NULL DEFAULT '',
    resolved_note TEXT NOT NULL DEFAULT ''
);
-- The same file is noted once while it is open.
CREATE UNIQUE INDEX unrecognised_files_open ON unrecognised_files (file_sha256) WHERE status = 'open';
