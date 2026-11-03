-- Migration 0011 (SPEC-016): document formats and parameters, kept in the database with every
-- version, as rules are (rule_versions, rule_state). A format says how one kind of file is read
-- (a profile); a parameter is a named figure the rules read, with its aliases. Nothing here is
-- ever overwritten or deleted: a change adds a version, and a format or parameter is switched
-- off with a reason (RU-41, RU-44). Every text column is NOT NULL, with '' for "none".

-- Every version of every format, standard and office. body_yaml is the profile as written.
CREATE TABLE format_versions (
    id         INTEGER PRIMARY KEY,
    format_id  TEXT NOT NULL,
    revision   INTEGER NOT NULL,           -- 1, 2, ... per format_id
    version    TEXT NOT NULL,              -- 'v1', 'v2', ...
    name       TEXT NOT NULL,
    source     TEXT NOT NULL CHECK (source IN ('builtin', 'office')),
    file_type  TEXT NOT NULL CHECK (file_type IN ('json', 'xlsx', 'csv')),
    document   TEXT NOT NULL,              -- which parameters it fills, for example 'GSTR-1'
    body_yaml  TEXT NOT NULL,
    sha256     TEXT NOT NULL,
    note       TEXT NOT NULL DEFAULT '',   -- what changed, or why it was switched back
    status     TEXT NOT NULL CHECK (status IN ('draft', 'testing', 'active', 'disabled')),
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE (format_id, revision)
);

-- The current state of each format: which version is in use and whether it may be used.
CREATE TABLE format_state (
    format_id         TEXT PRIMARY KEY,
    active_version_id INTEGER NOT NULL REFERENCES format_versions (id),
    status            TEXT NOT NULL CHECK (status IN ('draft', 'testing', 'active', 'disabled')),
    published_by      TEXT NOT NULL DEFAULT '',
    published_at      TEXT NOT NULL DEFAULT '',
    disabled_reason   TEXT NOT NULL DEFAULT ''
);

-- Every version of every parameter, with its aliases inside body_yaml. A parameter is a fact (or
-- record type) of the vocabulary: the standard ones come from content/vocabulary, an office one
-- is added here. Profiles may only write these names and rules may only read them (FV-03).
CREATE TABLE parameter_versions (
    id           INTEGER PRIMARY KEY,
    parameter_id TEXT NOT NULL,            -- the key, for example 'gst.taxable_value'
    revision     INTEGER NOT NULL,
    version      TEXT NOT NULL,
    name         TEXT NOT NULL,
    source       TEXT NOT NULL CHECK (source IN ('builtin', 'office')),
    param_type   TEXT NOT NULL CHECK (param_type IN ('money', 'int', 'text', 'date', 'bool', 'rate', 'record')), -- the vocabulary's kinds; 'record' is a record type
    document     TEXT NOT NULL,
    required     INTEGER NOT NULL CHECK (required IN (0, 1)),
    body_yaml    TEXT NOT NULL,
    sha256       TEXT NOT NULL,
    created_by   TEXT NOT NULL,
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE (parameter_id, revision)
);

CREATE TABLE parameter_state (
    parameter_id      TEXT PRIMARY KEY,
    active_version_id INTEGER NOT NULL REFERENCES parameter_versions (id),
    status            TEXT NOT NULL CHECK (status IN ('active', 'disabled')),
    disabled_reason   TEXT NOT NULL DEFAULT ''
);

-- A sample file the admin chose to keep, to re-read when a format is published. Only the hash
-- and the name are kept unless the admin chooses to keep the bytes; the bytes may hold client
-- figures, so the screen warns that a sample must be synthetic or masked.
CREATE TABLE format_samples (
    id          INTEGER PRIMARY KEY,
    format_id   TEXT NOT NULL,
    file_sha256 TEXT NOT NULL,
    file_name   TEXT NOT NULL,
    size_bytes  INTEGER NOT NULL CHECK (size_bytes >= 0),
    content     BLOB,                      -- NULL when only the hash was kept
    kept_by     TEXT NOT NULL,
    kept_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE (format_id, file_sha256)
);

-- A version is never changed or removed. The state tables point at the version in use.
CREATE TRIGGER format_versions_no_update BEFORE UPDATE ON format_versions
BEGIN SELECT RAISE(ABORT, 'format versions are never changed; add a new version instead'); END;
CREATE TRIGGER format_versions_no_delete BEFORE DELETE ON format_versions
BEGIN SELECT RAISE(ABORT, 'format versions are never deleted; disable the format instead'); END;
CREATE TRIGGER parameter_versions_no_update BEFORE UPDATE ON parameter_versions
BEGIN SELECT RAISE(ABORT, 'parameter versions are never changed; add a new version instead'); END;
CREATE TRIGGER parameter_versions_no_delete BEFORE DELETE ON parameter_versions
BEGIN SELECT RAISE(ABORT, 'parameter versions are never deleted; disable the parameter instead'); END;

-- A run points at the exact format version it read with, so an earlier comparison opens exactly
-- as it was reported (SPEC-009). NULL for rows made before this migration.
ALTER TABLE document_versions ADD COLUMN profile_version_id INTEGER REFERENCES format_versions (id);
ALTER TABLE run_documents     ADD COLUMN profile_version_id INTEGER REFERENCES format_versions (id);
