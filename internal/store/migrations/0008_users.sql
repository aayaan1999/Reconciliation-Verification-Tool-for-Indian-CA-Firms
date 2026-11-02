-- Migration 0008 (SPEC-012): users, sessions and the sign-in settings (technical spec §B1.1, §B12.2).

-- One row per person who uses the tool. A user is disabled, never deleted: every audit entry
-- names the login name, so the row must stay (the trigger below refuses a delete).
-- password_hash holds only an argon2id hash written by internal/auth; no plain-text password
-- is ever stored. The parameters are inside the hash, so they can be raised later.
CREATE TABLE users (
    id                   INTEGER PRIMARY KEY,
    login_name           TEXT NOT NULL,                  -- unique, ignoring capitals (see the index below)
    display_name         TEXT NOT NULL,
    role                 TEXT NOT NULL CHECK (role IN ('partner', 'manager', 'staff', 'read_only')),
    password_hash        TEXT NOT NULL CHECK (substr(password_hash, 1, 10) = '$argon2id$'),
    must_change_password INTEGER NOT NULL DEFAULT 1 CHECK (must_change_password IN (0, 1)),
    active               INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    failed_logins        INTEGER NOT NULL DEFAULT 0 CHECK (failed_logins >= 0),
    locked_until         TEXT NOT NULL DEFAULT '',       -- UTC, RFC 3339; '' when not locked
    last_login_at        TEXT NOT NULL DEFAULT '',       -- UTC, RFC 3339; '' when never signed in
    created_at           TEXT NOT NULL,                  -- UTC, RFC 3339
    updated_at           TEXT NOT NULL                   -- UTC, RFC 3339: the last change to the row
);
CREATE UNIQUE INDEX users_login_name ON users (login_name COLLATE NOCASE);
CREATE TRIGGER users_no_delete BEFORE DELETE ON users
BEGIN SELECT RAISE(ABORT, 'users are disabled, never deleted; set active to 0 instead'); END;

-- One row per signed-in session. The cookie holds a random token; only its SHA-256 is kept,
-- so a copy of the database cannot be used to sign in. A session ends at its idle timeout
-- (30 minutes after last_seen_at) or its absolute timeout (expires_at, 10 hours after
-- created_at), whichever comes first. Sessions may be deleted: they are not records of
-- the firm's work. The audit log keeps the sign-in and sign-out.
CREATE TABLE sessions (
    token_hash   TEXT PRIMARY KEY,                       -- hex SHA-256 of the cookie's value
    user_id      INTEGER NOT NULL REFERENCES users (id),
    created_at   TEXT NOT NULL,                          -- UTC, RFC 3339
    last_seen_at TEXT NOT NULL,                          -- UTC, RFC 3339
    expires_at   TEXT NOT NULL                           -- UTC, RFC 3339: created_at + 10 hours
);
CREATE INDEX sessions_user ON sessions (user_id);

-- The lockout settings, one row. The Partner changes them on the Admin tab. A wrong password
-- on the max_failed_logins-th consecutive attempt locks the account for lock_minutes.
CREATE TABLE auth_settings (
    id                INTEGER PRIMARY KEY CHECK (id = 1),
    max_failed_logins INTEGER NOT NULL CHECK (max_failed_logins BETWEEN 1 AND 100),
    lock_minutes      INTEGER NOT NULL CHECK (lock_minutes BETWEEN 1 AND 10080),
    updated_at        TEXT NOT NULL                       -- UTC, RFC 3339; '' until first changed
);
INSERT INTO auth_settings (id, max_failed_logins, lock_minutes, updated_at) VALUES (1, 5, 15, '');
