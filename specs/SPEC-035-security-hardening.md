# SPEC-035 — Security hardening: HTTPS, sessions, role matrix

| | |
|---|---|
| **Status** | Not started (certificate approach: T-Q2) |
| **Phase** | 6 |
| **Depends on** | SPEC-022 |
| **Requirements** | NF-04, requirements §3 |
| **Technical spec** | §B12.1, §B12.2 |
| **Code / content paths** | `cmd/recon-server`, `internal/authz`, `internal/api` (middleware), `internal/certs` |

## Goal

All traffic on the office network is HTTPS, sessions expire, brute-force logins are blocked, and the full role matrix is enforced and tested.

## Scope

- A firm-local CA and server certificate are generated at install, using the server's fixed name/IP (ENV-06), and renewed before expiry.
- The T-Q2 decision is implemented: either a guide for importing the CA on staff PCs, or documented acceptance of the browser warning.
- HTTPS only: HTTP redirects to HTTPS, with HSTS on the LAN name.
- Sessions: idle timeout (default 30 min), absolute timeout (10 h), and every session logged out on a password change.
- Login lockout after repeated failures, with Partner unlock.
- Security headers: CSP with no external sources, frame-ancestors none, no-sniff.
- Full §B12.1 matrix tests, including the delete-finding and export permissions.
- Security checklist document `docs/security-checklist.md` (Phase 6 gate).

## Acceptance criteria

- [ ] AC-1 — Plain HTTP requests are redirected; only TLS 1.2 or later is accepted.
- [ ] AC-2 — An idle session expires after the configured time.
- [ ] AC-3 — Repeated wrong passwords lock the account, and a Partner can unlock it.
- [ ] AC-4 — CSP blocks any external resource (tested with an injected external script tag).
- [ ] AC-5 — Role matrix tests pass for every route.
- [ ] AC-6 — The security checklist is completed and signed off.

## Verification

```
go test ./internal/authz/... ./internal/api/... -run 'Session|Lockout|Role|Header' -v
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
