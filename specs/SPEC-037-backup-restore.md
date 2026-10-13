# SPEC-037 — Backup and restore

| | |
|---|---|
| **Status** | Not started |
| **Phase** | 6 |
| **Depends on** | SPEC-034 |
| **Requirements** | ENV-07, NF-07 |
| **Technical spec** | §B12.2 |
| **Code / content paths** | `internal/backup` (server only), Admin backup screen in `web/` |

## Goal

A daily encrypted backup goes to a second drive, an external disk or a NAS. Restore is tested regularly and proven to work.

## Scope

- A daily backup at a configured time to a configured path, plus a manual "Back up now".
- The DB is copied with the SQLite online backup API, so the copy is consistent while the tool is running. Vault files are copied incrementally; they are content-addressed, so only new files are copied.
- The backup set stays encrypted (same keys) and includes a manifest with file hashes.
- Retention: keep the last N daily sets (configurable).
- Restore goes into a new data directory using the recovery code, and verifies the manifest hashes and the audit chain.
- A monthly automatic restore test into a scratch directory, with the result shown in Admin.
- Admin alerts when a backup fails or the target is unreachable.

## Acceptance criteria

- [ ] AC-1 — A backup taken during a running reconciliation restores to a consistent DB (integrity check passes).
- [ ] AC-2 — Restoring on another machine with the recovery code gives identical row counts, file hashes and findings.
- [ ] AC-3 — The automatic restore test runs and reports success or failure in Admin.
- [ ] AC-4 — An unavailable backup target raises an Admin alert and does not crash anything.
- [ ] AC-5 — Old sets beyond retention are removed, and the newest N remain.

## Verification

```
go test ./internal/backup/... -v
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
