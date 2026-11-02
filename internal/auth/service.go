package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/audit"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/store"
)

// Session timeouts (technical spec §B12.2): a session ends 30 minutes after the latest request
// and 10 hours after it began, whichever comes first.
const (
	IdleTimeout     = 30 * time.Minute
	AbsoluteTimeout = 10 * time.Hour
)

// Actors written to the audit log when no user is signed in. A typed login name is recorded
// only when it is a real user's; anything else would put typed text (perhaps a password typed
// by mistake) into the log.
const (
	ActorNotSignedIn = "(not signed in)"
	ActorNoSuchUser  = "(no such user)"
)

// Lockout limits the Admin tab may set.
const (
	MinMaxFailedLogins = 1
	MaxMaxFailedLogins = 20
	MinLockMinutes     = 1
	MaxLockMinutes     = 1440
)

// loginNamePattern is the form a login name must have: letters and digits, with dots,
// underscores or hyphens, two to forty characters.
var loginNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{1,39}$`)

// A Refusal is a request the rules refuse. Code is stable (the screens show their own text
// for it); Message is the sentence for the person. No message names a password.
type Refusal struct {
	Code    string
	Message string
}

func (e *Refusal) Error() string { return e.Message }

func refuse(code, message string) error { return &Refusal{Code: code, Message: message} }

// The refusals. The sign-in ones deliberately say the same thing for an unknown name and a
// wrong password (AC-3).
var (
	errBadCredentials = refuse("BAD_CREDENTIALS", "The login name or password is not right.")
	errLocked         = refuse("ACCOUNT_LOCKED", "This account is locked after too many failed sign-ins. A Partner can unlock it on the Admin tab.")
	errDisabled       = refuse("ACCOUNT_DISABLED", "This account is disabled. A Partner can enable it on the Admin tab.")
	errNotSignedIn    = refuse("NOT_SIGNED_IN", "Sign in to continue.")
	errSessionEnded   = refuse("SESSION_ENDED", "The session has ended. Sign in again.")
	errSetupDone      = refuse("SETUP_DONE", "The first Partner is already set up. Sign in instead.")
	errForbidden      = refuse("FORBIDDEN", "Your role does not allow this.")
	errLastPartner    = refuse("LAST_PARTNER", "At least one active Partner must remain. Make another user a Partner first.")
	errSelfChange     = refuse("SELF_CHANGE", "A Partner cannot change their own account while signed in. Another Partner must do it.")
	errNoChange       = refuse("NO_CHANGE", "Nothing would change.")
	errUserNotFound   = refuse("USER_NOT_FOUND", "There is no such user.")
	errNameTaken      = refuse("LOGIN_NAME_TAKEN", "Another user already has that login name.")
	errWrongPassword  = refuse("WRONG_PASSWORD", "The current password is not right.")
)

// Account is a user as the rest of the program sees it. It never carries the password hash.
type Account struct {
	ID                 int64
	LoginName          string
	DisplayName        string
	Role               Role
	MustChangePassword bool
	Active             bool
	FailedLogins       int
	LockedUntil        string // UTC, RFC 3339; "" when not locked
	LastLoginAt        string // UTC, RFC 3339; "" when never
	CreatedAt          string
	UpdatedAt          string
}

// Locked reports whether the account is locked at now.
func (a Account) Locked(now time.Time) bool {
	until, ok := parseStamp(a.LockedUntil)
	return ok && now.Before(until)
}

// Lockout is the sign-in lock setting shown on the Admin tab.
type Lockout struct {
	MaxFailedLogins int
	LockMinutes     int
}

// A NewUser is what the Partner types on the Admin tab to add a user.
type NewUser struct {
	LoginName   string
	DisplayName string
	Role        string
	Password    string // the starting password; it is changed at the user's first sign-in
}

// A Setup is the first Partner, created once on first start with the Partner's own password.
type Setup struct {
	LoginName   string
	DisplayName string
	Password    string
}

// A Service runs every sign-in and user rule. It holds no state of its own: everything is in
// the database, so a restart changes nothing and every rule is checked in a transaction.
type Service struct {
	db     *store.DB
	now    func() time.Time
	params Params
	dummy  string // a hash of a placeholder, verified when a login name is unknown, so both cases take about the same time
}

// NewService returns a service that uses the database and the caller's clock.
func NewService(db *store.DB, now func() time.Time) (*Service, error) {
	return newService(db, now, DefaultParams)
}

func newService(db *store.DB, now func() time.Time, p Params) (*Service, error) {
	if now == nil {
		return nil, fmt.Errorf("auth: a clock is required")
	}
	dummy, err := HashPassword("placeholder-for-unknown-names", p)
	if err != nil {
		return nil, err
	}
	return &Service{db: db, now: now, params: p, dummy: dummy}, nil
}

// SetupNeeded reports whether no user exists yet, so the first Partner must be set up.
func (s *Service) SetupNeeded() (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	n, err := tx.UserCount()
	return n == 0, err
}

// Setup creates the first Partner. It runs once: after any user exists it is refused and the
// refusal is audited. The Partner chooses the password, so it is not forced to change.
func (s *Service) Setup(in Setup) (Account, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Account{}, err
	}
	defer tx.Rollback()
	now := s.now()
	n, err := tx.UserCount()
	if err != nil {
		return Account{}, err
	}
	if n > 0 {
		if err := event(tx, now, ActorNotSignedIn, "setup.refused", "setup", "first_partner",
			map[string]string{"reason": "users already exist"}); err != nil {
			return Account{}, err
		}
		return Account{}, commitThen(tx, errSetupDone)
	}
	loginName, displayName, err := checkNames(in.LoginName, in.DisplayName)
	if err != nil {
		return Account{}, err
	}
	if err := CheckPasswordPolicy(in.Password); err != nil {
		return Account{}, badInput(err)
	}
	hash, err := HashPassword(in.Password, s.params)
	if err != nil {
		return Account{}, err
	}
	at := stamp(now)
	id, err := tx.InsertUser(store.User{LoginName: loginName, DisplayName: displayName, Role: string(RolePartner),
		PasswordHash: hash, MustChangePassword: false, Active: true, CreatedAt: at, UpdatedAt: at})
	if err != nil {
		return Account{}, err
	}
	if err := event(tx, now, loginName, "user.setup", "user", loginName, map[string]string{"role": string(RolePartner)}); err != nil {
		return Account{}, err
	}
	acc, err := accountByID(tx, id)
	if err != nil {
		return Account{}, err
	}
	return acc, tx.Commit()
}

// Login checks a login name and password and, when both are right and the account may sign
// in, starts a session. It returns the session token for the cookie. The token is never
// stored: only its hash is.
func (s *Service) Login(loginName, password string) (string, Account, error) {
	loginName = strings.TrimSpace(loginName)
	tx, err := s.db.Begin()
	if err != nil {
		return "", Account{}, err
	}
	defer tx.Rollback()
	now := s.now()
	u, found, err := tx.UserByLogin(loginName)
	if err != nil {
		return "", Account{}, err
	}
	if !found {
		// Verify against a placeholder, so an unknown name takes as long as a known one.
		if _, err := VerifyPassword(password, s.dummy); err != nil {
			return "", Account{}, err
		}
		if err := event(tx, now, ActorNoSuchUser, "login.failure", "user", ActorNoSuchUser,
			map[string]string{"reason": "wrong login name or password"}); err != nil {
			return "", Account{}, err
		}
		return "", Account{}, commitThen(tx, errBadCredentials)
	}
	maxFailed, lockMinutes, err := tx.AuthSettings()
	if err != nil {
		return "", Account{}, err
	}
	acc, err := accountOf(u)
	if err != nil {
		return "", Account{}, err
	}
	if acc.Locked(now) {
		if err := event(tx, now, u.LoginName, "login.failure", "user", u.LoginName,
			map[string]string{"reason": "account locked"}); err != nil {
			return "", Account{}, err
		}
		return "", Account{}, commitThen(tx, errLocked)
	}
	ok, err := VerifyPassword(password, u.PasswordHash)
	if err != nil {
		return "", Account{}, err
	}
	if !ok {
		failed := u.FailedLogins + 1
		lockedUntil, reason := "", "wrong password"
		if failed >= maxFailed {
			lockedUntil = now.Add(time.Duration(lockMinutes) * time.Minute).UTC().Format(time.RFC3339)
			failed, reason = 0, "wrong password; account locked"
		}
		if err := tx.SetUserLoginState(u.ID, failed, lockedUntil, u.LastLoginAt, stamp(now)); err != nil {
			return "", Account{}, err
		}
		if err := event(tx, now, u.LoginName, "login.failure", "user", u.LoginName,
			map[string]string{"reason": reason}); err != nil {
			return "", Account{}, err
		}
		return "", Account{}, commitThen(tx, errBadCredentials)
	}
	if !u.Active {
		if err := event(tx, now, u.LoginName, "login.failure", "user", u.LoginName,
			map[string]string{"reason": "account disabled"}); err != nil {
			return "", Account{}, err
		}
		return "", Account{}, commitThen(tx, errDisabled)
	}
	token, err := NewToken()
	if err != nil {
		return "", Account{}, err
	}
	at := stamp(now)
	if err := tx.SetUserLoginState(u.ID, 0, "", at, at); err != nil {
		return "", Account{}, err
	}
	if err := tx.InsertSession(store.Session{TokenHash: HashToken(token), UserID: u.ID, CreatedAt: at,
		LastSeenAt: at, ExpiresAt: stamp(now.Add(AbsoluteTimeout))}); err != nil {
		return "", Account{}, err
	}
	if err := event(tx, now, u.LoginName, "login.success", "user", u.LoginName, nil); err != nil {
		return "", Account{}, err
	}
	fresh, err := accountByID(tx, u.ID)
	if err != nil {
		return "", Account{}, err
	}
	return token, fresh, tx.Commit()
}

// Logout ends the session of a token and writes the sign-out to the audit log. A token that
// is not a session does nothing.
func (s *Service) Logout(token string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := s.now()
	sess, ok, err := tx.SessionByToken(HashToken(token))
	if err != nil || !ok {
		return err
	}
	u, found, err := tx.UserByID(sess.UserID)
	if err != nil {
		return err
	}
	if err := tx.DeleteSession(sess.TokenHash); err != nil {
		return err
	}
	if found {
		if err := event(tx, now, u.LoginName, "logout", "user", u.LoginName, nil); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Current returns the account of a token's session and records the request, which restarts
// the idle timeout. A session past either timeout, or whose account is disabled, is ended
// and refused. Every request calls it, so a role change or a disable applies at once.
func (s *Service) Current(token string) (Account, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Account{}, err
	}
	defer tx.Rollback()
	now := s.now()
	hash := HashToken(token)
	sess, ok, err := tx.SessionByToken(hash)
	if err != nil {
		return Account{}, err
	}
	if !ok {
		return Account{}, errNotSignedIn
	}
	last, okLast := parseStamp(sess.LastSeenAt)
	expires, okExp := parseStamp(sess.ExpiresAt)
	if !okLast || !okExp || !now.Before(expires) || now.Sub(last) >= IdleTimeout {
		if err := tx.DeleteSession(hash); err != nil {
			return Account{}, err
		}
		return Account{}, commitThen(tx, errSessionEnded)
	}
	u, found, err := tx.UserByID(sess.UserID)
	if err != nil {
		return Account{}, err
	}
	if !found {
		if err := tx.DeleteSession(hash); err != nil {
			return Account{}, err
		}
		return Account{}, commitThen(tx, errNotSignedIn)
	}
	if !u.Active {
		if err := tx.DeleteSession(hash); err != nil {
			return Account{}, err
		}
		return Account{}, commitThen(tx, errDisabled)
	}
	if err := tx.TouchSession(hash, stamp(now)); err != nil {
		return Account{}, err
	}
	acc, err := accountOf(u)
	if err != nil {
		return Account{}, err
	}
	return acc, tx.Commit()
}

// ChangeOwnPassword replaces the signed-in user's password. The current password must be
// given, so a starting password stops working once it is changed (AC-2).
func (s *Service) ChangeOwnPassword(actor Account, current, next string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := s.now()
	u, found, err := tx.UserByID(actor.ID)
	if err != nil {
		return err
	}
	if !found {
		return errNotSignedIn
	}
	ok, err := VerifyPassword(current, u.PasswordHash)
	if err != nil {
		return err
	}
	if !ok {
		return errWrongPassword
	}
	if err := CheckPasswordPolicy(next); err != nil {
		return badInput(err)
	}
	if next == current {
		return badInput(fmt.Errorf("the new password must differ from the current one"))
	}
	hash, err := HashPassword(next, s.params)
	if err != nil {
		return err
	}
	if err := tx.SetUserPassword(u.ID, hash, false, stamp(now)); err != nil {
		return err
	}
	if err := event(tx, now, u.LoginName, "password.change", "user", u.LoginName, nil); err != nil {
		return err
	}
	return tx.Commit()
}

// AddUser adds a user with a starting password. The user must change it at first sign-in.
func (s *Service) AddUser(actor Account, in NewUser) (Account, error) {
	if err := requireManager(actor); err != nil {
		return Account{}, err
	}
	loginName, displayName, err := checkNames(in.LoginName, in.DisplayName)
	if err != nil {
		return Account{}, err
	}
	role, err := ParseRole(in.Role)
	if err != nil {
		return Account{}, badInput(err)
	}
	if err := CheckPasswordPolicy(in.Password); err != nil {
		return Account{}, badInput(err)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return Account{}, err
	}
	defer tx.Rollback()
	now := s.now()
	if _, taken, err := tx.UserByLogin(loginName); err != nil {
		return Account{}, err
	} else if taken {
		return Account{}, errNameTaken
	}
	hash, err := HashPassword(in.Password, s.params)
	if err != nil {
		return Account{}, err
	}
	at := stamp(now)
	id, err := tx.InsertUser(store.User{LoginName: loginName, DisplayName: displayName, Role: string(role),
		PasswordHash: hash, MustChangePassword: true, Active: true, CreatedAt: at, UpdatedAt: at})
	if err != nil {
		return Account{}, err
	}
	if err := event(tx, now, actor.LoginName, "user.add", "user", loginName,
		map[string]string{"role": string(role)}); err != nil {
		return Account{}, err
	}
	acc, err := accountByID(tx, id)
	if err != nil {
		return Account{}, err
	}
	return acc, tx.Commit()
}

// SetRole changes a user's role. The reason is required. The change applies at the user's
// next request, because every request reads the role. A Partner cannot demote themselves, and
// the last active Partner cannot be demoted.
func (s *Service) SetRole(actor Account, userID int64, role Role, reason string) error {
	if err := requireManager(actor); err != nil {
		return err
	}
	reason, err := checkReason(reason)
	if err != nil {
		return err
	}
	if !slices.Contains(Roles, role) {
		return badInput(fmt.Errorf("the role must be one of partner, manager, staff, read_only"))
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := s.now()
	target, found, err := tx.UserByID(userID)
	if err != nil {
		return err
	}
	if !found {
		return errUserNotFound
	}
	from := Role(target.Role)
	switch {
	case from == role:
		return errNoChange
	case from == RolePartner && role != RolePartner && target.ID == actor.ID:
		return errSelfChange
	case from == RolePartner && role != RolePartner && target.Active:
		n, err := tx.CountActivePartners()
		if err != nil {
			return err
		}
		if n <= 1 {
			return errLastPartner
		}
	}
	if err := tx.SetUserRole(target.ID, string(role), stamp(now)); err != nil {
		return err
	}
	if err := event(tx, now, actor.LoginName, "user.role_change", "user", target.LoginName, map[string]string{
		"from": string(from), "to": string(role), "reason": reason,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// ResetPassword sets a new starting password for a user, who must change it at next sign-in.
// The user's open sessions end at once.
func (s *Service) ResetPassword(actor Account, userID int64, password string) error {
	if err := requireManager(actor); err != nil {
		return err
	}
	if err := CheckPasswordPolicy(password); err != nil {
		return badInput(err)
	}
	hash, err := HashPassword(password, s.params)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := s.now()
	target, found, err := tx.UserByID(userID)
	if err != nil {
		return err
	}
	if !found {
		return errUserNotFound
	}
	if err := tx.SetUserPassword(target.ID, hash, true, stamp(now)); err != nil {
		return err
	}
	if err := tx.DeleteUserSessions(target.ID); err != nil {
		return err
	}
	if err := event(tx, now, actor.LoginName, "user.password_reset", "user", target.LoginName, nil); err != nil {
		return err
	}
	return tx.Commit()
}

// SetActive disables or enables a user. A reason is required. Disabling ends the user's open
// sessions; enabling restores the same role. A Partner cannot disable their own account, and
// the last active Partner cannot be disabled.
func (s *Service) SetActive(actor Account, userID int64, active bool, reason string) error {
	if err := requireManager(actor); err != nil {
		return err
	}
	reason, err := checkReason(reason)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := s.now()
	target, found, err := tx.UserByID(userID)
	if err != nil {
		return err
	}
	if !found {
		return errUserNotFound
	}
	if target.Active == active {
		return errNoChange
	}
	if !active {
		if target.ID == actor.ID {
			return errSelfChange
		}
		if target.Role == string(RolePartner) {
			n, err := tx.CountActivePartners()
			if err != nil {
				return err
			}
			if n <= 1 {
				return errLastPartner
			}
		}
	}
	if err := tx.SetUserActive(target.ID, active, stamp(now)); err != nil {
		return err
	}
	action := "user.enable"
	if !active {
		action = "user.disable"
		if err := tx.DeleteUserSessions(target.ID); err != nil {
			return err
		}
	}
	if err := event(tx, now, actor.LoginName, action, "user", target.LoginName, map[string]string{"reason": reason}); err != nil {
		return err
	}
	return tx.Commit()
}

// Unlock clears a lock and the failed-login count of a user.
func (s *Service) Unlock(actor Account, userID int64) error {
	if err := requireManager(actor); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := s.now()
	target, found, err := tx.UserByID(userID)
	if err != nil {
		return err
	}
	if !found {
		return errUserNotFound
	}
	if err := tx.SetUserLoginState(target.ID, 0, "", target.LastLoginAt, stamp(now)); err != nil {
		return err
	}
	if err := event(tx, now, actor.LoginName, "user.unlock", "user", target.LoginName, nil); err != nil {
		return err
	}
	return tx.Commit()
}

// SetLockout changes how many failed sign-ins lock an account, and for how many minutes.
func (s *Service) SetLockout(actor Account, l Lockout) error {
	if err := requireManager(actor); err != nil {
		return err
	}
	if l.MaxFailedLogins < MinMaxFailedLogins || l.MaxFailedLogins > MaxMaxFailedLogins {
		return badInput(fmt.Errorf("the number of failed sign-ins must be from %d to %d", MinMaxFailedLogins, MaxMaxFailedLogins))
	}
	if l.LockMinutes < MinLockMinutes || l.LockMinutes > MaxLockMinutes {
		return badInput(fmt.Errorf("the lock must last from %d to %d minutes", MinLockMinutes, MaxLockMinutes))
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := s.now()
	if err := tx.SetAuthSettings(l.MaxFailedLogins, l.LockMinutes, stamp(now)); err != nil {
		return err
	}
	if err := event(tx, now, actor.LoginName, "auth.lockout_set", "settings", "lockout", map[string]string{
		"max_failed_logins": strconv.Itoa(l.MaxFailedLogins), "lock_minutes": strconv.Itoa(l.LockMinutes),
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// Lockout returns the current lock setting.
func (s *Service) Lockout() (Lockout, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Lockout{}, err
	}
	defer tx.Rollback()
	maxFailed, lock, err := tx.AuthSettings()
	return Lockout{MaxFailedLogins: maxFailed, LockMinutes: lock}, err
}

// Accounts lists every user in a fixed order.
func (s *Service) Accounts() ([]Account, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	users, err := tx.ListUsers()
	if err != nil {
		return nil, err
	}
	out := make([]Account, 0, len(users))
	for _, u := range users {
		a, err := accountOf(u)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// AccountByLogin returns the user with a login name, ignoring capitals, and false when there
// is none. The user commands use it to find the Partner who acts and the user they change.
func (s *Service) AccountByLogin(loginName string) (Account, bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Account{}, false, err
	}
	defer tx.Rollback()
	u, found, err := tx.UserByLogin(strings.TrimSpace(loginName))
	if err != nil || !found {
		return Account{}, false, err
	}
	a, err := accountOf(u)
	if err != nil {
		return Account{}, false, err
	}
	return a, true, nil
}

// AuditEntries returns the whole audit log in sequence order.
func (s *Service) AuditEntries() ([]audit.Entry, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	return tx.AuditEntries()
}

// NewToken returns a new session token: 32 random bytes, base64url without padding.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken is the form a session token is stored in: hex SHA-256.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// stamp is a time as the database keeps it: UTC, RFC 3339.
func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// event writes one audit entry: who did it, what, and to which object. The actor is a login
// name that exists or one of the Actor constants.
func event(tx *store.Tx, now time.Time, actor, action, objectType, objectID string, details map[string]string) error {
	if details == nil {
		details = map[string]string{}
	}
	return tx.Audit(stamp(now), actor, action, objectType, objectID, details)
}

// commitThen commits the transaction and returns err. A refusal that writes an audit entry
// must commit it; the change it refused is not made.
func commitThen(tx *store.Tx, err error) error {
	if cerr := tx.Commit(); cerr != nil {
		return cerr
	}
	return err
}

func requireManager(actor Account) error {
	if !Allowed(actor.Role, PermManageUsers) {
		return errForbidden
	}
	return nil
}

// checkNames trims and checks a login name and a display name.
func checkNames(loginName, displayName string) (string, string, error) {
	loginName = strings.TrimSpace(loginName)
	displayName = strings.TrimSpace(displayName)
	switch {
	case !loginNamePattern.MatchString(loginName):
		return "", "", badInput(fmt.Errorf("the login name must be 2 to 40 letters, digits, dots, underscores or hyphens, starting with a letter or digit"))
	case displayName == "" || utf8.RuneCountInString(displayName) > 100:
		return "", "", badInput(fmt.Errorf("the display name must have 1 to 100 characters"))
	}
	return loginName, displayName, nil
}

// checkReason trims a reason and requires one of at most 300 characters.
func checkReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	switch {
	case reason == "":
		return "", refuse("REASON_REQUIRED", "Say why: a reason is required.")
	case utf8.RuneCountInString(reason) > 300:
		return "", badInput(fmt.Errorf("the reason has at most 300 characters"))
	}
	return reason, nil
}

func badInput(err error) error { return refuse("BAD_INPUT", err.Error()) }

// accountByID reads a user inside a transaction.
func accountByID(tx *store.Tx, id int64) (Account, error) {
	u, found, err := tx.UserByID(id)
	if err != nil {
		return Account{}, err
	}
	if !found {
		return Account{}, errUserNotFound
	}
	return accountOf(u)
}

// accountOf converts a stored user, checking its role is one the matrix knows.
func accountOf(u store.User) (Account, error) {
	role, err := ParseRole(u.Role)
	if err != nil {
		return Account{}, fmt.Errorf("auth: user %d: %w", u.ID, err)
	}
	return Account{
		ID: u.ID, LoginName: u.LoginName, DisplayName: u.DisplayName, Role: role,
		MustChangePassword: u.MustChangePassword, Active: u.Active, FailedLogins: u.FailedLogins,
		LockedUntil: u.LockedUntil, LastLoginAt: u.LastLoginAt, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt,
	}, nil
}

// parseStamp reads an RFC 3339 time as the store keeps it.
func parseStamp(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, s)
	return t, err == nil
}
