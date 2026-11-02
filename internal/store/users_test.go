package store

import (
	"strings"
	"testing"
)

const testHash = "$argon2id$v=19$m=64,t=1,p=1$c2FsdHNhbHQ$aGFzaGhhc2hoYXNoaGFzaGhhc2hoYXNoaGFzaGhhc2g"

func newUser(login string) User {
	return User{LoginName: login, DisplayName: "Test " + login, Role: "staff", PasswordHash: testHash,
		MustChangePassword: true, Active: true, CreatedAt: "2026-10-30T09:00:00Z", UpdatedAt: "2026-10-30T09:00:00Z"}
}

// AC-1: the users table exists after migration 0008, and the password column accepts only
// argon2id hashes (the database checks the prefix).
func TestUsersTableAfterMigration(t *testing.T) {
	db, _ := open(t)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if n, err := tx.UserCount(); err != nil || n != 0 {
		t.Fatalf("UserCount = %d, %v; want 0", n, err)
	}
	bad := newUser("plain")
	bad.PasswordHash = "plain-text-password"
	if _, err := tx.InsertUser(bad); err == nil {
		t.Fatal("a plain-text password was stored; the table must refuse anything but an argon2id hash")
	}
	id, err := tx.InsertUser(newUser("Asha.K"))
	if err != nil {
		t.Fatal(err)
	}
	u, ok, err := tx.UserByID(id)
	if err != nil || !ok {
		t.Fatalf("UserByID: %v, %v", ok, err)
	}
	if !strings.HasPrefix(u.PasswordHash, "$argon2id$") {
		t.Errorf("stored password %q does not have the argon2id prefix", u.PasswordHash)
	}
}

// Login names are unique ignoring capitals, and a lookup ignores capitals too.
func TestLoginNameUniqueAndCaseInsensitive(t *testing.T) {
	db, _ := open(t)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.InsertUser(newUser("Asha.K")); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.InsertUser(newUser("asha.k")); err == nil {
		t.Error("a second login name differing only in capitals was accepted")
	}
	u, ok, err := tx.UserByLogin("ASHA.K")
	if err != nil || !ok || u.LoginName != "Asha.K" {
		t.Errorf("UserByLogin(ASHA.K) = %+v, %v, %v", u, ok, err)
	}
}

// AC-15 at the database level: a user row is never deleted; the trigger refuses it.
func TestUsersCannotBeDeleted(t *testing.T) {
	db, _ := open(t)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	id, err := tx.InsertUser(newUser("Ravi"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.tx.Exec(`DELETE FROM users WHERE id = ?`, id); err == nil {
		t.Fatal("a user row was deleted; users are disabled, never deleted")
	} else if !strings.Contains(err.Error(), "disabled, never deleted") {
		t.Errorf("delete refused with an unexpected message: %v", err)
	}
	if n, _ := tx.UserCount(); n != 1 {
		t.Errorf("UserCount after refused delete = %d, want 1", n)
	}
}

func TestSessionsAndAuthSettings(t *testing.T) {
	db, _ := open(t)
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	id, err := tx.InsertUser(newUser("Meera"))
	if err != nil {
		t.Fatal(err)
	}
	s := Session{TokenHash: "h1", UserID: id, CreatedAt: "2026-10-30T09:00:00Z",
		LastSeenAt: "2026-10-30T09:00:00Z", ExpiresAt: "2026-10-30T19:00:00Z"}
	if err := tx.InsertSession(s); err != nil {
		t.Fatal(err)
	}
	got, ok, err := tx.SessionByToken("h1")
	if err != nil || !ok || got != s {
		t.Fatalf("SessionByToken = %+v, %v, %v", got, ok, err)
	}
	if err := tx.TouchSession("h1", "2026-10-30T09:10:00Z"); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := tx.SessionByToken("h1"); got.LastSeenAt != "2026-10-30T09:10:00Z" {
		t.Errorf("TouchSession did not move last_seen_at: %q", got.LastSeenAt)
	}
	if err := tx.DeleteUserSessions(id); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := tx.SessionByToken("h1"); ok {
		t.Error("DeleteUserSessions left a session behind")
	}
	maxFailed, lock, err := tx.AuthSettings()
	if err != nil || maxFailed != 5 || lock != 15 {
		t.Fatalf("default AuthSettings = %d, %d, %v; want 5, 15", maxFailed, lock, err)
	}
	if err := tx.SetAuthSettings(3, 30, "2026-10-30T09:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if maxFailed, lock, _ := tx.AuthSettings(); maxFailed != 3 || lock != 30 {
		t.Errorf("AuthSettings after change = %d, %d", maxFailed, lock)
	}
}
