-- Migration 0012 (SPEC-016): editing a format. An edit is saved as a draft version; publishing
-- writes its profile files and makes it the version in use. A version is still never changed, with
-- one exception: publishing it records when and by whom, once.
ALTER TABLE format_versions ADD COLUMN mapping_json    TEXT NOT NULL DEFAULT ''; -- the mapping the screen edits ('' for a standard format)
ALTER TABLE format_versions ADD COLUMN profile_version TEXT NOT NULL DEFAULT ''; -- the profile's own version, "1", "2", ...
ALTER TABLE format_versions ADD COLUMN published_at    TEXT NOT NULL DEFAULT ''; -- '' while a draft
ALTER TABLE format_versions ADD COLUMN published_by    TEXT NOT NULL DEFAULT '';

DROP TRIGGER format_versions_no_update;
CREATE TRIGGER format_versions_no_update BEFORE UPDATE OF format_id, revision, version, name, source, file_type, document,
    body_yaml, sha256, note, status, created_by, created_at, mapping_json, profile_version ON format_versions
BEGIN SELECT RAISE(ABORT, 'format versions are never changed; add a new version instead'); END;
-- Publishing is recorded once.
CREATE TRIGGER format_versions_publish_once BEFORE UPDATE OF published_at, published_by ON format_versions
WHEN OLD.published_at <> ''
BEGIN SELECT RAISE(ABORT, 'a format version is published once'); END;
