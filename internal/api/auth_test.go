package api

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aayaan1999/Reconciliation-Verification-Tool-for-Indian-CA-Firms/internal/auth"
)

// The sign-in, role and Admin tests of SPEC-012 (AC-2, AC-3, AC-5, AC-6, AC-7, AC-9, AC-10,
// AC-11, AC-13). They run against the real server over TLS, as a browser would reach it.

// specAllows is technical spec §B12.1, written out again here so the walk checks the server
// against the spec and not against its own table. "signed_in" and "open" are the two
// permissions that are not role permissions.
func specAllows(role, perm string) bool {
	switch perm {
	case "open", "signed_in":
		return true
	case "view":
		return true
	case "run", "decide", "draft_rules":
		return role != "read_only"
	case "delete_findings", "export":
		return role == "partner" || role == "manager"
	case "activate_rules", "disable_rules", "manage_users":
		return role == "partner"
	}
	return false
}

// fillPathValues puts sample values in the path parameters of a route pattern: a client ID, a
// rule version and a finding ID of the form the server makes.
func fillPathValues(path string) string {
	return strings.NewReplacer("{id}", "1", "{version}", "1", "{finding}", "ABCDEFGHJKMNPQRS").Replace(path)
}

// startingPassword and changedPassword are test values for the users the walk creates.
const (
	startingPassword = "starting-test-2026"
	changedPassword  = "changed-test-2026"
)

// userClient makes a user of a role with the Partner, signs them in with the starting
// password, changes it, and returns a client holding their new session.
func userClient(t *testing.T, e env, login, role string) *http.Client {
	t.Helper()
	status, out := postJSON(t, e.client, e.srv.URL+"/api/v1/admin/users", map[string]string{
		"login_name": login, "display_name": login, "role": role, "password": startingPassword})
	if status != http.StatusCreated {
		t.Fatalf("add %s: status %d: %s", login, status, out)
	}
	c := e.newClient(t)
	if status, out := postJSON(t, c, e.srv.URL+"/api/v1/login", map[string]string{"login_name": login, "password": startingPassword}); status != http.StatusOK {
		t.Fatalf("first sign in as %s: status %d: %s", login, status, out)
	}
	if status, out := postJSON(t, c, e.srv.URL+"/api/v1/me/password", map[string]string{
		"current_password": startingPassword, "new_password": changedPassword}); status != http.StatusOK {
		t.Fatalf("change password of %s: status %d: %s", login, status, out)
	}
	return c
}

// postJSON sends a JSON body and returns the status and the response text.
func postJSON(t *testing.T, c *http.Client, url string, body any) (int, string) {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(out)
}

// codeAndStatus sends one request and returns the status and the error code (empty on success).
func codeAndStatus(t *testing.T, c *http.Client, method, url, body string) (int, string) {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rd)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		Code string `json:"error_code"`
	}
	json.Unmarshal(raw, &out)
	return resp.StatusCode, out.Code
}

// AC-6 and AC-7: every route, for every role, is allowed or refused exactly as §B12.1 gives.
// A refusal by the role is 403 FORBIDDEN; the handler's own answer (a 400, a 404, a 422) means
// the role was let through. Sign-out is left out: it would end the session the walk uses.
func TestEveryRouteFollowsTheRoleMatrix(t *testing.T) {
	e := newEnv(t)
	clients := map[string]*http.Client{
		"partner":   e.client,
		"manager":   userClient(t, e, "meera", "manager"),
		"staff":     userClient(t, e, "sunil", "staff"),
		"read_only": userClient(t, e, "tara", "read_only"),
	}
	walked := 0
	for _, ep := range e.server.endpoints() {
		if ep.perm == "open" || strings.HasSuffix(ep.pattern, "/logout") {
			continue
		}
		walked++
		method, path, _ := strings.Cut(ep.pattern, " ")
		path = fillPathValues(path)
		body := ""
		if method != http.MethodGet {
			body = "{}"
		}
		for _, role := range []string{"partner", "manager", "staff", "read_only"} {
			status, code := codeAndStatus(t, clients[role], method, e.srv.URL+path, body)
			allowed := specAllows(role, string(ep.perm)) || ep.perm == "signed_in"
			switch {
			case allowed && (status == http.StatusUnauthorized || code == "FORBIDDEN" || code == "PASSWORD_CHANGE_REQUIRED"):
				t.Errorf("%s %s as %s: refused (%d %s), but §B12.1 allows %s", method, path, role, status, code, ep.perm)
			case !allowed && code != "FORBIDDEN":
				t.Errorf("%s %s as %s: status %d %s, want FORBIDDEN (§B12.1 does not give %s)", method, path, role, status, code, ep.perm)
			}
		}
	}
	if walked < 35 { // the routes behind sign-in, less sign-out (see endpoints)
		t.Errorf("the walk checked %d routes, want at least 35", walked)
	}
	t.Logf("walked %d routes for four roles", walked)
}

// Every route refuses a caller who is not signed in with 401, except the open ones.
func TestEveryRouteNeedsSignInExceptTheOpenOnes(t *testing.T) {
	e := newEnv(t)
	anon := e.newClient(t)
	for _, ep := range e.server.endpoints() {
		if ep.perm == "open" {
			continue
		}
		method, path, _ := strings.Cut(ep.pattern, " ")
		path = fillPathValues(path)
		body := ""
		if method != http.MethodGet {
			body = "{}"
		}
		if status, code := codeAndStatus(t, anon, method, e.srv.URL+path, body); status != http.StatusUnauthorized || code != "NOT_SIGNED_IN" {
			t.Errorf("%s %s without a session: %d %s, want 401 NOT_SIGNED_IN", method, path, status, code)
		}
	}
}

// The registration check: every route is declared in endpoints() with a permission, and no
// other code registers a route. It reads the package's source, so a route cannot be added by
// calling HandleFunc somewhere else and skipping the permission.
func TestRoutesAreOnlyRegisteredFromTheDeclaredList(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || (sel.Sel.Name != "HandleFunc" && sel.Sel.Name != "Handle") {
					return true
				}
				if fn.Name.Name != "Handler" {
					t.Errorf("%s: %s registers a route outside Handler, which uses endpoints()", fset.Position(call.Pos()), fn.Name.Name)
				}
				return true
			})
		}
	}
	e := newEnv(t)
	for _, ep := range e.server.endpoints() {
		if ep.perm == "" || ep.handler == nil {
			t.Errorf("route %q has no permission or no handler", ep.pattern)
		}
		// AC-15 of SPEC-012: no route deletes a user. SPEC-013 adds a route that deletes a
		// finding, which is only marked deleted (its user-free pattern is allowed).
		lower := strings.ToLower(ep.pattern)
		if strings.HasPrefix(ep.pattern, "DELETE ") || (strings.Contains(lower, "delete") && strings.Contains(lower, "/users")) {
			t.Errorf("route %q would delete something; users are disabled, never deleted", ep.pattern)
		}
	}
}

// AC-2: a starting password signs the user in, and every page but the password change is refused
// until the change is made. The starting password stops working once changed.
func TestStartingPasswordGatesEverythingElse(t *testing.T) {
	e := newEnv(t)
	if status, out := postJSON(t, e.client, e.srv.URL+"/api/v1/admin/users", map[string]string{
		"login_name": "asha", "display_name": "Asha", "role": "manager", "password": startingPassword}); status != http.StatusCreated {
		t.Fatalf("add: %d %s", status, out)
	}
	c := e.newClient(t)
	status, out := postJSON(t, c, e.srv.URL+"/api/v1/login", map[string]string{"login_name": "asha", "password": startingPassword})
	if status != http.StatusOK || !strings.Contains(out, `"must_change_password":true`) {
		t.Fatalf("first sign-in: %d %s", status, out)
	}
	if status, code := codeAndStatus(t, c, "GET", e.srv.URL+"/api/v1/rules", ""); status != http.StatusForbidden || code != "PASSWORD_CHANGE_REQUIRED" {
		t.Errorf("a page before the change: %d %s, want PASSWORD_CHANGE_REQUIRED", status, code)
	}
	if status, _ := codeAndStatus(t, c, "GET", e.srv.URL+"/api/v1/me", ""); status != http.StatusOK {
		t.Errorf("the session check while a change is pending: %d, want 200", status)
	}
	if status, out := postJSON(t, c, e.srv.URL+"/api/v1/me/password", map[string]string{"current_password": "wrong-password-here", "new_password": changedPassword}); status != http.StatusForbidden || !strings.Contains(out, "WRONG_PASSWORD") {
		t.Errorf("a change with the wrong current password: %d %s", status, out)
	}
	if status, out := postJSON(t, c, e.srv.URL+"/api/v1/me/password", map[string]string{"current_password": startingPassword, "new_password": changedPassword}); status != http.StatusOK || !strings.Contains(out, `"must_change_password":false`) {
		t.Fatalf("the change: %d %s", status, out)
	}
	if status, _ := codeAndStatus(t, c, "GET", e.srv.URL+"/api/v1/rules", ""); status != http.StatusOK {
		t.Errorf("a page after the change: %d, want 200", status)
	}
	if status, out := postJSON(t, e.newClient(t), e.srv.URL+"/api/v1/login", map[string]string{"login_name": "asha", "password": startingPassword}); status != http.StatusUnauthorized {
		t.Errorf("the starting password after the change: %d %s, want 401", status, out)
	}
}

// AC-3: a wrong password gets the same answer for an unknown name and a known one; the answer
// never repeats the password.
func TestSignInRefusalIsTheSameForAnyName(t *testing.T) {
	e := newEnv(t)
	c := e.newClient(t)
	s1, b1 := postJSON(t, c, e.srv.URL+"/api/v1/login", map[string]string{"login_name": testPartner, "password": "not-the-password"})
	s2, b2 := postJSON(t, c, e.srv.URL+"/api/v1/login", map[string]string{"login_name": "nobody-here", "password": "not-the-password"})
	if s1 != http.StatusUnauthorized || s1 != s2 || b1 != b2 {
		t.Errorf("known name: %d %s; unknown name: %d %s; want the same refusal", s1, b1, s2, b2)
	}
	if strings.Contains(b1, "not-the-password") {
		t.Error("the refusal repeats the password")
	}
}

// AC-5 and AC-10 over HTTP: a role change applies at the next request with no sign-in again; a
// disable ends the open session at once.
func TestRoleChangeAndDisableApplyAtNextRequest(t *testing.T) {
	e := newEnv(t)
	staff := userClient(t, e, "sunil", "staff")
	if status, code := codeAndStatus(t, staff, "GET", e.srv.URL+"/api/v1/admin/users", ""); status != http.StatusForbidden || code != "FORBIDDEN" {
		t.Fatalf("staff on the Admin tab: %d %s", status, code)
	}
	id := userIDByLogin(t, e, "sunil")
	if status, out := postJSON(t, e.client, e.srv.URL+"/api/v1/admin/users/"+id+"/role", map[string]string{"role": "partner", "reason": "promoted"}); status != http.StatusOK {
		t.Fatalf("role change: %d %s", status, out)
	}
	if status, code := codeAndStatus(t, staff, "GET", e.srv.URL+"/api/v1/admin/users", ""); status != http.StatusOK {
		t.Errorf("the Admin tab after the change: %d %s, want 200 with no sign-in", status, code)
	}
	if status, out := postJSON(t, e.client, e.srv.URL+"/api/v1/admin/users/"+id+"/active", map[string]any{"active": false, "reason": "left the firm"}); status != http.StatusOK {
		t.Fatalf("disable: %d %s", status, out)
	}
	if status, code := codeAndStatus(t, staff, "GET", e.srv.URL+"/api/v1/me", ""); status != http.StatusUnauthorized || code != "NOT_SIGNED_IN" {
		t.Errorf("a disabled user's open session: %d %s, want 401", status, code)
	}
}

// userIDByLogin reads the ID of a user from the Admin list.
func userIDByLogin(t *testing.T, e env, login string) string {
	t.Helper()
	resp, err := e.client.Get(e.srv.URL + "/api/v1/admin/users")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		Users []struct {
			ID        int64  `json:"id"`
			LoginName string `json:"login_name"`
		} `json:"users"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	for _, u := range out.Users {
		if u.LoginName == login {
			return strconv.FormatInt(u.ID, 10)
		}
	}
	t.Fatalf("no user %s", login)
	return ""
}

// AC-4 over HTTP: the lock after the configured failures, and the Partner's unlock.
func TestLockoutOverHTTP(t *testing.T) {
	e := newEnv(t)
	userClient(t, e, "sunil", "staff")
	if status, out := postJSON(t, e.client, e.srv.URL+"/api/v1/admin/lockout", map[string]int{"max_failed_logins": 2, "lock_minutes": 10}); status != http.StatusOK {
		t.Fatalf("lockout setting: %d %s", status, out)
	}
	c := e.newClient(t)
	for i := 0; i < 2; i++ {
		postJSON(t, c, e.srv.URL+"/api/v1/login", map[string]string{"login_name": "sunil", "password": "guess-wrong-here"})
	}
	if status, out := postJSON(t, c, e.srv.URL+"/api/v1/login", map[string]string{"login_name": "sunil", "password": changedPassword}); status != http.StatusUnauthorized || !strings.Contains(out, "ACCOUNT_LOCKED") {
		t.Fatalf("right password while locked: %d %s", status, out)
	}
	id := userIDByLogin(t, e, "sunil")
	if status, out := postJSON(t, e.client, e.srv.URL+"/api/v1/admin/users/"+id+"/unlock", map[string]string{}); status != http.StatusOK {
		t.Fatalf("unlock: %d %s", status, out)
	}
	if status, out := postJSON(t, e.newClient(t), e.srv.URL+"/api/v1/login", map[string]string{"login_name": "sunil", "password": changedPassword}); status != http.StatusOK {
		t.Errorf("after the unlock: %d %s", status, out)
	}
}

// AC-9 over HTTP: the first start offers the setup once; a second attempt is refused.
func TestFirstPartnerSetupOnce(t *testing.T) {
	e := startEnv(t, filepath.Join(t.TempDir(), "first.db"))
	c := e.newClient(t)
	resp, err := c.Get(e.srv.URL + "/api/v1/setup")
	if err != nil {
		t.Fatal(err)
	}
	var st struct {
		Needed bool `json:"needed"`
	}
	json.NewDecoder(resp.Body).Decode(&st)
	resp.Body.Close()
	if !st.Needed {
		t.Fatal("setup is not offered on an empty database")
	}
	setup := map[string]string{"login_name": "priya", "display_name": "A. Partner", "password": testPartnerPassword}
	if status, out := postJSON(t, c, e.srv.URL+"/api/v1/setup", setup); status != http.StatusCreated || strings.Contains(out, testPartnerPassword) {
		t.Fatalf("setup: %d %s", status, out)
	}
	if status, out := postJSON(t, c, e.srv.URL+"/api/v1/setup", setup); status != http.StatusConflict || !strings.Contains(out, "SETUP_DONE") {
		t.Errorf("second setup: %d %s, want 409 SETUP_DONE", status, out)
	}
	if status, out := postJSON(t, c, e.srv.URL+"/api/v1/login", map[string]string{"login_name": "priya", "password": testPartnerPassword}); status != http.StatusOK || strings.Contains(out, `"must_change_password":true`) {
		t.Errorf("the first Partner signs in with no forced change: %d %s", status, out)
	}
}

// AC-11: the audit endpoint names the Partner for each change, and no password value is in
// any response or in the audit entries.
func TestAuditNamesThePartnerAndHoldsNoPassword(t *testing.T) {
	e := newEnv(t)
	secret := "SECRET-ADMIN-PASSWORD-5521"
	if status, out := postJSON(t, e.client, e.srv.URL+"/api/v1/admin/users", map[string]string{
		"login_name": "asha", "display_name": "Asha", "role": "staff", "password": secret}); status != http.StatusCreated {
		t.Fatalf("add: %d %s", status, out)
	}
	postJSON(t, e.newClient(t), e.srv.URL+"/api/v1/login", map[string]string{"login_name": "asha", "password": "guess-wrong-here"})
	resp, err := e.client.Get(e.srv.URL + "/api/v1/admin/audit")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.Contains(string(raw), secret) {
		t.Error("a password value appears in the audit endpoint")
	}
	var out struct {
		Entries []struct {
			Actor  string `json:"actor"`
			Action string `json:"action"`
		} `json:"entries"`
		Chain struct {
			OK bool `json:"ok"`
		} `json:"chain"`
	}
	json.Unmarshal(raw, &out)
	if !out.Chain.OK {
		t.Error("the audit chain does not verify")
	}
	found := false
	for _, en := range out.Entries {
		if en.Action == "user.add" && en.Actor == testPartner {
			found = true
		}
	}
	if !found {
		t.Errorf("no user.add entry by %s in %s", testPartner, raw)
	}
}

// The sign-in cookie: HttpOnly, Secure and SameSite=Strict.
func TestSessionCookieAttributes(t *testing.T) {
	e := newEnv(t)
	resp, err := e.client.Post(e.srv.URL+"/api/v1/login", "application/json",
		strings.NewReader(`{"login_name":"priya","password":"`+testPartnerPassword+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == sessionCookie {
			cookie = c
		}
	}
	if cookie == nil || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("session cookie %+v: want HttpOnly, Secure, SameSite=Strict", cookie)
	}
}

// AC-12 at the HTTP layer: a session that has been idle longer than the timeout is refused. The
// fake clock of the service is not reachable from here, so the expiry is set in the database.
func TestExpiredSessionIsRefused(t *testing.T) {
	e := newEnv(t)
	token := e.sessionToken(t)
	old := time.Now().UTC().Add(-(auth.IdleTimeout + time.Minute)).Format(time.RFC3339)
	e.setLastSeen(t, token, old)
	if status, code := codeAndStatus(t, e.client, "GET", e.srv.URL+"/api/v1/me", ""); status != http.StatusUnauthorized || code != "SESSION_ENDED" {
		t.Errorf("an idle session: %d %s, want 401 SESSION_ENDED", status, code)
	}
}

// sessionToken reads the cookie value of the test client.
func (e env) sessionToken(t *testing.T) string {
	t.Helper()
	u, err := url.Parse(e.srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range e.client.Jar.Cookies(u) {
		if c.Name == sessionCookie {
			return c.Value
		}
	}
	t.Fatal("no session cookie")
	return ""
}

// setLastSeen moves a session's last-seen time, for the timeout test.
func (e env) setLastSeen(t *testing.T, token, at string) {
	t.Helper()
	tx, err := e.server.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := tx.TouchSession(auth.HashToken(token), at); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}
