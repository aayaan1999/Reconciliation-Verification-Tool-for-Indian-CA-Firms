-- Migration 0006 (SPEC-006 part 3): decisions on findings, and the Worklist.

-- One row per decision taken on a finding (Fix, Not an issue, Ask client), kept against the
-- finding's stable ID (technical spec §B7.1) for its engagement. Rows are only ever added:
-- the latest row of a finding is its decision, the earlier ones its history.
CREATE TABLE finding_decisions (
    id                INTEGER PRIMARY KEY,
    engagement_id     INTEGER NOT NULL REFERENCES engagements (id),
    finding_id        TEXT NOT NULL,
    rule_id           TEXT NOT NULL,
    decision          TEXT NOT NULL CHECK (decision IN ('fix', 'not_an_issue', 'ask_client')),
    note              TEXT NOT NULL,
    decided_by        TEXT NOT NULL,
    decided_at        TEXT NOT NULL, -- UTC, RFC 3339
    value_fingerprint TEXT NOT NULL  -- the finding's values when it was decided
);
CREATE INDEX finding_decisions_by_finding ON finding_decisions (engagement_id, finding_id, id);
CREATE TRIGGER finding_decisions_no_update BEFORE UPDATE ON finding_decisions
BEGIN SELECT RAISE(ABORT, 'finding decisions are kept as written; add a new decision instead'); END;
CREATE TRIGGER finding_decisions_no_delete BEFORE DELETE ON finding_decisions
BEGIN SELECT RAISE(ABORT, 'finding decisions are kept as written; add a new decision instead'); END;

-- The classification staff give a Worklist item (a new, vanished or changed ledger head).
-- The items themselves are worked out from the ledgers on each run; a classification is
-- kept by engagement, kind and ledger head, so it survives later runs. Rows are only ever
-- added: the latest is the classification, the earlier ones its history.
CREATE TABLE worklist_classifications (
    id             INTEGER PRIMARY KEY,
    engagement_id  INTEGER NOT NULL REFERENCES engagements (id),
    kind           TEXT NOT NULL,
    ledger         TEXT NOT NULL,
    classification TEXT NOT NULL,
    classified_by  TEXT NOT NULL,
    classified_at  TEXT NOT NULL -- UTC, RFC 3339
);
CREATE INDEX worklist_classifications_by_item ON worklist_classifications (engagement_id, kind, ledger, id);
CREATE TRIGGER worklist_classifications_no_update BEFORE UPDATE ON worklist_classifications
BEGIN SELECT RAISE(ABORT, 'classifications are kept as written; add a new one instead'); END;
CREATE TRIGGER worklist_classifications_no_delete BEFORE DELETE ON worklist_classifications
BEGIN SELECT RAISE(ABORT, 'classifications are kept as written; add a new one instead'); END;

-- The Worklist items of a run, as the comparison of this year's and last year's ledgers gave
-- them (one JSON list per run), so the Worklist screen lists them without recomputing.
ALTER TABLE runs ADD COLUMN worklist_json TEXT NOT NULL DEFAULT '';
