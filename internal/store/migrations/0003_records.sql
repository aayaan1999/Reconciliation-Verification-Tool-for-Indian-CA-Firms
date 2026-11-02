-- Migration 0003 (SPEC-004 part 8): rows of tabular documents (invoices of a register or a
-- return), each with its source, for row-by-row matching (technical spec §B1.1, records).
CREATE TABLE records (
    id                  INTEGER PRIMARY KEY,
    document_version_id INTEGER NOT NULL REFERENCES document_versions (id),
    engagement_id       INTEGER NOT NULL REFERENCES engagements (id),
    record_type         TEXT NOT NULL,
    period              TEXT NOT NULL,
    scope               TEXT NOT NULL,
    match_key           TEXT NOT NULL,
    amounts_json        TEXT NOT NULL, -- {"column": paise, ...}
    fields_json         TEXT NOT NULL, -- {"column": "text", ...}
    locator_kind        TEXT NOT NULL,
    locator_path        TEXT NOT NULL
);
CREATE INDEX records_by_type ON records (engagement_id, record_type, period, match_key);
CREATE INDEX records_by_version ON records (document_version_id);
