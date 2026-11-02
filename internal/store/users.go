package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// A User is one row of the users table (migration 0008). Times are UTC, RFC 3339, and ""
// when not set. PasswordHash is an argon2id hash (internal/auth); it is never shown.
type User struct {
	ID                 int64
	LoginName          string
	DisplayName        string
	Role               string
	PasswordHash       string
	MustChangePassword bool
	Active             bool
	FailedLogins       int
	LockedUntil        string
	LastLoginAt        string
	CreatedAt          string
	UpdatedAt          string
}

// userColumns is the column list every user read uses, so the scans stay in step with it.
const userColumns = `id, login_name, display_name, role, password_hash, must_change_password,
	active, failed_logins, locked_until, last_login_at, created_at, updated_at`

// scanUser reads one row written with userColumns.
func scanUser(sc interface{ Scan(...any) error }) (User, error) {
	var u User
	var must, active int
	err := sc.Scan(&u.ID, &u.LoginName, &u.DisplayName, &u.Role, &u.PasswordHash, &must, &active,
		&u.FailedLogins, &u.LockedUntil, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	u.MustChangePassword, u.Active = must == 1, active == 1
	return u, err
}

// UserCount returns how many users exist, disabled ones included.
func (t *Tx) UserCount() (int, error) {
	var n int
	if err := t.tx.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: users: %w", err)
	}
	return n, nil
}

// CountActivePartners returns how many active users hold the Partner role.
func (t *Tx) CountActivePartners() (int, error) {
	var n int
	if err := t.tx.QueryRow(`SELECT COUNT(*) FROM users WHERE role = 'partner' AND active = 1`).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: users: %w", err)
	}
	return n, nil
}

// UserByID returns the user with that ID, and false when there is none.
func (t *Tx) UserByID(id int64) (User, bool, error) {
	u, err := scanUser(t.tx.QueryRow(`SELECT `+userColumns+` FROM users WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, fmt.Errorf("store: users: %w", err)
	}
	return u, true, nil
}

// UserByLogin returns the user whose login name matches, ignoring capitals. A login name is
// unique in any case (the index in migration 0008).
func (t *Tx) UserByLogin(loginName string) (User, bool, error) {
	u, err := scanUser(t.tx.QueryRow(`SELECT `+userColumns+` FROM users WHERE login_name = ? COLLATE NOCASE`, loginName))
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, fmt.Errorf("store: users: %w", err)
	}
	return u, true, nil
}

// ListUsers returns every user in a fixed order: by login name, ignoring capitals.
func (t *Tx) ListUsers() ([]User, error) {
	rows, err := t.tx.Query(`SELECT ` + userColumns + ` FROM users ORDER BY login_name COLLATE NOCASE, id`)
	if err != nil {
		return nil, fmt.Errorf("store: users: %w", err)
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("store: users: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// InsertUser adds a user and returns its ID. The caller has checked the login name is free.
func (t *Tx) InsertUser(u User) (int64, error) {
	res, err := t.tx.Exec(`INSERT INTO users (login_name, display_name, role, password_hash, must_change_password,
		active, failed_logins, locked_until, last_login_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, '', '', ?, ?)`,
		u.LoginName, u.DisplayName, u.Role, u.PasswordHash, boolInt(u.MustChangePassword), boolInt(u.Active),
		u.CreatedAt, u.UpdatedAt)
	if err != nil {
		return 0, fmt.Errorf("store: add user: %w", err)
	}
	return res.LastInsertId()
}

// SetUserPassword stores a new password hash and whether it must be changed at next sign-in.
func (t *Tx) SetUserPassword(id int64, hash string, mustChange bool, at string) error {
	return t.execUser(`UPDATE users SET password_hash = ?, must_change_password = ?, updated_at = ? WHERE id = ?`,
		hash, boolInt(mustChange), at, id)
}

// SetUserRole stores a user's role.
func (t *Tx) SetUserRole(id int64, role, at string) error {
	return t.execUser(`UPDATE users SET role = ?, updated_at = ? WHERE id = ?`, role, at, id)
}

// SetUserActive sets whether a user may sign in.
func (t *Tx) SetUserActive(id int64, active bool, at string) error {
	return t.execUser(`UPDATE users SET active = ?, updated_at = ? WHERE id = ?`, boolInt(active), at, id)
}

// SetUserLoginState stores the failed-login count, the lock end ("" for none) and the last
// sign-in time after a sign-in attempt.
func (t *Tx) SetUserLoginState(id int64, failed int, lockedUntil, lastLoginAt, at string) error {
	return t.execUser(`UPDATE users SET failed_logins = ?, locked_until = ?, last_login_at = ?, updated_at = ? WHERE id = ?`,
		failed, lockedUntil, lastLoginAt, at, id)
}

func (t *Tx) execUser(query string, args ...any) error {
	res, err := t.tx.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("store: users: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return fmt.Errorf("store: users: %d rows changed, want 1", n)
	}
	return nil
}

// A Session is one row of the sessions table. TokenHash is the hex SHA-256 of the cookie.
type Session struct {
	TokenHash  string
	UserID     int64
	CreatedAt  string
	LastSeenAt string
	ExpiresAt  string
}

// InsertSession records a new session.
func (t *Tx) InsertSession(s Session) error {
	if _, err := t.tx.Exec(`INSERT INTO sessions (token_hash, user_id, created_at, last_seen_at, expires_at)
		VALUES (?, ?, ?, ?, ?)`, s.TokenHash, s.UserID, s.CreatedAt, s.LastSeenAt, s.ExpiresAt); err != nil {
		return fmt.Errorf("store: sessions: %w", err)
	}
	return nil
}

// SessionByToken returns the session of a token hash, and false when there is none.
func (t *Tx) SessionByToken(tokenHash string) (Session, bool, error) {
	var s Session
	err := t.tx.QueryRow(`SELECT token_hash, user_id, created_at, last_seen_at, expires_at
		FROM sessions WHERE token_hash = ?`, tokenHash).
		Scan(&s.TokenHash, &s.UserID, &s.CreatedAt, &s.LastSeenAt, &s.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, false, nil
	}
	if err != nil {
		return Session{}, false, fmt.Errorf("store: sessions: %w", err)
	}
	return s, true, nil
}

// TouchSession records the time of the latest request in a session.
func (t *Tx) TouchSession(tokenHash, lastSeenAt string) error {
	_, err := t.tx.Exec(`UPDATE sessions SET last_seen_at = ? WHERE token_hash = ?`, lastSeenAt, tokenHash)
	if err != nil {
		return fmt.Errorf("store: sessions: %w", err)
	}
	return nil
}

// DeleteSession ends one session. Ending a session that is already gone does nothing.
func (t *Tx) DeleteSession(tokenHash string) error {
	if _, err := t.tx.Exec(`DELETE FROM sessions WHERE token_hash = ?`, tokenHash); err != nil {
		return fmt.Errorf("store: sessions: %w", err)
	}
	return nil
}

// DeleteUserSessions ends every open session of one user, so a disabled or reset user is
// signed out at once.
func (t *Tx) DeleteUserSessions(userID int64) error {
	if _, err := t.tx.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("store: sessions: %w", err)
	}
	return nil
}

// AuthSettings returns the lockout settings: the number of consecutive failed sign-ins that
// locks an account, and how many minutes the lock lasts.
func (t *Tx) AuthSettings() (maxFailed, lockMinutes int, err error) {
	err = t.tx.QueryRow(`SELECT max_failed_logins, lock_minutes FROM auth_settings WHERE id = 1`).
		Scan(&maxFailed, &lockMinutes)
	if err != nil {
		return 0, 0, fmt.Errorf("store: auth settings: %w", err)
	}
	return maxFailed, lockMinutes, nil
}

// SetAuthSettings stores the lockout settings. The caller has checked their ranges.
func (t *Tx) SetAuthSettings(maxFailed, lockMinutes int, at string) error {
	_, err := t.tx.Exec(`UPDATE auth_settings SET max_failed_logins = ?, lock_minutes = ?, updated_at = ? WHERE id = 1`,
		maxFailed, lockMinutes, at)
	if err != nil {
		return fmt.Errorf("store: auth settings: %w", err)
	}
	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
