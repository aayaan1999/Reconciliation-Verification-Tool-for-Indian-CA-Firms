package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/auth"
)

// The sign-in endpoints and the permission check on every route (SPEC-012, technical spec
// §B12.2). The session is a cookie: HttpOnly, Secure and SameSite=Strict. Its value is a random
// token; the database keeps only its hash.

// sessionCookie is the name of the session cookie.
const sessionCookie = "recon_session"

// accountKey carries the signed-in user of a request into its handler.
type accountKey struct{}

// authorize runs a handler only for a caller allowed to use it:
//   - PermOpen: anyone (the login and the first-start setup);
//   - PermSignedIn: any signed-in user, also while a password change is pending;
//   - any other permission: a signed-in user whose password change is done and whose role
//     holds the permission in the matrix. The role is read on every request, so a change of
//     role applies at once and a disabled user is refused at once.
//
// The refusals are 401 when nobody is signed in and 403 when the role or the pending change
// refuses the call. A 403 by the role names the permission and writes one auth.denied entry with
// the user, the route and the permission (SPEC-013 §1). A web page cannot skip this: the server
// checks, not the screen.
func (s *Server) authorize(route string, perm auth.Permission, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if perm == auth.PermOpen {
			h(w, r)
			return
		}
		acc, ok := s.signedInAccount(w, r)
		if !ok {
			return
		}
		if perm != auth.PermSignedIn {
			switch {
			case acc.MustChangePassword:
				writeError(w, http.StatusForbidden, "PASSWORD_CHANGE_REQUIRED", "change your password before using the tool")
				return
			case !auth.Allowed(acc.Role, perm):
				if err := s.recordDenied(acc, route, perm); err != nil {
					writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
					return
				}
				writeJSON(w, http.StatusForbidden, map[string]string{"error_code": "FORBIDDEN",
					"message":    "your role does not allow this: it needs the " + string(perm) + " permission",
					"permission": string(perm)})
				return
			}
		}
		h(w, r.WithContext(context.WithValue(r.Context(), accountKey{}, acc)))
	}
}

// signedInAccount returns the user of the request's session. A missing, ended or disabled
// session is 401 and clears the cookie.
func (s *Server) signedInAccount(w http.ResponseWriter, r *http.Request) (auth.Account, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil || c.Value == "" {
		writeError(w, http.StatusUnauthorized, "NOT_SIGNED_IN", "Sign in to continue.")
		return auth.Account{}, false
	}
	acc, err := s.auth.Current(c.Value)
	if err != nil {
		writeAuthError(w, err)
		return auth.Account{}, false
	}
	return acc, true
}

// accountFrom returns the signed-in user that authorize put into the request.
func accountFrom(r *http.Request) auth.Account {
	acc, _ := r.Context().Value(accountKey{}).(auth.Account)
	return acc
}

// actorOf is the login name the audit log and the engine record for a change made by the
// request. It is the signed-in user's login name, never a name the screen typed.
func actorOf(r *http.Request) string {
	return accountFrom(r).LoginName
}

// setSessionCookie gives the browser the session token.
func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/",
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
	})
}

// clearSessionCookie removes the session cookie from the browser.
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode,
	})
}

// writeAuthError answers a refusal of the rules with its code and sentence. Other errors are
// internal; their text names files and tables, never a password or a name.
func writeAuthError(w http.ResponseWriter, err error) {
	var ref *auth.Refusal
	if !errors.As(err, &ref) {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	status := http.StatusUnprocessableEntity
	switch ref.Code {
	case "BAD_CREDENTIALS", "ACCOUNT_LOCKED", "ACCOUNT_DISABLED", "NOT_SIGNED_IN", "SESSION_ENDED":
		status = http.StatusUnauthorized
	case "FORBIDDEN", "WRONG_PASSWORD":
		status = http.StatusForbidden
	case "USER_NOT_FOUND":
		status = http.StatusNotFound
	case "LOGIN_NAME_TAKEN", "SETUP_DONE":
		status = http.StatusConflict
	}
	writeError(w, status, ref.Code, ref.Message)
}

// authBodyLimit bounds the JSON of the sign-in and user endpoints.
const authBodyLimit = 1 << 14

// decodeAuthBody reads a small JSON body strictly: unknown keys are refused.
func decodeAuthBody(w http.ResponseWriter, r *http.Request, v any, hint string) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, authBodyLimit))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", hint)
		return false
	}
	return true
}

// sessionJSON is the signed-in user as the screens read it: the account, the permissions its
// role holds (the screens show tabs and buttons from this list, not from a copy of their own),
// and whether a password change is pending.
type sessionJSON struct {
	User               accountOut `json:"user"`
	Permissions        []string   `json:"permissions"`
	MustChangePassword bool       `json:"must_change_password"`
}

// accountOut is a user as the API sends it. It never carries a password or its hash.
type accountOut struct {
	ID                 int64  `json:"id"`
	LoginName          string `json:"login_name"`
	DisplayName        string `json:"display_name"`
	Role               string `json:"role"`
	Active             bool   `json:"active"`
	MustChangePassword bool   `json:"must_change_password"`
	FailedLogins       int    `json:"failed_logins"`
	Locked             bool   `json:"locked"`
	LockedUntil        string `json:"locked_until"`
	LastLoginAt        string `json:"last_login_at"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

func accountJSONOf(a auth.Account, now time.Time) accountOut {
	return accountOut{ID: a.ID, LoginName: a.LoginName, DisplayName: a.DisplayName, Role: string(a.Role),
		Active: a.Active, MustChangePassword: a.MustChangePassword, FailedLogins: a.FailedLogins,
		Locked: a.Locked(now), LockedUntil: a.LockedUntil, LastLoginAt: a.LastLoginAt,
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt}
}

func sessionOf(a auth.Account) sessionJSON {
	var perms []string
	for _, p := range auth.PermissionsOf(a.Role) {
		perms = append(perms, string(p))
	}
	if perms == nil {
		perms = []string{}
	}
	return sessionJSON{User: accountJSONOf(a, time.Now()), Permissions: perms, MustChangePassword: a.MustChangePassword}
}

// login checks a login name and password and starts a session.
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		LoginName string `json:"login_name"`
		Password  string `json:"password"`
	}
	if !decodeAuthBody(w, r, &in, "send the login name and password as JSON") {
		return
	}
	token, acc, err := s.auth.Login(in.LoginName, in.Password)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, sessionOf(acc))
}

// logout ends the session of the request and writes the sign-out to the audit log.
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.auth.Logout(c.Value); err != nil {
			writeAuthError(w, err)
			return
		}
	}
	clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"signed_out": true})
}

// me returns the signed-in user and the permissions of the role.
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, sessionOf(accountFrom(r)))
}

// changePassword replaces the signed-in user's password. The current password is required.
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decodeAuthBody(w, r, &in, "send the current and the new password as JSON") {
		return
	}
	if err := s.auth.ChangeOwnPassword(accountFrom(r), in.CurrentPassword, in.NewPassword); err != nil {
		writeAuthError(w, err)
		return
	}
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "NOT_SIGNED_IN", "Sign in to continue.")
		return
	}
	acc, err := s.auth.Current(c.Value)
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sessionOf(acc))
}

// setupStatus says whether the first Partner must still be set up.
func (s *Server) setupStatus(w http.ResponseWriter, _ *http.Request) {
	needed, err := s.auth.SetupNeeded()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needed": needed})
}

// setup creates the first Partner, once. The Partner's own password is kept; it is not forced
// to change. The response is the account, not a session: the Partner then signs in.
func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		LoginName   string `json:"login_name"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
	}
	if !decodeAuthBody(w, r, &in, "send the login name, display name and password as JSON") {
		return
	}
	acc, err := s.auth.Setup(auth.Setup{LoginName: in.LoginName, DisplayName: in.DisplayName, Password: in.Password})
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, accountJSONOf(acc, time.Now()))
}
