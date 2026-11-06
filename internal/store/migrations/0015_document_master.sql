-- Migration 0015 (SPEC-022): the Document Master. The Partner adds the kinds of document the tool
-- can read, and the record structure (columns) of each, without a code change. The base vocabulary
-- in content/vocabulary stays as it is; these tables are the office layer on top of it.

-- One row per document type. Built-in documents are rows with origin 'builtin', filled from the
-- YAML on every start (the YAML stays the definition; active and category are kept). record_type is
-- '' while an office document is still a Draft (its structure is not complete).
CREATE TABLE document_types (
    code           TEXT PRIMARY KEY,
    origin         TEXT NOT NULL CHECK (origin IN ('builtin', 'office')),
    name           TEXT NOT NULL,
    category       TEXT NOT NULL,
    description    TEXT NOT NULL,
    applicable_for TEXT NOT NULL,                  -- JSON array of 'json', 'xlsx', 'csv', in that order
    record_type    TEXT NOT NULL DEFAULT '',
    active         INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    created_by     TEXT NOT NULL,
    created_at     TEXT NOT NULL,
    updated_by     TEXT NOT NULL,
    updated_at     TEXT NOT NULL
);
CREATE TRIGGER document_types_builtin_no_delete BEFORE DELETE ON document_types
WHEN OLD.origin = 'builtin'
BEGIN SELECT RAISE(ABORT, 'a built-in document cannot be deleted'); END;

-- The Partner's record types. copied_from is for information only: after a copy the two record
-- types are independent and no query follows it.
CREATE TABLE office_record_types (
    name        TEXT PRIMARY KEY,
    description TEXT NOT NULL,
    copied_from TEXT NOT NULL DEFAULT '',
    created_by  TEXT NOT NULL,
    created_at  TEXT NOT NULL
);

-- The columns of a record type, in the order the wizard shows them.
CREATE TABLE office_record_columns (
    record_type TEXT NOT NULL REFERENCES office_record_types(name),
    position    INTEGER NOT NULL,
    name        TEXT NOT NULL,                     -- the key in files, lower snake case
    label       TEXT NOT NULL,                     -- shown in the wizard and the Rule Builder
    type        TEXT NOT NULL CHECK (type IN ('text', 'date', 'amount')),
    is_key      INTEGER NOT NULL DEFAULT 0 CHECK (is_key IN (0, 1)),
    required    INTEGER NOT NULL DEFAULT 0 CHECK (required IN (0, 1)),
    PRIMARY KEY (record_type, name),
    UNIQUE (record_type, position)
);

-- The office vocabulary a run used, as the canonical JSON that OfficeHash() hashes. Written once
-- per distinct hash and never changed, so an earlier comparison can always be shown as reported.
CREATE TABLE office_vocab_snapshots (
    hash      TEXT PRIMARY KEY,
    body_json TEXT NOT NULL
);
CREATE TRIGGER office_vocab_snapshots_no_update BEFORE UPDATE ON office_vocab_snapshots
BEGIN SELECT RAISE(ABORT, 'office vocabulary snapshots are never changed'); END;
CREATE TRIGGER office_vocab_snapshots_no_delete BEFORE DELETE ON office_vocab_snapshots
BEGIN SELECT RAISE(ABORT, 'office vocabulary snapshots are never deleted'); END;

-- A run's vocabulary identity is the pair (base hash, office hash). '' means no office definitions
-- (also for every run made before this migration), and then no snapshot is needed.
ALTER TABLE runs ADD COLUMN office_vocab_hash TEXT NOT NULL DEFAULT '';

-- The structured link from a format to its document, so "is this document used?" does not depend
-- on the free-text document label. Filled at save time for new versions; existing rows are filled
-- once after this migration by a Go step that reads the document_type line of body_yaml. The
-- format_versions_no_update trigger does not list this column, so that fill is allowed.
ALTER TABLE format_versions ADD COLUMN document_type TEXT NOT NULL DEFAULT '';
