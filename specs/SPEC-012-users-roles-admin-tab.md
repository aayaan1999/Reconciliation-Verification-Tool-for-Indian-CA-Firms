# SPEC-012 — Users, roles and the Admin tab

| | |
|---|---|
| **Status** | In progress |
| **Phase** | 6 |
| **Depends on** | SPEC-010 (the audit log and the rule activation that roles must guard) |
| **Requirements** | §3 (users and roles), NF-04 (logins and roles, session timeout), requirements §3 role table |
| **Technical spec** | §B12.1 (role matrix), §B12.2 (sessions, argon2id, lockout, audit), §B11 (every handler declares a permission), §B1 (`users` table to be added to the schema) |
| **Code / content paths** | `internal/store/migrations/0008_users.sql` (new), `internal/auth` (new: password hashing, sessions, role matrix), `internal/store` (user reads and writes), `cmd/recon-engine` (`user-add`, `user-set-role`, `user-reset-password` commands), `internal/api` (login, logout, user endpoints; permission on every handler), `web/src/screens/Login.tsx` (new), `web/src/screens/Admin.tsx` (new), `web/src/App.tsx` (Admin tab, role-based tab and button visibility), `web/src/api.ts`, `web/src/types.ts` |

## Goal

Each person who uses the tool logs in with their own name and password. The Partner, acting as administrator, opens an **Admin** tab to add a user, gives that user a starting password and changes the role of any person. Each role can do only what the role matrix allows, and every change to a user is written to the audit log.

## Scope

### 1. Users table

- A new `users` table holds: user ID, login name (unique, case-insensitive), display name, role (`partner`, `manager`, `staff`, `read_only`), password hash (argon2id), active flag, created time, last changed time, failed-login count and locked-until time.
- No plain-text password is stored or logged. Only the argon2id hash is kept.
- The table is added by migration `0008_users.sql`, which follows the existing migration pattern.

### 2. Login and sessions

- A login screen asks for login name and password. A wrong password is refused with the same message whether the name exists or not.
- Sessions use an HttpOnly, Secure, SameSite=Strict cookie, with an idle timeout of 30 minutes and an absolute timeout of 10 hours (§B12.2).
- After repeated failed logins the account is locked for a set time. The lock and its length are configurable and shown on the Admin tab.
- Every API handler declares the permission it needs. A test fails if any handler does not (§B11).

### 3. Role matrix

The permissions follow §B12.1 exactly:

| Role | Can do |
|---|---|
| Partner | Everything, including the Admin tab, activating rules and exports |
| Manager | View, upload and run, decide findings, delete findings, draft rules, export |
| Staff | View, upload and run, decide findings, draft rules. Cannot delete findings, export, or activate or disable rules |
| Read-only | View only |

A role check runs on the server for every request. Hiding a button in the browser is not enough.

### 4. Admin tab (Partner only)

The Admin tab is visible only to a Partner. It shows the list of users with role, active flag and last login.

- **Add user.** The Partner types login name, display name, role and a starting password. The password is shown once as typed (masked field with a "show" toggle) and is not shown again after saving.
- **Change role.** The Partner picks a new role for any user, with a reason. The change takes effect at that user's next request.
- **Reset password.** The Partner sets a new starting password for a user. The user is asked to change it at next login.
- **Disable or enable a user.** A disabled user cannot log in and their open sessions end at once. A reason is required.
- **No delete.** The Admin tab and the API offer no delete action for a user. A user who leaves the firm is disabled, so every past audit entry still names them.
- **Lockout.** The Admin tab shows locked accounts and lets the Partner unlock one.

### 5. Guard rails

- At least one active Partner must always exist. A change that would leave none (demoting or disabling the last Partner) is refused.
- A Partner cannot disable their own account or remove their own Partner role while logged in. Another Partner must do it.
- The first Partner is created by a one-time setup step on the login screen, run once on first start. The setup asks for the Partner's login name, display name and their own password, which the Partner chooses; the forced password change in §4 does not apply to this account. The step is refused once any user exists, and it writes an audit entry.

### 6. Audit

- Every add, role change, password reset, disable, enable, unlock, login success, login failure and logout writes one `audit_log` entry (§B12.2).
- The **actor** is the logged-in user's login name, taken from the session. It is no longer a name typed on the screen. This changes how decisions and rule changes record their actor from this spec onward, and earlier rows keep the name typed then.
- Password values never appear in the audit entry, in logs or in error messages.

## Out of scope

- HTTPS with a firm-local certificate (T-Q2 is open; the server stays on `127.0.0.1` until it is decided).
- Single sign-on, Active Directory or any login from outside the office.
- Password recovery by email. None is possible offline; the Partner resets it on the Admin tab.
- Two-factor sign-in.
- Whether Managers may disable rules (T-Q3 is open; the matrix stays as in §B12.1 until the CA answers).

## Design notes

- Password hashing uses argon2id with parameters chosen once and stored with each hash, so they can be raised later without breaking old logins.
- The session store lives in the database, so a restart does not log everyone out silently and sessions can be ended by the Partner.
- The role matrix is one Go table in `internal/auth`, read by the handlers and by the UI tab visibility. The UI reads the list of permissions the current user holds from the server; it does not keep its own copy.
- Decided with the Partner, and built in as requirements:
  1. **Starting passwords are changed at first login.** A user who gets a starting password from the Partner is shown a change-password screen before anything else, and no other page is reachable until the change is made. Each starting password is one-time.
  2. **Users are disabled, never deleted.** A disabled user keeps their row, so every audit entry still names them. Re-enabling restores their access with the same role.
  3. **The first Partner is set up on first start** by the one-time setup step in §5, which the Partner completes themselves.

## Acceptance criteria

- [x] AC-1 — `users` table exists after migration `0008`; a second migration run changes nothing; the password column holds only argon2id hashes (a test reads the row and checks the prefix).
- [x] AC-2 — the Partner adds a user on the Admin tab with a starting password; that user logs in with it and is shown the change-password screen; no other page is reachable until the password is changed, and the starting password stops working once changed.
- [x] AC-3 — a wrong password is refused with the same message for an unknown name and a known one.
- [x] AC-4 — after the configured number of failed logins the account is locked; the Partner unlocks it on the Admin tab and the user can log in again.
- [x] AC-5 — the Partner changes a user's role; the user's next request is allowed or refused by the new role, with no re-login needed.
- [x] AC-6 — for each role, a test calls every API handler and checks the allowed and refused results against the §B12.1 matrix (no role can do what the matrix does not give it).
- [x] AC-7 — the Admin tab and the Admin endpoints are refused to Manager, Staff and Read-only, both in the UI and on the server.
- [x] AC-8 — the last active Partner cannot be demoted or disabled; a Partner cannot disable their own account.
- [x] AC-9 — the setup step creates the first Partner once, with the Partner's own password and no forced change; a second attempt is refused and the refusal is audited.
- [x] AC-15 — no route, button or command deletes a user; a test lists every route and command and finds none that removes a `users` row; a disabled user's rows and audit entries remain, and re-enabling restores the same role.
- [x] AC-10 — a disabled user's open sessions stop working on their next request.
- [x] AC-11 — every add, role change, reset, disable, enable, unlock, login and logout writes one audit entry with the logged-in actor; no password value appears in the audit log, logs or error text (a test searches all of them).
- [x] AC-12 — idle timeout (30 minutes) and absolute timeout (10 hours) end a session; both are tested with a fake clock.
- [x] AC-13 — the server binds to `127.0.0.1` by default; no CDN, web font or remote asset is loaded by the login or Admin screens.
- [x] AC-14 — the engine binary still links no network code (archtest passes; `internal/auth` is classified in `internal/archtest/archtest_test.go`).
- [x] Every file this spec created is added to `docs/Tool1-File-Guide.docx` in plain language.

## Verification

- `go test -count=1 ./internal/auth/... ./internal/store/... ./internal/api/...` covers AC-1 to AC-12.
- A handler-permission test (AC-6, AC-7) walks every registered route and fails on a route with no declared permission.
- `go test -count=1 ./internal/archtest/ -v` for AC-14.
- `cd web && npm test && npm run lint && npm run build` for the Login and Admin screens and the role-based tab visibility.
- Manual: as Partner, add a Staff user, log in as that user, confirm Admin is hidden and an Admin URL is refused, change the user to Manager on the Admin tab, log in again and confirm Delete findings and Export appear. Record the result in the completion record.
- Manual: check the audit log with `recon-engine` (or the Admin tab's chain check) and confirm each step above wrote an entry with the Partner's login name.

## Open questions

- **T-Q3:** may Managers disable rules as well as Partners? (affects §B12.1 and AC-6).
- **T-Q2:** HTTPS on the office LAN, and how staff browsers trust the firm-local certificate (needed before the server listens beyond `127.0.0.1`).
- Decided (see Design notes): starting passwords are changed at first login; users are disabled, never deleted; the first Partner is set up on first start.

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
