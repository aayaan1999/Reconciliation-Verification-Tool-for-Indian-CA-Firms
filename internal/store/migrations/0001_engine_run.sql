-- Migration 0001 (SPEC-003): what one engine run needs. Columns follow technical spec
-- §B1.1. Money is INTEGER paise in columns named *_paise. Every text column is NOT NULL,
-- with '' for "none", so a query never has to tell NULL from ''.

-- An engagement is one client's tax year. Until clients and logins arrive (Module 16), the
-- command line names its engagement by a key: the input folder of the run.
CREATE TABLE engagements (
    id       INTEGER PRIMARY KEY,
    cli_key  TEXT NOT NULL,
    tax_year TEXT NOT NULL,
    UNIQUE (cli_key, tax_year)
);

-- Every version of every rule ever loaded. A changed file adds a revision; none is ever
-- overwritten or deleted (RU-53).
CREATE TABLE rule_versions (
    id         INTEGER PRIMARY KEY,
    rule_id    TEXT NOT NULL,
    revision   INTEGER NOT NULL,         -- 1, 2, ... per rule_id
    version    TEXT NOT NULL,            -- the version written in the file
    source     TEXT NOT NULL CHECK (source IN ('rulebook', 'builtin', 'office')),
    schema     TEXT NOT NULL,            -- 'recon.rule/1' (can be evaluated) or 'rulestore/1.0' (descriptive only)
    tax_years  TEXT NOT NULL,            -- comma-separated, '' when the file names none
    file_path  TEXT NOT NULL,
    body_yaml  TEXT NOT NULL,
    sha256     TEXT NOT NULL,
    pack_id    TEXT NOT NULL DEFAULT '',
    created_by TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE (rule_id, revision)
);

-- The current state of each rule: which version is in use and whether it may run.
CREATE TABLE rule_state (
    rule_id           TEXT PRIMARY KEY,
    active_version_id INTEGER NOT NULL REFERENCES rule_versions (id),
    status            TEXT NOT NULL CHECK (status IN ('draft', 'testing', 'active', 'disabled')),
    approved_by       TEXT NOT NULL DEFAULT '',
    approved_at       TEXT NOT NULL DEFAULT '',
    disabled_reason   TEXT NOT NULL DEFAULT ''
);

-- A document slot (IN-11): one document type, period, scope and part of an engagement.
CREATE TABLE documents (
    id            INTEGER PRIMARY KEY,
    engagement_id INTEGER NOT NULL REFERENCES engagements (id),
    document_type TEXT NOT NULL,
    period        TEXT NOT NULL,
    scope         TEXT NOT NULL,
    part          TEXT NOT NULL,
    UNIQUE (engagement_id, document_type, period, scope, part)
);

-- Each file uploaded into a slot is a new draft (IN-12), unless it is the same file, read
-- with the same profile, as the slot's latest draft. An older draft uploaded again becomes
-- the latest, so the current view is always what was uploaded last.
CREATE TABLE document_versions (
    id                     INTEGER PRIMARY KEY,
    document_id            INTEGER NOT NULL REFERENCES documents (id),
    draft_no               INTEGER NOT NULL,
    file_sha256            TEXT NOT NULL,
    file_name              TEXT NOT NULL,
    profile_id             TEXT NOT NULL,
    profile_version        TEXT NOT NULL,
    uploaded_by            TEXT NOT NULL,
    uploaded_at            TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    extraction_status      TEXT NOT NULL CHECK (extraction_status IN ('ok', 'failed')),
    validation_report_json TEXT NOT NULL,
    UNIQUE (document_id, draft_no)
);

-- One figure read from one document version, with where it was found.
CREATE TABLE facts (
    id                  INTEGER PRIMARY KEY,
    document_version_id INTEGER NOT NULL REFERENCES document_versions (id),
    engagement_id       INTEGER NOT NULL REFERENCES engagements (id),
    name                TEXT NOT NULL,
    period              TEXT NOT NULL,
    scope               TEXT NOT NULL,
    kind                TEXT NOT NULL CHECK (kind IN ('money', 'int', 'text', 'date', 'bool', 'rate')),
    money_paise         INTEGER NOT NULL DEFAULT 0,
    int_value           INTEGER NOT NULL DEFAULT 0,
    text_value          TEXT NOT NULL DEFAULT '',
    date_value          TEXT NOT NULL DEFAULT '',
    bool_value          INTEGER NOT NULL DEFAULT 0,
    rate_bp             INTEGER NOT NULL DEFAULT 0,
    locator_kind        TEXT NOT NULL,
    locator_path        TEXT NOT NULL
);
CREATE INDEX facts_by_name ON facts (engagement_id, name, period, scope);
CREATE INDEX facts_by_version ON facts (document_version_id);

-- A run and its snapshot (RU-53): engine and vocabulary versions here, the document and rule
-- versions used in run_documents and run_rules.
CREATE TABLE runs (
    id              INTEGER PRIMARY KEY,
    engagement_id   INTEGER NOT NULL REFERENCES engagements (id),
    tax_year        TEXT NOT NULL,
    engine_version  TEXT NOT NULL,
    vocab_version   INTEGER NOT NULL,
    vocab_hash      TEXT NOT NULL,
    include_testing INTEGER NOT NULL CHECK (include_testing IN (0, 1)),
    started_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    finished_at     TEXT NOT NULL DEFAULT ''
);

CREATE TABLE run_documents (
    run_id              INTEGER NOT NULL REFERENCES runs (id),
    document_version_id INTEGER NOT NULL REFERENCES document_versions (id),
    document_type       TEXT NOT NULL,
    file_sha256         TEXT NOT NULL,
    profile_id          TEXT NOT NULL,
    profile_version     TEXT NOT NULL,
    PRIMARY KEY (run_id, document_version_id)
);

CREATE TABLE run_rules (
    run_id          INTEGER NOT NULL REFERENCES runs (id),
    rule_version_id INTEGER NOT NULL REFERENCES rule_versions (id),
    rule_id         TEXT NOT NULL,
    version         TEXT NOT NULL,
    status          TEXT NOT NULL,           -- the rule's status when the run used it
    PRIMARY KEY (run_id, rule_id)
);

-- One result of one rule for one subject. seq is the position in (rule_id, subject_key)
-- order, so the stored order is the evaluation order of every run.
CREATE TABLE evaluations (
    id               INTEGER PRIMARY KEY,
    run_id           INTEGER NOT NULL REFERENCES runs (id),
    seq              INTEGER NOT NULL,
    rule_id          TEXT NOT NULL,
    rule_version     TEXT NOT NULL,
    subject_key      TEXT NOT NULL,
    status           TEXT NOT NULL CHECK (status IN ('pass', 'fail', 'cannot_check')),
    values_json      TEXT NOT NULL,
    difference_paise INTEGER NOT NULL,
    missing_json     TEXT NOT NULL,
    UNIQUE (run_id, seq)
);
CREATE INDEX evaluations_by_rule ON evaluations (run_id, rule_id);
