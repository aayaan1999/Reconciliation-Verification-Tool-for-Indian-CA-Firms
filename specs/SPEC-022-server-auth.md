# SPEC-022 — Server foundation, login and roles

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 4 |
| **Depends on** | SPEC-010 |
| **Requirements** | ENV-02, ENV-03, ENV-06, requirements §3, NF-04 (logins, roles) |
| **Technical spec** | §A3, §B11, §B12.1 |
| **Code / content paths** | `cmd/recon-server`, `internal/api`, `internal/authz`, migration adding `users`, `sessions` |

## Goal

`recon-server` serves the embedded UI and a JSON API on the office network, with logins and the four roles enforced on every endpoint.

## Scope

- Config file (`recon.toml` or YAML): bind address and port, data directory, worker count, session timeouts.
- The server binds to a configured LAN address and refuses to start if that address is not local.
- It serves `web/dist` via `go:embed`, with all fonts and libraries bundled (ENV-03). A test fails if the built UI references any `http://` or `https://` URL outside the server's own origin.
- Users: create (first run creates the Partner account), disable, reset password, assign role. Passwords are hashed with argon2id.
- Sessions: a random ID in an HttpOnly, SameSite=Strict cookie, plus a CSRF token for state-changing requests.
- The permission matrix of §B12.1 lives in `internal/authz`. Every handler declares its permission, and a test enumerates the routes and fails if any handler lacks one.
- Endpoints: `/api/v1/me`, `/login`, `/logout`, user admin, and a health endpoint for the installer.
- Plain HTTP is allowed only in development builds; HTTPS is SPEC-035.

## Out of scope

- TLS, lockout and timeout hardening (SPEC-035). Audit log (SPEC-036).

## Acceptance criteria

- [ ] AC-1 — Every role gets 403 for each action the matrix denies (table-driven test over all routes).
- [ ] AC-2 — The route-permission test fails if a new handler is added without a permission.
- [ ] AC-3 — The built UI makes no request to any external origin (automated test, plus a manual check with the network disconnected).
- [ ] AC-4 — A state-changing request without the CSRF token is rejected.
- [ ] AC-5 — The server refuses to bind to a non-local address.
- [ ] AC-6 — The server is reachable from a second PC on the office LAN (manual, on the test setup from Q5).

## Verification

```
go test ./internal/api/... ./internal/authz/... -v
go run ./cmd/recon-server --config dev.toml
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
