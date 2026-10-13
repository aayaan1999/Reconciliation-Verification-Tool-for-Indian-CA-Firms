# SPEC-036 — Hash-chained audit log

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 6 |
| **Depends on** | SPEC-022 |
| **Requirements** | NF-05, RU-42, RU-34, NF-09 (access logs) |
| **Technical spec** | §B1.3, §B12.2 |
| **Code / content paths** | `internal/audit`, migration adding `audit_log` and its triggers, Admin audit viewer in `web/` |

## Goal

Every view, decision, export, rule change, law-table change and pack action is written to an append-only, hash-chained log that can be verified.

## Scope

- `audit_log` table, with `BEFORE UPDATE` / `BEFORE DELETE` triggers that raise errors.
- `hash = sha256(prev_hash || canonical(row))`; the first row chains from a fixed genesis value.
- Events logged: login, logout and failed login; viewing a client file; upload; run; finding decisions; delete; export; rule create/edit/test/approve/disable/reset; law-table row add/approve; pack install/rollback; user admin; backup/restore.
- Every governance event carries the reason the user entered.
- Chain verification runs on demand in Admin and daily in the background. A break raises an Admin alert naming the first bad sequence number.
- Audit viewer: filter by user, client, action and date; export (Partner only).
- Writes go through the server's writer goroutine. The engine's system transitions are logged by the server when it receives `job.done`.

## Acceptance criteria

- [ ] AC-1 — Each listed event type produces an entry (table-driven test).
- [ ] AC-2 — `UPDATE` or `DELETE` on `audit_log` fails.
- [ ] AC-3 — Altering one row's details directly in the DB file (via a test harness) is detected by chain verification at that row.
- [ ] AC-4 — Rule activation without a reason is rejected.
- [ ] AC-5 — Logging 100,000 events and verifying the chain takes under 10 s.

## Verification

```
go test ./internal/audit/... -v
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
