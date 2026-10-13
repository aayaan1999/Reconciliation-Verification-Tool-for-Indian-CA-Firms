# SPEC-039 — Data protection: client export, deletion, retention

| | |
|---|---|
| **Status** | Not started (no compliance wording until the Q7 legal opinion) |
| **Phase** | 6 |
| **Depends on** | SPEC-034, SPEC-036 |
| **Requirements** | NF-09 |
| **Technical spec** | §B1.3 |
| **Code / content paths** | `internal/dataprotection` (server only), Admin screens in `web/` |

## Goal

The firm can find and export all data held for one client, delete a client completely with an audit trail, and apply a retention period. The tool makes no compliance claim.

## Scope

- Client data export (Partner only): a zip of all facts, records, findings, worklist items, decisions and original files for one client.
- Client deletion (Partner only, typed confirmation): removes the client's facts, records, findings, worklist, documents, and any vault files not shared with another client. It writes a tombstone audit entry containing IDs only.
- A retention setting per tax year, with a report listing engagements past retention. Deletion stays a manual Partner action.
- Access report: who viewed a client's file and when (from the audit log).
- UI and docs use neutral wording ("supports the firm's obligations"). A test fails if "DPDP compliant" appears in UI strings or docs before Q7 is closed.

## Acceptance criteria

- [ ] AC-1 — After a client is deleted, no row in any table references the client ID except the tombstone, and its vault files are gone.
- [ ] AC-2 — The audit chain still verifies after a deletion.
- [ ] AC-3 — The client export contains every document version and finding for that client and nothing from other clients.
- [ ] AC-4 — The access report matches the audit log for a test client.
- [ ] AC-5 — The compliance-wording test passes.

## Verification

```
go test ./internal/dataprotection/... -v
```

## Completion record

| Date | By | Commit | Notes |
|---|---|---|---|
| | | | |
