-- Migration 0009 (SPEC-010 scope 8): who last set a rule's status by hand.
-- A rule-book rule's status follows governance.enabled each time the rule book is loaded,
-- until a Partner disables or enables it. From then on the person's choice stands, and a
-- later load does not change it back. '' means the status still follows the file.
ALTER TABLE rule_state ADD COLUMN status_set_by TEXT NOT NULL DEFAULT '';
