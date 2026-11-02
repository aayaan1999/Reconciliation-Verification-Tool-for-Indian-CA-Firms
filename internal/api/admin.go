package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/audit"
	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/auth"
)

// The Admin tab's endpoints (SPEC-012 §4): users, roles, passwords, disable and enable,
// unlock, the lockout setting and the audit log. Every route here needs PermManageUsers, the
// Partner's permission; the handlers do not check it again. There is deliberately no route
// that deletes a user.

// adminAuditLimit is how many of the newest audit entries the Admin tab lists.
const adminAuditLimit = 200

// adminUsers lists every user with the lockout setting.
func (s *Server) adminUsers(w http.ResponseWriter, _ *http.Request) {
	accounts, err := s.auth.Accounts()
	if err != nil {
		writeAuthError(w, err)
		return
	}
	lock, err := s.auth.Lockout()
	if err != nil {
		writeAuthError(w, err)
		return
	}
	now := time.Now()
	users := make([]accountOut, 0, len(accounts))
	for _, a := range accounts {
		users = append(users, accountJSONOf(a, now))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"users":   users,
		"roles":   roleNames(),
		"lockout": lockoutJSON(lock),
	})
}

func roleNames() []string {
	out := make([]string, 0, len(auth.Roles))
	for _, r := range auth.Roles {
		out = append(out, string(r))
	}
	return out
}

func lockoutJSON(l auth.Lockout) map[string]int {
	return map[string]int{"max_failed_logins": l.MaxFailedLogins, "lock_minutes": l.LockMinutes}
}

// adminAddUser adds a user with a starting password, who must change it at first sign-in.
func (s *Server) adminAddUser(w http.ResponseWriter, r *http.Request) {
	var in struct {
		LoginName   string `json:"login_name"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
		Password    string `json:"password"`
	}
	if !decodeAuthBody(w, r, &in, "send the login name, display name, role and starting password as JSON") {
		return
	}
	acc, err := s.auth.AddUser(accountFrom(r), auth.NewUser{LoginName: in.LoginName, DisplayName: in.DisplayName, Role: in.Role, Password: in.Password})
	if err != nil {
		writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, accountJSONOf(acc, time.Now()))
}

// userIDOf reads the user ID in the path.
func userIDOf(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id := r.PathValue("id")
	if !idPattern.MatchString(id) {
		writeError(w, http.StatusBadRequest, "BAD_USER_ID", "the user ID is a whole number")
		return 0, false
	}
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_USER_ID", "the user ID is a whole number")
		return 0, false
	}
	return n, true
}

// adminSetRole changes a user's role. The reason is required; the change applies at the
// user's next request.
func (s *Server) adminSetRole(w http.ResponseWriter, r *http.Request) {
	id, ok := userIDOf(w, r)
	if !ok {
		return
	}
	var in struct {
		Role   string `json:"role"`
		Reason string `json:"reason"`
	}
	if !decodeAuthBody(w, r, &in, "send the new role and the reason as JSON") {
		return
	}
	role, err := auth.ParseRole(in.Role)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "BAD_INPUT", err.Error())
		return
	}
	if err := s.auth.SetRole(accountFrom(r), id, role, in.Reason); err != nil {
		writeAuthError(w, err)
		return
	}
	s.writeUserAfter(w, id)
}

// adminResetPassword sets a new starting password. The user is asked to change it at next
// sign-in, and their open sessions end.
func (s *Server) adminResetPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := userIDOf(w, r)
	if !ok {
		return
	}
	var in struct {
		Password string `json:"password"`
	}
	if !decodeAuthBody(w, r, &in, "send the new starting password as JSON") {
		return
	}
	if err := s.auth.ResetPassword(accountFrom(r), id, in.Password); err != nil {
		writeAuthError(w, err)
		return
	}
	s.writeUserAfter(w, id)
}

// adminSetActive disables or enables a user. The reason is required.
func (s *Server) adminSetActive(w http.ResponseWriter, r *http.Request) {
	id, ok := userIDOf(w, r)
	if !ok {
		return
	}
	var in struct {
		Active *bool  `json:"active"`
		Reason string `json:"reason"`
	}
	if !decodeAuthBody(w, r, &in, "send active (true or false) and the reason as JSON") {
		return
	}
	if in.Active == nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "say whether the user is active: true or false")
		return
	}
	if err := s.auth.SetActive(accountFrom(r), id, *in.Active, in.Reason); err != nil {
		writeAuthError(w, err)
		return
	}
	s.writeUserAfter(w, id)
}

// adminUnlock clears a user's lock.
func (s *Server) adminUnlock(w http.ResponseWriter, r *http.Request) {
	id, ok := userIDOf(w, r)
	if !ok {
		return
	}
	if err := s.auth.Unlock(accountFrom(r), id); err != nil {
		writeAuthError(w, err)
		return
	}
	s.writeUserAfter(w, id)
}

// adminLockout changes how many failed sign-ins lock an account, and for how many minutes.
func (s *Server) adminLockout(w http.ResponseWriter, r *http.Request) {
	var in struct {
		MaxFailedLogins int `json:"max_failed_logins"`
		LockMinutes     int `json:"lock_minutes"`
	}
	if !decodeAuthBody(w, r, &in, "send max_failed_logins and lock_minutes as JSON") {
		return
	}
	l := auth.Lockout{MaxFailedLogins: in.MaxFailedLogins, LockMinutes: in.LockMinutes}
	if err := s.auth.SetLockout(accountFrom(r), l); err != nil {
		writeAuthError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, lockoutJSON(l))
}

// adminAudit lists the newest audit entries and says whether the hash chain is intact.
// Entries are read only; nothing here changes the log.
func (s *Server) adminAudit(w http.ResponseWriter, _ *http.Request) {
	entries, err := s.auth.AuditEntries()
	if err != nil {
		writeAuthError(w, err)
		return
	}
	chain := map[string]any{"ok": true, "message": "every entry links to the one before it"}
	if err := audit.Verify(entries); err != nil {
		chain = map[string]any{"ok": false, "message": err.Error()}
	}
	type entryOut struct {
		Seq        int64           `json:"seq"`
		At         string          `json:"at"`
		Actor      string          `json:"actor"`
		Action     string          `json:"action"`
		ObjectType string          `json:"object_type"`
		ObjectID   string          `json:"object_id"`
		Details    json.RawMessage `json:"details"`
	}
	var out []entryOut
	for i := len(entries) - 1; i >= 0 && len(out) < adminAuditLimit; i-- {
		e := entries[i]
		details := json.RawMessage(e.Details)
		if !json.Valid(details) {
			details = json.RawMessage(`{}`)
		}
		out = append(out, entryOut{Seq: e.Seq, At: e.At, Actor: e.Actor, Action: e.Action,
			ObjectType: e.ObjectType, ObjectID: e.ObjectID, Details: details})
	}
	if out == nil {
		out = []entryOut{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": out, "total": len(entries), "chain": chain})
}

// writeUserAfter answers with the user as it is after a change.
func (s *Server) writeUserAfter(w http.ResponseWriter, id int64) {
	accounts, err := s.auth.Accounts()
	if err != nil {
		writeAuthError(w, err)
		return
	}
	for _, a := range accounts {
		if a.ID == id {
			writeJSON(w, http.StatusOK, accountJSONOf(a, time.Now()))
			return
		}
	}
	writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "there is no such user")
}
