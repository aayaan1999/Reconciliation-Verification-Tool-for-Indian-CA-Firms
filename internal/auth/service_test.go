package auth

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/audit"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
)

// fastParams keep the tests quick. The defaults are checked separately.
var fastParams = Params{MemoryKiB: 64, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32}

// A clock is the fake time the service reads. Tests move it forward; nothing sleeps.
type clock struct{ now time.Time }

func (c *clock) Now() time.Time { return c.now }

func (c *clock) advance(d time.Duration) { c.now = c.now.Add(d) }

const (
	partnerPassword = "partner-pass-2026"
	startPassword   = "starting-pass-2026"
	newPassword     = "chosen-pass-2026"
)

func newFixture(t *testing.T) (*Service, *clock, *store.DB) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	clk := &clock{now: time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)}
	svc, err := newService(db, clk.Now, fastParams)
	if err != nil {
		t.Fatal(err)
	}
	return svc, clk, db
}

// setupPartner creates the first Partner, "priya", with a password she chose.
func setupPartner(t *testing.T, svc *Service) Account {
	t.Helper()
	acc, err := svc.Setup(Setup{LoginName: "priya", DisplayName: "A. Partner", Password: partnerPassword})
	if err != nil {
		t.Fatal(err)
	}
	return acc
}

// signIn returns the session token of a sign-in that must succeed.
func signIn(t *testing.T, svc *Service, name, password string) string {
	t.Helper()
	token, _, err := svc.Login(name, password)
	if err != nil {
		t.Fatalf("sign in as %s: %v", name, err)
	}
	return token
}

func codeOf(err error) string {
	if err == nil {
		return ""
	}
	var r *Refusal
	if errors.As(err, &r) {
		return r.Code
	}
	return "NOT A REFUSAL: " + err.Error()
}

func TestPasswordHashIsArgon2idWithParameters(t *testing.T) {
	h, err := HashPassword(startPassword, DefaultParams)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Errorf("hash %q does not carry the argon2id prefix and default parameters", h)
	}
	ok, err := VerifyPassword(startPassword, h)
	if err != nil || !ok {
		t.Errorf("right password does not verify: %v, %v", ok, err)
	}
	ok, _ = VerifyPassword(startPassword+"x", h)
	if ok {
		t.Error("a wrong password verified")
	}
	other, _ := HashPassword(startPassword, DefaultParams)
	if other == h {
		t.Error("two hashes of one password are equal; the salt is not random")
	}
	for _, bad := range []string{
		"plain-text", "$argon2i$v=19$m=64,t=1,p=1$c2FsdHNhbHQ$aGFzaGhhc2hoYXNoaGFzaA",
		"$argon2id$v=19$m=999999999,t=1,p=1$c2FsdHNhbHQ$aGFzaGhhc2hoYXNoaGFzaA",
	} {
		if _, err := VerifyPassword(startPassword, bad); err == nil {
			t.Errorf("damaged or foreign hash %q was accepted", bad)
		}
	}
}

func TestPasswordPolicy(t *testing.T) {
	if err := CheckPasswordPolicy("short"); err == nil {
		t.Error("a short password was accepted")
	}
	if err := CheckPasswordPolicy(strings.Repeat("a", MaxPasswordLength+1)); err == nil {
		t.Error("an over-long password was accepted")
	}
	if err := CheckPasswordPolicy(startPassword); err != nil {
		t.Errorf("a valid password was refused: %v", err)
	}
}

// AC-6 (matrix part): the role matrix matches technical spec §B12.1 exactly, written here
// again independently of the table in roles.go.
func TestRoleMatrixMatchesSpec(t *testing.T) {
	want := map[Role]map[Permission]bool{
		RolePartner: {PermView: true, PermRun: true, PermDecide: true, PermDeleteFindings: true, PermDraftRules: true,
			PermActivateRules: true, PermDisableRules: true, PermDisableClients: true, PermExport: true, PermManageUsers: true, PermManageFormats: true, PermManageDocuments: true},
		RoleManager: {PermView: true, PermRun: true, PermDecide: true, PermDeleteFindings: true, PermDraftRules: true,
			PermExport: true},
		RoleStaff:    {PermView: true, PermRun: true, PermDecide: true, PermDraftRules: true},
		RoleReadOnly: {PermView: true},
	}
	all := AllRolePermissions()
	for _, r := range Roles {
		for _, p := range all {
			if got := Allowed(r, p); got != want[r][p] {
				t.Errorf("Allowed(%s, %s) = %v, want %v", r, p, got, want[r][p])
			}
		}
	}
	for _, p := range []Permission{PermOpen, PermSignedIn, "unknown"} {
		if Allowed(RolePartner, p) {
			t.Errorf("Allowed(partner, %q) = true; only the role permissions are in the matrix", p)
		}
	}
	if _, err := ParseRole("readonly"); err == nil {
		t.Error("ParseRole accepted readonly; the role is read_only")
	}
}

// AC-9: the first Partner is set up once, with their own password and no forced change; a
// second attempt is refused and the refusal is audited.
func TestSetupCreatesFirstPartnerOnce(t *testing.T) {
	svc, _, _ := newFixture(t)
	if needed, err := svc.SetupNeeded(); err != nil || !needed {
		t.Fatalf("SetupNeeded on an empty database = %v, %v", needed, err)
	}
	acc := setupPartner(t, svc)
	if acc.Role != RolePartner || acc.MustChangePassword || !acc.Active {
		t.Errorf("first Partner = %+v; want an active Partner with no forced change", acc)
	}
	if needed, _ := svc.SetupNeeded(); needed {
		t.Error("SetupNeeded is still true after the first Partner")
	}
	_, err := svc.Setup(Setup{LoginName: "second", DisplayName: "Second", Password: partnerPassword})
	if codeOf(err) != "SETUP_DONE" {
		t.Errorf("second setup: code %q, want SETUP_DONE", codeOf(err))
	}
	entries := mustEntries(t, svc)
	last := entries[len(entries)-1]
	if last.Action != "setup.refused" || last.Actor != ActorNotSignedIn {
		t.Errorf("refused setup audit = %s by %s; want setup.refused by %s", last.Action, last.Actor, ActorNotSignedIn)
	}
	if accounts, _ := svc.Accounts(); len(accounts) != 1 {
		t.Errorf("accounts after refused setup = %d, want 1", len(accounts))
	}
}

// AC-2: a starting password works once, then the user must change it; after the change the
// starting password stops working.
func TestStartingPasswordChangedAtFirstSignIn(t *testing.T) {
	svc, _, db := newFixture(t)
	partner := setupPartner(t, svc)
	if _, err := svc.AddUser(partner, NewUser{LoginName: "asha", DisplayName: "Asha", Role: "staff", Password: startPassword}); err != nil {
		t.Fatal(err)
	}
	token, acc, err := svc.Login("asha", startPassword)
	if err != nil {
		t.Fatalf("sign in with the starting password: %v", err)
	}
	if !acc.MustChangePassword {
		t.Error("a starting password did not mark the account for a change at first sign-in")
	}
	cur, err := svc.Current(token)
	if err != nil || !cur.MustChangePassword {
		t.Errorf("Current after first sign-in = %+v, %v", cur, err)
	}
	if err := svc.ChangeOwnPassword(acc, startPassword, newPassword); err != nil {
		t.Fatalf("change password: %v", err)
	}
	if _, _, err := svc.Login("asha", startPassword); codeOf(err) != "BAD_CREDENTIALS" {
		t.Errorf("starting password after the change: %v", err)
	}
	_, acc2, err := svc.Login("asha", newPassword)
	if err != nil || acc2.MustChangePassword {
		t.Errorf("new password: %+v, %v", acc2, err)
	}
	// The stored value is still only an argon2id hash.
	tx, _ := db.Begin()
	defer tx.Rollback()
	u, _, _ := tx.UserByLogin("asha")
	if !IsArgon2id(u.PasswordHash) || strings.Contains(u.PasswordHash, newPassword) {
		t.Errorf("stored password is not an argon2id hash only: %q", u.PasswordHash)
	}
}

// AC-3: a wrong password gets the same refusal whether the name exists or not.
func TestWrongPasswordSameMessageForUnknownAndKnownName(t *testing.T) {
	svc, _, _ := newFixture(t)
	setupPartner(t, svc)
	_, _, errKnown := svc.Login("priya", "not-the-password")
	_, _, errUnknown := svc.Login("nobody-here", "not-the-password")
	if errKnown == nil || errUnknown == nil {
		t.Fatal("a wrong password was accepted")
	}
	if errKnown.Error() != errUnknown.Error() || codeOf(errKnown) != codeOf(errUnknown) {
		t.Errorf("known %q/%s and unknown %q/%s differ", errKnown, codeOf(errKnown), errUnknown, codeOf(errUnknown))
	}
}

// AC-4: after the configured number of failed sign-ins the account is locked; a Partner
// unlocks it and the user signs in again. The lock also ends when its time runs out.
func TestLockoutAndUnlock(t *testing.T) {
	svc, clk, _ := newFixture(t)
	partner := setupPartner(t, svc)
	if _, err := svc.AddUser(partner, NewUser{LoginName: "asha", DisplayName: "Asha", Role: "staff", Password: startPassword}); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetLockout(partner, Lockout{MaxFailedLogins: 3, LockMinutes: 10}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if _, _, err := svc.Login("asha", "wrong-guess-here"); codeOf(err) != "BAD_CREDENTIALS" {
			t.Fatalf("failure %d: %v", i+1, err)
		}
	}
	if _, _, err := svc.Login("asha", startPassword); codeOf(err) != "ACCOUNT_LOCKED" {
		t.Fatalf("right password while locked: %v, want ACCOUNT_LOCKED", err)
	}
	accounts, _ := svc.Accounts()
	asha := findAccount(t, accounts, "asha")
	if err := svc.Unlock(partner, asha.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Login("asha", startPassword); err != nil {
		t.Errorf("after unlock: %v", err)
	}
	for i := 0; i < 3; i++ {
		svc.Login("asha", "wrong-guess-here")
	}
	clk.advance(11 * time.Minute)
	if _, _, err := svc.Login("asha", startPassword); err != nil {
		t.Errorf("after the lock ran out: %v", err)
	}
}

// AC-5: a role change applies at the user's next request, with no sign-in again.
func TestRoleChangeAppliesAtNextRequest(t *testing.T) {
	svc, _, _ := newFixture(t)
	partner := setupPartner(t, svc)
	if _, err := svc.AddUser(partner, NewUser{LoginName: "ravi", DisplayName: "Ravi", Role: "staff", Password: startPassword}); err != nil {
		t.Fatal(err)
	}
	token := signIn(t, svc, "ravi", startPassword)
	ravi, err := svc.Current(token)
	if err != nil || ravi.Role != RoleStaff || Allowed(ravi.Role, PermExport) {
		t.Fatalf("before the change: %+v, %v", ravi, err)
	}
	if err := svc.SetRole(partner, ravi.ID, RoleManager, "takes on exports"); err != nil {
		t.Fatal(err)
	}
	after, err := svc.Current(token)
	if err != nil || after.Role != RoleManager || !Allowed(after.Role, PermExport) {
		t.Errorf("the next request after the change: %+v, %v", after, err)
	}
}

// AC-8: a Partner cannot demote or disable their own account, and the last active Partner
// cannot be demoted or disabled.
func TestPartnerGuardRails(t *testing.T) {
	svc, _, _ := newFixture(t)
	priya := setupPartner(t, svc)
	if err := svc.SetRole(priya, priya.ID, RoleStaff, "stepping down"); codeOf(err) != "SELF_CHANGE" {
		t.Errorf("self demotion: %v", err)
	}
	if err := svc.SetActive(priya, priya.ID, false, "leaving"); codeOf(err) != "SELF_CHANGE" {
		t.Errorf("self disable: %v", err)
	}
	// Only one active Partner exists. A different acting Partner (not in the database)
	// still may not remove the last one.
	ghost := Account{ID: 9999, LoginName: "ghost", Role: RolePartner, Active: true}
	if err := svc.SetRole(ghost, priya.ID, RoleStaff, "test"); codeOf(err) != "LAST_PARTNER" {
		t.Errorf("demoting the last Partner: %v", err)
	}
	if err := svc.SetActive(ghost, priya.ID, false, "test"); codeOf(err) != "LAST_PARTNER" {
		t.Errorf("disabling the last Partner: %v", err)
	}
	// A second Partner may be disabled by the first, because one active Partner remains.
	if _, err := svc.AddUser(priya, NewUser{LoginName: "dev", DisplayName: "Dev", Role: "partner", Password: startPassword}); err != nil {
		t.Fatal(err)
	}
	accounts, _ := svc.Accounts()
	dev := findAccount(t, accounts, "dev")
	if err := svc.SetActive(priya, dev.ID, false, "temporary"); err != nil {
		t.Errorf("disabling a second Partner: %v", err)
	}
}

// AC-10: a disabled user's open sessions stop working on the next request; a reason is
// required; re-enabling restores the same role (AC-15).
func TestDisableEndsSessionsAndEnableRestoresRole(t *testing.T) {
	svc, _, _ := newFixture(t)
	partner := setupPartner(t, svc)
	if _, err := svc.AddUser(partner, NewUser{LoginName: "meera", DisplayName: "Meera", Role: "manager", Password: startPassword}); err != nil {
		t.Fatal(err)
	}
	token := signIn(t, svc, "meera", startPassword)
	meera := findAccount(t, mustAccounts(t, svc), "meera")
	if err := svc.SetActive(partner, meera.ID, false, ""); codeOf(err) != "REASON_REQUIRED" {
		t.Errorf("disable without a reason: %v", err)
	}
	if err := svc.SetActive(partner, meera.ID, false, "has left the firm"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Current(token); codeOf(err) != "NOT_SIGNED_IN" {
		t.Errorf("open session after disable: %v", err)
	}
	if _, _, err := svc.Login("meera", startPassword); codeOf(err) != "ACCOUNT_DISABLED" {
		t.Errorf("sign in while disabled: %v", err)
	}
	if err := svc.SetActive(partner, meera.ID, true, "returns to the firm"); err != nil {
		t.Fatal(err)
	}
	_, back, err := svc.Login("meera", startPassword)
	if err != nil || back.Role != RoleManager || back.ID != meera.ID {
		t.Errorf("after enable: %+v, %v; want the same user with role manager", back, err)
	}
	// The row and its audit entries remain.
	if _, ok, _ := svc.AccountByLogin("meera"); !ok {
		t.Error("the disabled user's row is gone")
	}
}

// AC-12: the idle timeout (30 minutes since the latest request) and the absolute timeout (10
// hours since sign-in) each end a session. Requests restart the idle timeout only.
func TestSessionIdleAndAbsoluteTimeouts(t *testing.T) {
	svc, clk, _ := newFixture(t)
	partner := setupPartner(t, svc)
	if _, err := svc.AddUser(partner, NewUser{LoginName: "asha", DisplayName: "Asha", Role: "staff", Password: startPassword}); err != nil {
		t.Fatal(err)
	}

	idle := signIn(t, svc, "asha", startPassword)
	clk.advance(29 * time.Minute)
	if _, err := svc.Current(idle); err != nil {
		t.Fatalf("29 minutes idle: %v", err)
	}
	clk.advance(30 * time.Minute)
	if _, err := svc.Current(idle); codeOf(err) != "SESSION_ENDED" {
		t.Errorf("30 minutes idle: %v, want SESSION_ENDED", err)
	}

	clk.now = time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	long := signIn(t, svc, "asha", startPassword)
	for elapsed := time.Duration(0); elapsed < 10*time.Hour-20*time.Minute; elapsed += 20 * time.Minute {
		clk.advance(20 * time.Minute)
		if _, err := svc.Current(long); err != nil {
			t.Fatalf("request at %v after sign-in: %v", elapsed+20*time.Minute, err)
		}
	}
	clk.advance(20 * time.Minute)
	if _, err := svc.Current(long); codeOf(err) != "SESSION_ENDED" {
		t.Errorf("10 hours after sign-in: %v, want SESSION_ENDED", err)
	}
}

// AC-11: every add, role change, reset, disable, enable, unlock, sign-in, failure and sign-out
// writes one entry naming the signed-in Partner, and no password value appears in any entry.
func TestEveryChangeIsAuditedWithTheActor(t *testing.T) {
	svc, _, _ := newFixture(t)
	partner := setupPartner(t, svc)
	secret := "SECRET-START-PASSWORD-7741"
	if _, err := svc.AddUser(partner, NewUser{LoginName: "asha", DisplayName: "Asha", Role: "staff", Password: secret}); err != nil {
		t.Fatal(err)
	}
	asha := findAccount(t, mustAccounts(t, svc), "asha")
	svc.Login("asha", "wrong-guess-here")
	token := signIn(t, svc, "asha", secret)
	if err := svc.SetRole(partner, asha.ID, RoleManager, "more work"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ResetPassword(partner, asha.ID, secret+"-reset"); err != nil {
		t.Fatal(err)
	}
	// The reset ended the earlier session, so the sign-out is of the new one.
	if _, err := svc.Current(token); codeOf(err) != "NOT_SIGNED_IN" {
		t.Errorf("session before the reset: %v, want it ended", err)
	}
	token = signIn(t, svc, "asha", secret+"-reset")
	if err := svc.Logout(token); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetActive(partner, asha.ID, false, "leave"); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetActive(partner, asha.ID, true, "back"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Unlock(partner, asha.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetLockout(partner, Lockout{MaxFailedLogins: 4, LockMinutes: 20}); err != nil {
		t.Fatal(err)
	}
	_ = token

	entries := mustEntries(t, svc)
	if err := audit.Verify(entries); err != nil {
		t.Fatalf("the audit chain is broken: %v", err)
	}
	want := map[string]string{ // action -> actor
		"user.setup": "priya", "user.add": "priya", "login.failure": "asha", "login.success": "asha",
		"user.role_change": "priya", "user.password_reset": "priya", "logout": "asha",
		"user.disable": "priya", "user.enable": "priya", "user.unlock": "priya", "auth.lockout_set": "priya",
	}
	seen := map[string]string{}
	for _, e := range entries {
		seen[e.Action] = e.Actor
	}
	for action, actor := range want {
		if seen[action] != actor {
			t.Errorf("audit action %s: actor %q, want %q", action, seen[action], actor)
		}
	}
	raw, _ := json.Marshal(entries)
	if strings.Contains(string(raw), "SECRET-START-PASSWORD") {
		t.Error("a password value appears in the audit log")
	}
}

// A refused sign-in never writes the typed name as the actor unless that name is a user's.
func TestUnknownNameIsNotWrittenToAudit(t *testing.T) {
	svc, _, _ := newFixture(t)
	setupPartner(t, svc)
	svc.Login("someone-typed-a-password-here", "x")
	entries := mustEntries(t, svc)
	last := entries[len(entries)-1]
	if last.Actor != ActorNoSuchUser || strings.Contains(string(mustJSON(t, last)), "typed-a-password") {
		t.Errorf("unknown name recorded as %+v", last)
	}
}

// Changes that are refused write nothing to the users table.
func TestInvalidInputIsRefused(t *testing.T) {
	svc, _, _ := newFixture(t)
	partner := setupPartner(t, svc)
	for _, in := range []NewUser{
		{LoginName: "a", DisplayName: "A", Role: "staff", Password: startPassword},
		{LoginName: "bad name", DisplayName: "A", Role: "staff", Password: startPassword},
		{LoginName: "okname", DisplayName: "", Role: "staff", Password: startPassword},
		{LoginName: "okname", DisplayName: "A", Role: "readonly", Password: startPassword},
		{LoginName: "okname", DisplayName: "A", Role: "staff", Password: "short"},
	} {
		if _, err := svc.AddUser(partner, in); codeOf(err) != "BAD_INPUT" {
			t.Errorf("AddUser(%+v): %v, want BAD_INPUT", in, err)
		}
	}
	if _, err := svc.AddUser(partner, NewUser{LoginName: "PRIYA", DisplayName: "X", Role: "staff", Password: startPassword}); codeOf(err) != "LOGIN_NAME_TAKEN" {
		t.Errorf("a name differing only in capitals: %v", err)
	}
}

// A user without the manage-users permission cannot change users even if the handler let
// the call through: the service checks again.
func TestServiceChecksPermissionAgain(t *testing.T) {
	svc, _, _ := newFixture(t)
	partner := setupPartner(t, svc)
	if _, err := svc.AddUser(partner, NewUser{LoginName: "asha", DisplayName: "Asha", Role: "staff", Password: startPassword}); err != nil {
		t.Fatal(err)
	}
	staff := findAccount(t, mustAccounts(t, svc), "asha")
	if _, err := svc.AddUser(staff, NewUser{LoginName: "other", DisplayName: "O", Role: "staff", Password: startPassword}); codeOf(err) != "FORBIDDEN" {
		t.Errorf("staff adding a user: %v", err)
	}
	if err := svc.SetLockout(staff, Lockout{MaxFailedLogins: 2, LockMinutes: 2}); codeOf(err) != "FORBIDDEN" {
		t.Errorf("staff changing the lockout: %v", err)
	}
}

func findAccount(t *testing.T, accounts []Account, login string) Account {
	t.Helper()
	for _, a := range accounts {
		if a.LoginName == login {
			return a
		}
	}
	t.Fatalf("no account %s", login)
	return Account{}
}

func mustAccounts(t *testing.T, svc *Service) []Account {
	t.Helper()
	accounts, err := svc.Accounts()
	if err != nil {
		t.Fatal(err)
	}
	return accounts
}

func mustEntries(t *testing.T, svc *Service) []audit.Entry {
	t.Helper()
	entries, err := svc.AuditEntries()
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
