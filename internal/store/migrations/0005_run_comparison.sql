-- Migration 0005 (SPEC-004 part 9): each run records the type of check it ran, so a
-- client keeps its latest result per type. '' is a run of every rule (no type chosen).
ALTER TABLE runs ADD COLUMN comparison_id TEXT NOT NULL DEFAULT '';
CREATE INDEX runs_by_comparison ON runs (engagement_id, comparison_id, id);
